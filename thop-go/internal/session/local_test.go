package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLocalSession(t *testing.T) {
	session := NewLocalSession("test", "/bin/bash")

	if session.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", session.Name())
	}

	if session.Type() != "local" {
		t.Errorf("expected type 'local', got '%s'", session.Type())
	}

	if !session.IsConnected() {
		t.Error("local session should be connected by default")
	}

	if session.GetCWD() == "" {
		t.Error("expected non-empty CWD")
	}
}

func TestNewLocalSessionDefaultShell(t *testing.T) {
	// Without explicit shell
	session := NewLocalSession("test", "")

	// Should use SHELL env or /bin/sh
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// The shell field is private, but we can test behavior through Execute
	result, err := session.Execute("echo $0")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Output should contain a shell name
	if result.Stdout == "" {
		t.Error("expected output from shell")
	}
}

func TestLocalSessionConnect(t *testing.T) {
	session := NewLocalSession("test", "")

	// Connect should be no-op
	if err := session.Connect(); err != nil {
		t.Errorf("Connect should not error: %v", err)
	}

	if !session.IsConnected() {
		t.Error("should be connected after Connect")
	}
}

func TestLocalSessionDisconnect(t *testing.T) {
	session := NewLocalSession("test", "")

	if err := session.Disconnect(); err != nil {
		t.Errorf("Disconnect should not error: %v", err)
	}

	if session.IsConnected() {
		t.Error("should be disconnected after Disconnect")
	}
}

func TestLocalSessionExecute(t *testing.T) {
	session := NewLocalSession("test", "")

	tests := []struct {
		name       string
		cmd        string
		wantStdout string
		wantExit   int
	}{
		{
			name:       "simple echo",
			cmd:        "echo hello",
			wantStdout: "hello\n",
			wantExit:   0,
		},
		{
			name:       "pwd",
			cmd:        "pwd",
			wantStdout: session.GetCWD() + "\n",
			wantExit:   0,
		},
		{
			name:     "failing command",
			cmd:      "exit 42",
			wantExit: 42,
		},
		{
			name:       "command with args",
			cmd:        "echo one two three",
			wantStdout: "one two three\n",
			wantExit:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.Execute(tt.cmd)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			if tt.wantStdout != "" && result.Stdout != tt.wantStdout {
				t.Errorf("expected stdout '%s', got '%s'", tt.wantStdout, result.Stdout)
			}

			if result.ExitCode != tt.wantExit {
				t.Errorf("expected exit code %d, got %d", tt.wantExit, result.ExitCode)
			}
		})
	}
}

func TestLocalSessionExecuteStderr(t *testing.T) {
	session := NewLocalSession("test", "")

	result, err := session.Execute("echo error >&2")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result.Stderr, "error") {
		t.Errorf("expected stderr to contain 'error', got '%s'", result.Stderr)
	}
}

func TestLocalSessionCD(t *testing.T) {
	session := NewLocalSession("test", "")
	originalCWD := session.GetCWD()

	// Change to /tmp
	result, err := session.Execute("cd /tmp")
	if err != nil {
		t.Fatalf("Execute cd failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("cd should succeed, got exit code %d", result.ExitCode)
	}

	if session.GetCWD() != "/tmp" {
		t.Errorf("expected CWD '/tmp', got '%s'", session.GetCWD())
	}

	// Subsequent commands should run in /tmp
	result, err = session.Execute("pwd")
	if err != nil {
		t.Fatalf("Execute pwd failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "/tmp" {
		t.Errorf("expected pwd '/tmp', got '%s'", result.Stdout)
	}

	// cd to home
	result, err = session.Execute("cd")
	if err != nil {
		t.Fatalf("Execute cd home failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	if session.GetCWD() != home {
		t.Errorf("expected CWD '%s', got '%s'", home, session.GetCWD())
	}

	// cd with ~
	session.Execute("cd /tmp")
	result, err = session.Execute("cd ~")
	if err != nil {
		t.Fatalf("Execute cd ~ failed: %v", err)
	}

	if session.GetCWD() != home {
		t.Errorf("expected CWD '%s' after cd ~, got '%s'", home, session.GetCWD())
	}

	// Restore original
	session.Execute("cd " + originalCWD)
}

func TestLocalSessionCDNonExistent(t *testing.T) {
	session := NewLocalSession("test", "")
	originalCWD := session.GetCWD()

	result, err := session.Execute("cd /nonexistent_path_12345")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitCode == 0 {
		t.Error("cd to nonexistent should fail")
	}

	if !strings.Contains(result.Stderr, "No such file") {
		t.Errorf("expected error message, got '%s'", result.Stderr)
	}

	// CWD should be unchanged
	if session.GetCWD() != originalCWD {
		t.Errorf("CWD should be unchanged, got '%s'", session.GetCWD())
	}
}

func TestLocalSessionCDToFile(t *testing.T) {
	session := NewLocalSession("test", "")

	// Create a temporary file
	tmpFile := filepath.Join(os.TempDir(), "thop_test_file")
	os.WriteFile(tmpFile, []byte("test"), 0644)
	defer os.Remove(tmpFile)

	result, err := session.Execute("cd " + tmpFile)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitCode == 0 {
		t.Error("cd to file should fail")
	}

	if !strings.Contains(result.Stderr, "Not a directory") {
		t.Errorf("expected 'Not a directory' error, got '%s'", result.Stderr)
	}
}

func TestLocalSessionSetCWD(t *testing.T) {
	session := NewLocalSession("test", "")

	// Valid directory
	if err := session.SetCWD("/tmp"); err != nil {
		t.Errorf("SetCWD should succeed for /tmp: %v", err)
	}

	if session.GetCWD() != "/tmp" {
		t.Errorf("expected CWD '/tmp', got '%s'", session.GetCWD())
	}

	// Invalid directory
	if err := session.SetCWD("/nonexistent_12345"); err == nil {
		t.Error("SetCWD should fail for nonexistent directory")
	}
}

func TestLocalSessionEnv(t *testing.T) {
	session := NewLocalSession("test", "")

	// Set environment variable
	session.SetEnv("TEST_VAR", "test_value")

	env := session.GetEnv()
	if env["TEST_VAR"] != "test_value" {
		t.Errorf("expected TEST_VAR='test_value', got '%s'", env["TEST_VAR"])
	}

	// Environment should be used in commands
	result, err := session.Execute("echo $TEST_VAR")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "test_value" {
		t.Errorf("expected 'test_value', got '%s'", result.Stdout)
	}

	// GetEnv should return a copy
	env["NEW_VAR"] = "new"
	env2 := session.GetEnv()
	if _, ok := env2["NEW_VAR"]; ok {
		t.Error("GetEnv should return a copy")
	}
}

func TestLocalSessionSetShell(t *testing.T) {
	session := NewLocalSession("test", "/bin/bash")

	// Change shell
	session.SetShell("/bin/sh")

	// Test that it uses the new shell
	result, err := session.Execute("echo test")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "test" {
		t.Errorf("expected 'test', got '%s'", result.Stdout)
	}
}

func TestFormatPrompt(t *testing.T) {
	// Test without cwd
	prompt := FormatPrompt("local", "")
	if prompt != "(local) $ " {
		t.Errorf("expected '(local) $ ', got '%s'", prompt)
	}

	prompt = FormatPrompt("prod", "")
	if prompt != "(prod) $ " {
		t.Errorf("expected '(prod) $ ', got '%s'", prompt)
	}

	// Test with cwd
	prompt = FormatPrompt("local", "/tmp")
	if prompt != "(local) /tmp $ " {
		t.Errorf("expected '(local) /tmp $ ', got '%s'", prompt)
	}

	// Test with home directory shortening
	home, _ := os.UserHomeDir()
	prompt = FormatPrompt("local", home)
	if prompt != "(local) ~ $ " {
		t.Errorf("expected '(local) ~ $ ', got '%s'", prompt)
	}

	// Test with subdirectory of home
	prompt = FormatPrompt("local", home+"/projects")
	if prompt != "(local) ~/projects $ " {
		t.Errorf("expected '(local) ~/projects $ ', got '%s'", prompt)
	}
}

func TestLocalSessionTimeout(t *testing.T) {
	session := NewLocalSession("test", "")
	session.SetTimeout(100 * time.Millisecond) // Very short timeout

	// Run a command that takes longer than timeout
	_, err := session.Execute("sleep 2")

	if err == nil {
		t.Fatal("expected timeout error")
	}

	sessionErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}

	if sessionErr.Code != ErrCommandTimeout {
		t.Errorf("expected code %s, got %s", ErrCommandTimeout, sessionErr.Code)
	}

	if !sessionErr.Retryable {
		t.Error("timeout error should be retryable")
	}
}

func TestLocalSessionSetTimeout(t *testing.T) {
	session := NewLocalSession("test", "")

	// Default timeout should be 300 seconds
	// We can't directly access the timeout field, but we can test that
	// setting a new timeout works by running a command that would timeout

	session.SetTimeout(50 * time.Millisecond)

	// Quick command should succeed
	result, err := session.Execute("echo fast")
	if err != nil {
		t.Fatalf("fast command should succeed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "fast" {
		t.Errorf("expected 'fast', got '%s'", result.Stdout)
	}
}
