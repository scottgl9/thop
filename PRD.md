# Product Requirements Document: thop

## Terminal Hopper for Agents

> *Seamlessly switch between local and remote terminals without interrupting agent flow*

**Version:** 0.2.0
**Status:** Revised
**Last Updated:** January 16, 2026
**Implementation Language:** Go and Rust (evaluating both)

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

thop is an interactive shell wrapper with two operating modes:

1. **Interactive Mode**: Run `thop` to get an interactive shell with a `(local) $` prompt. Use slash commands (`/connect`, `/switch`) to manage sessions.

2. **Proxy Mode**: Run `thop --proxy` as the `SHELL` environment variable for AI agents. Commands route transparently to the active session.

The agent executes commands normally, and thop routes them to the appropriate destination (local or remote) based on the current context. Sessions persist within the thop process, and state is shared via a lightweight state file.

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

thop is a single binary with two operating modes: Interactive and Proxy.

```
┌─────────────────────────────────────────────────────────────────┐
│                        User's Machine                           │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              thop (single binary)                        │   │
│  │  ┌─────────────────────┬─────────────────────────────┐  │   │
│  │  │  Interactive Mode   │      Proxy Mode             │  │   │
│  │  │  - (local) $ prompt │  - SHELL compatible         │  │   │
│  │  │  - Slash commands   │  - Line-by-line I/O         │  │   │
│  │  │  - Human UX         │  - For AI agents            │  │   │
│  │  └─────────────────────┴─────────────────────────────┘  │   │
│  │                         │                                │   │
│  │              ┌──────────┴──────────┐                    │   │
│  │              ▼                     ▼                    │   │
│  │  ┌─────────────────────────────────────────────────┐   │   │
│  │  │           Session Manager                        │   │   │
│  │  │  - Manages SSH connections                       │   │   │
│  │  │  - Tracks state (cwd, env) per session          │   │   │
│  │  │  - Handles reconnection                          │   │   │
│  │  └─────────────────────────────────────────────────┘   │   │
│  │              │                                          │   │
│  │      ┌───────┼───────┬───────────┐                     │   │
│  │      ▼       ▼       ▼           ▼                     │   │
│  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                  │   │
│  │  │local │ │ prod │ │ stg  │ │ dev  │                  │   │
│  │  │shell │ │(SSH) │ │(SSH) │ │(SSH) │                  │   │
│  │  └──────┘ └──┬───┘ └──┬───┘ └──┬───┘                  │   │
│  └──────────────┼────────┼────────┼─────────────────────┘   │
│                 │        │        │                          │
└─────────────────┼────────┼────────┼──────────────────────────┘
                  │ SSH    │        │
       ┌──────────┘        │        └──────────┐
       ▼                   ▼                   ▼
┌────────────┐      ┌────────────┐      ┌────────────┐
│ Production │      │  Staging   │      │    Dev     │
│   Server   │      │   Server   │      │   Server   │
└────────────┘      └────────────┘      └────────────┘
```

### 7.2 Component Design

#### 7.2.1 Interactive Mode (`thop`)

- Default mode when running `thop` with no arguments
- Displays prompt with current context: `(local) $`, `(prod) $`
- Parses slash commands for session management
- Full PTY support for interactive commands
- Signal handling (Ctrl+C forwarded to active session)

#### 7.2.2 Proxy Mode (`thop --proxy`)

- Designed for AI agent integration
- Set as `SHELL` environment variable: `SHELL="thop --proxy" claude`
- Line-buffered I/O (reads stdin, writes stdout/stderr)
- No prompt modification
- Transparent command passthrough to active session

#### 7.2.3 Session Manager

- Manages local shell and SSH sessions
- Tracks per-session state (cwd, environment variables)
- Handles SSH connection lifecycle
- Implements reconnection with exponential backoff

#### 7.2.4 State Sharing

- State file at `~/.local/share/thop/state.json`
- File locking for concurrent access
- Allows multiple thop instances to share active session

### 7.3 Data Flow

```
Interactive Mode Flow:
──────────────────────

1. User types command at (prod) $ prompt
   │
   ├─► If starts with "/" → Handle as slash command
   │                         /connect, /switch, /status, etc.
   │
   └─► Otherwise → Route to active session
         │
         ▼
2. Session Manager sends to local shell or SSH
   │
   ▼
3. Output displayed to user


Proxy Mode Flow (AI Agent):
───────────────────────────

1. AI Agent spawns SHELL (thop --proxy)
   │
   ▼
2. Agent writes command to stdin
   │
   ▼
3. thop reads line, routes to active session
   │
   ├─► [local] Execute in local shell
   │
   └─► [remote] Send over SSH channel
         │
         ▼
4. Output streams to stdout/stderr
   │
   ▼
5. AI Agent reads output
```

### 7.4 State Management

```
State File (~/.local/share/thop/state.json):
────────────────────────────────────────────
{
  "active_session": "prod",
  "sessions": {
    "local": {
      "type": "local",
      "cwd": "/home/user/projects",
      "env": {}
    },
    "prod": {
      "type": "ssh",
      "host": "prod.example.com",
      "user": "deploy",
      "connected": true,
      "cwd": "/var/www/app",
      "env": {
        "RAILS_ENV": "production"
      }
    }
  },
  "updated_at": "2026-01-16T14:22:15Z"
}
```

### 7.5 Slash Commands

| Command | Action |
|---------|--------|
| `/connect <session>` | Establish SSH connection to configured session |
| `/switch <session>` | Change active context |
| `/local` | Switch to local shell (shortcut for `/switch local`) |
| `/status` | Show all sessions and connection state |
| `/close <session>` | Close SSH connection |
| `/help` | Show available commands |

### 7.6 Project Structure

```
thop/
├── cmd/
│   └── thop/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/
│   │   ├── interactive.go    # Interactive mode
│   │   ├── proxy.go          # Proxy mode
│   │   └── commands.go       # Slash command handlers
│   ├── session/
│   │   ├── manager.go        # Session lifecycle management
│   │   ├── local.go          # Local shell session
│   │   └── ssh.go            # SSH session (golang.org/x/crypto/ssh)
│   ├── config/
│   │   └── config.go         # TOML configuration parsing
│   └── state/
│       └── state.go          # Shared state file management
├── go.mod
├── go.sum
├── Makefile
└── configs/
    └── example.toml
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
state_file = "~/.local/share/thop/state.json"

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
| `THOP_STATE_FILE` | Path to state file | `~/.local/share/thop/state.json` |
| `THOP_LOG_LEVEL` | Logging verbosity | `info` |
| `THOP_DEFAULT_SESSION` | Initial active session | `local` |

---

## 9. CLI Reference

### 9.1 Command Line Usage

```
thop - Terminal Hopper for Agents

USAGE:
    thop [OPTIONS]              # Start interactive mode
    thop --proxy                # Start proxy mode (for AI agents)
    thop --status               # Show status and exit
    thop --version              # Show version and exit
    thop --help                 # Show help and exit

OPTIONS:
    --proxy         Run in proxy mode (SHELL compatible)
    --status        Show all sessions and exit
    --config <path> Use alternate config file
    --json          Output in JSON format
    -v, --verbose   Increase logging verbosity
    -q, --quiet     Suppress non-error output
    -h, --help      Print help information
    -V, --version   Print version

EXIT CODES:
    0   Success
    1   Error (details in JSON to stderr)
    2   Authentication required
    3   Host key verification required
```

### 9.2 Interactive Mode Slash Commands

When running `thop` in interactive mode, use slash commands:

```
SLASH COMMANDS (in interactive mode):
    /connect <session>  Establish SSH connection to session
    /switch <session>   Change active context
    /local              Switch to local shell (alias for /switch local)
    /status             Show all sessions and connection state
    /close <session>    Close SSH connection
    /auth <session>     Provide credentials for session
    /trust <session>    Trust host key, add to known_hosts
    /help               Show available commands

```

### 9.3 Examples

```bash
# Start thop in interactive mode
$ thop
(local) $ ls -la                    # Commands run locally
(local) $ /connect prod             # Establish SSH to prod
Connecting to prod (prod.example.com)... connected
(local) $ /switch prod              # Switch context to prod
(prod) $ pwd                        # Commands now run on prod
/var/www/app
(prod) $ docker ps                  # Run commands remotely
(prod) $ /local                     # Switch back to local
(local) $ /status                   # Show all sessions
Sessions:
  local   [active] /home/user/projects
  prod    [connected] /var/www/app

# If password required:
(local) $ /auth prod                # Will prompt for password

# Trust a new host (shows fingerprint):
(local) $ /trust staging

# Use as shell for AI agent
$ SHELL="thop --proxy" claude
```

---

## 10. Integration Guide

### 10.1 Claude Code Integration

**Recommended: SHELL Override**
```bash
# Terminal 1: Setup thop session
$ thop
(local) $ /connect prod
(local) $ /switch prod
(prod) $                          # Leave running

# Terminal 2: Start Claude with thop as SHELL
$ SHELL="thop --proxy" claude     # Claude's commands go through thop
```

**Alternative: Wrapper Script**
```bash
#!/bin/bash
# ~/bin/claude-thop
export SHELL="$(which thop) --proxy"
exec claude "$@"
```

**Future: Claude Code MCP**
```json
{
  "mcpServers": {
    "thop": {
      "command": "thop",
      "args": ["--mcp-server"]
    }
  }
}
```

### 10.2 Generic AI Agent Integration

Any AI agent that executes shell commands can use thop by setting SHELL:

```python
import subprocess
import os

# Set thop as the shell for subprocesses
os.environ["SHELL"] = "thop --proxy"

# Execute commands - they route to the active thop session
result = subprocess.run(
    ["thop", "--proxy"],
    input="ls -la\n",
    capture_output=True,
    text=True
)
print(result.stdout)
```

### 10.3 Workflow

```
┌──────────────────────────────────────────────────────────────────┐
│  1. User starts thop in Terminal 1                               │
│     $ thop                                                       │
│     (local) $ /connect prod                                      │
│     (local) $ /switch prod                                       │
│     (prod) $                                                     │
│                                                                  │
│  2. User starts AI agent with thop as SHELL in Terminal 2       │
│     $ SHELL="thop --proxy" claude                               │
│                                                                  │
│  3. AI agent commands flow through thop to prod server          │
│     Claude: "Run ls -la"                                        │
│     → thop --proxy receives "ls -la"                            │
│     → Routes to active session (prod)                           │
│     → SSH executes on prod.example.com                          │
│     → Output returned to Claude                                  │
└──────────────────────────────────────────────────────────────────┘
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
| State | `STATE_FILE_ERROR` | Cannot read/write state | Return error with path info |

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

### Phase 0: Language Evaluation

Implement minimal prototype in both Go and Rust to evaluate:

**Go Prototype (`thop-go/`)**
- [ ] Basic interactive mode with prompt
- [ ] Local shell command execution
- [ ] Single SSH session using `golang.org/x/crypto/ssh`
- [ ] Slash command parsing (`/connect`, `/switch`, `/status`)
- [ ] Proxy mode (`--proxy`)

**Rust Prototype (`thop-rust/`)**
- [ ] Basic interactive mode with prompt
- [ ] Local shell command execution
- [ ] Single SSH session using `russh` crate
- [ ] Slash command parsing (`/connect`, `/switch`, `/status`)
- [ ] Proxy mode (`--proxy`)

**Evaluation Criteria:**
- Code complexity and maintainability
- Binary size and startup time
- SSH library ergonomics
- PTY handling ease
- Developer experience

### Phase 1: Core MVP

After language selection:

- [ ] Interactive mode with `(session) $` prompt
- [ ] Local shell session management
- [ ] Single SSH session support
- [ ] Slash commands: `/connect`, `/switch`, `/local`, `/status`, `/close`
- [ ] Proxy mode (`--proxy`) for AI agents
- [ ] State file for session sharing
- [ ] Configuration file parsing (TOML)
- [ ] Basic error handling with JSON output

### Phase 2: Robustness

- [ ] Multiple concurrent SSH sessions
- [ ] Automatic reconnection with exponential backoff
- [ ] Session state persistence (cwd, env vars)
- [ ] Command timeout handling
- [ ] File locking for concurrent state access
- [ ] Signal handling (Ctrl+C forwarding)

### Phase 3: Polish

- [ ] SSH config file integration (`~/.ssh/config`)
- [ ] SSH key and agent support
- [ ] Jump host / bastion support
- [ ] Startup commands per session
- [ ] Logging infrastructure
- [ ] `/auth` and `/trust` commands
- [ ] Shell completions (bash, zsh, fish)

### Phase 4: Advanced Features

- [ ] PTY support for interactive commands
- [ ] Async command execution
- [ ] Command history per session
- [ ] MCP server wrapper (future)
- [ ] Metrics and observability

---

## 15. Open Questions

1. **Q: Should thop manage its own SSH connections or delegate to system SSH?**
   - **Decision**: Use native SSH library (Go: `golang.org/x/crypto/ssh`, Rust: `russh`)
   - Provides more control over connection lifecycle and non-blocking behavior

2. **Q: How to handle long-running commands that exceed timeout?**
   - Option A: Kill and return error
   - Option B: Transition to async mode automatically
   - **Recommendation**: Option B with clear status reporting

3. **Q: Should we support Windows (native, not WSL)?**
   - **Recommendation**: Not in initial release, evaluate demand later

4. **Q: How to handle session-specific environment without polluting global state?**
   - **Decision**: Track env changes per session in state file, replay on reconnect

5. **Q: Daemon vs shell wrapper architecture?**
   - **Decision**: Shell wrapper approach (no daemon)
   - Simpler architecture, single binary, state shared via file

6. **Q: Go vs Rust for implementation?**
   - **Decision**: Prototype both, evaluate based on criteria in Phase 0

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
