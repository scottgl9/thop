mod interactive;
mod proxy;

use std::collections::HashMap;
use std::sync::{Arc, Mutex, RwLock};
use std::time::Instant;

use clap::Parser;
use serde_json;

use crate::config::Config;
use crate::error::{Result, ThopError};
use crate::logger::{self, LogLevel, Logger};
use crate::session::Manager as SessionManager;
use crate::state::Manager as StateManager;

pub use interactive::run_interactive;
pub use proxy::run_proxy;

/// Background job state
#[derive(Debug, Clone)]
pub struct BackgroundJob {
    pub id: usize,
    pub command: String,
    pub session: String,
    pub start_time: Instant,
    pub end_time: Option<Instant>,
    pub status: String, // "running", "completed", "failed"
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
}

impl BackgroundJob {
    pub fn new(id: usize, command: String, session: String) -> Self {
        Self {
            id,
            command,
            session,
            start_time: Instant::now(),
            end_time: None,
            status: "running".to_string(),
            exit_code: 0,
            stdout: String::new(),
            stderr: String::new(),
        }
    }
}

/// thop - Terminal Hopper for Agents
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Args {
    /// Run in proxy mode (for AI agents)
    #[arg(long)]
    pub proxy: bool,

    /// Run as MCP (Model Context Protocol) server
    #[arg(long)]
    pub mcp: bool,

    /// Execute command and exit
    #[arg(short = 'c', value_name = "COMMAND")]
    pub command: Option<String>,

    /// Show status and exit
    #[arg(long)]
    pub status: bool,

    /// Path to config file
    #[arg(long, short = 'C')]
    pub config: Option<String>,

    /// Output in JSON format
    #[arg(long)]
    pub json: bool,

    /// Generate shell completions
    #[arg(long, value_name = "SHELL")]
    pub completions: Option<String>,

    /// Verbose output
    #[arg(long, short)]
    pub verbose: bool,

    /// Quiet output
    #[arg(long, short)]
    pub quiet: bool,
}

/// Main application
pub struct App {
    pub version: String,
    pub args: Args,
    pub config: Config,
    pub state: StateManager,
    pub sessions: SessionManager,
    /// Background jobs
    pub bg_jobs: Arc<RwLock<HashMap<usize, BackgroundJob>>>,
    /// Next job ID
    pub next_job_id: Arc<Mutex<usize>>,
}

impl App {
    /// Create a new App instance
    pub fn new(version: impl Into<String>) -> Result<Self> {
        let args = Args::parse();

        // Load configuration
        let config = Config::load(args.config.as_deref())?;

        // Initialize logger
        let log_level = if args.quiet {
            LogLevel::Off
        } else if args.verbose {
            LogLevel::Debug
        } else {
            LogLevel::from_str(&config.settings.log_level)
        };

        // Only enable file logging in verbose mode
        let log_file = if args.verbose {
            Some(Logger::default_log_path())
        } else {
            None
        };

        Logger::init(log_level, log_file);
        logger::debug("Logger initialized");

        // Initialize state manager
        let state = StateManager::new(&config.settings.state_file);
        if let Err(e) = state.load() {
            logger::warn(&format!("Failed to load state: {}", e));
        }

        // Initialize session manager
        let sessions = SessionManager::new(&config, Some(StateManager::new(&config.settings.state_file)));
        logger::debug(&format!("Loaded {} sessions", sessions.session_names().len()));

        Ok(Self {
            version: version.into(),
            args,
            config,
            state,
            sessions,
            bg_jobs: Arc::new(RwLock::new(HashMap::new())),
            next_job_id: Arc::new(Mutex::new(1)),
        })
    }

    /// Run the application
    pub fn run(&mut self) -> Result<()> {
        // Handle special flags
        if self.args.status {
            return self.print_status();
        }

        // Handle shell completions
        if let Some(ref shell) = self.args.completions {
            return self.print_completions(shell);
        }

        // Handle single command execution
        if let Some(ref cmd) = self.args.command.clone() {
            return self.execute_command(cmd);
        }

        // Run in appropriate mode
        if self.args.mcp {
            self.run_mcp()
        } else if self.args.proxy {
            run_proxy(self)
        } else {
            run_interactive(self)
        }
    }

    /// Run as MCP server
    fn run_mcp(&mut self) -> Result<()> {
        use crate::mcp::Server as McpServer;
        use crate::state::Manager as StateManager;

        // Create a fresh config, state, and session manager for MCP
        let config = self.config.clone();
        let state = StateManager::new(&config.settings.state_file);
        let sessions = crate::session::Manager::new(&config, Some(StateManager::new(&config.settings.state_file)));

        let mut mcp_server = McpServer::new(config, sessions, state);
        mcp_server.run()
    }

    /// Execute a single command and exit
    fn execute_command(&mut self, cmd: &str) -> Result<()> {
        let result = self.sessions.execute(cmd)?;

        if !result.stdout.is_empty() {
            print!("{}", result.stdout);
        }
        if !result.stderr.is_empty() {
            eprint!("{}", result.stderr);
        }

        if result.exit_code != 0 {
            std::process::exit(result.exit_code);
        }

        Ok(())
    }

    /// Print shell completions
    fn print_completions(&self, shell: &str) -> Result<()> {
        match shell.to_lowercase().as_str() {
            "bash" => {
                println!("{}", generate_bash_completion());
            }
            "zsh" => {
                println!("{}", generate_zsh_completion());
            }
            "fish" => {
                println!("{}", generate_fish_completion());
            }
            _ => {
                return Err(ThopError::Other(format!(
                    "Unsupported shell: {}. Supported: bash, zsh, fish",
                    shell
                )));
            }
        }
        Ok(())
    }

    /// Print status of all sessions
    pub fn print_status(&self) -> Result<()> {
        let sessions = self.sessions.list_sessions();

        if self.args.json {
            let json = serde_json::to_string_pretty(&sessions)
                .map_err(|e| ThopError::Other(format!("Failed to serialize: {}", e)))?;
            println!("{}", json);
        } else {
            println!("Sessions:");
            for s in sessions {
                let status = if s.connected { "connected" } else { "disconnected" };
                let active = if s.active { " [active]" } else { "" };

                if s.session_type == "ssh" {
                    let host = s.host.as_deref().unwrap_or("unknown");
                    let user = s.user.as_deref().unwrap_or("unknown");
                    println!("  {:12} {}@{} ({}){} {}", s.name, user, host, status, active, s.cwd);
                } else {
                    println!("  {:12} local ({}){} {}", s.name, status, active, s.cwd);
                }
            }
        }

        Ok(())
    }

    /// Output an error in the appropriate format
    pub fn output_error(&self, err: &ThopError) {
        if self.args.json {
            match err {
                ThopError::Session(session_err) => {
                    if let Ok(json) = serde_json::to_string(session_err) {
                        eprintln!("{}", json);
                    }
                }
                _ => {
                    let json = serde_json::json!({
                        "error": true,
                        "message": err.to_string()
                    });
                    eprintln!("{}", json);
                }
            }
        } else {
            match err {
                ThopError::Session(session_err) => {
                    eprintln!("Error: {}", session_err.message);
                    if let Some(ref suggestion) = session_err.suggestion {
                        eprintln!("Suggestion: {}", suggestion);
                    }
                }
                _ => {
                    eprintln!("Error: {}", err);
                }
            }
        }
    }
}

/// Print help for slash commands
pub fn print_slash_help() {
    println!(
        r#"Available commands:
  /connect <session>  Connect to an SSH session
  /switch <session>   Switch to a session
  /local              Switch to local shell (alias for /switch local)
  /status             Show all sessions
  /close <session>    Close an SSH connection
  /auth <session>     Set password for SSH session
  /trust <session>    Trust host key for SSH session
  /copy <src> <dst>   Copy file between sessions (session:path format)
  /add-session <name> <host> [user]  Add new SSH session
  /read <path>        Read file contents from current session
  /write <path> <content>  Write content to file
  /env [KEY=VALUE]    Show or set environment variables
  /shell <command>    Run interactive command (vim, top, etc.)
  /bg <command>       Run command in background
  /jobs               List background jobs
  /fg <job_id>        Wait for job and show output
  /kill <job_id>      Kill a running background job
  /help               Show this help
  /exit               Exit thop

Shortcuts:
  /c    = /connect
  /sw   = /switch
  /l    = /local
  /s    = /status
  /d    = /close (disconnect)
  /cp   = /copy
  /cat  = /read
  /sh   = /shell
  /add  = /add-session
  /q    = /exit

Copy examples:
  /copy local:/path/file remote:/path/file    Upload to active SSH session
  /copy remote:/path/file local:/path/file    Download from active SSH session
  /copy server1:/path/file server2:/path/file Copy between two SSH sessions

Interactive commands:
  /shell vim file.txt            Edit file with vim
  /shell top                     Run interactive top
  /sh bash                       Start interactive bash shell

Background jobs:
  /bg sleep 60                   Run 'sleep 60' in background
  /jobs                          List all background jobs
  /fg 1                          Wait for job 1 and show output
  /kill 1                        Kill running job 1"#
    );
}

/// Print CLI help
pub fn print_help() {
    println!(
        r#"thop - Terminal Hopper for Agents

USAGE:
    thop [OPTIONS]              Start interactive mode
    thop --proxy                Start proxy mode (for AI agents)
    thop --mcp                  Start MCP server mode (for AI agents)
    thop -c "command"           Execute command and exit
    thop --status               Show status and exit

OPTIONS:
    --proxy           Run in proxy mode (SHELL compatible)
    --mcp             Run as MCP (Model Context Protocol) server
    -c <command>      Execute command and exit with its exit code
    --status          Show all sessions and exit
    -C, --config <path> Use alternate config file
    --json            Output in JSON format
    --completions <s> Generate shell completions (bash, zsh, fish)
    -v, --verbose     Increase logging verbosity
    -q, --quiet       Suppress non-error output
    -h, --help        Print help information
    -V, --version     Print version

INTERACTIVE MODE COMMANDS:
    /connect <session>  Establish SSH connection
    /switch <session>   Change active context
    /local              Switch to local shell
    /status             Show all sessions
    /close <session>    Close SSH connection
    /env [KEY=VALUE]    Show or set environment variables
    /help               Show commands

EXAMPLES:
    # Start interactive mode
    thop

    # Execute single command
    thop -c "ls -la"

    # Use as shell for AI agent
    SHELL="thop --proxy" claude

    # Run as MCP server
    thop --mcp

    # Check status
    thop --status"#
    );
}

/// Generate bash completion script
fn generate_bash_completion() -> &'static str {
    r#"# Bash completion for thop

_thop() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main options
    opts="--proxy --mcp --status --config --json -v --verbose -q --quiet -h --help -V --version -c --completions"

    # Handle specific options
    case "${prev}" in
        --config|-C)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        -c)
            # No completion for command argument
            return 0
            ;;
        --completions)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            return 0
            ;;
    esac

    # Complete options
    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
}

complete -F _thop thop"#
}

/// Generate zsh completion script
fn generate_zsh_completion() -> &'static str {
    r#"#compdef thop

# Zsh completion for thop

_thop() {
    local -a opts

    opts=(
        '--proxy[Run in proxy mode for AI agents]'
        '--mcp[Run as MCP (Model Context Protocol) server]'
        '-c[Execute command and exit]:command:'
        '--status[Show status and exit]'
        '-C[Use alternate config file]:config file:_files'
        '--config[Use alternate config file]:config file:_files'
        '--json[Output in JSON format]'
        '--completions[Generate shell completions]:shell:(bash zsh fish)'
        '-v[Verbose output]'
        '--verbose[Verbose output]'
        '-q[Quiet output]'
        '--quiet[Quiet output]'
        '-h[Show help]'
        '--help[Show help]'
        '-V[Show version]'
        '--version[Show version]'
    )

    _arguments -s $opts
}

_thop "$@""#
}

/// Generate fish completion script
fn generate_fish_completion() -> &'static str {
    r#"# Fish completion for thop

# Main options
complete -c thop -l proxy -d 'Run in proxy mode for AI agents'
complete -c thop -l mcp -d 'Run as MCP (Model Context Protocol) server'
complete -c thop -s c -r -d 'Execute command and exit'
complete -c thop -l status -d 'Show status and exit'
complete -c thop -s C -l config -r -F -d 'Use alternate config file'
complete -c thop -l json -d 'Output in JSON format'
complete -c thop -l completions -r -a 'bash zsh fish' -d 'Generate shell completions'
complete -c thop -s v -l verbose -d 'Verbose output'
complete -c thop -s q -l quiet -d 'Quiet output'
complete -c thop -s h -l help -d 'Show help'
complete -c thop -s V -l version -d 'Show version'"#
}
