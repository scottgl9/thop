package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	config, err := LoadFromFile("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadFromFile should not error for nonexistent: %v", err)
	}

	if config == nil {
		t.Fatal("config should not be nil")
	}

	if len(config.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(config.Hosts))
	}
}

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
# Test SSH config
Host myserver
    HostName server.example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/mykey

Host prod
    HostName prod.example.com
    User admin

Host dev staging
    HostName dev.example.com
    User developer
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Check myserver
	host := config.GetHost("myserver")
	if host == nil {
		t.Fatal("expected to find 'myserver' host")
	}

	if host.HostName != "server.example.com" {
		t.Errorf("expected hostname 'server.example.com', got '%s'", host.HostName)
	}

	if host.User != "deploy" {
		t.Errorf("expected user 'deploy', got '%s'", host.User)
	}

	if host.Port != "2222" {
		t.Errorf("expected port '2222', got '%s'", host.Port)
	}

	// Check prod
	host = config.GetHost("prod")
	if host == nil {
		t.Fatal("expected to find 'prod' host")
	}

	if host.HostName != "prod.example.com" {
		t.Errorf("expected hostname 'prod.example.com', got '%s'", host.HostName)
	}

	// Check dev (multiple patterns)
	host = config.GetHost("dev")
	if host == nil {
		t.Fatal("expected to find 'dev' host")
	}

	if host.HostName != "dev.example.com" {
		t.Errorf("expected hostname 'dev.example.com', got '%s'", host.HostName)
	}

	// Check staging (same config as dev)
	host = config.GetHost("staging")
	if host == nil {
		t.Fatal("expected to find 'staging' host")
	}
}

func TestResolveHost(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host myserver
    HostName actual.server.com
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	// Known alias
	if got := config.ResolveHost("myserver"); got != "actual.server.com" {
		t.Errorf("expected 'actual.server.com', got '%s'", got)
	}

	// Unknown host - should return itself
	if got := config.ResolveHost("unknown"); got != "unknown" {
		t.Errorf("expected 'unknown', got '%s'", got)
	}
}

func TestResolveUser(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host myserver
    User deploy
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	// Known alias
	if got := config.ResolveUser("myserver"); got != "deploy" {
		t.Errorf("expected 'deploy', got '%s'", got)
	}

	// Unknown host
	if got := config.ResolveUser("unknown"); got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestResolvePort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host myserver
    Port 2222

Host defaultport
    HostName example.com
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	// Custom port
	if got := config.ResolvePort("myserver"); got != "2222" {
		t.Errorf("expected '2222', got '%s'", got)
	}

	// Default port
	if got := config.ResolvePort("defaultport"); got != "22" {
		t.Errorf("expected '22', got '%s'", got)
	}

	// Unknown host
	if got := config.ResolvePort("unknown"); got != "22" {
		t.Errorf("expected '22', got '%s'", got)
	}
}

func TestWildcardPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host *
    User defaultuser

Host *.example.com
    User exampleuser

Host specific
    HostName specific.example.com
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	// Wildcard patterns should be skipped
	if config.GetHost("*") != nil {
		t.Error("wildcard * should not be stored")
	}

	if config.GetHost("*.example.com") != nil {
		t.Error("wildcard *.example.com should not be stored")
	}

	// Specific host should be stored
	if config.GetHost("specific") == nil {
		t.Error("specific host should be stored")
	}
}

func TestListHosts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host server1
    HostName server1.example.com

Host server2
    HostName server2.example.com
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	hosts := config.ListHosts()
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestEqualsFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	// Test with = separator
	content := `
Host myserver
    HostName=server.example.com
    User=deploy
`

	os.WriteFile(configPath, []byte(content), 0644)

	config, _ := LoadFromFile(configPath)

	host := config.GetHost("myserver")
	if host == nil {
		t.Fatal("expected to find 'myserver'")
	}

	if host.HostName != "server.example.com" {
		t.Errorf("expected 'server.example.com', got '%s'", host.HostName)
	}

	if host.User != "deploy" {
		t.Errorf("expected 'deploy', got '%s'", host.User)
	}
}
