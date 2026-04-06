use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use serde_json::{Value, json};

/// Handle the `makework://catalog` resource — returns list of all repos.
pub fn handle_catalog_resource(config: &MakeworkConfig) -> Value {
    let catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    let repos: Vec<Value> = catalog
        .repos
        .iter()
        .map(|(name, repo)| {
            json!({
                "name": name,
                "url": repo.url,
                "main_branch": repo.main_branch,
                "path": repo.path.display().to_string(),
            })
        })
        .collect();

    json!({ "repos": repos })
}

/// Handle the `makework://status` resource — returns worktree status for all repos.
pub fn handle_status_resource(config: &MakeworkConfig) -> Value {
    let catalog = match Catalog::load(config) {
        Ok(c) => c,
        Err(e) => return json!({"error": e.to_string()}),
    };

    let statuses = mw_core::status::get_all_status(&catalog, config);
    let result: Vec<Value> = statuses
        .iter()
        .map(|(repo_name, wts)| {
            let worktrees: Vec<Value> = wts
                .iter()
                .map(|s| {
                    json!({
                        "branch": s.branch,
                        "dirty_count": s.dirty_count,
                        "ahead": s.ahead,
                        "behind": s.behind,
                        "path": s.path.display().to_string(),
                        "is_orphaned": s.is_orphaned,
                    })
                })
                .collect();
            json!({
                "repo": repo_name,
                "worktrees": worktrees,
            })
        })
        .collect();

    json!({ "status": result })
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
    fn catalog_resource_returns_repo_list() {
        let tmp = tempfile::tempdir().unwrap();
        let config = test_config(tmp.path());

        // Empty catalog
        let result = handle_catalog_resource(&config);
        assert!(result["repos"].is_array());
    }

    #[test]
    fn status_resource_returns_worktree_status() {
        let tmp = tempfile::tempdir().unwrap();
        let config = test_config(tmp.path());

        let result = handle_status_resource(&config);
        assert!(result["status"].is_array());
    }
}
