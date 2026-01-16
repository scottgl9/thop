package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.GetActiveSession() != "local" {
		t.Errorf("expected active session 'local', got '%s'", mgr.GetActiveSession())
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "subdir", "state.json")

	mgr := NewManager(statePath)

	// Load should create file if not exists
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// File should exist now
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state file was not created")
	}

	// Check file permissions
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// Should be readable/writable by owner only (0600)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

func TestSetAndGetActiveSession(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Set active session
	if err := mgr.SetActiveSession("prod"); err != nil {
		t.Fatalf("SetActiveSession failed: %v", err)
	}

	// Get should return the new value
	if mgr.GetActiveSession() != "prod" {
		t.Errorf("expected 'prod', got '%s'", mgr.GetActiveSession())
	}

	// Create new manager to verify persistence
	mgr2 := NewManager(statePath)
	if err := mgr2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if mgr2.GetActiveSession() != "prod" {
		t.Errorf("expected persisted 'prod', got '%s'", mgr2.GetActiveSession())
	}
}

func TestSessionState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Update session state
	state := SessionState{
		Type:      "ssh",
		Connected: true,
		CWD:       "/var/www",
		Env:       map[string]string{"RAILS_ENV": "production"},
	}

	if err := mgr.UpdateSessionState("prod", state); err != nil {
		t.Fatalf("UpdateSessionState failed: %v", err)
	}

	// Get session state
	retrieved, ok := mgr.GetSessionState("prod")
	if !ok {
		t.Fatal("expected to find 'prod' session state")
	}

	if retrieved.Type != "ssh" {
		t.Errorf("expected type 'ssh', got '%s'", retrieved.Type)
	}

	if !retrieved.Connected {
		t.Error("expected connected to be true")
	}

	if retrieved.CWD != "/var/www" {
		t.Errorf("expected cwd '/var/www', got '%s'", retrieved.CWD)
	}

	if retrieved.Env["RAILS_ENV"] != "production" {
		t.Errorf("expected RAILS_ENV 'production', got '%s'", retrieved.Env["RAILS_ENV"])
	}

	// Get non-existent session
	_, ok = mgr.GetSessionState("nonexistent")
	if ok {
		t.Error("expected not to find 'nonexistent' session")
	}
}

func TestSetSessionConnected(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Set connected for new session
	if err := mgr.SetSessionConnected("test", true); err != nil {
		t.Fatalf("SetSessionConnected failed: %v", err)
	}

	state, ok := mgr.GetSessionState("test")
	if !ok {
		t.Fatal("expected to find 'test' session")
	}

	if !state.Connected {
		t.Error("expected connected to be true")
	}

	// Set disconnected
	if err := mgr.SetSessionConnected("test", false); err != nil {
		t.Fatalf("SetSessionConnected failed: %v", err)
	}

	state, _ = mgr.GetSessionState("test")
	if state.Connected {
		t.Error("expected connected to be false")
	}
}

func TestSetSessionCWD(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Set CWD for new session
	if err := mgr.SetSessionCWD("test", "/home/user"); err != nil {
		t.Fatalf("SetSessionCWD failed: %v", err)
	}

	state, ok := mgr.GetSessionState("test")
	if !ok {
		t.Fatal("expected to find 'test' session")
	}

	if state.CWD != "/home/user" {
		t.Errorf("expected cwd '/home/user', got '%s'", state.CWD)
	}

	// Update CWD
	if err := mgr.SetSessionCWD("test", "/tmp"); err != nil {
		t.Fatalf("SetSessionCWD failed: %v", err)
	}

	state, _ = mgr.GetSessionState("test")
	if state.CWD != "/tmp" {
		t.Errorf("expected cwd '/tmp', got '%s'", state.CWD)
	}
}

func TestGetAllSessions(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Add some sessions
	mgr.SetSessionConnected("session1", true)
	mgr.SetSessionConnected("session2", false)

	sessions := mgr.GetAllSessions()

	// Should have local + 2 new sessions
	if len(sessions) < 2 {
		t.Errorf("expected at least 2 sessions, got %d", len(sessions))
	}

	// Verify it's a copy (modifying shouldn't affect original)
	sessions["new"] = SessionState{Connected: true}

	sessions2 := mgr.GetAllSessions()
	if _, ok := sessions2["new"]; ok {
		t.Error("GetAllSessions should return a copy")
	}
}

func TestStateFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	mgr.SetActiveSession("prod")
	mgr.UpdateSessionState("prod", SessionState{
		Type:      "ssh",
		Connected: true,
		CWD:       "/var/www",
		Env:       map[string]string{"KEY": "value"},
	})

	// Read and verify JSON format
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse state JSON: %v", err)
	}

	if state.ActiveSession != "prod" {
		t.Errorf("expected active_session 'prod', got '%s'", state.ActiveSession)
	}

	if state.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	mgr := NewManager(statePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			mgr.SetSessionCWD("test", "/path"+string(rune('0'+n)))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or corrupt data
	state, ok := mgr.GetSessionState("test")
	if !ok {
		t.Fatal("expected to find 'test' session")
	}

	if state.CWD == "" {
		t.Error("expected CWD to be set")
	}
}
