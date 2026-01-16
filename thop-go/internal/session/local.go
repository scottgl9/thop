package session

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/scottgl9/thop/internal/logger"
)

// LocalSession represents a local shell session
type LocalSession struct {
	name      string
	shell     string
	cwd       string
	env       map[string]string
	connected bool
	timeout   time.Duration
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
	return nil
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
	// Handle cd commands specially to track cwd
	trimmedCmd := strings.TrimSpace(cmdStr)
	if trimmedCmd == "cd" || strings.HasPrefix(trimmedCmd, "cd ") {
		return s.handleCD(cmdStr)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Create the command with context
	cmd := exec.CommandContext(ctx, s.shell, "-c", cmdStr)
	cmd.Dir = s.cwd

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.env {
		cmd.Env = append(cmd.Env, k+"="+v)
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
		// Check if timeout was exceeded
		if ctx.Err() == context.DeadlineExceeded {
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
