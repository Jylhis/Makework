use std::collections::BTreeMap;
use std::path::PathBuf;
use std::process::Command;

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use mw_core::query::query_activity;
use mw_core::repository::Repository;

fn setup_repo_with_commits(dir: &std::path::Path) {
    std::fs::create_dir_all(dir).unwrap();
    Command::new("git")
        .args(["init"])
        .arg(dir)
        .output()
        .expect("git init failed");
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["config", "user.email", "test@test.com"])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["config", "user.name", "Test User"])
        .output()
        .unwrap();

    // Create multiple commits
    std::fs::write(dir.join("file1.txt"), "first").unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["add", "."])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["commit", "-m", "first commit"])
        .output()
        .unwrap();

    std::fs::write(dir.join("file2.txt"), "second").unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["add", "."])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["commit", "-m", "second commit"])
        .output()
        .unwrap();
}

fn setup_bare_with_worktree(tmp: &std::path::Path) -> (PathBuf, PathBuf) {
    let source = tmp.join("source");
    setup_repo_with_commits(&source);

    let bare = tmp.join("repo.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source)
        .arg(&bare)
        .output()
        .expect("bare clone failed");

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();
    let wt_path = tmp.join("worktrees").join(&default_branch);
    mw_core::worktree::create_worktree(&bare, &default_branch, &wt_path).unwrap();

    (bare, wt_path)
}

#[test]
fn query_activity_finds_recent_commits() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path) = setup_bare_with_worktree(tmp.path());

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let mut catalog = Catalog::default();
    catalog.repos.insert(
        "test-repo".to_string(),
        Repository {
            name: "test-repo".to_string(),
            path: bare,
            url: None,
            main_branch: default_branch,
            remotes: BTreeMap::new(),
            projects: BTreeMap::new(),
        },
    );

    let entries = query_activity(&catalog, &config, "1 year ago", None, None);
    assert!(
        entries.len() >= 2,
        "should find at least 2 commits, got: {}",
        entries.len()
    );
    assert_eq!(entries[0].repo_name, "test-repo");
    assert_eq!(entries[0].author, "Test User");
}

#[test]
fn query_activity_with_author_filter() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path) = setup_bare_with_worktree(tmp.path());

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let mut catalog = Catalog::default();
    catalog.repos.insert(
        "test-repo".to_string(),
        Repository {
            name: "test-repo".to_string(),
            path: bare,
            url: None,
            main_branch: default_branch,
            remotes: BTreeMap::new(),
            projects: BTreeMap::new(),
        },
    );

    // Filter by existing author
    let entries = query_activity(&catalog, &config, "1 year ago", None, Some("Test User"));
    assert!(!entries.is_empty(), "should find commits by Test User");

    // Filter by non-existing author
    let entries = query_activity(&catalog, &config, "1 year ago", None, Some("Nobody"));
    assert!(entries.is_empty(), "should not find commits by Nobody");
}

#[test]
fn query_activity_empty_catalog() {
    let config = MakeworkConfig {
        worktree_root: PathBuf::from("/tmp/nonexistent-wt"),
        bare_root: PathBuf::from("/tmp/nonexistent-bare"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let catalog = Catalog::default();
    let entries = query_activity(&catalog, &config, "1 year ago", None, None);
    assert!(entries.is_empty());
}
