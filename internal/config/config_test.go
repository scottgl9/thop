package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check default settings
	if cfg.Settings.DefaultSession != "local" {
		t.Errorf("expected default_session 'local', got '%s'", cfg.Settings.DefaultSession)
	}

	if cfg.Settings.CommandTimeout != 300 {
		t.Errorf("expected command_timeout 300, got %d", cfg.Settings.CommandTimeout)
	}

	if cfg.Settings.ReconnectAttempts != 5 {
		t.Errorf("expected reconnect_attempts 5, got %d", cfg.Settings.ReconnectAttempts)
	}

	if cfg.Settings.LogLevel != "info" {
		t.Errorf("expected log_level 'info', got '%s'", cfg.Settings.LogLevel)
	}

	// Check local session exists
	localSession, ok := cfg.Sessions["local"]
	if !ok {
		t.Fatal("expected 'local' session to exist")
	}

	if localSession.Type != "local" {
		t.Errorf("expected local session type 'local', got '%s'", localSession.Type)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Load from non-existent path should return defaults
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load should not error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load returned nil config")
	}

	// Should have defaults
	if cfg.Settings.DefaultSession != "local" {
		t.Errorf("expected default_session 'local', got '%s'", cfg.Settings.DefaultSession)
	}
}

func TestLoadValidConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
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
identity_file = "~/.ssh/prod_key"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check settings
	if cfg.Settings.DefaultSession != "prod" {
		t.Errorf("expected default_session 'prod', got '%s'", cfg.Settings.DefaultSession)
	}

	if cfg.Settings.CommandTimeout != 600 {
		t.Errorf("expected command_timeout 600, got %d", cfg.Settings.CommandTimeout)
	}

	if cfg.Settings.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got '%s'", cfg.Settings.LogLevel)
	}

	// Check local session
	localSession, ok := cfg.Sessions["local"]
	if !ok {
		t.Fatal("expected 'local' session")
	}

	if localSession.Shell != "/bin/zsh" {
		t.Errorf("expected shell '/bin/zsh', got '%s'", localSession.Shell)
	}

	// Check prod session
	prodSession, ok := cfg.Sessions["prod"]
	if !ok {
		t.Fatal("expected 'prod' session")
	}

	if prodSession.Type != "ssh" {
		t.Errorf("expected type 'ssh', got '%s'", prodSession.Type)
	}

	if prodSession.Host != "prod.example.com" {
		t.Errorf("expected host 'prod.example.com', got '%s'", prodSession.Host)
	}

	if prodSession.User != "deploy" {
		t.Errorf("expected user 'deploy', got '%s'", prodSession.User)
	}

	if prodSession.Port != 2222 {
		t.Errorf("expected port 2222, got %d", prodSession.Port)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Write invalid TOML
	if err := os.WriteFile(configPath, []byte("invalid toml {{{"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestGetSession(t *testing.T) {
	cfg := DefaultConfig()

	// Get existing session
	session, ok := cfg.GetSession("local")
	if !ok {
		t.Error("expected to find 'local' session")
	}
	if session.Type != "local" {
		t.Errorf("expected type 'local', got '%s'", session.Type)
	}

	// Get non-existing session
	_, ok = cfg.GetSession("nonexistent")
	if ok {
		t.Error("expected not to find 'nonexistent' session")
	}
}

func TestSessionNames(t *testing.T) {
	cfg := DefaultConfig()

	names := cfg.SessionNames()
	if len(names) == 0 {
		t.Error("expected at least one session name")
	}

	found := false
	for _, name := range names {
		if name == "local" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'local' in session names")
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("THOP_LOG_LEVEL", "trace")
	os.Setenv("THOP_DEFAULT_SESSION", "test")
	defer func() {
		os.Unsetenv("THOP_LOG_LEVEL")
		os.Unsetenv("THOP_DEFAULT_SESSION")
	}()

	cfg, err := Load("/nonexistent/path")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Settings.LogLevel != "trace" {
		t.Errorf("expected log_level 'trace' from env, got '%s'", cfg.Settings.LogLevel)
	}

	if cfg.Settings.DefaultSession != "test" {
		t.Errorf("expected default_session 'test' from env, got '%s'", cfg.Settings.DefaultSession)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	// Without env override
	os.Unsetenv("THOP_CONFIG")
	path := DefaultConfigPath()

	if path == "" {
		t.Error("DefaultConfigPath returned empty string")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got '%s'", path)
	}

	// With env override
	os.Setenv("THOP_CONFIG", "/custom/path/config.toml")
	defer os.Unsetenv("THOP_CONFIG")

	path = DefaultConfigPath()
	if path != "/custom/path/config.toml" {
		t.Errorf("expected '/custom/path/config.toml', got '%s'", path)
	}
}
