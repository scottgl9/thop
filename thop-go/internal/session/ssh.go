package session

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHSession represents an SSH session to a remote host
type SSHSession struct {
	name       string
	host       string
	port       int
	user       string
	keyFile    string
	client     *ssh.Client
	cwd        string
	env        map[string]string
	connected  bool
	timeout    time.Duration
}

// SSHConfig contains SSH session configuration
type SSHConfig struct {
	Name     string
	Host     string
	Port     int
	User     string
	KeyFile  string
	Password string // Optional, for auth command
	Timeout  time.Duration
}

// NewSSHSession creates a new SSH session
func NewSSHSession(cfg SSHConfig) *SSHSession {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &SSHSession{
		name:    cfg.Name,
		host:    cfg.Host,
		port:    cfg.Port,
		user:    cfg.User,
		keyFile: cfg.KeyFile,
		env:     make(map[string]string),
		timeout: cfg.Timeout,
	}
}

// Name returns the session name
func (s *SSHSession) Name() string {
	return s.name
}

// Type returns the session type
func (s *SSHSession) Type() string {
	return "ssh"
}

// Connect establishes the SSH connection
func (s *SSHSession) Connect() error {
	if s.connected && s.client != nil {
		return nil
	}

	// Build auth methods
	authMethods, err := s.getAuthMethods()
	if err != nil {
		return err
	}

	if len(authMethods) == 0 {
		return &Error{
			Code:       ErrAuthPasswordRequired,
			Message:    fmt.Sprintf("No authentication methods available for %s", s.name),
			Session:    s.name,
			Host:       s.host,
			Suggestion: fmt.Sprintf("Use /auth %s to provide credentials", s.name),
		}
	}

	// Get host key callback
	hostKeyCallback, err := s.getHostKeyCallback()
	if err != nil {
		return err
	}

	// Create SSH config
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         s.timeout,
	}

	// Connect
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return s.wrapConnectionError(err)
	}

	s.client = client
	s.connected = true

	// Get initial working directory
	result, err := s.executeRaw("pwd")
	if err == nil && result.ExitCode == 0 {
		s.cwd = strings.TrimSpace(result.Stdout)
	} else {
		s.cwd = "~"
	}

	return nil
}

// Disconnect closes the SSH connection
func (s *SSHSession) Disconnect() error {
	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		s.connected = false
		return err
	}
	return nil
}

// IsConnected returns true if connected
func (s *SSHSession) IsConnected() bool {
	return s.connected && s.client != nil
}

// Execute runs a command over SSH
func (s *SSHSession) Execute(cmdStr string) (*ExecuteResult, error) {
	if !s.IsConnected() {
		return nil, &Error{
			Code:       ErrSessionDisconnected,
			Message:    fmt.Sprintf("Session %s is not connected", s.name),
			Session:    s.name,
			Retryable:  true,
			Suggestion: fmt.Sprintf("Use /connect %s to reconnect", s.name),
		}
	}

	// Handle cd commands specially
	if strings.HasPrefix(strings.TrimSpace(cmdStr), "cd ") {
		return s.handleCD(cmdStr)
	}

	// Prepend cd to cwd if set
	if s.cwd != "" && s.cwd != "~" {
		cmdStr = fmt.Sprintf("cd %s && %s", s.cwd, cmdStr)
	}

	return s.executeRaw(cmdStr)
}

// executeRaw executes a command without cwd handling
func (s *SSHSession) executeRaw(cmdStr string) (*ExecuteResult, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set environment variables
	for k, v := range s.env {
		session.Setenv(k, v)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Run command
	err = session.Run(cmdStr)

	result := &ExecuteResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			return nil, err
		}
	}

	return result, nil
}

// handleCD handles cd commands
func (s *SSHSession) handleCD(cmdStr string) (*ExecuteResult, error) {
	parts := strings.Fields(cmdStr)
	var targetDir string

	if len(parts) == 1 {
		targetDir = "~"
	} else {
		targetDir = parts[1]
	}

	// Execute cd and pwd to get the actual path
	fullCmd := fmt.Sprintf("cd %s && pwd", targetDir)
	if s.cwd != "" && s.cwd != "~" && !strings.HasPrefix(targetDir, "/") && !strings.HasPrefix(targetDir, "~") {
		fullCmd = fmt.Sprintf("cd %s && cd %s && pwd", s.cwd, targetDir)
	}

	result, err := s.executeRaw(fullCmd)
	if err != nil {
		return nil, err
	}

	if result.ExitCode == 0 {
		s.cwd = strings.TrimSpace(result.Stdout)
		result.Stdout = "" // Don't show pwd output for cd
	}

	return result, nil
}

// GetCWD returns the current working directory
func (s *SSHSession) GetCWD() string {
	return s.cwd
}

// SetCWD sets the current working directory
func (s *SSHSession) SetCWD(path string) error {
	s.cwd = path
	return nil
}

// GetEnv returns the environment variables
func (s *SSHSession) GetEnv() map[string]string {
	env := make(map[string]string, len(s.env))
	for k, v := range s.env {
		env[k] = v
	}
	return env
}

// SetEnv sets an environment variable
func (s *SSHSession) SetEnv(key, value string) {
	s.env[key] = value
}

// getAuthMethods returns available authentication methods
func (s *SSHSession) getAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try SSH agent first
	if agentAuth := s.getAgentAuth(); agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	// Try key file
	if s.keyFile != "" {
		if keyAuth, err := s.getKeyAuth(s.keyFile); err == nil {
			methods = append(methods, keyAuth)
		}
	}

	// Try default key files
	home, _ := os.UserHomeDir()
	defaultKeys := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}

	for _, keyPath := range defaultKeys {
		if keyPath == s.keyFile {
			continue // Already tried
		}
		if _, err := os.Stat(keyPath); err == nil {
			if keyAuth, err := s.getKeyAuth(keyPath); err == nil {
				methods = append(methods, keyAuth)
			}
		}
	}

	return methods, nil
}

// getAgentAuth returns SSH agent authentication if available
func (s *SSHSession) getAgentAuth() ssh.AuthMethod {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers)
}

// getKeyAuth returns key file authentication
func (s *SSHSession) getKeyAuth(keyPath string) (ssh.AuthMethod, error) {
	// Expand ~ in path
	if strings.HasPrefix(keyPath, "~") {
		home, _ := os.UserHomeDir()
		keyPath = strings.Replace(keyPath, "~", home, 1)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		// Key might be encrypted
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

// getHostKeyCallback returns the host key callback
func (s *SSHSession) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	// Check if known_hosts exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		// Create empty known_hosts
		dir := filepath.Dir(knownHostsPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		if _, err := os.Create(knownHostsPath); err != nil {
			return nil, err
		}
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, err
	}

	// Wrap to provide better error messages
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err != nil {
			if strings.Contains(err.Error(), "knownhosts: key is unknown") {
				return &Error{
					Code:       ErrHostKeyVerification,
					Message:    fmt.Sprintf("Host key verification failed for %s", hostname),
					Session:    s.name,
					Host:       s.host,
					Suggestion: fmt.Sprintf("Use /trust %s to add the host key", s.name),
				}
			}
			if strings.Contains(err.Error(), "key mismatch") {
				return &Error{
					Code:    ErrHostKeyChanged,
					Message: fmt.Sprintf("Host key has changed for %s. This could indicate a MITM attack.", hostname),
					Session: s.name,
					Host:    s.host,
				}
			}
		}
		return err
	}, nil
}

// wrapConnectionError wraps connection errors with more context
func (s *SSHSession) wrapConnectionError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "connection refused") {
		return &Error{
			Code:      ErrConnectionFailed,
			Message:   fmt.Sprintf("Connection refused to %s:%d", s.host, s.port),
			Session:   s.name,
			Host:      s.host,
			Retryable: true,
		}
	}

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return &Error{
			Code:      ErrConnectionTimeout,
			Message:   fmt.Sprintf("Connection timed out to %s:%d", s.host, s.port),
			Session:   s.name,
			Host:      s.host,
			Retryable: true,
		}
	}

	if strings.Contains(errStr, "unable to authenticate") || strings.Contains(errStr, "no supported methods") {
		return &Error{
			Code:       ErrAuthPasswordRequired,
			Message:    fmt.Sprintf("Authentication failed for %s@%s", s.user, s.host),
			Session:    s.name,
			Host:       s.host,
			Suggestion: fmt.Sprintf("Use /auth %s to provide credentials", s.name),
		}
	}

	return &Error{
		Code:      ErrConnectionFailed,
		Message:   fmt.Sprintf("Failed to connect to %s: %s", s.host, err.Error()),
		Session:   s.name,
		Host:      s.host,
		Retryable: true,
	}
}

// Host returns the SSH host
func (s *SSHSession) Host() string {
	return s.host
}

// Port returns the SSH port
func (s *SSHSession) Port() int {
	return s.port
}

// User returns the SSH user
func (s *SSHSession) User() string {
	return s.user
}
