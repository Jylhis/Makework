mod common;
use common::setup_temp_git_repo;

#[test]
fn get_worktree_status_clean_repo() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo = tmp.path().join("clean-repo");
    std::fs::create_dir_all(&repo).unwrap();
    setup_temp_git_repo(&repo);

    let status = mw_core::status::get_worktree_status(&repo);
    assert!(!status.is_orphaned);
    assert_eq!(status.dirty_count, 0);
    assert!(
        status.branch == "main" || status.branch == "master",
        "expected main or master, got: {}",
        status.branch
    );
}

#[test]
fn get_worktree_status_dirty_repo() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let repo = tmp.path().join("dirty-repo");
    std::fs::create_dir_all(&repo).unwrap();
    setup_temp_git_repo(&repo);

    // Create untracked and modified files
    std::fs::write(repo.join("new-file.txt"), "new content").unwrap();
    std::fs::write(repo.join("README.md"), "modified").unwrap();

    let status = mw_core::status::get_worktree_status(&repo);
    assert!(!status.is_orphaned);
    assert!(
        status.dirty_count >= 2,
        "should have at least 2 dirty files, got: {}",
        status.dirty_count
    );
}

#[test]
fn get_worktree_status_orphaned_path() {
    let status =
        mw_core::status::get_worktree_status(std::path::Path::new("/nonexistent/worktree/path"));
    assert!(status.is_orphaned);
    assert_eq!(status.branch, "unknown");
}
