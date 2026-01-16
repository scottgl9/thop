package session

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/sshconfig"
	"github.com/scottgl9/thop/internal/state"
)

// Manager manages all sessions
type Manager struct {
	sessions          map[string]Session
	activeSession     string
	config            *config.Config
	state             *state.Manager
	sshConfig         *sshconfig.Config
	commandTimeout    time.Duration
	reconnectAttempts int
	reconnectBackoff  time.Duration
	mu                sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(cfg *config.Config, stateMgr *state.Manager) *Manager {
	// Load SSH config from ~/.ssh/config
	sshCfg, _ := sshconfig.Load()

	// Calculate command timeout from config (in seconds)
	timeout := time.Duration(cfg.Settings.CommandTimeout) * time.Second
	if timeout == 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}

	// Reconnection settings
	reconnectAttempts := cfg.Settings.ReconnectAttempts
	if reconnectAttempts == 0 {
		reconnectAttempts = 3 // Default 3 attempts
	}

	reconnectBackoff := time.Duration(cfg.Settings.ReconnectBackoff) * time.Second
	if reconnectBackoff == 0 {
		reconnectBackoff = 2 * time.Second // Default 2 seconds base backoff
	}

	m := &Manager{
		sessions:          make(map[string]Session),
		activeSession:     cfg.Settings.DefaultSession,
		config:            cfg,
		state:             stateMgr,
		sshConfig:         sshCfg,
		commandTimeout:    timeout,
		reconnectAttempts: reconnectAttempts,
		reconnectBackoff:  reconnectBackoff,
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
		// Resolve SSH settings from ~/.ssh/config if not specified in thop config
		host := cfg.Host
		user := cfg.User
		port := cfg.Port
		keyFile := cfg.IdentityFile

		if m.sshConfig != nil {
			// Use host alias to look up in SSH config
			alias := host
			if alias == "" {
				alias = name // Use session name as alias if no host specified
			}

			// Resolve hostname from SSH config
			if host == "" {
				host = m.sshConfig.ResolveHost(alias)
			} else {
				// Host was specified, but might be an alias
				resolved := m.sshConfig.ResolveHost(host)
				if resolved != host {
					host = resolved
				}
			}

			// Resolve user from SSH config if not specified
			if user == "" {
				user = m.sshConfig.ResolveUser(alias)
			}

			// Resolve port from SSH config if not specified
			if port == 0 {
				portStr := m.sshConfig.ResolvePort(alias)
				if portStr != "22" {
					if p, err := strconv.Atoi(portStr); err == nil {
						port = p
					}
				}
			}

			// Resolve identity file from SSH config if not specified
			if keyFile == "" {
				keyFile = m.sshConfig.ResolveIdentityFile(alias)
			}
		}

		session := NewSSHSession(SSHConfig{
			Name:    name,
			Host:    host,
			Port:    port,
			User:    user,
			KeyFile: keyFile,
			Timeout: m.commandTimeout,
		})
		return session
	default:
		session := NewLocalSession(name, cfg.Shell)
		session.SetTimeout(m.commandTimeout)
		return session
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

	// For SSH sessions, try to reconnect on connection errors
	if err != nil && session.Type() == "ssh" {
		if sessionErr, ok := err.(*Error); ok && sessionErr.Retryable {
			if sessionErr.Code == ErrSessionDisconnected || sessionErr.Code == ErrConnectionFailed {
				// Attempt reconnection
				if reconnectErr := m.attemptReconnect(session); reconnectErr == nil {
					// Retry the command after successful reconnection
					result, err = session.Execute(cmd)
				}
			}
		}
	}

	// Update cwd in state if successful
	if err == nil && m.state != nil {
		m.state.SetSessionCWD(session.Name(), session.GetCWD())
	}

	return result, err
}

// attemptReconnect attempts to reconnect an SSH session with exponential backoff
func (m *Manager) attemptReconnect(session Session) error {
	sshSession, ok := session.(*SSHSession)
	if !ok {
		return fmt.Errorf("not an SSH session")
	}

	var lastErr error
	backoff := m.reconnectBackoff

	for attempt := 1; attempt <= m.reconnectAttempts; attempt++ {
		// Wait before retry (except first attempt)
		if attempt > 1 {
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		// Attempt reconnection
		if err := sshSession.Reconnect(); err != nil {
			lastErr = err
			continue
		}

		// Update state on successful reconnection
		if m.state != nil {
			m.state.SetSessionConnected(session.Name(), true)
		}

		return nil
	}

	return &Error{
		Code:      ErrConnectionFailed,
		Message:   fmt.Sprintf("Failed to reconnect after %d attempts: %v", m.reconnectAttempts, lastErr),
		Session:   session.Name(),
		Retryable: false,
	}
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

// SSHConfigHosts returns all hosts defined in ~/.ssh/config
func (m *Manager) SSHConfigHosts() []string {
	if m.sshConfig == nil {
		return nil
	}
	return m.sshConfig.ListHosts()
}

// HasSSHConfigHost returns true if a host is defined in ~/.ssh/config
func (m *Manager) HasSSHConfigHost(name string) bool {
	if m.sshConfig == nil {
		return false
	}
	return m.sshConfig.GetHost(name) != nil
}
