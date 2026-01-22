import type { Plugin } from "@opencode-ai/plugin"

/**
 * thop Plugin for OpenCode
 * 
 * This plugin detects if OpenCode is running inside thop and provides
 * context-aware instructions for working with remote sessions.
 * 
 * This plugin is completely self-contained - all documentation is embedded.
 */
export const ThopPlugin: Plugin = async ({ project, client, $, directory, worktree }) => {
  const isInThop = async (): Promise<boolean> => {
    try {
      const result = await $`echo $THOP_SESSION`.quiet()
      return result.text().trim().length > 0
    } catch {
      return false
    }
  }

  const getCurrentSession = async (): Promise<string> => {
    try {
      const result = await $`echo $THOP_SESSION`.quiet()
      return result.text().trim() || "unknown"
    } catch {
      return "unknown"
    }
  }

  // Complete thop documentation embedded in the plugin
  const THOP_INSTRUCTIONS = `# thop Guide for AI Agents

**thop** is a terminal session manager that enables AI agents to execute commands across local and remote SSH servers seamlessly.

## Detection

Check if running in thop:
\`\`\`bash
echo $THOP_SESSION
\`\`\`

Returns session name (\`local\`, \`prod\`, etc.) if in thop, empty otherwise.

## Core Concepts

- **Sessions**: Local or remote SSH contexts with independent state
- **Active Session**: Only one session is active at a time (all commands run there)
- **State Persistence**: Each session maintains its own working directory and environment

## Essential Commands

### Session Management
\`\`\`bash
/status              # List all sessions and their status
/connect <session>   # Connect to SSH session
/switch <session>    # Switch to different session
/local               # Switch to local session
/close <session>     # Close SSH connection
\`\`\`

### File Operations
\`\`\`bash
/copy <src> <dst>    # Copy between sessions (e.g., /copy local:file prod:/path/)
/read <path>         # Read file from current session
/write <path> <content>  # Write to file in current session
\`\`\`

### Environment & Jobs
\`\`\`bash
/env [KEY=VALUE]     # Show or set environment (persists in session)
/bg <command>        # Run command in background
/jobs                # List background jobs
/fg <job-id>         # Wait for background job
/kill <job-id>       # Kill background job
\`\`\`

### Other
\`\`\`bash
/shell <command>     # Interactive command with PTY (vim, top, etc.)
/add-session <name> <host>  # Add new SSH session
/auth <session>      # Provide password for SSH
/trust <session>     # Trust host key
\`\`\`

## Typical Workflow

### Example: Deploy to Production

\`\`\`bash
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
\`\`\`

### Example: Compare Configurations

\`\`\`bash
# Copy configs from different servers to local
/copy dev:/etc/app/config.yaml local:/tmp/dev-config
/copy prod:/etc/app/config.yaml local:/tmp/prod-config

# Switch to local and compare
/local
diff /tmp/dev-config /tmp/prod-config
\`\`\`

### Example: Multi-Server Operation

\`\`\`bash
# Update all servers
/connect server1
cd /app && git pull && sudo systemctl restart app

/switch server2
cd /app && git pull && sudo systemctl restart app

/switch server3
cd /app && git pull && sudo systemctl restart app

/local
\`\`\`

## Best Practices

1. **Always start with \`/status\`** - Verify available sessions
2. **Return to \`/local\` when done** - Clean workflow pattern
3. **Use \`/copy\` not \`scp\`** - Leverages existing connections
4. **Use \`/env\` not \`export\`** - Persists in session state
5. **Verify active session** - Check \`$THOP_SESSION\` before destructive commands
6. **Handle errors gracefully** - Check error codes and take appropriate action

## Error Handling

Common errors and solutions:

\`\`\`bash
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
\`\`\`

## Configuration

Sessions are defined in \`~/.config/thop/config.toml\`:

\`\`\`toml
[sessions.prod]
type = "ssh"
host = "prod.example.com"
user = "deploy"

[sessions.dev]
type = "ssh"
host = "dev.example.com"
user = "ubuntu"
\`\`\`

thop also reads \`~/.ssh/config\` for host definitions.

## Quick Reference

| Operation | Command |
|-----------|---------|
| List sessions | \`/status\` |
| Connect to server | \`/connect prod\` |
| Switch session | \`/switch dev\` |
| Return to local | \`/local\` |
| Copy file | \`/copy src:file dst:path\` |
| Read file | \`/read /path/file\` |
| Set env var | \`/env VAR=value\` |
| Background job | \`/bg command\` |
| List jobs | \`/jobs\` |
| Interactive shell | \`/shell vim file\` |

## State Management

Each session maintains:
- **Current working directory**: \`cd\` changes persist
- **Environment variables**: Set with \`/env\` (not \`export\`)
- **Connection status**: View with \`/status\`

State is preserved:
- When switching between sessions
- Across thop restarts (stored in \`~/.local/share/thop/state.json\`)

## Common Pitfalls

❌ **Don't**: Use \`export\` for environment
\`\`\`bash
export VAR=value  # Not tracked by thop
\`\`\`

✅ **Do**: Use \`/env\`
\`\`\`bash
/env VAR=value    # Persists in session
\`\`\`

---

❌ **Don't**: Use \`scp\` between sessions
\`\`\`bash
scp file user@host:/path  # Requires separate SSH
\`\`\`

✅ **Do**: Use \`/copy\`
\`\`\`bash
/copy local:file remote:/path  # Uses existing connection
\`\`\`

---

❌ **Don't**: Run interactive commands directly
\`\`\`bash
vim config.yaml   # Will hang
\`\`\`

✅ **Do**: Use \`/shell\`
\`\`\`bash
/shell vim config.yaml  # Proper PTY
\`\`\`

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
`

  // Check if we're in thop on startup
  const inThop = await isInThop()
  
  if (inThop) {
    const session = await getCurrentSession()
    await client.app.log({
      service: "thop-plugin",
      level: "info",
      message: `thop detected - active session: ${session}`,
    })
  }

  return {
    // Inject thop instructions into the system prompt
    "session.created": async (input) => {
      if (await isInThop()) {
        const session = await getCurrentSession()
        
        await client.app.log({
          service: "thop-plugin",
          level: "info",
          message: `Injecting thop instructions for session: ${session}`,
        })
        
        // Add thop context to the session
        // Note: This uses client SDK to add system context
        await client.message.create({
          role: "system",
          content: `# Running in thop

Current session: **${session}**

${THOP_INSTRUCTIONS}

Remember to use thop commands when working across local and remote systems.
`,
        })
      }
    },

    // Warn before executing potentially destructive commands on remote sessions
    "tool.execute.before": async (input, output) => {
      if (input.tool === "bash" && await isInThop()) {
        const session = await getCurrentSession()
        const command = output.args.command
        
        // Detect potentially destructive commands
        const destructivePatterns = [
          /\brm\s+-rf\b/,
          /\brm\s+.*\/\s*$/,
          /\bmv\s+.*\/\s*$/,
          /\bdd\s+if=/,
          /\bmkfs\./,
          /\bformat\b/,
        ]
        
        const isDestructive = destructivePatterns.some(pattern => pattern.test(command))
        
        if (isDestructive && session !== "local") {
          await client.app.log({
            service: "thop-plugin",
            level: "warn",
            message: `Destructive command detected on remote session: ${session}`,
            extra: { command, session },
          })
          
          // Log warning but don't block (let user handle it)
          console.warn(`⚠️  Executing destructive command on remote session: ${session}`)
        }
      }
    },

    // Log session switches
    "tool.execute.after": async (input, output) => {
      if (input.tool === "bash" && output.result) {
        const command = input.args?.command || ""
        
        // Detect thop session commands
        if (command.startsWith("/connect") || command.startsWith("/switch") || command === "/local") {
          const newSession = await getCurrentSession()
          
          await client.app.log({
            service: "thop-plugin",
            level: "info",
            message: `Session switched to: ${newSession}`,
            extra: { command, session: newSession },
          })
          
          console.log(`📡 Now in session: ${newSession}`)
        }
      }
    },
  }
}
