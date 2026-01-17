package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/scottgl9/thop/internal/logger"
	"github.com/scottgl9/thop/internal/session"
)

// Tool implementation functions

// toolConnect handles the connect tool
func (s *Server) toolConnect(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, ok := args["session"].(string)
	if !ok {
		return MissingParameterError("session").ToToolResult(), nil
	}

	if err := s.sessions.Connect(sessionName); err != nil {
		// Parse error and return appropriate error code
		errStr := err.Error()

		// Check for specific error patterns
		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "does not exist") {
			return SessionNotFoundError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "key") && strings.Contains(errStr, "auth") {
			return AuthKeyFailedError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "password") {
			return AuthPasswordFailedError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "host key") || strings.Contains(errStr, "known_hosts") {
			return HostKeyUnknownError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "timeout") {
			return NewMCPError(ErrorConnectionTimeout, "Connection timed out").
				WithSession(sessionName).
				WithSuggestion("Check network connectivity and firewall settings").
				ToToolResult(), nil
		}
		if strings.Contains(errStr, "refused") {
			return NewMCPError(ErrorConnectionRefused, "Connection refused").
				WithSession(sessionName).
				WithSuggestion("Verify the host and port are correct").
				ToToolResult(), nil
		}

		// Generic connection failure
		return ConnectionFailedError(sessionName, errStr).ToToolResult(), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Successfully connected to session '%s'", sessionName),
			},
		},
	}, nil
}

// toolSwitch handles the switch tool
func (s *Server) toolSwitch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, ok := args["session"].(string)
	if !ok {
		return MissingParameterError("session").ToToolResult(), nil
	}

	if err := s.sessions.SetActiveSession(sessionName); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			return SessionNotFoundError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "not connected") {
			return SessionNotConnectedError(sessionName).ToToolResult(), nil
		}
		return NewMCPError(ErrorOperationFailed, fmt.Sprintf("Failed to switch session: %v", err)).
			WithSession(sessionName).
			ToToolResult(), nil
	}

	// Get session info
	sess, ok := s.sessions.GetSession(sessionName)
	if !ok || sess == nil {
		return SessionNotFoundError(sessionName).ToToolResult(), nil
	}

	cwd := sess.GetCWD()
	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Switched to session '%s' (cwd: %s)", sessionName, cwd),
			},
		},
	}, nil
}

// toolClose handles the close tool
func (s *Server) toolClose(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, ok := args["session"].(string)
	if !ok {
		return MissingParameterError("session").ToToolResult(), nil
	}

	if err := s.sessions.Disconnect(sessionName); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			return SessionNotFoundError(sessionName).ToToolResult(), nil
		}
		if strings.Contains(errStr, "cannot close local") || strings.Contains(errStr, "local session") {
			return NewMCPError(ErrorCannotCloseLocal, "Cannot close the local session").
				WithSession(sessionName).
				WithSuggestion("Use /switch to change to another session instead").
				ToToolResult(), nil
		}
		return NewMCPError(ErrorOperationFailed, fmt.Sprintf("Failed to close session: %v", err)).
			WithSession(sessionName).
			ToToolResult(), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Session '%s' closed", sessionName),
			},
		},
	}, nil
}

// toolStatus handles the status tool
func (s *Server) toolStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessions := s.sessions.ListSessions()

	// Format status as JSON
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return NewMCPError(ErrorOperationFailed, fmt.Sprintf("Failed to format status: %v", err)).
			WithSuggestion("Check system resources and try again").
			ToToolResult(), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type:     "text",
				Text:     string(data),
				MimeType: "application/json",
			},
		},
	}, nil
}

// toolExecute handles the execute tool
func (s *Server) toolExecute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	command, ok := args["command"].(string)
	if !ok {
		return MissingParameterError("command").ToToolResult(), nil
	}

	sessionName, _ := args["session"].(string)
	background := false

	if bg, ok := args["background"].(bool); ok {
		background = bg
	}

	// Get the session
	var sess session.Session
	if sessionName != "" {
		var ok bool
		sess, ok = s.sessions.GetSession(sessionName)
		if !ok || sess == nil {
			return SessionNotFoundError(sessionName).ToToolResult(), nil
		}
		sessionName = sess.Name() // Use actual session name
	} else {
		sess = s.sessions.GetActiveSession()
		if sess == nil {
			return NewMCPError(ErrorNoActiveSession, "No active session").
				WithSuggestion("Use /connect to establish a session or specify a session name").
				ToToolResult(), nil
		}
		sessionName = sess.Name()
	}

	// Determine timeout: explicit parameter > session config > global default
	timeout := s.config.GetTimeout(sessionName)
	if t, ok := args["timeout"].(float64); ok && int(t) > 0 {
		timeout = int(t)
	}

	// Handle background execution
	if background {
		return NotImplementedError("Background execution").ToToolResult(), nil
	}

	// Execute the command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	result, err := sess.ExecuteWithContext(cmdCtx, command)
	if err != nil {
		errStr := err.Error()

		// Check for timeout
		if strings.Contains(errStr, "context deadline exceeded") || strings.Contains(errStr, "timeout") {
			return CommandTimeoutError(sessionName, timeout).ToToolResult(), nil
		}

		// Check for permission denied
		if strings.Contains(errStr, "permission denied") {
			return NewMCPError(ErrorPermissionDenied, "Permission denied").
				WithSession(sessionName).
				WithSuggestion("Check file/directory permissions or use sudo if appropriate").
				ToToolResult(), nil
		}

		// Check for command not found
		if strings.Contains(errStr, "command not found") || strings.Contains(errStr, "not found") {
			return NewMCPError(ErrorCommandNotFound, fmt.Sprintf("Command not found: %s", command)).
				WithSession(sessionName).
				WithSuggestion("Verify the command is installed and in PATH").
				ToToolResult(), nil
		}

		// Generic command failure
		errorText := errStr
		if result != nil && result.Stderr != "" {
			errorText = fmt.Sprintf("%s\nStderr: %s", errorText, result.Stderr)
		}

		return NewMCPError(ErrorCommandFailed, errorText).
			WithSession(sessionName).
			ToToolResult(), nil
	}

	// Prepare content
	content := []Content{}

	// Add stdout if present
	if result.Stdout != "" {
		content = append(content, Content{
			Type: "text",
			Text: result.Stdout,
		})
	}

	// Add stderr if present
	if result.Stderr != "" {
		content = append(content, Content{
			Type: "text",
			Text: fmt.Sprintf("stderr:\n%s", result.Stderr),
		})
	}

	// Add exit code if non-zero
	if result.ExitCode != 0 {
		content = append(content, Content{
			Type: "text",
			Text: fmt.Sprintf("Exit code: %d", result.ExitCode),
		})
	}

	// If no output at all, indicate success
	if len(content) == 0 {
		content = append(content, Content{
			Type: "text",
			Text: "Command executed successfully (no output)",
		})
	}

	return ToolCallResult{
		Content: content,
		IsError: result.ExitCode != 0,
	}, nil
}

// Helper functions

// errorResult creates an error tool result
func (s *Server) errorResult(message string) interface{} {
	logger.Debug("Tool error: %s", message)
	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: message,
			},
		},
		IsError: true,
	}
}

// Resource helper functions

// getActiveSessionResource returns the active session as a JSON resource
func (s *Server) getActiveSessionResource() (string, error) {
	sess := s.sessions.GetActiveSession()
	if sess == nil {
		return "", fmt.Errorf("no active session")
	}

	// Create session info
	info := map[string]interface{}{
		"name":       sess.Name(),
		"type":       sess.Type(),
		"connected":  sess.IsConnected(),
		"cwd":        sess.GetCWD(),
		"environment": sess.GetEnv(),
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// getAllSessionsResource returns all sessions as a JSON resource
func (s *Server) getAllSessionsResource() (string, error) {
	sessions := s.sessions.ListSessions()
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// getConfigResource returns the configuration as a JSON resource
func (s *Server) getConfigResource() (string, error) {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// getStateResource returns the state as a JSON resource
func (s *Server) getStateResource() (string, error) {
	// Get the current state
	sessions := s.state.GetAllSessions()
	activeSession := s.state.GetActiveSession()

	stateData := map[string]interface{}{
		"active_session": activeSession,
		"sessions":       sessions,
	}

	data, err := json.MarshalIndent(stateData, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}