use std::collections::HashMap;
use std::env;
use std::path::PathBuf;
use std::process::Command;

use crate::error::Result;
use super::{ExecuteResult, Session};

/// Local shell session
pub struct LocalSession {
    name: String,
    shell: String,
    cwd: String,
    env: HashMap<String, String>,
    connected: bool,
}

impl LocalSession {
    /// Create a new local session
    pub fn new(name: impl Into<String>, shell: Option<String>) -> Self {
        let shell = shell.unwrap_or_else(|| {
            env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string())
        });

        let cwd = env::current_dir()
            .map(|p| p.to_string_lossy().to_string())
            .unwrap_or_else(|_| {
                dirs::home_dir()
                    .map(|p| p.to_string_lossy().to_string())
                    .unwrap_or_else(|| "/".to_string())
            });

        Self {
            name: name.into(),
            shell,
            cwd,
            env: HashMap::new(),
            connected: true, // Local is always "connected"
        }
    }

    /// Handle cd commands specially to track cwd
    fn handle_cd(&mut self, cmd: &str) -> Result<ExecuteResult> {
        let parts: Vec<&str> = cmd.split_whitespace().collect();

        let target_dir = if parts.len() == 1 {
            // cd with no args goes to home
            match dirs::home_dir() {
                Some(p) => p.to_string_lossy().to_string(),
                None => {
                    return Ok(ExecuteResult {
                        stderr: "cd: HOME not set\n".to_string(),
                        exit_code: 1,
                        ..Default::default()
                    });
                }
            }
        } else {
            let target = parts[1];

            // Handle ~ expansion
            let expanded = if target.starts_with('~') {
                let home = dirs::home_dir()
                    .map(|p| p.to_string_lossy().to_string())
                    .unwrap_or_else(|| "~".to_string());
                target.replacen('~', &home, 1)
            } else {
                target.to_string()
            };

            // Handle relative paths
            if expanded.starts_with('/') {
                expanded
            } else {
                format!("{}/{}", self.cwd, expanded)
            }
        };

        // Check if directory exists
        let path = PathBuf::from(&target_dir);
        if !path.exists() {
            return Ok(ExecuteResult {
                stderr: format!("cd: {}: No such file or directory\n", target_dir),
                exit_code: 1,
                ..Default::default()
            });
        }

        if !path.is_dir() {
            return Ok(ExecuteResult {
                stderr: format!("cd: {}: Not a directory\n", target_dir),
                exit_code: 1,
                ..Default::default()
            });
        }

        // Get the canonical path
        let output = Command::new(&self.shell)
            .arg("-c")
            .arg(format!("cd {} && pwd", target_dir))
            .output();

        match output {
            Ok(output) if output.status.success() => {
                self.cwd = String::from_utf8_lossy(&output.stdout).trim().to_string();
                Ok(ExecuteResult::default())
            }
            Ok(output) => Ok(ExecuteResult {
                stderr: String::from_utf8_lossy(&output.stderr).to_string(),
                exit_code: output.status.code().unwrap_or(1),
                ..Default::default()
            }),
            Err(e) => Ok(ExecuteResult {
                stderr: format!("cd: {}: {}\n", target_dir, e),
                exit_code: 1,
                ..Default::default()
            }),
        }
    }

    /// Set the shell to use
    pub fn set_shell(&mut self, shell: impl Into<String>) {
        self.shell = shell.into();
    }
}

impl Session for LocalSession {
    fn name(&self) -> &str {
        &self.name
    }

    fn session_type(&self) -> &str {
        "local"
    }

    fn is_connected(&self) -> bool {
        self.connected
    }

    fn connect(&mut self) -> Result<()> {
        self.connected = true;
        Ok(())
    }

    fn disconnect(&mut self) -> Result<()> {
        self.connected = false;
        Ok(())
    }

    fn execute(&mut self, cmd: &str) -> Result<ExecuteResult> {
        let trimmed = cmd.trim();

        // Handle cd commands specially
        if trimmed == "cd" || trimmed.starts_with("cd ") {
            return self.handle_cd(cmd);
        }

        // Execute command via shell
        let mut command = Command::new(&self.shell);
        command.arg("-c").arg(cmd).current_dir(&self.cwd);

        // Set environment
        for (key, value) in &self.env {
            command.env(key, value);
        }

        let output = command.output()?;

        Ok(ExecuteResult {
            stdout: String::from_utf8_lossy(&output.stdout).to_string(),
            stderr: String::from_utf8_lossy(&output.stderr).to_string(),
            exit_code: output.status.code().unwrap_or(-1),
        })
    }

    fn get_cwd(&self) -> &str {
        &self.cwd
    }

    fn set_cwd(&mut self, path: &str) -> Result<()> {
        let path_buf = PathBuf::from(path);
        if !path_buf.exists() || !path_buf.is_dir() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "Directory not found",
            )
            .into());
        }
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
    fn test_new_local_session() {
        let session = LocalSession::new("test", Some("/bin/bash".to_string()));
        assert_eq!(session.name(), "test");
        assert_eq!(session.session_type(), "local");
        assert!(session.is_connected());
        assert!(!session.get_cwd().is_empty());
    }

    #[test]
    fn test_default_shell() {
        let session = LocalSession::new("test", None);
        // Should use SHELL env or /bin/sh
        assert!(!session.shell.is_empty());
    }

    #[test]
    fn test_connect_disconnect() {
        let mut session = LocalSession::new("test", None);

        session.disconnect().unwrap();
        assert!(!session.is_connected());

        session.connect().unwrap();
        assert!(session.is_connected());
    }

    #[test]
    fn test_execute() {
        let mut session = LocalSession::new("test", None);

        let result = session.execute("echo hello").unwrap();
        assert_eq!(result.stdout.trim(), "hello");
        assert_eq!(result.exit_code, 0);
    }

    #[test]
    fn test_execute_failing_command() {
        let mut session = LocalSession::new("test", None);

        let result = session.execute("exit 42").unwrap();
        assert_eq!(result.exit_code, 42);
    }

    #[test]
    fn test_execute_stderr() {
        let mut session = LocalSession::new("test", None);

        let result = session.execute("echo error >&2").unwrap();
        assert!(result.stderr.contains("error"));
    }

    #[test]
    fn test_cd() {
        let mut session = LocalSession::new("test", None);
        let original_cwd = session.get_cwd().to_string();

        // cd to /tmp
        let result = session.execute("cd /tmp").unwrap();
        assert_eq!(result.exit_code, 0);
        assert_eq!(session.get_cwd(), "/tmp");

        // pwd should return /tmp
        let result = session.execute("pwd").unwrap();
        assert_eq!(result.stdout.trim(), "/tmp");

        // cd with no args goes to home
        session.execute("cd").unwrap();
        let home = dirs::home_dir().unwrap().to_string_lossy().to_string();
        assert_eq!(session.get_cwd(), home);

        // Restore
        session.execute(&format!("cd {}", original_cwd)).ok();
    }

    #[test]
    fn test_cd_nonexistent() {
        let mut session = LocalSession::new("test", None);
        let original_cwd = session.get_cwd().to_string();

        let result = session.execute("cd /nonexistent_path_12345").unwrap();
        assert_ne!(result.exit_code, 0);
        assert!(result.stderr.contains("No such file"));
        assert_eq!(session.get_cwd(), original_cwd);
    }

    #[test]
    fn test_env() {
        let mut session = LocalSession::new("test", None);

        session.set_env("TEST_VAR", "test_value");
        let env = session.get_env();
        assert_eq!(env.get("TEST_VAR").unwrap(), "test_value");

        let result = session.execute("echo $TEST_VAR").unwrap();
        assert_eq!(result.stdout.trim(), "test_value");
    }

    #[test]
    fn test_set_cwd() {
        let mut session = LocalSession::new("test", None);

        session.set_cwd("/tmp").unwrap();
        assert_eq!(session.get_cwd(), "/tmp");

        let err = session.set_cwd("/nonexistent_12345");
        assert!(err.is_err());
    }
}
