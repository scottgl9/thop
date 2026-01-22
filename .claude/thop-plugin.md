# thop Plugin for Claude Code

This plugin provides Claude Code with comprehensive instructions for using thop - a terminal session manager for AI agents.

**This is a completely self-contained plugin** - all instructions are embedded in this file and can be copied to any project.

---

## Detection

You may be running inside **thop**. Check by running:

```bash
echo $THOP_SESSION
```

If this returns a session name (like `local`, `prod`, `dev`), you're in thop and have access to all thop commands.

---

## What is thop?

**thop** is a terminal session manager that enables AI agents to execute commands across local and remote SSH servers seamlessly. It provides:

- Multiple sessions (local + remote SSH servers)
- Instant context switching between sessions
- Per-session state (working directory, environment variables)
- File operations between sessions
- Background job management

---

## Core Concepts

- **Sessions**: Local or remote SSH contexts with independent state
- **Active Session**: Only one session is active at a time (all commands run there)
- **State Persistence**: Each session maintains its own working directory and environment
- **Non-blocking**: All operations return immediately with actionable errors

---

## Essential Commands

### Session Management

```bash
/status              # List all sessions, show which is active (*)
/connect <session>   # Connect to SSH session
/switch <session>    # Switch to different session
/local               # Switch to local session
/close <session>     # Close SSH connection
```

### File Operations

```bash
/copy <src> <dst>    # Copy between sessions
                     # Example: /copy local:app.js prod:/var/www/app/
                     # Example: /copy prod:/etc/nginx.conf local:/tmp/

/read <path>         # Read file from current session
/write <path> <content>  # Write to file in current session
```

### Environment & Jobs

```bash
/env                 # Show environment variables
/env KEY=value       # Set environment variable (persists in session)
/bg <command>        # Run command in background
/jobs                # List background jobs
/fg <job-id>         # Wait for background job and show output
/kill <job-id>       # Kill a running background job
```

### Interactive Commands

```bash
/shell <command>     # Run interactive command with PTY
                     # Use for: vim, top, htop, nano, etc.
                     # Example: /shell vim config.yaml
```

### Configuration

```bash
/add-session <name> <host>  # Add new SSH session dynamically
/auth <session>             # Provide password for SSH
/trust <session>            # Trust host key for SSH
```

---

## Workflow Guidelines

### 1. Always Start with /status

Before any work, check what sessions are available:

```bash
/status
```

This shows:
- All configured sessions
- Which session is active (marked with `*`)
- Connection status of each session

### 2. Announce Session Switches

When switching sessions, tell the user what you're doing:

```
"Switching to production server to check logs..."
/connect prod
```

### 3. Return to /local When Done

Clean workflow pattern - always return to local when finished with remote work:

```bash
# Work on remote
/connect prod
# ... do stuff ...

# Return to local
/local
```

### 4. Use /copy Instead of scp

```bash
# ✅ Good - uses existing thop connection
/copy local:app.js prod:/var/www/app/

# ❌ Avoid - requires separate SSH connection
scp app.js user@prod:/var/www/app/
```

### 5. Use /env Instead of export

```bash
# ✅ Good - persists in thop session state
/env NODE_ENV=production

# ❌ Avoid - not tracked by thop
export NODE_ENV=production
```

### 6. Verify Before Destructive Commands

Always check which session you're in before running destructive commands:

```bash
echo $THOP_SESSION  # Verify you're on the right server
rm -rf /some/path   # Then proceed if correct
```

---

## Example Workflows

### Deploy to Production

```bash
# 1. Check sessions
/status

# 2. Connect to prod
/connect prod

# 3. Deploy
cd /var/www/app
git pull origin main
npm install
npm run build
sudo systemctl restart app

# 4. Verify
curl http://localhost:8080/health

# 5. Return to local
/local
```

### Debug Production Issue

```bash
# Connect and investigate
/connect prod
tail -100 /var/log/app/error.log
journalctl -u app -n 50

# Copy logs to local for analysis
/copy prod:/var/log/app/error.log local:/tmp/prod-errors.log

# Return to local and analyze
/local
grep "ERROR" /tmp/prod-errors.log
```

### Compare Configs Between Servers

```bash
# Copy configs from different servers to local
/copy dev:/etc/app/config.yaml local:/tmp/dev-config
/copy prod:/etc/app/config.yaml local:/tmp/prod-config

# Switch to local and compare
/local
diff /tmp/dev-config /tmp/prod-config
```

### Deploy to Multiple Servers

```bash
# Update staging
/connect staging
cd /app && git pull && npm run build && sudo systemctl restart app

# Update prod
/switch prod
cd /app && git pull && npm run build && sudo systemctl restart app

# Back to local
/local
```

### Background Job Example

```bash
# Start long-running build in background
/bg npm run build

# Check job status
/jobs

# Continue with other work...
ls -la

# Check on the job later
/fg 1  # Wait for job 1 and show output
```

---

## Error Handling

### Session Not Found

```bash
/connect newserver
# Error: Session 'newserver' not found
# → Solution: Add the session first
/add-session newserver host.example.com
/connect newserver
```

### Authentication Failed

```bash
/connect prod
# Error: AUTH_KEY_FAILED
# → Solution: Provide password
/auth prod
# (user will be prompted for password)
/connect prod
```

### Host Key Unknown

```bash
/connect newserver
# Error: HOST_KEY_UNKNOWN
# → Solution: Trust the host key
/trust newserver
/connect newserver
```

---

## Configuration

### Session Configuration

Sessions are defined in `~/.config/thop/config.toml`:

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
port = 22
identity_file = "~/.ssh/prod_key"

[sessions.dev]
type = "ssh"
host = "dev.example.com"
user = "ubuntu"
startup_commands = [
    "cd ~/project",
    "source venv/bin/activate"
]
```

### SSH Config Integration

thop automatically reads `~/.ssh/config` for host definitions:

```
# ~/.ssh/config
Host prod
    HostName prod.example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/prod_key
```

Then in thop config:

```toml
[sessions.prod]
type = "ssh"
host = "prod"  # Will resolve from SSH config
```

---

## State Management

Each session maintains independent state:

- **Current working directory**: `cd` changes persist across commands
- **Environment variables**: Set with `/env` (not `export`)
- **Connection status**: View with `/status`

State is preserved:
- When switching between sessions
- Across thop restarts (stored in `~/.local/share/thop/state.json`)

---

## Common Pitfalls

### ❌ Don't use `export` for environment

```bash
export VAR=value  # Not tracked by thop
```

**✅ Do use `/env`**

```bash
/env VAR=value    # Persists in session state
```

---

### ❌ Don't use `scp` between sessions

```bash
scp file user@host:/path  # Requires separate SSH connection
```

**✅ Do use `/copy`**

```bash
/copy local:file remote:/path  # Uses existing thop connection
```

---

### ❌ Don't run interactive commands directly

```bash
vim config.yaml   # Will hang without PTY
top               # Will hang without PTY
```

**✅ Do use `/shell`**

```bash
/shell vim config.yaml  # Proper PTY allocation
/shell top              # Works correctly
```

---

### ❌ Don't forget to check active session

```bash
rm -rf /data/*    # Which server am I on?!
```

**✅ Do verify first**

```bash
echo $THOP_SESSION     # Check: "prod"
# Realize you meant to be on staging
/switch staging
echo $THOP_SESSION     # Check: "staging"
rm -rf /data/*         # Now safe
```

---

## Quick Reference Table

| Operation | Command |
|-----------|---------|
| List sessions | `/status` |
| Connect to server | `/connect prod` |
| Switch session | `/switch dev` |
| Return to local | `/local` |
| Copy file | `/copy src:file dst:path` |
| Read file | `/read /path/file` |
| Write file | `/write /path/file "content"` |
| Show environment | `/env` |
| Set env var | `/env VAR=value` |
| Background job | `/bg long-command` |
| List jobs | `/jobs` |
| Wait for job | `/fg <job-id>` |
| Kill job | `/kill <job-id>` |
| Interactive shell | `/shell vim file` |
| Add session | `/add-session name host` |
| Authenticate | `/auth session` |
| Trust host key | `/trust session` |
| Close session | `/close session` |

---

## Performance Notes

- **Session switching**: Instant (<50ms)
- **Command timeout**: Configurable (default: 300s)
- **State file**: Uses locking for concurrent access
- **Memory per session**: ~10MB
- **Command overhead**: <10ms

---

## Security

- **Never stores passwords** in config or state files
- **State file permissions**: User-only (0600)
- **No credentials in logs**: Sanitized output
- **Never auto-accepts host keys**: Must use `/trust` explicitly
- **Password files**: Require 0600 permissions

---

## Key Principle

**All operations return immediately.** If authentication fails or requires user input, thop returns an actionable error - it never blocks waiting for input.

This ensures AI agents can always make progress and provide feedback to users about what's needed.

---

## Usage Tips for AI Agents

1. **Start every multi-server task with `/status`** to show the user what's available
2. **Announce session switches** so the user knows where commands are running
3. **Verify the active session** before destructive operations
4. **Return to `/local`** when done to leave the system in a clean state
5. **Use `/copy` for file transfers** to leverage existing connections
6. **Handle errors gracefully** and explain to the user what action is needed
7. **Use background jobs** (`/bg`) for long-running tasks to show progress

---

**This plugin is completely self-contained and can be copied to any project that uses thop.**
