mod interactive;
mod proxy;

use clap::Parser;
use serde_json;

use crate::config::Config;
use crate::error::{Result, SessionError, ThopError};
use crate::session::{format_prompt, Manager as SessionManager, SessionInfo};
use crate::state::Manager as StateManager;

pub use interactive::run_interactive;
pub use proxy::run_proxy;

/// thop - Terminal Hopper for Agents
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Args {
    /// Run in proxy mode (for AI agents)
    #[arg(long)]
    pub proxy: bool,

    /// Show status and exit
    #[arg(long)]
    pub status: bool,

    /// Path to config file
    #[arg(long, short)]
    pub config: Option<String>,

    /// Output in JSON format
    #[arg(long)]
    pub json: bool,

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
}

impl App {
    /// Create a new App instance
    pub fn new(version: impl Into<String>) -> Result<Self> {
        let args = Args::parse();

        // Load configuration
        let config = Config::load(args.config.as_deref())?;

        // Initialize state manager
        let state = StateManager::new(&config.settings.state_file);
        if let Err(e) = state.load() {
            if args.verbose {
                eprintln!("Warning: failed to load state: {}", e);
            }
        }

        // Initialize session manager
        let sessions = SessionManager::new(&config, Some(StateManager::new(&config.settings.state_file)));

        Ok(Self {
            version: version.into(),
            args,
            config,
            state,
            sessions,
        })
    }

    /// Run the application
    pub fn run(&mut self) -> Result<()> {
        // Handle special flags
        if self.args.status {
            return self.print_status();
        }

        // Run in appropriate mode
        if self.args.proxy {
            run_proxy(self)
        } else {
            run_interactive(self)
        }
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
  /help               Show this help
  /exit               Exit thop

Shortcuts:
  /c   = /connect
  /sw  = /switch
  /l   = /local
  /s   = /status
  /d   = /close (disconnect)
  /q   = /exit"#
    );
}

/// Print CLI help
pub fn print_help() {
    println!(
        r#"thop - Terminal Hopper for Agents

USAGE:
    thop [OPTIONS]              Start interactive mode
    thop --proxy                Start proxy mode (for AI agents)
    thop --status               Show status and exit

OPTIONS:
    --proxy         Run in proxy mode (SHELL compatible)
    --status        Show all sessions and exit
    --config <path> Use alternate config file
    --json          Output in JSON format
    -v, --verbose   Increase logging verbosity
    -q, --quiet     Suppress non-error output
    -h, --help      Print help information
    -V, --version   Print version

INTERACTIVE MODE COMMANDS:
    /connect <session>  Establish SSH connection
    /switch <session>   Change active context
    /local              Switch to local shell
    /status             Show all sessions
    /close <session>    Close SSH connection
    /help               Show commands

EXAMPLES:
    # Start interactive mode
    thop

    # Use as shell for AI agent
    SHELL="thop --proxy" claude

    # Check status
    thop --status"#
    );
}
