use std::process::Command;

use mw_core::catalog::Catalog;
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
    };
    let mut catalog = Catalog::default();

    let added = catalog
        .sync(&config, std::slice::from_ref(&scan_root))
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
    };
    let mut catalog = Catalog::default();

    let first = catalog
        .sync(&config, std::slice::from_ref(&scan_root))
        .unwrap();
    assert_eq!(first.len(), 1);

    // Second run: should not add any new repos
    let second = catalog.sync(&config, &[scan_root]).unwrap();
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
