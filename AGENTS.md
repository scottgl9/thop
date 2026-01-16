# Agent Development Guide for thop

This document provides instructions for AI agents and automated tools contributing to thop development.

## Project Summary

**thop** is a terminal hopper that allows AI agents to execute commands on remote systems seamlessly. It provides:

- Persistent SSH sessions managed by a background daemon
- Instant context switching between local and remote shells
- Non-blocking operation (never prompts for input)
- Transparent proxy mode for AI agent integration

## Repository Structure

```
thop/
├── PRD.md          # Product requirements (source of truth)
├── TODO.md         # Task list derived from PRD
├── PROGRESS.md     # Completion tracking
├── CLAUDE.md       # Claude Code-specific instructions
├── AGENTS.md       # This file
├── src/            # Rust source code
├── tests/          # Test suites
└── Cargo.toml      # Rust dependencies
```

## Key Documents

| Document | Purpose |
|----------|---------|
| `PRD.md` | Complete requirements specification |
| `TODO.md` | Actionable task list with priorities |
| `PROGRESS.md` | Implementation status tracking |

## Development Workflow

### Before Starting Work

1. Read `PRD.md` for context on requirements
2. Check `TODO.md` for available tasks
3. Review `PROGRESS.md` for current status
4. Identify dependencies between tasks

### During Development

1. Update `PROGRESS.md` when starting a task
2. Follow the code patterns in existing files
3. Write tests alongside implementation
4. Keep commits focused and atomic

### After Completing Work

1. Mark task complete in `TODO.md`
2. Update `PROGRESS.md` with completion status
3. Run test suite to verify no regressions

## Technical Stack

- **Language**: Rust
- **IPC**: Unix domain sockets
- **SSH**: System SSH subprocess (initially)
- **Config**: TOML format
- **Logging**: Structured logging to files

## Architecture Guidelines

### Daemon Design
- Single background process per user
- Manages all SSH connections
- Communicates via Unix socket at `$XDG_RUNTIME_DIR/thop.sock`
- Must handle reconnection automatically

### CLI Design
- Thin client that sends RPC to daemon
- Returns structured JSON on errors
- Exit codes: 0=success, 1=error, 2=auth needed, 3=host key needed

### Proxy Mode
- Primary interface for AI agents
- Reads from stdin, writes to stdout/stderr
- Transparent passthrough to active session

## Error Handling Patterns

All errors must be:
1. **Non-blocking** - Return immediately with error info
2. **Structured** - JSON format with error code
3. **Actionable** - Include suggestion for resolution

Example error response:
```json
{
  "error": true,
  "code": "AUTH_PASSWORD_REQUIRED",
  "message": "SSH key authentication failed. Password required.",
  "session": "prod",
  "suggestion": "Run: thop auth prod --password"
}
```

## Priority Definitions

| Priority | Meaning |
|----------|---------|
| P0 | Core functionality, must have for MVP |
| P1 | Important features for usability |
| P2 | Nice to have, can defer |

## Testing Requirements

### Unit Tests
- All parsing logic
- State management
- Error handling

### Integration Tests
- Daemon communication
- SSH connections (use Docker)
- Context switching

### E2E Tests
- Full workflows with simulated agent
- Multi-session scenarios

## Code Quality Standards

- Idiomatic Rust
- Public APIs documented
- No `unwrap()` in production paths
- Proper error propagation with `?`
- Logging at appropriate levels

## Security Requirements

- Never store passwords
- Unix socket: 0600 permissions
- Password files: 0600 permissions required
- No credentials in logs
- Never auto-accept host keys

## Performance Targets

| Metric | Target |
|--------|--------|
| Context switch | < 50ms |
| Command overhead | < 10ms |
| Daemon memory (idle) | < 50MB |
| Session memory | < 10MB |

## Common Mistakes to Avoid

1. **Blocking on user input** - Always return with error instead
2. **Storing credentials** - Use SSH agent or per-request auth
3. **Ignoring host key verification** - Return error, let user decide
4. **Polling for connection status** - Use event-driven approach
5. **Tight coupling CLI and daemon** - Keep RPC interface clean

## Getting Help

- `PRD.md` Section 5: Functional Requirements
- `PRD.md` Section 6: Non-Functional Requirements
- `PRD.md` Section 11: Error Handling
- `PRD.md` Section 14: Implementation Phases
