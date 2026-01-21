# Agent Development Guide for thop

This document provides instructions for AI agents and automated tools contributing to thop development.

## Project Summary

**thop** is a terminal hopper that allows AI agents to execute commands on remote systems seamlessly. It provides:

- Interactive shell wrapper with `(session) $` prompt
- Slash commands for session management (`/connect`, `/switch`, etc.)
- Proxy mode (`--proxy`) for AI agent integration
- State sharing between instances via file

**Architecture**: Shell wrapper (no daemon)
**Language**: Go

## Repository Structure

```
thop/
├── TODO.md         # Task list by phase
├── PROGRESS.md     # Completion tracking
├── CLAUDE.md       # Claude Code-specific guide
├── AGENTS.md       # This file
├── cmd/            # Main entry point
├── internal/       # Internal packages
└── go.mod
```

## Key Documents

| Document | Purpose |
|----------|---------|
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

1. Check `TODO.md` for current phase tasks
2. Review `PROGRESS.md` for status

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

- **Language**: Go 1.21+
- **SSH**: `golang.org/x/crypto/ssh`
- **Config**: `github.com/pelletier/go-toml`
- **State**: JSON file with file locking

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
| **Phase 1** | Core MVP |
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

- `gofmt` for formatting
- `golint` for style
- No `panic()` in production paths
- Error wrapping with context

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

- `TODO.md`: Task list and requirements
- `README.md`: User documentation
