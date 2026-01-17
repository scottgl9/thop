package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

// toolReadFile handles the readFile tool
func (s *Server) toolReadFile(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return s.errorResult("path parameter is required"), nil
	}

	sessionName, _ := args["session"].(string)

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

	// For local sessions, read directly
	if sess.Type() == "local" {
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return s.errorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
		}
		return ToolCallResult{
			Content: []Content{
				{
					Type: "text",
					Text: string(content),
				},
			},
		}, nil
	}

	// For SSH sessions, use cat command
	result, err := sess.ExecuteWithContext(ctx, fmt.Sprintf("cat %s", path))
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: result.Stdout,
			},
		},
		IsError: result.ExitCode != 0,
	}, nil
}

// toolWriteFile handles the writeFile tool
func (s *Server) toolWriteFile(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return s.errorResult("path parameter is required"), nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return s.errorResult("content parameter is required"), nil
	}

	sessionName, _ := args["session"].(string)

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

	// For local sessions, write directly
	if sess.Type() == "local" {
		if err := ioutil.WriteFile(path, []byte(content), 0644); err != nil {
			return s.errorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
		}
		return ToolCallResult{
			Content: []Content{
				{
					Type: "text",
					Text: fmt.Sprintf("File written: %s", path),
				},
			},
		}, nil
	}

	// For SSH sessions, use echo or cat with heredoc
	// Escape the content for shell
	escapedContent := strings.ReplaceAll(content, "'", "'\\''")
	cmd := fmt.Sprintf("cat > '%s' << 'EOF'\n%s\nEOF", path, escapedContent)

	result, err := sess.ExecuteWithContext(ctx, cmd)
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
	}

	if result.ExitCode != 0 {
		return s.errorResult(fmt.Sprintf("Failed to write file: exit code %d", result.ExitCode)), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("File written: %s", path),
			},
		},
	}, nil
}

// toolListFiles handles the listFiles tool
func (s *Server) toolListFiles(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := "."
	if p, ok := args["path"].(string); ok {
		path = p
	}

	sessionName, _ := args["session"].(string)

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

	// For local sessions, list directly
	if sess.Type() == "local" {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return s.errorResult(fmt.Sprintf("Failed to list files: %v", err)), nil
		}

		var fileList []string
		for _, f := range files {
			name := f.Name()
			if f.IsDir() {
				name += "/"
			}
			fileList = append(fileList, name)
		}

		return ToolCallResult{
			Content: []Content{
				{
					Type: "text",
					Text: strings.Join(fileList, "\n"),
				},
			},
		}, nil
	}

	// For SSH sessions, use ls command
	result, err := sess.ExecuteWithContext(ctx, fmt.Sprintf("ls -la %s", path))
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to list files: %v", err)), nil
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: result.Stdout,
			},
		},
		IsError: result.ExitCode != 0,
	}, nil
}

// toolGetEnvironment handles the getEnvironment tool
func (s *Server) toolGetEnvironment(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, _ := args["session"].(string)

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

	env := sess.GetEnv()

	// Format environment as JSON
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to format environment: %v", err)), nil
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

// toolSetEnvironment handles the setEnvironment tool
func (s *Server) toolSetEnvironment(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	variables, ok := args["variables"].(map[string]interface{})
	if !ok {
		return s.errorResult("variables parameter is required and must be an object"), nil
	}

	sessionName, _ := args["session"].(string)

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

	// Convert variables to string map
	envVars := make(map[string]string)
	for k, v := range variables {
		envVars[k] = fmt.Sprintf("%v", v)
	}

	// Set environment variables
	for k, v := range envVars {
		sess.SetEnv(k, v)
	}

	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Set %d environment variables", len(envVars)),
			},
		},
	}, nil
}

// toolGetCwd handles the getCwd tool
func (s *Server) toolGetCwd(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionName, _ := args["session"].(string)

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

	cwd := sess.GetCWD()
	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: cwd,
			},
		},
	}, nil
}

// toolSetCwd handles the setCwd tool
func (s *Server) toolSetCwd(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return s.errorResult("path parameter is required"), nil
	}

	sessionName, _ := args["session"].(string)

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

	// Change directory
	result, err := sess.ExecuteWithContext(ctx, fmt.Sprintf("cd %s && pwd", path))
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to change directory: %v", err)), nil
	}

	if result.ExitCode != 0 {
		return s.errorResult(fmt.Sprintf("Failed to change directory: %s", result.Stderr)), nil
	}

	newCwd := strings.TrimSpace(result.Stdout)
	return ToolCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Changed directory to: %s", newCwd),
			},
		},
	}, nil
}

// toolListJobs handles the listJobs tool
func (s *Server) toolListJobs(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// TODO: Implement job listing
	// This requires extending the Session interface with background job support
	return s.errorResult("Job listing not yet implemented"), nil
}

// toolGetJobOutput handles the getJobOutput tool
func (s *Server) toolGetJobOutput(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// TODO: Implement job output retrieval
	// This requires extending the Session interface with background job support
	return s.errorResult("Job output retrieval not yet implemented"), nil
}

// toolKillJob handles the killJob tool
func (s *Server) toolKillJob(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// TODO: Implement job killing
	// This requires extending the Session interface with background job support
	return s.errorResult("Job killing not yet implemented"), nil
}

// toolGetConfig handles the getConfig tool
func (s *Server) toolGetConfig(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Serialize config to JSON
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to format config: %v", err)), nil
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

// toolListSessions handles the listSessions tool
func (s *Server) toolListSessions(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessions := s.sessions.ListSessions()

	// Format sessions as JSON
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to format sessions: %v", err)), nil
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