package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/scottgl9/thop/internal/logger"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// LocalSession represents a local shell session
type LocalSession struct {
	name            string
	shell           string
	cwd             string
	env             map[string]string
	connected       bool
	timeout         time.Duration
	startupCommands []string
}

// NewLocalSession creates a new local session
func NewLocalSession(name, shell string) *LocalSession {
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		cwd = home
	}

	return &LocalSession{
		name:      name,
		shell:     shell,
		cwd:       cwd,
		env:       make(map[string]string),
		connected: true, // Local is always "connected"
		timeout:   300 * time.Second,
	}
}

// SetTimeout sets the command timeout
func (s *LocalSession) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// SetStartupCommands sets the startup commands to run on connect
func (s *LocalSession) SetStartupCommands(commands []string) {
	s.startupCommands = commands
}

// Name returns the session name
func (s *LocalSession) Name() string {
	return s.name
}

// Type returns the session type
func (s *LocalSession) Type() string {
	return "local"
}

// Connect establishes the session (no-op for local)
func (s *LocalSession) Connect() error {
	s.connected = true

	// Run startup commands if any
	if len(s.startupCommands) > 0 {
		s.runStartupCommands()
	}

	return nil
}

// runStartupCommands executes the configured startup commands
func (s *LocalSession) runStartupCommands() {
	logger.Debug("local running %d startup command(s) on session %q", len(s.startupCommands), s.name)
	for _, cmd := range s.startupCommands {
		logger.Debug("local startup command: %s", cmd)
		result, err := s.Execute(cmd)
		if err != nil {
			logger.Warn("local startup command failed: %s - %v", cmd, err)
			continue
		}
		if result.ExitCode != 0 {
			logger.Warn("local startup command exited with code %d: %s", result.ExitCode, cmd)
		}
	}
}

// Disconnect closes the session (no-op for local)
func (s *LocalSession) Disconnect() error {
	s.connected = false
	return nil
}

// IsConnected returns true if connected
func (s *LocalSession) IsConnected() bool {
	return s.connected
}

// Execute runs a command in the local shell
func (s *LocalSession) Execute(cmdStr string) (*ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	return s.ExecuteWithContext(ctx, cmdStr)
}

// ExecuteWithContext runs a command with cancellation support
func (s *LocalSession) ExecuteWithContext(ctx context.Context, cmdStr string) (*ExecuteResult, error) {
	// Handle cd commands specially to track cwd
	trimmedCmd := strings.TrimSpace(cmdStr)
	if trimmedCmd == "cd" || strings.HasPrefix(trimmedCmd, "cd ") {
		return s.handleCD(cmdStr)
	}

	// Create context with timeout if not already set
	execCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Create the command with context
	cmd := exec.CommandContext(execCtx, s.shell, "-c", cmdStr)
	cmd.Dir = s.cwd

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Ensure TERM is set for color support
	hasTerm := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "TERM=") {
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	}

	// Enable color output for common commands
	// CLICOLOR=1 enables colors for BSD/macOS commands
	// CLICOLOR_FORCE=1 forces colors even when not a TTY
	cmd.Env = append(cmd.Env, "CLICOLOR=1")
	cmd.Env = append(cmd.Env, "CLICOLOR_FORCE=1")
	// GCC_COLORS enables colored diagnostics in GCC
	if !hasEnvPrefix(cmd.Env, "GCC_COLORS=") {
		cmd.Env = append(cmd.Env, "GCC_COLORS=error=01;31:warning=01;35:note=01;36:caret=01;32:locus=01:quote=01")
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	result := &ExecuteResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		// Check if context was canceled (user interrupt)
		if ctx.Err() == context.Canceled {
			logger.Debug("local command interrupted on %q", s.name)
			return &ExecuteResult{
				Stderr:   "^C\n",
				ExitCode: 130, // Standard exit code for SIGINT
			}, nil
		}
		// Check if timeout was exceeded
		if execCtx.Err() == context.DeadlineExceeded {
			logger.Warn("local command timed out after %s on %q", s.timeout, s.name)
			return nil, &Error{
				Code:      ErrCommandTimeout,
				Message:   "Command timed out after " + s.timeout.String(),
				Session:   s.name,
				Retryable: true,
			}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return result, nil
}

// ExecuteInteractive runs a command with PTY support for interactive programs
func (s *LocalSession) ExecuteInteractive(cmdStr string) (int, error) {
	// Create the command
	cmd := exec.Command(s.shell, "-c", cmdStr)
	cmd.Dir = s.cwd

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Ensure TERM is set
	hasTerm := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "TERM=") {
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	}

	// Start with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, err
	}
	defer ptmx.Close()

	// Get terminal fd
	fd := int(os.Stdin.Fd())

	// Handle window resize (SIGWINCH)
	sigwinchCh := make(chan os.Signal, 1)
	signal.Notify(sigwinchCh, syscall.SIGWINCH)
	defer signal.Stop(sigwinchCh)

	// Initial resize
	if term.IsTerminal(fd) {
		if resizeErr := pty.InheritSize(os.Stdin, ptmx); resizeErr != nil {
			logger.Debug("Failed to set initial PTY size: %v", resizeErr)
		}
	}

	// Start goroutine to handle resize events
	go func() {
		for range sigwinchCh {
			if resizeErr := pty.InheritSize(os.Stdin, ptmx); resizeErr != nil {
				logger.Debug("Failed to resize PTY: %v", resizeErr)
			}
		}
	}()

	// Put terminal in raw mode
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
			_ = term.Restore(fd, oldState)
		}
	}()

	// Create interrupt pipe to signal stdin goroutine to exit
	interruptR, interruptW, pipeErr := os.Pipe()
	if pipeErr != nil {
		return 1, fmt.Errorf("failed to create interrupt pipe: %w", pipeErr)
	}

	stdinFd := int(os.Stdin.Fd())
	interruptFd := int(interruptR.Fd())

	// Copy stdin to PTY using poll() for interruptibility
	stdinDone := make(chan struct{})
	go func() {
		defer close(stdinDone)
		buf := make([]byte, 32*1024)

		pollFds := []unix.PollFd{
			{Fd: int32(stdinFd), Events: unix.POLLIN},
			{Fd: int32(interruptFd), Events: unix.POLLIN},
		}

		for {
			// Wait for input on stdin or interrupt pipe
			n, pollErr := unix.Poll(pollFds, -1)
			if pollErr != nil {
				if pollErr == syscall.EINTR {
					continue // Interrupted by signal, retry
				}
				return
			}
			if n <= 0 {
				continue
			}

			// Check if interrupt pipe has data (time to exit)
			if pollFds[1].Revents&unix.POLLIN != 0 {
				return
			}

			// Check if stdin has data
			if pollFds[0].Revents&unix.POLLIN != 0 {
				nr, readErr := os.Stdin.Read(buf)
				if readErr != nil || nr == 0 {
					return
				}
				if _, writeErr := ptmx.Write(buf[:nr]); writeErr != nil {
					return
				}
			}

			// Check for hangup/error on stdin
			if pollFds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
				return
			}
		}
	}()

	// Copy PTY to stdout
	_, _ = io.Copy(os.Stdout, ptmx)

	// Signal stdin goroutine to exit
	_, _ = interruptW.Write([]byte{0})
	interruptW.Close()
	interruptR.Close()

	// Wait for stdin goroutine to finish
	<-stdinDone

	// Wait for command to complete
	err = cmd.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// PTY closed errors are normal when the process exits
			if !strings.Contains(err.Error(), "input/output error") {
				return 1, err
			}
		}
	}

	return exitCode, nil
}

// handleCD handles cd commands to track working directory
func (s *LocalSession) handleCD(cmdStr string) (*ExecuteResult, error) {
	// Parse the cd command
	parts := strings.Fields(cmdStr)
	var targetDir string

	if len(parts) == 1 {
		// cd with no args goes to home
		home, err := os.UserHomeDir()
		if err != nil {
			return &ExecuteResult{
				Stderr:   "cd: HOME not set\n",
				ExitCode: 1,
			}, nil
		}
		targetDir = home
	} else {
		targetDir = parts[1]

		// Handle ~ expansion
		if strings.HasPrefix(targetDir, "~") {
			home, _ := os.UserHomeDir()
			targetDir = strings.Replace(targetDir, "~", home, 1)
		}

		// Handle relative paths
		if !strings.HasPrefix(targetDir, "/") {
			targetDir = s.cwd + "/" + targetDir
		}
	}

	// Check if directory exists
	info, err := os.Stat(targetDir)
	if err != nil {
		return &ExecuteResult{
			Stderr:   "cd: " + targetDir + ": No such file or directory\n",
			ExitCode: 1,
		}, nil
	}

	if !info.IsDir() {
		return &ExecuteResult{
			Stderr:   "cd: " + targetDir + ": Not a directory\n",
			ExitCode: 1,
		}, nil
	}

	// Clean the path
	cmd := exec.Command(s.shell, "-c", "cd "+targetDir+" && pwd")
	output, err := cmd.Output()
	if err != nil {
		return &ExecuteResult{
			Stderr:   "cd: " + targetDir + ": " + err.Error() + "\n",
			ExitCode: 1,
		}, nil
	}

	s.cwd = strings.TrimSpace(string(output))

	return &ExecuteResult{
		ExitCode: 0,
	}, nil
}

// GetCWD returns the current working directory
func (s *LocalSession) GetCWD() string {
	return s.cwd
}

// SetCWD sets the current working directory
func (s *LocalSession) SetCWD(path string) error {
	// Verify the path exists
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrNotExist
	}

	s.cwd = path
	return nil
}

// GetEnv returns the environment variables
func (s *LocalSession) GetEnv() map[string]string {
	// Return a copy
	env := make(map[string]string, len(s.env))
	for k, v := range s.env {
		env[k] = v
	}
	return env
}

// SetEnv sets an environment variable
func (s *LocalSession) SetEnv(key, value string) {
	s.env[key] = value
}

// SetShell sets the shell to use
func (s *LocalSession) SetShell(shell string) {
	s.shell = shell
}

// hasEnvPrefix checks if any environment variable starts with the given prefix
func hasEnvPrefix(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}
