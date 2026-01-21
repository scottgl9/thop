package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the thop configuration
type Config struct {
	Settings Settings           `toml:"settings"`
	Sessions map[string]Session `toml:"sessions"`
}

// Settings contains global settings
type Settings struct {
	DefaultSession    string `toml:"default_session"`
	CommandTimeout    int    `toml:"command_timeout"`
	ReconnectAttempts int    `toml:"reconnect_attempts"`
	ReconnectBackoff  int    `toml:"reconnect_backoff_base"`
	LogLevel          string `toml:"log_level"`
	StateFile         string `toml:"state_file"`
}

// Session represents a session configuration
type Session struct {
	Type            string   `toml:"type"` // "local" or "ssh"
	Shell           string   `toml:"shell,omitempty"`
	Host            string   `toml:"host,omitempty"`
	User            string   `toml:"user,omitempty"`
	Port            int      `toml:"port,omitempty"`
	IdentityFile    string   `toml:"identity_file,omitempty"`
	JumpHost        string   `toml:"jump_host,omitempty"`
	AgentForwarding bool     `toml:"agent_forwarding,omitempty"`
	PasswordEnv     string   `toml:"password_env,omitempty"`  // Environment variable containing password
	PasswordFile    string   `toml:"password_file,omitempty"` // File containing password (must be 0600)
	StartupCommands []string `toml:"startup_commands,omitempty"`
	CommandTimeout  int      `toml:"command_timeout,omitempty"` // Command timeout in seconds (overrides global default)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Settings: Settings{
			DefaultSession:    "local",
			CommandTimeout:    300,
			ReconnectAttempts: 5,
			ReconnectBackoff:  2,
			LogLevel:          "info",
			StateFile:         defaultStateFile(),
		},
		Sessions: map[string]Session{
			"local": {
				Type:  "local",
				Shell: getDefaultShell(),
			},
		},
	}
}

// Load loads configuration from the default or specified path
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Apply environment overrides even without a config file
		cfg.applyEnvOverrides()
		return cfg, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure local session exists
	if _, ok := cfg.Sessions["local"]; !ok {
		cfg.Sessions["local"] = Session{
			Type:  "local",
			Shell: getDefaultShell(),
		}
	}

	// Apply environment overrides
	cfg.applyEnvOverrides()

	return cfg, nil
}

// DefaultConfigPath returns the default config file path
func DefaultConfigPath() string {
	if path := os.Getenv("THOP_CONFIG"); path != "" {
		return path
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, "thop", "config.toml")
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if val := os.Getenv("THOP_STATE_FILE"); val != "" {
		c.Settings.StateFile = val
	}
	if val := os.Getenv("THOP_LOG_LEVEL"); val != "" {
		c.Settings.LogLevel = val
	}
	if val := os.Getenv("THOP_DEFAULT_SESSION"); val != "" {
		c.Settings.DefaultSession = val
	}
}

// GetSession returns a session by name
func (c *Config) GetSession(name string) (*Session, bool) {
	session, ok := c.Sessions[name]
	if !ok {
		return nil, false
	}
	return &session, true
}

// SessionNames returns all configured session names
func (c *Config) SessionNames() []string {
	names := make([]string, 0, len(c.Sessions))
	for name := range c.Sessions {
		names = append(names, name)
	}
	return names
}

// GetTimeout returns the command timeout for a session (session-specific or global default)
func (c *Config) GetTimeout(sessionName string) int {
	if session, ok := c.Sessions[sessionName]; ok && session.CommandTimeout > 0 {
		return session.CommandTimeout
	}
	if c.Settings.CommandTimeout > 0 {
		return c.Settings.CommandTimeout
	}
	return 300 // Default 5 minutes
}

// AddSession adds a new session to the config
func (c *Config) AddSession(name string, session Session) error {
	if _, exists := c.Sessions[name]; exists {
		return fmt.Errorf("session '%s' already exists", name)
	}
	c.Sessions[name] = session
	return nil
}

// Save saves the configuration to the specified path
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Ensure config directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to TOML
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func defaultStateFile() string {
	if val := os.Getenv("THOP_STATE_FILE"); val != "" {
		return val
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataDir, "thop", "state.json")
}

func getDefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
