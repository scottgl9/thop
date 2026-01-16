# Agent Development Guide for thop

This document provides instructions for AI agents and automated tools contributing to thop development.

## Project Summary

**thop** is a terminal hopper that allows AI agents to execute commands on remote systems seamlessly. It provides:

- Interactive shell wrapper with `(session) $` prompt
- Slash commands for session management (`/connect`, `/switch`, etc.)
- Proxy mode (`--proxy`) for AI agent integration
- State sharing between instances via file

**Architecture**: Shell wrapper (no daemon)
**Languages**: Evaluating Go and Rust in Phase 0

## Repository Structure

```
thop/
├── PRD.md          # Product requirements (v0.2.0)
├── RESEARCH.md     # Architecture research findings
├── TODO.md         # Task list by phase
├── PROGRESS.md     # Completion tracking
├── CLAUDE.md       # Claude Code-specific guide
├── AGENTS.md       # This file
├── thop-go/        # Go prototype (Phase 0)
│   ├── cmd/
│   ├── internal/
│   └── go.mod
└── thop-rust/      # Rust prototype (Phase 0)
    ├── src/
    └── Cargo.toml
```

## Key Documents

| Document | Purpose |
|----------|---------|
| `PRD.md` | Complete requirements (v0.2.0 - shell wrapper) |
| `RESEARCH.md` | Architecture decisions and language evaluation |
| `TODO.md` | Actionable task list with phases |
| `PROGRESS.md` | Implementation status tracking |

## Architecture Overview

```
┌─────────────────────────────────────┐
│  thop (single binary, two modes)    │
├─────────────────────────────────────┤
│  Interactive Mode    Proxy Mode     │
│  - Shows prompt      - SHELL compat │
│  - Slash commands    - Line-by-line │
│  - Human UX          - For agents   │
└────────────┬────────────────────────┘
             │
             ▼
    ┌─────────────────────────┐
    │ Session Manager         │
    │ - Local shell           │
    │ - SSH sessions          │
    │ - State (cwd, env)      │
    └─────────────────────────┘
             │
     ┌───────┴───────┐
     ▼               ▼
  Local           SSH Sessions
  Shell           (persistent)
```

## Development Workflow

### Before Starting Work

1. Read `PRD.md` for requirements context
2. Check `TODO.md` for current phase tasks
3. Review `PROGRESS.md` for status
4. Identify which prototype (Go or Rust) to work on

### During Development

1. Update `PROGRESS.md` when starting a task
2. Follow existing code patterns
3. Write tests alongside implementation
4. Keep commits focused

### After Completing Work

1. Mark task complete in `TODO.md`
2. Update `PROGRESS.md` with status
3. Run tests to verify

## Technical Stack

### Go Prototype
- **Language**: Go 1.21+
- **SSH**: `golang.org/x/crypto/ssh`
- **Config**: `github.com/pelletier/go-toml`
- **State**: JSON file with file locking

### Rust Prototype
- **Language**: Rust 1.70+
- **SSH**: `russh` crate
- **Config**: `toml` crate
- **Async**: `tokio`
- **CLI**: `clap`

## Slash Commands

| Command | Action |
|---------|--------|
| `/connect <session>` | Establish SSH connection |
| `/switch <session>` | Change active context |
| `/local` | Switch to local shell |
| `/status` | Show all sessions |
| `/close <session>` | Close SSH connection |
| `/help` | Show commands |

## Error Handling

All errors must be:
1. **Non-blocking** - Return immediately
2. **Structured** - JSON format with error code
3. **Actionable** - Include suggestion

```json
{
  "error": true,
  "code": "AUTH_PASSWORD_REQUIRED",
  "message": "SSH key authentication failed.",
  "session": "prod",
  "suggestion": "Use /auth prod to provide password"
}
```

## Development Phases

| Phase | Focus |
|-------|-------|
| **Phase 0** | Build prototypes in Go and Rust, evaluate |
| **Phase 1** | Core MVP in chosen language |
| **Phase 2** | Robustness (reconnection, timeouts) |
| **Phase 3** | Polish (SSH config, completions) |
| **Phase 4** | Advanced (PTY, async) |

## Testing Requirements

### Unit Tests
- Configuration parsing
- Slash command parsing
- Session state management
- Error handling

### Integration Tests
- Local shell execution
- SSH connections (Docker)
- State file operations

### E2E Tests
- Full workflow
- Proxy mode with AI agent

## Code Quality Standards

### Go
- `gofmt` for formatting
- `golint` for style
- No `panic()` in production paths
- Error wrapping with context

### Rust
- `rustfmt` for formatting
- `clippy` for lints
- No `unwrap()` in production paths
- Proper error propagation with `?`

## Security Requirements

- Never store passwords
- State file: 0600 permissions
- No credentials in logs
- Never auto-accept host keys

## Performance Targets

| Metric | Target |
|--------|--------|
| Context switch | < 50ms |
| Command overhead | < 10ms |
| Memory (idle) | < 50MB |
| Per-session memory | < 10MB |

## Common Mistakes to Avoid

1. **Blocking on input** - Always return error instead
2. **Storing credentials** - Use SSH agent or per-request auth
3. **Auto-accepting host keys** - Security risk
4. **Ignoring file locks** - Causes state corruption
5. **Mixing sync/async** - Performance issues

## Getting Help

- `PRD.md` Section 5: Functional Requirements
- `PRD.md` Section 7: Technical Architecture
- `PRD.md` Section 11: Error Handling
- `RESEARCH.md`: Architecture decisions
