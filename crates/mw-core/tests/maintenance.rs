use std::process::Command;

fn setup_bare_repo(dir: &std::path::Path) {
    let source = dir.parent().unwrap().join("source-repo");
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
    Command::new("git")
        .args(["clone", "--bare"])
        .arg(&source)
        .arg(dir)
        .output()
        .expect("git clone --bare failed");
}

/// Tests the full maintenance lifecycle: status (unregistered) → register →
/// status (registered) → unregister → status (unregistered).
///
/// Consolidated into a single test because `git maintenance register` writes
/// to GIT_CONFIG_GLOBAL, and setting that env var races if tests run in parallel.
#[test]
fn maintenance_lifecycle() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let bare = tmp.path().join("test.git");
    setup_bare_repo(&bare);

    // Point GIT_CONFIG_GLOBAL to a writable file inside our temp dir so
    // `git maintenance register` doesn't try to write to a read-only system config.
    let global_cfg = tmp.path().join("gitconfig");
    std::fs::write(&global_cfg, "").unwrap();
    // SAFETY: consolidated into one test to avoid env-var races between tests.
    unsafe {
        std::env::set_var("GIT_CONFIG_GLOBAL", &global_cfg);
    }

    // Initially not registered
    let is_registered =
        mw_core::maintenance::maintenance_status(&bare).expect("status should succeed");
    assert!(
        !is_registered,
        "fresh repo should not be registered for maintenance"
    );

    // Register
    mw_core::maintenance::maintenance_register(&bare).expect("register should succeed");
    let is_registered =
        mw_core::maintenance::maintenance_status(&bare).expect("status should succeed");
    assert!(is_registered, "repo should be registered after register");

    // Unregister
    mw_core::maintenance::maintenance_unregister(&bare).expect("unregister should succeed");
    let is_registered =
        mw_core::maintenance::maintenance_status(&bare).expect("status should succeed");
    assert!(
        !is_registered,
        "repo should not be registered after unregister"
    );

    unsafe {
        std::env::remove_var("GIT_CONFIG_GLOBAL");
    }
}
