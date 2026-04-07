use std::collections::BTreeMap;
use std::path::PathBuf;
use std::process::Command;

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use mw_core::repository::Repository;
use mw_core::worktree::{create_worktree, list_all_worktrees, list_worktrees, remove_worktree};

fn setup_bare_repo(tmp: &std::path::Path) -> PathBuf {
    let source = tmp.join("source");
    std::fs::create_dir_all(&source).unwrap();
    Command::new("git")
        .args(["init"])
        .arg(&source)
        .output()
        .expect("git init failed");
    Command::new("git")
        .args(["-C"])
        .arg(&source)
        .args(["config", "user.email", "test@test.com"])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source)
        .args(["config", "user.name", "Test"])
        .output()
        .unwrap();
    std::fs::write(source.join("README.md"), "# Test\n").unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source)
        .args(["add", "."])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source)
        .args(["commit", "-m", "initial"])
        .output()
        .unwrap();

    let bare = tmp.join("repo.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source)
        .arg(&bare)
        .output()
        .expect("git clone --bare failed");
    bare
}

#[test]
fn create_and_list_worktree() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare = setup_bare_repo(tmp.path());

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();

    let wt_path = tmp.path().join("worktrees").join("main");
    create_worktree(&bare, &default_branch, &wt_path).expect("create_worktree should succeed");

    assert!(wt_path.exists(), "worktree directory should exist");
    assert!(
        wt_path.join("README.md").exists(),
        "README.md should be in worktree"
    );

    let worktrees = list_worktrees(&bare).expect("list_worktrees should succeed");
    // Should have the bare entry + our new worktree
    assert!(
        worktrees.len() >= 2,
        "should have at least 2 entries (bare + worktree), got: {}",
        worktrees.len()
    );

    let non_bare: Vec<_> = worktrees.iter().filter(|wt| !wt.is_bare).collect();
    assert_eq!(non_bare.len(), 1, "should have exactly 1 non-bare worktree");
    // Canonicalize both sides — on macOS, tempdir returns /var/... but git resolves
    // it to /private/var/..., so the raw paths can't be compared directly.
    assert_eq!(
        non_bare[0].path.canonicalize().unwrap(),
        wt_path.canonicalize().unwrap()
    );
}

#[test]
fn remove_worktree_cleans_up() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare = setup_bare_repo(tmp.path());
    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();

    let wt_path = tmp.path().join("worktrees").join("to-remove");
    create_worktree(&bare, &default_branch, &wt_path).expect("create should succeed");
    assert!(wt_path.exists());

    remove_worktree(&bare, &wt_path).expect("remove_worktree should succeed");
    assert!(!wt_path.exists(), "worktree should be removed");

    let worktrees = list_worktrees(&bare).unwrap();
    let non_bare: Vec<_> = worktrees.iter().filter(|wt| !wt.is_bare).collect();
    assert!(
        non_bare.is_empty(),
        "no non-bare worktrees should remain after removal"
    );
}

#[test]
fn create_worktree_with_new_branch() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare = setup_bare_repo(tmp.path());

    // Create a new branch in the bare repo
    Command::new("git")
        .args(["-C"])
        .arg(&bare)
        .args(["branch", "feature/new"])
        .output()
        .unwrap();

    let wt_path = tmp.path().join("worktrees").join("feature-new");
    create_worktree(&bare, "feature/new", &wt_path).expect("create with new branch should work");

    assert!(wt_path.exists());
    let worktrees = list_worktrees(&bare).unwrap();
    let feature_wt = worktrees
        .iter()
        .find(|wt| wt.branch.as_deref() == Some("feature/new"));
    assert!(feature_wt.is_some(), "should find feature/new worktree");
}

#[test]
fn list_all_worktrees_across_repos() {
    let tmp = tempfile::tempdir().expect("tempdir");

    // Create two bare repos
    let source1 = tmp.path().join("src1");
    std::fs::create_dir_all(&source1).unwrap();
    Command::new("git")
        .args(["init"])
        .arg(&source1)
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source1)
        .args(["config", "user.email", "t@t.com"])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source1)
        .args(["config", "user.name", "T"])
        .output()
        .unwrap();
    std::fs::write(source1.join("f.txt"), "a").unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source1)
        .args(["add", "."])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(&source1)
        .args(["commit", "-m", "init"])
        .output()
        .unwrap();

    let bare1 = tmp.path().join("r1.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source1)
        .arg(&bare1)
        .output()
        .unwrap();

    let bare2 = tmp.path().join("r2.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source1)
        .arg(&bare2)
        .output()
        .unwrap();

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
        "repo1".to_string(),
        Repository {
            name: "repo1".to_string(),
            path: bare1,
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::new(),
        },
    );
    catalog.repos.insert(
        "repo2".to_string(),
        Repository {
            name: "repo2".to_string(),
            path: bare2,
            url: None,
            main_branch: "main".to_string(),
            remotes: BTreeMap::new(),
            projects: BTreeMap::new(),
        },
    );

    let all = list_all_worktrees(&catalog, &config);
    assert_eq!(all.len(), 2, "should have entries for both repos");
    // Each bare repo should have at least the bare worktree entry
    for (_name, worktrees) in &all {
        assert!(
            !worktrees.is_empty(),
            "each repo should have at least one worktree entry"
        );
    }
}

#[test]
fn create_worktree_fails_for_nonexistent_branch() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare = setup_bare_repo(tmp.path());

    let wt_path = tmp.path().join("worktrees").join("nonexistent");
    let result = create_worktree(&bare, "nonexistent-branch", &wt_path);
    assert!(
        result.is_err(),
        "should fail for a branch that doesn't exist"
    );
}
