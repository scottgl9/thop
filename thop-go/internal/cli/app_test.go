package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/session"
	"github.com/scottgl9/thop/internal/state"
)

func TestNewApp(t *testing.T) {
	app := NewApp("1.0.0", "abc123", "2025-01-01")

	if app == nil {
		t.Fatal("NewApp returned nil")
	}

	if app.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", app.Version)
	}

	if app.GitCommit != "abc123" {
		t.Errorf("expected commit 'abc123', got '%s'", app.GitCommit)
	}

	if app.BuildTime != "2025-01-01" {
		t.Errorf("expected build time '2025-01-01', got '%s'", app.BuildTime)
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantProxy  bool
		wantStatus bool
		wantConfig string
		wantJSON   bool
		wantErr    bool
	}{
		{
			name:       "no flags",
			args:       []string{"thop"},
			wantProxy:  false,
			wantStatus: false,
			wantConfig: "",
			wantJSON:   false,
		},
		{
			name:      "proxy mode",
			args:      []string{"thop", "--proxy"},
			wantProxy: true,
		},
		{
			name:       "status mode",
			args:       []string{"thop", "--status"},
			wantStatus: true,
		},
		{
			name:       "config path",
			args:       []string{"thop", "--config", "/path/to/config.toml"},
			wantConfig: "/path/to/config.toml",
		},
		{
			name:     "json output",
			args:     []string{"thop", "--json"},
			wantJSON: true,
		},
		{
			name:       "combined flags",
			args:       []string{"thop", "--status", "--json", "--config", "/etc/thop.toml"},
			wantStatus: true,
			wantJSON:   true,
			wantConfig: "/etc/thop.toml",
		},
		{
			name:    "invalid flag",
			args:    []string{"thop", "--invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp("test", "test", "test")
			err := app.parseFlags(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if app.proxyMode != tt.wantProxy {
				t.Errorf("proxy mode: expected %v, got %v", tt.wantProxy, app.proxyMode)
			}

			if app.showStatus != tt.wantStatus {
				t.Errorf("status mode: expected %v, got %v", tt.wantStatus, app.showStatus)
			}

			if app.configPath != tt.wantConfig {
				t.Errorf("config path: expected '%s', got '%s'", tt.wantConfig, app.configPath)
			}

			if app.jsonOutput != tt.wantJSON {
				t.Errorf("json output: expected %v, got %v", tt.wantJSON, app.jsonOutput)
			}
		})
	}
}

func createTestApp(t *testing.T) *App {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
			StateFile:      tmpDir + "/state.json",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/sh",
			},
			"testserver": {
				Type: "ssh",
				Host: "example.com",
				User: "testuser",
				Port: 22,
			},
		},
	}

	stateMgr := state.NewManager(cfg.Settings.StateFile)
	stateMgr.Load()

	app := NewApp("1.0.0", "test", "test")
	app.config = cfg
	app.state = stateMgr
	app.sessions = session.NewManager(cfg, stateMgr)

	return app
}

func TestPrintStatus(t *testing.T) {
	app := createTestApp(t)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.printStatus()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printStatus failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check output contains session info
	if !strings.Contains(output, "local") {
		t.Error("expected output to contain 'local'")
	}

	if !strings.Contains(output, "Sessions:") {
		t.Error("expected output to contain 'Sessions:'")
	}
}

func TestPrintStatusJSON(t *testing.T) {
	app := createTestApp(t)
	app.jsonOutput = true

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := app.printStatus()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printStatus failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify it's valid JSON
	var sessions []session.SessionInfo
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	// Check sessions are present
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestOutputError(t *testing.T) {
	app := createTestApp(t)

	// Test plain error
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := session.NewError(
		session.ErrConnectionFailed,
		"Connection failed",
		"test",
	)
	err.Host = "example.com"
	err.Retryable = true
	err.Suggestion = "Check connectivity"
	app.outputError(err)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Connection failed") {
		t.Errorf("expected error message in output, got: %s", output)
	}

	if !strings.Contains(output, "Check connectivity") {
		t.Errorf("expected suggestion in output, got: %s", output)
	}
}

func TestOutputErrorJSON(t *testing.T) {
	app := createTestApp(t)
	app.jsonOutput = true

	// Test JSON error
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := session.NewError(
		session.ErrAuthFailed,
		"Auth failed",
		"prod",
	)
	err.Host = "prod.example.com"
	err.Retryable = false
	err.Suggestion = "Check credentials"
	app.outputError(err)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify it's valid JSON
	var errData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &errData); err != nil {
		t.Errorf("output is not valid JSON: %v, output: %s", err, output)
	}

	if errData["code"] != string(session.ErrAuthFailed) {
		t.Errorf("expected code '%s', got '%v'", session.ErrAuthFailed, errData["code"])
	}

	if errData["session"] != "prod" {
		t.Errorf("expected session 'prod', got '%v'", errData["session"])
	}
}
