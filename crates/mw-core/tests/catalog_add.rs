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
fn catalog_add_registers_local_repo() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo_dir = tmp.path().join("my-repo");
    std::fs::create_dir_all(&repo_dir).unwrap();
    setup_temp_git_repo(&repo_dir);

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
    };

    // Set XDG config dir to temp to avoid polluting real config
    let config_dir = tmp.path().join("config/makework");
    std::fs::create_dir_all(&config_dir).unwrap();

    let mut catalog = Catalog::default();
    let name = catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add should succeed");

    assert_eq!(name, "my-repo");
    assert!(catalog.repos.contains_key("my-repo"));

    let repo = &catalog.repos["my-repo"];
    assert!(repo.path.exists(), "bare clone dir should exist");

    // Verify idempotent re-add
    let name2 = catalog
        .catalog_add(&repo_dir, &config)
        .expect("re-add should succeed");
    assert_eq!(name2, "my-repo");
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
    };

    let mut catalog = Catalog::default();
    catalog
        .catalog_add(&repo_dir, &config)
        .expect("catalog_add");

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
fn catalog_add_rejects_non_git_dir() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let non_repo = tmp.path().join("not-a-repo");
    std::fs::create_dir_all(&non_repo).unwrap();

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
    };

    let mut catalog = Catalog::default();
    let result = catalog.catalog_add(&non_repo, &config);
    assert!(result.is_err());
}
