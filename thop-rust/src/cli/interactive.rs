use std::fs;
use std::io::{self, BufRead, Write};
use std::path::PathBuf;

use crate::error::{Result, SessionError, ThopError};
use crate::session::format_prompt;
use super::{print_slash_help, App};

/// Read password from terminal (with echo disabled if possible)
fn read_password(prompt: &str) -> io::Result<String> {
    print!("{}", prompt);
    io::stdout().flush()?;

    // Try to read without echo using rpassword-like behavior
    // For simplicity, we'll just read a line (a proper impl would disable echo)
    let mut password = String::new();
    io::stdin().read_line(&mut password)?;
    println!(); // Add newline after password entry
    Ok(password.trim().to_string())
}

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

        "/env" => {
            cmd_env(app, args)
        }

        "/auth" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /auth <session>".to_string()));
            }
            cmd_auth(app, args[0])
        }

        "/read" | "/cat" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /read <path>".to_string()));
            }
            cmd_read(app, args[0])
        }

        "/write" => {
            if args.len() < 2 {
                return Err(ThopError::Other("usage: /write <path> <content>".to_string()));
            }
            let path = args[0];
            let content = args[1..].join(" ");
            cmd_write(app, path, &content)
        }

        "/trust" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /trust <session>".to_string()));
            }
            cmd_trust(app, args[0])
        }

        "/add-session" | "/add" => {
            if args.len() < 2 {
                return Err(ThopError::Other("usage: /add-session <name> <host> [user]".to_string()));
            }
            let name = args[0];
            let host = args[1];
            let user = args.get(2).copied();
            cmd_add_session(app, name, host, user)
        }

        "/bg" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /bg <command>".to_string()));
            }
            cmd_bg(app, &args.join(" "))
        }

        "/jobs" => {
            cmd_jobs(app)
        }

        "/fg" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /fg <job_id>".to_string()));
            }
            cmd_fg(app, args[0])
        }

        "/kill" => {
            if args.is_empty() {
                return Err(ThopError::Other("usage: /kill <job_id>".to_string()));
            }
            cmd_kill_job(app, args[0])
        }

        "/copy" | "/cp" => {
            if args.len() < 2 {
                return Err(ThopError::Other(
                    "usage: /copy <source> <destination>\n  Examples:\n    /copy local:/path/to/file remote:/path/to/file\n    /copy remote:/path/to/file local:/path/to/file".to_string()
                ));
            }
            cmd_copy(app, args[0], args[1])
        }

        "/shell" | "/sh" => {
            if args.is_empty() {
                return Err(ThopError::Other(
                    "usage: /shell <command>\n  Runs command with interactive support (vim, top, etc.)".to_string()
                ));
            }
            cmd_shell(app, &args.join(" "))
        }

        _ => {
            Err(ThopError::Other(format!(
                "unknown command: {} (use /help for available commands)",
                cmd
            )))
        }
    }
}

/// Handle /env command - show or set environment variables
fn cmd_env(app: &mut App, args: &[&str]) -> Result<()> {
    if args.is_empty() {
        // Show all environment variables for active session
        let session_name = app.sessions.get_active_session_name();
        if let Some(session) = app.sessions.get_session(session_name) {
            let env = session.get_env();
            if env.is_empty() {
                println!("No environment variables set for session '{}'", session_name);
            } else {
                println!("Environment variables for '{}':", session_name);
                let mut keys: Vec<_> = env.keys().collect();
                keys.sort();
                for key in keys {
                    println!("  {}={}", key, env.get(key).unwrap());
                }
            }
        }
    } else {
        // Set environment variable
        let arg = args.join(" ");
        if let Some(pos) = arg.find('=') {
            let key = &arg[..pos];
            let value = &arg[pos + 1..];
            let session_name = app.sessions.get_active_session_name().to_string();
            if let Some(session) = app.sessions.get_session_mut(&session_name) {
                session.set_env(key, value);
                println!("Set {}={}", key, value);
            }
        } else {
            // Show specific variable
            let session_name = app.sessions.get_active_session_name();
            if let Some(session) = app.sessions.get_session(session_name) {
                let env = session.get_env();
                if let Some(value) = env.get(args[0]) {
                    println!("{}={}", args[0], value);
                } else {
                    println!("{} is not set", args[0]);
                }
            }
        }
    }
    Ok(())
}

/// Handle /auth command - set password for SSH session
fn cmd_auth(app: &mut App, name: &str) -> Result<()> {
    if !app.sessions.has_session(name) {
        return Err(SessionError::session_not_found(name).into());
    }

    let session = app.sessions.get_session(name).unwrap();
    if session.session_type() == "local" {
        return Err(ThopError::Other("Cannot set password for local session".to_string()));
    }

    let password = read_password("Password: ")
        .map_err(|e| ThopError::Other(format!("Failed to read password: {}", e)))?;

    if password.is_empty() {
        return Err(ThopError::Other("Password cannot be empty".to_string()));
    }

    app.sessions.set_session_password(name, &password)?;
    println!("Password set for {}", name);

    Ok(())
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

/// Handle /read command - read file contents
fn cmd_read(app: &mut App, path: &str) -> Result<()> {
    let session_name = app.sessions.get_active_session_name();
    let session = app.sessions.get_session(session_name).unwrap();

    if session.session_type() == "local" {
        // Local file read
        let expanded_path = expand_path(path);
        match fs::read_to_string(&expanded_path) {
            Ok(content) => {
                print!("{}", content);
                if !content.ends_with('\n') {
                    println!();
                }
            }
            Err(e) => {
                return Err(ThopError::Other(format!("Failed to read file: {}", e)));
            }
        }
    } else {
        // Remote file read via cat
        let result = app.sessions.execute(&format!("cat {}", shell_escape(path)))?;
        if result.exit_code != 0 {
            return Err(ThopError::Other(format!(
                "Failed to read file: {}",
                result.stderr.trim()
            )));
        }
        print!("{}", result.stdout);
        if !result.stdout.ends_with('\n') {
            println!();
        }
    }

    Ok(())
}

/// Handle /write command - write content to file
fn cmd_write(app: &mut App, path: &str, content: &str) -> Result<()> {
    let session_name = app.sessions.get_active_session_name();
    let session = app.sessions.get_session(session_name).unwrap();

    if session.session_type() == "local" {
        // Local file write
        let expanded_path = expand_path(path);
        match fs::write(&expanded_path, content) {
            Ok(_) => {
                println!("Written {} bytes to {}", content.len(), path);
            }
            Err(e) => {
                return Err(ThopError::Other(format!("Failed to write file: {}", e)));
            }
        }
    } else {
        // Remote file write via cat with heredoc
        let cmd = format!(
            "cat > {} << 'THOP_EOF'\n{}\nTHOP_EOF",
            shell_escape(path),
            content
        );
        let result = app.sessions.execute(&cmd)?;
        if result.exit_code != 0 {
            return Err(ThopError::Other(format!(
                "Failed to write file: {}",
                result.stderr.trim()
            )));
        }
        println!("Written {} bytes to {}", content.len(), path);
    }

    Ok(())
}

/// Handle /trust command - trust host key for SSH session
fn cmd_trust(app: &mut App, name: &str) -> Result<()> {
    if !app.sessions.has_session(name) {
        return Err(SessionError::session_not_found(name).into());
    }

    let session = app.sessions.get_session(name).unwrap();
    if session.session_type() == "local" {
        return Err(ThopError::Other("Cannot trust host key for local session".to_string()));
    }

    // Get the host from the session
    // For now, we'll use ssh-keyscan to fetch and add the key
    // This requires knowing the host - we'd need to store it in the session
    println!("To trust the host key for '{}', run:", name);
    println!("  ssh-keyscan <hostname> >> ~/.ssh/known_hosts");
    println!();
    println!("Or connect with ssh once to manually verify and add the key:");
    println!("  ssh <hostname>");

    Ok(())
}

/// Handle /add-session command - add new SSH session
fn cmd_add_session(app: &mut App, name: &str, host: &str, user: Option<&str>) -> Result<()> {
    if app.sessions.has_session(name) {
        return Err(ThopError::Other(format!("Session '{}' already exists", name)));
    }

    let user = user.map(|s| s.to_string()).unwrap_or_else(|| {
        std::env::var("USER").unwrap_or_else(|_| "root".to_string())
    });

    // Add session to the manager
    app.sessions.add_ssh_session(name, host, &user, 22)?;

    println!("Added SSH session '{}'", name);
    println!("  Host: {}", host);
    println!("  User: {}", user);
    println!("  Port: 22");
    println!();
    println!("Use '/connect {}' to connect", name);

    Ok(())
}

/// Handle /bg command - run command in background
fn cmd_bg(app: &mut App, command: &str) -> Result<()> {
    use std::thread;
    use super::BackgroundJob;

    let session_name = app.sessions.get_active_session_name().to_string();

    // Get next job ID
    let job_id = {
        let mut id = app.next_job_id.lock().unwrap();
        let current = *id;
        *id += 1;
        current
    };

    // Create the job
    let job = BackgroundJob::new(job_id, command.to_string(), session_name.clone());

    // Add to jobs map
    {
        let mut jobs = app.bg_jobs.write().unwrap();
        jobs.insert(job_id, job);
    }

    println!("[{}] Started in background: {}", job_id, command);

    // Clone what we need for the thread
    let bg_jobs = app.bg_jobs.clone();
    let cmd = command.to_string();

    // Execute in a separate thread
    // Note: This spawns a new session manager which isn't ideal but works for simple cases
    let config = app.config.clone();
    thread::spawn(move || {
        use crate::session::Manager as SessionManager;
        use crate::state::Manager as StateManager;

        let state = StateManager::new(&config.settings.state_file);
        let mut sessions = SessionManager::new(&config, Some(state));

        // Try to set to same session (local should work)
        let _ = sessions.set_active_session(&session_name);

        let result = sessions.execute(&cmd);

        // Update job with result
        let mut jobs = bg_jobs.write().unwrap();
        if let Some(job) = jobs.get_mut(&job_id) {
            job.end_time = Some(std::time::Instant::now());

            match result {
                Ok(exec_result) => {
                    job.status = "completed".to_string();
                    job.stdout = exec_result.stdout;
                    job.stderr = exec_result.stderr;
                    job.exit_code = exec_result.exit_code;
                }
                Err(e) => {
                    job.status = "failed".to_string();
                    job.stderr = e.to_string();
                    job.exit_code = 1;
                }
            }

            let duration = job.end_time.unwrap().duration_since(job.start_time);
            if job.status == "completed" {
                println!("\n[{}] Done ({:.1?}): {}", job_id, duration, cmd);
            } else {
                println!("\n[{}] Failed ({:.1?}): {}", job_id, duration, cmd);
            }
        }
    });

    Ok(())
}

/// Handle /jobs command - list background jobs
fn cmd_jobs(app: &mut App) -> Result<()> {
    let jobs = app.bg_jobs.read().unwrap();

    if jobs.is_empty() {
        println!("No background jobs");
        return Ok(());
    }

    println!("Background jobs:");
    for job in jobs.values() {
        let status = match job.status.as_str() {
            "running" => {
                let duration = job.start_time.elapsed();
                format!("running ({:.0?})", duration)
            }
            "completed" => {
                let duration = job.end_time.map(|e| e.duration_since(job.start_time));
                format!("completed (exit {}, {:.1?})", job.exit_code, duration.unwrap_or_default())
            }
            "failed" => {
                let duration = job.end_time.map(|e| e.duration_since(job.start_time));
                format!("failed ({:.1?})", duration.unwrap_or_default())
            }
            _ => job.status.clone(),
        };

        let cmd_display = if job.command.len() > 40 {
            format!("{}...", &job.command[..37])
        } else {
            job.command.clone()
        };

        println!("  [{}] {:12} {}  {}", job.id, job.session, status, cmd_display);
    }

    Ok(())
}

/// Handle /fg command - wait for job and display output
fn cmd_fg(app: &mut App, job_id_str: &str) -> Result<()> {
    use std::thread;
    use std::time::Duration;

    let job_id: usize = job_id_str.parse()
        .map_err(|_| ThopError::Other(format!("Invalid job ID: {}", job_id_str)))?;

    // Check if job exists
    {
        let jobs = app.bg_jobs.read().unwrap();
        if !jobs.contains_key(&job_id) {
            return Err(ThopError::Other(format!("Job {} not found", job_id)));
        }
    }

    // Wait for job if still running
    loop {
        {
            let jobs = app.bg_jobs.read().unwrap();
            if let Some(job) = jobs.get(&job_id) {
                if job.status != "running" {
                    break;
                }
            } else {
                return Err(ThopError::Other(format!("Job {} not found", job_id)));
            }
        }
        println!("Waiting for job {}...", job_id);
        thread::sleep(Duration::from_millis(500));
    }

    // Display output
    let job = {
        let mut jobs = app.bg_jobs.write().unwrap();
        jobs.remove(&job_id)
    };

    if let Some(job) = job {
        println!("Job {} ({}):", job_id, job.status);
        if !job.stdout.is_empty() {
            print!("{}", job.stdout);
            if !job.stdout.ends_with('\n') {
                println!();
            }
        }
        if !job.stderr.is_empty() {
            eprint!("{}", job.stderr);
            if !job.stderr.ends_with('\n') {
                eprintln!();
            }
        }
    }

    Ok(())
}

/// Handle /kill command - kill a running background job
fn cmd_kill_job(app: &mut App, job_id_str: &str) -> Result<()> {
    let job_id: usize = job_id_str.parse()
        .map_err(|_| ThopError::Other(format!("Invalid job ID: {}", job_id_str)))?;

    let mut jobs = app.bg_jobs.write().unwrap();

    let job = jobs.get_mut(&job_id)
        .ok_or_else(|| ThopError::Other(format!("Job {} not found", job_id)))?;

    if job.status != "running" {
        return Err(ThopError::Other(format!("Job {} is not running (status: {})", job_id, job.status)));
    }

    // Mark as failed/killed
    job.status = "failed".to_string();
    job.end_time = Some(std::time::Instant::now());
    job.stderr = "killed by user".to_string();
    job.exit_code = 137; // SIGKILL exit code

    // Remove from job list
    jobs.remove(&job_id);

    println!("Job {} killed", job_id);

    Ok(())
}

/// Handle /copy command - copy files between sessions
fn cmd_copy(app: &mut App, src: &str, dst: &str) -> Result<()> {
    // Parse source and destination (format: session:path or just path for active session)
    let (src_session, src_path) = parse_file_spec(src);
    let (dst_session, dst_path) = parse_file_spec(dst);

    // Default to active session if not specified
    let active_session = app.sessions.get_active_session_name().to_string();
    let src_session = if src_session.is_empty() { active_session.clone() } else { src_session };
    let dst_session = if dst_session.is_empty() { active_session.clone() } else { dst_session };

    // Handle "remote" as alias for active SSH session
    let src_session = if src_session == "remote" {
        if active_session == "local" {
            return Err(ThopError::Other("no remote session active - use session name instead".to_string()));
        }
        active_session.clone()
    } else {
        src_session
    };

    let dst_session = if dst_session == "remote" {
        if active_session == "local" {
            return Err(ThopError::Other("no remote session active - use session name instead".to_string()));
        }
        active_session.clone()
    } else {
        dst_session
    };

    // Validate sessions exist
    if !app.sessions.has_session(&src_session) {
        return Err(ThopError::Other(format!("source session '{}' not found", src_session)));
    }
    if !app.sessions.has_session(&dst_session) {
        return Err(ThopError::Other(format!("destination session '{}' not found", dst_session)));
    }

    let src_type = app.sessions.get_session(&src_session).map(|s| s.session_type().to_string()).unwrap_or_default();
    let dst_type = app.sessions.get_session(&dst_session).map(|s| s.session_type().to_string()).unwrap_or_default();

    // Handle different transfer scenarios
    if src_type == "local" && dst_type == "local" {
        return Err(ThopError::Other("both source and destination are local - use regular cp command".to_string()));
    }

    if src_type == "local" && dst_type == "ssh" {
        // Upload: local -> remote (via cat + execute)
        println!("Uploading {} to {}:{}...", src_path, dst_session, dst_path);
        let expanded_src = expand_path(&src_path);
        let content = fs::read(&expanded_src)
            .map_err(|e| ThopError::Other(format!("failed to read source file: {}", e)))?;

        // Use cat with heredoc to write file
        let cmd = format!(
            "cat > {} << 'THOP_EOF'\n{}\nTHOP_EOF",
            shell_escape(&dst_path),
            String::from_utf8_lossy(&content)
        );
        let result = app.sessions.execute_on(&dst_session, &cmd)?;
        if result.exit_code != 0 {
            return Err(ThopError::Other(format!("failed to write file: {}", result.stderr.trim())));
        }
        println!("Upload complete ({} bytes)", content.len());
        return Ok(());
    }

    if src_type == "ssh" && dst_type == "local" {
        // Download: remote -> local (via cat)
        println!("Downloading {}:{} to {}...", src_session, src_path, dst_path);
        let cmd = format!("cat {}", shell_escape(&src_path));
        let result = app.sessions.execute_on(&src_session, &cmd)?;
        if result.exit_code != 0 {
            return Err(ThopError::Other(format!("failed to read file: {}", result.stderr.trim())));
        }

        let expanded_dst = expand_path(&dst_path);
        fs::write(&expanded_dst, result.stdout.as_bytes())
            .map_err(|e| ThopError::Other(format!("failed to write file: {}", e)))?;
        println!("Download complete ({} bytes)", result.stdout.len());
        return Ok(());
    }

    if src_type == "ssh" && dst_type == "ssh" {
        // Remote to remote: download then upload
        println!("Reading {}:{}...", src_session, src_path);
        let cmd = format!("cat {}", shell_escape(&src_path));
        let result = app.sessions.execute_on(&src_session, &cmd)?;
        if result.exit_code != 0 {
            return Err(ThopError::Other(format!("failed to read from {}: {}", src_session, result.stderr.trim())));
        }

        println!("Writing to {}:{}...", dst_session, dst_path);
        let write_cmd = format!(
            "cat > {} << 'THOP_EOF'\n{}\nTHOP_EOF",
            shell_escape(&dst_path),
            result.stdout
        );
        let write_result = app.sessions.execute_on(&dst_session, &write_cmd)?;
        if write_result.exit_code != 0 {
            return Err(ThopError::Other(format!("failed to write to {}: {}", dst_session, write_result.stderr.trim())));
        }
        println!("Copy complete ({} bytes)", result.stdout.len());
        return Ok(());
    }

    Err(ThopError::Other("unsupported copy operation".to_string()))
}

/// Handle /shell command - run interactive command
fn cmd_shell(app: &mut App, command: &str) -> Result<()> {
    use std::process::{Command, Stdio};

    let session_name = app.sessions.get_active_session_name();
    let session = app.sessions.get_session(session_name)
        .ok_or_else(|| ThopError::Other("No active session".to_string()))?;

    if session.session_type() == "local" {
        // For local sessions, spawn the command with inherited stdio
        // This allows interactive programs like vim, top, etc. to work
        let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string());

        let status = Command::new(&shell)
            .arg("-c")
            .arg(command)
            .stdin(Stdio::inherit())
            .stdout(Stdio::inherit())
            .stderr(Stdio::inherit())
            .status()
            .map_err(|e| ThopError::Other(format!("Failed to execute command: {}", e)))?;

        if !status.success() {
            if let Some(code) = status.code() {
                println!("Command exited with code {}", code);
            }
        }

        Ok(())
    } else {
        // For SSH sessions, we need PTY support which is more complex
        // For now, provide a helpful message
        Err(ThopError::Other(
            "Interactive shell commands on SSH sessions require PTY support.\n\
             This feature is not yet fully implemented for remote sessions.\n\
             Tip: For simple commands, use regular execution instead of /shell.".to_string()
        ))
    }
}

/// Parse a file specification in the format "session:path" or just "path"
fn parse_file_spec(spec: &str) -> (String, String) {
    // Handle Windows-style paths (C:\...) by checking if it looks like a drive letter
    if spec.len() >= 2 && spec.chars().nth(1) == Some(':') {
        let first = spec.chars().next().unwrap();
        if first.is_ascii_alphabetic() {
            return (String::new(), spec.to_string());
        }
    }

    // Look for session:path format
    if let Some(idx) = spec.find(':') {
        if idx > 0 {
            return (spec[..idx].to_string(), spec[idx + 1..].to_string());
        }
    }

    // Just a path, no session specified
    (String::new(), spec.to_string())
}

/// Expand ~ to home directory in path
fn expand_path(path: &str) -> PathBuf {
    if path.starts_with("~/") {
        dirs::home_dir()
            .map(|h| h.join(&path[2..]))
            .unwrap_or_else(|| PathBuf::from(path))
    } else {
        PathBuf::from(path)
    }
}

/// Escape a string for shell use
fn shell_escape(s: &str) -> String {
    if s.contains(|c: char| c.is_whitespace() || c == '\'' || c == '"' || c == '\\' || c == '$') {
        format!("'{}'", s.replace('\'', "'\\''"))
    } else {
        s.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_expand_path() {
        let expanded = expand_path("~/test.txt");
        assert!(expanded.to_string_lossy().contains("test.txt"));
        assert!(!expanded.to_string_lossy().starts_with("~/"));

        let regular = expand_path("/tmp/test.txt");
        assert_eq!(regular.to_string_lossy(), "/tmp/test.txt");
    }

    #[test]
    fn test_shell_escape() {
        assert_eq!(shell_escape("simple"), "simple");
        assert_eq!(shell_escape("with space"), "'with space'");
        assert_eq!(shell_escape("with'quote"), "'with'\\''quote'");
    }
}
