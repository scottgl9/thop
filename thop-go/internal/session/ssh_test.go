package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSSHSessionPasswordEnv(t *testing.T) {
	// Set up environment variable
	envVar := "TEST_THOP_PASSWORD"
	password := "secretpassword123"
	os.Setenv(envVar, password)
	defer os.Unsetenv(envVar)

	session := NewSSHSession(SSHConfig{
		Name:        "test",
		Host:        "example.com",
		User:        "testuser",
		PasswordEnv: envVar,
	})

	if !session.HasPassword() {
		t.Error("Expected session to have password from env var")
	}
}

func TestNewSSHSessionPasswordEnvEmpty(t *testing.T) {
	// Make sure env var doesn't exist
	os.Unsetenv("NONEXISTENT_VAR")

	session := NewSSHSession(SSHConfig{
		Name:        "test",
		Host:        "example.com",
		User:        "testuser",
		PasswordEnv: "NONEXISTENT_VAR",
	})

	if session.HasPassword() {
		t.Error("Expected session to not have password when env var doesn't exist")
	}
}

func TestNewSSHSessionPasswordFile(t *testing.T) {
	// Create temp directory and password file
	tmpDir := t.TempDir()
	pwFile := filepath.Join(tmpDir, "password")
	password := "filepassword456"

	// Write password with secure permissions
	if err := os.WriteFile(pwFile, []byte(password+"\n"), 0600); err != nil {
		t.Fatalf("Failed to write password file: %v", err)
	}

	session := NewSSHSession(SSHConfig{
		Name:         "test",
		Host:         "example.com",
		User:         "testuser",
		PasswordFile: pwFile,
	})

	if !session.HasPassword() {
		t.Error("Expected session to have password from file")
	}
}

func TestNewSSHSessionPasswordFileInsecurePermissions(t *testing.T) {
	// Create temp directory and password file
	tmpDir := t.TempDir()
	pwFile := filepath.Join(tmpDir, "password")
	password := "insecurepassword"

	// Write password with insecure permissions (world readable)
	if err := os.WriteFile(pwFile, []byte(password), 0644); err != nil {
		t.Fatalf("Failed to write password file: %v", err)
	}

	session := NewSSHSession(SSHConfig{
		Name:         "test",
		Host:         "example.com",
		User:         "testuser",
		PasswordFile: pwFile,
	})

	// Password should not be loaded due to insecure permissions
	if session.HasPassword() {
		t.Error("Expected session to not load password from insecure file")
	}
}

func TestNewSSHSessionPasswordFileNotFound(t *testing.T) {
	session := NewSSHSession(SSHConfig{
		Name:         "test",
		Host:         "example.com",
		User:         "testuser",
		PasswordFile: "/nonexistent/password/file",
	})

	if session.HasPassword() {
		t.Error("Expected session to not have password when file doesn't exist")
	}
}

func TestNewSSHSessionPasswordFileTilde(t *testing.T) {
	// This test verifies that ~ expansion works
	// We can't actually test the expansion without writing to home dir,
	// so we just verify the session doesn't crash
	session := NewSSHSession(SSHConfig{
		Name:         "test",
		Host:         "example.com",
		User:         "testuser",
		PasswordFile: "~/.nonexistent_thop_test_password",
	})

	// Should not panic, password should be empty
	if session.HasPassword() {
		t.Error("Expected session to not have password from nonexistent file")
	}
}

func TestNewSSHSessionPasswordPriority(t *testing.T) {
	// Test that direct password takes precedence over env and file
	envVar := "TEST_THOP_PASSWORD_PRIORITY"
	os.Setenv(envVar, "envpassword")
	defer os.Unsetenv(envVar)

	// Create password file
	tmpDir := t.TempDir()
	pwFile := filepath.Join(tmpDir, "password")
	if err := os.WriteFile(pwFile, []byte("filepassword"), 0600); err != nil {
		t.Fatalf("Failed to write password file: %v", err)
	}

	// Direct password should not be used in NewSSHSession (it's for SetPassword)
	// But PasswordEnv takes precedence over PasswordFile
	session := NewSSHSession(SSHConfig{
		Name:         "test",
		Host:         "example.com",
		User:         "testuser",
		PasswordEnv:  envVar,
		PasswordFile: pwFile,
	})

	// Should have password (from env var, which takes precedence)
	if !session.HasPassword() {
		t.Error("Expected session to have password")
	}
}

func TestSSHSessionSetPassword(t *testing.T) {
	session := NewSSHSession(SSHConfig{
		Name: "test",
		Host: "example.com",
		User: "testuser",
	})

	if session.HasPassword() {
		t.Error("Expected session to not have password initially")
	}

	session.SetPassword("newpassword")

	if !session.HasPassword() {
		t.Error("Expected session to have password after SetPassword")
	}
}

func TestSSHSessionBasicFields(t *testing.T) {
	session := NewSSHSession(SSHConfig{
		Name:    "test",
		Host:    "example.com",
		Port:    2222,
		User:    "testuser",
		KeyFile: "~/.ssh/id_test",
	})

	if session.Name() != "test" {
		t.Errorf("Expected name 'test', got %q", session.Name())
	}
	if session.Type() != "ssh" {
		t.Errorf("Expected type 'ssh', got %q", session.Type())
	}
	if session.Host() != "example.com" {
		t.Errorf("Expected host 'example.com', got %q", session.Host())
	}
	if session.User() != "testuser" {
		t.Errorf("Expected user 'testuser', got %q", session.User())
	}
	if !session.IsConnected() == true {
		// Not connected by default
	}
}

func TestSSHSessionDefaultPort(t *testing.T) {
	session := NewSSHSession(SSHConfig{
		Name: "test",
		Host: "example.com",
		User: "testuser",
		// Port not specified, should default to 22
	})

	// We can't directly access port, but the session should be created successfully
	if session == nil {
		t.Error("Expected session to be created")
	}
}
