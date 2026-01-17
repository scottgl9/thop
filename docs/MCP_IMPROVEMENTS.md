# MCP Server Improvement Recommendations

Based on research of existing SSH MCP server implementations and MCP best practices, here are recommended improvements for thop's MCP server.

## Research Summary

Analyzed implementations:
- **tufantunc/ssh-mcp**: TypeScript-based with timeout handling and sudo support
- **AiondaDotCom/mcp-ssh**: Native SSH with config auto-discovery and SCP support
- **Official MCP filesystem server**: Advanced file operations with read/write separation
- **MCP best practices**: Architecture, security, and performance guidelines

## Priority 1: Essential Improvements

### 1. SSH Config Auto-Discovery
**Current State**: Sessions must be manually configured in `config.toml`

**Recommendation**: Automatically discover SSH hosts from:
- `~/.ssh/config` - Parse host definitions, jump hosts, ports, etc.
- `~/.ssh/known_hosts` - Discover previously connected hosts

**Benefits**:
- Zero configuration for existing SSH setups
- Agents can immediately work with known hosts
- Supports complex SSH configurations (jump hosts, custom ports)

**Implementation**:
```go
// Add to session package
func DiscoverSSHHosts() ([]SessionConfig, error) {
    // Parse ~/.ssh/config using ssh_config library
    // Parse ~/.ssh/known_hosts
    // Return list of available sessions
}
```

**Reference**: AiondaDotCom/mcp-ssh approach

### 2. Command Timeout Handling
**Current State**: No timeout protection for long-running commands

**Recommendation**:
- Add configurable timeout per session (default: 300s)
- Add timeout parameter to execute tool
- Gracefully abort processes on timeout
- Return timeout errors with context

**Benefits**:
- Prevents hung connections
- Protects against runaway processes
- Better user experience for agents

**Implementation**:
```go
type ExecuteParams struct {
    Command    string `json:"command"`
    Session    string `json:"session,omitempty"`
    Timeout    int    `json:"timeout,omitempty"` // seconds, default 300
    Background bool   `json:"background,omitempty"`
}
```

**Reference**: tufantunc/ssh-mcp with 60s default timeout

### 3. Structured Error Codes
**Current State**: Text-based error messages

**Recommendation**: Return structured error codes for common scenarios:
- `SESSION_NOT_FOUND`
- `SESSION_NOT_CONNECTED`
- `COMMAND_TIMEOUT`
- `AUTH_FAILED`
- `HOST_KEY_UNKNOWN`
- `PERMISSION_DENIED`

**Benefits**:
- Agents can handle errors programmatically
- Better retry logic
- Clearer debugging

**Implementation**:
```go
type ErrorResponse struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    Session    string `json:"session,omitempty"`
    Suggestion string `json:"suggestion,omitempty"`
}
```

**Reference**: CLAUDE.md error handling section, MCP best practices

## Priority 2: High-Value Tools

### 4. File Transfer Tools
**Current State**: File operations only through shell commands

**Recommendation**: Add dedicated SCP-based tools:
- `upload_file` - Upload local file to remote session
- `download_file` - Download remote file to local system
- `upload_directory` - Upload directory recursively
- `download_directory` - Download directory recursively

**Benefits**:
- More reliable than shell redirection
- Progress tracking for large files
- Better error handling
- Works across all session types

**Reference**: AiondaDotCom/mcp-ssh SCP support

### 5. Read-Only File Operations
**Current State**: File reading only through `cat` commands

**Recommendation**: Add specialized read-only tools following official filesystem server pattern:
- `read_file` - Read file contents with encoding detection
- `read_multiple_files` - Read multiple files in one call
- `list_directory` - List directory with metadata
- `search_files` - Search files using glob patterns
- `get_file_info` - Get file metadata (size, permissions, timestamps)

**Benefits**:
- More efficient than spawning shell processes
- Structured responses
- Better handling of binary/text files
- Reduced token usage for agents

**Implementation Note**: These should be lightweight wrappers around existing execute functionality but with typed responses.

**Reference**: Official MCP filesystem server

## Priority 3: Observability & Reliability

### 6. Comprehensive Logging
**Current State**: Basic logging via logger package

**Recommendation**: Implement structured logging with:
- Request/response logging for all MCP calls
- Session lifecycle events (connect, disconnect, errors)
- Command execution audit trail
- Performance metrics (latency, throughput)

**Benefits**:
- Better debugging
- Security auditing
- Performance monitoring
- Compliance requirements

**Reference**: MCP best practices on monitoring & observability

### 7. Connection Health Checks
**Current State**: No connection monitoring

**Recommendation**:
- Periodic keepalive for SSH sessions
- Automatic reconnection on connection loss
- Connection status in session info
- Health check tool for verifying connectivity

**Benefits**:
- Prevent silent connection failures
- Better user experience
- Proactive error detection

### 8. Rate Limiting & Circuit Breakers
**Current State**: No protection against command floods

**Recommendation**:
- Rate limit commands per session
- Circuit breaker pattern for failing sessions
- Configurable limits per session type

**Benefits**:
- Prevent resource exhaustion
- Protect remote systems
- Graceful degradation

**Reference**: MCP best practices on fail-safe design

## Priority 4: Advanced Features

### 9. Background Job Management
**Current State**: Background execution not implemented

**Recommendation**: Full background job support:
- `execute` with `background: true` returns job ID
- `jobs_list` - List running background jobs
- `jobs_get` - Get job output/status
- `jobs_cancel` - Cancel running job

**Benefits**:
- Support for long-running operations
- Better async workflow support
- Resource management

**Note**: This aligns with existing thop job management

### 10. Batch Command Support
**Current State**: Single command execution only

**Recommendation**: Add `execute_batch` tool:
- Execute multiple commands in sequence
- Return combined results
- Stop on first error (configurable)

**Benefits**:
- Reduce round-trips
- Atomic operations
- More efficient for agents

**Reference**: AiondaDotCom/mcp-ssh batch support

### 11. Sudo/Privilege Elevation
**Current State**: Sudo requires manual password entry

**Recommendation**:
- Add `sudo` parameter to execute tool
- Support sudo password in session config (optional)
- Automatic sudo prompt detection/handling

**Benefits**:
- Enable privileged operations
- Better automation support

**Reference**: tufantunc/ssh-mcp sudo-exec tool

**Security Note**: Require explicit configuration, never enable by default

## Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- [ ] SSH config auto-discovery
- [ ] Command timeout handling
- [ ] Structured error codes
- [ ] Enhanced logging

### Phase 2: File Operations (Week 3)
- [ ] Read-only file tools
- [ ] File transfer tools (SCP)
- [ ] Binary file handling

### Phase 3: Reliability (Week 4)
- [ ] Connection health checks
- [ ] Rate limiting
- [ ] Circuit breakers
- [ ] Automatic reconnection

### Phase 4: Advanced (Week 5+)
- [ ] Background job management
- [ ] Batch command execution
- [ ] Sudo support
- [ ] Performance optimizations

## Design Principles to Follow

1. **Minimize Token Usage**: Design tools to return only essential data
2. **Progressive Disclosure**: Don't load all tools at once if not needed
3. **Fail-Safe Design**: Always fail gracefully with actionable errors
4. **Security First**: Never auto-accept keys, never store credentials
5. **Native Tool Integration**: Leverage SSH/SCP binaries rather than reimplementing

## Architecture Considerations

### Tool Organization
Following official MCP patterns, organize tools by capability:

**Session Tools** (current):
- connect, switch, close, status

**Execution Tools** (current + enhanced):
- execute (with timeout, background)
- execute_batch (new)
- jobs_* (new)

**File Tools** (new):
- read_file, read_multiple_files
- list_directory, search_files, get_file_info
- upload_file, download_file

**Admin Tools** (new):
- health_check
- reconnect
- clear_cache

### Resource Enhancements
Add additional resources:
- `session://config/discovered` - Auto-discovered SSH hosts
- `session://{name}/jobs` - Background jobs for session
- `session://{name}/history` - Command history
- `logs://mcp` - MCP server logs

## Testing Strategy

For each new tool/feature:
1. Unit tests for tool logic
2. Integration tests with mock SSH
3. Error condition tests
4. Performance tests for large operations
5. Security tests (injection, path traversal, etc.)

## Security Considerations

1. **Input Validation**: Validate all tool parameters
2. **Path Traversal**: Prevent access outside allowed directories
3. **Command Injection**: Sanitize command parameters
4. **Authentication**: Never expose credentials in logs/errors
5. **Least Privilege**: Run with minimum required permissions

## Performance Targets

- Tool call overhead: < 10ms
- SSH command execution: < 100ms + actual command time
- File read (1MB): < 200ms
- Connection establishment: < 2s
- Auto-discovery scan: < 500ms

## Metrics to Track

- Tool call success/error rates
- Average tool execution time (p50, p95, p99)
- SSH connection success rate
- Number of active sessions
- Background job completion rate
- File transfer throughput

## References

1. [tufantunc/ssh-mcp](https://github.com/tufantunc/ssh-mcp) - Timeout handling, sudo support
2. [AiondaDotCom/mcp-ssh](https://github.com/AiondaDotCom/mcp-ssh) - Config discovery, SCP support
3. [Official MCP Filesystem Server](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem) - File operation patterns
4. [MCP Best Practices](https://modelcontextprotocol.info/docs/best-practices/) - Architecture guidelines
5. [Anthropic Code Execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp) - Efficiency patterns

## Conclusion

The most impactful improvements are:
1. **SSH config auto-discovery** - Zero configuration UX
2. **Command timeout handling** - Reliability and safety
3. **File transfer tools** - Essential for remote operations
4. **Structured error codes** - Better agent integration

These improvements will make thop's MCP server competitive with standalone SSH MCP servers while maintaining its unique multi-session management capabilities.
