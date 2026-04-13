use mw_core::catalog::{Catalog, SyncOptions};
use mw_core::config::MakeworkConfig;
use serde_json::{Value, json};

/// Handle the `go` tool — navigate to project worktree.
pub fn handle_go(config: &MakeworkConfig, params: &Value) -> Value {
    let project = match params["project"].as_str() {
        Some(p) => p,
        None => return json!({"error": "missing required parameter: project"}),
    };
    let ref_ = params["ref"].as_str();

    let catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    match mw_core::worktree::go(&catalog, config, project, ref_) {
        Ok(result) => {
            let mut response = json!({
                "path": result.path.display().to_string(),
            });
            if let Some(ref nix) = result.nix_activation {
                response["nix_command"] = json!(nix.activation_command.clone());
            }
            if let Some(ref dir) = result.nix_activation_dir {
                response["nix_dir"] = json!(dir.display().to_string());
            }
            response
        }
        Err(e) => json!({"error": e.to_string()}),
    }
}

/// Handle the `sync` tool — discover and register repos.
pub fn handle_sync(config: &MakeworkConfig, params: &Value) -> Value {
    let mut catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    let scan_roots = if config.scan_roots.is_empty() {
        match mw_core::config::expand_tilde(std::path::Path::new("~/Developer")) {
            Ok(dev) if dev.exists() => vec![dev],
            _ => vec![],
        }
    } else {
        config.scan_roots.clone()
    };

    let depth = params["depth"].as_u64().map(|d| d as u32);
    let options = SyncOptions {
        max_depth: depth.unwrap_or_else(|| config.sync_max_depth.unwrap_or(1)),
        exclude: config.sync_exclude.clone(),
    };

    match catalog.sync(config, &scan_roots, &options) {
        Ok(added) => json!({
            "added": added,
            "count": added.len(),
        }),
        Err(e) => json!({"error": e.to_string()}),
    }
}

/// Handle the `catalog_add` tool — register a repo from URL or path.
pub fn handle_catalog_add(config: &MakeworkConfig, params: &Value) -> Value {
    let source = match params["source"].as_str() {
        Some(s) => s,
        None => return json!({"error": "missing required parameter: source"}),
    };

    let mut catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    let result = if mw_core::repository::parse_remote_url(source).is_some() {
        catalog.catalog_add_url(source, config)
    } else {
        catalog.catalog_add(std::path::Path::new(source), config)
    };

    match result {
        Ok((name, is_new)) => json!({"name": name, "created": is_new}),
        Err(e) => json!({"error": e.to_string()}),
    }
}

/// Handle the `fetch` tool — fetch updates for repos.
pub fn handle_fetch(config: &MakeworkConfig, params: &Value) -> Value {
    let catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    let project = params["project"].as_str();

    let repos: Vec<_> = if let Some(name) = project {
        match catalog.find_repo(name) {
            Some(r) => vec![r],
            None => return json!({"error": format!("repository not found: {name}")}),
        }
    } else {
        catalog.repos.values().collect()
    };

    let mut results = Vec::new();
    for repo in &repos {
        match mw_core::repository::fetch(&repo.path) {
            Ok(()) => results.push(json!({"repo": repo.name, "status": "ok"})),
            Err(e) => {
                results.push(json!({"repo": repo.name, "status": "error", "error": e.to_string()}))
            }
        }
    }

    json!({"results": results})
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_config(tmp: &std::path::Path) -> MakeworkConfig {
        MakeworkConfig {
            worktree_root: tmp.join("worktrees"),
            bare_root: tmp.join("repos"),
            scan_roots: Vec::new(),
            sync_max_depth: None,
            sync_exclude: Vec::new(),
            template_dir: None,
            resolver: None,
        }
    }

    #[test]
    fn go_tool_error_on_unknown_project() {
        let tmp = tempfile::tempdir().unwrap();
        let config = test_config(tmp.path());
        let params = json!({"project": "nonexistent"});
        let result = handle_go(&config, &params);
        assert!(result["error"].is_string());
    }

    #[test]
    fn go_tool_error_on_missing_project() {
        let tmp = tempfile::tempdir().unwrap();
        let config = test_config(tmp.path());
        let params = json!({});
        let result = handle_go(&config, &params);
        assert!(result["error"].as_str().unwrap().contains("missing"));
    }
}
