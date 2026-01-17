package mcp

import "fmt"

// ErrorCode represents a structured MCP error code
type ErrorCode string

const (
	// Session errors
	ErrorSessionNotFound      ErrorCode = "SESSION_NOT_FOUND"
	ErrorSessionNotConnected  ErrorCode = "SESSION_NOT_CONNECTED"
	ErrorSessionAlreadyExists ErrorCode = "SESSION_ALREADY_EXISTS"
	ErrorNoActiveSession      ErrorCode = "NO_ACTIVE_SESSION"
	ErrorCannotCloseLocal     ErrorCode = "CANNOT_CLOSE_LOCAL"

	// Connection errors
	ErrorConnectionFailed    ErrorCode = "CONNECTION_FAILED"
	ErrorAuthFailed          ErrorCode = "AUTH_FAILED"
	ErrorAuthKeyFailed       ErrorCode = "AUTH_KEY_FAILED"
	ErrorAuthPasswordFailed  ErrorCode = "AUTH_PASSWORD_FAILED"
	ErrorHostKeyUnknown      ErrorCode = "HOST_KEY_UNKNOWN"
	ErrorHostKeyMismatch     ErrorCode = "HOST_KEY_MISMATCH"
	ErrorConnectionTimeout   ErrorCode = "CONNECTION_TIMEOUT"
	ErrorConnectionRefused   ErrorCode = "CONNECTION_REFUSED"

	// Command execution errors
	ErrorCommandFailed       ErrorCode = "COMMAND_FAILED"
	ErrorCommandTimeout      ErrorCode = "COMMAND_TIMEOUT"
	ErrorCommandNotFound     ErrorCode = "COMMAND_NOT_FOUND"
	ErrorPermissionDenied    ErrorCode = "PERMISSION_DENIED"

	// Parameter errors
	ErrorInvalidParameter    ErrorCode = "INVALID_PARAMETER"
	ErrorMissingParameter    ErrorCode = "MISSING_PARAMETER"

	// Feature errors
	ErrorNotImplemented      ErrorCode = "NOT_IMPLEMENTED"
	ErrorOperationFailed     ErrorCode = "OPERATION_FAILED"
)

// MCPError represents a structured error for MCP responses
type MCPError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Session    string    `json:"session,omitempty"`
	Suggestion string    `json:"suggestion,omitempty"`
}

// Error implements the error interface
func (e MCPError) Error() string {
	if e.Session != "" {
		return fmt.Sprintf("[%s] %s (session: %s)", e.Code, e.Message, e.Session)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewMCPError creates a new MCP error
func NewMCPError(code ErrorCode, message string) MCPError {
	return MCPError{
		Code:    code,
		Message: message,
	}
}

// WithSession adds session information to an error
func (e MCPError) WithSession(session string) MCPError {
	e.Session = session
	return e
}

// WithSuggestion adds a suggestion to an error
func (e MCPError) WithSuggestion(suggestion string) MCPError {
	e.Suggestion = suggestion
	return e
}

// Common error constructors with suggestions

func SessionNotFoundError(sessionName string) MCPError {
	return NewMCPError(ErrorSessionNotFound, fmt.Sprintf("Session '%s' not found", sessionName)).
		WithSession(sessionName).
		WithSuggestion("Use /status to see available sessions or /add-session to create a new one")
}

func SessionNotConnectedError(sessionName string) MCPError {
	return NewMCPError(ErrorSessionNotConnected, fmt.Sprintf("Session '%s' is not connected", sessionName)).
		WithSession(sessionName).
		WithSuggestion("Use /connect to establish a connection")
}

func AuthKeyFailedError(sessionName string) MCPError {
	return NewMCPError(ErrorAuthKeyFailed, "SSH key authentication failed").
		WithSession(sessionName).
		WithSuggestion("Use /auth to provide a password or check your SSH key configuration")
}

func AuthPasswordFailedError(sessionName string) MCPError {
	return NewMCPError(ErrorAuthPasswordFailed, "Password authentication failed").
		WithSession(sessionName).
		WithSuggestion("Verify the password is correct")
}

func HostKeyUnknownError(sessionName string) MCPError {
	return NewMCPError(ErrorHostKeyUnknown, "Host key is not in known_hosts").
		WithSession(sessionName).
		WithSuggestion("Use /trust to accept the host key")
}

func ConnectionFailedError(sessionName string, reason string) MCPError {
	return NewMCPError(ErrorConnectionFailed, fmt.Sprintf("Connection failed: %s", reason)).
		WithSession(sessionName).
		WithSuggestion("Check network connectivity and session configuration")
}

func CommandTimeoutError(sessionName string, timeout int) MCPError {
	return NewMCPError(ErrorCommandTimeout, fmt.Sprintf("Command execution timed out after %d seconds", timeout)).
		WithSession(sessionName).
		WithSuggestion("Increase timeout parameter or run command in background")
}

func MissingParameterError(param string) MCPError {
	return NewMCPError(ErrorMissingParameter, fmt.Sprintf("Required parameter '%s' is missing", param)).
		WithSuggestion(fmt.Sprintf("Provide the '%s' parameter", param))
}

func NotImplementedError(feature string) MCPError {
	return NewMCPError(ErrorNotImplemented, fmt.Sprintf("%s is not yet implemented", feature)).
		WithSuggestion("This feature is planned for a future release")
}

// Helper function to format error as MCP tool result
func (e MCPError) ToToolResult() ToolCallResult {
	text := e.Message
	if e.Suggestion != "" {
		text = fmt.Sprintf("%s\n\nSuggestion: %s", text, e.Suggestion)
	}
	if e.Session != "" {
		text = fmt.Sprintf("%s\n\nSession: %s", text, e.Session)
	}
	text = fmt.Sprintf("[%s] %s", e.Code, text)

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: true,
	}
}
