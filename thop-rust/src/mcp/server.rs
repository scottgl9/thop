//! MCP server implementation

use std::collections::HashMap;
use std::io::{self, BufRead, BufReader, Write};
use std::sync::{Arc, Mutex};

use serde_json::Value;

use crate::config::Config;
use crate::logger;
use crate::session::Manager as SessionManager;
use crate::state::Manager as StateManager;

use super::errors::MCPError;
use super::handlers;
use super::protocol::{JsonRpcError, JsonRpcMessage, JsonRpcResponse};

/// MCP protocol version
pub const MCP_VERSION: &str = "2024-11-05";

/// Handler function type
type HandlerFn = fn(&mut Server, Option<Value>) -> Result<Option<Value>, MCPError>;

/// MCP Server for thop
pub struct Server {
    pub config: Config,
    pub sessions: SessionManager,
    pub state: StateManager,
    handlers: HashMap<String, HandlerFn>,
    output: Arc<Mutex<Box<dyn Write + Send>>>,
}

impl Server {
    /// Create a new MCP server
    pub fn new(config: Config, sessions: SessionManager, state: StateManager) -> Self {
        let mut server = Self {
            config,
            sessions,
            state,
            handlers: HashMap::new(),
            output: Arc::new(Mutex::new(Box::new(io::stdout()))),
        };

        server.register_handlers();
        server
    }

    /// Set custom output writer (useful for testing)
    pub fn set_output(&mut self, output: Box<dyn Write + Send>) {
        self.output = Arc::new(Mutex::new(output));
    }

    /// Register all JSON-RPC method handlers
    fn register_handlers(&mut self) {
        // MCP protocol methods
        self.handlers.insert("initialize".to_string(), handlers::handle_initialize);
        self.handlers.insert("initialized".to_string(), handlers::handle_initialized);
        self.handlers.insert("tools/list".to_string(), handlers::handle_tools_list);
        self.handlers.insert("tools/call".to_string(), handlers::handle_tool_call);
        self.handlers.insert("resources/list".to_string(), handlers::handle_resources_list);
        self.handlers.insert("resources/read".to_string(), handlers::handle_resource_read);
        self.handlers.insert("ping".to_string(), handlers::handle_ping);

        // Notification handlers
        self.handlers.insert("cancelled".to_string(), handlers::handle_cancelled);
        self.handlers.insert("progress".to_string(), handlers::handle_progress);
    }

    /// Run the MCP server, reading from stdin
    pub fn run(&mut self) -> crate::error::Result<()> {
        logger::info("Starting MCP server");

        let stdin = io::stdin();
        let reader = BufReader::new(stdin.lock());

        for line in reader.lines() {
            let line = line.map_err(|e| {
                crate::error::ThopError::Other(format!("Failed to read input: {}", e))
            })?;

            if line.is_empty() {
                continue;
            }

            if let Err(e) = self.handle_message(&line) {
                logger::error(&format!("Error handling message: {}", e));
                self.send_error(None, -32603, "Internal error", Some(&e.to_string()));
            }
        }

        Ok(())
    }

    /// Handle a single JSON-RPC message
    fn handle_message(&mut self, data: &str) -> Result<(), String> {
        let msg: JsonRpcMessage = serde_json::from_str(data)
            .map_err(|e| format!("Failed to parse JSON-RPC message: {}", e))?;

        // Handle request if method is present
        if let Some(ref method) = msg.method {
            return self.handle_request(&msg, method);
        }

        Ok(())
    }

    /// Handle a JSON-RPC request
    fn handle_request(&mut self, msg: &JsonRpcMessage, method: &str) -> Result<(), String> {
        logger::debug(&format!("Handling request: method={} id={:?}", method, msg.id));

        let handler = match self.handlers.get(method) {
            Some(h) => *h,
            None => {
                self.send_error(
                    msg.id.clone(),
                    -32601,
                    "Method not found",
                    Some(&format!("Unknown method: {}", method)),
                );
                return Ok(());
            }
        };

        // Execute handler
        match handler(self, msg.params.clone()) {
            Ok(result) => {
                // Send successful response if it's a request with an ID
                if msg.id.is_some() {
                    self.send_response(msg.id.clone(), result);
                }
            }
            Err(mcp_err) => {
                // Send error response
                let tool_result = mcp_err.to_tool_result();
                let result_value = serde_json::to_value(&tool_result).ok();
                self.send_response(msg.id.clone(), result_value);
            }
        }

        Ok(())
    }

    /// Send a JSON-RPC response
    fn send_response(&self, id: Option<Value>, result: Option<Value>) {
        let response = JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result,
            error: None,
        };

        if let Ok(data) = serde_json::to_string(&response) {
            self.write_output(&data);
        }
    }

    /// Send a JSON-RPC error response
    fn send_error(&self, id: Option<Value>, code: i32, message: &str, data: Option<&str>) {
        let response = JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: None,
            error: Some(JsonRpcError {
                code,
                message: message.to_string(),
                data: data.map(|s| Value::String(s.to_string())),
            }),
        };

        if let Ok(data) = serde_json::to_string(&response) {
            self.write_output(&data);
        }
    }

    /// Write output with newline
    fn write_output(&self, data: &str) {
        if let Ok(mut output) = self.output.lock() {
            let _ = writeln!(output, "{}", data);
            let _ = output.flush();
        }
    }

    /// Send a JSON-RPC notification
    #[allow(dead_code)]
    pub fn send_notification(&self, method: &str, params: Option<Value>) {
        let notification = JsonRpcMessage {
            jsonrpc: "2.0".to_string(),
            method: Some(method.to_string()),
            id: None,
            params,
        };

        if let Ok(data) = serde_json::to_string(&notification) {
            self.write_output(&data);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;
    use std::sync::{Arc, Mutex};

    struct TestOutput {
        buffer: Arc<Mutex<Vec<u8>>>,
    }

    impl TestOutput {
        fn new() -> (Self, Arc<Mutex<Vec<u8>>>) {
            let buffer = Arc::new(Mutex::new(Vec::new()));
            (Self { buffer: buffer.clone() }, buffer)
        }
    }

    impl Write for TestOutput {
        fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
            self.buffer.lock().unwrap().extend_from_slice(buf);
            Ok(buf.len())
        }

        fn flush(&mut self) -> io::Result<()> {
            Ok(())
        }
    }

    fn create_test_server() -> Server {
        let config = Config::default();
        let state = StateManager::new(&config.settings.state_file);
        let sessions = SessionManager::new(&config, Some(StateManager::new(&config.settings.state_file)));
        Server::new(config, sessions, state)
    }

    #[test]
    fn test_server_creation() {
        let server = create_test_server();
        assert!(!server.handlers.is_empty());
    }

    #[test]
    fn test_handler_registration() {
        let server = create_test_server();
        assert!(server.handlers.contains_key("initialize"));
        assert!(server.handlers.contains_key("tools/list"));
        assert!(server.handlers.contains_key("tools/call"));
        assert!(server.handlers.contains_key("resources/list"));
        assert!(server.handlers.contains_key("resources/read"));
        assert!(server.handlers.contains_key("ping"));
    }

    #[test]
    fn test_send_response() {
        let mut server = create_test_server();
        let (output, buffer) = TestOutput::new();
        server.set_output(Box::new(output));

        server.send_response(Some(Value::from(1)), Some(Value::String("test".to_string())));

        let output = buffer.lock().unwrap();
        let response: JsonRpcResponse = serde_json::from_slice(&output).unwrap();
        assert_eq!(response.jsonrpc, "2.0");
        assert_eq!(response.id, Some(Value::from(1)));
        assert_eq!(response.result, Some(Value::String("test".to_string())));
    }

    #[test]
    fn test_send_error() {
        let mut server = create_test_server();
        let (output, buffer) = TestOutput::new();
        server.set_output(Box::new(output));

        server.send_error(Some(Value::from(1)), -32601, "Method not found", Some("test"));

        let output = buffer.lock().unwrap();
        let response: JsonRpcResponse = serde_json::from_slice(&output).unwrap();
        assert_eq!(response.jsonrpc, "2.0");
        assert!(response.error.is_some());
        assert_eq!(response.error.as_ref().unwrap().code, -32601);
    }

    #[test]
    fn test_handle_unknown_method() {
        let mut server = create_test_server();
        let (output, buffer) = TestOutput::new();
        server.set_output(Box::new(output));

        let msg = JsonRpcMessage {
            jsonrpc: "2.0".to_string(),
            method: Some("unknown_method".to_string()),
            id: Some(Value::from(1)),
            params: None,
        };

        let _ = server.handle_request(&msg, "unknown_method");

        let output = buffer.lock().unwrap();
        let response: JsonRpcResponse = serde_json::from_slice(&output).unwrap();
        assert!(response.error.is_some());
        assert_eq!(response.error.as_ref().unwrap().code, -32601);
    }
}
