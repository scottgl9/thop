use std::io::{self, BufRead, Write};

use crate::error::Result;
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

#[cfg(test)]
mod tests {
    // Proxy mode tests would typically be integration tests
    // due to stdin/stdout interaction
}
