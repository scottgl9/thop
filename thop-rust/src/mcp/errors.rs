//! MCP error codes and types

use serde::{Deserialize, Serialize};

use super::protocol::{Content, ToolCallResult};

/// Error codes for MCP responses
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum ErrorCode {
    // Session errors
    SessionNotFound,
    SessionNotConnected,
    SessionAlreadyExists,
    NoActiveSession,
    CannotCloseLocal,

    // Connection errors
    ConnectionFailed,
    AuthFailed,
    AuthKeyFailed,
    AuthPasswordFailed,
    HostKeyUnknown,
    HostKeyMismatch,
    ConnectionTimeout,
    ConnectionRefused,

    // Command execution errors
    CommandFailed,
    CommandTimeout,
    CommandNotFound,
    PermissionDenied,

    // Parameter errors
    InvalidParameter,
    MissingParameter,

    // Feature errors
    NotImplemented,
    OperationFailed,
}

impl std::fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            ErrorCode::SessionNotFound => "SESSION_NOT_FOUND",
            ErrorCode::SessionNotConnected => "SESSION_NOT_CONNECTED",
            ErrorCode::SessionAlreadyExists => "SESSION_ALREADY_EXISTS",
            ErrorCode::NoActiveSession => "NO_ACTIVE_SESSION",
            ErrorCode::CannotCloseLocal => "CANNOT_CLOSE_LOCAL",
            ErrorCode::ConnectionFailed => "CONNECTION_FAILED",
            ErrorCode::AuthFailed => "AUTH_FAILED",
            ErrorCode::AuthKeyFailed => "AUTH_KEY_FAILED",
            ErrorCode::AuthPasswordFailed => "AUTH_PASSWORD_FAILED",
            ErrorCode::HostKeyUnknown => "HOST_KEY_UNKNOWN",
            ErrorCode::HostKeyMismatch => "HOST_KEY_MISMATCH",
            ErrorCode::ConnectionTimeout => "CONNECTION_TIMEOUT",
            ErrorCode::ConnectionRefused => "CONNECTION_REFUSED",
            ErrorCode::CommandFailed => "COMMAND_FAILED",
            ErrorCode::CommandTimeout => "COMMAND_TIMEOUT",
            ErrorCode::CommandNotFound => "COMMAND_NOT_FOUND",
            ErrorCode::PermissionDenied => "PERMISSION_DENIED",
            ErrorCode::InvalidParameter => "INVALID_PARAMETER",
            ErrorCode::MissingParameter => "MISSING_PARAMETER",
            ErrorCode::NotImplemented => "NOT_IMPLEMENTED",
            ErrorCode::OperationFailed => "OPERATION_FAILED",
        };
        write!(f, "{}", s)
    }
}

/// Structured MCP error
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MCPError {
    pub code: ErrorCode,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub suggestion: Option<String>,
}

impl std::fmt::Display for MCPError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if let Some(ref session) = self.session {
            write!(f, "[{}] {} (session: {})", self.code, self.message, session)
        } else {
            write!(f, "[{}] {}", self.code, self.message)
        }
    }
}

impl std::error::Error for MCPError {}

impl MCPError {
    /// Create a new MCP error
    pub fn new(code: ErrorCode, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
            session: None,
            suggestion: None,
        }
    }

    /// Add session information to the error
    pub fn with_session(mut self, session: impl Into<String>) -> Self {
        self.session = Some(session.into());
        self
    }

    /// Add a suggestion to the error
    pub fn with_suggestion(mut self, suggestion: impl Into<String>) -> Self {
        self.suggestion = Some(suggestion.into());
        self
    }

    /// Convert error to a tool call result
    pub fn to_tool_result(&self) -> ToolCallResult {
        let mut text = self.message.clone();
        if let Some(ref suggestion) = self.suggestion {
            text = format!("{}\n\nSuggestion: {}", text, suggestion);
        }
        if let Some(ref session) = self.session {
            text = format!("{}\n\nSession: {}", text, session);
        }
        text = format!("[{}] {}", self.code, text);

        ToolCallResult {
            content: vec![Content::text(text)],
            is_error: true,
        }
    }

    // Common error constructors

    /// Session not found error
    pub fn session_not_found(session_name: &str) -> Self {
        Self::new(
            ErrorCode::SessionNotFound,
            format!("Session '{}' not found", session_name),
        )
        .with_session(session_name)
        .with_suggestion("Use /status to see available sessions or /add-session to create a new one")
    }

    /// Session not connected error
    pub fn session_not_connected(session_name: &str) -> Self {
        Self::new(
            ErrorCode::SessionNotConnected,
            format!("Session '{}' is not connected", session_name),
        )
        .with_session(session_name)
        .with_suggestion("Use /connect to establish a connection")
    }

    /// SSH key authentication failed error
    pub fn auth_key_failed(session_name: &str) -> Self {
        Self::new(ErrorCode::AuthKeyFailed, "SSH key authentication failed")
            .with_session(session_name)
            .with_suggestion("Use /auth to provide a password or check your SSH key configuration")
    }

    /// Password authentication failed error
    pub fn auth_password_failed(session_name: &str) -> Self {
        Self::new(
            ErrorCode::AuthPasswordFailed,
            "Password authentication failed",
        )
        .with_session(session_name)
        .with_suggestion("Verify the password is correct")
    }

    /// Host key unknown error
    pub fn host_key_unknown(session_name: &str) -> Self {
        Self::new(ErrorCode::HostKeyUnknown, "Host key is not in known_hosts")
            .with_session(session_name)
            .with_suggestion("Use /trust to accept the host key")
    }

    /// Connection failed error
    pub fn connection_failed(session_name: &str, reason: &str) -> Self {
        Self::new(ErrorCode::ConnectionFailed, format!("Connection failed: {}", reason))
            .with_session(session_name)
            .with_suggestion("Check network connectivity and session configuration")
    }

    /// Command timeout error
    pub fn command_timeout(session_name: &str, timeout: u64) -> Self {
        Self::new(
            ErrorCode::CommandTimeout,
            format!("Command execution timed out after {} seconds", timeout),
        )
        .with_session(session_name)
        .with_suggestion("Increase timeout parameter or run command in background")
    }

    /// Missing parameter error
    pub fn missing_parameter(param: &str) -> Self {
        Self::new(
            ErrorCode::MissingParameter,
            format!("Required parameter '{}' is missing", param),
        )
        .with_suggestion(format!("Provide the '{}' parameter", param))
    }

    /// Not implemented error
    pub fn not_implemented(feature: &str) -> Self {
        Self::new(
            ErrorCode::NotImplemented,
            format!("{} is not yet implemented", feature),
        )
        .with_suggestion("This feature is planned for a future release")
    }

    /// No active session error
    pub fn no_active_session() -> Self {
        Self::new(ErrorCode::NoActiveSession, "No active session")
            .with_suggestion("Use /connect to establish a session or specify a session name")
    }

    /// Cannot close local session error
    pub fn cannot_close_local(session_name: &str) -> Self {
        Self::new(ErrorCode::CannotCloseLocal, "Cannot close the local session")
            .with_session(session_name)
            .with_suggestion("Use /switch to change to another session instead")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_code_display() {
        assert_eq!(format!("{}", ErrorCode::SessionNotFound), "SESSION_NOT_FOUND");
        assert_eq!(format!("{}", ErrorCode::AuthKeyFailed), "AUTH_KEY_FAILED");
    }

    #[test]
    fn test_mcp_error_creation() {
        let err = MCPError::new(ErrorCode::SessionNotFound, "Test error");
        assert_eq!(err.code, ErrorCode::SessionNotFound);
        assert_eq!(err.message, "Test error");
        assert!(err.session.is_none());
        assert!(err.suggestion.is_none());
    }

    #[test]
    fn test_mcp_error_with_session() {
        let err = MCPError::new(ErrorCode::SessionNotFound, "Test error")
            .with_session("test-session");
        assert_eq!(err.session, Some("test-session".to_string()));
    }

    #[test]
    fn test_mcp_error_to_tool_result() {
        let err = MCPError::session_not_found("test-session");
        let result = err.to_tool_result();

        assert!(result.is_error);
        assert_eq!(result.content.len(), 1);
        assert!(result.content[0].text.as_ref().unwrap().contains("SESSION_NOT_FOUND"));
    }

    #[test]
    fn test_common_error_constructors() {
        let err = MCPError::session_not_found("prod");
        assert_eq!(err.code, ErrorCode::SessionNotFound);
        assert!(err.session.is_some());
        assert!(err.suggestion.is_some());

        let err = MCPError::command_timeout("prod", 30);
        assert_eq!(err.code, ErrorCode::CommandTimeout);
        assert!(err.message.contains("30"));
    }
}
