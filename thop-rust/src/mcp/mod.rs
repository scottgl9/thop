//! MCP (Model Context Protocol) server implementation for thop
//!
//! This module implements the MCP protocol to allow AI agents to interact
//! with thop sessions programmatically.

mod errors;
mod handlers;
mod protocol;
mod server;
mod tools;

// Re-exports for external use
#[allow(unused_imports)]
pub use errors::{ErrorCode, MCPError};
pub use server::Server;
