package cli

import (
	"github.com/scottgl9/thop/internal/logger"
	"github.com/scottgl9/thop/internal/mcp"
)

// runMCP runs thop as an MCP server
func (a *App) runMCP() error {
	logger.Info("Starting MCP server mode")

	// Create MCP server
	server := mcp.NewServer(a.config, a.sessions, a.state)

	// Run the server (blocks until stopped)
	return server.Run()
}