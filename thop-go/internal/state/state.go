package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// State represents the shared application state
type State struct {
	ActiveSession string                  `json:"active_session"`
	Sessions      map[string]SessionState `json:"sessions"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

// SessionState represents the state of a single session
type SessionState struct {
	Type      string            `json:"type"`
	Connected bool              `json:"connected"`
	CWD       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
}

// Manager handles state persistence
type Manager struct {
	path  string
	mu    sync.Mutex
	state *State
}

// NewManager creates a new state manager
func NewManager(path string) *Manager {
	return &Manager{
		path: path,
		state: &State{
			ActiveSession: "local",
			Sessions:      make(map[string]SessionState),
			UpdatedAt:     time.Now(),
		},
	}
}

// Load loads state from disk
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		// Initialize with defaults
		m.state = &State{
			ActiveSession: "local",
			Sessions: map[string]SessionState{
				"local": {
					Type:      "local",
					Connected: true,
					CWD:       getCurrentDir(),
					Env:       make(map[string]string),
				},
			},
			UpdatedAt: time.Now(),
		}
		return m.saveWithLock()
	}

	// Read file with lock
	data, err := m.readWithLock()
	if err != nil {
		return err
	}

	// Parse JSON
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	m.state = &state
	return nil
}

// Save saves state to disk
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveWithLock()
}

// saveWithLock saves state (caller must hold lock)
func (m *Manager) saveWithLock() error {
	m.state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return m.writeWithLock(data)
}

// readWithLock reads state file with file locking
func (m *Manager) readWithLock() ([]byte, error) {
	file, err := os.OpenFile(m.path, os.O_RDONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	// Acquire shared lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to acquire read lock: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return os.ReadFile(m.path)
}

// writeWithLock writes state file with file locking
func (m *Manager) writeWithLock(data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	file, err := os.OpenFile(m.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open state file for writing: %w", err)
	}
	defer file.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GetActiveSession returns the active session name
func (m *Manager) GetActiveSession() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.ActiveSession
}

// SetActiveSession sets the active session
func (m *Manager) SetActiveSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.ActiveSession = name
	return m.saveWithLock()
}

// GetSessionState returns the state for a session
func (m *Manager) GetSessionState(name string) (*SessionState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.state.Sessions[name]
	if !ok {
		return nil, false
	}
	return &state, true
}

// UpdateSessionState updates the state for a session
func (m *Manager) UpdateSessionState(name string, state SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Sessions[name] = state
	return m.saveWithLock()
}

// SetSessionConnected sets the connected status for a session
func (m *Manager) SetSessionConnected(name string, connected bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.state.Sessions[name]; ok {
		state.Connected = connected
		m.state.Sessions[name] = state
	} else {
		m.state.Sessions[name] = SessionState{
			Connected: connected,
			Env:       make(map[string]string),
		}
	}

	return m.saveWithLock()
}

// SetSessionCWD sets the current working directory for a session
func (m *Manager) SetSessionCWD(name string, cwd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.state.Sessions[name]; ok {
		state.CWD = cwd
		m.state.Sessions[name] = state
	} else {
		m.state.Sessions[name] = SessionState{
			CWD: cwd,
			Env: make(map[string]string),
		}
	}

	return m.saveWithLock()
}

// GetAllSessions returns all session states
func (m *Manager) GetAllSessions() map[string]SessionState {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy
	sessions := make(map[string]SessionState, len(m.state.Sessions))
	for k, v := range m.state.Sessions {
		sessions[k] = v
	}
	return sessions
}

// SetSessionEnv sets an environment variable for a session
func (m *Manager) SetSessionEnv(name, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.state.Sessions[name]; ok {
		if state.Env == nil {
			state.Env = make(map[string]string)
		}
		state.Env[key] = value
		m.state.Sessions[name] = state
	} else {
		m.state.Sessions[name] = SessionState{
			Env: map[string]string{key: value},
		}
	}

	return m.saveWithLock()
}

// GetSessionEnv returns the environment variables for a session
func (m *Manager) GetSessionEnv(name string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.state.Sessions[name]; ok && state.Env != nil {
		// Return a copy
		env := make(map[string]string, len(state.Env))
		for k, v := range state.Env {
			env[k] = v
		}
		return env
	}
	return make(map[string]string)
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		return home
	}
	return dir
}
