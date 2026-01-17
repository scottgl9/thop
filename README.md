# thop - Terminal Hopper for Agents

A lightweight CLI tool that enables AI agents to execute commands across local and remote (SSH) sessions with seamless context switching.

## Features

- **Multi-session support**: Manage multiple SSH connections alongside your local shell
- **SSH config integration**: Automatically reads `~/.ssh/config` for host aliases
- **Context switching**: Switch between sessions with simple slash commands
- **Proxy mode**: Use as a SHELL for AI agents like Claude Code
- **MCP server**: Built-in Model Context Protocol server for AI agent integration
- **State persistence**: Maintains working directory and environment across commands
- **Shell completions**: Tab completion for bash, zsh, and fish

## Installation

### From Source (Go)

```bash
cd thop-go
go build -o thop ./cmd/thop
sudo mv thop /usr/local/bin/
```

### Shell Completions

```bash
# Bash (add to ~/.bashrc)
eval "$(thop --completions bash)"

# Zsh (add to ~/.zshrc)
eval "$(thop --completions zsh)"

# Fish
thop --completions fish > ~/.config/fish/completions/thop.fish
```

## Quick Start

### Interactive Mode

```bash
# Start interactive mode
thop

# You'll see a prompt like:
(local) $ ls -la
(local) $ /connect myserver
Connecting to myserver...
Connected to myserver
(myserver) $ pwd
/home/user
(myserver) $ /local
Switched to local
(local) $
```

### Proxy Mode (for AI Agents)

```bash
# Execute a single command
thop -c "ls -la"

# Use as SHELL for Claude Code
SHELL="thop --proxy" claude

# Read commands from stdin
echo "ls -la" | thop --proxy
```

### MCP Server Mode

```bash
# Start as MCP server
thop --mcp

# Configure in Claude Desktop
# See docs/MCP.md for full configuration
```

## Configuration

### Config File

Create `~/.config/thop/config.toml`:

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

[sessions.staging]
type = "ssh"
host = "staging"  # Uses ~/.ssh/config alias

[sessions.dev]
type = "ssh"
host = "dev.example.com"
user = "developer"
startup_commands = [
    "cd ~/project",
    "source venv/bin/activate"
]
```

### Password Authentication

For servers requiring password authentication, you can provide the password in several ways:

**Via environment variable:**

```toml
[sessions.legacy]
type = "ssh"
host = "legacy.example.com"
user = "admin"
password_env = "LEGACY_SSH_PASSWORD"  # Reads from $LEGACY_SSH_PASSWORD
```

**Via password file (must have 0600 permissions):**

```toml
[sessions.legacy]
type = "ssh"
host = "legacy.example.com"
user = "admin"
password_file = "~/.ssh/legacy_password"  # File must be chmod 0600
```

**Interactively via /auth command:**

```bash
(local) $ /auth legacy
Password:
Password set for legacy
(local) $ /connect legacy
```

### Startup Commands

You can configure commands to run automatically when connecting to a session:

```toml
[sessions.myserver]
type = "ssh"
host = "myserver"
startup_commands = [
    "cd ~/workspace",
    "source ~/.bashrc",
    "export PATH=$PATH:/custom/bin"
]
```

### SSH Config Integration

thop automatically reads `~/.ssh/config` to resolve host aliases:

```
# ~/.ssh/config
Host myserver
    HostName actual.server.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/mykey
```

Then in your thop config:

```toml
[sessions.myserver]
type = "ssh"
host = "myserver"  # Will resolve from SSH config
```

### Jump Host / Bastion Support

thop supports connecting through jump hosts (bastion servers). You can configure this in two ways:

**Via thop config:**

```toml
[sessions.internal]
type = "ssh"
host = "internal.server.com"
user = "deploy"
jump_host = "bastion.example.com"  # Simple hostname
# Or with full details:
# jump_host = "jumpuser@bastion.example.com:2222"
```

**Via SSH config (ProxyJump):**

```
# ~/.ssh/config
Host internal
    HostName internal.server.com
    User deploy
    ProxyJump bastion.example.com

Host bastion.example.com
    User jumpuser
    IdentityFile ~/.ssh/bastion_key
```

Then in thop config:

```toml
[sessions.internal]
type = "ssh"
host = "internal"  # Will use ProxyJump from SSH config
```

The jump host connection is established first, then the target connection is made through the jump host tunnel.

### SSH Agent Forwarding

thop supports SSH agent forwarding, which allows the remote server to use your local SSH keys for authentication (useful for git over SSH, chained SSH connections, etc.).

**Via thop config:**

```toml
[sessions.dev]
type = "ssh"
host = "dev.example.com"
user = "developer"
agent_forwarding = true
```

**Via SSH config (ForwardAgent):**

```
# ~/.ssh/config
Host dev
    HostName dev.example.com
    User developer
    ForwardAgent yes
```

Then in thop config:

```toml
[sessions.dev]
type = "ssh"
host = "dev"  # Will use ForwardAgent from SSH config
```

With agent forwarding enabled, you can use git over SSH, SSH to other servers, or any other operation that requires your SSH keys on the remote server.

### Environment Variables

- `THOP_CONFIG`: Path to config file (default: `~/.config/thop/config.toml`)
- `THOP_STATE_FILE`: Path to state file (default: `~/.local/share/thop/state.json`)
- `THOP_LOG_LEVEL`: Log level (debug, info, warn, error)
- `THOP_DEFAULT_SESSION`: Default session name

## Commands

### Interactive Mode Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `/connect <session>` | `/c` | Connect to an SSH session |
| `/switch <session>` | `/sw` | Switch to a session |
| `/local` | `/l` | Switch to local shell |
| `/status` | `/s` | Show all sessions |
| `/close <session>` | `/d` | Disconnect from SSH session |
| `/auth <session>` | | Set password for SSH session |
| `/trust <session>` | | Trust host key for SSH session |
| `/copy <src> <dst>` | `/cp` | Copy file between sessions |
| `/add-session <name> <host>` | `/add` | Add new SSH session to config |
| `/read <path>` | `/cat` | Read file contents from current session |
| `/write <path> <content>` | | Write content to file on current session |
| `/env [KEY=VALUE]` | | Show or set environment variables |
| `/shell <command>` | `/sh` | Run interactive command with PTY (vim, top) |
| `/bg <command>` | | Run command in background |
| `/jobs` | | List background jobs |
| `/fg <job_id>` | | Wait for job and show output |
| `/kill <job_id>` | | Kill a running background job |
| `/help` | `/h` | Show help |
| `/exit` | `/q` | Exit thop |

### CLI Flags

| Flag | Description |
|------|-------------|
| `--proxy` | Run in proxy mode (for AI agents) |
| `--mcp` | Run as MCP server (Model Context Protocol) |
| `-c <cmd>` | Execute command and exit |
| `--status` | Show status and exit |
| `--config <path>` | Use alternate config file |
| `--json` | Output in JSON format |
| `--completions <shell>` | Generate shell completions (bash, zsh, fish) |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Quiet output |
| `-h, --help` | Show help |
| `-V, --version` | Show version |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |
| 2 | Authentication failed |
| 3 | Host key verification failed |

## Integration with Claude Code

To use thop as the shell for Claude Code:

```bash
SHELL="thop --proxy" claude
```

This allows Claude to:
- Execute commands on your local machine
- Connect to and execute commands on remote servers
- Switch between sessions seamlessly
- Maintain working directory state across commands

### Example Workflow

```bash
# Start Claude with thop as shell
SHELL="thop --proxy" claude

# Claude can now:
# 1. Run local commands
ls -la

# 2. Connect to remote server
/connect prod

# 3. Run commands on prod
pwd
cat /var/log/app.log

# 4. Switch back to local
/local
```

## State Persistence

thop maintains state in `~/.local/share/thop/state.json`:

- Active session name
- Per-session working directory
- Per-session environment variables
- Connection status

State is preserved across thop restarts and uses file locking for safe concurrent access.

## Troubleshooting

### SSH Connection Issues

1. **Authentication failed**: Ensure your SSH key is loaded in ssh-agent or specified in config
   ```bash
   ssh-add ~/.ssh/mykey
   ```

2. **Host key verification failed**: The host key is not in `~/.ssh/known_hosts`
   ```bash
   ssh-keyscan hostname >> ~/.ssh/known_hosts
   ```

3. **Connection refused**: Check the host and port are correct
   ```bash
   thop --status  # Shows configured sessions
   ```

### Config Issues

1. Check config syntax:
   ```bash
   cat ~/.config/thop/config.toml
   ```

2. Use verbose mode:
   ```bash
   thop -v
   ```

## Development

### Building

```bash
cd thop-go
go build ./cmd/thop
```

### Testing

```bash
cd thop-go
go test ./...
```

### Project Structure

```
thop-go/
├── cmd/thop/          # Main entry point
├── internal/
│   ├── cli/           # CLI handling (interactive, proxy, completions)
│   ├── config/        # Configuration parsing
│   ├── mcp/           # MCP server implementation
│   ├── session/       # Session management (local, SSH)
│   ├── sshconfig/     # SSH config parsing
│   └── state/         # State persistence
└── go.mod
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## Author

Scott Glover <scottgl@gmail.com>
