use std::process::Command;

use mw_core::catalog::{Catalog, SyncOptions};
use mw_core::config::MakeworkConfig;

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
fn sync_discovers_git_repos() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");
    std::fs::create_dir_all(&scan_root).unwrap();

    // Create two git repos
    let repo_a = scan_root.join("repo-a");
    let repo_b = scan_root.join("repo-b");
    std::fs::create_dir_all(&repo_a).unwrap();
    std::fs::create_dir_all(&repo_b).unwrap();
    setup_temp_git_repo(&repo_a);
    setup_temp_git_repo(&repo_b);

    // Create a non-git directory
    let not_repo = scan_root.join("not-a-repo");
    std::fs::create_dir_all(&not_repo).unwrap();
    std::fs::write(not_repo.join("file.txt"), "hello").unwrap();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };
    let mut catalog = Catalog::default();

    let added = catalog
        .sync(
            &config,
            std::slice::from_ref(&scan_root),
            &SyncOptions::default(),
        )
        .unwrap();
    assert_eq!(added.len(), 2, "expected 2 repos, got: {added:?}");
    assert!(added.contains(&"repo-a".to_string()));
    assert!(added.contains(&"repo-b".to_string()));
}

#[test]
fn sync_is_idempotent() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");
    std::fs::create_dir_all(&scan_root).unwrap();

    let repo = scan_root.join("my-repo");
    std::fs::create_dir_all(&repo).unwrap();
    setup_temp_git_repo(&repo);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };
    let mut catalog = Catalog::default();

    let first = catalog
        .sync(
            &config,
            std::slice::from_ref(&scan_root),
            &SyncOptions::default(),
        )
        .unwrap();
    assert_eq!(first.len(), 1);

    // Second run: should not add any new repos
    let second = catalog
        .sync(&config, &[scan_root], &SyncOptions::default())
        .unwrap();
    assert_eq!(
        second.len(),
        1,
        "idempotent sync should still return the same repo since catalog_add is idempotent"
    );
    assert_eq!(
        catalog.repos.len(),
        1,
        "catalog should still have exactly 1 repo"
    );
}

#[test]
fn sync_respects_max_depth() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");

    // Create a repo at depth 2: scan_root/group/nested-repo/
    let nested = scan_root.join("group").join("nested-repo");
    std::fs::create_dir_all(&nested).unwrap();
    setup_temp_git_repo(&nested);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    // depth 1: should NOT find repo at depth 2
    let mut catalog = Catalog::default();
    let added = catalog
        .sync(
            &config,
            std::slice::from_ref(&scan_root),
            &SyncOptions {
                max_depth: 1,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(added.len(), 0, "depth 1 should not find nested repo");

    // depth 2: should find it
    let added = catalog
        .sync(
            &config,
            std::slice::from_ref(&scan_root),
            &SyncOptions {
                max_depth: 2,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(added.len(), 1, "depth 2 should find nested repo");
    assert!(added.contains(&"nested-repo".to_string()));
}

#[test]
fn sync_excludes_patterns() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");

    // Create a good repo at depth 1
    let good = scan_root.join("good-repo");
    std::fs::create_dir_all(&good).unwrap();
    setup_temp_git_repo(&good);

    // Create a repo hidden inside node_modules at depth 2
    let hidden = scan_root.join("node_modules").join("hidden-repo");
    std::fs::create_dir_all(&hidden).unwrap();
    setup_temp_git_repo(&hidden);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let mut catalog = Catalog::default();
    let added = catalog
        .sync(
            &config,
            &[scan_root],
            &SyncOptions {
                max_depth: 2,
                exclude: vec!["node_modules".to_string()],
            },
        )
        .unwrap();
    assert_eq!(added.len(), 1, "should only find good-repo");
    assert!(added.contains(&"good-repo".to_string()));
}

#[test]
fn sync_skips_submodules() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");

    // Create parent repo
    let parent = scan_root.join("parent");
    std::fs::create_dir_all(&parent).unwrap();
    setup_temp_git_repo(&parent);

    // Simulate a submodule: create child dir with .git as a FILE (not dir)
    let child = scan_root.join("parent").join("vendor").join("child");
    std::fs::create_dir_all(&child).unwrap();
    std::fs::write(child.join(".git"), "gitdir: ../../../.git/modules/child\n").unwrap();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let mut catalog = Catalog::default();
    let added = catalog
        .sync(&config, &[scan_root], &SyncOptions::default())
        .unwrap();
    assert!(
        added.contains(&"parent".to_string()),
        "should find parent repo"
    );
    assert!(
        !added.iter().any(|n| n == "child"),
        "should NOT register submodule child"
    );
}

#[test]
fn sync_skips_bare_repos() {
    let tmp = tempfile::tempdir().unwrap();
    let scan_root = tmp.path().join("projects");

    // Create a normal repo
    let normal = scan_root.join("normal-repo");
    std::fs::create_dir_all(&normal).unwrap();
    setup_temp_git_repo(&normal);

    // Create a bare repo: has HEAD + objects/ but no .git
    let bare = scan_root.join("bare.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&normal)
        .arg(&bare)
        .output()
        .expect("git clone --bare failed");

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("bare"),
        scan_roots: vec![scan_root.clone()],
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let mut catalog = Catalog::default();
    let added = catalog
        .sync(&config, &[scan_root], &SyncOptions::default())
        .unwrap();
    assert!(
        added.contains(&"normal-repo".to_string()),
        "should find normal repo"
    );
    assert!(
        !added.iter().any(|n| n.contains("bare")),
        "should NOT register bare repo"
    );
}
