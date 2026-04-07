//! Shared test fixtures.
//!
//! Each integration test file lives in its own crate, so this module is
//! included with `mod common;` rather than imported as a library item.

use std::path::Path;
use std::process::Command;

/// Initialize a git repository at `dir` with one initial commit.
///
/// Creates the directory if missing, configures a dummy user, and commits a
/// `README.md` so the repo has a HEAD ref usable by worktree operations.
#[allow(dead_code)] // not all test files use every helper
pub fn setup_temp_git_repo(dir: &Path) {
    if !dir.exists() {
        std::fs::create_dir_all(dir).expect("create_dir_all failed");
    }
    git(dir, &["init"]);
    git(dir, &["config", "user.email", "test@test.com"]);
    git(dir, &["config", "user.name", "Test"]);
    std::fs::write(dir.join("README.md"), "# Test\n").expect("write README failed");
    git(dir, &["add", "."]);
    git(dir, &["commit", "-m", "initial"]);
}

fn git(dir: &Path, args: &[&str]) {
    let status = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .output()
        .unwrap_or_else(|e| panic!("git {args:?} failed to spawn: {e}"));
    assert!(
        status.status.success(),
        "git {args:?} in {} failed: {}",
        dir.display(),
        String::from_utf8_lossy(&status.stderr)
    );
}
