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

func createProxyTestApp(t *testing.T) *App {
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
		},
	}

	stateMgr := state.NewManager(cfg.Settings.StateFile)
	stateMgr.Load()

	app := NewApp("1.0.0", "test", "test")
	app.config = cfg
	app.state = stateMgr
	app.sessions = session.NewManager(cfg, stateMgr)
	app.proxyMode = true

	return app
}

func TestProxyModeBasic(t *testing.T) {
	app := createProxyTestApp(t)

	// Create input with commands
	input := "echo hello\necho world\n"
	inputReader := strings.NewReader(input)

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Write input in background
	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	// Run proxy mode
	err := app.runProxy()

	outW.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, outR)
	output := buf.String()

	// Restore stdin for other tests
	_ = inputReader

	// Check output contains expected results
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", output)
	}

	if !strings.Contains(output, "world") {
		t.Errorf("expected 'world' in output, got: %s", output)
	}
}

func TestProxyModeEmptyLine(t *testing.T) {
	app := createProxyTestApp(t)

	// Create input with empty lines
	input := "echo test\n\n\necho test2\n"

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	err := app.runProxy()

	outW.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, outR)
	output := buf.String()

	// Should have output from both commands
	if !strings.Contains(output, "test") {
		t.Errorf("expected 'test' in output")
	}

	if !strings.Contains(output, "test2") {
		t.Errorf("expected 'test2' in output")
	}
}

func TestProxyModeEOF(t *testing.T) {
	app := createProxyTestApp(t)

	// Create input that ends immediately
	input := ""

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	_, outW, _ := os.Pipe()
	os.Stdout = outW

	err := app.runProxy()

	outW.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	// Should exit cleanly on EOF
	if err != nil {
		t.Errorf("runProxy should exit cleanly on EOF: %v", err)
	}
}

func TestProxyModeStderr(t *testing.T) {
	app := createProxyTestApp(t)

	// Create input that writes to stderr
	input := "echo error >&2\n"

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout and stderr
	oldStdout := os.Stdout
	_, outW, _ := os.Pipe()
	os.Stdout = outW

	oldStderr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW

	err := app.runProxy()

	outW.Close()
	errW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, errR)
	stderrOutput := buf.String()

	if !strings.Contains(stderrOutput, "error") {
		t.Errorf("expected 'error' in stderr, got: %s", stderrOutput)
	}
}

func TestProxyModeFailingCommand(t *testing.T) {
	app := createProxyTestApp(t)
	app.verbose = true

	// Create input with failing command
	input := "exit 1\necho still running\n"

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout and stderr
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	oldStderr := os.Stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW

	err := app.runProxy()

	outW.Close()
	errW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	io.Copy(&stdoutBuf, outR)
	io.Copy(&stderrBuf, errR)

	// Should continue after failing command
	if !strings.Contains(stdoutBuf.String(), "still running") {
		t.Error("proxy mode should continue after failing command")
	}

	// Verbose mode should show exit code
	if !strings.Contains(stderrBuf.String(), "exit code") {
		t.Error("verbose mode should show exit code")
	}
}

func TestProxyModeWindowsLineEndings(t *testing.T) {
	app := createProxyTestApp(t)

	// Create input with Windows line endings
	input := "echo test\r\necho test2\r\n"

	// Replace stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString(input)
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	err := app.runProxy()

	outW.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, outR)
	output := buf.String()

	// Should handle CRLF properly
	if !strings.Contains(output, "test") {
		t.Errorf("expected 'test' in output")
	}

	if !strings.Contains(output, "test2") {
		t.Errorf("expected 'test2' in output")
	}
}

func TestProxyModeExecuteSingleCommand(t *testing.T) {
	app := createProxyTestApp(t)

	// Execute single command
	result := app.executeProxyCommand("echo hello world")

	// Should succeed with exit code 0
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestProxyModeExecuteFailingCommand(t *testing.T) {
	app := createProxyTestApp(t)

	// Execute failing command
	result := app.executeProxyCommand("exit 42")

	// Should return the command's exit code
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestErrorToExitCode(t *testing.T) {
	app := createProxyTestApp(t)

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "auth password required",
			err:      &session.Error{Code: session.ErrAuthPasswordRequired},
			expected: ExitAuthError,
		},
		{
			name:     "auth failed",
			err:      &session.Error{Code: session.ErrAuthFailed},
			expected: ExitAuthError,
		},
		{
			name:     "host key verification",
			err:      &session.Error{Code: session.ErrHostKeyVerification},
			expected: ExitHostKeyError,
		},
		{
			name:     "host key changed",
			err:      &session.Error{Code: session.ErrHostKeyChanged},
			expected: ExitHostKeyError,
		},
		{
			name:     "connection failed",
			err:      &session.Error{Code: session.ErrConnectionFailed},
			expected: ExitGeneralError,
		},
		{
			name:     "session not found",
			err:      &session.Error{Code: session.ErrSessionNotFound},
			expected: ExitGeneralError,
		},
		{
			name:     "generic error",
			err:      os.ErrNotExist,
			expected: ExitGeneralError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := app.errorToExitCode(tt.err)
			if exitCode != tt.expected {
				t.Errorf("expected exit code %d, got %d", tt.expected, exitCode)
			}
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	// Verify exit code constants are correct
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess should be 0, got %d", ExitSuccess)
	}
	if ExitGeneralError != 1 {
		t.Errorf("ExitGeneralError should be 1, got %d", ExitGeneralError)
	}
	if ExitAuthError != 2 {
		t.Errorf("ExitAuthError should be 2, got %d", ExitAuthError)
	}
	if ExitHostKeyError != 3 {
		t.Errorf("ExitHostKeyError should be 3, got %d", ExitHostKeyError)
	}
}
