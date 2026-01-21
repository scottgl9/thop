package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// Session interface defines the contract for all session types
type Session interface {
	// Name returns the session name
	Name() string

	// Type returns the session type ("local" or "ssh")
	Type() string

	// Connect establishes the session connection
	Connect() error

	// Disconnect closes the session connection
	Disconnect() error

	// IsConnected returns true if the session is connected
	IsConnected() bool

	// Execute runs a command and returns the output
	Execute(cmd string) (*ExecuteResult, error)

	// ExecuteWithContext runs a command with cancellation support
	ExecuteWithContext(ctx context.Context, cmd string) (*ExecuteResult, error)

	// ExecuteInteractive runs a command with PTY support for interactive programs
	// stdin, stdout, stderr are connected directly to the user's terminal
	// Returns exit code and error
	ExecuteInteractive(cmd string) (int, error)

	// GetCWD returns the current working directory
	GetCWD() string

	// SetCWD sets the current working directory
	SetCWD(path string) error

	// GetEnv returns the environment variables
	GetEnv() map[string]string

	// SetEnv sets an environment variable
	SetEnv(key, value string)
}

// ExecuteResult contains the result of command execution
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Error represents a session error with structured information
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Session    string `json:"session,omitempty"`
	Host       string `json:"host,omitempty"`
	Retryable  bool   `json:"retryable"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// Error codes
const (
	ErrConnectionFailed     = "CONNECTION_FAILED"
	ErrConnectionTimeout    = "CONNECTION_TIMEOUT"
	ErrAuthPasswordRequired = "AUTH_PASSWORD_REQUIRED"
	ErrAuthKeyRejected      = "AUTH_KEY_REJECTED"
	ErrAuthFailed           = "AUTH_FAILED"
	ErrHostKeyVerification  = "HOST_KEY_VERIFICATION_FAILED"
	ErrHostKeyChanged       = "HOST_KEY_CHANGED"
	ErrCommandTimeout       = "COMMAND_TIMEOUT"
	ErrCommandInterrupted   = "COMMAND_INTERRUPTED"
	ErrCommandRestricted    = "COMMAND_RESTRICTED"
	ErrSessionNotFound      = "SESSION_NOT_FOUND"
	ErrSessionDisconnected  = "SESSION_DISCONNECTED"
)

// NewError creates a new session error
func NewError(code, message, session string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Session: session,
	}
}

// CopyOutput copies data from reader to writer
func CopyOutput(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}

// ANSI color codes for prompt
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

// FormatPrompt formats the prompt for a session
// If cwd is provided, it will be shown after the session name
// The cwd is shortened: home directory becomes ~, long paths are truncated
func FormatPrompt(sessionName, cwd string) string {
	return formatPromptWithColor(sessionName, cwd, true)
}

// FormatPromptPlain formats the prompt without colors
func FormatPromptPlain(sessionName, cwd string) string {
	return formatPromptWithColor(sessionName, cwd, false)
}

func formatPromptWithColor(sessionName, cwd string, useColor bool) string {
	// Shorten home directory to ~
	displayCwd := cwd
	if cwd != "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if displayCwd == home {
				displayCwd = "~"
			} else if strings.HasPrefix(displayCwd, home+"/") {
				displayCwd = "~" + displayCwd[len(home):]
			}
		}

		// Truncate long paths to show only last 3 components
		parts := strings.Split(displayCwd, "/")
		if len(parts) > 4 && !strings.HasPrefix(displayCwd, "~") {
			displayCwd = ".../" + strings.Join(parts[len(parts)-3:], "/")
		}
	}

	if !useColor {
		if displayCwd == "" {
			return fmt.Sprintf("(%s) $ ", sessionName)
		}
		return fmt.Sprintf("(%s) %s $ ", sessionName, displayCwd)
	}

	// Colored prompt: (session) cwd $
	// - session name in green for local, cyan for SSH
	sessionColor := colorGreen
	if sessionName != "local" {
		sessionColor = colorCyan
	}

	if displayCwd == "" {
		return fmt.Sprintf("(%s%s%s) $ ", sessionColor, sessionName, colorReset)
	}

	return fmt.Sprintf("(%s%s%s) %s%s%s $ ",
		sessionColor, sessionName, colorReset,
		colorBlue, displayCwd, colorReset)
}
