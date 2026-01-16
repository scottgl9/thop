# Product Requirements Document: thop

## Terminal Hopper for Agents

> *Seamlessly switch between local and remote terminals without interrupting agent flow*

**Version:** 0.1.0  
**Status:** Draft  
**Last Updated:** January 16, 2026

---

## 1. Executive Summary

**thop** is a command-line tool that enables AI coding agents (such as Claude Code) to seamlessly execute commands on remote systems without the complexity of managing SSH connections. It maintains persistent SSH sessions in the background and allows instant, non-blocking switching between local and remote terminal contexts with a single command.

### Core Principle: Never Block the Agent

Traditional SSH workflows block on password prompts, host key confirmations, and connection timeouts. This breaks AI agent flow. thop is designed to **never block**—if authentication fails or requires user input, it returns immediately with actionable error information that the agent can handle programmatically.

### Problem Statement

AI coding agents like Claude Code are powerful but fundamentally local—they execute commands in the terminal where they're running. When developers need the agent to work on remote servers, they face friction:

1. **Connection overhead**: Each SSH command requires connection setup
2. **Context loss**: SSH sessions don't persist state between commands
3. **Blocking operations**: SSH commands block the terminal until complete
4. **Manual intervention**: Developers must explicitly manage remote vs. local execution
5. **Session fragility**: Disconnections require manual reconnection and context restoration

### Solution

thop acts as a transparent proxy layer between the AI agent and the underlying shell(s). The agent executes commands normally, and thop routes them to the appropriate destination (local or remote) based on the current context. Sessions persist in the background, surviving disconnections and allowing instant context switches.

---

## 2. Goals and Non-Goals

### Goals

- **G1**: Allow AI agents to execute commands on remote systems with zero connection overhead after initial setup
- **G2**: Provide instant, non-blocking switching between local and multiple remote contexts
- **G3**: Maintain persistent SSH sessions that survive network interruptions
- **G4**: Be completely transparent to the AI agent—no special command syntax required
- **G5**: Support multiple concurrent remote sessions (e.g., prod, staging, dev)
- **G6**: Preserve shell state (cwd, environment variables) across command invocations
- **G7**: Work with any AI agent or automation tool that uses stdin/stdout

### Non-Goals

- **NG1**: Replacing SSH—thop uses SSH under the hood
- **NG2**: Providing a GUI or TUI interface
- **NG3**: Managing SSH keys or credentials (uses existing SSH config)
- **NG4**: File transfer (use scp/rsync separately)
- **NG5**: Being an MCP server (though could be wrapped as one later)
- **NG6**: Session sharing between users

---

## 3. User Personas

### Primary: AI-Assisted Developer

- Uses Claude Code, Cursor, or similar AI coding tools daily
- Has remote development servers (cloud VMs, home lab, work servers)
- Wants AI agent to seamlessly work across local and remote environments
- Comfortable with command line but wants reduced friction

### Secondary: DevOps Engineer

- Manages multiple servers and environments
- Uses AI agents for automation and scripting
- Needs to switch between environments frequently
- Values auditability and session persistence

### Tertiary: Automation Pipeline

- CI/CD systems that leverage AI agents
- Headless operation without human intervention
- Requires reliable, scriptable interface

---

## 4. User Stories

### Core Workflow

**US1**: As a developer, I want to start thop and have it maintain connections to my configured servers so that I can switch contexts instantly.

**US2**: As a developer, I want to type `thop prod` and have all subsequent commands execute on my production server transparently.

**US3**: As a developer, I want to type `thop local` to return to executing commands locally.

**US4**: As a developer, I want my SSH sessions to persist even if my network briefly disconnects so that I don't lose my working state.

**US5**: As a developer, I want the AI agent to be completely unaware that commands are running remotely—it should just see normal stdout/stderr.

### Session Management

**US6**: As a developer, I want to see all active sessions with `thop status` so I know what's connected.

**US7**: As a developer, I want to define named sessions in a config file so I don't have to remember connection details.

**US8**: As a developer, I want sessions to automatically reconnect if the connection drops.

**US9**: As a developer, I want to explicitly close a session with `thop close prod` when I'm done.

### Advanced Usage

**US10**: As a developer, I want to run a one-off command on a remote server without switching context: `thop exec prod "docker ps"`.

**US11**: As a developer, I want to see command history per session for debugging.

**US12**: As a developer, I want environment variables set in a session to persist across commands.

**US13**: As a developer, I want the current working directory to persist across commands within a session.

---

## 5. Functional Requirements

### 5.1 Session Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR1.1 | Daemon process maintains persistent SSH connections | P0 |
| FR1.2 | Sessions defined in `~/.config/thop/config.toml` | P0 |
| FR1.3 | Sessions can also be created ad-hoc via CLI | P1 |
| FR1.4 | Automatic reconnection on connection failure (with backoff) | P0 |
| FR1.5 | Maximum reconnection attempts configurable (default: 5) | P1 |
| FR1.6 | Session timeout for idle connections configurable | P2 |
| FR1.7 | Support for SSH config file (~/.ssh/config) host aliases | P0 |
| FR1.8 | Support for SSH key authentication (~/.ssh/id_*, ~/.ssh/config IdentityFile) | P0 |
| FR1.9 | Support for SSH agent forwarding | P1 |
| FR1.10 | Support for jump hosts / bastion servers | P2 |

### 5.2 Authentication (Non-Blocking)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR2.1 | Automatically discover and use SSH keys from ~/.ssh/ | P0 |
| FR2.2 | Respect IdentityFile settings in ~/.ssh/config | P0 |
| FR2.3 | Use running ssh-agent if available | P0 |
| FR2.4 | **NEVER prompt or block** for password input | P0 |
| FR2.5 | If password required, return structured error with `AUTH_PASSWORD_REQUIRED` | P0 |
| FR2.6 | `thop auth <session> --password` reads password from stdin (single line, no echo) | P0 |
| FR2.7 | `thop auth <session> --password-env VAR` reads password from environment variable | P0 |
| FR2.8 | `thop auth <session> --password-file PATH` reads password from file (0600 perms required) | P1 |
| FR2.9 | Cached credentials expire after configurable timeout (default: 1 hour) | P1 |
| FR2.10 | `thop auth <session> --clear` removes cached credentials | P1 |
| FR2.11 | If host key verification fails, return `HOST_KEY_VERIFICATION_FAILED` (never auto-accept) | P0 |
| FR2.12 | `thop trust <session>` adds host key to known_hosts after displaying fingerprint | P1 |

### 5.3 Context Switching

| ID | Requirement | Priority |
|----|-------------|----------|
| FR3.1 | `thop <session>` changes active context | P0 |
| FR3.2 | `thop local` returns to local shell | P0 |
| FR3.3 | Context switch is instant (<50ms) | P0 |
| FR3.4 | Context switch is non-blocking | P0 |
| FR3.5 | Current context persists across thop restarts | P1 |
| FR3.6 | Context indicator available via `thop current` | P0 |

### 5.4 Command Execution

| ID | Requirement | Priority |
|----|-------------|----------|
| FR4.1 | Commands executed via stdin are routed to active session | P0 |
| FR4.2 | stdout/stderr from session returned to caller | P0 |
| FR4.3 | Exit codes preserved and returned accurately | P0 |
| FR4.4 | Shell state (cwd, env vars) persists within session | P0 |
| FR4.5 | Support for interactive commands (with PTY) | P1 |
| FR4.6 | Command timeout configurable (default: 300s) | P1 |
| FR4.7 | Async command execution with status polling | P2 |
| FR4.8 | Command interruption (Ctrl+C forwarding) | P1 |

### 5.5 CLI Interface

| ID | Requirement | Priority |
|----|-------------|----------|
| FR5.1 | `thop start` - Start the daemon | P0 |
| FR5.2 | `thop stop` - Stop the daemon | P0 |
| FR5.3 | `thop status` - Show all sessions and their state | P0 |
| FR5.4 | `thop <session>` - Change active context | P0 |
| FR5.5 | `thop current` - Print current context name | P0 |
| FR5.6 | `thop exec <session> "<command>"` - One-off execution | P1 |
| FR5.7 | `thop connect <session>` - Establish connection to session | P1 |
| FR5.8 | `thop close <session>` - Close specific session | P1 |
| FR5.9 | `thop auth <session> [OPTIONS]` - Provide credentials for session | P0 |
| FR5.10 | `thop trust <session>` - Trust and add host key to known_hosts | P1 |
| FR5.11 | `thop logs [session]` - View session logs | P2 |
| FR5.12 | `thop config` - Edit/view configuration | P2 |

### 5.6 Proxy Mode (Primary Interface for AI Agents)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR6.1 | `thop proxy` - Enter proxy mode, reading commands from stdin | P0 |
| FR6.2 | Proxy mode passes commands to active session transparently | P0 |
| FR6.3 | Proxy mode outputs session stdout/stderr to own stdout/stderr | P0 |
| FR6.4 | Proxy mode handles special commands (starting with `#thop`) | P1 |
| FR6.5 | Proxy mode can be used as SHELL environment variable | P1 |

---

## 6. Non-Functional Requirements

### 6.1 Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR1.1 | Context switch latency | < 50ms |
| NFR1.2 | Command routing overhead | < 10ms |
| NFR1.3 | Memory usage (daemon, idle) | < 50MB |
| NFR1.4 | Memory per active session | < 10MB |
| NFR1.5 | CPU usage (idle) | < 1% |

### 6.2 Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR2.1 | Daemon uptime | 99.9% (crashes auto-restart) |
| NFR2.2 | Session recovery after network interruption | < 30s |
| NFR2.3 | No command loss during brief disconnections | Buffered for 60s |
| NFR2.4 | Graceful degradation if daemon unavailable | Fall back to local |

### 6.3 Security

| ID | Requirement | Priority |
|----|-------------|----------|
| NFR3.1 | No credential storage—uses SSH agent/keys | P0 |
| NFR3.2 | Unix socket for daemon communication (user-only perms) | P0 |
| NFR3.3 | No sensitive data in logs | P0 |
| NFR3.4 | Session isolation between users | P0 |

### 6.4 Compatibility

| ID | Requirement | Priority |
|----|-------------|----------|
| NFR4.1 | Linux (Ubuntu 20.04+, Debian 11+, Fedora 36+) | P0 |
| NFR4.2 | macOS (12.0+) | P0 |
| NFR4.3 | WSL2 (Windows 11) | P1 |
| NFR4.4 | Works with bash, zsh, fish shells | P0 |
| NFR4.5 | Works with Claude Code, Cursor, Aider | P0 |

---

## 7. Technical Architecture

### 7.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        User's Machine                           │
│                                                                 │
│  ┌─────────────┐      ┌─────────────────────────────────────┐  │
│  │ Claude Code │      │            thopd (daemon)            │  │
│  │   / AI      │      │  ┌─────────────────────────────────┐│  │
│  │   Agent     │      │  │       Session Manager           ││  │
│  └──────┬──────┘      │  └──────────────┬──────────────────┘│  │
│         │             │                 │                    │  │
│         │ stdin/out   │    ┌────────────┼────────────┐      │  │
│         ▼             │    ▼            ▼            ▼      │  │
│  ┌──────────────┐     │ ┌──────┐    ┌──────┐    ┌──────┐   │  │
│  │    thop      │◄────┼─┤local │    │ prod │    │ stg  │   │  │
│  │    proxy     │     │ │shell │    │(SSH) │    │(SSH) │   │  │
│  └──────────────┘     │ └──────┘    └──┬───┘    └──┬───┘   │  │
│         ▲             │                │           │        │  │
│         │ Unix Socket │  ┌─────────────┴───────────┘        │  │
│         └─────────────┼──┤                                  │  │
│                       └──┼──────────────────────────────────┘  │
└──────────────────────────┼─────────────────────────────────────┘
                           │ SSH connections
              ┌────────────┴────────────┐
              ▼                         ▼
       ┌────────────┐            ┌────────────┐
       │ Production │            │  Staging   │
       │   Server   │            │   Server   │
       └────────────┘            └────────────┘
```

### 7.2 Component Design

#### 7.2.1 Daemon (`thopd`)

- Long-running background process
- Manages all SSH connections
- Maintains session state (cwd, env)
- Handles reconnection logic
- Communicates via Unix socket at `$XDG_RUNTIME_DIR/thop.sock`

#### 7.2.2 CLI (`thop`)

- Thin client that communicates with daemon
- All commands are RPC calls to daemon
- Can operate without daemon for `--help`, `start`, etc.

#### 7.2.3 Proxy (`thop proxy`)

- Special mode for AI agent integration
- Reads commands from stdin, writes output to stdout
- Maintains persistent connection to daemon
- Transparent passthrough of data

#### 7.2.4 Session

- Represents a single shell context (local or remote)
- For remote: wraps SSH connection with PTY
- Tracks: connection state, cwd, environment, history
- Implements reconnection with exponential backoff

### 7.3 Data Flow

```
Command Execution Flow:
──────────────────────

1. AI Agent writes command to stdin
   │
   ▼
2. thop proxy reads command
   │
   ▼
3. Proxy sends to daemon via Unix socket
   │
   ▼
4. Daemon routes to active session
   │
   ├─► [local] Fork and exec in local shell
   │
   └─► [remote] Send over SSH channel to remote shell
         │
         ▼
5. Output streams back through same path
   │
   ▼
6. Proxy writes to stdout/stderr
   │
   ▼
7. AI Agent reads output
```

### 7.4 State Management

```
Session State:
─────────────
{
  "name": "prod",
  "type": "ssh",
  "host": "prod.example.com",
  "user": "deploy",
  "status": "connected",
  "cwd": "/var/www/app",
  "env": {
    "RAILS_ENV": "production"
  },
  "connected_at": "2026-01-16T10:30:00Z",
  "last_command_at": "2026-01-16T14:22:15Z",
  "reconnect_attempts": 0
}

Daemon State:
─────────────
{
  "active_session": "prod",
  "sessions": { ... },
  "started_at": "2026-01-16T08:00:00Z"
}
```

---

## 8. Configuration

### 8.1 Configuration File

Location: `~/.config/thop/config.toml`

```toml
# Global settings
[settings]
default_session = "local"
command_timeout = 300
reconnect_attempts = 5
reconnect_backoff_base = 2
log_level = "info"
socket_path = "$XDG_RUNTIME_DIR/thop.sock"

# Local session (always available)
[sessions.local]
type = "local"
shell = "/bin/bash"

# Remote session example
[sessions.prod]
type = "ssh"
host = "prod.example.com"  # Can reference ~/.ssh/config alias
user = "deploy"
port = 22
identity_file = "~/.ssh/id_ed25519"
startup_commands = [
  "cd /var/www/app",
  "source .env"
]

[sessions.staging]
type = "ssh"
host = "staging"  # Uses ~/.ssh/config
startup_commands = [
  "cd /var/www/app"
]

# Jump host example
[sessions.internal]
type = "ssh"
host = "internal-server"
jump_host = "bastion.example.com"
user = "admin"
```

### 8.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `THOP_CONFIG` | Path to config file | `~/.config/thop/config.toml` |
| `THOP_SOCKET` | Path to daemon socket | `$XDG_RUNTIME_DIR/thop.sock` |
| `THOP_LOG_LEVEL` | Logging verbosity | `info` |
| `THOP_DEFAULT_SESSION` | Initial active session | `local` |

---

## 9. CLI Reference

### Commands

```
thop - Terminal Hopper for Agents

USAGE:
    thop [COMMAND] [OPTIONS]
    thop <session>              # Quick switch: thop prod

COMMANDS:
    <session>       Switch to named session (e.g., thop prod)
    start           Start the thop daemon
    stop            Stop the thop daemon
    status          Show status of all sessions
    current         Print current active session name
    exec <s> <cmd>  Execute command in specific session
    connect <s>     Establish connection to a session
    close <s>       Close a specific session
    auth <s>        Provide credentials for a session
    trust <s>       Trust host key, add to known_hosts
    proxy           Enter proxy mode (for AI agents)
    config          Show or edit configuration
    logs            View daemon and session logs
    help            Print help information
    version         Print version

AUTH OPTIONS (for 'thop auth <session>'):
    --password          Read password from stdin (single line)
    --password-env VAR  Read from environment variable
    --password-file F   Read from file (requires 0600 perms)
    --clear             Clear cached credentials

GLOBAL OPTIONS:
    -v, --verbose   Increase logging verbosity
    -q, --quiet     Suppress non-error output
    --config <path> Use alternate config file
    --json          Output in JSON format (for agents)

EXIT CODES:
    0   Success
    1   Error (details in JSON to stderr)
    2   Authentication required (use 'thop auth')
    3   Host key verification required (use 'thop trust')

EXAMPLES:
    # Start daemon
    thop start
    
    # Switch to production (uses SSH key from ~/.ssh automatically)
    thop prod
    
    # If password required, provide it securely:
    thop auth prod --password-env PROD_PASSWORD  # From env var
    thop auth prod --password-file ~/.secrets/prod  # From file
    echo "$pw" | thop auth prod --password  # From stdin
    
    # Trust a new host's key (shows fingerprint first)
    thop trust staging
    
    # Check current context
    thop current
    
    # Run one-off command without switching
    thop exec staging "docker ps"
    
    # Return to local shell
    thop local
    
    # Use as shell for AI agent
    SHELL="thop proxy" claude
```

---

## 10. Integration Guide

### 10.1 Claude Code Integration

**Option A: Shell Replacement**
```bash
# In .bashrc or before starting Claude Code
export SHELL="$(which thop) proxy"
claude
```

**Option B: Wrapper Script**
```bash
#!/bin/bash
# ~/bin/claude-remote
thop start
thop "${1:-local}"
claude
```

**Option C: Claude Code MCP (Future)**
```json
{
  "mcpServers": {
    "thop": {
      "command": "thop",
      "args": ["mcp-server"]
    }
  }
}
```

### 10.2 Generic AI Agent Integration

Any AI agent that executes shell commands can use thop:

```python
import subprocess

# Start daemon once
subprocess.run(["thop", "start"])

# Switch context
subprocess.run(["thop", "prod"])

# Execute commands - they go to prod automatically
result = subprocess.run(
    ["thop", "proxy"],
    input="ls -la\n",
    capture_output=True,
    text=True
)
print(result.stdout)
```

---

## 11. Error Handling

### 11.1 Error Categories

| Category | Code | Example | Handling |
|----------|------|---------|----------|
| Connection | `CONNECTION_FAILED` | SSH connection refused | Retry with backoff, return error |
| Connection | `CONNECTION_TIMEOUT` | Host unreachable | Return error with timeout info |
| Auth | `AUTH_PASSWORD_REQUIRED` | Key auth failed, password needed | Return error with auth instructions |
| Auth | `AUTH_KEY_REJECTED` | SSH key not accepted | Return error, suggest key setup |
| Auth | `AUTH_FAILED` | Wrong password | Return error, allow retry |
| Auth | `HOST_KEY_VERIFICATION_FAILED` | Unknown host key | Return error with fingerprint, suggest `thop trust` |
| Auth | `HOST_KEY_CHANGED` | Host key mismatch (MITM?) | Return error, require manual intervention |
| Timeout | `COMMAND_TIMEOUT` | Command exceeded timeout | Kill command, return timeout error |
| Session | `SESSION_NOT_FOUND` | Unknown session name | Return error, list available sessions |
| Session | `SESSION_DISCONNECTED` | Session lost connection | Attempt reconnect or return error |
| Daemon | `DAEMON_NOT_RUNNING` | Daemon not started | Auto-start or return clear error |

### 11.2 Error Response Format

All errors are returned as JSON to stderr with exit code 1:

```json
{
  "error": true,
  "code": "AUTH_PASSWORD_REQUIRED",
  "message": "SSH key authentication failed. Password required for prod.",
  "session": "prod",
  "host": "prod.example.com",
  "retryable": false,
  "action_required": "auth",
  "suggestion": "Run: thop auth prod --password"
}
```

### 11.3 Authentication Error Flow

When connecting to a session that requires a password:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────────────────────────┐
│ thop prod   │────►│ Try SSH key │────►│ AUTH_PASSWORD_REQUIRED (exit 1) │
└─────────────┘     │ auth first  │     │ Returns JSON with instructions  │
                    └─────────────┘     └─────────────────────────────────┘
                                                       │
                          ┌────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Agent reads error, extracts code, handles programmatically:             │
│                                                                         │
│   # Option 1: Password from environment (set by user beforehand)        │
│   thop auth prod --password-env PROD_SSH_PASSWORD                       │
│                                                                         │
│   # Option 2: Password from secure file                                 │
│   thop auth prod --password-file ~/.secrets/prod.pass                   │
│                                                                         │
│   # Option 3: Prompt user and pipe (interactive fallback)               │
│   read -s -p "Password for prod: " pw && echo "$pw" | thop auth prod    │
└─────────────────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ thop prod   (retry - now succeeds with cached credentials)              │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 12. Logging and Observability

### 12.1 Log Locations

- Daemon log: `~/.local/share/thop/daemon.log`
- Session logs: `~/.local/share/thop/sessions/<name>.log`

### 12.2 Log Format

```
2026-01-16T14:22:15.123Z INFO  [daemon] Session 'prod' connected
2026-01-16T14:22:16.456Z DEBUG [prod] Executing: ls -la
2026-01-16T14:22:16.789Z DEBUG [prod] Output: total 42...
2026-01-16T14:22:16.790Z INFO  [prod] Command completed (exit: 0)
```

### 12.3 Metrics (Future)

- Commands executed per session
- Average command latency
- Connection uptime per session
- Reconnection frequency

---

## 13. Testing Strategy

### 13.1 Unit Tests

- Session state management
- Configuration parsing
- Command routing logic
- Reconnection backoff calculation

### 13.2 Integration Tests

- Local shell execution
- SSH connection establishment
- Context switching
- Command timeout handling
- Reconnection after network drop

### 13.3 End-to-End Tests

- Full workflow with mock AI agent
- Multi-session scenarios
- Long-running command handling
- Stress testing with rapid context switches

### 13.4 Test Environments

- Docker containers for remote SSH targets
- Network simulation for disconnect testing
- CI pipeline with SSH test infrastructure

---

## 14. Implementation Phases

### Phase 1: Core MVP (Weeks 1-3)

- [ ] Daemon process with Unix socket communication
- [ ] Local session execution
- [ ] Single SSH session support
- [ ] Basic `start`, `stop`, `status`, `switch` commands
- [ ] Proxy mode for stdin/stdout passthrough
- [ ] Configuration file parsing

### Phase 2: Robustness (Weeks 4-5)

- [ ] Multiple concurrent SSH sessions
- [ ] Automatic reconnection with backoff
- [ ] Session state persistence (cwd, env)
- [ ] Command timeout handling
- [ ] `exec` command for one-off execution

### Phase 3: Polish (Weeks 6-7)

- [ ] SSH config file integration
- [ ] Jump host support
- [ ] Startup commands per session
- [ ] Logging infrastructure
- [ ] Error messages and suggestions
- [ ] Shell completions (bash, zsh, fish)

### Phase 4: Advanced Features (Weeks 8+)

- [ ] Async command execution
- [ ] Command interruption (Ctrl+C)
- [ ] Session sharing (optional)
- [ ] MCP server wrapper
- [ ] Metrics and observability

---

## 15. Open Questions

1. **Q: Should thop manage its own SSH connections or delegate to system SSH?**
   - Option A: Use `ssh` subprocess (simpler, relies on system config)
   - Option B: Use SSH library like `libssh` (more control, but complex)
   - **Recommendation**: Start with subprocess, consider library later

2. **Q: How to handle long-running commands that exceed timeout?**
   - Option A: Kill and return error
   - Option B: Transition to async mode automatically
   - **Recommendation**: Option B with clear status reporting

3. **Q: Should we support Windows (native, not WSL)?**
   - **Recommendation**: Not in initial release, evaluate demand later

4. **Q: How to handle session-specific environment without polluting global state?**
   - **Recommendation**: Track env changes per session, replay on reconnect

---

## 16. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Context switch latency | < 50ms p99 | Benchmark tests |
| Adoption | 1000 GitHub stars in 6 months | GitHub analytics |
| Reliability | < 1 crash per 1000 hours | Error tracking |
| User satisfaction | > 4.5/5 rating | User surveys |
| AI agent compatibility | Works with top 5 agents | Manual testing |

---

## 17. Appendix

### A. Competitive Analysis

| Tool | Pros | Cons |
|------|------|------|
| mcp-ssh-session | MCP integration, persistent sessions | Requires MCP setup |
| tmux + SSH | Battle-tested, widely available | Manual session management |
| Tailscale SSH | Zero config networking | Not a session manager |
| mosh | Connection resilience | Not transparent to apps |

### B. References

- [MCP SSH Session](https://github.com/devnullvoid/mcp-ssh-session)
- [Claude Code Documentation](https://docs.anthropic.com/claude-code)
- [SSH Protocol Specification](https://www.rfc-editor.org/rfc/rfc4253)
- [tmux Manual](https://man.openbsd.org/tmux)

### C. Glossary

| Term | Definition |
|------|------------|
| Session | A persistent shell context (local or remote) |
| Context | The currently active session for command execution |
| Daemon | Background process managing sessions |
| Proxy | Mode where thop acts as transparent shell |
| PTY | Pseudo-terminal for interactive shell support |
