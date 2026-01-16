use std::io::{self, BufRead, Write};

use crate::error::{Result, SessionError, ThopError};
use crate::session::format_prompt;
use super::{print_slash_help, App};

/// Run interactive mode
pub fn run_interactive(app: &mut App) -> Result<()> {
    let stdin = io::stdin();
    let mut handle = stdin.lock();

    if !app.args.quiet {
        println!("thop - Terminal Hopper for Agents");
        println!("Type /help for available commands");
        println!();
    }

    loop {
        // Print prompt
        let session_name = app.sessions.get_active_session_name();
        let prompt = format_prompt(session_name);
        print!("{}", prompt);
        io::stdout().flush()?;

        // Read input
        let mut input = String::new();
        if handle.read_line(&mut input)? == 0 {
            // EOF (Ctrl+D)
            println!();
            return Ok(());
        }

        let input = input.trim();
        if input.is_empty() {
            continue;
        }

        // Check for slash commands
        if input.starts_with('/') {
            if let Err(e) = handle_slash_command(app, input) {
                app.output_error(&e);
            }
            continue;
        }

        // Execute command
        match app.sessions.execute(input) {
            Ok(result) => {
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
            }
            Err(e) => {
                app.output_error(&e);
            }
        }
    }
}

/// Handle slash commands
fn handle_slash_command(app: &mut App, input: &str) -> Result<()> {
    let parts: Vec<&str> = input.split_whitespace().collect();
    if parts.is_empty() {
        return Ok(());
    }

    let cmd = parts[0].to_lowercase();
    let args = &parts[1..];

    match cmd.as_str() {
        "/help" | "/h" | "/?" => {
            print_slash_help();
            Ok(())
        }

        "/status" | "/s" | "/sessions" | "/list" => {
            app.print_status()
        }

        "/connect" | "/c" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /connect <session>".to_string()));
            }
            cmd_connect(app, args[0])
        }

        "/switch" | "/sw" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /switch <session>".to_string()));
            }
            cmd_switch(app, args[0])
        }

        "/local" | "/l" => {
            cmd_switch(app, "local")
        }

        "/close" | "/disconnect" | "/d" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /close <session>".to_string()));
            }
            cmd_close(app, args[0])
        }

        "/exit" | "/quit" | "/q" => {
            println!("Goodbye!");
            std::process::exit(0);
        }

        _ => {
            Err(ThopError::Other(format!(
                "unknown command: {} (use /help for available commands)",
                cmd
            )))
        }
    }
}

/// Handle /connect command
fn cmd_connect(app: &mut App, name: &str) -> Result<()> {
    if !app.sessions.has_session(name) {
        return Err(SessionError::session_not_found(name).into());
    }

    let session = app.sessions.get_session(name).unwrap();

    if session.session_type() == "local" {
        println!("Session '{}' is local, no connection needed", name);
        return Ok(());
    }

    if session.is_connected() {
        println!("Session '{}' is already connected", name);
        return Ok(());
    }

    println!("Connecting to {}...", name);
    app.sessions.connect(name)?;
    println!("Connected to {}", name);

    Ok(())
}

/// Handle /switch command
fn cmd_switch(app: &mut App, name: &str) -> Result<()> {
    if !app.sessions.has_session(name) {
        return Err(SessionError::session_not_found(name).into());
    }

    let session = app.sessions.get_session(name).unwrap();

    // For SSH sessions, connect if not connected
    if session.session_type() == "ssh" && !session.is_connected() {
        println!("Connecting to {}...", name);
        app.sessions.connect(name)?;
        println!("Connected to {}", name);
    }

    app.sessions.set_active_session(name)?;

    if !app.args.quiet {
        println!("Switched to {}", name);
    }

    Ok(())
}

/// Handle /close command
fn cmd_close(app: &mut App, name: &str) -> Result<()> {
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

#[cfg(test)]
mod tests {
    use super::*;

    // Note: Interactive mode tests are more difficult to unit test
    // due to stdin/stdout interaction. These would typically be
    // integration tests instead.
}
