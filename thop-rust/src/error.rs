use serde::Serialize;
use thiserror::Error;

/// Error codes for structured error handling
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub enum ErrorCode {
    #[serde(rename = "CONNECTION_FAILED")]
    ConnectionFailed,
    #[serde(rename = "CONNECTION_TIMEOUT")]
    ConnectionTimeout,
    #[serde(rename = "AUTH_PASSWORD_REQUIRED")]
    AuthPasswordRequired,
    #[serde(rename = "AUTH_KEY_REJECTED")]
    AuthKeyRejected,
    #[serde(rename = "AUTH_FAILED")]
    AuthFailed,
    #[serde(rename = "HOST_KEY_VERIFICATION_FAILED")]
    HostKeyVerificationFailed,
    #[serde(rename = "HOST_KEY_CHANGED")]
    HostKeyChanged,
    #[serde(rename = "COMMAND_TIMEOUT")]
    CommandTimeout,
    #[serde(rename = "COMMAND_RESTRICTED")]
    CommandRestricted,
    #[serde(rename = "SESSION_NOT_FOUND")]
    SessionNotFound,
    #[serde(rename = "SESSION_DISCONNECTED")]
    SessionDisconnected,
}

/// Structured session error
#[derive(Debug, Error, Serialize)]
#[error("{message}")]
pub struct SessionError {
    pub code: ErrorCode,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub host: Option<String>,
    pub retryable: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub suggestion: Option<String>,
}

impl SessionError {
    pub fn new(code: ErrorCode, message: impl Into<String>, session: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
            session: Some(session.into()),
            host: None,
            retryable: false,
            suggestion: None,
        }
    }

    pub fn with_host(mut self, host: impl Into<String>) -> Self {
        self.host = Some(host.into());
        self
    }

    pub fn with_retryable(mut self, retryable: bool) -> Self {
        self.retryable = retryable;
        self
    }

    pub fn with_suggestion(mut self, suggestion: impl Into<String>) -> Self {
        self.suggestion = Some(suggestion.into());
        self
    }

    pub fn session_not_found(name: &str) -> Self {
        Self::new(
            ErrorCode::SessionNotFound,
            format!("Session '{}' not found", name),
            name,
        )
    }

    pub fn session_disconnected(name: &str) -> Self {
        Self::new(
            ErrorCode::SessionDisconnected,
            format!("Session '{}' is not connected", name),
            name,
        )
        .with_suggestion("Use /connect to establish connection")
    }

    pub fn connection_failed(session: &str, host: &str, err: impl std::fmt::Display) -> Self {
        Self::new(
            ErrorCode::ConnectionFailed,
            format!("Failed to connect to {}: {}", host, err),
            session,
        )
        .with_host(host)
        .with_retryable(true)
        .with_suggestion("Check network connectivity and host address")
    }

    pub fn connection_timeout(session: &str, host: &str) -> Self {
        Self::new(
            ErrorCode::ConnectionTimeout,
            format!("Connection timed out to {}", host),
            session,
        )
        .with_host(host)
        .with_retryable(true)
        .with_suggestion("Check network connectivity and firewall settings")
    }

    pub fn auth_failed(session: &str, host: &str) -> Self {
        Self::new(
            ErrorCode::AuthFailed,
            format!("Authentication failed for {}", host),
            session,
        )
        .with_host(host)
        .with_suggestion("Check SSH key or credentials")
    }

    pub fn host_key_verification_failed(session: &str, host: &str) -> Self {
        Self::new(
            ErrorCode::HostKeyVerificationFailed,
            format!("Host key verification failed for {}", host),
            session,
        )
        .with_host(host)
        .with_suggestion("Add the host to known_hosts: ssh-keyscan <host> >> ~/.ssh/known_hosts")
    }

    pub fn command_restricted(command: &str, category: &str) -> Self {
        Self {
            code: ErrorCode::CommandRestricted,
            message: format!("{}: '{}' is not allowed in restricted mode", category, command),
            session: None,
            host: None,
            retryable: false,
            suggestion: Some("Remove --restricted flag to allow this command, or use a different approach".to_string()),
        }
    }
}

/// General application error
#[derive(Debug, Error)]
pub enum ThopError {
    #[error("{0}")]
    Session(#[from] SessionError),

    #[error("Configuration error: {0}")]
    Config(String),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("State error: {0}")]
    State(String),

    #[error("{0}")]
    Other(String),
}

pub type Result<T> = std::result::Result<T, ThopError>;
