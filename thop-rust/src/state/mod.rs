use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::sync::Mutex;

use crate::error::{Result, ThopError};

/// Per-session state
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionState {
    #[serde(rename = "type", default)]
    pub session_type: String,
    #[serde(default)]
    pub connected: bool,
    #[serde(default)]
    pub cwd: String,
    #[serde(default)]
    pub env: HashMap<String, String>,
}

/// Complete application state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct State {
    pub active_session: String,
    #[serde(default)]
    pub sessions: HashMap<String, SessionState>,
    pub updated_at: DateTime<Utc>,
}

impl Default for State {
    fn default() -> Self {
        Self {
            active_session: "local".to_string(),
            sessions: HashMap::new(),
            updated_at: Utc::now(),
        }
    }
}

/// State manager for loading and saving state
pub struct Manager {
    path: PathBuf,
    state: Mutex<State>,
}

impl Manager {
    /// Create a new state manager
    pub fn new(path: impl Into<PathBuf>) -> Self {
        Self {
            path: path.into(),
            state: Mutex::new(State::default()),
        }
    }

    /// Load state from file
    pub fn load(&self) -> Result<()> {
        // Create parent directory if needed
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)?;
        }

        // Check if file exists
        if !self.path.exists() {
            // Create with defaults
            self.save()?;
            return Ok(());
        }

        // Read and parse file
        let content = fs::read_to_string(&self.path)?;
        let loaded_state: State = serde_json::from_str(&content)
            .map_err(|e| ThopError::State(format!("Failed to parse state file: {}", e)))?;

        let mut state = self.state.lock().unwrap();
        *state = loaded_state;

        Ok(())
    }

    /// Save state to file
    pub fn save(&self) -> Result<()> {
        // Create parent directory if needed
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)?;
        }

        let mut state = self.state.lock().unwrap();
        state.updated_at = Utc::now();

        let content = serde_json::to_string_pretty(&*state)
            .map_err(|e| ThopError::State(format!("Failed to serialize state: {}", e)))?;

        // Write atomically using temp file
        let temp_path = self.path.with_extension("tmp");
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .set_mode(0o600)
            .open(&temp_path)?;

        file.write_all(content.as_bytes())?;
        file.sync_all()?;
        drop(file);

        fs::rename(&temp_path, &self.path)?;

        Ok(())
    }

    /// Get the active session name
    pub fn get_active_session(&self) -> String {
        self.state.lock().unwrap().active_session.clone()
    }

    /// Set the active session
    pub fn set_active_session(&self, name: impl Into<String>) -> Result<()> {
        {
            let mut state = self.state.lock().unwrap();
            state.active_session = name.into();
        }
        self.save()
    }

    /// Get session state
    pub fn get_session_state(&self, name: &str) -> Option<SessionState> {
        self.state.lock().unwrap().sessions.get(name).cloned()
    }

    /// Update session state
    pub fn update_session_state(&self, name: impl Into<String>, session_state: SessionState) -> Result<()> {
        {
            let mut state = self.state.lock().unwrap();
            state.sessions.insert(name.into(), session_state);
        }
        self.save()
    }

    /// Set session connected status
    pub fn set_session_connected(&self, name: &str, connected: bool) -> Result<()> {
        {
            let mut state = self.state.lock().unwrap();
            let session = state.sessions.entry(name.to_string()).or_default();
            session.connected = connected;
        }
        self.save()
    }

    /// Set session CWD
    pub fn set_session_cwd(&self, name: &str, cwd: impl Into<String>) -> Result<()> {
        {
            let mut state = self.state.lock().unwrap();
            let session = state.sessions.entry(name.to_string()).or_default();
            session.cwd = cwd.into();
        }
        self.save()
    }

    /// Get all sessions
    pub fn get_all_sessions(&self) -> HashMap<String, SessionState> {
        self.state.lock().unwrap().sessions.clone()
    }
}

// Helper trait for setting file mode
trait FileMode {
    fn set_mode(&mut self, mode: u32) -> &mut Self;
}

impl FileMode for OpenOptions {
    #[cfg(unix)]
    fn set_mode(&mut self, mode: u32) -> &mut Self {
        use std::os::unix::fs::OpenOptionsExt;
        OpenOptionsExt::mode(self, mode)
    }

    #[cfg(not(unix))]
    fn set_mode(&mut self, _mode: u32) -> &mut Self {
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_new_manager() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("state.json");

        let mgr = Manager::new(&state_path);
        assert_eq!(mgr.get_active_session(), "local");
    }

    #[test]
    fn test_load_and_save() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("subdir/state.json");

        let mgr = Manager::new(&state_path);
        mgr.load().unwrap();

        // File should exist now
        assert!(state_path.exists());
    }

    #[test]
    fn test_set_and_get_active_session() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("state.json");

        let mgr = Manager::new(&state_path);
        mgr.load().unwrap();

        mgr.set_active_session("prod").unwrap();
        assert_eq!(mgr.get_active_session(), "prod");

        // Create new manager to verify persistence
        let mgr2 = Manager::new(&state_path);
        mgr2.load().unwrap();
        assert_eq!(mgr2.get_active_session(), "prod");
    }

    #[test]
    fn test_session_state() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("state.json");

        let mgr = Manager::new(&state_path);
        mgr.load().unwrap();

        let mut env = HashMap::new();
        env.insert("RAILS_ENV".to_string(), "production".to_string());

        let session_state = SessionState {
            session_type: "ssh".to_string(),
            connected: true,
            cwd: "/var/www".to_string(),
            env,
        };

        mgr.update_session_state("prod", session_state).unwrap();

        let retrieved = mgr.get_session_state("prod").unwrap();
        assert_eq!(retrieved.session_type, "ssh");
        assert!(retrieved.connected);
        assert_eq!(retrieved.cwd, "/var/www");
        assert_eq!(retrieved.env.get("RAILS_ENV").unwrap(), "production");

        // Non-existent session
        assert!(mgr.get_session_state("nonexistent").is_none());
    }

    #[test]
    fn test_set_session_connected() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("state.json");

        let mgr = Manager::new(&state_path);
        mgr.load().unwrap();

        mgr.set_session_connected("test", true).unwrap();
        let state = mgr.get_session_state("test").unwrap();
        assert!(state.connected);

        mgr.set_session_connected("test", false).unwrap();
        let state = mgr.get_session_state("test").unwrap();
        assert!(!state.connected);
    }

    #[test]
    fn test_set_session_cwd() {
        let tmp_dir = TempDir::new().unwrap();
        let state_path = tmp_dir.path().join("state.json");

        let mgr = Manager::new(&state_path);
        mgr.load().unwrap();

        mgr.set_session_cwd("test", "/home/user").unwrap();
        let state = mgr.get_session_state("test").unwrap();
        assert_eq!(state.cwd, "/home/user");

        mgr.set_session_cwd("test", "/tmp").unwrap();
        let state = mgr.get_session_state("test").unwrap();
        assert_eq!(state.cwd, "/tmp");
    }
}
