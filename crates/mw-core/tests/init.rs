use mw_core::catalog::Catalog;
use mw_core::config::MakeworkConfig;

#[test]
fn init_creates_directories() {
    let tmp = tempfile::tempdir().expect("tempdir");

    // Point XDG config to tempdir so config.toml and catalog.toml land there
    unsafe {
        std::env::set_var("XDG_CONFIG_HOME", tmp.path().join("config"));
    }

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let result = Catalog::init(&config).expect("init should succeed");

    assert!(!result.created.is_empty(), "should have created something");
    assert!(tmp.path().join("worktrees").exists());
    assert!(tmp.path().join("repos").exists());
}

#[test]
fn init_is_idempotent() {
    let tmp = tempfile::tempdir().expect("tempdir");

    unsafe {
        std::env::set_var("XDG_CONFIG_HOME", tmp.path().join("config"));
    }

    let config = MakeworkConfig {
        worktree_root: tmp.path().join("worktrees"),
        bare_root: tmp.path().join("repos"),
        scan_roots: Vec::new(),
        sync_max_depth: None,
        sync_exclude: Vec::new(),
        template_dir: None,
        resolver: None,
    };

    let result1 = Catalog::init(&config).expect("first init");
    assert!(!result1.created.is_empty());

    let result2 = Catalog::init(&config).expect("second init");
    assert!(
        result2.created.is_empty(),
        "second init should create nothing new"
    );
    assert!(
        !result2.already_existed.is_empty(),
        "second init should report existing items"
    );
}
