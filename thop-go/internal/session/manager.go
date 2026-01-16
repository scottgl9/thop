package session

import (
	"fmt"
	"sync"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/state"
)

// Manager manages all sessions
type Manager struct {
	sessions      map[string]Session
	activeSession string
	config        *config.Config
	state         *state.Manager
	mu            sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(cfg *config.Config, stateMgr *state.Manager) *Manager {
	m := &Manager{
		sessions:      make(map[string]Session),
		activeSession: cfg.Settings.DefaultSession,
		config:        cfg,
		state:         stateMgr,
	}

	// Initialize sessions from config
	for name, sessionCfg := range cfg.Sessions {
		m.sessions[name] = m.createSession(name, sessionCfg)
	}

	// Load active session from state
	if stateMgr != nil {
		active := stateMgr.GetActiveSession()
		if _, ok := m.sessions[active]; ok {
			m.activeSession = active
		}
	}

	return m
}

// createSession creates a session from config
func (m *Manager) createSession(name string, cfg config.Session) Session {
	switch cfg.Type {
	case "ssh":
		return NewSSHSession(SSHConfig{
			Name:    name,
			Host:    cfg.Host,
			Port:    cfg.Port,
			User:    cfg.User,
			KeyFile: cfg.IdentityFile,
		})
	default:
		return NewLocalSession(name, cfg.Shell)
	}
}

// GetSession returns a session by name
func (m *Manager) GetSession(name string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[name]
	return session, ok
}

// GetActiveSession returns the currently active session
func (m *Manager) GetActiveSession() Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sessions[m.activeSession]
}

// GetActiveSessionName returns the name of the active session
func (m *Manager) GetActiveSessionName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeSession
}

// SetActiveSession sets the active session
func (m *Manager) SetActiveSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[name]; !ok {
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	m.activeSession = name

	// Persist to state file
	if m.state != nil {
		m.state.SetActiveSession(name)
	}

	return nil
}

// Connect connects a session by name
func (m *Manager) Connect(name string) error {
	m.mu.Lock()
	session, ok := m.sessions[name]
	m.mu.Unlock()

	if !ok {
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	err := session.Connect()
	if err != nil {
		return err
	}

	// Update state
	if m.state != nil {
		m.state.SetSessionConnected(name, true)
	}

	return nil
}

// Disconnect disconnects a session by name
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	session, ok := m.sessions[name]
	m.mu.Unlock()

	if !ok {
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	err := session.Disconnect()

	// Update state
	if m.state != nil {
		m.state.SetSessionConnected(name, false)
	}

	return err
}

// Execute executes a command on the active session
func (m *Manager) Execute(cmd string) (*ExecuteResult, error) {
	session := m.GetActiveSession()
	if session == nil {
		return nil, &Error{
			Code:    ErrSessionNotFound,
			Message: "No active session",
		}
	}

	result, err := session.Execute(cmd)

	// Update cwd in state if successful
	if err == nil && m.state != nil {
		m.state.SetSessionCWD(session.Name(), session.GetCWD())
	}

	return result, err
}

// ExecuteOn executes a command on a specific session
func (m *Manager) ExecuteOn(sessionName, cmd string) (*ExecuteResult, error) {
	session, ok := m.GetSession(sessionName)
	if !ok {
		return nil, &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", sessionName),
			Session: sessionName,
		}
	}

	return session.Execute(cmd)
}

// ListSessions returns information about all sessions
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []SessionInfo
	for name, session := range m.sessions {
		info := SessionInfo{
			Name:      name,
			Type:      session.Type(),
			Connected: session.IsConnected(),
			CWD:       session.GetCWD(),
			Active:    name == m.activeSession,
		}

		if sshSession, ok := session.(*SSHSession); ok {
			info.Host = sshSession.Host()
			info.User = sshSession.User()
		}

		sessions = append(sessions, info)
	}

	return sessions
}

// SessionInfo contains information about a session
type SessionInfo struct {
	Name      string
	Type      string
	Connected bool
	CWD       string
	Active    bool
	Host      string
	User      string
}

// SessionNames returns all session names
func (m *Manager) SessionNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	return names
}

// HasSession returns true if a session exists
func (m *Manager) HasSession(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.sessions[name]
	return ok
}
