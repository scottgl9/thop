use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::env;
use std::fs;
use std::path::PathBuf;

use crate::error::{Result, ThopError};

/// Main configuration structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub settings: Settings,
    #[serde(default)]
    pub sessions: HashMap<String, Session>,
}

/// Global settings
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Settings {
    #[serde(default = "default_session")]
    pub default_session: String,
    #[serde(default = "default_command_timeout")]
    pub command_timeout: u32,
    #[serde(default = "default_reconnect_attempts")]
    pub reconnect_attempts: u32,
    #[serde(default = "default_reconnect_backoff")]
    pub reconnect_backoff_base: u32,
    #[serde(default = "default_log_level")]
    pub log_level: String,
    #[serde(default = "default_state_file")]
    pub state_file: String,
}

/// Session configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    #[serde(rename = "type")]
    pub session_type: String,
    #[serde(default)]
    pub shell: Option<String>,
    #[serde(default)]
    pub host: Option<String>,
    #[serde(default)]
    pub user: Option<String>,
    #[serde(default)]
    pub port: Option<u16>,
    #[serde(default)]
    pub identity_file: Option<String>,
    #[serde(default)]
    pub jump_host: Option<String>,
    #[serde(default)]
    pub startup_commands: Vec<String>,
}

fn default_session() -> String {
    "local".to_string()
}

fn default_command_timeout() -> u32 {
    300
}

fn default_reconnect_attempts() -> u32 {
    5
}

fn default_reconnect_backoff() -> u32 {
    2
}

fn default_log_level() -> String {
    "info".to_string()
}

fn default_state_file() -> String {
    if let Some(val) = env::var_os("THOP_STATE_FILE") {
        return val.to_string_lossy().to_string();
    }

    let data_dir = env::var("XDG_DATA_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| {
            dirs::home_dir()
                .unwrap_or_else(|| PathBuf::from("."))
                .join(".local/share")
        });

    data_dir
        .join("thop/state.json")
        .to_string_lossy()
        .to_string()
}

fn default_shell() -> String {
    env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string())
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            default_session: default_session(),
            command_timeout: default_command_timeout(),
            reconnect_attempts: default_reconnect_attempts(),
            reconnect_backoff_base: default_reconnect_backoff(),
            log_level: default_log_level(),
            state_file: default_state_file(),
        }
    }
}

impl Default for Config {
    fn default() -> Self {
        let mut sessions = HashMap::new();
        sessions.insert(
            "local".to_string(),
            Session {
                session_type: "local".to_string(),
                shell: Some(default_shell()),
                host: None,
                user: None,
                port: None,
                identity_file: None,
                jump_host: None,
                startup_commands: vec![],
            },
        );

        Self {
            settings: Settings::default(),
            sessions,
        }
    }
}

impl Config {
    /// Load configuration from file or return defaults
    pub fn load(path: Option<&str>) -> Result<Self> {
        let path = path.map(PathBuf::from).unwrap_or_else(default_config_path);

        let mut config = if path.exists() {
            let content = fs::read_to_string(&path).map_err(|e| {
                ThopError::Config(format!("Failed to read config file: {}", e))
            })?;
            toml::from_str(&content)
                .map_err(|e| ThopError::Config(format!("Failed to parse config file: {}", e)))?
        } else {
            Config::default()
        };

        // Ensure local session exists
        if !config.sessions.contains_key("local") {
            config.sessions.insert(
                "local".to_string(),
                Session {
                    session_type: "local".to_string(),
                    shell: Some(default_shell()),
                    host: None,
                    user: None,
                    port: None,
                    identity_file: None,
                    jump_host: None,
                    startup_commands: vec![],
                },
            );
        }

        // Apply environment overrides
        config.apply_env_overrides();

        Ok(config)
    }

    /// Apply environment variable overrides
    fn apply_env_overrides(&mut self) {
        if let Ok(val) = env::var("THOP_STATE_FILE") {
            self.settings.state_file = val;
        }
        if let Ok(val) = env::var("THOP_LOG_LEVEL") {
            self.settings.log_level = val;
        }
        if let Ok(val) = env::var("THOP_DEFAULT_SESSION") {
            self.settings.default_session = val;
        }
    }

    /// Get a session by name
    pub fn get_session(&self, name: &str) -> Option<&Session> {
        self.sessions.get(name)
    }

    /// Get all session names
    pub fn session_names(&self) -> Vec<&str> {
        self.sessions.keys().map(|s| s.as_str()).collect()
    }
}

/// Get the default config file path
pub fn default_config_path() -> PathBuf {
    if let Ok(path) = env::var("THOP_CONFIG") {
        return PathBuf::from(path);
    }

    let config_dir = env::var("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|_| {
            dirs::home_dir()
                .unwrap_or_else(|| PathBuf::from("."))
                .join(".config")
        });

    config_dir.join("thop/config.toml")
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::TempDir;

    #[test]
    fn test_default_config() {
        let config = Config::default();
        assert_eq!(config.settings.default_session, "local");
        assert_eq!(config.settings.command_timeout, 300);
        assert!(config.sessions.contains_key("local"));
    }

    #[test]
    fn test_load_nonexistent_config() {
        let config = Config::load(Some("/nonexistent/path/config.toml")).unwrap();
        assert_eq!(config.settings.default_session, "local");
    }

    #[test]
    fn test_load_valid_config() {
        let tmp_dir = TempDir::new().unwrap();
        let config_path = tmp_dir.path().join("config.toml");

        let content = r#"
[settings]
default_session = "prod"
command_timeout = 600
log_level = "debug"

[sessions.local]
type = "local"
shell = "/bin/zsh"

[sessions.prod]
type = "ssh"
host = "prod.example.com"
user = "deploy"
port = 2222
"#;

        let mut file = fs::File::create(&config_path).unwrap();
        file.write_all(content.as_bytes()).unwrap();

        let config = Config::load(Some(config_path.to_str().unwrap())).unwrap();

        assert_eq!(config.settings.default_session, "prod");
        assert_eq!(config.settings.command_timeout, 600);
        assert_eq!(config.settings.log_level, "debug");

        let prod = config.sessions.get("prod").unwrap();
        assert_eq!(prod.session_type, "ssh");
        assert_eq!(prod.host.as_ref().unwrap(), "prod.example.com");
        assert_eq!(prod.user.as_ref().unwrap(), "deploy");
        assert_eq!(prod.port.unwrap(), 2222);
    }

    #[test]
    fn test_get_session() {
        let config = Config::default();
        assert!(config.get_session("local").is_some());
        assert!(config.get_session("nonexistent").is_none());
    }
}
