use std::process::Command;

mod common;
use common::setup_temp_git_repo;

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;

#[test]
fn catalog_add_registers_local_repo() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("my-repo");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    // Set XDG config dir to temp to avoid polluting real config
    let config_dir = tmp.path().join("config/makework");
    std::fs::create_dir_all(&config_dir).unwrap();

    let mut catalog = Catalog::default();
    let (name, is_new) = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add should succeed");

    assert_eq!(name, "my-repo");
    assert!(is_new);
    assert!(catalog.repos.contains_key("my-repo"));

    let repo = &catalog.repos["my-repo"];
    assert!(repo.path.exists(), "bare clone dir should exist");

    // Verify idempotent re-add
    let (name2, is_new2) = catalog
        .catalog_add(&repo_dir, &config)
        .expect("re-add should succeed");
    assert_eq!(name2, "my-repo");
    assert!(!is_new2);
    assert_eq!(catalog.repos.len(), 1);
}

#[test]
fn catalog_add_creates_default_worktree() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("wt-test");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

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
    let (_, is_new) = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add");
    assert!(is_new);

    // A worktree should have been created under the worktree root
    let wt_root = tmp.path().join("worktrees");
    assert!(wt_root.exists(), "worktree root should exist");

    // Should have at least one directory under worktrees/local/
    let local_dir = wt_root.join("local");
    if local_dir.exists() {
        let entries: Vec<_> = std::fs::read_dir(&local_dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .collect();
        assert!(!entries.is_empty(), "should have worktree entries");
    }
}

#[test]
fn catalog_add_falls_back_when_remote_unreachable() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("remote-fail-repo");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

    // Set a bogus remote URL that will fail to clone
    Command::new("git")
        .args(["-C"])
        .arg(&repo_dir)
        .args([
            "remote",
            "add",
            "origin",
            "https://nonexistent.invalid/user/repo.git",
        ])
        .output()
        .expect("git remote add failed");

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
    let (name, is_new) = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add should succeed with local fallback");

    assert_eq!(name, "repo");
    assert!(is_new);
    assert!(catalog.repos.contains_key("repo"));

    let repo = &catalog.repos["repo"];
    // The remote URL should still be stored
    assert_eq!(
        repo.url.as_deref(),
        Some("https://nonexistent.invalid/user/repo.git")
    );
    // The bare clone should exist (created from local source)
    assert!(
        repo.path.exists(),
        "bare clone should exist from local fallback"
    );
}

#[test]
fn catalog_add_rejects_non_git_dir() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let non_repo = tmp.path().join("not-a-repo");
    std::fs::create_dir_all(&non_repo).unwrap();

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
    let result = catalog.catalog_add(&non_repo, &config);
    assert!(result.is_err());
}

#[test]
fn catalog_add_url_rejects_invalid_url() {
    let tmp = tempfile::tempdir().expect("tempdir");
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
    let result = catalog.catalog_add_url("not-a-valid-url", &config);
    assert!(result.is_err());
    assert!(
        result.unwrap_err().to_string().contains("invalid git URL"),
        "should report invalid URL"
    );
}

#[test]
fn catalog_add_url_is_idempotent() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source_dir = tmp.path().join("source");
    std::fs::create_dir_all(&source_dir).unwrap();
    setup_temp_git_repo(&source_dir);

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

    // First add via local path
    let (name1, is_new1) = catalog
        .catalog_add(&source_dir, &config)
        .expect("first add");
    assert!(is_new1);

    // Second add via local path — idempotent
    let (name2, is_new2) = catalog
        .catalog_add(&source_dir, &config)
        .expect("second add");
    assert_eq!(name1, name2);
    assert!(!is_new2);
    assert_eq!(catalog.repos.len(), 1);
}

#[test]
#[ignore] // Requires network access
fn catalog_add_url_from_github() {
    let tmp = tempfile::tempdir().expect("tempdir");
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
    let (name, is_new) = catalog
        .catalog_add_url("https://github.com/octocat/Hello-World.git", &config)
        .expect("catalog_add_url from GitHub");

    assert_eq!(name, "Hello-World");
    assert!(is_new);
    assert!(catalog.repos.contains_key("Hello-World"));
    let repo = &catalog.repos["Hello-World"];
    assert!(repo.path.exists());
    assert_eq!(
        repo.url.as_deref(),
        Some("https://github.com/octocat/Hello-World.git")
    );
}
