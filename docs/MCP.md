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

The MCP server exposes the following tools for AI agents:

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
  - `timeout` (integer, optional): Command timeout in seconds (default: 300)

- **executeBackground** - Execute a command in the background (not yet implemented)
  - `command` (string, required): Command to execute in background
  - `session` (string, optional): Specific session to execute in

### File Operations

- **readFile** - Read a file from the active session
  - `path` (string, required): Path to the file to read
  - `session` (string, optional): Specific session to read from

- **writeFile** - Write content to a file in the active session
  - `path` (string, required): Path to the file to write
  - `content` (string, required): Content to write to the file
  - `session` (string, optional): Specific session to write to

- **listFiles** - List files in a directory
  - `path` (string, optional): Directory path to list (default: ".")
  - `session` (string, optional): Specific session to list from

### Environment and State

- **getEnvironment** - Get environment variables from the active session
  - `session` (string, optional): Specific session to get environment from

- **setEnvironment** - Set environment variables in the active session
  - `variables` (object, required): Key-value pairs of environment variables
  - `session` (string, optional): Specific session to set environment in

- **getCwd** - Get current working directory of the active session
  - `session` (string, optional): Specific session to get cwd from

- **setCwd** - Set current working directory of the active session
  - `path` (string, required): Directory path to change to
  - `session` (string, optional): Specific session to set cwd in

### Job Management (Not Yet Implemented)

- **listJobs** - List background jobs
- **getJobOutput** - Get output from a background job
- **killJob** - Kill a background job

### Configuration

- **getConfig** - Get thop configuration
  - No parameters required

- **listSessions** - List all configured sessions
  - No parameters required

## Available Resources

The MCP server provides the following resources:

- **session://active** - Information about the currently active session
- **session://all** - Information about all configured sessions
- **config://thop** - Current thop configuration
- **state://thop** - Current thop state including session states

## Available Prompts

The MCP server includes pre-defined prompt templates:

- **deploy** - Deploy code to a remote server
  - Arguments: `server` (required), `branch` (optional)

- **debug** - Debug an issue on a remote server
  - Arguments: `server` (required), `service` (optional)

- **backup** - Create a backup of files on a server
  - Arguments: `server` (required), `path` (required)

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
- **Prompts**: Pre-defined prompt templates for common operations
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

The MCP server returns structured errors following the JSON-RPC 2.0 specification:

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

Tool errors are returned as successful responses with `isError: true`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Session not found: invalid-session"
      }
    ],
    "isError": true
  }
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