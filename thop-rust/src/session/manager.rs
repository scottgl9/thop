use std::collections::HashMap;

use serde::Serialize;

use crate::config::Config;
use crate::error::{Result, SessionError};
use crate::state::Manager as StateManager;
use super::{ExecuteResult, LocalSession, Session, SshConfig, SshSession};

/// Session info for listing
#[derive(Debug, Clone, Serialize)]
pub struct SessionInfo {
    pub name: String,
    #[serde(rename = "type")]
    pub session_type: String,
    pub connected: bool,
    pub active: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub host: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user: Option<String>,
    pub cwd: String,
}

/// Session manager
pub struct Manager {
    sessions: HashMap<String, Box<dyn Session>>,
    active_session: String,
    state_manager: Option<StateManager>,
}

impl Manager {
    /// Create a new session manager from config
    pub fn new(config: &Config, state_manager: Option<StateManager>) -> Self {
        let mut sessions: HashMap<String, Box<dyn Session>> = HashMap::new();

        // Create sessions from config
        for (name, session_config) in &config.sessions {
            let session: Box<dyn Session> = match session_config.session_type.as_str() {
                "local" => Box::new(LocalSession::new(
                    name.clone(),
                    session_config.shell.clone(),
                )),
                "ssh" => {
                    let ssh_config = SshConfig {
                        host: session_config.host.clone().unwrap_or_default(),
                        user: session_config.user.clone().unwrap_or_else(|| {
                            std::env::var("USER").unwrap_or_else(|_| "root".to_string())
                        }),
                        port: session_config.port.unwrap_or(22),
                        identity_file: session_config.identity_file.clone(),
                    };
                    Box::new(SshSession::new(name.clone(), ssh_config))
                }
                _ => continue,
            };
            sessions.insert(name.clone(), session);
        }

        // Get active session from state or config default
        let active_session = state_manager
            .as_ref()
            .map(|s| s.get_active_session())
            .unwrap_or_else(|| config.settings.default_session.clone());

        Self {
            sessions,
            active_session,
            state_manager,
        }
    }

    /// Check if a session exists
    pub fn has_session(&self, name: &str) -> bool {
        self.sessions.contains_key(name)
    }

    /// Get a session by name
    pub fn get_session(&self, name: &str) -> Option<&dyn Session> {
        self.sessions.get(name).map(|s| s.as_ref())
    }

    /// Get a mutable session by name
    pub fn get_session_mut(&mut self, name: &str) -> Option<&mut Box<dyn Session>> {
        self.sessions.get_mut(name)
    }

    /// Get the active session
    pub fn get_active_session(&self) -> Option<&dyn Session> {
        self.sessions.get(&self.active_session).map(|s| s.as_ref())
    }

    /// Get the active session name
    pub fn get_active_session_name(&self) -> &str {
        &self.active_session
    }

    /// Set the active session
    pub fn set_active_session(&mut self, name: &str) -> Result<()> {
        if !self.sessions.contains_key(name) {
            return Err(SessionError::session_not_found(name).into());
        }

        self.active_session = name.to_string();

        // Persist to state
        if let Some(ref state_manager) = self.state_manager {
            state_manager.set_active_session(name)?;
        }

        Ok(())
    }

    /// Execute a command on the active session
    pub fn execute(&mut self, cmd: &str) -> Result<ExecuteResult> {
        let session = self.sessions.get_mut(&self.active_session).ok_or_else(|| {
            SessionError::session_not_found(&self.active_session)
        })?;

        session.execute(cmd)
    }

    /// Execute a command on a specific session
    pub fn execute_on(&mut self, name: &str, cmd: &str) -> Result<ExecuteResult> {
        let session = self.sessions.get_mut(name).ok_or_else(|| {
            SessionError::session_not_found(name)
        })?;

        session.execute(cmd)
    }

    /// Connect a session
    pub fn connect(&mut self, name: &str) -> Result<()> {
        let session = self.sessions.get_mut(name).ok_or_else(|| {
            SessionError::session_not_found(name)
        })?;

        session.connect()?;

        // Update state
        if let Some(ref state_manager) = self.state_manager {
            state_manager.set_session_connected(name, true)?;
        }

        Ok(())
    }

    /// Disconnect a session
    pub fn disconnect(&mut self, name: &str) -> Result<()> {
        let session = self.sessions.get_mut(name).ok_or_else(|| {
            SessionError::session_not_found(name)
        })?;

        session.disconnect()?;

        // Update state
        if let Some(ref state_manager) = self.state_manager {
            state_manager.set_session_connected(name, false)?;
        }

        Ok(())
    }

    /// List all sessions with their info
    pub fn list_sessions(&self) -> Vec<SessionInfo> {
        self.sessions
            .iter()
            .map(|(name, session)| {
                let (host, user) = if session.session_type() == "ssh" {
                    // Try to get host/user from config - we don't have direct access here
                    // In a real implementation, we'd store this info or get it from the session
                    (None, None)
                } else {
                    (None, None)
                };

                SessionInfo {
                    name: name.clone(),
                    session_type: session.session_type().to_string(),
                    connected: session.is_connected(),
                    active: name == &self.active_session,
                    host,
                    user,
                    cwd: session.get_cwd().to_string(),
                }
            })
            .collect()
    }

    /// Get session names
    pub fn session_names(&self) -> Vec<&str> {
        self.sessions.keys().map(|s| s.as_str()).collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{Config, Session as ConfigSession, Settings};

    fn create_test_config() -> Config {
        let mut sessions = HashMap::new();
        sessions.insert(
            "local".to_string(),
            ConfigSession {
                session_type: "local".to_string(),
                shell: Some("/bin/sh".to_string()),
                host: None,
                user: None,
                port: None,
                identity_file: None,
                jump_host: None,
                startup_commands: vec![],
            },
        );
        sessions.insert(
            "testserver".to_string(),
            ConfigSession {
                session_type: "ssh".to_string(),
                shell: None,
                host: Some("example.com".to_string()),
                user: Some("testuser".to_string()),
                port: Some(22),
                identity_file: None,
                jump_host: None,
                startup_commands: vec![],
            },
        );

        Config {
            settings: Settings {
                default_session: "local".to_string(),
                ..Settings::default()
            },
            sessions,
        }
    }

    #[test]
    fn test_new_manager() {
        let config = create_test_config();
        let mgr = Manager::new(&config, None);

        assert!(mgr.has_session("local"));
        assert!(mgr.has_session("testserver"));
        assert_eq!(mgr.get_active_session_name(), "local");
    }

    #[test]
    fn test_get_session() {
        let config = create_test_config();
        let mgr = Manager::new(&config, None);

        let session = mgr.get_session("local");
        assert!(session.is_some());
        assert_eq!(session.unwrap().session_type(), "local");

        assert!(mgr.get_session("nonexistent").is_none());
    }

    #[test]
    fn test_set_active_session() {
        let config = create_test_config();
        let mut mgr = Manager::new(&config, None);

        mgr.set_active_session("testserver").unwrap();
        assert_eq!(mgr.get_active_session_name(), "testserver");

        let result = mgr.set_active_session("nonexistent");
        assert!(result.is_err());
    }

    #[test]
    fn test_execute() {
        let config = create_test_config();
        let mut mgr = Manager::new(&config, None);

        let result = mgr.execute("echo hello").unwrap();
        assert_eq!(result.stdout.trim(), "hello");
        assert_eq!(result.exit_code, 0);
    }

    #[test]
    fn test_execute_on() {
        let config = create_test_config();
        let mut mgr = Manager::new(&config, None);

        let result = mgr.execute_on("local", "echo test").unwrap();
        assert_eq!(result.stdout.trim(), "test");

        let result = mgr.execute_on("nonexistent", "echo test");
        assert!(result.is_err());
    }

    #[test]
    fn test_list_sessions() {
        let config = create_test_config();
        let mgr = Manager::new(&config, None);

        let sessions = mgr.list_sessions();
        assert_eq!(sessions.len(), 2);

        let local = sessions.iter().find(|s| s.name == "local").unwrap();
        assert_eq!(local.session_type, "local");
        assert!(local.active);
    }

    #[test]
    fn test_session_names() {
        let config = create_test_config();
        let mgr = Manager::new(&config, None);

        let names = mgr.session_names();
        assert_eq!(names.len(), 2);
        assert!(names.contains(&"local"));
        assert!(names.contains(&"testserver"));
    }

    #[test]
    fn test_connect_disconnect_local() {
        let config = create_test_config();
        let mut mgr = Manager::new(&config, None);

        // Local connect/disconnect should be no-ops
        mgr.connect("local").unwrap();
        mgr.disconnect("local").unwrap();

        // Non-existent session
        assert!(mgr.connect("nonexistent").is_err());
        assert!(mgr.disconnect("nonexistent").is_err());
    }
}
