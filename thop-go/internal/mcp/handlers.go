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

		// Command execution tool
		{
			Name:        "execute",
			Description: "Execute a command in the active session (optionally in background)",
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
						Description: "Optional: command timeout in seconds (ignored if background is true)",
						Default:     300,
					},
					"background": {
						Type:        "boolean",
						Description: "Optional: run command in background (default: false)",
						Default:     false,
					},
				},
				Required: []string{"command"},
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