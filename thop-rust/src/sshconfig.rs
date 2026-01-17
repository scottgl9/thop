//! SSH config file parser (~/.ssh/config)

use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;

/// Parsed SSH config entry
#[derive(Debug, Clone, Default)]
pub struct SshConfigEntry {
    pub hostname: Option<String>,
    pub user: Option<String>,
    pub port: Option<u16>,
    pub identity_file: Option<String>,
    pub proxy_jump: Option<String>,
    pub forward_agent: bool,
}

/// SSH config parser
pub struct SshConfigParser {
    entries: HashMap<String, SshConfigEntry>,
}

impl SshConfigParser {
    /// Create a new parser and load the default config file
    pub fn new() -> Self {
        let mut parser = Self {
            entries: HashMap::new(),
        };
        parser.load_default();
        parser
    }

    /// Load the default ~/.ssh/config file
    fn load_default(&mut self) {
        if let Some(home) = dirs::home_dir() {
            let config_path = home.join(".ssh/config");
            if config_path.exists() {
                self.load_file(&config_path);
            }
        }
    }

    /// Load and parse an SSH config file
    pub fn load_file(&mut self, path: &PathBuf) {
        if let Ok(content) = fs::read_to_string(path) {
            self.parse(&content);
        }
    }

    /// Parse SSH config content
    fn parse(&mut self, content: &str) {
        let mut current_host: Option<String> = None;
        let mut current_entry = SshConfigEntry::default();

        for line in content.lines() {
            let line = line.trim();

            // Skip comments and empty lines
            if line.is_empty() || line.starts_with('#') {
                continue;
            }

            // Split into keyword and value
            let parts: Vec<&str> = line.splitn(2, char::is_whitespace).collect();
            if parts.len() < 2 {
                continue;
            }

            let keyword = parts[0].to_lowercase();
            let value = parts[1].trim().trim_matches('"');

            match keyword.as_str() {
                "host" => {
                    // Save previous entry if exists
                    if let Some(host) = current_host.take() {
                        self.entries.insert(host, current_entry);
                    }
                    current_host = Some(value.to_string());
                    current_entry = SshConfigEntry::default();
                }
                "hostname" => {
                    current_entry.hostname = Some(value.to_string());
                }
                "user" => {
                    current_entry.user = Some(value.to_string());
                }
                "port" => {
                    if let Ok(port) = value.parse() {
                        current_entry.port = Some(port);
                    }
                }
                "identityfile" => {
                    // Expand ~ to home directory
                    let expanded = if value.starts_with("~/") {
                        dirs::home_dir()
                            .map(|h| h.join(&value[2..]).to_string_lossy().to_string())
                            .unwrap_or_else(|| value.to_string())
                    } else {
                        value.to_string()
                    };
                    current_entry.identity_file = Some(expanded);
                }
                "proxyjump" => {
                    current_entry.proxy_jump = Some(value.to_string());
                }
                "forwardagent" => {
                    current_entry.forward_agent = value.to_lowercase() == "yes";
                }
                _ => {}
            }
        }

        // Save last entry
        if let Some(host) = current_host {
            self.entries.insert(host, current_entry);
        }
    }

    /// Get config entry for a host
    pub fn get(&self, host: &str) -> Option<&SshConfigEntry> {
        self.entries.get(host)
    }

    /// Resolve hostname for a host alias
    pub fn resolve_hostname(&self, host: &str) -> String {
        self.entries
            .get(host)
            .and_then(|e| e.hostname.clone())
            .unwrap_or_else(|| host.to_string())
    }

    /// Resolve user for a host
    pub fn resolve_user(&self, host: &str) -> Option<String> {
        self.entries.get(host).and_then(|e| e.user.clone())
    }

    /// Resolve port for a host
    pub fn resolve_port(&self, host: &str) -> u16 {
        self.entries
            .get(host)
            .and_then(|e| e.port)
            .unwrap_or(22)
    }

    /// Resolve identity file for a host
    pub fn resolve_identity_file(&self, host: &str) -> Option<String> {
        self.entries.get(host).and_then(|e| e.identity_file.clone())
    }

    /// Resolve proxy jump for a host
    pub fn resolve_proxy_jump(&self, host: &str) -> Option<String> {
        self.entries.get(host).and_then(|e| e.proxy_jump.clone())
    }

    /// Check if forward agent is enabled for a host
    pub fn forward_agent(&self, host: &str) -> bool {
        self.entries
            .get(host)
            .map(|e| e.forward_agent)
            .unwrap_or(false)
    }
}

impl Default for SshConfigParser {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_basic() {
        let mut parser = SshConfigParser {
            entries: HashMap::new(),
        };

        let config = r#"
Host myserver
    HostName example.com
    User deploy
    Port 2222

Host prod
    HostName production.example.com
    User admin
    IdentityFile ~/.ssh/prod_key
    ForwardAgent yes
"#;

        parser.parse(config);

        // Check myserver
        let entry = parser.get("myserver").unwrap();
        assert_eq!(entry.hostname.as_deref(), Some("example.com"));
        assert_eq!(entry.user.as_deref(), Some("deploy"));
        assert_eq!(entry.port, Some(2222));

        // Check prod
        let entry = parser.get("prod").unwrap();
        assert_eq!(entry.hostname.as_deref(), Some("production.example.com"));
        assert_eq!(entry.user.as_deref(), Some("admin"));
        assert!(entry.forward_agent);
    }

    #[test]
    fn test_resolve_hostname() {
        let mut parser = SshConfigParser {
            entries: HashMap::new(),
        };

        let config = r#"
Host myalias
    HostName real.server.com
"#;

        parser.parse(config);

        assert_eq!(parser.resolve_hostname("myalias"), "real.server.com");
        assert_eq!(parser.resolve_hostname("unknown"), "unknown");
    }

    #[test]
    fn test_resolve_port() {
        let mut parser = SshConfigParser {
            entries: HashMap::new(),
        };

        let config = r#"
Host custom
    Port 3333
"#;

        parser.parse(config);

        assert_eq!(parser.resolve_port("custom"), 3333);
        assert_eq!(parser.resolve_port("unknown"), 22);
    }

    #[test]
    fn test_proxy_jump() {
        let mut parser = SshConfigParser {
            entries: HashMap::new(),
        };

        let config = r#"
Host internal
    HostName internal.server.com
    ProxyJump bastion.example.com
"#;

        parser.parse(config);

        let entry = parser.get("internal").unwrap();
        assert_eq!(entry.proxy_jump.as_deref(), Some("bastion.example.com"));
    }
}
