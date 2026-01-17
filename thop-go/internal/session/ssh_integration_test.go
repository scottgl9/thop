//go:build integration

package session

import (
	"os"
	"testing"
	"time"
)

// Integration tests for SSH sessions
// Run with: go test -tags=integration ./internal/session/...
// Requires Docker test container running: docker-compose -f test/docker/docker-compose.yml up -d

const (
	testSSHHost     = "localhost"
	testSSHPort     = 2222
	testSSHUser     = "testuser"
	testSSHPassword = "testpass123"
)

func skipIfNoDocker(t *testing.T) {
	if os.Getenv("THOP_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test (set THOP_INTEGRATION_TESTS=1 to run)")
	}
}

func TestSSHSessionConnect(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	if !session.IsConnected() {
		t.Error("Session should be connected")
	}
}

func TestSSHSessionExecute(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	result, err := session.Execute("echo hello")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Stdout != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
}

func TestSSHSessionCD(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	// Change to /tmp
	_, err = session.Execute("cd /tmp")
	if err != nil {
		t.Fatalf("cd failed: %v", err)
	}

	// Verify cwd changed
	result, err := session.Execute("pwd")
	if err != nil {
		t.Fatalf("pwd failed: %v", err)
	}

	if result.Stdout != "/tmp\n" {
		t.Errorf("Expected '/tmp\\n', got %q", result.Stdout)
	}

	if session.GetCWD() != "/tmp" {
		t.Errorf("Expected cwd '/tmp', got %q", session.GetCWD())
	}
}

func TestSSHSessionEnv(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	// Set an environment variable
	session.SetEnv("MY_VAR", "my_value")

	// Verify it's set
	result, err := session.Execute("echo $MY_VAR")
	if err != nil {
		t.Fatalf("echo failed: %v", err)
	}

	if result.Stdout != "my_value\n" {
		t.Errorf("Expected 'my_value\\n', got %q", result.Stdout)
	}
}

func TestSSHSessionDisconnect(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !session.IsConnected() {
		t.Error("Session should be connected")
	}

	err = session.Disconnect()
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	if session.IsConnected() {
		t.Error("Session should be disconnected")
	}
}

func TestSSHSessionReconnect(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Set up some state
	session.SetEnv("TEST_VAR", "test_value")
	session.Execute("cd /tmp")

	// Disconnect
	session.Disconnect()

	// Reconnect
	err = session.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}
	defer session.Disconnect()

	if !session.IsConnected() {
		t.Error("Session should be connected after reconnect")
	}

	// Verify state was restored
	result, err := session.Execute("pwd")
	if err != nil {
		t.Fatalf("pwd failed: %v", err)
	}

	if result.Stdout != "/tmp\n" {
		t.Errorf("Expected cwd '/tmp' after reconnect, got %q", result.Stdout)
	}
}

func TestSSHSessionCommandTimeout(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
		Timeout:  1 * time.Second, // Very short timeout
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	// Run a command that takes longer than timeout
	_, err = session.Execute("sleep 5")
	if err == nil {
		t.Error("Expected timeout error")
	}

	// Verify it's a timeout error
	if sessErr, ok := err.(*Error); ok {
		if sessErr.Code != ErrCommandTimeout {
			t.Errorf("Expected ErrCommandTimeout, got %s", sessErr.Code)
		}
	}
}

func TestSSHSessionSFTP(t *testing.T) {
	skipIfNoDocker(t)

	session := NewSSHSession(SSHConfig{
		Name:     "test",
		Host:     testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	})

	err := session.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Disconnect()

	// Write a file
	testContent := []byte("Hello from SFTP test!")
	err = session.WriteFile("/tmp/thop_test.txt", testContent, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Read the file back
	content, err := session.ReadFile("/tmp/thop_test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Expected %q, got %q", string(testContent), string(content))
	}

	// Clean up
	session.Execute("rm /tmp/thop_test.txt")
}
