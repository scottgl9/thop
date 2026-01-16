package session

import (
	"fmt"
	"io"
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

// FormatPrompt formats the prompt for a session
func FormatPrompt(sessionName string) string {
	return fmt.Sprintf("(%s) $ ", sessionName)
}
