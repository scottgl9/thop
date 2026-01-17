mod cli;
mod config;
mod error;
mod logger;
mod mcp;
mod session;
mod sshconfig;
mod state;

use std::process::ExitCode;

use cli::App;

const VERSION: &str = env!("CARGO_PKG_VERSION");

fn main() -> ExitCode {
    match App::new(VERSION) {
        Ok(mut app) => {
            if let Err(e) = app.run() {
                app.output_error(&e);
                ExitCode::from(1)
            } else {
                ExitCode::SUCCESS
            }
        }
        Err(e) => {
            eprintln!("Error: {}", e);
            ExitCode::from(1)
        }
    }
}
