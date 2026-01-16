# Claude Code Development Guide for thop

## Project Overview

**thop** (Terminal Hopper for Agents) is a CLI tool that enables AI coding agents to seamlessly execute commands on remote systems. It's an interactive shell wrapper with two modes:

1. **Interactive Mode**: Run `thop` for a shell with `(session) $` prompt and slash commands
2. **Proxy Mode**: Run `thop --proxy` as `SHELL` for AI agents

## Core Principle

**Never Block the Agent** - All operations must return immediately. If authentication fails or requires user input, return actionable error information that can be handled programmatically.

## Architecture (Shell Wrapper - No Daemon)

```
┌─────────────────────────────────────────────────────────────────┐
│                    thop (single binary)                         │
│  ┌─────────────────────┬─────────────────────────────┐         │
│  │  Interactive Mode   │      Proxy Mode             │         │
│  │  - (local) $ prompt │  - SHELL compatible         │         │
│  │  - Slash commands   │  - Line-by-line I/O         │         │
│  │  - Human UX         │  - For AI agents            │         │
│  └─────────────────────┴─────────────────────────────┘         │
│                         │                                       │
│              ┌──────────┴──────────┐                           │
│              ▼                     ▼                           │
│  ┌─────────────────────────────────────────────────┐          │
│  │           Session Manager                        │          │
│  │  - Local shell + SSH sessions                   │          │
│  │  - State tracking (cwd, env)                    │          │
│  └─────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Languages

We are evaluating both **Go** and **Rust**. Prototypes in both languages will be built in Phase 0.

### Go Stack
- SSH: `golang.org/x/crypto/ssh`
- Config: `github.com/pelletier/go-toml`
- CLI: Standard library or `cobra`

### Rust Stack
- SSH: `russh`
- Config: `toml`
- CLI: `clap`
- Async: `tokio`

## Key Components

### Interactive Mode
- Default when running `thop` with no arguments
- Displays prompt: `(local) $` or `(prod) $`
- Parses slash commands (`/connect`, `/switch`, etc.)
- Passes regular commands to active session

### Proxy Mode (`thop --proxy`)
- For AI agent integration via `SHELL` environment variable
- Reads stdin line-by-line
- Routes to active session
- Outputs to stdout/stderr

### Session Manager
- Manages local shell and SSH sessions
- Tracks per-session state (cwd, env vars)
- Handles SSH connection lifecycle

### State File
- Location: `~/.local/share/thop/state.json`
- Shares state between thop instances
- File locking for concurrent access

## Configuration

Location: `~/.config/thop/config.toml`

```toml
[settings]
default_session = "local"
command_timeout = 300
log_level = "info"

[sessions.local]
type = "local"
shell = "/bin/bash"

[sessions.prod]
type = "ssh"
host = "prod.example.com"
user = "deploy"
```

## Slash Commands

| Command | Action |
|---------|--------|
| `/connect <session>` | Establish SSH connection |
| `/switch <session>` | Change active context |
| `/local` | Switch to local shell |
| `/status` | Show all sessions |
| `/close <session>` | Close SSH connection |
| `/help` | Show available commands |

## Project Structure

### Go (`thop-go/`)
```
thop-go/
├── cmd/
│   └── thop/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── interactive.go
│   │   ├── proxy.go
│   │   └── commands.go
│   ├── session/
│   │   ├── manager.go
│   │   ├── local.go
│   │   └── ssh.go
│   ├── config/
│   │   └── config.go
│   └── state/
│       └── state.go
├── go.mod
└── go.sum
```

### Rust (`thop-rust/`)
```
thop-rust/
├── src/
│   ├── main.rs
│   ├── cli/
│   │   ├── mod.rs
│   │   ├── interactive.rs
│   │   ├── proxy.rs
│   │   └── commands.rs
│   ├── session/
│   │   ├── mod.rs
│   │   ├── manager.rs
│   │   ├── local.rs
│   │   └── ssh.rs
│   ├── config/
│   │   └── mod.rs
│   └── state/
│       └── mod.rs
├── Cargo.toml
└── Cargo.lock
```

## Development Phases

### Phase 0: Language Evaluation
Build minimal prototypes in both Go and Rust:
- Interactive mode with prompt
- Local shell execution
- Single SSH session
- Basic slash commands
- Proxy mode

### Phase 1: Core MVP
Full implementation in chosen language:
- Complete interactive and proxy modes
- Multiple sessions
- State management
- Configuration parsing
- Error handling

### Phase 2-4: Robustness, Polish, Advanced
See TODO.md for detailed tasks.

## Error Handling

Return structured JSON errors:
```json
{
  "error": true,
  "code": "AUTH_PASSWORD_REQUIRED",
  "message": "SSH key authentication failed.",
  "session": "prod",
  "suggestion": "Use /auth prod to provide password"
}
```

Exit codes:
- 0: Success
- 1: Error
- 2: Authentication required
- 3: Host key verification required

## Performance Targets

- Context switch: < 50ms
- Command overhead: < 10ms
- Memory (idle): < 50MB
- Memory per session: < 10MB

## Security Considerations

- Never store credentials
- State file with user-only permissions
- No sensitive data in logs
- Never auto-accept host keys
- Password files require 0600 permissions

## Common Tasks

### Adding a Slash Command
1. Add to command parser in `commands.go`/`commands.rs`
2. Implement handler function
3. Update `/help` output
4. Add tests

### Adding SSH Feature
1. Update session manager
2. Handle in SSH session module
3. Add configuration option if needed
4. Test with Docker SSH container

## Debugging

- Set `THOP_LOG_LEVEL=debug` for verbose output
- Check state file: `cat ~/.local/share/thop/state.json`
- Test proxy mode: `echo "ls -la" | thop --proxy`
