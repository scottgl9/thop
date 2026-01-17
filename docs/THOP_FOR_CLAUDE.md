# thop Guide for Claude

You may be running inside **thop**, a terminal session manager that lets you seamlessly work across local and remote SSH servers.

## Am I in thop?

```bash
echo $THOP_SESSION
```

If this outputs a session name (like `local`, `prod`, `dev`), you're in thop.

## What You Can Do

- Execute commands on local machine
- Connect to remote SSH servers
- Switch between sessions instantly
- Copy files between local and remote
- Each session keeps its own directory and environment

## Core Commands

### See Available Sessions
```bash
/status
```

Shows all sessions, which is active (marked with `*`), and their connection status.

### Connect & Switch
```bash
/connect prod     # Connect to prod server
/switch dev       # Switch to dev server
/local            # Switch to local
```

### Copy Files
```bash
/copy local:file.txt prod:/app/           # Local to remote
/copy prod:/etc/nginx/nginx.conf local:/  # Remote to local
```

### Other Useful Commands
```bash
/read /path/to/file        # Read file from current session
/env VAR=value             # Set environment variable (persists)
/bg npm run build          # Run command in background
/jobs                      # List background jobs
/shell vim config.yaml     # Interactive command (vim, top, etc.)
```

## Typical Workflow Example

User asks: *"Deploy the latest code to production"*

```bash
# Check what's available
/status

# Connect to production
/connect prod

# Deploy
cd /var/www/app
git pull origin main
npm install
npm run build
sudo systemctl restart app

# Verify
curl http://localhost:8080/health

# Return to local
/local
```

## Quick Reference

| Task | Command |
|------|---------|
| See sessions | `/status` |
| Connect to server | `/connect prod` |
| Switch session | `/switch dev` |
| Back to local | `/local` |
| Copy file | `/copy local:file remote:/path/` |
| Set env var | `/env NODE_ENV=prod` |
| Background job | `/bg long-command` |
| Interactive cmd | `/shell vim file` |

## Best Practices

1. **Always check `/status` first** - Know what's available
2. **Return to `/local` when done** - Clean workflow
3. **Use `/copy` not `scp`** - Simpler and uses existing connection
4. **Use `/env` not `export`** - Persists in session state
5. **Verify before destructive commands** - Check `$THOP_SESSION` before `rm -rf`
6. **Tell the user what you're doing** - Announce session switches

## Error Handling

If connection fails:
```bash
/connect prod
# Error: AUTH_KEY_FAILED → User needs to run: /auth prod

/connect newserver
# Error: HOST_KEY_UNKNOWN → User needs to run: /trust newserver
```

## Common Patterns

### Debug production issue
```bash
/status
/connect prod
tail -100 /var/log/app/error.log
# ... investigate ...
/local
```

### Compare config between servers
```bash
/copy dev:/etc/app/config.yaml local:/tmp/dev-config
/copy prod:/etc/app/config.yaml local:/tmp/prod-config
/local
diff /tmp/dev-config /tmp/prod-config
```

### Deploy to multiple servers
```bash
/connect staging
cd /app && git pull && npm run build && sudo systemctl restart app
/switch prod
cd /app && git pull && npm run build && sudo systemctl restart app
/local
```

---

**Remember**: thop keeps track of directories and environment per session. When you `/switch`, you're instantly in that server's context. This is your superpower for multi-server workflows.
