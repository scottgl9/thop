# thop Guide for AI Agents

**thop** is a terminal session manager that enables AI agents to execute commands across local and remote SSH servers seamlessly.

## Detection

Check if running in thop:
```bash
echo $THOP_SESSION
```

Returns session name (`local`, `prod`, etc.) if in thop, empty otherwise.

## Core Concepts

- **Sessions**: Local or remote SSH contexts with independent state
- **Active Session**: Only one session is active at a time (all commands run there)
- **State Persistence**: Each session maintains its own working directory and environment

## Essential Commands

### Session Management
```bash
/status              # List all sessions and their status
/connect <session>   # Connect to SSH session
/switch <session>    # Switch to different session
/local               # Switch to local session
/close <session>     # Close SSH connection
```

### File Operations
```bash
/copy <src> <dst>    # Copy between sessions (e.g., /copy local:file prod:/path/)
/read <path>         # Read file from current session
/write <path> <content>  # Write to file in current session
```

### Environment & Jobs
```bash
/env [KEY=VALUE]     # Show or set environment (persists in session)
/bg <command>        # Run command in background
/jobs                # List background jobs
/fg <job-id>         # Wait for background job
/kill <job-id>       # Kill background job
```

### Other
```bash
/shell <command>     # Interactive command with PTY (vim, top, etc.)
/add-session <name> <host>  # Add new SSH session
/auth <session>      # Provide password for SSH
/trust <session>     # Trust host key
```

## Typical Workflow

### Example: Deploy to Production

```bash
# 1. Check available sessions
/status

# 2. Connect to server
/connect prod

# 3. Execute deployment
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

### Example: Compare Configurations

```bash
# Copy configs from different servers to local
/copy dev:/etc/app/config.yaml local:/tmp/dev-config
/copy prod:/etc/app/config.yaml local:/tmp/prod-config

# Switch to local and compare
/local
diff /tmp/dev-config /tmp/prod-config
```

### Example: Multi-Server Operation

```bash
# Update all servers
/connect server1
cd /app && git pull && sudo systemctl restart app

/switch server2
cd /app && git pull && sudo systemctl restart app

/switch server3
cd /app && git pull && sudo systemctl restart app

/local
```

## Best Practices

1. **Always start with `/status`** - Verify available sessions
2. **Return to `/local` when done** - Clean workflow pattern
3. **Use `/copy` not `scp`** - Leverages existing connections
4. **Use `/env` not `export`** - Persists in session state
5. **Verify active session** - Check `$THOP_SESSION` before destructive commands
6. **Handle errors gracefully** - Check error codes and take appropriate action

## Error Handling

Common errors and solutions:

```bash
# Session not found
/connect newserver
# Error: Session 'newserver' not found
# → Add session: /add-session newserver host.example.com

# Authentication failed
/connect prod
# Error: AUTH_KEY_FAILED
# → Provide password: /auth prod

# Host key unknown
/connect newserver
# Error: HOST_KEY_UNKNOWN
# → Trust host: /trust newserver
```

## Configuration

Sessions are defined in `~/.config/thop/config.toml`:

```toml
[sessions.prod]
type = "ssh"
host = "prod.example.com"
user = "deploy"

[sessions.dev]
type = "ssh"
host = "dev.example.com"
user = "ubuntu"
```

thop also reads `~/.ssh/config` for host definitions.

## Quick Reference

| Operation | Command |
|-----------|---------|
| List sessions | `/status` |
| Connect to server | `/connect prod` |
| Switch session | `/switch dev` |
| Return to local | `/local` |
| Copy file | `/copy src:file dst:path` |
| Read file | `/read /path/file` |
| Set env var | `/env VAR=value` |
| Background job | `/bg command` |
| List jobs | `/jobs` |
| Interactive shell | `/shell vim file` |

## State Management

Each session maintains:
- **Current working directory**: `cd` changes persist
- **Environment variables**: Set with `/env` (not `export`)
- **Connection status**: View with `/status`

State is preserved:
- When switching between sessions
- Across thop restarts (stored in `~/.local/share/thop/state.json`)

## Common Pitfalls

❌ **Don't**: Use `export` for environment
```bash
export VAR=value  # Not tracked by thop
```

✅ **Do**: Use `/env`
```bash
/env VAR=value    # Persists in session
```

---

❌ **Don't**: Use `scp` between sessions
```bash
scp file user@host:/path  # Requires separate SSH
```

✅ **Do**: Use `/copy`
```bash
/copy local:file remote:/path  # Uses existing connection
```

---

❌ **Don't**: Run interactive commands directly
```bash
vim config.yaml   # Will hang
```

✅ **Do**: Use `/shell`
```bash
/shell vim config.yaml  # Proper PTY
```

## Performance Notes

- Session switching is instant (<50ms)
- Commands have configurable timeout (default: 300s)
- State file uses locking for concurrent access
- Each SSH session adds ~10MB memory

## Security

- Never stores credentials
- State file has user-only permissions
- No sensitive data in logs
- Never auto-accepts host keys
- Password files require 0600 permissions

---

**Key Principle**: All operations return immediately. If authentication fails or requires user input, thop returns an actionable error - it never blocks waiting for input.
