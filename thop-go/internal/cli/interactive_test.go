package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/session"
	"github.com/scottgl9/thop/internal/state"
)

func createInteractiveTestApp(t *testing.T) *App {
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
	app.quiet = true // Suppress output

	return app
}

func TestHandleSlashCommandHelp(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	commands := []string{"/help", "/h", "/?"}
	for _, cmd := range commands {
		err := app.handleSlashCommand(cmd)
		if err != nil {
			t.Errorf("command %s should not error: %v", cmd, err)
		}
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check help output contains expected commands
	if !strings.Contains(output, "/connect") {
		t.Error("help output should contain /connect")
	}

	if !strings.Contains(output, "/switch") {
		t.Error("help output should contain /switch")
	}

	if !strings.Contains(output, "/local") {
		t.Error("help output should contain /local")
	}
}

func TestHandleSlashCommandStatus(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	commands := []string{"/status", "/s", "/sessions", "/list"}
	for _, cmd := range commands {
		err := app.handleSlashCommand(cmd)
		if err != nil {
			t.Errorf("command %s should not error: %v", cmd, err)
		}
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check status output
	if !strings.Contains(output, "local") {
		t.Error("status output should contain 'local' session")
	}
}

func TestHandleSlashCommandConnect(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Connect without argument
	err := app.handleSlashCommand("/connect")
	if err == nil {
		t.Error("expected error for /connect without argument")
	}

	// Connect to non-existent session
	err = app.handleSlashCommand("/connect nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	sessionErr, ok := err.(*session.Error)
	if !ok {
		t.Errorf("expected *session.Error, got %T", err)
	}

	if sessionErr.Code != session.ErrSessionNotFound {
		t.Errorf("expected code %s, got %s", session.ErrSessionNotFound, sessionErr.Code)
	}

	// Connect to local (should print message)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/connect local")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("connect local should not error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "local") {
		t.Error("expected message about local session")
	}
}

func TestHandleSlashCommandSwitch(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Switch without argument
	err := app.handleSlashCommand("/switch")
	if err == nil {
		t.Error("expected error for /switch without argument")
	}

	// Switch to non-existent session
	err = app.handleSlashCommand("/switch nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	// Switch to local
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/switch local")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("switch local should not error: %v", err)
	}

	if app.sessions.GetActiveSessionName() != "local" {
		t.Error("active session should be local")
	}

	// Test shortcut
	err = app.handleSlashCommand("/sw local")
	if err != nil {
		t.Errorf("/sw shortcut should work: %v", err)
	}
}

func TestHandleSlashCommandLocal(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress output
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := app.handleSlashCommand("/local")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/local should not error: %v", err)
	}

	if app.sessions.GetActiveSessionName() != "local" {
		t.Error("active session should be local")
	}

	// Test shortcut
	oldStdout = os.Stdout
	_, w, _ = os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/l")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/l shortcut should work: %v", err)
	}
}

func TestHandleSlashCommandClose(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Close without argument
	err := app.handleSlashCommand("/close")
	if err == nil {
		t.Error("expected error for /close without argument")
	}

	// Close non-existent session
	err = app.handleSlashCommand("/close nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	// Close local (should print message)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/close local")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("close local should not error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Cannot close local") {
		t.Errorf("expected 'Cannot close local' message, got: %s", output)
	}

	// Test shortcuts
	err = app.handleSlashCommand("/d nonexistent")
	if err == nil {
		t.Error("/d shortcut should work and return error for nonexistent")
	}

	err = app.handleSlashCommand("/disconnect nonexistent")
	if err == nil {
		t.Error("/disconnect should work and return error for nonexistent")
	}
}

func TestHandleSlashCommandUnknown(t *testing.T) {
	app := createInteractiveTestApp(t)

	err := app.handleSlashCommand("/unknowncmd")
	if err == nil {
		t.Error("expected error for unknown command")
	}

	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got: %v", err)
	}
}

func TestHandleSlashCommandEmpty(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Empty string shouldn't error
	err := app.handleSlashCommand("")
	if err != nil {
		t.Errorf("empty command should not error: %v", err)
	}
}

func TestCmdConnectSSH(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Try to connect to SSH session (will fail since server isn't real)
	err := app.cmdConnect("testserver")

	w.Close()
	os.Stdout = oldStdout

	// Should return an error since we can't actually connect
	if err == nil {
		t.Log("cmdConnect returned nil error (SSH connection might have succeeded in test environment)")
	} else {
		// Expected to fail
		t.Logf("cmdConnect returned expected error: %v", err)
	}
}

func TestCmdSwitchToSSH(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Try to switch to SSH session (will fail connection)
	err := app.cmdSwitch("testserver")

	w.Close()
	os.Stdout = oldStdout

	// Should return error since we can't connect
	if err == nil {
		t.Log("cmdSwitch returned nil error (SSH connection might have succeeded)")
	}
}

func TestCmdCloseNotConnected(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Try to close SSH session that's not connected
	err := app.cmdClose("testserver")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("cmdClose should not error for not-connected session: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "not connected") {
		t.Errorf("expected 'not connected' message, got: %s", output)
	}
}

func TestFormatPrompt(t *testing.T) {
	prompt := session.FormatPrompt("local")
	if prompt != "(local) $ " {
		t.Errorf("expected '(local) $ ', got '%s'", prompt)
	}

	prompt = session.FormatPrompt("prod")
	if prompt != "(prod) $ " {
		t.Errorf("expected '(prod) $ ', got '%s'", prompt)
	}
}
