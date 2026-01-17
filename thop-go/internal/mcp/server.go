package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/scottgl9/thop/internal/config"
	"github.com/scottgl9/thop/internal/logger"
	"github.com/scottgl9/thop/internal/session"
	"github.com/scottgl9/thop/internal/state"
)

// MCPVersion is the supported MCP protocol version
const MCPVersion = "2024-11-05"

// Server implements the MCP (Model Context Protocol) server for thop
type Server struct {
	config   *config.Config
	sessions *session.Manager
	state    *state.Manager

	// I/O channels for JSON-RPC communication
	input  io.Reader
	output io.Writer

	// Request handling
	mu       sync.Mutex
	handlers map[string]HandlerFunc

	// Server state
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// HandlerFunc is the signature for JSON-RPC method handlers
type HandlerFunc func(context.Context, json.RawMessage) (interface{}, error)

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, sessions *session.Manager, state *state.Manager) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:   cfg,
		sessions: sessions,
		state:    state,
		input:    os.Stdin,
		output:   os.Stdout,
		handlers: make(map[string]HandlerFunc),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Register handlers
	s.registerHandlers()

	return s
}

// SetIO sets custom input/output streams (useful for testing)
func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.input = input
	s.output = output
}

// registerHandlers registers all JSON-RPC method handlers
func (s *Server) registerHandlers() {
	// MCP protocol methods
	s.handlers["initialize"] = s.handleInitialize
	s.handlers["initialized"] = s.handleInitialized
	s.handlers["tools/list"] = s.handleToolsList
	s.handlers["tools/call"] = s.handleToolCall
	s.handlers["resources/list"] = s.handleResourcesList
	s.handlers["resources/read"] = s.handleResourceRead
	s.handlers["prompts/list"] = s.handlePromptsList
	s.handlers["prompts/get"] = s.handlePromptGet
	s.handlers["ping"] = s.handlePing

	// Notification handlers
	s.handlers["cancelled"] = s.handleCancelled
	s.handlers["progress"] = s.handleProgress
}

// Run starts the MCP server and processes incoming requests
func (s *Server) Run() error {
	logger.Info("Starting MCP server")
	s.running = true
	defer func() {
		s.running = false
		s.cancel()
	}()

	scanner := bufio.NewScanner(s.input)
	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
			line := scanner.Bytes()
			if err := s.handleMessage(line); err != nil {
				logger.Error("Error handling message: %v", err)
				// Send error response
				s.sendError(nil, -32603, "Internal error", err.Error())
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("input scanner error: %w", err)
	}

	return nil
}

// Stop gracefully stops the MCP server
func (s *Server) Stop() {
	logger.Info("Stopping MCP server")
	s.cancel()
}

// handleMessage processes a single JSON-RPC message
func (s *Server) handleMessage(data []byte) error {
	var msg JSONRPCMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	// Handle request
	if msg.Method != "" {
		return s.handleRequest(&msg)
	}

	// Handle response (if we're waiting for one)
	// For now, we don't send requests, so we don't handle responses

	return nil
}

// handleRequest processes a JSON-RPC request
func (s *Server) handleRequest(msg *JSONRPCMessage) error {
	logger.Debug("Handling request: method=%s id=%v", msg.Method, msg.ID)

	handler, ok := s.handlers[msg.Method]
	if !ok {
		return s.sendError(msg.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", msg.Method))
	}

	// Execute handler
	result, err := handler(s.ctx, msg.Params)
	if err != nil {
		// Check if it's already a JSON-RPC error
		if rpcErr, ok := err.(*JSONRPCError); ok {
			return s.sendErrorResponse(msg.ID, rpcErr)
		}
		// Generic error
		return s.sendError(msg.ID, -32603, "Internal error", err.Error())
	}

	// Send successful response (only if it's a request with an ID)
	if msg.ID != nil {
		return s.sendResponse(msg.ID, result)
	}

	return nil
}

// sendResponse sends a successful JSON-RPC response
func (s *Server) sendResponse(id interface{}, result interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.output.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if _, err := s.output.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// sendError sends a JSON-RPC error response
func (s *Server) sendError(id interface{}, code int, message string, data string) error {
	rpcErr := &JSONRPCError{
		Code:    code,
		Message: message,
	}
	if data != "" {
		rpcErr.Data = data
	}
	return s.sendErrorResponse(id, rpcErr)
}

// sendErrorResponse sends a JSON-RPC error response
func (s *Server) sendErrorResponse(id interface{}, rpcErr *JSONRPCError) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal error response: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.output.Write(data); err != nil {
		return fmt.Errorf("failed to write error response: %w", err)
	}
	if _, err := s.output.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// sendNotification sends a JSON-RPC notification (no ID)
func (s *Server) sendNotification(method string, params interface{}) error {
	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  nil,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		notification.Params = data
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.output.Write(data); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}
	if _, err := s.output.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}