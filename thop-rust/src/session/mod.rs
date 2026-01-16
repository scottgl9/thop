mod local;
mod ssh;
mod manager;

pub use local::LocalSession;
pub use ssh::{SshConfig, SshSession};
pub use manager::{Manager, SessionInfo};

use crate::error::{Result, SessionError};
use serde::Serialize;

/// Result of command execution
#[derive(Debug, Clone, Default, Serialize)]
pub struct ExecuteResult {
    pub stdout: String,
    pub stderr: String,
    pub exit_code: i32,
}

/// Session trait defining common operations
pub trait Session: Send {
    /// Get the session name
    fn name(&self) -> &str;

    /// Get the session type ("local" or "ssh")
    fn session_type(&self) -> &str;

    /// Check if session is connected
    fn is_connected(&self) -> bool;

    /// Connect the session
    fn connect(&mut self) -> Result<()>;

    /// Disconnect the session
    fn disconnect(&mut self) -> Result<()>;

    /// Execute a command
    fn execute(&mut self, cmd: &str) -> Result<ExecuteResult>;

    /// Get current working directory
    fn get_cwd(&self) -> &str;

    /// Set current working directory
    fn set_cwd(&mut self, path: &str) -> Result<()>;

    /// Get environment variables
    fn get_env(&self) -> std::collections::HashMap<String, String>;

    /// Set an environment variable
    fn set_env(&mut self, key: &str, value: &str);
}

/// Format a prompt with session name
pub fn format_prompt(session_name: &str) -> String {
    format!("({}) $ ", session_name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_prompt() {
        assert_eq!(format_prompt("local"), "(local) $ ");
        assert_eq!(format_prompt("prod"), "(prod) $ ");
    }
}
