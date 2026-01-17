package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/scottgl9/thop/internal/logger"
)

// handleInitialize handles the MCP initialize request
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	logger.Info("MCP client connected: %s v%s (protocol %s)",
		initParams.ClientInfo.Name,
		initParams.ClientInfo.Version,
		initParams.ProtocolVersion)

	// Return server capabilities
	return InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
			Prompts: &PromptsCapability{
				ListChanged: false,
			},
			Logging: &LoggingCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "thop-mcp",
			Version: "1.0.0",
		},
	}, nil
}

// handleInitialized handles the initialized notification
func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) (interface{}, error) {
	logger.Debug("MCP client initialized")
	return nil, nil
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(ctx context.Context, params json.RawMessage) (interface{}, error) {
	tools := []Tool{
		// Session management tools
		{
			Name:        "connect",
			Description: "Connect to an SSH session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session": {
						Type:        "string",
						Description: "Name of the session to connect to",
					},
				},
				Required: []string{"session"},
			},
		},
		{
			Name:        "switch",
			Description: "Switch to a different session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session": {
						Type:        "string",
						Description: "Name of the session to switch to",
					},
				},
				Required: []string{"session"},
			},
		},
		{
			Name:        "close",
			Description: "Close an SSH session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session": {
						Type:        "string",
						Description: "Name of the session to close",
					},
				},
				Required: []string{"session"},
			},
		},
		{
			Name:        "status",
			Description: "Get status of all sessions",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},

		// Command execution tools
		{
			Name:        "execute",
			Description: "Execute a command in the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"command": {
						Type:        "string",
						Description: "Command to execute",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to execute in (uses active session if not specified)",
					},
					"timeout": {
						Type:        "integer",
						Description: "Optional: command timeout in seconds",
						Default:     300,
					},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "executeBackground",
			Description: "Execute a command in the background",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"command": {
						Type:        "string",
						Description: "Command to execute in background",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to execute in",
					},
				},
				Required: []string{"command"},
			},
		},

		// File operations
		{
			Name:        "readFile",
			Description: "Read a file from the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the file to read",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to read from",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "writeFile",
			Description: "Write content to a file in the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the file to write",
					},
					"content": {
						Type:        "string",
						Description: "Content to write to the file",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to write to",
					},
				},
				Required: []string{"path", "content"},
			},
		},
		{
			Name:        "listFiles",
			Description: "List files in a directory",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Directory path to list",
						Default:     ".",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to list from",
					},
				},
			},
		},

		// Environment and state
		{
			Name:        "getEnvironment",
			Description: "Get environment variables from the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session": {
						Type:        "string",
						Description: "Optional: specific session to get environment from",
					},
				},
			},
		},
		{
			Name:        "setEnvironment",
			Description: "Set environment variables in the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"variables": {
						Type:        "object",
						Description: "Key-value pairs of environment variables to set",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to set environment in",
					},
				},
				Required: []string{"variables"},
			},
		},
		{
			Name:        "getCwd",
			Description: "Get current working directory of the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session": {
						Type:        "string",
						Description: "Optional: specific session to get cwd from",
					},
				},
			},
		},
		{
			Name:        "setCwd",
			Description: "Set current working directory of the active session",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Directory path to change to",
					},
					"session": {
						Type:        "string",
						Description: "Optional: specific session to set cwd in",
					},
				},
				Required: []string{"path"},
			},
		},

		// Job management
		{
			Name:        "listJobs",
			Description: "List background jobs",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "getJobOutput",
			Description: "Get output from a background job",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"jobId": {
						Type:        "integer",
						Description: "ID of the job to get output from",
					},
				},
				Required: []string{"jobId"},
			},
		},
		{
			Name:        "killJob",
			Description: "Kill a background job",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"jobId": {
						Type:        "integer",
						Description: "ID of the job to kill",
					},
				},
				Required: []string{"jobId"},
			},
		},

		// Configuration
		{
			Name:        "getConfig",
			Description: "Get thop configuration",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "listSessions",
			Description: "List all configured sessions",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
	}

	return map[string]interface{}{
		"tools": tools,
	}, nil
}

// handleToolCall handles the tools/call request
func (s *Server) handleToolCall(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	logger.Debug("Tool call: %s", callParams.Name)

	// Route to appropriate tool handler
	switch callParams.Name {
	// Session management
	case "connect":
		return s.toolConnect(ctx, callParams.Arguments)
	case "switch":
		return s.toolSwitch(ctx, callParams.Arguments)
	case "close":
		return s.toolClose(ctx, callParams.Arguments)
	case "status":
		return s.toolStatus(ctx, callParams.Arguments)

	// Command execution
	case "execute":
		return s.toolExecute(ctx, callParams.Arguments)
	case "executeBackground":
		return s.toolExecuteBackground(ctx, callParams.Arguments)

	// File operations
	case "readFile":
		return s.toolReadFile(ctx, callParams.Arguments)
	case "writeFile":
		return s.toolWriteFile(ctx, callParams.Arguments)
	case "listFiles":
		return s.toolListFiles(ctx, callParams.Arguments)

	// Environment and state
	case "getEnvironment":
		return s.toolGetEnvironment(ctx, callParams.Arguments)
	case "setEnvironment":
		return s.toolSetEnvironment(ctx, callParams.Arguments)
	case "getCwd":
		return s.toolGetCwd(ctx, callParams.Arguments)
	case "setCwd":
		return s.toolSetCwd(ctx, callParams.Arguments)

	// Job management
	case "listJobs":
		return s.toolListJobs(ctx, callParams.Arguments)
	case "getJobOutput":
		return s.toolGetJobOutput(ctx, callParams.Arguments)
	case "killJob":
		return s.toolKillJob(ctx, callParams.Arguments)

	// Configuration
	case "getConfig":
		return s.toolGetConfig(ctx, callParams.Arguments)
	case "listSessions":
		return s.toolListSessions(ctx, callParams.Arguments)

	default:
		return nil, &JSONRPCError{
			Code:    -32601,
			Message: "Unknown tool",
			Data:    fmt.Sprintf("Tool not found: %s", callParams.Name),
		}
	}
}

// handleResourcesList handles the resources/list request
func (s *Server) handleResourcesList(ctx context.Context, params json.RawMessage) (interface{}, error) {
	resources := []Resource{
		{
			URI:         "session://active",
			Name:        "Active Session",
			Description: "Information about the currently active session",
			MimeType:    "application/json",
		},
		{
			URI:         "session://all",
			Name:        "All Sessions",
			Description: "Information about all configured sessions",
			MimeType:    "application/json",
		},
		{
			URI:         "config://thop",
			Name:        "Thop Configuration",
			Description: "Current thop configuration",
			MimeType:    "application/json",
		},
		{
			URI:         "state://thop",
			Name:        "Thop State",
			Description: "Current thop state including session states",
			MimeType:    "application/json",
		},
	}

	return map[string]interface{}{
		"resources": resources,
	}, nil
}

// handleResourceRead handles the resources/read request
func (s *Server) handleResourceRead(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var readParams ResourceReadParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	var content string
	var err error

	switch readParams.URI {
	case "session://active":
		content, err = s.getActiveSessionResource()
	case "session://all":
		content, err = s.getAllSessionsResource()
	case "config://thop":
		content, err = s.getConfigResource()
	case "state://thop":
		content, err = s.getStateResource()
	default:
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Unknown resource URI",
			Data:    readParams.URI,
		}
	}

	if err != nil {
		return nil, &JSONRPCError{
			Code:    -32603,
			Message: "Failed to read resource",
			Data:    err.Error(),
		}
	}

	return ResourceReadResult{
		Contents: []ResourceContent{
			{
				URI:      readParams.URI,
				MimeType: "application/json",
				Text:     content,
			},
		},
	}, nil
}

// handlePromptsList handles the prompts/list request
func (s *Server) handlePromptsList(ctx context.Context, params json.RawMessage) (interface{}, error) {
	prompts := []Prompt{
		{
			Name:        "deploy",
			Description: "Deploy code to a remote server",
			Arguments: []PromptArgument{
				{
					Name:        "server",
					Description: "Target server session name",
					Required:    true,
				},
				{
					Name:        "branch",
					Description: "Git branch to deploy",
					Required:    false,
				},
			},
		},
		{
			Name:        "debug",
			Description: "Debug an issue on a remote server",
			Arguments: []PromptArgument{
				{
					Name:        "server",
					Description: "Server session to debug on",
					Required:    true,
				},
				{
					Name:        "service",
					Description: "Service name to debug",
					Required:    false,
				},
			},
		},
		{
			Name:        "backup",
			Description: "Create a backup of files on a server",
			Arguments: []PromptArgument{
				{
					Name:        "server",
					Description: "Server session to backup from",
					Required:    true,
				},
				{
					Name:        "path",
					Description: "Path to backup",
					Required:    true,
				},
			},
		},
	}

	return map[string]interface{}{
		"prompts": prompts,
	}, nil
}

// handlePromptGet handles the prompts/get request
func (s *Server) handlePromptGet(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var getParams PromptGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	var messages []PromptMessage
	var description string

	switch getParams.Name {
	case "deploy":
		server := getParams.Arguments["server"].(string)
		branch, _ := getParams.Arguments["branch"].(string)
		if branch == "" {
			branch = "main"
		}
		description = fmt.Sprintf("Deploy %s branch to %s server", branch, server)
		messages = []PromptMessage{
			{
				Role: "user",
				Content: Content{
					Type: "text",
					Text: fmt.Sprintf("Please deploy the %s branch to the %s server. "+
						"1. Connect to %s\n"+
						"2. Navigate to the deployment directory\n"+
						"3. Pull the latest changes from %s branch\n"+
						"4. Run any build or deployment scripts\n"+
						"5. Verify the deployment was successful",
						branch, server, server, branch),
				},
			},
		}

	case "debug":
		server := getParams.Arguments["server"].(string)
		service, _ := getParams.Arguments["service"].(string)

		debugText := fmt.Sprintf("Please help me debug an issue on the %s server.", server)
		if service != "" {
			debugText = fmt.Sprintf("Please help me debug the %s service on the %s server.", service, server)
		}

		description = fmt.Sprintf("Debug issue on %s", server)
		messages = []PromptMessage{
			{
				Role: "user",
				Content: Content{
					Type: "text",
					Text: debugText + "\n" +
						"1. Connect to the server\n" +
						"2. Check system resources (CPU, memory, disk)\n" +
						"3. Review relevant logs\n" +
						"4. Identify any errors or issues\n" +
						"5. Suggest fixes or next steps",
				},
			},
		}

	case "backup":
		server := getParams.Arguments["server"].(string)
		path := getParams.Arguments["path"].(string)

		description = fmt.Sprintf("Backup %s from %s", path, server)
		messages = []PromptMessage{
			{
				Role: "user",
				Content: Content{
					Type: "text",
					Text: fmt.Sprintf("Please create a backup of %s on the %s server.\n"+
						"1. Connect to %s\n"+
						"2. Create a timestamped backup of %s\n"+
						"3. Compress the backup\n"+
						"4. Verify the backup was created successfully\n"+
						"5. Report the backup location and size",
						path, server, server, path),
				},
			},
		}

	default:
		return nil, &JSONRPCError{
			Code:    -32602,
			Message: "Unknown prompt",
			Data:    fmt.Sprintf("Prompt not found: %s", getParams.Name),
		}
	}

	return PromptGetResult{
		Description: description,
		Messages:    messages,
	}, nil
}

// handlePing handles ping requests
func (s *Server) handlePing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"pong": true,
	}, nil
}

// handleCancelled handles cancellation notifications
func (s *Server) handleCancelled(ctx context.Context, params json.RawMessage) (interface{}, error) {
	logger.Debug("Received cancellation notification")
	// TODO: Implement request cancellation
	return nil, nil
}

// handleProgress handles progress notifications
func (s *Server) handleProgress(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var progressParams ProgressParams
	if err := json.Unmarshal(params, &progressParams); err != nil {
		logger.Error("Failed to parse progress params: %v", err)
		return nil, nil
	}

	logger.Debug("Progress update: token=%s progress=%f/%f",
		progressParams.ProgressToken,
		progressParams.Progress,
		progressParams.Total)

	return nil, nil
}