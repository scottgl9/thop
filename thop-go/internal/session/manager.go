package session

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/logger"
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

	logger.Debug("session manager initialized with %d sessions, timeout=%v", len(m.sessions), timeout)

	// Load state (active session and cwd for each session)
	if stateMgr != nil {
		// Restore active session
		active := stateMgr.GetActiveSession()
		if _, ok := m.sessions[active]; ok {
			m.activeSession = active
		}

		// Restore cwd for each session from state
		for name, sess := range m.sessions {
			if sessionState, ok := stateMgr.GetSessionState(name); ok && sessionState.CWD != "" {
				if err := sess.SetCWD(sessionState.CWD); err != nil {
					logger.Debug("failed to restore cwd for session %q: %v", name, err)
				} else {
					logger.Debug("restored cwd for session %q: %s", name, sessionState.CWD)
				}
			}
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
		jumpHost := cfg.JumpHost
		agentForwarding := cfg.AgentForwarding

		// Use host alias to look up in SSH config
		alias := host
		if alias == "" {
			alias = name // Use session name as alias if no host specified
		}

		if m.sshConfig != nil {
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

			// Resolve jump host from SSH config if not specified
			if jumpHost == "" {
				jumpHost = m.sshConfig.ResolveProxyJump(alias)
			}

			// Resolve agent forwarding from SSH config if not specified in thop config
			if !agentForwarding {
				agentForwarding = m.sshConfig.ResolveForwardAgent(alias)
			}
		}

		session := NewSSHSession(SSHConfig{
			Name:            name,
			Host:            host,
			Port:            port,
			User:            user,
			KeyFile:         keyFile,
			PasswordEnv:     cfg.PasswordEnv,
			PasswordFile:    cfg.PasswordFile,
			JumpHost:        jumpHost,
			AgentForwarding: agentForwarding,
			Timeout:         m.commandTimeout,
			StartupCommands: cfg.StartupCommands,
		})
		if jumpHost != "" {
			logger.Debug("created SSH session %q: user=%s host=%s port=%d via jump_host=%s, startup_commands=%d", name, user, host, port, jumpHost, len(cfg.StartupCommands))
		} else {
			logger.Debug("created SSH session %q: user=%s host=%s port=%d, startup_commands=%d", name, user, host, port, len(cfg.StartupCommands))
		}
		return session
	default:
		session := NewLocalSession(name, cfg.Shell)
		session.SetTimeout(m.commandTimeout)
		if len(cfg.StartupCommands) > 0 {
			session.SetStartupCommands(cfg.StartupCommands)
		}
		logger.Debug("created local session %q: shell=%s, startup_commands=%d", name, cfg.Shell, len(cfg.StartupCommands))
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
		logger.Warn("set active session failed: session %q not found", name)
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	logger.Info("switching active session from %q to %q", m.activeSession, name)
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
		logger.Warn("connect failed: session %q not found", name)
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	logger.Info("connecting to session %q", name)
	err := session.Connect()
	if err != nil {
		logger.Error("connect failed for session %q: %v", name, err)
		return err
	}

	// Update state
	if m.state != nil {
		m.state.SetSessionConnected(name, true)
	}

	// Restore environment from state for SSH sessions
	if sshSession, ok := session.(*SSHSession); ok {
		m.restoreSessionEnv(sshSession)
	}

	logger.Info("connected to session %q", name)
	return nil
}

// Disconnect disconnects a session by name
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	session, ok := m.sessions[name]
	m.mu.Unlock()

	if !ok {
		logger.Warn("disconnect failed: session %q not found", name)
		return &Error{
			Code:    ErrSessionNotFound,
			Message: fmt.Sprintf("Session '%s' not found", name),
			Session: name,
		}
	}

	logger.Info("disconnecting from session %q", name)
	err := session.Disconnect()

	// Update state
	if m.state != nil {
		m.state.SetSessionConnected(name, false)
	}

	if err != nil {
		logger.Warn("disconnect error for session %q: %v", name, err)
	} else {
		logger.Info("disconnected from session %q", name)
	}

	return err
}

// Execute executes a command on the active session
func (m *Manager) Execute(cmd string) (*ExecuteResult, error) {
	return m.ExecuteWithContext(context.Background(), cmd)
}

// ExecuteWithContext executes a command on the active session with cancellation support
func (m *Manager) ExecuteWithContext(ctx context.Context, cmd string) (*ExecuteResult, error) {
	session := m.GetActiveSession()
	if session == nil {
		logger.Warn("execute failed: no active session")
		return nil, &Error{
			Code:    ErrSessionNotFound,
			Message: "No active session",
		}
	}

	logger.Debug("executing on session %q: %s", session.Name(), cmd)
	result, err := session.ExecuteWithContext(ctx, cmd)

	// For SSH sessions, try to reconnect on connection errors (but not if context was cancelled)
	if err != nil && session.Type() == "ssh" && ctx.Err() == nil {
		if sessionErr, ok := err.(*Error); ok && sessionErr.Retryable {
			if sessionErr.Code == ErrSessionDisconnected || sessionErr.Code == ErrConnectionFailed {
				logger.Info("connection lost on session %q, attempting reconnect", session.Name())
				// Attempt reconnection
				if reconnectErr := m.attemptReconnect(session); reconnectErr == nil {
					// Retry the command after successful reconnection
					logger.Debug("retrying command after reconnect: %s", cmd)
					result, err = session.ExecuteWithContext(ctx, cmd)
				}
			}
		}
	}

	// Update cwd in state if successful
	if err == nil && m.state != nil {
		m.state.SetSessionCWD(session.Name(), session.GetCWD())
	}

	if err != nil {
		logger.Debug("execute failed on session %q: %v", session.Name(), err)
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

	logger.Info("starting reconnection attempts for session %q (max %d attempts)", session.Name(), m.reconnectAttempts)

	for attempt := 1; attempt <= m.reconnectAttempts; attempt++ {
		// Wait before retry (except first attempt)
		if attempt > 1 {
			logger.Debug("reconnect attempt %d/%d for session %q, waiting %v", attempt, m.reconnectAttempts, session.Name(), backoff)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		// Attempt reconnection
		if err := sshSession.Reconnect(); err != nil {
			lastErr = err
			logger.Warn("reconnect attempt %d/%d failed for session %q: %v", attempt, m.reconnectAttempts, session.Name(), err)
			continue
		}

		// Update state on successful reconnection
		if m.state != nil {
			m.state.SetSessionConnected(session.Name(), true)
		}

		// Restore environment from state
		m.restoreSessionEnv(sshSession)

		logger.Info("reconnected to session %q after %d attempt(s)", session.Name(), attempt)
		return nil
	}

	logger.Error("failed to reconnect to session %q after %d attempts", session.Name(), m.reconnectAttempts)
	return &Error{
		Code:      ErrConnectionFailed,
		Message:   fmt.Sprintf("Failed to reconnect after %d attempts: %v", m.reconnectAttempts, lastErr),
		Session:   session.Name(),
		Retryable: false,
	}
}

// restoreSessionEnv restores environment variables from state after reconnect
func (m *Manager) restoreSessionEnv(session *SSHSession) {
	if m.state == nil {
		return
	}

	env := m.state.GetSessionEnv(session.Name())
	if len(env) > 0 {
		session.RestoreEnv(env)
	}
}

// SetSessionEnv sets and persists an environment variable for the active session
func (m *Manager) SetSessionEnv(key, value string) error {
	session := m.GetActiveSession()
	if session == nil {
		return &Error{
			Code:    ErrSessionNotFound,
			Message: "No active session",
		}
	}

	session.SetEnv(key, value)

	// Persist to state
	if m.state != nil {
		m.state.SetSessionEnv(session.Name(), key, value)
	}

	logger.Debug("set environment %s=%s on session %q", key, value, session.Name())
	return nil
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

// AddSession adds a new session to the manager
func (m *Manager) AddSession(name string, cfg config.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[name]; exists {
		return fmt.Errorf("session '%s' already exists", name)
	}

	m.sessions[name] = m.createSession(name, cfg)
	logger.Info("added new session %q", name)
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *config.Config {
	return m.config
}
