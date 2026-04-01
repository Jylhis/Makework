use std::process::Command;

fn setup_source_repo(dir: &std::path::Path) {
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
    std::fs::write(dir.join("README.md"), "# Test\n").unwrap();
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

#[test]
fn clone_bare_from_local_path() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    let bare_dest = tmp.path().join("cloned.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare_dest)
        .expect("clone_bare should succeed");

    assert!(bare_dest.exists(), "bare clone should exist");
    assert!(bare_dest.join("HEAD").exists(), "HEAD should exist");
    assert!(
        bare_dest.join("objects").exists(),
        "objects dir should exist"
    );
}

#[test]
fn get_default_branch_returns_main() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    let bare = tmp.path().join("repo.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare).unwrap();

    let branch =
        mw_core::repository::get_default_branch(&bare).expect("get_default_branch should succeed");
    // Default branch is typically "main" or "master" depending on git config
    assert!(
        branch == "main" || branch == "master",
        "expected main or master, got: {branch}"
    );
}

#[test]
fn list_branches_includes_default() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    // Create an extra branch in source before cloning
    Command::new("git")
        .args(["-C"])
        .arg(&source)
        .args(["branch", "feature/test"])
        .output()
        .unwrap();

    let bare = tmp.path().join("repo.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare).unwrap();

    let branches =
        mw_core::repository::list_branches(&bare).expect("list_branches should succeed");
    assert!(
        branches.len() >= 2,
        "should have at least 2 branches, got: {branches:?}"
    );
    assert!(
        branches.contains(&"feature/test".to_string()),
        "should contain feature/test"
    );
}

#[test]
fn check_default_branch_changed_returns_none_when_same() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    let bare = tmp.path().join("repo.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare).unwrap();

    let default_branch = mw_core::repository::get_default_branch(&bare).unwrap();
    let changed = mw_core::repository::check_default_branch_changed(&bare, &default_branch)
        .expect("check should succeed");
    assert!(changed.is_none(), "should be None when branch is the same");
}

#[test]
fn check_default_branch_changed_returns_some_when_different() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    let bare = tmp.path().join("repo.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare).unwrap();

    let changed = mw_core::repository::check_default_branch_changed(&bare, "nonexistent-branch")
        .expect("check should succeed");
    assert!(changed.is_some(), "should be Some when branch differs");
}

#[test]
fn fetch_succeeds_on_bare_repo() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let source = tmp.path().join("source");
    setup_source_repo(&source);

    let bare = tmp.path().join("repo.git");
    mw_core::repository::clone_bare(source.to_str().unwrap(), &bare).unwrap();

    // fetch should succeed even if there's nothing new
    mw_core::repository::fetch(&bare).expect("fetch should succeed");
}

#[test]
fn clone_bare_fails_on_invalid_url() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare_dest = tmp.path().join("should-not-exist.git");

    let result = mw_core::repository::clone_bare("not-a-valid-url-or-path", &bare_dest);
    assert!(result.is_err(), "clone_bare should fail on invalid URL");
    assert!(
        !bare_dest.exists(),
        "dest should not exist after failed clone"
    );
}
