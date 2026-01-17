# MCP Server for thop

## Overview

thop includes a built-in MCP (Model Context Protocol) server that allows AI agents to have full control over thop's functionality through a standardized protocol. This enables seamless integration with AI coding assistants and agents.

## Starting the MCP Server

To run thop as an MCP server:

```bash
thop --mcp
```

The MCP server communicates via JSON-RPC over stdin/stdout, following the MCP protocol specification.

## Available Tools

The MCP server exposes a streamlined set of tools for AI agents:

### Session Management

- **connect** - Connect to an SSH session
  - `session` (string, required): Name of the session to connect to

- **switch** - Switch to a different session
  - `session` (string, required): Name of the session to switch to

- **close** - Close an SSH session
  - `session` (string, required): Name of the session to close

- **status** - Get status of all sessions
  - No parameters required

### Command Execution

- **execute** - Execute a command in the active session
  - `command` (string, required): Command to execute
  - `session` (string, optional): Specific session to execute in
  - `timeout` (integer, optional): Command timeout in seconds (default: session/global config or 300s)
  - `background` (boolean, optional): Run command in background (default: false, not yet implemented)

  This is the primary tool for interacting with sessions. Use it to run any command including file operations (`cat`, `ls`, `echo`, etc.), environment management (`export`, `env`), directory navigation (`cd`, `pwd`), and more.

  **Timeout Behavior**: The timeout is determined in this order:
  1. Explicit `timeout` parameter (if provided)
  2. Session-specific `command_timeout` in config
  3. Global `command_timeout` setting
  4. Default 300 seconds (5 minutes)

### Design Philosophy

The MCP server follows a minimalist design philosophy:

- **Single execution tool**: The `execute` tool handles all command execution needs, avoiding duplication
- **Use shell commands directly**: Instead of specialized tools for file operations, environment management, or directory navigation, use standard shell commands through `execute`
- **Resources for read-only data**: Configuration and state information is exposed through MCP resources rather than duplicate tools

#### Examples

```bash
# Read a file
execute: "cat /path/to/file"

# Write a file
execute: "echo 'content' > /path/to/file"

# List files
execute: "ls -la /path"

# Change directory (persists in session)
execute: "cd /new/directory"

# Set environment variable
execute: "export VAR=value"

# Check current directory
execute: "pwd"
```

## Configuration

### Timeout Configuration

You can configure command timeouts at three levels:

**Global Default** (`~/.config/thop/config.toml`):
```toml
[settings]
command_timeout = 300  # Default for all sessions
```

**Per-Session Override**:
```toml
[sessions.slow-server]
type = "ssh"
host = "slow.example.com"
command_timeout = 600  # Higher timeout for slow server
```

**Per-Command Override** (via MCP execute tool):
```json
{
  "name": "execute",
  "arguments": {
    "command": "npm run build",
    "timeout": 900
  }
}
```

Priority order: command parameter > session config > global setting > default (300s)

## Available Resources

The MCP server provides the following resources:

- **session://active** - Information about the currently active session
- **session://all** - Information about all configured sessions
- **config://thop** - Current thop configuration
- **state://thop** - Current thop state including session states

## Example Integration

### Using with Claude Desktop

Add thop as an MCP server in your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "thop": {
      "command": "thop",
      "args": ["--mcp"],
      "env": {}
    }
  }
}
```

### Using with Other AI Agents

Any AI agent that supports the MCP protocol can use thop by running:

```bash
thop --mcp
```

And communicating via JSON-RPC over stdin/stdout.

## Protocol Details

The MCP server implements the MCP protocol version 2024-11-05 and supports:

- **Tools**: Full support for tool discovery and invocation
- **Resources**: Read-only access to session and configuration data
- **Logging**: Structured logging support

## Example Tool Call

Here's an example of calling the `execute` tool via JSON-RPC:

Request:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "id": 1,
  "params": {
    "name": "execute",
    "arguments": {
      "command": "ls -la",
      "session": "prod"
    }
  }
}
```

Response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "total 48\ndrwxr-xr-x  5 user user 4096 Jan 16 12:00 .\ndrwxr-xr-x 10 user user 4096 Jan 16 11:00 ..\n..."
      }
    ],
    "isError": false
  }
}
```

## Error Handling

The MCP server uses structured error codes for programmatic error handling.

### JSON-RPC Errors

Protocol-level errors follow JSON-RPC 2.0 specification:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found",
    "data": "Unknown method: invalid/method"
  }
}
```

### Tool Errors

Tool errors are returned as successful responses with `isError: true` and include structured error codes:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "[SESSION_NOT_FOUND] Session 'invalid-session' not found\n\nSuggestion: Use /status to see available sessions or /add-session to create a new one\n\nSession: invalid-session"
      }
    ],
    "isError": true
  }
}
```

### Error Codes

#### Session Errors
- `SESSION_NOT_FOUND` - Session does not exist
- `SESSION_NOT_CONNECTED` - Session exists but is not connected
- `SESSION_ALREADY_EXISTS` - Attempting to create duplicate session
- `NO_ACTIVE_SESSION` - No session is currently active
- `CANNOT_CLOSE_LOCAL` - Cannot close the local session

#### Connection Errors
- `CONNECTION_FAILED` - Generic connection failure
- `AUTH_FAILED` - Authentication failed (generic)
- `AUTH_KEY_FAILED` - SSH key authentication failed
- `AUTH_PASSWORD_FAILED` - Password authentication failed
- `HOST_KEY_UNKNOWN` - Host key not in known_hosts
- `HOST_KEY_MISMATCH` - Host key mismatch (security)
- `CONNECTION_TIMEOUT` - Connection attempt timed out
- `CONNECTION_REFUSED` - Connection refused by host

#### Command Execution Errors
- `COMMAND_FAILED` - Command execution failed
- `COMMAND_TIMEOUT` - Command execution timed out
- `COMMAND_NOT_FOUND` - Command not found in PATH
- `PERMISSION_DENIED` - Insufficient permissions

#### Parameter Errors
- `INVALID_PARAMETER` - Parameter has invalid value
- `MISSING_PARAMETER` - Required parameter not provided

#### Feature Errors
- `NOT_IMPLEMENTED` - Feature not yet implemented
- `OPERATION_FAILED` - Generic operation failure

### Error Response Format

All tool errors include:
- **Error Code**: Structured code for programmatic handling
- **Message**: Human-readable error description
- **Session**: Session name (if applicable)
- **Suggestion**: Actionable suggestion for resolving the error

Example error with all fields:

```json
{
  "content": [
    {
      "type": "text",
      "text": "[AUTH_KEY_FAILED] SSH key authentication failed\n\nSuggestion: Use /auth to provide a password or check your SSH key configuration\n\nSession: prod"
    }
  ],
  "isError": true
}
```

## Security Considerations

- The MCP server has full access to all thop functionality
- It can execute commands on local and remote systems
- It can read and write files
- Use appropriate security measures when exposing the MCP server
- Consider running in a restricted environment or container

## Future Enhancements

- Background job management
- Session transcript recording
- File transfer capabilities
- Interactive command support
- Custom tool registration
- WebSocket transport support