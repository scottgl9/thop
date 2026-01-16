# thop TODO List

Tasks derived from PRD.md. See PROGRESS.md for completion tracking.

## Phase 1: Core MVP

### Daemon Infrastructure
- [ ] Create daemon process structure with graceful startup/shutdown
- [ ] Implement Unix socket server at `$XDG_RUNTIME_DIR/thop.sock`
- [ ] Design and implement RPC protocol for CLI-daemon communication
- [ ] Add daemon auto-restart on crash
- [ ] Implement socket permission handling (user-only access)

### Local Session
- [ ] Implement local shell session management
- [ ] Handle command execution with stdin/stdout/stderr
- [ ] Preserve exit codes from executed commands
- [ ] Track current working directory across commands
- [ ] Track environment variables across commands

### SSH Session (Single)
- [ ] Implement SSH connection using system `ssh` subprocess
- [ ] Integrate with `~/.ssh/config` host aliases (FR1.7)
- [ ] Support SSH key authentication from `~/.ssh/` (FR1.8, FR2.1)
- [ ] Respect `IdentityFile` settings in SSH config (FR2.2)
- [ ] Use running ssh-agent if available (FR2.3)
- [ ] Return `AUTH_PASSWORD_REQUIRED` error when password needed (FR2.5)
- [ ] Return `HOST_KEY_VERIFICATION_FAILED` error for unknown hosts (FR2.11)
- [ ] Never block or prompt for password input (FR2.4)

### CLI Commands (Basic)
- [ ] `thop start` - Start the daemon (FR5.1)
- [ ] `thop stop` - Stop the daemon (FR5.2)
- [ ] `thop status` - Show all sessions and state (FR5.3)
- [ ] `thop <session>` - Switch active context (FR5.4, FR3.1)
- [ ] `thop local` - Return to local shell (FR3.2)
- [ ] `thop current` - Print current context name (FR5.5, FR3.6)

### Proxy Mode
- [ ] `thop proxy` - Enter proxy mode reading from stdin (FR6.1)
- [ ] Pass commands to active session transparently (FR6.2)
- [ ] Output session stdout/stderr to own stdout/stderr (FR6.3)
- [ ] Maintain persistent connection to daemon

### Configuration
- [ ] Parse `~/.config/thop/config.toml` configuration file (FR1.2)
- [ ] Support global settings (timeout, log level, socket path)
- [ ] Support local session configuration
- [ ] Support SSH session configuration
- [ ] Environment variable overrides (THOP_CONFIG, THOP_SOCKET, etc.)

### Error Handling
- [ ] Define structured JSON error format
- [ ] Implement error codes (CONNECTION_FAILED, AUTH_*, SESSION_*, etc.)
- [ ] Exit code mapping (0=success, 1=error, 2=auth, 3=host key)
- [ ] Include actionable suggestions in error messages

---

## Phase 2: Robustness

### Multiple Sessions
- [ ] Support multiple concurrent SSH sessions (G5)
- [ ] Session isolation (independent state per session)
- [ ] Session listing in `thop status`

### Reconnection
- [ ] Implement automatic reconnection on connection failure (FR1.4)
- [ ] Exponential backoff for reconnection attempts
- [ ] Configurable max reconnection attempts (FR1.5, default: 5)
- [ ] Session recovery preserving cwd and environment

### State Persistence
- [ ] Persist shell state (cwd, env vars) across commands (FR4.4, G6)
- [ ] Replay environment on reconnection
- [ ] Context persistence across thop restarts (FR3.5)

### Command Handling
- [ ] Configurable command timeout (FR4.6, default: 300s)
- [ ] Kill and report on timeout
- [ ] `thop exec <session> "<command>"` - One-off execution (FR5.6)

### Additional CLI Commands
- [ ] `thop connect <session>` - Establish connection (FR5.7)
- [ ] `thop close <session>` - Close specific session (FR5.8)

---

## Phase 3: Polish

### SSH Integration
- [ ] Full SSH config file parsing and integration
- [ ] SSH agent forwarding support (FR1.9)
- [ ] Jump host / bastion server support (FR1.10)
- [ ] Startup commands per session

### Authentication Commands
- [ ] `thop auth <session> --password` - Read password from stdin (FR2.6)
- [ ] `thop auth <session> --password-env VAR` - From env var (FR2.7)
- [ ] `thop auth <session> --password-file PATH` - From file (FR2.8)
- [ ] Validate file permissions (0600 required)
- [ ] `thop auth <session> --clear` - Clear cached credentials (FR2.10)
- [ ] Credential timeout (configurable, default: 1 hour) (FR2.9)
- [ ] `thop trust <session>` - Add host key to known_hosts (FR5.10, FR2.12)
- [ ] Display host key fingerprint before trusting

### Logging
- [ ] Daemon logging to `~/.local/share/thop/daemon.log`
- [ ] Session logging to `~/.local/share/thop/sessions/<name>.log`
- [ ] Configurable log levels
- [ ] `thop logs [session]` - View logs (FR5.11)
- [ ] No sensitive data in logs (NFR3.3)

### CLI Polish
- [ ] `thop config` - Edit/view configuration (FR5.12)
- [ ] Shell completions for bash
- [ ] Shell completions for zsh
- [ ] Shell completions for fish
- [ ] `--json` flag for machine-readable output
- [ ] `-v/--verbose` and `-q/--quiet` flags

### Proxy Enhancements
- [ ] Handle special `#thop` commands in proxy mode (FR6.4)
- [ ] Support use as SHELL environment variable (FR6.5)

---

## Phase 4: Advanced Features

### Interactive & Async
- [ ] PTY support for interactive commands (FR4.5)
- [ ] Async command execution with status polling (FR4.7)
- [ ] Command interruption / Ctrl+C forwarding (FR4.8)
- [ ] Automatic transition to async for long commands

### Configuration Enhancements
- [ ] Ad-hoc session creation via CLI (FR1.3)
- [ ] Session timeout for idle connections (FR1.6)

### Future Considerations
- [ ] MCP server wrapper
- [ ] Metrics and observability
- [ ] Session sharing (optional)

---

## Testing Tasks

### Unit Tests
- [ ] Configuration parsing tests
- [ ] Session state management tests
- [ ] Command routing logic tests
- [ ] Reconnection backoff calculation tests
- [ ] Error handling tests

### Integration Tests
- [ ] Local shell execution tests
- [ ] SSH connection tests (with Docker)
- [ ] Context switching tests
- [ ] Command timeout handling tests
- [ ] Reconnection tests (network simulation)

### E2E Tests
- [ ] Full workflow with mock AI agent
- [ ] Multi-session scenarios
- [ ] Long-running command handling
- [ ] Stress testing with rapid context switches

### Test Infrastructure
- [ ] Docker containers for SSH test targets
- [ ] Network simulation for disconnect testing
- [ ] CI pipeline configuration

---

## Documentation Tasks

- [ ] README.md with quick start guide
- [ ] Installation instructions
- [ ] Configuration reference
- [ ] Integration guide for Claude Code
- [ ] Integration guide for other AI agents
- [ ] Troubleshooting guide

---

## Priority Legend

Tasks are organized by implementation phase from PRD Section 14:
- **Phase 1**: Core MVP - Essential for first working version
- **Phase 2**: Robustness - Production-ready reliability
- **Phase 3**: Polish - User experience improvements
- **Phase 4**: Advanced - Extended capabilities
