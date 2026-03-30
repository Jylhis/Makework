use std::io::{BufRead, Write};

use mw_core::config::MakeworkConfig;
use serde_json::{Value, json};

use crate::resources;
use crate::tools;

/// Tool definitions for the MCP tools/list response.
fn tool_definitions() -> Vec<Value> {
    vec![
        json!({
            "name": "go",
            "description": "Navigate to a project worktree, creating it if needed. Returns the worktree path and optional nix activation info.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "project": { "type": "string", "description": "Repository, project, or subproject name" },
                    "ref": { "type": "string", "description": "Branch, tag, or commit (defaults to main branch)" }
                },
                "required": ["project"]
            }
        }),
        json!({
            "name": "sync",
            "description": "Discover and register git repositories from scan roots.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "depth": { "type": "integer", "description": "Maximum scan depth" }
                }
            }
        }),
        json!({
            "name": "catalog_add",
            "description": "Register a git repository from a URL or local path.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "source": { "type": "string", "description": "Git URL or local path" }
                },
                "required": ["source"]
            }
        }),
        json!({
            "name": "fetch",
            "description": "Fetch updates for one or all repositories.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "project": { "type": "string", "description": "Specific repository to fetch (all if omitted)" }
                }
            }
        }),
    ]
}

/// Resource definitions for the MCP resources/list response.
fn resource_definitions() -> Vec<Value> {
    vec![
        json!({
            "uri": "makework://catalog",
            "name": "Catalog",
            "description": "List of all registered repositories with metadata.",
            "mimeType": "application/json"
        }),
        json!({
            "uri": "makework://status",
            "name": "Status",
            "description": "Worktree status overview for all repositories.",
            "mimeType": "application/json"
        }),
    ]
}

/// Handle a single JSON-RPC request and return a response.
pub fn handle_request(request: &Value, config: &MakeworkConfig) -> Option<Value> {
    let id = request.get("id");
    let method = request["method"].as_str()?;

    let result = match method {
        "initialize" => {
            json!({
                "protocolVersion": "2024-11-05",
                "capabilities": {
                    "tools": { "listChanged": false },
                    "resources": { "subscribe": false, "listChanged": false }
                },
                "serverInfo": {
                    "name": "makework",
                    "version": env!("CARGO_PKG_VERSION")
                }
            })
        }
        "notifications/initialized" | "initialized" => {
            // Notification — no response needed
            return None;
        }
        "tools/list" => {
            json!({ "tools": tool_definitions() })
        }
        "tools/call" => {
            let tool_name = request["params"]["name"].as_str().unwrap_or("");
            let arguments = &request["params"]["arguments"];
            let tool_result = match tool_name {
                "go" => tools::handle_go(config, arguments),
                "sync" => tools::handle_sync(config, arguments),
                "catalog_add" => tools::handle_catalog_add(config, arguments),
                "fetch" => tools::handle_fetch(config, arguments),
                _ => json!({"error": format!("unknown tool: {tool_name}")}),
            };

            let is_error = tool_result.get("error").is_some();
            json!({
                "content": [{
                    "type": "text",
                    "text": serde_json::to_string_pretty(&tool_result).unwrap_or_default()
                }],
                "isError": is_error
            })
        }
        "resources/list" => {
            json!({ "resources": resource_definitions() })
        }
        "resources/read" => {
            let uri = request["params"]["uri"].as_str().unwrap_or("");
            let content = match uri {
                "makework://catalog" => resources::handle_catalog_resource(config),
                "makework://status" => resources::handle_status_resource(config),
                _ => json!({"error": format!("unknown resource: {uri}")}),
            };
            json!({
                "contents": [{
                    "uri": uri,
                    "mimeType": "application/json",
                    "text": serde_json::to_string_pretty(&content).unwrap_or_default()
                }]
            })
        }
        _ => {
            json!({"error": {"code": -32601, "message": format!("method not found: {method}")}})
        }
    };

    let response = json!({
        "jsonrpc": "2.0",
        "id": id,
        "result": result
    });

    Some(response)
}

/// Run the MCP server on stdin/stdout.
pub fn run_stdio() {
    let config = MakeworkConfig::load().unwrap_or_else(|e| {
        eprintln!("Error loading config: {e}");
        std::process::exit(1);
    });

    let stdin = std::io::stdin();
    let stdout = std::io::stdout();
    let mut stdout = stdout.lock();

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };
        if line.is_empty() {
            continue;
        }

        let request: Value = match serde_json::from_str(&line) {
            Ok(v) => v,
            Err(e) => {
                let error_response = json!({
                    "jsonrpc": "2.0",
                    "id": null,
                    "error": {"code": -32700, "message": format!("parse error: {e}")}
                });
                let _ = writeln!(stdout, "{}", error_response);
                let _ = stdout.flush();
                continue;
            }
        };

        if let Some(response) = handle_request(&request, &config) {
            let _ = writeln!(stdout, "{}", response);
            let _ = stdout.flush();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_config() -> MakeworkConfig {
        let tmp = std::env::temp_dir().join("mw-mcp-test");
        MakeworkConfig {
            worktree_root: tmp.join("worktrees"),
            bare_root: tmp.join("repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
        }
    }

    #[test]
    fn initialize_returns_server_info() {
        let config = test_config();
        let request = json!({
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {"capabilities": {}}
        });
        let response = handle_request(&request, &config).unwrap();
        assert_eq!(response["id"], 1);
        assert_eq!(response["result"]["serverInfo"]["name"], "makework");
        assert!(response["result"]["capabilities"]["tools"].is_object());
    }

    #[test]
    fn tools_list_returns_tools() {
        let config = test_config();
        let request = json!({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list",
            "params": {}
        });
        let response = handle_request(&request, &config).unwrap();
        let tools = response["result"]["tools"].as_array().unwrap();
        let names: Vec<&str> = tools.iter().filter_map(|t| t["name"].as_str()).collect();
        assert!(names.contains(&"go"));
        assert!(names.contains(&"sync"));
        assert!(names.contains(&"catalog_add"));
        assert!(names.contains(&"fetch"));
    }

    #[test]
    fn resources_list_returns_resources() {
        let config = test_config();
        let request = json!({
            "jsonrpc": "2.0",
            "id": 3,
            "method": "resources/list",
            "params": {}
        });
        let response = handle_request(&request, &config).unwrap();
        let resources = response["result"]["resources"].as_array().unwrap();
        let uris: Vec<&str> = resources.iter().filter_map(|r| r["uri"].as_str()).collect();
        assert!(uris.contains(&"makework://catalog"));
        assert!(uris.contains(&"makework://status"));
    }

    #[test]
    fn initialized_notification_returns_none() {
        let config = test_config();
        let request = json!({
            "jsonrpc": "2.0",
            "method": "notifications/initialized"
        });
        assert!(handle_request(&request, &config).is_none());
    }
}
