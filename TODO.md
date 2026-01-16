# thop TODO List

Tasks derived from PRD.md (v0.2.0 - Shell Wrapper Architecture).
See PROGRESS.md for completion tracking.

---

## Phase 0: Language Evaluation ✅ COMPLETE

Both Go and Rust prototypes implemented and tested.

### Go Prototype (`thop-go/`) ✅
- [x] Project setup with go.mod
- [x] Interactive mode with prompt
- [x] Local shell execution
- [x] SSH session support
- [x] Slash commands (/connect, /switch, /local, /status, /help)
- [x] Proxy mode (--proxy)
- [x] TOML configuration
- [x] Unit tests (105 tests passing)

### Rust Prototype (`thop-rust/`) ✅
- [x] Project setup with Cargo
- [x] Interactive mode with prompt
- [x] Local shell execution
- [x] SSH session support
- [x] Slash commands
- [x] Proxy mode
- [x] TOML configuration
- [x] Unit tests (32 tests passing)

### Evaluation Results ✅
- [x] Binary size: Go 4.8MB, Rust 1.4MB
- [x] Both have similar code complexity
- [x] Both have fast startup (<100ms)
- [x] **Decision: Continue with Go** for faster development

---

## Phase 1: Core MVP ✅ MOSTLY COMPLETE

After language selection, implement full MVP in chosen language.

### Interactive Mode ✅
- [x] Full interactive shell with `(session) $` prompt
- [x] Readline support (history, line editing)
- [x] Proper terminal handling
- [x] Graceful exit on Ctrl+D

### Local Session ✅
- [x] Local shell session management
- [x] Track current working directory
- [x] Track environment variables
- [x] Persist state across commands

### SSH Session ✅
- [x] SSH connection establishment
- [x] Integrate with `~/.ssh/config` host aliases
- [x] Support SSH key authentication from `~/.ssh/`
- [x] Respect `IdentityFile` settings
- [x] Use ssh-agent if available
- [x] Non-blocking authentication (return error, don't prompt)

### Slash Commands ✅
- [x] `/connect <session>` - Full implementation
- [x] `/switch <session>` - Full implementation
- [x] `/local` - Shortcut for `/switch local`
- [x] `/status` - Show all sessions with state
- [x] `/close <session>` - Close SSH connection
- [x] `/help` - Show all commands with descriptions

### Proxy Mode ✅
- [x] `thop --proxy` for AI agent integration
- [x] SHELL-compatible execution (-c flag)
- [x] Line-buffered stdin reading
- [x] stdout/stderr passthrough
- [x] Exit code preservation

### State Management ✅
- [x] State file at `~/.local/share/thop/state.json`
- [x] Active session tracking
- [x] Per-session state (cwd, env)
- [x] File locking for concurrent access

### Configuration ✅
- [x] Parse `~/.config/thop/config.toml`
- [x] Global settings (timeout, log level)
- [x] Session definitions (local, SSH)
- [x] Environment variable overrides

### Error Handling ✅
- [x] Structured JSON error format
- [x] Error codes: `CONNECTION_FAILED`, `AUTH_*`, `SESSION_*`
- [x] Exit codes: 0=success, 1=error, 2=auth, 3=host key
- [x] Actionable error suggestions

---

## Phase 2: Robustness

### Multiple Sessions
- [x] Support multiple concurrent SSH sessions
- [x] Session isolation (independent state per session)
- [x] Session listing with connection status

### Reconnection ✅
- [x] Automatic reconnection on connection failure
- [x] Exponential backoff for retries
- [x] Configurable max reconnection attempts
- [x] State recovery after reconnect

### State Persistence
- [x] Persist cwd/env across commands
- [ ] Replay environment on reconnect
- [x] State survives thop restart

### Command Handling ✅
- [x] Configurable command timeout (default: 300s)
- [x] Kill and report on timeout
- [x] Signal forwarding (Ctrl+C to active session)

---

## Phase 3: Polish

### SSH Integration
- [x] Full `~/.ssh/config` parsing
- [ ] SSH agent forwarding support
- [ ] Jump host / bastion support
- [ ] Startup commands per session

### Authentication
- [ ] `/auth <session>` - Provide password interactively
- [ ] Password from environment variable
- [ ] Password from file (0600 perms required)
- [ ] `/trust <session>` - Trust host key
- [ ] Display fingerprint before trusting

### Logging ✅
- [x] Log file at `~/.local/share/thop/thop.log`
- [x] Configurable log levels
- [x] No sensitive data in logs

### CLI Polish ✅
- [x] `--status` flag to show status and exit
- [x] `--json` flag for machine-readable output
- [x] `-v/--verbose` and `-q/--quiet` flags
- [x] Shell completions for bash
- [x] Shell completions for zsh
- [x] Shell completions for fish

---

## Phase 4: Advanced Features

### Interactive Improvements
- [ ] PTY support for interactive commands (vim, top)
- [ ] Window resize handling (SIGWINCH)
- [ ] Command history per session

### Async Features
- [ ] Async command execution
- [ ] Background command with status polling

### Future
- [ ] MCP server wrapper
- [ ] Metrics and observability

---

## Testing Tasks

### Unit Tests ✅
- [x] Configuration parsing tests
- [x] Session state management tests
- [x] Slash command parsing tests
- [x] Error handling tests

### Integration Tests
- [x] Local shell execution tests
- [ ] SSH connection tests (Docker)
- [x] Context switching tests
- [x] State file tests

### E2E Tests
- [ ] Full workflow with mock AI agent
- [ ] Multi-session scenarios
- [ ] Proxy mode with Claude Code

### Test Infrastructure
- [ ] Docker containers for SSH test targets
- [ ] CI pipeline configuration
- [ ] Test coverage reporting

---

## Documentation Tasks

- [x] README.md with quick start guide
- [x] Installation instructions
- [x] Configuration reference
- [x] Integration guide for Claude Code
- [x] Troubleshooting guide

---

## Priority Legend

- **Phase 0**: Language evaluation - Compare Go and Rust ✅
- **Phase 1**: Core MVP - Minimum viable product ✅
- **Phase 2**: Robustness - Production reliability
- **Phase 3**: Polish - User experience
- **Phase 4**: Advanced - Extended capabilities
