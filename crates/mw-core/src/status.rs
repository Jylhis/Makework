use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::catalog::Catalog;
use crate::config::MakeworkConfig;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorktreeStatus {
    pub path: PathBuf,
    pub branch: String,
    pub dirty_count: u32,
    pub ahead: u32,
    pub behind: u32,
    pub is_orphaned: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RepoStatusCache {
    pub repo_name: String,
    pub updated_at: String,
    pub worktrees: Vec<WorktreeStatus>,
}

fn cache_dir() -> Result<PathBuf, crate::config::ConfigError> {
    Ok(MakeworkConfig::state_dir()?.join("cache"))
}

fn cache_path(repo_name: &str) -> Result<PathBuf, crate::config::ConfigError> {
    Ok(cache_dir()?.join(format!("{repo_name}.json")))
}

pub fn read_cache(repo_name: &str) -> Option<RepoStatusCache> {
    let path = cache_path(repo_name).ok()?;
    let content = fs::read_to_string(path).ok()?;
    serde_json::from_str(&content).ok()
}

pub fn write_cache(cache: &RepoStatusCache) -> Result<(), std::io::Error> {
    let dir = cache_dir().map_err(|e| std::io::Error::other(e.to_string()))?;
    fs::create_dir_all(&dir)?;
    let path = dir.join(format!("{}.json", cache.repo_name));
    let content = serde_json::to_string_pretty(cache).map_err(std::io::Error::other)?;
    fs::write(path, content)
}

pub fn get_worktree_status(worktree_path: &Path) -> WorktreeStatus {
    let is_orphaned = !worktree_path.exists();
    let branch = get_branch_name(worktree_path).unwrap_or_else(|| "unknown".to_string());
    let dirty_count = get_dirty_count(worktree_path).unwrap_or(0);
    let (ahead, behind) = get_ahead_behind(worktree_path).unwrap_or((0, 0));

    WorktreeStatus {
        path: worktree_path.to_path_buf(),
        branch,
        dirty_count,
        ahead,
        behind,
        is_orphaned,
    }
}

fn get_branch_name(worktree_path: &Path) -> Option<String> {
    let output = std::process::Command::new("git")
        .args(["-C"])
        .arg(worktree_path)
        .args(["rev-parse", "--abbrev-ref", "HEAD"])
        .output()
        .ok()?;
    if output.status.success() {
        Some(String::from_utf8_lossy(&output.stdout).trim().to_string())
    } else {
        None
    }
}

fn get_dirty_count(worktree_path: &Path) -> Option<u32> {
    let output = std::process::Command::new("git")
        .args(["-C"])
        .arg(worktree_path)
        .args(["status", "--porcelain"])
        .output()
        .ok()?;
    if output.status.success() {
        let count = String::from_utf8_lossy(&output.stdout)
            .lines()
            .filter(|l| !l.is_empty())
            .count();
        Some(count as u32)
    } else {
        None
    }
}

fn get_ahead_behind(worktree_path: &Path) -> Option<(u32, u32)> {
    let output = std::process::Command::new("git")
        .args(["-C"])
        .arg(worktree_path)
        .args(["rev-list", "--left-right", "--count", "HEAD...@{upstream}"])
        .output()
        .ok()?;
    if output.status.success() {
        let text = String::from_utf8_lossy(&output.stdout);
        let parts: Vec<&str> = text.trim().split('\t').collect();
        if parts.len() == 2 {
            let ahead = parts[0].parse().unwrap_or(0);
            let behind = parts[1].parse().unwrap_or(0);
            Some((ahead, behind))
        } else {
            Some((0, 0))
        }
    } else {
        Some((0, 0))
    }
}

pub fn get_all_status(
    catalog: &Catalog,
    _config: &MakeworkConfig,
) -> Vec<(String, Vec<WorktreeStatus>)> {
    let mut results = Vec::new();
    for (repo_name, repo) in &catalog.repos {
        // Try to get worktrees for this repo
        let worktrees = crate::worktree::list_worktrees(&repo.path).unwrap_or_default();
        let mut statuses = Vec::new();
        for wt in &worktrees {
            if wt.is_bare {
                continue;
            }
            statuses.push(get_worktree_status(&wt.path));
        }
        if !statuses.is_empty() {
            // Update cache
            let cache = RepoStatusCache {
                repo_name: repo_name.clone(),
                updated_at: "now".to_string(),
                worktrees: statuses.clone(),
            };
            let _ = write_cache(&cache);
        }
        results.push((repo_name.clone(), statuses));
    }
    results
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn worktree_status_json_round_trip() {
        let status = WorktreeStatus {
            path: PathBuf::from("/tmp/wt/main"),
            branch: "main".to_string(),
            dirty_count: 3,
            ahead: 1,
            behind: 2,
            is_orphaned: false,
        };
        let json = serde_json::to_string(&status).unwrap();
        let deserialized: WorktreeStatus = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.branch, "main");
        assert_eq!(deserialized.dirty_count, 3);
        assert_eq!(deserialized.ahead, 1);
        assert_eq!(deserialized.behind, 2);
        assert!(!deserialized.is_orphaned);
    }

    #[test]
    fn repo_status_cache_json_round_trip() {
        let cache = RepoStatusCache {
            repo_name: "test-repo".to_string(),
            updated_at: "2026-04-01T12:00:00Z".to_string(),
            worktrees: vec![
                WorktreeStatus {
                    path: PathBuf::from("/tmp/wt/main"),
                    branch: "main".to_string(),
                    dirty_count: 0,
                    ahead: 0,
                    behind: 0,
                    is_orphaned: false,
                },
                WorktreeStatus {
                    path: PathBuf::from("/tmp/wt/feature"),
                    branch: "feature/auth".to_string(),
                    dirty_count: 5,
                    ahead: 3,
                    behind: 0,
                    is_orphaned: false,
                },
            ],
        };
        let json = serde_json::to_string_pretty(&cache).unwrap();
        let deserialized: RepoStatusCache = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.repo_name, "test-repo");
        assert_eq!(deserialized.worktrees.len(), 2);
        assert_eq!(deserialized.worktrees[1].dirty_count, 5);
    }

    #[test]
    fn write_and_read_cache_round_trip() {
        let tmp = tempfile::tempdir().unwrap();
        // Override XDG_STATE_HOME so cache goes into our temp dir
        // SAFETY: test runs single-threaded for this env var
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }

        let cache = RepoStatusCache {
            repo_name: "roundtrip-test".to_string(),
            updated_at: "2026-04-01".to_string(),
            worktrees: vec![WorktreeStatus {
                path: PathBuf::from("/tmp/wt/main"),
                branch: "main".to_string(),
                dirty_count: 2,
                ahead: 0,
                behind: 1,
                is_orphaned: false,
            }],
        };

        write_cache(&cache).expect("write_cache should succeed");
        let loaded = read_cache("roundtrip-test").expect("read_cache should return Some");
        assert_eq!(loaded.repo_name, "roundtrip-test");
        assert_eq!(loaded.worktrees.len(), 1);
        assert_eq!(loaded.worktrees[0].dirty_count, 2);
        assert_eq!(loaded.worktrees[0].behind, 1);

        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    #[test]
    fn read_cache_nonexistent_returns_none() {
        let tmp = tempfile::tempdir().unwrap();
        // SAFETY: test runs single-threaded for this env var
        unsafe {
            std::env::set_var("XDG_STATE_HOME", tmp.path());
        }

        let result = read_cache("nonexistent-repo");
        assert!(result.is_none());

        unsafe {
            std::env::remove_var("XDG_STATE_HOME");
        }
    }

    #[test]
    fn get_worktree_status_nonexistent_path_is_orphaned() {
        let status = get_worktree_status(Path::new("/nonexistent/path/to/worktree"));
        assert!(status.is_orphaned);
        assert_eq!(status.branch, "unknown");
        assert_eq!(status.dirty_count, 0);
    }
}
