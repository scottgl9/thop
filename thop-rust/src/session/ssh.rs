use std::collections::HashMap;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::PathBuf;
use std::time::Duration;

use ssh2::Session as Ssh2Session;

use crate::error::{ErrorCode, Result, SessionError, ThopError};
use super::{ExecuteResult, Session};

/// SSH session configuration
pub struct SshConfig {
    pub host: String,
    pub user: String,
    pub port: u16,
    pub identity_file: Option<String>,
}

/// SSH session
pub struct SshSession {
    name: String,
    config: SshConfig,
    session: Option<Ssh2Session>,
    cwd: String,
    env: HashMap<String, String>,
}

impl SshSession {
    /// Create a new SSH session
    pub fn new(name: impl Into<String>, config: SshConfig) -> Self {
        Self {
            name: name.into(),
            config,
            session: None,
            cwd: "/".to_string(),
            env: HashMap::new(),
        }
    }

    /// Get the host address
    pub fn host(&self) -> &str {
        &self.config.host
    }

    /// Get the user
    pub fn user(&self) -> &str {
        &self.config.user
    }

    /// Get the port
    pub fn port(&self) -> u16 {
        self.config.port
    }

    /// Load known hosts and verify server key
    fn verify_host_key(session: &Ssh2Session, host: &str) -> Result<()> {
        // Get server's host key
        let (key, key_type) = session.host_key().ok_or_else(|| {
            SessionError::new(
                ErrorCode::HostKeyVerificationFailed,
                "No host key provided by server",
                "",
            )
        })?;

        // Load known_hosts
        let mut known_hosts = session.known_hosts().map_err(|e| {
            ThopError::Other(format!("Failed to create known_hosts: {}", e))
        })?;

        // Try to load known_hosts file
        let known_hosts_path = dirs::home_dir()
            .map(|p| p.join(".ssh/known_hosts"))
            .unwrap_or_else(|| PathBuf::from("/dev/null"));

        if known_hosts_path.exists() {
            known_hosts.read_file(&known_hosts_path, ssh2::KnownHostFileKind::OpenSSH)
                .map_err(|e| {
                    ThopError::Other(format!("Failed to read known_hosts: {}", e))
                })?;
        }

        // Check host key
        match known_hosts.check(host, key) {
            ssh2::CheckResult::Match => Ok(()),
            ssh2::CheckResult::NotFound => {
                Err(SessionError::host_key_verification_failed("", host).into())
            }
            ssh2::CheckResult::Mismatch => {
                Err(SessionError::new(
                    ErrorCode::HostKeyChanged,
                    format!("Host key for {} has changed! This could be a security issue.", host),
                    "",
                )
                .with_host(host)
                .with_suggestion("Remove the old key from known_hosts and re-verify")
                .into())
            }
            ssh2::CheckResult::Failure => {
                Err(SessionError::host_key_verification_failed("", host).into())
            }
        }
    }

    /// Authenticate using SSH agent or key file
    fn authenticate(&self, session: &Ssh2Session) -> Result<()> {
        // Try SSH agent first
        if let Ok(mut agent) = session.agent() {
            if agent.connect().is_ok() {
                agent.list_identities().ok();
                for identity in agent.identities().unwrap_or_default() {
                    if agent.userauth(&self.config.user, &identity).is_ok() {
                        return Ok(());
                    }
                }
            }
        }

        // Try identity file if specified
        if let Some(ref identity_file) = self.config.identity_file {
            let identity_path = if identity_file.starts_with('~') {
                dirs::home_dir()
                    .map(|p| p.join(&identity_file[2..]))
                    .unwrap_or_else(|| PathBuf::from(identity_file))
            } else {
                PathBuf::from(identity_file)
            };

            if identity_path.exists() {
                session.userauth_pubkey_file(
                    &self.config.user,
                    None,
                    &identity_path,
                    None,
                ).map_err(|e| {
                    SessionError::new(
                        ErrorCode::AuthKeyRejected,
                        format!("Key rejected: {}", e),
                        &self.name,
                    )
                    .with_host(&self.config.host)
                })?;

                return Ok(());
            }
        }

        // Try default key locations
        let home = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        let default_keys = [
            home.join(".ssh/id_ed25519"),
            home.join(".ssh/id_rsa"),
            home.join(".ssh/id_ecdsa"),
        ];

        for key_path in &default_keys {
            if key_path.exists() {
                if session.userauth_pubkey_file(&self.config.user, None, key_path, None).is_ok() {
                    return Ok(());
                }
            }
        }

        Err(SessionError::auth_failed(&self.name, &self.config.host).into())
    }
}

impl Session for SshSession {
    fn name(&self) -> &str {
        &self.name
    }

    fn session_type(&self) -> &str {
        "ssh"
    }

    fn is_connected(&self) -> bool {
        self.session.is_some()
    }

    fn connect(&mut self) -> Result<()> {
        if self.session.is_some() {
            return Ok(());
        }

        let addr = format!("{}:{}", self.config.host, self.config.port);

        // Connect with timeout
        let stream = TcpStream::connect_timeout(
            &addr.parse().map_err(|e| {
                SessionError::connection_failed(&self.name, &self.config.host, e)
            })?,
            Duration::from_secs(30),
        ).map_err(|e| {
            if e.kind() == std::io::ErrorKind::TimedOut {
                SessionError::connection_timeout(&self.name, &self.config.host)
            } else {
                SessionError::connection_failed(&self.name, &self.config.host, e)
            }
        })?;

        // Create SSH session
        let mut session = Ssh2Session::new().map_err(|e| {
            ThopError::Other(format!("Failed to create SSH session: {}", e))
        })?;

        session.set_tcp_stream(stream);
        session.handshake().map_err(|e| {
            SessionError::connection_failed(&self.name, &self.config.host, e)
        })?;

        // Verify host key
        Self::verify_host_key(&session, &self.config.host)?;

        // Authenticate
        self.authenticate(&session)?;

        // Get initial CWD
        let mut channel = session.channel_session().map_err(|e| {
            ThopError::Other(format!("Failed to open channel: {}", e))
        })?;

        channel.exec("pwd").map_err(|e| {
            ThopError::Other(format!("Failed to execute pwd: {}", e))
        })?;

        let mut output = String::new();
        channel.read_to_string(&mut output).ok();
        channel.wait_close().ok();

        self.cwd = output.trim().to_string();
        if self.cwd.is_empty() {
            self.cwd = "/".to_string();
        }

        self.session = Some(session);
        Ok(())
    }

    fn disconnect(&mut self) -> Result<()> {
        if let Some(session) = self.session.take() {
            session.disconnect(None, "Closing connection", None).ok();
        }
        Ok(())
    }

    fn execute(&mut self, cmd: &str) -> Result<ExecuteResult> {
        let session = self.session.as_ref().ok_or_else(|| {
            SessionError::session_disconnected(&self.name)
        })?;

        // Build command with cd and env
        let mut full_cmd = format!("cd {} && ", self.cwd);

        for (key, value) in &self.env {
            full_cmd.push_str(&format!("export {}='{}' && ", key, value.replace('\'', "'\\''")));
        }

        full_cmd.push_str(cmd);

        // Open channel
        let mut channel = session.channel_session().map_err(|e| {
            ThopError::Other(format!("Failed to open channel: {}", e))
        })?;

        channel.exec(&full_cmd).map_err(|e| {
            ThopError::Other(format!("Failed to execute command: {}", e))
        })?;

        // Read output
        let mut stdout = String::new();
        let mut stderr = String::new();

        channel.read_to_string(&mut stdout).ok();
        channel.stderr().read_to_string(&mut stderr).ok();

        channel.wait_close().ok();
        let exit_code = channel.exit_status().unwrap_or(-1);

        // Handle cd commands - update cwd
        let trimmed = cmd.trim();
        if trimmed == "cd" || trimmed.starts_with("cd ") {
            if exit_code == 0 {
                // Get new cwd
                if let Ok(result) = self.execute("pwd") {
                    if result.exit_code == 0 {
                        self.cwd = result.stdout.trim().to_string();
                    }
                }
            }
        }

        Ok(ExecuteResult {
            stdout,
            stderr,
            exit_code,
        })
    }

    fn get_cwd(&self) -> &str {
        &self.cwd
    }

    fn set_cwd(&mut self, path: &str) -> Result<()> {
        self.cwd = path.to_string();
        Ok(())
    }

    fn get_env(&self) -> HashMap<String, String> {
        self.env.clone()
    }

    fn set_env(&mut self, key: &str, value: &str) {
        self.env.insert(key.to_string(), value.to_string());
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_new_ssh_session() {
        let config = SshConfig {
            host: "example.com".to_string(),
            user: "testuser".to_string(),
            port: 22,
            identity_file: None,
        };

        let session = SshSession::new("test", config);
        assert_eq!(session.name(), "test");
        assert_eq!(session.session_type(), "ssh");
        assert!(!session.is_connected());
        assert_eq!(session.host(), "example.com");
        assert_eq!(session.user(), "testuser");
        assert_eq!(session.port(), 22);
    }

    #[test]
    fn test_env() {
        let config = SshConfig {
            host: "example.com".to_string(),
            user: "testuser".to_string(),
            port: 22,
            identity_file: None,
        };

        let mut session = SshSession::new("test", config);
        session.set_env("TEST_VAR", "test_value");

        let env = session.get_env();
        assert_eq!(env.get("TEST_VAR").unwrap(), "test_value");
    }

    #[test]
    fn test_set_cwd() {
        let config = SshConfig {
            host: "example.com".to_string(),
            user: "testuser".to_string(),
            port: 22,
            identity_file: None,
        };

        let mut session = SshSession::new("test", config);
        session.set_cwd("/tmp").unwrap();
        assert_eq!(session.get_cwd(), "/tmp");
    }
}
