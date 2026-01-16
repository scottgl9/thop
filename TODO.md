# thop TODO List

Tasks derived from PRD.md (v0.2.0 - Shell Wrapper Architecture).
See PROGRESS.md for completion tracking.

---

## Phase 0: Language Evaluation

Implement minimal prototypes in both Go and Rust to compare.

### Go Prototype (`thop-go/`)

#### Project Setup
- [ ] Initialize Go module (`go mod init`)
- [ ] Add dependencies: `golang.org/x/crypto/ssh`, `github.com/pelletier/go-toml`
- [ ] Create project structure (`cmd/`, `internal/`)

#### Interactive Mode
- [ ] Implement main loop with `(session) $` prompt
- [ ] Parse user input for slash commands vs regular commands
- [ ] Display output from command execution

#### Local Shell
- [ ] Execute commands in local shell subprocess
- [ ] Capture stdout/stderr
- [ ] Return exit codes

#### SSH Session
- [ ] Establish SSH connection using `golang.org/x/crypto/ssh`
- [ ] Execute commands over SSH channel
- [ ] Handle authentication (key-based)
- [ ] Return `AUTH_PASSWORD_REQUIRED` error when needed

#### Slash Commands
- [ ] `/connect <session>` - Establish SSH connection
- [ ] `/switch <session>` - Change active context
- [ ] `/local` - Switch to local shell
- [ ] `/status` - Show all sessions
- [ ] `/help` - Show available commands

#### Proxy Mode
- [ ] `--proxy` flag to run in proxy mode
- [ ] Read commands from stdin line-by-line
- [ ] Route to active session
- [ ] Output to stdout/stderr

#### Configuration
- [ ] Parse `~/.config/thop/config.toml`
- [ ] Load session definitions

---

### Rust Prototype (`thop-rust/`)

#### Project Setup
- [ ] Initialize Cargo project (`cargo init`)
- [ ] Add dependencies: `russh`, `toml`, `clap`, `tokio`
- [ ] Create project structure (`src/`)

#### Interactive Mode
- [ ] Implement main loop with `(session) $` prompt
- [ ] Parse user input for slash commands vs regular commands
- [ ] Display output from command execution

#### Local Shell
- [ ] Execute commands in local shell subprocess
- [ ] Capture stdout/stderr
- [ ] Return exit codes

#### SSH Session
- [ ] Establish SSH connection using `russh`
- [ ] Execute commands over SSH channel
- [ ] Handle authentication (key-based)
- [ ] Return `AUTH_PASSWORD_REQUIRED` error when needed

#### Slash Commands
- [ ] `/connect <session>` - Establish SSH connection
- [ ] `/switch <session>` - Change active context
- [ ] `/local` - Switch to local shell
- [ ] `/status` - Show all sessions
- [ ] `/help` - Show available commands

#### Proxy Mode
- [ ] `--proxy` flag to run in proxy mode
- [ ] Read commands from stdin line-by-line
- [ ] Route to active session
- [ ] Output to stdout/stderr

#### Configuration
- [ ] Parse `~/.config/thop/config.toml`
- [ ] Load session definitions

---

### Evaluation Criteria
- [ ] Compare code complexity and lines of code
- [ ] Measure binary size
- [ ] Measure startup time
- [ ] Evaluate SSH library ergonomics
- [ ] Document developer experience
- [ ] Make language selection decision

---

## Phase 1: Core MVP

After language selection, implement full MVP in chosen language.

### Interactive Mode
- [ ] Full interactive shell with `(session) $` prompt
- [ ] Readline support (history, line editing)
- [ ] Proper terminal handling
- [ ] Graceful exit on Ctrl+D

### Local Session
- [ ] Local shell session management
- [ ] Track current working directory
- [ ] Track environment variables
- [ ] Persist state across commands

### SSH Session
- [ ] SSH connection establishment
- [ ] Integrate with `~/.ssh/config` host aliases
- [ ] Support SSH key authentication from `~/.ssh/`
- [ ] Respect `IdentityFile` settings
- [ ] Use ssh-agent if available
- [ ] Non-blocking authentication (return error, don't prompt)

### Slash Commands
- [ ] `/connect <session>` - Full implementation
- [ ] `/switch <session>` - Full implementation
- [ ] `/local` - Shortcut for `/switch local`
- [ ] `/status` - Show all sessions with state
- [ ] `/close <session>` - Close SSH connection
- [ ] `/help` - Show all commands with descriptions

### Proxy Mode
- [ ] `thop --proxy` for AI agent integration
- [ ] SHELL-compatible execution
- [ ] Line-buffered stdin reading
- [ ] stdout/stderr passthrough
- [ ] Exit code preservation

### State Management
- [ ] State file at `~/.local/share/thop/state.json`
- [ ] Active session tracking
- [ ] Per-session state (cwd, env)
- [ ] File locking for concurrent access

### Configuration
- [ ] Parse `~/.config/thop/config.toml`
- [ ] Global settings (timeout, log level)
- [ ] Session definitions (local, SSH)
- [ ] Environment variable overrides

### Error Handling
- [ ] Structured JSON error format
- [ ] Error codes: `CONNECTION_FAILED`, `AUTH_*`, `SESSION_*`
- [ ] Exit codes: 0=success, 1=error, 2=auth, 3=host key
- [ ] Actionable error suggestions

---

## Phase 2: Robustness

### Multiple Sessions
- [ ] Support multiple concurrent SSH sessions
- [ ] Session isolation (independent state per session)
- [ ] Session listing with connection status

### Reconnection
- [ ] Automatic reconnection on connection failure
- [ ] Exponential backoff for retries
- [ ] Configurable max reconnection attempts
- [ ] State recovery after reconnect

### State Persistence
- [ ] Persist cwd/env across commands
- [ ] Replay environment on reconnect
- [ ] State survives thop restart

### Command Handling
- [ ] Configurable command timeout (default: 300s)
- [ ] Kill and report on timeout
- [ ] Signal forwarding (Ctrl+C to active session)

---

## Phase 3: Polish

### SSH Integration
- [ ] Full `~/.ssh/config` parsing
- [ ] SSH agent forwarding support
- [ ] Jump host / bastion support
- [ ] Startup commands per session

### Authentication
- [ ] `/auth <session>` - Provide password interactively
- [ ] Password from environment variable
- [ ] Password from file (0600 perms required)
- [ ] `/trust <session>` - Trust host key
- [ ] Display fingerprint before trusting

### Logging
- [ ] Log file at `~/.local/share/thop/thop.log`
- [ ] Configurable log levels
- [ ] No sensitive data in logs

### CLI Polish
- [ ] `--status` flag to show status and exit
- [ ] `--json` flag for machine-readable output
- [ ] `-v/--verbose` and `-q/--quiet` flags
- [ ] Shell completions for bash
- [ ] Shell completions for zsh
- [ ] Shell completions for fish

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

### Unit Tests
- [ ] Configuration parsing tests
- [ ] Session state management tests
- [ ] Slash command parsing tests
- [ ] Error handling tests

### Integration Tests
- [ ] Local shell execution tests
- [ ] SSH connection tests (Docker)
- [ ] Context switching tests
- [ ] State file tests

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

- [ ] README.md with quick start guide
- [ ] Installation instructions
- [ ] Configuration reference
- [ ] Integration guide for Claude Code
- [ ] Troubleshooting guide

---

## Priority Legend

- **Phase 0**: Language evaluation - Compare Go and Rust
- **Phase 1**: Core MVP - Minimum viable product
- **Phase 2**: Robustness - Production reliability
- **Phase 3**: Polish - User experience
- **Phase 4**: Advanced - Extended capabilities
