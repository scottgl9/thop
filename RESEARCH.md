# thop Architecture Research

**Date**: January 16, 2026
**Status**: Complete
**Decision**: Shell wrapper approach with Go implementation

---

## Executive Summary

This document captures research findings comparing two architectural approaches for thop:

1. **Daemon Approach** (original PRD) - Background service with Unix socket IPC
2. **Shell Wrapper Approach** (revised) - Interactive shell wrapper with proxy mode

**Conclusion**: The shell wrapper approach is simpler, feasible, and recommended. Go is the optimal implementation language.

---

## 1. Architecture Comparison

### Original: Daemon Approach

```
┌─────────────┐     ┌─────────────────────────────────────┐
│ AI Agent    │     │            thopd (daemon)            │
└──────┬──────┘     │  ┌─────────────────────────────────┐│
       │            │  │       Session Manager           ││
       │ stdin/out  │  └──────────────┬──────────────────┘│
       ▼            │                 │                    │
┌──────────────┐    │ ┌──────┐    ┌──────┐    ┌──────┐   │
│  thop proxy  │◄───┼─┤local │    │ prod │    │ stg  │   │
└──────────────┘    │ │shell │    │(SSH) │    │(SSH) │   │
       ▲            └─────────────────────────────────────┘
       │ Unix Socket
```

**Characteristics:**
- Background daemon process (`thopd`)
- CLI communicates via Unix socket RPC
- Separate proxy mode for AI agents
- State stored in daemon memory

### Revised: Shell Wrapper Approach

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
    │ Session State Manager   │
    │ - SSH connections       │
    │ - Current context       │
    │ - CWD/env per session   │
    └─────────────────────────┘
             │
     ┌───────┴───────┐
     ▼               ▼
  Local SSH      SSH Sessions
  Shell          (persistent)
```

**Characteristics:**
- Single binary with two modes
- Interactive mode: `thop` with `(local) $` prompt
- Proxy mode: `thop --proxy` for AI agent SHELL
- Slash commands for session management
- State shared via file or lightweight socket

### Comparison Matrix

| Aspect | Daemon Approach | Shell Wrapper Approach |
|--------|-----------------|------------------------|
| Complexity | Higher (daemon + CLI + proxy) | Lower (single binary) |
| State Persistence | Daemon memory (robust) | File/socket (simpler) |
| Process Management | Requires daemon lifecycle | Self-contained |
| AI Integration | Proxy mode via socket | SHELL environment variable |
| Human UX | CLI commands only | Interactive prompt + slash commands |
| Distribution | Two binaries | Single binary |
| Failure Recovery | Daemon restart needed | Reconnect on next command |

---

## 2. AI Agent Integration Challenge

### The Core Problem

When Claude Code (or similar AI agents) runs bash commands, it spawns its own subprocess:

```python
# What Claude does internally:
subprocess.run(["bash", "-c", "ls -la"])
```

This creates a NEW shell process that doesn't inherit the parent's thop session.

### Solution: SHELL Override

```bash
# For AI agent use:
SHELL="thop --proxy" claude
```

When `SHELL` is set to thop, the AI agent's commands route through thop:

```
Claude Code
    │
    ▼ spawns SHELL
thop --proxy
    │
    ▼ routes to active session
Local Shell / SSH Session
```

### Workflow Example

```bash
# Terminal 1: Human operator
$ thop
(local) $ /connect prod        # Establish SSH connection
(local) $ /switch prod         # Switch context to prod
(prod) $ pwd                   # Commands go to prod server
/var/www/app

# Terminal 2: AI agent (same machine)
$ SHELL="thop --proxy" claude
# Claude's commands now route through thop to prod
```

---

## 3. Shell Wrapper Implementation Details

### How Python venv Works (Reference)

Python venv modifies PS1 by sourcing a script into the current shell:
```bash
PS1="(venv_name) ${PS1-}"
```

This doesn't apply to thop because:
- We can't modify the AI agent's shell
- We need to intercept commands, not just change the prompt

### How rlwrap Works (Better Model)

rlwrap is a readline wrapper that:
1. Spawns target command in a PTY
2. Monitors PTY mode (raw vs cooked)
3. Intercepts input for readline editing
4. Passes through to child process

**Relevant patterns for thop:**
- PTY handling for interactive commands
- Input interception for slash commands
- Transparent passthrough for normal commands

### Two Operating Modes

**Mode A: Interactive (for humans)**
```
$ thop
(local) $ /connect prod     # Slash command → internal handling
(local) $ /switch prod
(prod) $ ls -la             # Regular command → route to session
```

- Full PTY support
- Prompt modification showing context
- Slash command parsing
- Signal handling (Ctrl+C, Ctrl+Z)

**Mode B: Proxy (for AI agents)**
```
$ thop --proxy
# Reads stdin line-by-line
# Routes to active session
# Outputs to stdout/stderr
```

- Line-buffered I/O (no PTY needed)
- No prompt modification
- Simpler, faster
- SHELL-compatible

### State Sharing Between Modes

Options evaluated:

| Method | Pros | Cons |
|--------|------|------|
| State file | Simple, no daemon | Concurrency issues |
| Unix socket (light) | Robust, handles concurrency | Slightly more complex |
| Environment variables | No persistence needed | Can't share SSH connections |

**Recommendation**: Lightweight Unix socket for state coordination, but SSH connections managed per-process with reconnection support.

---

## 4. Language Evaluation

### Candidates

| Language | Consideration |
|----------|---------------|
| Rust | Maximum performance, excellent PTY/SSH crates |
| Go | Great balance, excellent concurrency, simpler |
| TypeScript | Rapid development, but runtime dependency |
| Python | Easy development, but slow and requires interpreter |

### Detailed Comparison

#### Rust

**Pros:**
- Fastest execution (2x faster than Go, 60x faster than Python)
- Excellent PTY support (`portable_pty`, `pty-process`)
- Good SSH libraries (`russh`, `ssh2`)
- Single binary distribution
- Memory safety guarantees

**Cons:**
- Steeper learning curve
- Longer development time
- Borrow checker complexity for async + state management

**Relevant Crates:**
- `clap` - CLI argument parsing
- `portable_pty` - Cross-platform PTY
- `russh` - Pure Rust SSH client
- `tokio` - Async runtime
- `ratatui` - Terminal UI (if needed)

#### Go

**Pros:**
- Excellent concurrency (goroutines trivialize multi-session management)
- Native SSH support (`golang.org/x/crypto/ssh`)
- Single binary distribution
- Fast enough (context switch < 50ms easily achievable)
- Simpler than Rust, faster development
- Battle-tested CLI ecosystem (Cobra, Viper)

**Cons:**
- Slightly larger binaries than Rust
- No generics until recently (less relevant now)
- GC pauses (negligible for CLI tools)

**Relevant Packages:**
- `cobra` - CLI framework
- `golang.org/x/crypto/ssh` - SSH client
- `creack/pty` - PTY handling
- `bubbletea` - Terminal UI (if needed)

#### TypeScript

**Pros:**
- Rapid development
- Good ecosystem (Ink for terminal UI)
- What Claude Code itself uses

**Cons:**
- Requires Node.js runtime
- PTY support requires native dependencies
- Larger distribution size
- Slower startup time

#### Python

**Pros:**
- Fastest development time
- Good libraries (`paramiko` for SSH, `pexpect` for PTY)
- Easy to prototype

**Cons:**
- Requires Python interpreter
- Slowest performance
- Distribution complexity (venv, dependencies)
- PTY handling is awkward

### Performance Requirements Check

From PRD:
- Context switch latency: < 50ms
- Command routing overhead: < 10ms
- Memory usage (idle): < 50MB

All candidates can meet these requirements. Performance is not the differentiator.

### Decision Matrix

| Factor | Weight | Rust | Go | TypeScript | Python |
|--------|--------|------|-----|------------|--------|
| Development speed | 25% | 2 | 4 | 5 | 5 |
| Performance | 15% | 5 | 4 | 3 | 2 |
| Distribution | 20% | 5 | 5 | 2 | 2 |
| SSH support | 20% | 4 | 5 | 3 | 4 |
| PTY support | 10% | 5 | 4 | 2 | 3 |
| Maintainability | 10% | 3 | 4 | 4 | 4 |
| **Weighted Score** | | **3.7** | **4.4** | **3.3** | **3.4** |

### Recommendation: Go

Go provides the best balance for thop:

1. **Single binary** - Easy distribution, no runtime dependencies
2. **Native SSH** - `golang.org/x/crypto/ssh` is production-grade
3. **Goroutines** - Trivial concurrency for managing multiple sessions
4. **Fast development** - Ship faster than Rust
5. **Ecosystem** - Cobra for CLI, excellent standard library

---

## 5. Implementation Strategy

### Revised Architecture

```
thop/
├── cmd/
│   └── thop/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/
│   │   ├── interactive.go    # Interactive mode
│   │   └── proxy.go          # Proxy mode (SHELL compat)
│   ├── session/
│   │   ├── manager.go        # Session state management
│   │   ├── local.go          # Local shell session
│   │   └── ssh.go            # SSH session
│   ├── config/
│   │   └── config.go         # TOML configuration
│   └── state/
│       └── state.go          # Shared state (file/socket)
├── go.mod
├── go.sum
└── configs/
    └── example.toml
```

### Slash Commands

| Command | Action |
|---------|--------|
| `/connect <session>` | Establish SSH connection |
| `/switch <session>` | Change active context |
| `/local` | Switch to local shell |
| `/status` | Show all sessions |
| `/close <session>` | Close SSH connection |
| `/help` | Show available commands |

### State Management

```go
type State struct {
    ActiveSession string            `json:"active_session"`
    Sessions      map[string]Session `json:"sessions"`
}

type Session struct {
    Name      string            `json:"name"`
    Type      string            `json:"type"` // "local" or "ssh"
    Connected bool              `json:"connected"`
    CWD       string            `json:"cwd"`
    Env       map[string]string `json:"env"`
}
```

### AI Agent Integration

```bash
# Option 1: SHELL override (recommended)
SHELL="thop --proxy" claude

# Option 2: Wrapper script
#!/bin/bash
# ~/bin/claude-thop
export SHELL="$(which thop) --proxy"
exec claude "$@"

# Option 3: Shell alias
alias claude-remote='SHELL="thop --proxy" claude'
```

---

## 6. Key Implementation Challenges

### Challenge 1: SSH Connection Persistence

**Problem**: Each thop process needs access to SSH connections.

**Solution Options**:
1. **Per-process connections**: Each thop instance maintains its own SSH. Reconnect as needed.
2. **Connection sharing via socket**: Light daemon holds connections, thop instances communicate.
3. **SSH ControlMaster**: Leverage OpenSSH's built-in connection sharing.

**Recommendation**: Start with option 1 (simpler), add SSH ControlMaster support for optimization.

### Challenge 2: State Synchronization

**Problem**: Interactive mode and proxy mode need consistent view of active session.

**Solution**:
- State file at `~/.local/share/thop/state.json`
- File locking for concurrent access
- Watch for changes (fsnotify) in interactive mode

### Challenge 3: PTY Handling

**Problem**: Interactive commands (vim, top) need PTY support.

**Solution**:
- Interactive mode: Full PTY via `creack/pty`
- Proxy mode: Line-buffered I/O (PTY optional)
- Detect if stdin is TTY to choose mode

### Challenge 4: Signal Forwarding

**Problem**: Ctrl+C should interrupt remote command, not kill thop.

**Solution**:
```go
// Catch SIGINT, forward to active session
signal.Notify(sigChan, syscall.SIGINT)
go func() {
    for range sigChan {
        activeSession.SendSignal(ssh.SIGINT)
    }
}()
```

---

## 7. Migration from Original PRD

### What Changes

| Original PRD | New Approach |
|--------------|--------------|
| `thopd` daemon | Removed (no daemon) |
| `thop start/stop` | Removed (no daemon lifecycle) |
| `thop proxy` | `thop --proxy` flag |
| Unix socket IPC | State file + optional socket |
| Rust implementation | Go implementation |

### What Stays the Same

- Configuration file format (`~/.config/thop/config.toml`)
- Session concepts (local, SSH)
- Non-blocking authentication errors
- SSH config integration
- Environment variable overrides
- Error response format (JSON)

### New Additions

- Interactive mode with prompt `(session) $`
- Slash commands (`/connect`, `/switch`, etc.)
- `SHELL` compatibility for AI agents

---

## 8. Conclusion

### Recommendation Summary

1. **Architecture**: Shell wrapper approach (no daemon)
2. **Language**: Go
3. **Modes**: Interactive (human) + Proxy (AI agent)
4. **State**: File-based with file locking
5. **SSH**: Per-process with ControlMaster optimization later

### Benefits

- Simpler architecture (single binary)
- Better human UX (interactive prompt)
- Same AI agent support (SHELL override)
- Faster development (Go vs Rust)
- Easier maintenance

### Trade-offs Accepted

- SSH connections not shared between processes (acceptable, use ControlMaster)
- File-based state vs in-memory (acceptable for CLI tool)
- Go vs Rust performance (negligible difference for this use case)

---

## References

- [Go vs Python vs Rust Comparison](https://pullflow.com/blog/go-vs-python-vs-rust-complete-performance-comparison)
- [Best Languages for CLI Utilities](https://www.slant.co/topics/2469/~best-languages-for-writing-command-line-utilities)
- [Rust vs Go in 2025](https://blog.jetbrains.com/rust/2025/06/12/rust-vs-go/)
- [AI CLI Tools Comparison](https://mer.vin/2025/12/ai-cli-tools-comparison-why-openai-switched-to-rust-while-claude-code-stays-with-typescript/)
- [rlwrap - readline wrapper](https://github.com/hanslub42/rlwrap)
- [portable_pty Rust crate](https://docs.rs/portable-pty)
- [russh - Rust SSH library](https://github.com/Eugeny/russh)
- [Go crypto/ssh package](https://pkg.go.dev/golang.org/x/crypto/ssh)
- [creack/pty - Go PTY package](https://github.com/creack/pty)
