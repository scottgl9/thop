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
	prompt := session.FormatPrompt("local", "")
	if prompt != "(local) $ " {
		t.Errorf("expected '(local) $ ', got '%s'", prompt)
	}

	prompt = session.FormatPrompt("prod", "")
	if prompt != "(prod) $ " {
		t.Errorf("expected '(prod) $ ', got '%s'", prompt)
	}

	// Test with cwd
	prompt = session.FormatPrompt("local", "/var/log")
	if prompt != "(local) /var/log $ " {
		t.Errorf("expected '(local) /var/log $ ', got '%s'", prompt)
	}
}

// Tests for new slash commands

func TestHandleSlashCommandAuth(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Auth without argument
	err := app.handleSlashCommand("/auth")
	if err == nil {
		t.Error("expected error for /auth without argument")
	}

	// Auth on non-existent session
	err = app.handleSlashCommand("/auth nonexistent")
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

	// Auth on local session (should fail - not SSH)
	err = app.handleSlashCommand("/auth local")
	if err == nil {
		t.Error("expected error for /auth on local session")
	}
	if !strings.Contains(err.Error(), "not an SSH session") {
		t.Errorf("expected 'not an SSH session' error, got: %v", err)
	}
}

func TestHandleSlashCommandTrust(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Trust without argument
	err := app.handleSlashCommand("/trust")
	if err == nil {
		t.Error("expected error for /trust without argument")
	}

	// Trust on non-existent session
	err = app.handleSlashCommand("/trust nonexistent")
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

	// Trust on local session (should fail - not SSH)
	err = app.handleSlashCommand("/trust local")
	if err == nil {
		t.Error("expected error for /trust on local session")
	}
	if !strings.Contains(err.Error(), "not an SSH session") {
		t.Errorf("expected 'not an SSH session' error, got: %v", err)
	}
}

func TestHandleSlashCommandCopy(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Copy without arguments
	err := app.handleSlashCommand("/copy")
	if err == nil {
		t.Error("expected error for /copy without arguments")
	}

	// Copy with only one argument
	err = app.handleSlashCommand("/copy source")
	if err == nil {
		t.Error("expected error for /copy with only one argument")
	}

	// Copy with /cp alias
	err = app.handleSlashCommand("/cp")
	if err == nil {
		t.Error("expected error for /cp without arguments")
	}

	// Copy local to local (should fail)
	err = app.handleSlashCommand("/copy local:/tmp/test local:/tmp/test2")
	if err == nil {
		t.Error("expected error for local-to-local copy")
	}
	if !strings.Contains(err.Error(), "both source and destination are local") {
		t.Errorf("expected 'both source and destination are local' error, got: %v", err)
	}

	// Copy with non-existent source session
	err = app.handleSlashCommand("/copy nonexistent:/path local:/path")
	if err == nil {
		t.Error("expected error for non-existent source session")
	}

	// Copy with non-existent destination session
	err = app.handleSlashCommand("/copy local:/path nonexistent:/path")
	if err == nil {
		t.Error("expected error for non-existent destination session")
	}
}

func TestHandleSlashCommandAddSession(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Add without arguments
	err := app.handleSlashCommand("/add-session")
	if err == nil {
		t.Error("expected error for /add-session without arguments")
	}

	// Add with only one argument
	err = app.handleSlashCommand("/add-session newsession")
	if err == nil {
		t.Error("expected error for /add-session with only one argument")
	}

	// Add with /add alias
	err = app.handleSlashCommand("/add")
	if err == nil {
		t.Error("expected error for /add without arguments")
	}

	// Try to add session with existing name
	err = app.handleSlashCommand("/add-session local user@host.com")
	if err == nil {
		t.Error("expected error for adding session with existing name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
}

func TestHandleSlashCommandRead(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Read without argument
	err := app.handleSlashCommand("/read")
	if err == nil {
		t.Error("expected error for /read without argument")
	}

	// Read non-existent file on local session
	err = app.handleSlashCommand("/read /nonexistent/file/path")
	if err == nil {
		t.Error("expected error for reading non-existent file")
	}

	// Create a test file and read it
	tmpDir := t.TempDir()
	testFile := tmpDir + "/testfile.txt"
	testContent := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/read " + testFile)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/read should not error for existing file: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output != testContent {
		t.Errorf("expected '%s', got '%s'", testContent, output)
	}

	// Test /cat alias
	oldStdout = os.Stdout
	r, w, _ = os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/cat " + testFile)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/cat alias should work: %v", err)
	}

	buf.Reset()
	io.Copy(&buf, r)
	output = buf.String()

	if output != testContent {
		t.Errorf("/cat expected '%s', got '%s'", testContent, output)
	}
}

func TestHandleSlashCommandWrite(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Write without argument
	err := app.handleSlashCommand("/write")
	if err == nil {
		t.Error("expected error for /write without argument")
	}

	// Write without content
	err = app.handleSlashCommand("/write /tmp/test")
	if err == nil {
		t.Error("expected error for /write without content")
	}

	// Write to a file
	tmpDir := t.TempDir()
	testFile := tmpDir + "/output.txt"
	testContent := "Test content"

	// Suppress stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/write " + testFile + " " + testContent)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/write should not error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected '%s', got '%s'", testContent, string(data))
	}
}

func TestHandleSlashCommandEnv(t *testing.T) {
	app := createInteractiveTestApp(t)

	// Suppress stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Show env (no args)
	err := app.handleSlashCommand("/env")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/env should not error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "No environment") {
		t.Errorf("expected 'No environment' message, got: %s", output)
	}

	// Set env
	oldStdout = os.Stdout
	r, w, _ = os.Pipe()
	os.Stdout = w

	err = app.handleSlashCommand("/env TEST_VAR=test_value")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("/env KEY=VALUE should not error: %v", err)
	}

	// Invalid env format
	err = app.handleSlashCommand("/env INVALID")
	if err == nil {
		t.Error("expected error for invalid env format")
	}
}

func TestParseFileSpec(t *testing.T) {
	tests := []struct {
		input           string
		expectedSession string
		expectedPath    string
	}{
		{"local:/path/to/file", "local", "/path/to/file"},
		{"remote:/home/user/file", "remote", "/home/user/file"},
		{"server1:/etc/config", "server1", "/etc/config"},
		{"/absolute/path", "", "/absolute/path"},
		{"relative/path", "", "relative/path"},
		{"C:/Windows/path", "", "C:/Windows/path"}, // Windows path
		{"D:\\Windows\\path", "", "D:\\Windows\\path"}, // Windows path with backslash
	}

	for _, tt := range tests {
		sess, path := parseFileSpec(tt.input)
		if sess != tt.expectedSession {
			t.Errorf("parseFileSpec(%q) session = %q, want %q", tt.input, sess, tt.expectedSession)
		}
		if path != tt.expectedPath {
			t.Errorf("parseFileSpec(%q) path = %q, want %q", tt.input, path, tt.expectedPath)
		}
	}
}

func TestParseHostSpec(t *testing.T) {
	// Save and restore USER env
	oldUser := os.Getenv("USER")
	os.Setenv("USER", "testuser")
	defer os.Setenv("USER", oldUser)

	tests := []struct {
		input        string
		expectedUser string
		expectedHost string
		expectedPort int
	}{
		{"example.com", "testuser", "example.com", 22},
		{"user@example.com", "user", "example.com", 22},
		{"example.com:2222", "testuser", "example.com", 2222},
		{"user@example.com:2222", "user", "example.com", 2222},
		{"deploy@prod.server.com:22", "deploy", "prod.server.com", 22},
		{"admin@192.168.1.1:22222", "admin", "192.168.1.1", 22222},
	}

	for _, tt := range tests {
		user, host, port := parseHostSpec(tt.input)
		if user != tt.expectedUser {
			t.Errorf("parseHostSpec(%q) user = %q, want %q", tt.input, user, tt.expectedUser)
		}
		if host != tt.expectedHost {
			t.Errorf("parseHostSpec(%q) host = %q, want %q", tt.input, host, tt.expectedHost)
		}
		if port != tt.expectedPort {
			t.Errorf("parseHostSpec(%q) port = %d, want %d", tt.input, port, tt.expectedPort)
		}
	}
}
