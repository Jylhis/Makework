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
fn prune_no_orphans_returns_zero() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("prune-test");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
    };

    let mut catalog = Catalog::default();
    catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add");

    let repo = catalog.repos.values().next().unwrap();
    let count = mw_core::worktree::prune_worktrees(&repo.path).expect("prune should succeed");
    assert_eq!(count, 0);
}

#[test]
fn prune_cleans_orphaned_worktree() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("prune-orphan");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

    // Create a branch in the source repo
    Command::new("git")
        .args(["-C"])
        .arg(&repo_dir)
        .args(["checkout", "-b", "feature-branch"])
        .output()
        .expect("git checkout failed");
    Command::new("git")
        .args(["-C"])
        .arg(&repo_dir)
        .args(["checkout", "master"])
        .output()
        .ok(); // may be "main" instead
    Command::new("git")
        .args(["-C"])
        .arg(&repo_dir)
        .args(["checkout", "main"])
        .output()
        .ok();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
    };

    let mut catalog = Catalog::default();
    catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add");

    let repo = catalog.repos.values().next().unwrap();

    // Create an extra worktree
    let extra_wt = tmp
        .path()
        .join("worktrees/local/prune-orphan/feature-branch");
    let _ = mw_core::worktree::create_worktree(&repo.path, "feature-branch", &extra_wt);

    // Verify it exists
    let before = mw_core::worktree::list_worktrees(&repo.path).expect("list");
    let non_bare_before = before.iter().filter(|wt| !wt.is_bare).count();
    assert!(
        non_bare_before >= 2,
        "should have at least 2 non-bare worktrees"
    );

    // Delete the worktree directory to make it orphaned
    std::fs::remove_dir_all(&extra_wt).expect("remove dir");

    // Prune should detect and clean the orphan
    let count = mw_core::worktree::prune_worktrees(&repo.path).expect("prune");
    assert_eq!(count, 1);

    // After pruning, list_worktrees should no longer show the orphaned entry
    let after = mw_core::worktree::list_worktrees(&repo.path).expect("list after prune");
    let non_bare_after = after.iter().filter(|wt| !wt.is_bare).count();
    assert_eq!(non_bare_after, non_bare_before - 1);
}
