package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/sftp"
	"github.com/scottgl9/thop/internal/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// SSHSession represents an SSH session to a remote host
type SSHSession struct {
	name                 string
	host                 string
	port                 int
	user                 string
	keyFile              string
	password             string // Password for authentication (set via /auth command)
	jumpHost             string // Jump host for ProxyJump (format: user@host:port or just host)
	agentForwarding      bool   // Whether to forward SSH agent to remote
	insecureIgnoreHostKey bool  // Skip host key verification (for testing only)
	client               *ssh.Client
	jumpClient           *ssh.Client // Jump host client (if using jump host)
	cwd                  string
	env                  map[string]string
	connected            bool
	connectTimeout       time.Duration
	commandTimeout       time.Duration
	startupCommands      []string
}

// SSHConfig contains SSH session configuration
type SSHConfig struct {
	Name                  string
	Host                  string
	Port                  int
	User                  string
	KeyFile               string
	Password              string        // Optional, for auth command
	PasswordEnv           string        // Environment variable containing password
	PasswordFile          string        // File containing password (must be 0600)
	JumpHost              string        // Jump host for ProxyJump (format: user@host:port or just host)
	AgentForwarding       bool          // Whether to forward SSH agent to remote
	InsecureIgnoreHostKey bool          // Skip host key verification (for testing only)
	ConnectTimeout        time.Duration // Connection timeout (default 30s)
	Timeout               time.Duration // Command timeout (default 300s)
	StartupCommands       []string      // Commands to run after connecting
}

// NewSSHSession creates a new SSH session
func NewSSHSession(cfg SSHConfig) *SSHSession {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 30 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300 * time.Second
	}

	// Resolve password from config, environment variable, or file
	password := cfg.Password
	if password == "" && cfg.PasswordEnv != "" {
		password = os.Getenv(cfg.PasswordEnv)
		if password != "" {
			logger.Debug("SSH session %q: password loaded from env var %s", cfg.Name, cfg.PasswordEnv)
		}
	}
	if password == "" && cfg.PasswordFile != "" {
		// Expand ~ in password file path
		pwFile := cfg.PasswordFile
		if strings.HasPrefix(pwFile, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				pwFile = filepath.Join(home, pwFile[2:])
			}
		}

		// Check file permissions (must be 0600 or stricter)
		if info, err := os.Stat(pwFile); err == nil {
			mode := info.Mode().Perm()
			if mode&0077 != 0 {
				logger.Warn("SSH session %q: password file %s has insecure permissions %o (should be 0600)", cfg.Name, pwFile, mode)
			} else {
				// Read password from file
				if data, err := os.ReadFile(pwFile); err == nil {
					password = strings.TrimSpace(string(data))
					logger.Debug("SSH session %q: password loaded from file %s", cfg.Name, pwFile)
				} else {
					logger.Warn("SSH session %q: failed to read password file %s: %v", cfg.Name, pwFile, err)
				}
			}
		} else {
			logger.Warn("SSH session %q: password file %s not found: %v", cfg.Name, pwFile, err)
		}
	}

	session := &SSHSession{
		name:                  cfg.Name,
		host:                  cfg.Host,
		port:                  cfg.Port,
		user:                  cfg.User,
		keyFile:               cfg.KeyFile,
		password:              password,
		jumpHost:              cfg.JumpHost,
		agentForwarding:       cfg.AgentForwarding,
		insecureIgnoreHostKey: cfg.InsecureIgnoreHostKey,
		env:                   make(map[string]string),
		connectTimeout:        cfg.ConnectTimeout,
		commandTimeout:        cfg.Timeout,
		startupCommands:       cfg.StartupCommands,
	}

	return session
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
		logger.Debug("SSH session %q already connected", s.name)
		return nil
	}

	logger.Debug("SSH connecting to %s@%s:%d", s.user, s.host, s.port)

	// Build auth methods
	authMethods, err := s.getAuthMethods()
	if err != nil {
		return err
	}

	if len(authMethods) == 0 {
		logger.Warn("SSH no authentication methods available for %q", s.name)
		return &Error{
			Code:       ErrAuthPasswordRequired,
			Message:    fmt.Sprintf("No authentication methods available for %s", s.name),
			Session:    s.name,
			Host:       s.host,
			Suggestion: fmt.Sprintf("Use /auth %s to provide credentials", s.name),
		}
	}

	logger.Debug("SSH found %d authentication method(s)", len(authMethods))

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
		Timeout:         s.connectTimeout,
	}

	// Connect (with or without jump host)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var client *ssh.Client

	if s.jumpHost != "" {
		// Connect via jump host
		client, err = s.connectViaJumpHost(addr, config)
	} else {
		// Direct connection
		client, err = ssh.Dial("tcp", addr, config)
	}

	if err != nil {
		logger.Debug("SSH dial failed: %v", err)
		return s.wrapConnectionError(err)
	}

	s.client = client
	s.connected = true
	logger.Debug("SSH connection established to %s", addr)

	// Get initial working directory
	result, err := s.executeRaw("pwd")
	if err == nil && result.ExitCode == 0 {
		s.cwd = strings.TrimSpace(result.Stdout)
		logger.Debug("SSH initial cwd: %s", s.cwd)
	} else {
		s.cwd = "~"
	}

	// Run startup commands
	if len(s.startupCommands) > 0 {
		s.runStartupCommands()
	}

	return nil
}

// connectViaJumpHost establishes a connection through a jump host
func (s *SSHSession) connectViaJumpHost(targetAddr string, targetConfig *ssh.ClientConfig) (*ssh.Client, error) {
	// Parse jump host (format: user@host:port or host)
	jumpUser, jumpHostAddr, jumpPort := s.parseJumpHost(s.jumpHost)

	logger.Debug("SSH connecting via jump host %s@%s:%d", jumpUser, jumpHostAddr, jumpPort)

	// Build auth methods for jump host (reuse same methods)
	jumpAuthMethods, err := s.getAuthMethods()
	if err != nil {
		return nil, fmt.Errorf("jump host auth: %w", err)
	}

	// Get host key callback for jump host
	jumpHostKeyCallback, err := s.getHostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("jump host key callback: %w", err)
	}

	// Create jump host config
	jumpConfig := &ssh.ClientConfig{
		User:            jumpUser,
		Auth:            jumpAuthMethods,
		HostKeyCallback: jumpHostKeyCallback,
		Timeout:         s.connectTimeout,
	}

	// Connect to jump host
	jumpAddr := fmt.Sprintf("%s:%d", jumpHostAddr, jumpPort)
	jumpClient, err := ssh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to jump host %s: %w", jumpAddr, err)
	}
	s.jumpClient = jumpClient
	logger.Debug("SSH connected to jump host %s", jumpAddr)

	// Open a connection to the target through the jump host
	conn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		s.jumpClient = nil
		return nil, fmt.Errorf("failed to connect to target %s via jump host: %w", targetAddr, err)
	}

	// Create SSH connection over the jump host connection
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, targetConfig)
	if err != nil {
		conn.Close()
		jumpClient.Close()
		s.jumpClient = nil
		return nil, fmt.Errorf("failed to establish SSH connection to %s: %w", targetAddr, err)
	}

	client := ssh.NewClient(ncc, chans, reqs)
	logger.Debug("SSH connection established to %s via jump host", targetAddr)

	return client, nil
}

// parseJumpHost parses a jump host specification (user@host:port or host)
func (s *SSHSession) parseJumpHost(jumpHost string) (user, host string, port int) {
	port = 22 // Default SSH port

	// Check for user@
	if idx := strings.Index(jumpHost, "@"); idx != -1 {
		user = jumpHost[:idx]
		jumpHost = jumpHost[idx+1:]
	} else {
		// Use current user as default
		user = s.user
	}

	// Check for :port
	if idx := strings.LastIndex(jumpHost, ":"); idx != -1 {
		host = jumpHost[:idx]
		if p, err := fmt.Sscanf(jumpHost[idx+1:], "%d", &port); err != nil || p != 1 {
			port = 22
		}
	} else {
		host = jumpHost
	}

	return user, host, port
}

// runStartupCommands executes the configured startup commands
func (s *SSHSession) runStartupCommands() {
	logger.Debug("SSH running %d startup command(s) on session %q", len(s.startupCommands), s.name)
	for _, cmd := range s.startupCommands {
		logger.Debug("SSH startup command: %s", cmd)
		result, err := s.executeRaw(cmd)
		if err != nil {
			logger.Warn("SSH startup command failed: %s - %v", cmd, err)
			continue
		}
		if result.ExitCode != 0 {
			logger.Warn("SSH startup command exited with code %d: %s", result.ExitCode, cmd)
		}
	}
}

// Disconnect closes the SSH connection
func (s *SSHSession) Disconnect() error {
	if s.client != nil {
		logger.Debug("SSH disconnecting from %s@%s", s.user, s.host)
		err := s.client.Close()
		s.client = nil
		s.connected = false

		// Also close jump client if present
		if s.jumpClient != nil {
			s.jumpClient.Close()
			s.jumpClient = nil
			logger.Debug("SSH jump host connection closed")
		}

		return err
	}
	return nil
}

// IsConnected returns true if connected
func (s *SSHSession) IsConnected() bool {
	return s.connected && s.client != nil
}

// CheckConnection checks if the connection is still alive
func (s *SSHSession) CheckConnection() bool {
	if !s.IsConnected() {
		return false
	}

	// Try to create a session to verify connection is alive
	session, err := s.client.NewSession()
	if err != nil {
		// Connection is dead
		s.connected = false
		return false
	}
	session.Close()
	return true
}

// Reconnect attempts to reconnect the SSH session
func (s *SSHSession) Reconnect() error {
	// Close any existing connection
	s.Disconnect()

	// Attempt to connect
	return s.Connect()
}

// Execute runs a command over SSH
func (s *SSHSession) Execute(cmdStr string) (*ExecuteResult, error) {
	ctx := context.Background()
	return s.ExecuteWithContext(ctx, cmdStr)
}

// ExecuteWithContext runs a command over SSH with cancellation support
func (s *SSHSession) ExecuteWithContext(ctx context.Context, cmdStr string) (*ExecuteResult, error) {
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

	return s.executeRawWithContext(ctx, cmdStr)
}

// executeRaw executes a command without cwd handling
func (s *SSHSession) executeRaw(cmdStr string) (*ExecuteResult, error) {
	return s.executeRawWithContext(context.Background(), cmdStr)
}

// executeRawWithContext executes a command with context cancellation support
func (s *SSHSession) executeRawWithContext(ctx context.Context, cmdStr string) (*ExecuteResult, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Request agent forwarding if enabled
	if s.agentForwarding {
		if err := agent.RequestAgentForwarding(session); err != nil {
			logger.Debug("SSH agent forwarding request failed: %v", err)
			// Don't fail the command, just log and continue
		}
	}

	// Build environment prefix (export commands)
	// This is more reliable than session.Setenv which requires server AcceptEnv config
	var envPrefix strings.Builder

	// Add color-related environment variables for better terminal experience
	envPrefix.WriteString("export TERM=${TERM:-xterm-256color}; ")
	envPrefix.WriteString("export CLICOLOR=1; ")
	envPrefix.WriteString("export CLICOLOR_FORCE=1; ")

	// Add user-defined environment variables
	for k, v := range s.env {
		// Escape single quotes in value
		escapedVal := strings.ReplaceAll(v, "'", "'\\''")
		envPrefix.WriteString(fmt.Sprintf("export %s='%s'; ", k, escapedVal))
	}

	if envPrefix.Len() > 0 {
		cmdStr = envPrefix.String() + cmdStr
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Create a channel for command completion
	done := make(chan error, 1)

	// Start command in goroutine
	go func() {
		done <- session.Run(cmdStr)
	}()

	// Wait for command, context cancellation, or timeout
	var runErr error
	select {
	case runErr = <-done:
		// Command completed
	case <-ctx.Done():
		// Context cancelled (user interrupt)
		logger.Debug("SSH command interrupted on %q", s.name)
		// Send SIGINT to the remote process
		session.Signal(ssh.SIGINT)
		// Give a brief moment for clean termination
		time.Sleep(100 * time.Millisecond)
		session.Close()
		return &ExecuteResult{
			Stderr:   "^C\n",
			ExitCode: 130, // Standard exit code for SIGINT
		}, nil
	case <-time.After(s.commandTimeout):
		// Timeout - close the session to kill the command
		logger.Warn("SSH command timed out after %s on %q", s.commandTimeout, s.name)
		session.Close()
		return nil, &Error{
			Code:      ErrCommandTimeout,
			Message:   fmt.Sprintf("Command timed out after %s", s.commandTimeout),
			Session:   s.name,
			Host:      s.host,
			Retryable: true,
		}
	}

	result := &ExecuteResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			return nil, runErr
		}
	}

	return result, nil
}

// ExecuteInteractive runs a command with PTY support for interactive programs
// This connects stdin/stdout/stderr directly to the user's terminal
func (s *SSHSession) ExecuteInteractive(cmdStr string) (int, error) {
	if !s.IsConnected() {
		return 1, &Error{
			Code:       ErrSessionDisconnected,
			Message:    fmt.Sprintf("Session %s is not connected", s.name),
			Session:    s.name,
			Retryable:  true,
			Suggestion: fmt.Sprintf("Use /connect %s to reconnect", s.name),
		}
	}

	session, err := s.client.NewSession()
	if err != nil {
		return 1, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Request agent forwarding if enabled
	if s.agentForwarding {
		if err := agent.RequestAgentForwarding(session); err != nil {
			logger.Debug("SSH agent forwarding request failed: %v", err)
		}
	}

	// Get terminal size
	fd := int(os.Stdin.Fd())
	width, height := 80, 24 // defaults
	if term.IsTerminal(fd) {
		if w, h, err := term.GetSize(fd); err == nil {
			width, height = w, h
		}
	}

	// Request a PTY with terminal type and size
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // Enable echo
		ssh.TTY_OP_ISPEED: 14400, // Input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // Output speed = 14.4kbaud
	}

	if err := session.RequestPty(termType, height, width, modes); err != nil {
		return 1, fmt.Errorf("failed to request PTY: %w", err)
	}

	// Get pipes for stdin/stdout
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Put terminal in raw mode before starting
	var oldState *term.State
	if term.IsTerminal(fd) {
		oldState, err = term.MakeRaw(fd)
		if err != nil {
			logger.Debug("Failed to set raw mode: %v", err)
		}
	}

	// Restore terminal on exit
	defer func() {
		if oldState != nil {
			term.Restore(fd, oldState)
		}
	}()

	// Handle window resize (SIGWINCH)
	sigwinchCh := make(chan os.Signal, 1)
	signal.Notify(sigwinchCh, syscall.SIGWINCH)
	defer signal.Stop(sigwinchCh)

	// Start goroutine to handle resize events
	go func() {
		for range sigwinchCh {
			if w, h, err := term.GetSize(fd); err == nil {
				session.WindowChange(h, w)
			}
		}
	}()

	// Build command with cwd and environment
	fullCmd := cmdStr
	if s.cwd != "" && s.cwd != "~" {
		fullCmd = fmt.Sprintf("cd %s && %s", s.cwd, cmdStr)
	}

	// Add environment variables
	var envPrefix strings.Builder
	envPrefix.WriteString("export TERM=" + termType + "; ")
	for k, v := range s.env {
		escapedVal := strings.ReplaceAll(v, "'", "'\\''")
		envPrefix.WriteString(fmt.Sprintf("export %s='%s'; ", k, escapedVal))
	}
	fullCmd = envPrefix.String() + fullCmd

	// Start the command (non-blocking)
	if err := session.Start(fullCmd); err != nil {
		return 1, fmt.Errorf("failed to start command: %w", err)
	}

	// Copy stdin to remote in a goroutine
	go func() {
		io.Copy(stdinPipe, os.Stdin)
		stdinPipe.Close()
	}()

	// Copy remote stdout to local stdout
	io.Copy(os.Stdout, stdoutPipe)

	// Wait for command to complete
	err = session.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			// Some errors are expected when the session closes
			logger.Debug("Session wait error: %v", err)
		}
	}

	return exitCode, nil
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

// RestoreEnv restores environment variables from a map (used after reconnect)
func (s *SSHSession) RestoreEnv(env map[string]string) {
	for k, v := range env {
		s.env[k] = v
	}
	if len(env) > 0 {
		logger.Debug("SSH restored %d environment variable(s) for session %q", len(env), s.name)
	}
}

// SetPassword sets the password for authentication
func (s *SSHSession) SetPassword(password string) {
	s.password = password
}

// HasPassword returns true if a password is set
func (s *SSHSession) HasPassword() bool {
	return s.password != ""
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

	// Try password authentication if password is set
	if s.password != "" {
		methods = append(methods, ssh.Password(s.password))
		logger.Debug("SSH using password authentication for session %q", s.name)
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
	// For testing only: skip host key verification
	if s.insecureIgnoreHostKey {
		logger.Debug("SSH session %q: skipping host key verification (insecure mode)", s.name)
		return ssh.InsecureIgnoreHostKey(), nil
	}

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

// FetchHostKey fetches and returns the host key and fingerprint for the session
func (s *SSHSession) FetchHostKey() (keyType string, fingerprint string, err error) {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	// Create a config that accepts any host key (to fetch it)
	var fetchedKey ssh.PublicKey
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{}, // No auth needed just to fetch key
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fetchedKey = key
			return nil // Accept the key to fetch it
		},
		Timeout: 10 * time.Second,
	}

	// Try to connect - will fail auth but we'll get the host key
	conn, err := net.DialTimeout("tcp", addr, config.Timeout)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Do SSH handshake to get the host key
	sshConn, _, _, err := ssh.NewClientConn(conn, addr, config)
	if sshConn != nil {
		sshConn.Close()
	}
	conn.Close()

	if fetchedKey == nil {
		return "", "", fmt.Errorf("failed to fetch host key")
	}

	keyType = fetchedKey.Type()
	fingerprint = ssh.FingerprintSHA256(fetchedKey)

	return keyType, fingerprint, nil
}

// AddHostKey adds the host's key to known_hosts
func (s *SSHSession) AddHostKey() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(knownHostsPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Fetch the host key
	var fetchedKey ssh.PublicKey
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fetchedKey = key
			return nil
		},
		Timeout: 10 * time.Second,
	}

	conn, err := net.DialTimeout("tcp", addr, config.Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	sshConn, _, _, err := ssh.NewClientConn(conn, addr, config)
	if sshConn != nil {
		sshConn.Close()
	}
	conn.Close()

	if fetchedKey == nil {
		return fmt.Errorf("failed to fetch host key")
	}

	// Format the host entry
	// If using non-standard port, format is [host]:port
	var hostEntry string
	if s.port != 22 {
		hostEntry = fmt.Sprintf("[%s]:%d", s.host, s.port)
	} else {
		hostEntry = s.host
	}

	// Create the known_hosts line
	line := knownhosts.Line([]string{hostEntry}, fetchedKey)

	// Append to known_hosts
	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("failed to write to known_hosts: %w", err)
	}

	logger.Info("added host key for %s to known_hosts", hostEntry)
	return nil
}

// Port returns the SSH port
func (s *SSHSession) Port() int {
	return s.port
}

// User returns the SSH user
func (s *SSHSession) User() string {
	return s.user
}

// UploadFile uploads a local file to the remote server
func (s *SSHSession) UploadFile(localPath, remotePath string) error {
	if !s.IsConnected() {
		return fmt.Errorf("session is not connected")
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get local file info for permissions
	localInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy file contents
	bytesWritten, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions on remote file
	if err := sftpClient.Chmod(remotePath, localInfo.Mode()); err != nil {
		logger.Warn("failed to set permissions on remote file: %v", err)
	}

	logger.Debug("uploaded %d bytes from %s to %s:%s", bytesWritten, localPath, s.host, remotePath)
	return nil
}

// DownloadFile downloads a remote file to the local filesystem
func (s *SSHSession) DownloadFile(remotePath, localPath string) error {
	if !s.IsConnected() {
		return fmt.Errorf("session is not connected")
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Get remote file info for permissions
	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy file contents
	bytesWritten, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions on local file
	if err := os.Chmod(localPath, remoteInfo.Mode()); err != nil {
		logger.Warn("failed to set permissions on local file: %v", err)
	}

	logger.Debug("downloaded %d bytes from %s:%s to %s", bytesWritten, s.host, remotePath, localPath)
	return nil
}

// ReadFile reads a file from the remote server and returns its contents
func (s *SSHSession) ReadFile(remotePath string) ([]byte, error) {
	if !s.IsConnected() {
		return nil, fmt.Errorf("session is not connected")
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(s.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Read file contents
	contents, err := io.ReadAll(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file: %w", err)
	}

	return contents, nil
}

// WriteFile writes data to a file on the remote server
func (s *SSHSession) WriteFile(remotePath string, data []byte, perm os.FileMode) error {
	if !s.IsConnected() {
		return fmt.Errorf("session is not connected")
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Write data
	if _, err := remoteFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to remote file: %w", err)
	}

	// Set permissions
	if err := sftpClient.Chmod(remotePath, perm); err != nil {
		logger.Warn("failed to set permissions on remote file: %v", err)
	}

	return nil
}
