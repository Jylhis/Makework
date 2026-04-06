use std::process::Command;

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use mw_core::worktree::go;

fn setup_temp_git_repo(dir: &std::path::Path) {
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
        .expect("git config failed");
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["config", "user.name", "Test"])
        .output()
        .expect("git config failed");
    std::fs::write(dir.join("README.md"), "# Test\n").expect("write failed");
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["add", "."])
        .output()
        .expect("git add failed");
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["commit", "-m", "initial"])
        .output()
        .expect("git commit failed");
}

#[test]
fn go_default_branch_returns_existing_path() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("go-test");
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
    let name = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add should succeed");

    // go with default branch (None ref)
    let result = go(&catalog, &config, &name, None).expect("go should succeed");
    assert!(
        result.path.exists(),
        "returned path should exist: {}",
        result.path.display()
    );
    assert!(result.path.is_dir(), "returned path should be a directory");
}

#[test]
fn go_new_branch_creates_worktree() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("go-branch-test");
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
    let name = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add should succeed");

    // Create a new branch in the bare repo so git worktree add can check it out
    let repo = &catalog.repos[&name];
    let bare_path = &repo.path;
    Command::new("git")
        .args(["-C"])
        .arg(bare_path)
        .args(["branch", "feature/test"])
        .output()
        .expect("git branch failed");

    // go with explicit new branch
    let result = go(&catalog, &config, &name, Some("feature/test"))
        .expect("go with new branch should succeed");
    assert!(
        result.path.exists(),
        "worktree path should exist: {}",
        result.path.display()
    );
    assert!(result.path.is_dir(), "worktree path should be a directory");

    // Verify it's a different path than the default branch worktree
    let default_result =
        go(&catalog, &config, &name, None).expect("go with default branch should succeed");
    assert_ne!(
        result.path, default_result.path,
        "new branch path should differ from default"
    );
}
