# Claude Code Development Guide for thop

## Project Overview

**thop** (Terminal Hopper for Agents) is a CLI tool that enables AI coding agents to seamlessly execute commands on remote systems without managing SSH connections. It maintains persistent SSH sessions in the background and allows instant, non-blocking context switching.

## Core Principle

**Never Block the Agent** - All operations must return immediately. If authentication fails or requires user input, return actionable error information that can be handled programmatically.

## Architecture

```
┌─────────────┐     ┌─────────────────────────────────────┐
│ AI Agent    │     │            thopd (daemon)            │
└──────┬──────┘     │  ┌─────────────────────────────────┐│
       │            │  │       Session Manager           ││
       │ stdin/out  │  └──────────────┬──────────────────┘│
       ▼            │                 │                    │
┌──────────────┐    │ ┌──────┐    ┌──────┐    ┌──────┐   │
│    thop      │◄───┼─┤local │    │ prod │    │ stg  │   │
│    proxy     │    │ │shell │    │(SSH) │    │(SSH) │   │
└──────────────┘    │ └──────┘    └──────┘    └──────┘   │
       ▲            └─────────────────────────────────────┘
       │ Unix Socket
```

## Key Components

1. **Daemon (`thopd`)** - Background process at `$XDG_RUNTIME_DIR/thop.sock`
2. **CLI (`thop`)** - Thin client communicating with daemon via RPC
3. **Proxy (`thop proxy`)** - Transparent command passthrough for AI agents
4. **Session** - Represents a shell context (local or remote)

## Configuration

Location: `~/.config/thop/config.toml`

```toml
[settings]
default_session = "local"
command_timeout = 300
reconnect_attempts = 5

[sessions.local]
type = "local"
shell = "/bin/bash"

[sessions.prod]
type = "ssh"
host = "prod.example.com"
user = "deploy"
```

## Development Guidelines

### Code Style
- Use idiomatic Rust patterns
- Follow the existing project structure
- Keep functions focused and small
- Document public APIs

### Testing
- Write unit tests for all business logic
- Integration tests for SSH and daemon communication
- Use Docker containers for SSH test targets

### Error Handling
- Return structured JSON errors to stderr
- Use specific error codes: `AUTH_PASSWORD_REQUIRED`, `CONNECTION_FAILED`, etc.
- Include actionable suggestions in error messages

### Performance Targets
- Context switch: < 50ms
- Command routing overhead: < 10ms
- Memory (idle daemon): < 50MB
- Memory per session: < 10MB

## CLI Commands Reference

| Command | Description |
|---------|-------------|
| `thop start` | Start the daemon |
| `thop stop` | Stop the daemon |
| `thop status` | Show all sessions |
| `thop <session>` | Switch context |
| `thop current` | Print current context |
| `thop exec <s> <cmd>` | One-off execution |
| `thop proxy` | Enter proxy mode |
| `thop auth <s>` | Provide credentials |
| `thop trust <s>` | Trust host key |

## File Structure

```
thop/
├── src/
│   ├── main.rs           # CLI entry point
│   ├── daemon/           # Daemon process
│   │   ├── mod.rs
│   │   ├── session.rs    # Session management
│   │   └── socket.rs     # Unix socket handling
│   ├── cli/              # CLI commands
│   │   ├── mod.rs
│   │   └── commands/
│   ├── proxy/            # Proxy mode
│   ├── config/           # Configuration parsing
│   └── error/            # Error types
├── tests/
│   ├── integration/
│   └── e2e/
├── Cargo.toml
├── PRD.md
├── TODO.md
├── PROGRESS.md
└── CLAUDE.md
```

## Implementation Priorities

### P0 (Must Have)
- Daemon with Unix socket
- Local session execution
- SSH session support
- Context switching
- Proxy mode
- Non-blocking authentication errors

### P1 (Should Have)
- Multiple concurrent sessions
- Automatic reconnection
- Command timeout handling
- SSH agent forwarding

### P2 (Nice to Have)
- Jump host support
- Async command execution
- Session logs

## Common Tasks

### Adding a New CLI Command
1. Create command module in `src/cli/commands/`
2. Register in CLI argument parser
3. Implement handler that communicates with daemon
4. Add tests

### Adding a New Session Type
1. Implement `Session` trait
2. Add configuration parsing
3. Register in session factory
4. Add integration tests

### Debugging
- Check daemon logs: `~/.local/share/thop/daemon.log`
- Session logs: `~/.local/share/thop/sessions/<name>.log`
- Use `THOP_LOG_LEVEL=debug` for verbose output

## Security Considerations

- Never store credentials
- Unix socket with user-only permissions (0600)
- No sensitive data in logs
- Never auto-accept host keys
- Password files require 0600 permissions
