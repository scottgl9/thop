//! MCP request handlers

use serde_json::Value;

use crate::logger;

use super::errors::MCPError;
use super::protocol::{
    InitializeParams, InitializeResult, LoggingCapability, Resource,
    ResourceContent, ResourceReadParams, ResourceReadResult, ResourcesCapability,
    ServerCapabilities, ServerInfo, ToolCallParams, ToolsCapability,
};
use super::server::{Server, MCP_VERSION};
use super::tools;

/// Handle initialize request
pub fn handle_initialize(_server: &mut Server, params: Option<Value>) -> Result<Option<Value>, MCPError> {
    let params_value = params.ok_or_else(|| MCPError::missing_parameter("params"))?;

    let init_params: InitializeParams = serde_json::from_value(params_value)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::InvalidParameter, format!("Invalid params: {}", e)))?;

    logger::info(&format!(
        "MCP client connected: {} v{} (protocol {})",
        init_params.client_info.name,
        init_params.client_info.version,
        init_params.protocol_version
    ));

    let result = InitializeResult {
        protocol_version: MCP_VERSION.to_string(),
        capabilities: ServerCapabilities {
            tools: Some(ToolsCapability { list_changed: false }),
            resources: Some(ResourcesCapability {
                subscribe: false,
                list_changed: false,
            }),
            logging: Some(LoggingCapability {}),
            prompts: None,
            experimental: None,
        },
        server_info: ServerInfo {
            name: "thop-mcp".to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
        },
    };

    Ok(Some(serde_json::to_value(result).unwrap()))
}

/// Handle initialized notification
pub fn handle_initialized(_server: &mut Server, _params: Option<Value>) -> Result<Option<Value>, MCPError> {
    logger::debug("MCP client initialized");
    Ok(None)
}

/// Handle tools/list request
pub fn handle_tools_list(_server: &mut Server, _params: Option<Value>) -> Result<Option<Value>, MCPError> {
    let tools = tools::get_tool_definitions();

    let result = serde_json::json!({
        "tools": tools
    });

    Ok(Some(result))
}

/// Handle tools/call request
pub fn handle_tool_call(server: &mut Server, params: Option<Value>) -> Result<Option<Value>, MCPError> {
    let params_value = params.ok_or_else(|| MCPError::missing_parameter("params"))?;

    let call_params: ToolCallParams = serde_json::from_value(params_value)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::InvalidParameter, format!("Invalid params: {}", e)))?;

    logger::debug(&format!("Tool call: {}", call_params.name));

    // Route to appropriate tool handler
    let result = match call_params.name.as_str() {
        "connect" => tools::tool_connect(server, call_params.arguments),
        "switch" => tools::tool_switch(server, call_params.arguments),
        "close" => tools::tool_close(server, call_params.arguments),
        "status" => tools::tool_status(server, call_params.arguments),
        "execute" => tools::tool_execute(server, call_params.arguments),
        _ => {
            return Err(MCPError::new(
                super::errors::ErrorCode::InvalidParameter,
                format!("Unknown tool: {}", call_params.name),
            ));
        }
    };

    Ok(Some(serde_json::to_value(result).unwrap()))
}

/// Handle resources/list request
pub fn handle_resources_list(_server: &mut Server, _params: Option<Value>) -> Result<Option<Value>, MCPError> {
    let resources = vec![
        Resource {
            uri: "session://active".to_string(),
            name: "Active Session".to_string(),
            description: Some("Information about the currently active session".to_string()),
            mime_type: Some("application/json".to_string()),
        },
        Resource {
            uri: "session://all".to_string(),
            name: "All Sessions".to_string(),
            description: Some("Information about all configured sessions".to_string()),
            mime_type: Some("application/json".to_string()),
        },
        Resource {
            uri: "config://thop".to_string(),
            name: "Thop Configuration".to_string(),
            description: Some("Current thop configuration".to_string()),
            mime_type: Some("application/json".to_string()),
        },
        Resource {
            uri: "state://thop".to_string(),
            name: "Thop State".to_string(),
            description: Some("Current thop state including session states".to_string()),
            mime_type: Some("application/json".to_string()),
        },
    ];

    let result = serde_json::json!({
        "resources": resources
    });

    Ok(Some(result))
}

/// Handle resources/read request
pub fn handle_resource_read(server: &mut Server, params: Option<Value>) -> Result<Option<Value>, MCPError> {
    let params_value = params.ok_or_else(|| MCPError::missing_parameter("params"))?;

    let read_params: ResourceReadParams = serde_json::from_value(params_value)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::InvalidParameter, format!("Invalid params: {}", e)))?;

    let content = match read_params.uri.as_str() {
        "session://active" => get_active_session_resource(server)?,
        "session://all" => get_all_sessions_resource(server)?,
        "config://thop" => get_config_resource(server)?,
        "state://thop" => get_state_resource(server)?,
        _ => {
            return Err(MCPError::new(
                super::errors::ErrorCode::InvalidParameter,
                format!("Unknown resource URI: {}", read_params.uri),
            ));
        }
    };

    let result = ResourceReadResult {
        contents: vec![ResourceContent {
            uri: read_params.uri,
            mime_type: Some("application/json".to_string()),
            text: Some(content),
            blob: None,
        }],
    };

    Ok(Some(serde_json::to_value(result).unwrap()))
}

/// Handle ping request
pub fn handle_ping(_server: &mut Server, _params: Option<Value>) -> Result<Option<Value>, MCPError> {
    Ok(Some(serde_json::json!({
        "pong": true
    })))
}

/// Handle cancelled notification
pub fn handle_cancelled(_server: &mut Server, _params: Option<Value>) -> Result<Option<Value>, MCPError> {
    logger::debug("Received cancellation notification");
    Ok(None)
}

/// Handle progress notification
pub fn handle_progress(_server: &mut Server, params: Option<Value>) -> Result<Option<Value>, MCPError> {
    if let Some(params) = params {
        if let Ok(progress) = serde_json::from_value::<super::protocol::ProgressParams>(params) {
            logger::debug(&format!(
                "Progress update: token={} progress={}/{}",
                progress.progress_token,
                progress.progress,
                progress.total.unwrap_or(0.0)
            ));
        }
    }
    Ok(None)
}

// Resource helper functions

fn get_active_session_resource(server: &Server) -> Result<String, MCPError> {
    let session_name = server.sessions.get_active_session_name();
    let session = server.sessions.get_session(session_name)
        .ok_or_else(|| MCPError::no_active_session())?;

    let info = serde_json::json!({
        "name": session_name,
        "type": session.session_type(),
        "connected": session.is_connected(),
        "cwd": session.get_cwd(),
        "environment": session.get_env()
    });

    serde_json::to_string_pretty(&info)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::OperationFailed, format!("Failed to serialize: {}", e)))
}

fn get_all_sessions_resource(server: &Server) -> Result<String, MCPError> {
    let sessions = server.sessions.list_sessions();
    serde_json::to_string_pretty(&sessions)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::OperationFailed, format!("Failed to serialize: {}", e)))
}

fn get_config_resource(server: &Server) -> Result<String, MCPError> {
    serde_json::to_string_pretty(&server.config)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::OperationFailed, format!("Failed to serialize: {}", e)))
}

fn get_state_resource(server: &Server) -> Result<String, MCPError> {
    let active_session = server.state.get_active_session();

    let state_data = serde_json::json!({
        "active_session": active_session,
    });

    serde_json::to_string_pretty(&state_data)
        .map_err(|e| MCPError::new(super::errors::ErrorCode::OperationFailed, format!("Failed to serialize: {}", e)))
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
    fn test_handle_ping() {
        let mut server = create_test_server();
        let result = handle_ping(&mut server, None).unwrap();
        assert!(result.is_some());
        let value = result.unwrap();
        assert_eq!(value["pong"], true);
    }

    #[test]
    fn test_handle_tools_list() {
        let mut server = create_test_server();
        let result = handle_tools_list(&mut server, None).unwrap();
        assert!(result.is_some());
        let value = result.unwrap();
        assert!(value["tools"].is_array());
    }

    #[test]
    fn test_handle_resources_list() {
        let mut server = create_test_server();
        let result = handle_resources_list(&mut server, None).unwrap();
        assert!(result.is_some());
        let value = result.unwrap();
        assert!(value["resources"].is_array());
    }

    #[test]
    fn test_handle_initialized() {
        let mut server = create_test_server();
        let result = handle_initialized(&mut server, None).unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn test_handle_cancelled() {
        let mut server = create_test_server();
        let result = handle_cancelled(&mut server, None).unwrap();
        assert!(result.is_none());
    }
}
