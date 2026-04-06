use std::collections::BTreeMap;
use std::path::PathBuf;
use std::process::Command;

use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;
use mw_core::repository::Repository;
use mw_core::search::{SearchOptions, search_all};

fn setup_repo_with_files(dir: &std::path::Path) {
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
        .args(["config", "user.name", "Test"])
        .output()
        .unwrap();

    std::fs::create_dir_all(dir.join("src")).unwrap();
    std::fs::write(
        dir.join("src/main.rs"),
        "fn main() {\n    println!(\"hello\");\n}\n",
    )
    .unwrap();
    std::fs::write(
        dir.join("src/lib.rs"),
        "pub fn greet() {\n    println!(\"hello from lib\");\n}\n",
    )
    .unwrap();
    std::fs::write(
        dir.join("README.md"),
        "# My Project\nThis is a test project.\n",
    )
    .unwrap();

    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["add", "."])
        .output()
        .unwrap();
    Command::new("git")
        .args(["-C"])
        .arg(dir)
        .args(["commit", "-m", "initial"])
        .output()
        .unwrap();
}

fn setup_worktree(tmp: &std::path::Path) -> (PathBuf, PathBuf, String) {
    let source = tmp.join("source");
    setup_repo_with_files(&source);

    let bare = tmp.join("repo.git");
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source)
        .arg(&bare)
        .output()
        .expect("bare clone failed");

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();
    let wt_path = tmp
        .join("worktrees")
        .join("local")
        .join("test-repo")
        .join(&default_branch);
    mw_core::worktree::create_worktree(&bare, &default_branch, &wt_path).unwrap();

    (bare, wt_path, default_branch)
}

#[test]
fn search_all_finds_pattern_in_worktree() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path, default_branch) = setup_worktree(tmp.path());

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

    let results = search_all(&catalog, &config, "println", &SearchOptions::default());
    assert!(
        results.len() >= 2,
        "should find println in at least 2 files, got: {}",
        results.len()
    );
    assert_eq!(results[0].repo_name, "test-repo");
}

#[test]
fn search_all_with_file_glob() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path, default_branch) = setup_worktree(tmp.path());

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

    let options = SearchOptions {
        file_glob: Some("*.rs".to_string()),
        case_insensitive: false,
        max_results: None,
    };
    let results = search_all(&catalog, &config, "println", &options);
    assert!(!results.is_empty(), "should find println in .rs files");
    for r in &results {
        assert!(
            r.file_path.ends_with(".rs"),
            "all matches should be .rs files, got: {}",
            r.file_path
        );
    }
}

#[test]
fn search_all_case_insensitive() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path, default_branch) = setup_worktree(tmp.path());

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

    let options = SearchOptions {
        file_glob: None,
        case_insensitive: true,
        max_results: None,
    };
    // "MY" should match "My" in README.md due to case insensitive
    let results = search_all(&catalog, &config, "MY", &options);
    assert!(
        !results.is_empty(),
        "case-insensitive search for 'MY' should match 'My'"
    );
}

#[test]
fn search_all_max_results() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path, default_branch) = setup_worktree(tmp.path());

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

    let options = SearchOptions {
        file_glob: None,
        case_insensitive: false,
        max_results: Some(1),
    };
    let results = search_all(&catalog, &config, "println", &options);
    assert_eq!(
        results.len(),
        1,
        "should return at most 1 result with max_results=1"
    );
}

#[test]
fn search_all_no_match() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let (bare, _wt_path, default_branch) = setup_worktree(tmp.path());

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

    let results = search_all(
        &catalog,
        &config,
        "xyzzy_nonexistent_pattern",
        &SearchOptions::default(),
    );
    assert!(results.is_empty(), "should find no matches");
}

#[test]
fn search_all_empty_catalog() {
    let config = MakeworkConfig {
        worktree_root: PathBuf::from("/tmp/nonexistent"),
        bare_root: PathBuf::from("/tmp/nonexistent"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let catalog = Catalog::default();
    let results = search_all(&catalog, &config, "anything", &SearchOptions::default());
    assert!(results.is_empty());
}
