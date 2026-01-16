package session

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/state"
)

func createTestManager(t *testing.T) (*Manager, string) {
	t.Helper()

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
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

	stateMgr := state.NewManager(statePath)
	stateMgr.Load()

	mgr := NewManager(cfg, stateMgr)
	return mgr, tmpDir
}

func TestNewManager(t *testing.T) {
	mgr, _ := createTestManager(t)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// Should have local and testserver sessions
	if !mgr.HasSession("local") {
		t.Error("expected 'local' session")
	}

	if !mgr.HasSession("testserver") {
		t.Error("expected 'testserver' session")
	}

	// Default active should be local
	if mgr.GetActiveSessionName() != "local" {
		t.Errorf("expected active session 'local', got '%s'", mgr.GetActiveSessionName())
	}
}

func TestGetSession(t *testing.T) {
	mgr, _ := createTestManager(t)

	// Get existing session
	session, ok := mgr.GetSession("local")
	if !ok {
		t.Fatal("expected to find 'local' session")
	}

	if session.Type() != "local" {
		t.Errorf("expected type 'local', got '%s'", session.Type())
	}

	// Get non-existing session
	_, ok = mgr.GetSession("nonexistent")
	if ok {
		t.Error("expected not to find 'nonexistent' session")
	}
}

func TestGetActiveSession(t *testing.T) {
	mgr, _ := createTestManager(t)

	session := mgr.GetActiveSession()
	if session == nil {
		t.Fatal("GetActiveSession returned nil")
	}

	if session.Name() != "local" {
		t.Errorf("expected active session name 'local', got '%s'", session.Name())
	}
}

func TestSetActiveSession(t *testing.T) {
	mgr, _ := createTestManager(t)

	// Set to existing session
	if err := mgr.SetActiveSession("testserver"); err != nil {
		t.Fatalf("SetActiveSession failed: %v", err)
	}

	if mgr.GetActiveSessionName() != "testserver" {
		t.Errorf("expected 'testserver', got '%s'", mgr.GetActiveSessionName())
	}

	// Set to non-existing session
	err := mgr.SetActiveSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	sessionErr, ok := err.(*Error)
	if !ok {
		t.Errorf("expected *Error, got %T", err)
	}

	if sessionErr.Code != ErrSessionNotFound {
		t.Errorf("expected code %s, got %s", ErrSessionNotFound, sessionErr.Code)
	}
}

func TestExecute(t *testing.T) {
	mgr, _ := createTestManager(t)

	// Execute on local session
	result, err := mgr.Execute("echo hello")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Errorf("expected 'hello', got '%s'", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestExecuteOn(t *testing.T) {
	mgr, _ := createTestManager(t)

	// Execute on specific session
	result, err := mgr.ExecuteOn("local", "echo test")
	if err != nil {
		t.Fatalf("ExecuteOn failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "test" {
		t.Errorf("expected 'test', got '%s'", result.Stdout)
	}

	// Execute on non-existing session
	_, err = mgr.ExecuteOn("nonexistent", "echo test")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	mgr, _ := createTestManager(t)

	sessions := mgr.ListSessions()

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Find local session
	var localInfo *SessionInfo
	for i := range sessions {
		if sessions[i].Name == "local" {
			localInfo = &sessions[i]
			break
		}
	}

	if localInfo == nil {
		t.Fatal("expected to find 'local' in list")
	}

	if localInfo.Type != "local" {
		t.Errorf("expected type 'local', got '%s'", localInfo.Type)
	}

	if !localInfo.Active {
		t.Error("expected 'local' to be active")
	}

	// Find testserver session
	var sshInfo *SessionInfo
	for i := range sessions {
		if sessions[i].Name == "testserver" {
			sshInfo = &sessions[i]
			break
		}
	}

	if sshInfo == nil {
		t.Fatal("expected to find 'testserver' in list")
	}

	if sshInfo.Type != "ssh" {
		t.Errorf("expected type 'ssh', got '%s'", sshInfo.Type)
	}

	if sshInfo.Host != "example.com" {
		t.Errorf("expected host 'example.com', got '%s'", sshInfo.Host)
	}

	if sshInfo.User != "testuser" {
		t.Errorf("expected user 'testuser', got '%s'", sshInfo.User)
	}
}

func TestSessionNames(t *testing.T) {
	mgr, _ := createTestManager(t)

	names := mgr.SessionNames()

	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}

	hasLocal := false
	hasTestserver := false
	for _, name := range names {
		if name == "local" {
			hasLocal = true
		}
		if name == "testserver" {
			hasTestserver = true
		}
	}

	if !hasLocal {
		t.Error("expected 'local' in names")
	}

	if !hasTestserver {
		t.Error("expected 'testserver' in names")
	}
}

func TestHasSession(t *testing.T) {
	mgr, _ := createTestManager(t)

	if !mgr.HasSession("local") {
		t.Error("expected HasSession('local') to be true")
	}

	if !mgr.HasSession("testserver") {
		t.Error("expected HasSession('testserver') to be true")
	}

	if mgr.HasSession("nonexistent") {
		t.Error("expected HasSession('nonexistent') to be false")
	}
}

func TestConnectDisconnect(t *testing.T) {
	mgr, _ := createTestManager(t)

	// Connect local (should be no-op)
	if err := mgr.Connect("local"); err != nil {
		t.Errorf("Connect local should not error: %v", err)
	}

	// Disconnect local
	if err := mgr.Disconnect("local"); err != nil {
		t.Errorf("Disconnect local should not error: %v", err)
	}

	// Connect non-existing
	err := mgr.Connect("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	// Disconnect non-existing
	err = mgr.Disconnect("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestManagerWithNilState(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type: "local",
			},
		},
	}

	// Create manager with nil state manager
	mgr := NewManager(cfg, nil)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// Operations should still work
	result, err := mgr.Execute("echo test")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "test" {
		t.Errorf("expected 'test', got '%s'", result.Stdout)
	}

	// SetActiveSession should work without state persistence
	if err := mgr.SetActiveSession("local"); err != nil {
		t.Errorf("SetActiveSession failed: %v", err)
	}
}
