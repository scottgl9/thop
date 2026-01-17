package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scottgl9/thop/internal/logger"
	"github.com/scottgl9/thop/internal/session"
)

// Tool implementation functions

// toolConnect handles the connect tool
func (s *Server) toolConnect(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, ok := args["session"].(string)
	if !ok {
		return s.errorResult("session parameter is required"), nil
	}

	if err := s.sessions.Connect(sessionName); err != nil {
		return s.errorResult(fmt.Sprintf("Failed to connect: %v", err)), nil
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
		return s.errorResult("session parameter is required"), nil
	}

	if err := s.sessions.SetActiveSession(sessionName); err != nil {
		return s.errorResult(fmt.Sprintf("Failed to switch session: %v", err)), nil
	}

	// Get session info
	sess, ok := s.sessions.GetSession(sessionName)
	if !ok || sess == nil {
		return s.errorResult("Session not found after switch"), nil
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
		return s.errorResult("session parameter is required"), nil
	}

	if err := s.sessions.Disconnect(sessionName); err != nil {
		return s.errorResult(fmt.Sprintf("Failed to close session: %v", err)), nil
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
		return s.errorResult(fmt.Sprintf("Failed to format status: %v", err)), nil
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
		return s.errorResult("command parameter is required"), nil
	}

	sessionName, _ := args["session"].(string)
	timeout := 300 // default 5 minutes

	if t, ok := args["timeout"].(float64); ok {
		timeout = int(t)
	}

	// Get the session
	var sess session.Session
	if sessionName != "" {
		var ok bool
		sess, ok = s.sessions.GetSession(sessionName)
		if !ok || sess == nil {
			return s.errorResult(fmt.Sprintf("Session not found: %s", sessionName)), nil
		}
	} else {
		sess = s.sessions.GetActiveSession()
		if sess == nil {
			return s.errorResult("No active session"), nil
		}
	}

	// Execute the command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	result, err := sess.ExecuteWithContext(cmdCtx, command)
	if err != nil {
		// Include stderr in error if available
		errorText := err.Error()
		if result != nil && result.Stderr != "" {
			errorText = fmt.Sprintf("%s\nStderr: %s", errorText, result.Stderr)
		}
		return s.errorResult(errorText), nil
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

// toolExecuteBackground handles the executeBackground tool
func (s *Server) toolExecuteBackground(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// TODO: Implement background job execution
	// This requires extending the Session interface with background job support
	return s.errorResult("Background execution not yet implemented"), nil
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