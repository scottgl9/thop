use std::io::{self, BufRead, Write};

use crate::error::{Result, SessionError, ThopError};
use super::App;

/// Run proxy mode for AI agent integration
pub fn run_proxy(app: &mut App) -> Result<()> {
    let stdin = io::stdin();
    let handle = stdin.lock();

    for line in handle.lines() {
        let input = match line {
            Ok(input) => input,
            Err(_) => break, // EOF or error
        };

        // Strip CR if present (Windows line endings)
        let input = input.trim_end_matches('\r');

        if input.is_empty() {
            continue;
        }

        // Check for slash commands
        if input.starts_with('/') {
            if let Err(e) = handle_proxy_slash_command(app, input) {
                app.output_error(&e);
            }
            continue;
        }

        // Execute command on active session
        match app.sessions.execute(input) {
            Ok(result) => {
                // Output results
                if !result.stdout.is_empty() {
                    print!("{}", result.stdout);
                    if !result.stdout.ends_with('\n') {
                        println!();
                    }
                }

                if !result.stderr.is_empty() {
                    eprint!("{}", result.stderr);
                    if !result.stderr.ends_with('\n') {
                        eprintln!();
                    }
                }

                // Flush output
                io::stdout().flush().ok();
                io::stderr().flush().ok();

                // In verbose mode, show exit code for non-zero exits
                if result.exit_code != 0 && app.args.verbose {
                    eprintln!("[exit code: {}]", result.exit_code);
                }
            }
            Err(e) => {
                app.output_error(&e);
                // In proxy mode, continue even on error
            }
        }
    }

    Ok(())
}

/// Handle slash commands in proxy mode
fn handle_proxy_slash_command(app: &mut App, input: &str) -> Result<()> {
    let parts: Vec<&str> = input.split_whitespace().collect();
    if parts.is_empty() {
        return Ok(());
    }

    let cmd = parts[0].to_lowercase();
    let args = &parts[1..];

    match cmd.as_str() {
        "/status" | "/s" => {
            app.print_status()
        }

        "/connect" | "/c" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /connect <session>".to_string()));
            }
            let name = args[0];
            if !app.sessions.has_session(name) {
                return Err(SessionError::session_not_found(name).into());
            }
            println!("Connecting to {}...", name);
            app.sessions.connect(name)?;
            println!("Connected to {}", name);
            Ok(())
        }

        "/switch" | "/sw" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /switch <session>".to_string()));
            }
            let name = args[0];
            if !app.sessions.has_session(name) {
                return Err(SessionError::session_not_found(name).into());
            }

            // For SSH sessions, connect if not connected
            let session = app.sessions.get_session(name).unwrap();
            if session.session_type() == "ssh" && !session.is_connected() {
                println!("Connecting to {}...", name);
                app.sessions.connect(name)?;
                println!("Connected to {}", name);
            }

            app.sessions.set_active_session(name)?;
            println!("Switched to {}", name);
            Ok(())
        }

        "/local" | "/l" => {
            app.sessions.set_active_session("local")?;
            println!("Switched to local");
            Ok(())
        }

        "/close" | "/disconnect" | "/d" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /close <session>".to_string()));
            }
            let name = args[0];
            if !app.sessions.has_session(name) {
                return Err(SessionError::session_not_found(name).into());
            }

            let session = app.sessions.get_session(name).unwrap();
            if session.session_type() == "local" {
                println!("Cannot close local session");
                return Ok(());
            }

            if !session.is_connected() {
                println!("Session '{}' is not connected", name);
                return Ok(());
            }

            app.sessions.disconnect(name)?;
            println!("Disconnected from {}", name);

            // Switch to local if we closed the active session
            if app.sessions.get_active_session_name() == name {
                app.sessions.set_active_session("local")?;
                println!("Switched to local");
            }
            Ok(())
        }

        _ => {
            Err(ThopError::Other(format!(
                "unknown command: {} (supported: /connect, /switch, /local, /status, /close)",
                cmd
            )))
        }
    }
}

#[cfg(test)]
mod tests {
    // Proxy mode tests would typically be integration tests
    // due to stdin/stdout interaction
}
