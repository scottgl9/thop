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
		"execute",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s not found", expected)
		}
	}

	// Ensure we only have these 5 tools
	if len(tools) != 5 {
		t.Errorf("Expected exactly 5 tools, got %d", len(tools))
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
// Test helper function
func createTestServer() *Server {
	cfg := &config.Config{
		Settings: config.Settings{DefaultSession: "local"},
		Sessions: map[string]config.Session{
			"local": {Type: "local", Shell: "/bin/bash"},
		},
	}
	return NewServer(cfg, session.NewManager(cfg, state.NewManager("/tmp/test-mcp.json")), state.NewManager("/tmp/test-mcp.json"))
}

func TestMCPServer_ToolCall_Execute(t *testing.T) {
	srv := createTestServer()
	tests := []struct {
		name string
		params string
		wantErr bool
	}{
		{"valid", `{"name":"execute","arguments":{"command":"echo test"}}`, false},
		{"background", `{"name":"execute","arguments":{"command":"sleep 1","background":true}}`, true},
		{"no command", `{"name":"execute","arguments":{}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := srv.handleToolCall(context.Background(), json.RawMessage(tt.params))
			if tr, ok := res.(ToolCallResult); ok && tr.IsError != tt.wantErr {
				t.Errorf("wantErr=%v, got IsError=%v", tt.wantErr, tr.IsError)
			}
		})
	}
}

func TestMCPServer_ResourceRead(t *testing.T) {
	srv := createTestServer()
	tests := []struct{ uri string; wantErr bool }{
		{"session://active", false},
		{"session://all", false},
		{"config://thop", false},
		{"state://thop", false},
		{"unknown://x", true},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			_, err := srv.handleResourceRead(context.Background(), json.RawMessage(`{"uri":"`+tt.uri+`"}`))
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}


func TestMCPServer_Notifications(t *testing.T) {
	srv := createTestServer()
	if _, err := srv.handleInitialized(context.Background(), nil); err != nil {
		t.Error(err)
	}
	if _, err := srv.handleCancelled(context.Background(), nil); err != nil {
		t.Error(err)
	}
	if _, err := srv.handleProgress(context.Background(), json.RawMessage(`{"progressToken":"t","progress":1}`)); err != nil {
		t.Error(err)
	}
}

func TestMCPServer_SendMethods(t *testing.T) {
	srv := createTestServer()
	buf := &bytes.Buffer{}
	srv.SetIO(nil, buf)
	
	if err := srv.sendError(1, -32600, "test", "data"); err != nil {
		t.Error(err)
	}
	buf.Reset()
	
	if err := srv.sendNotification("test", map[string]string{"k":"v"}); err != nil {
		t.Error(err)
	}
}

func TestMCPServer_Errors(t *testing.T) {
	srv := createTestServer()
	buf := &bytes.Buffer{}
	srv.SetIO(nil, buf)
	
	msg := &JSONRPCMessage{JSONRPC: "2.0", Method: "unknown", ID: 1}
	if err := srv.handleRequest(msg); err != nil {
		t.Error(err)
	}
	
	var resp JSONRPCResponse
	json.Unmarshal(buf.Bytes()[:buf.Len()-1], &resp)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Error("Expected method not found error")
	}
}

func TestMCPServer_Stop(t *testing.T) {
	srv := createTestServer()
	srv.Stop()
	select {
	case <-srv.ctx.Done():
	default:
		t.Error("Context should be cancelled")
	}
}

func TestJSONRPCError_Error(t *testing.T) {
	err := &JSONRPCError{Code: -1, Message: "test"}
	if err.Error() != "test" {
		t.Errorf("Expected 'test', got '%s'", err.Error())
	}
}

func TestMCPServer_ToolCall_Connect(t *testing.T) {
	srv := createTestServer()
	tests := []struct {
		name string
		params string
		wantErr bool
	}{
		{"missing session", `{"name":"connect","arguments":{}}`, true},
		{"nonexistent session", `{"name":"connect","arguments":{"session":"invalid"}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := srv.handleToolCall(context.Background(), json.RawMessage(tt.params))
			if tr, ok := res.(ToolCallResult); ok && tr.IsError != tt.wantErr {
				t.Errorf("wantErr=%v, got IsError=%v", tt.wantErr, tr.IsError)
			}
		})
	}
}

func TestMCPServer_ToolCall_Switch(t *testing.T) {
	srv := createTestServer()
	tests := []struct {
		name string
		params string
		wantErr bool
	}{
		{"missing session", `{"name":"switch","arguments":{}}`, true},
		{"nonexistent session", `{"name":"switch","arguments":{"session":"invalid"}}`, true},
		{"valid switch", `{"name":"switch","arguments":{"session":"local"}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := srv.handleToolCall(context.Background(), json.RawMessage(tt.params))
			if tr, ok := res.(ToolCallResult); ok && tr.IsError != tt.wantErr {
				t.Errorf("wantErr=%v, got IsError=%v", tt.wantErr, tr.IsError)
			}
		})
	}
}

func TestMCPServer_ToolCall_Close(t *testing.T) {
	srv := createTestServer()
	tests := []struct {
		name string
		params string
		wantErr bool
	}{
		{"missing session", `{"name":"close","arguments":{}}`, true},
		{"nonexistent session", `{"name":"close","arguments":{"session":"invalid"}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := srv.handleToolCall(context.Background(), json.RawMessage(tt.params))
			if tr, ok := res.(ToolCallResult); ok && tr.IsError != tt.wantErr {
				t.Errorf("wantErr=%v, got IsError=%v", tt.wantErr, tr.IsError)
			}
		})
	}
}

func TestMCPServer_HandleToolCall_InvalidParamsError(t *testing.T) {
	srv := createTestServer()
	_, err := srv.handleToolCall(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("Expected error for invalid params")
	}
	if rpcErr, ok := err.(*JSONRPCError); !ok || rpcErr.Code != -32602 {
		t.Error("Expected invalid params error")
	}
}

func TestMCPServer_HandleToolCall_UnknownTool(t *testing.T) {
	srv := createTestServer()
	_, err := srv.handleToolCall(context.Background(), json.RawMessage(`{"name":"unknownTool","arguments":{}}`))
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
	if rpcErr, ok := err.(*JSONRPCError); !ok || rpcErr.Code != -32601 {
		t.Error("Expected method not found error")
	}
}

func TestMCPServer_ResourceRead_InvalidParamsError(t *testing.T) {
	srv := createTestServer()
	_, err := srv.handleResourceRead(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("Expected error for invalid params")
	}
}


func TestMCPServer_HandleInitialize_InvalidParamsError(t *testing.T) {
	srv := createTestServer()
	_, err := srv.handleInitialize(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("Expected error for invalid params")
	}
	if rpcErr, ok := err.(*JSONRPCError); !ok || rpcErr.Code != -32602 {
		t.Error("Expected invalid params error")
	}
}

func TestMCPServer_HandleProgress_InvalidParams(t *testing.T) {
	srv := createTestServer()
	// Should not error even with invalid params (logs and continues)
	if _, err := srv.handleProgress(context.Background(), json.RawMessage(`{invalid}`)); err != nil {
		t.Error(err)
	}
}
