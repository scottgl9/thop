# MCP Prompt Templates - Implementation Guide

This document contains the implementation of MCP prompt templates that were removed from thop to reduce context bloat and maintain minimalism. If you need to add prompt templates back in the future, use this as a reference.

## Why Were Prompts Removed?

1. **Context Bloat**: Added ~500-700 tokens per MCP session
2. **Limited Value**: Generic instructions that AI agents already know
3. **Redundancy**: Users can just ask naturally ("deploy to prod") instead of invoking templates
4. **Complexity**: Extra code to maintain without clear benefit

## When to Add Prompts Back

Consider re-adding prompts if:
- You have **domain-specific** workflows that require specialized knowledge
- You need **complex multi-step procedures** with specific thop context
- You want **reusable templates** that leverage thop's unique features
- You're building **standardized workflows** for teams

## Implementation Reference

### 1. Add Prompts Capability to Server Initialization

In `handlers.go`, update `handleInitialize`:

```go
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, error) {
	// ... existing code ...

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
			Prompts: &PromptsCapability{  // ADD THIS
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
```

### 2. Register Prompt Handlers

In `server.go`, add to `registerHandlers`:

```go
func (s *Server) registerHandlers() {
	// ... existing handlers ...
	s.handlers["prompts/list"] = s.handlePromptsList
	s.handlers["prompts/get"] = s.handlePromptGet
}
```

### 3. Implement Prompt List Handler

Add to `handlers.go`:

```go
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
```

### 4. Implement Prompt Get Handler

Add to `handlers.go`:

```go
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
```

### 5. Add Tests

Add to `server_test.go`:

```go
func TestMCPServer_PromptsList(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultSession: "local"},
		Sessions: map[string]config.Session{
			"local": {Type: "local", Shell: "/bin/bash"},
		},
	}
	stateMgr := state.NewManager("/tmp/test-state.json")
	sessionMgr := session.NewManager(cfg, stateMgr)
	server := NewServer(cfg, sessionMgr, stateMgr)

	ctx := context.Background()
	result, err := server.handlePromptsList(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	promptsResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Invalid prompts list result type")
	}

	prompts, ok := promptsResult["prompts"].([]Prompt)
	if !ok {
		t.Fatal("Invalid prompts array type")
	}

	if len(prompts) == 0 {
		t.Error("No prompts returned")
	}

	promptNames := make(map[string]bool)
	for _, prompt := range prompts {
		promptNames[prompt.Name] = true
	}

	expectedPrompts := []string{"deploy", "debug", "backup"}
	for _, expected := range expectedPrompts {
		if !promptNames[expected] {
			t.Errorf("Expected prompt %s not found", expected)
		}
	}
}

func TestMCPServer_PromptGet(t *testing.T) {
	srv := createTestServer()
	tests := []struct{ name string; args string; wantErr bool }{
		{"deploy", `{"server":"s","branch":"main"}`, false},
		{"debug", `{"server":"s","service":"api"}`, false},
		{"backup", `{"server":"s","path":"/data"}`, false},
		{"unknown", `{}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.handlePromptGet(context.Background(),
				json.RawMessage(`{"name":"`+tt.name+`","arguments":`+tt.args+`}`))
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}
```

## Example Prompts for Thop-Specific Workflows

If you do add prompts back, make them **thop-specific** to add real value:

### Example: Multi-Server Deployment

```go
{
	Name:        "multi-deploy",
	Description: "Deploy to multiple servers in sequence with rollback",
	Arguments: []PromptArgument{
		{Name: "servers", Description: "Comma-separated session names", Required: true},
		{Name: "branch", Description: "Branch to deploy", Required: true},
		{Name: "healthcheck", Description: "Health check command", Required: false},
	},
}
```

### Example: Server Migration

```go
{
	Name:        "migrate-data",
	Description: "Migrate data between two thop sessions",
	Arguments: []PromptArgument{
		{Name: "source", Description: "Source session name", Required: true},
		{Name: "target", Description: "Target session name", Required: true},
		{Name: "paths", Description: "Paths to migrate", Required: true},
	},
}
```

### Example: Parallel Execution

```go
{
	Name:        "parallel-check",
	Description: "Run health checks across all sessions in parallel",
	Arguments: []PromptArgument{
		{Name: "command", Description: "Command to run on each session", Required: true},
	},
}
```

## Best Practices for Prompts

1. **Make them thop-specific** - Leverage session management, state persistence
2. **Avoid generic tasks** - Don't duplicate what AI already knows
3. **Keep them concise** - Minimize context bloat
4. **Add real value** - Complex workflows that benefit from templates
5. **Document clearly** - Explain when and how to use each prompt

## Conclusion

Only add prompts back if they provide **clear value** beyond what natural language requests can achieve. Focus on workflows that are:
- Complex and multi-step
- Require specific thop features
- Standardized across teams
- Worth the context overhead

Otherwise, let AI agents work with direct natural language instructions!