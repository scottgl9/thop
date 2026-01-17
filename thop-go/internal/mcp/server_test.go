package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/session"
	"github.com/scottgl9/thop/internal/state"
)

func TestMCPServer_Initialize(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Create input/output buffers
	input := &bytes.Buffer{}
	output := &bytes.Buffer{}
	server.SetIO(input, output)

	// Send initialize request
	initRequest := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	}

	requestData, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatal(err)
	}
	input.Write(requestData)
	input.Write([]byte("\n"))

	// Handle the message
	server.handleMessage(requestData)

	// Parse the response
	responseData := output.Bytes()
	if len(responseData) == 0 {
		t.Fatal("No response received")
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(responseData[:len(responseData)-1], &response); err != nil { // Remove trailing newline
		t.Fatal(err)
	}

	// Check response
	if response.Error != nil {
		t.Fatalf("Initialize failed: %v", response.Error)
	}

	// Check result
	resultData, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatal(err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resultData, &initResult); err != nil {
		t.Fatal(err)
	}

	if initResult.ProtocolVersion != MCPVersion {
		t.Errorf("Expected protocol version %s, got %s", MCPVersion, initResult.ProtocolVersion)
	}

	if initResult.ServerInfo.Name != "thop-mcp" {
		t.Errorf("Expected server name 'thop-mcp', got %s", initResult.ServerInfo.Name)
	}

	// Test capabilities
	if initResult.Capabilities.Tools == nil {
		t.Error("Tools capability not set")
	}

	if initResult.Capabilities.Resources == nil {
		t.Error("Resources capability not set")
	}

	if initResult.Capabilities.Prompts == nil {
		t.Error("Prompts capability not set")
	}
}

func TestMCPServer_ToolsList(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Test tools/list
	ctx := context.Background()
	result, err := server.handleToolsList(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	toolsResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Invalid tools list result type")
	}

	tools, ok := toolsResult["tools"].([]Tool)
	if !ok {
		t.Fatal("Invalid tools array type")
	}

	// Check we have tools
	if len(tools) == 0 {
		t.Error("No tools returned")
	}

	// Check for specific tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"connect", "switch", "close", "status",
		"execute", "executeBackground",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s not found", expected)
		}
	}

	// Ensure we only have these 6 tools
	if len(tools) != 6 {
		t.Errorf("Expected exactly 6 tools, got %d", len(tools))
	}
}

func TestMCPServer_ToolCall_Status(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
			"test-ssh": {
				Type: "ssh",
				Host: "test.example.com",
				User: "testuser",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Test status tool
	ctx := context.Background()
	params := json.RawMessage(`{"name":"status","arguments":{}}`)

	result, err := server.handleToolCall(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	toolResult, ok := result.(ToolCallResult)
	if !ok {
		t.Fatal("Invalid tool result type")
	}

	if toolResult.IsError {
		t.Error("Status tool returned error")
	}

	if len(toolResult.Content) == 0 {
		t.Error("No content returned from status tool")
	}

	// Check content contains JSON
	content := toolResult.Content[0]
	if content.Type != "text" {
		t.Error("Expected text content")
	}

	// Try to parse as JSON (sessions list)
	var sessions []interface{}
	if err := json.Unmarshal([]byte(content.Text), &sessions); err != nil {
		t.Errorf("Failed to parse sessions JSON: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}
}

func TestMCPServer_ResourcesList(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Test resources/list
	ctx := context.Background()
	result, err := server.handleResourcesList(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	resourcesResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Invalid resources list result type")
	}

	resources, ok := resourcesResult["resources"].([]Resource)
	if !ok {
		t.Fatal("Invalid resources array type")
	}

	// Check we have resources
	if len(resources) == 0 {
		t.Error("No resources returned")
	}

	// Check for specific resources
	resourceURIs := make(map[string]bool)
	for _, resource := range resources {
		resourceURIs[resource.URI] = true
	}

	expectedResources := []string{
		"session://active",
		"session://all",
		"config://thop",
		"state://thop",
	}

	for _, expected := range expectedResources {
		if !resourceURIs[expected] {
			t.Errorf("Expected resource %s not found", expected)
		}
	}
}

func TestMCPServer_PromptsList(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Test prompts/list
	ctx := context.Background()
	result, err := server.handlePromptsList(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	promptsResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Invalid prompts list result type")
	}

	prompts, ok := promptsResult["prompts"].([]Prompt)
	if !ok {
		t.Fatal("Invalid prompts array type")
	}

	// Check we have prompts
	if len(prompts) == 0 {
		t.Error("No prompts returned")
	}

	// Check for specific prompts
	promptNames := make(map[string]bool)
	for _, prompt := range prompts {
		promptNames[prompt.Name] = true
	}

	expectedPrompts := []string{
		"deploy",
		"debug",
		"backup",
	}

	for _, expected := range expectedPrompts {
		if !promptNames[expected] {
			t.Errorf("Expected prompt %s not found", expected)
		}
	}
}

func TestMCPServer_Ping(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Settings: config.Settings{
			DefaultSession: "local",
		},
		Sessions: map[string]config.Session{
			"local": {
				Type:  "local",
				Shell: "/bin/bash",
			},
		},
	}

	// Create state manager
	stateMgr := state.NewManager("/tmp/test-state.json")

	// Create session manager
	sessionMgr := session.NewManager(cfg, stateMgr)

	// Create MCP server
	server := NewServer(cfg, sessionMgr, stateMgr)

	// Test ping
	ctx := context.Background()
	result, err := server.handlePing(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	pingResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Invalid ping result type")
	}

	if pong, ok := pingResult["pong"].(bool); !ok || !pong {
		t.Error("Expected pong: true")
	}
}

func TestMCPServer_JSONRPCParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid request",
			input:   `{"jsonrpc":"2.0","method":"ping","id":1}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{"jsonrpc":"2.0","method":}`,
			wantErr: true,
		},
		{
			name:    "notification (no ID)",
			input:   `{"jsonrpc":"2.0","method":"cancelled"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Settings: config.Settings{
					DefaultSession: "local",
				},
				Sessions: map[string]config.Session{
					"local": {
						Type:  "local",
						Shell: "/bin/bash",
					},
				},
			}

			// Create state manager
			stateMgr := state.NewManager("/tmp/test-state.json")

			// Create session manager
			sessionMgr := session.NewManager(cfg, stateMgr)

			// Create MCP server
			server := NewServer(cfg, sessionMgr, stateMgr)

			// Create output buffer
			output := &bytes.Buffer{}
			server.SetIO(strings.NewReader(tt.input), output)

			// Handle message
			err := server.handleMessage([]byte(tt.input))

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}