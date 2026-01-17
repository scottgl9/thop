//! MCP tool implementations

use std::collections::HashMap;

use serde_json::Value;

use super::errors::{ErrorCode, MCPError};
use super::protocol::{Content, InputSchema, Property, Tool, ToolCallResult};
use super::server::Server;

/// Get all tool definitions
pub fn get_tool_definitions() -> Vec<Tool> {
    vec![
        // Session management tools
        Tool {
            name: "connect".to_string(),
            description: "Connect to an SSH session".to_string(),
            input_schema: InputSchema {
                schema_type: "object".to_string(),
                properties: {
                    let mut props = HashMap::new();
                    props.insert(
                        "session".to_string(),
                        Property {
                            property_type: "string".to_string(),
                            description: Some("Name of the session to connect to".to_string()),
                            enum_values: None,
                            default: None,
                        },
                    );
                    props
                },
                required: Some(vec!["session".to_string()]),
            },
        },
        Tool {
            name: "switch".to_string(),
            description: "Switch to a different session".to_string(),
            input_schema: InputSchema {
                schema_type: "object".to_string(),
                properties: {
                    let mut props = HashMap::new();
                    props.insert(
                        "session".to_string(),
                        Property {
                            property_type: "string".to_string(),
                            description: Some("Name of the session to switch to".to_string()),
                            enum_values: None,
                            default: None,
                        },
                    );
                    props
                },
                required: Some(vec!["session".to_string()]),
            },
        },
        Tool {
            name: "close".to_string(),
            description: "Close an SSH session".to_string(),
            input_schema: InputSchema {
                schema_type: "object".to_string(),
                properties: {
                    let mut props = HashMap::new();
                    props.insert(
                        "session".to_string(),
                        Property {
                            property_type: "string".to_string(),
                            description: Some("Name of the session to close".to_string()),
                            enum_values: None,
                            default: None,
                        },
                    );
                    props
                },
                required: Some(vec!["session".to_string()]),
            },
        },
        Tool {
            name: "status".to_string(),
            description: "Get status of all sessions".to_string(),
            input_schema: InputSchema {
                schema_type: "object".to_string(),
                properties: HashMap::new(),
                required: None,
            },
        },
        // Command execution tool
        Tool {
            name: "execute".to_string(),
            description: "Execute a command in the active session (optionally in background)".to_string(),
            input_schema: InputSchema {
                schema_type: "object".to_string(),
                properties: {
                    let mut props = HashMap::new();
                    props.insert(
                        "command".to_string(),
                        Property {
                            property_type: "string".to_string(),
                            description: Some("Command to execute".to_string()),
                            enum_values: None,
                            default: None,
                        },
                    );
                    props.insert(
                        "session".to_string(),
                        Property {
                            property_type: "string".to_string(),
                            description: Some("Optional: specific session to execute in (uses active session if not specified)".to_string()),
                            enum_values: None,
                            default: None,
                        },
                    );
                    props.insert(
                        "timeout".to_string(),
                        Property {
                            property_type: "integer".to_string(),
                            description: Some("Optional: command timeout in seconds (ignored if background is true)".to_string()),
                            enum_values: None,
                            default: Some(Value::from(300)),
                        },
                    );
                    props.insert(
                        "background".to_string(),
                        Property {
                            property_type: "boolean".to_string(),
                            description: Some("Optional: run command in background (default: false)".to_string()),
                            enum_values: None,
                            default: Some(Value::Bool(false)),
                        },
                    );
                    props
                },
                required: Some(vec!["command".to_string()]),
            },
        },
    ]
}

/// Handle connect tool
pub fn tool_connect(server: &mut Server, args: HashMap<String, Value>) -> ToolCallResult {
    let session_name = match args.get("session").and_then(|v| v.as_str()) {
        Some(s) => s,
        None => return MCPError::missing_parameter("session").to_tool_result(),
    };

    if let Err(e) = server.sessions.connect(session_name) {
        let err_str = e.to_string();

        // Check for specific error patterns
        if err_str.contains("not found") || err_str.contains("does not exist") {
            return MCPError::session_not_found(session_name).to_tool_result();
        }
        if err_str.contains("key") && err_str.contains("auth") {
            return MCPError::auth_key_failed(session_name).to_tool_result();
        }
        if err_str.contains("password") {
            return MCPError::auth_password_failed(session_name).to_tool_result();
        }
        if err_str.contains("host key") || err_str.contains("known_hosts") {
            return MCPError::host_key_unknown(session_name).to_tool_result();
        }
        if err_str.contains("timeout") {
            return MCPError::new(ErrorCode::ConnectionTimeout, "Connection timed out")
                .with_session(session_name)
                .with_suggestion("Check network connectivity and firewall settings")
                .to_tool_result();
        }
        if err_str.contains("refused") {
            return MCPError::new(ErrorCode::ConnectionRefused, "Connection refused")
                .with_session(session_name)
                .with_suggestion("Verify the host and port are correct")
                .to_tool_result();
        }

        return MCPError::connection_failed(session_name, &err_str).to_tool_result();
    }

    ToolCallResult {
        content: vec![Content::text(format!(
            "Successfully connected to session '{}'",
            session_name
        ))],
        is_error: false,
    }
}

/// Handle switch tool
pub fn tool_switch(server: &mut Server, args: HashMap<String, Value>) -> ToolCallResult {
    let session_name = match args.get("session").and_then(|v| v.as_str()) {
        Some(s) => s,
        None => return MCPError::missing_parameter("session").to_tool_result(),
    };

    if let Err(e) = server.sessions.set_active_session(session_name) {
        let err_str = e.to_string();

        if err_str.contains("not found") {
            return MCPError::session_not_found(session_name).to_tool_result();
        }
        if err_str.contains("not connected") {
            return MCPError::session_not_connected(session_name).to_tool_result();
        }

        return MCPError::new(ErrorCode::OperationFailed, format!("Failed to switch session: {}", e))
            .with_session(session_name)
            .to_tool_result();
    }

    // Get session info
    let cwd = server
        .sessions
        .get_session(session_name)
        .map(|s| s.get_cwd().to_string())
        .unwrap_or_else(|| "unknown".to_string());

    ToolCallResult {
        content: vec![Content::text(format!(
            "Switched to session '{}' (cwd: {})",
            session_name, cwd
        ))],
        is_error: false,
    }
}

/// Handle close tool
pub fn tool_close(server: &mut Server, args: HashMap<String, Value>) -> ToolCallResult {
    let session_name = match args.get("session").and_then(|v| v.as_str()) {
        Some(s) => s,
        None => return MCPError::missing_parameter("session").to_tool_result(),
    };

    if let Err(e) = server.sessions.disconnect(session_name) {
        let err_str = e.to_string();

        if err_str.contains("not found") {
            return MCPError::session_not_found(session_name).to_tool_result();
        }
        if err_str.contains("cannot close local") || err_str.contains("local session") {
            return MCPError::cannot_close_local(session_name).to_tool_result();
        }

        return MCPError::new(ErrorCode::OperationFailed, format!("Failed to close session: {}", e))
            .with_session(session_name)
            .to_tool_result();
    }

    ToolCallResult {
        content: vec![Content::text(format!("Session '{}' closed", session_name))],
        is_error: false,
    }
}

/// Handle status tool
pub fn tool_status(server: &mut Server, _args: HashMap<String, Value>) -> ToolCallResult {
    let sessions = server.sessions.list_sessions();

    match serde_json::to_string_pretty(&sessions) {
        Ok(data) => ToolCallResult {
            content: vec![Content::text_with_mime(data, "application/json")],
            is_error: false,
        },
        Err(e) => MCPError::new(ErrorCode::OperationFailed, format!("Failed to format status: {}", e))
            .with_suggestion("Check system resources and try again")
            .to_tool_result(),
    }
}

/// Handle execute tool
pub fn tool_execute(server: &mut Server, args: HashMap<String, Value>) -> ToolCallResult {
    let command = match args.get("command").and_then(|v| v.as_str()) {
        Some(s) => s,
        None => return MCPError::missing_parameter("command").to_tool_result(),
    };

    let session_name = args.get("session").and_then(|v| v.as_str());
    let background = args.get("background").and_then(|v| v.as_bool()).unwrap_or(false);
    let _timeout = args.get("timeout").and_then(|v| v.as_u64()).unwrap_or(300);

    // Handle background execution
    if background {
        return MCPError::not_implemented("Background execution").to_tool_result();
    }

    // Execute the command
    let result = if let Some(name) = session_name {
        if !server.sessions.has_session(name) {
            return MCPError::session_not_found(name).to_tool_result();
        }
        server.sessions.execute_on(name, command)
    } else {
        server.sessions.execute(command)
    };

    let active_session = session_name
        .map(|s| s.to_string())
        .unwrap_or_else(|| server.sessions.get_active_session_name().to_string());

    match result {
        Ok(exec_result) => {
            let mut content = vec![];

            // Add stdout if present
            if !exec_result.stdout.is_empty() {
                content.push(Content::text(&exec_result.stdout));
            }

            // Add stderr if present
            if !exec_result.stderr.is_empty() {
                content.push(Content::text(format!("stderr:\n{}", exec_result.stderr)));
            }

            // Add exit code if non-zero
            if exec_result.exit_code != 0 {
                content.push(Content::text(format!("Exit code: {}", exec_result.exit_code)));
            }

            // If no output at all, indicate success
            if content.is_empty() {
                content.push(Content::text("Command executed successfully (no output)"));
            }

            ToolCallResult {
                content,
                is_error: exec_result.exit_code != 0,
            }
        }
        Err(e) => {
            let err_str = e.to_string();

            // Check for timeout
            if err_str.contains("timeout") {
                return MCPError::command_timeout(&active_session, _timeout).to_tool_result();
            }

            // Check for permission denied
            if err_str.contains("permission denied") {
                return MCPError::new(ErrorCode::PermissionDenied, "Permission denied")
                    .with_session(&active_session)
                    .with_suggestion("Check file/directory permissions or use sudo if appropriate")
                    .to_tool_result();
            }

            // Check for command not found
            if err_str.contains("command not found") || err_str.contains("not found") {
                return MCPError::new(ErrorCode::CommandNotFound, format!("Command not found: {}", command))
                    .with_session(&active_session)
                    .with_suggestion("Verify the command is installed and in PATH")
                    .to_tool_result();
            }

            // Generic command failure
            MCPError::new(ErrorCode::CommandFailed, err_str)
                .with_session(&active_session)
                .to_tool_result()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::session::Manager as SessionManager;
    use crate::state::Manager as StateManager;

    fn create_test_server() -> Server {
        let config = Config::default();
        let state = StateManager::new(&config.settings.state_file);
        let sessions = SessionManager::new(&config, Some(StateManager::new(&config.settings.state_file)));
        Server::new(config, sessions, state)
    }

    #[test]
    fn test_get_tool_definitions() {
        let tools = get_tool_definitions();
        assert!(!tools.is_empty());

        // Check for required tools
        let tool_names: Vec<&str> = tools.iter().map(|t| t.name.as_str()).collect();
        assert!(tool_names.contains(&"connect"));
        assert!(tool_names.contains(&"switch"));
        assert!(tool_names.contains(&"close"));
        assert!(tool_names.contains(&"status"));
        assert!(tool_names.contains(&"execute"));
    }

    #[test]
    fn test_tool_status() {
        let mut server = create_test_server();
        let result = tool_status(&mut server, HashMap::new());
        assert!(!result.is_error);
        assert!(!result.content.is_empty());
    }

    #[test]
    fn test_tool_connect_missing_session() {
        let mut server = create_test_server();
        let result = tool_connect(&mut server, HashMap::new());
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("MISSING_PARAMETER"));
    }

    #[test]
    fn test_tool_switch_missing_session() {
        let mut server = create_test_server();
        let result = tool_switch(&mut server, HashMap::new());
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("MISSING_PARAMETER"));
    }

    #[test]
    fn test_tool_close_missing_session() {
        let mut server = create_test_server();
        let result = tool_close(&mut server, HashMap::new());
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("MISSING_PARAMETER"));
    }

    #[test]
    fn test_tool_execute_missing_command() {
        let mut server = create_test_server();
        let result = tool_execute(&mut server, HashMap::new());
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("MISSING_PARAMETER"));
    }

    #[test]
    fn test_tool_execute_local() {
        let mut server = create_test_server();
        let mut args = HashMap::new();
        args.insert("command".to_string(), Value::String("echo hello".to_string()));

        let result = tool_execute(&mut server, args);
        assert!(!result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("hello"));
    }

    #[test]
    fn test_tool_switch_local() {
        let mut server = create_test_server();
        let mut args = HashMap::new();
        args.insert("session".to_string(), Value::String("local".to_string()));

        let result = tool_switch(&mut server, args);
        assert!(!result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("Switched to session 'local'"));
    }

    #[test]
    fn test_tool_connect_nonexistent() {
        let mut server = create_test_server();
        let mut args = HashMap::new();
        args.insert("session".to_string(), Value::String("nonexistent".to_string()));

        let result = tool_connect(&mut server, args);
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("SESSION_NOT_FOUND"));
    }

    #[test]
    fn test_tool_execute_background_not_implemented() {
        let mut server = create_test_server();
        let mut args = HashMap::new();
        args.insert("command".to_string(), Value::String("sleep 10".to_string()));
        args.insert("background".to_string(), Value::Bool(true));

        let result = tool_execute(&mut server, args);
        assert!(result.is_error);
        assert!(result.content[0].text.as_ref().unwrap().contains("NOT_IMPLEMENTED"));
    }
}
