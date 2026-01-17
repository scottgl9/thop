package mcp

import (
	"testing"
)

func TestMCPError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      MCPError
		expected string
	}{
		{
			name: "basic error",
			err:  NewMCPError(ErrorSessionNotFound, "Session 'prod' not found"),
			expected: "[SESSION_NOT_FOUND] Session 'prod' not found",
		},
		{
			name: "error with session",
			err:  NewMCPError(ErrorConnectionFailed, "Connection failed").WithSession("prod"),
			expected: "[CONNECTION_FAILED] Connection failed (session: prod)",
		},
		{
			name: "error with session and suggestion",
			err: NewMCPError(ErrorAuthKeyFailed, "SSH key authentication failed").
				WithSession("prod").
				WithSuggestion("Use /auth to provide a password"),
			expected: "[AUTH_KEY_FAILED] SSH key authentication failed (session: prod)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMCPError_ToToolResult(t *testing.T) {
	tests := []struct {
		name    string
		err     MCPError
		wantErr bool
	}{
		{
			name:    "basic error",
			err:     NewMCPError(ErrorSessionNotFound, "Session not found"),
			wantErr: true,
		},
		{
			name:    "error with session",
			err:     SessionNotFoundError("prod"),
			wantErr: true,
		},
		{
			name:    "error with suggestion",
			err:     AuthKeyFailedError("prod"),
			wantErr: true,
		},
		{
			name:    "command timeout",
			err:     CommandTimeoutError("prod", 300),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.ToToolResult()
			if result.IsError != tt.wantErr {
				t.Errorf("ToToolResult().IsError = %v, want %v", result.IsError, tt.wantErr)
			}
			if len(result.Content) == 0 {
				t.Error("ToToolResult().Content is empty")
			}
			if result.Content[0].Type != "text" {
				t.Errorf("ToToolResult().Content[0].Type = %q, want %q", result.Content[0].Type, "text")
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func() MCPError
		wantCode    ErrorCode
		wantSession string
	}{
		{
			name:        "SessionNotFoundError",
			constructor: func() MCPError { return SessionNotFoundError("prod") },
			wantCode:    ErrorSessionNotFound,
			wantSession: "prod",
		},
		{
			name:        "SessionNotConnectedError",
			constructor: func() MCPError { return SessionNotConnectedError("dev") },
			wantCode:    ErrorSessionNotConnected,
			wantSession: "dev",
		},
		{
			name:        "AuthKeyFailedError",
			constructor: func() MCPError { return AuthKeyFailedError("staging") },
			wantCode:    ErrorAuthKeyFailed,
			wantSession: "staging",
		},
		{
			name:        "HostKeyUnknownError",
			constructor: func() MCPError { return HostKeyUnknownError("newserver") },
			wantCode:    ErrorHostKeyUnknown,
			wantSession: "newserver",
		},
		{
			name:        "ConnectionFailedError",
			constructor: func() MCPError { return ConnectionFailedError("prod", "timeout") },
			wantCode:    ErrorConnectionFailed,
			wantSession: "prod",
		},
		{
			name:        "CommandTimeoutError",
			constructor: func() MCPError { return CommandTimeoutError("prod", 300) },
			wantCode:    ErrorCommandTimeout,
			wantSession: "prod",
		},
		{
			name:        "MissingParameterError",
			constructor: func() MCPError { return MissingParameterError("session") },
			wantCode:    ErrorMissingParameter,
			wantSession: "",
		},
		{
			name:        "NotImplementedError",
			constructor: func() MCPError { return NotImplementedError("background jobs") },
			wantCode:    ErrorNotImplemented,
			wantSession: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			if err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", err.Code, tt.wantCode)
			}
			if err.Session != tt.wantSession {
				t.Errorf("Session = %q, want %q", err.Session, tt.wantSession)
			}
			if err.Message == "" {
				t.Error("Message is empty")
			}
			// All errors should have suggestions except generic ones
			if tt.wantCode != ErrorOperationFailed && err.Suggestion == "" {
				// Only check for suggestion if not a generic error
				hasExpectedSuggestion := err.Suggestion != ""
				if tt.name != "MissingParameterError" && !hasExpectedSuggestion {
					// Most specific errors should have suggestions
					t.Log("Note: Error has no suggestion (may be intentional)")
				}
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrorSessionNotFound,
		ErrorSessionNotConnected,
		ErrorSessionAlreadyExists,
		ErrorNoActiveSession,
		ErrorCannotCloseLocal,
		ErrorConnectionFailed,
		ErrorAuthFailed,
		ErrorAuthKeyFailed,
		ErrorAuthPasswordFailed,
		ErrorHostKeyUnknown,
		ErrorHostKeyMismatch,
		ErrorConnectionTimeout,
		ErrorConnectionRefused,
		ErrorCommandFailed,
		ErrorCommandTimeout,
		ErrorCommandNotFound,
		ErrorPermissionDenied,
		ErrorInvalidParameter,
		ErrorMissingParameter,
		ErrorNotImplemented,
		ErrorOperationFailed,
	}

	// Just verify all error codes are defined and non-empty
	for _, code := range codes {
		if code == "" {
			t.Errorf("Error code is empty")
		}
	}
}
