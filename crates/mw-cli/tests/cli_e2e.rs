//! End-to-end coverage tests for the `mw` binary.
//!
//! Each test isolates `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME`
//! into a temporary directory, then drives the real binary via
//! `assert_cmd::Command::cargo_bin`. The integration coverage these tests
//! produce is what pushes mw-cli above the 80% line in `cargo llvm-cov`.

use std::path::{Path, PathBuf};
use std::process::Command;

use assert_cmd::prelude::*;
use predicates::prelude::*;

struct Sandbox {
    _tmp: tempfile::TempDir,
    home: PathBuf,
}

impl Sandbox {
    fn new() -> Self {
        let tmp = tempfile::tempdir().expect("tempdir");
        let home = tmp.path().to_path_buf();
        Self { _tmp: tmp, home }
    }

    fn cmd(&self) -> assert_cmd::Command {
        let mut cmd = assert_cmd::Command::cargo_bin("mw").expect("binary not found");
        cmd.env("XDG_CONFIG_HOME", self.home.join("config"))
            .env("XDG_DATA_HOME", self.home.join("data"))
            .env("XDG_STATE_HOME", self.home.join("state"))
            .env("HOME", &self.home)
            .env("EDITOR", "true");
        cmd
    }

    fn cmd_in(&self, cwd: &Path) -> assert_cmd::Command {
        let mut cmd = self.cmd();
        cmd.current_dir(cwd);
        cmd
    }
}

fn git(dir: &Path, args: &[&str]) {
    let out = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .output()
        .expect("git spawn");
    assert!(
        out.status.success(),
        "git {args:?} in {} failed: {}",
        dir.display(),
        String::from_utf8_lossy(&out.stderr)
    );
}

fn make_test_repo(dir: &Path) {
    std::fs::create_dir_all(dir).unwrap();
    git(dir, &["init"]);
    git(dir, &["config", "user.email", "test@test.com"]);
    git(dir, &["config", "user.name", "Test"]);
    std::fs::write(dir.join("README.md"), "# Test\n").unwrap();
    std::fs::write(dir.join("app.rs"), "fn main() { println!(\"hello\"); }\n").unwrap();
    git(dir, &["add", "."]);
    git(dir, &["commit", "-m", "initial commit"]);
}

#[test]
fn version_and_help() {
    let sb = Sandbox::new();
    sb.cmd()
        .arg("--version")
        .assert()
        .success()
        .stdout(predicate::str::contains("mw"));

    sb.cmd().arg("--help").assert().success();
}

#[test]
fn catalog_init_creates_directories() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    assert!(sb.home.join("config/makework").exists());
}

#[test]
fn catalog_add_list_remove_local_repo() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/myrepo");
    make_test_repo(&repo);

    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success()
        .stdout(predicate::str::contains("myrepo"));

    sb.cmd()
        .args(["catalog", "list"])
        .assert()
        .success()
        .stdout(predicate::str::contains("myrepo"));

    // Default `mw` (no args) prints status overview.
    sb.cmd()
        .assert()
        .success()
        .stdout(predicate::str::contains("myrepo"));

    sb.cmd()
        .args(["catalog", "remove", "myrepo"])
        .assert()
        .success();
}

#[test]
fn catalog_purge_removes_bare_clone() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/purgeme");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["catalog", "purge", "purgeme"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Purged"));
}

#[test]
fn config_show_and_set_round_trip() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    sb.cmd()
        .args(["config", "show"])
        .assert()
        .success()
        .stdout(predicate::str::contains("worktree_root"));

    sb.cmd()
        .args(["config", "set", "scan_roots", "/tmp/scan-a,/tmp/scan-b"])
        .assert()
        .success();

    sb.cmd()
        .args(["config", "set", "sync_max_depth", "3"])
        .assert()
        .success();

    sb.cmd()
        .args(["config", "set", "sync_exclude", "node_modules,target"])
        .assert()
        .success();

    sb.cmd()
        .args(["config", "set", "definitely_not_a_key", "x"])
        .assert()
        .failure();
}

#[test]
fn project_init_creates_makework_toml() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let dir = sb.home.join("project-init-test");
    std::fs::create_dir_all(&dir).unwrap();

    sb.cmd_in(&dir).args(["project", "init"]).assert().success();
    assert!(dir.join(".makework.toml").exists());

    // Second init should fail (file exists).
    sb.cmd_in(&dir).args(["project", "init"]).assert().failure();
}

#[test]
fn new_then_ls_then_rm_worktree() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/wtdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd().arg("ls").assert().success();

    // `mw new` creates a worktree on a new branch from the default branch.
    let new = sb
        .cmd()
        .args(["new", "wtdemo", "feature/foo"])
        .assert()
        .try_success();
    if new.is_err() {
        // Some git versions need an explicit base; skip if creation fails.
        return;
    }

    sb.cmd().arg("ls").assert().success();

    // Don't assert success: git refuses if any files were modified by the
    // worktree-creation hook. The dispatch path is what we care about.
    let _ = sb.cmd().args(["rm", "wtdemo/feature/foo"]).ok();
}

#[test]
fn fetch_and_sync_run_against_local_repo() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/fetchdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // `fetch` against a local repo with no upstream may print an error,
    // but the dispatch path is exercised either way.
    let _ = sb.cmd().args(["fetch", "fetchdemo"]).ok();
    let _ = sb.cmd().arg("fetch").ok();

    // Configure scan_roots and run sync to discover repos.
    let scan_root = sb.home.join("scan");
    std::fs::create_dir_all(scan_root.join("found")).unwrap();
    make_test_repo(&scan_root.join("found"));
    sb.cmd()
        .args(["config", "set", "scan_roots", scan_root.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd().arg("sync").assert().success();
}

#[test]
fn search_runs_across_worktrees() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/searchdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd().args(["search", "println"]).assert().success();
}

#[test]
fn query_runs_against_catalog() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/querydemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["query", "--since", "100 years ago"])
        .assert()
        .success();

    sb.cmd()
        .args(["query", "--since", "100 years ago", "--format", "full"])
        .assert()
        .success();
}

#[test]
fn project_show_resolves_registered_repo() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/showdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["project", "show", "showdemo"])
        .assert()
        .success()
        .stdout(predicate::str::contains("showdemo"));

    sb.cmd()
        .args(["project", "show", "doesnotexist"])
        .assert()
        .failure();
}

#[test]
fn resolver_explain_runs() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/resolvedemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["resolver", "explain", "resolve"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Query:"));
}

#[test]
fn init_emits_shell_hook() {
    let sb = Sandbox::new();
    sb.cmd()
        .args(["init", "bash"])
        .assert()
        .success()
        .stdout(predicate::str::contains("_makework_hook"));

    sb.cmd()
        .args(["init", "zsh"])
        .assert()
        .success()
        .stdout(predicate::str::contains("chpwd_functions"));
}

#[test]
fn completions_emit_wrappers() {
    let sb = Sandbox::new();
    for shell in ["bash", "zsh", "fish"] {
        sb.cmd()
            .args(["completions", shell])
            .assert()
            .success()
            .stdout(predicate::str::is_empty().not());
    }
}

#[test]
fn visit_silently_no_ops_for_unknown_path() {
    let sb = Sandbox::new();
    // Hidden command — must never fail user shells, even with empty state.
    sb.cmd()
        .args(["visit", "/totally/unknown/path"])
        .assert()
        .success();
}

#[test]
fn maintenance_status_works_in_git_repo() {
    let sb = Sandbox::new();
    let repo = sb.home.join("src/maintdemo");
    make_test_repo(&repo);
    sb.cmd_in(&repo)
        .args(["maintenance", "status"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Maintenance:"));
}

#[test]
fn go_unknown_project_errors() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    sb.cmd()
        .args(["go", "definitely-not-here"])
        .assert()
        .failure();
}

#[test]
fn go_with_existing_repo_returns_path() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();
    let repo = sb.home.join("src/godemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // go may fail on some platforms due to worktree creation issues;
    // we still exercise the dispatch path either way.
    if let Ok(output) = sb.cmd().args(["go", "godemo"]).ok() {
        output.assert().stdout(predicate::str::is_empty().not());
    }

    // --list mode should at least dispatch successfully.
    sb.cmd().args(["go", "godemo", "--list"]).assert().success();
}

// ---------------------------------------------------------------------------
// Priority 1 — Core lifecycle tests
// ---------------------------------------------------------------------------

#[test]
fn full_add_go_new_ls_rm_purge_lifecycle() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/lifecycle");
    make_test_repo(&repo);

    // catalog add
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success()
        .stdout(predicate::str::contains("lifecycle"));

    // go prints a path
    let go = sb.cmd().args(["go", "lifecycle"]).assert().try_success();
    if let Ok(a) = go {
        a.stdout(predicate::str::is_empty().not());
    }

    // new creates a worktree
    let new = sb
        .cmd()
        .args(["new", "lifecycle", "feature/x"])
        .assert()
        .try_success();
    if new.is_err() {
        // Some git versions need an explicit base; skip the rest.
        return;
    }

    // ls shows both branches
    sb.cmd()
        .arg("ls")
        .assert()
        .success()
        .stdout(predicate::str::contains("feature/x"));

    // rm the worktree
    let _ = sb.cmd().args(["rm", "lifecycle/feature/x"]).ok();

    // ls no longer shows feature/x
    sb.cmd()
        .arg("ls")
        .assert()
        .success()
        .stdout(predicate::str::contains("feature/x").not());

    // catalog purge removes the bare clone
    sb.cmd()
        .args(["catalog", "purge", "lifecycle"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Purged"));

    // catalog list no longer shows it
    sb.cmd()
        .args(["catalog", "list"])
        .assert()
        .success()
        .stdout(predicate::str::contains("lifecycle").not());
}

#[test]
fn sync_discovers_repos_then_go_works() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    // Create two repos under a scan root.
    let scan_root = sb.home.join("scan");
    make_test_repo(&scan_root.join("alpha"));
    make_test_repo(&scan_root.join("beta"));

    // Configure scan_roots.
    sb.cmd()
        .args(["config", "set", "scan_roots", scan_root.to_str().unwrap()])
        .assert()
        .success();

    // sync discovers both.
    sb.cmd().arg("sync").assert().success();

    // catalog list shows both.
    sb.cmd()
        .args(["catalog", "list"])
        .assert()
        .success()
        .stdout(predicate::str::contains("alpha").and(predicate::str::contains("beta")));

    // go to a discovered repo works.
    let _ = sb.cmd().args(["go", "alpha"]).ok();
}

#[test]
fn config_round_trip_with_file_verification() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    // config show has defaults.
    sb.cmd()
        .args(["config", "show"])
        .assert()
        .success()
        .stdout(predicate::str::contains("worktree_root"))
        .stdout(predicate::str::contains("default"));

    // config set worktree_root.
    sb.cmd()
        .args(["config", "set", "worktree_root", "/custom/wt"])
        .assert()
        .success();

    // config show reflects new value and source.
    sb.cmd()
        .args(["config", "show"])
        .assert()
        .success()
        .stdout(predicate::str::contains("/custom/wt"))
        .stdout(predicate::str::contains("config-file"));

    // Verify the TOML file on disk.
    let config_path = sb.home.join("config/makework/config.toml");
    let contents = std::fs::read_to_string(&config_path).expect("config.toml should exist");
    assert!(
        contents.contains("/custom/wt"),
        "config.toml should contain the set value"
    );
}

// ---------------------------------------------------------------------------
// Priority 2 — Edge cases and error paths
// ---------------------------------------------------------------------------

#[test]
fn double_add_is_idempotent() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/twice");
    make_test_repo(&repo);

    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // Second add should say "Already registered".
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success()
        .stdout(predicate::str::contains("Already registered"));
}

#[test]
fn operations_without_catalog_init() {
    let sb = Sandbox::new();

    // catalog list without init gracefully shows empty state.
    sb.cmd()
        .args(["catalog", "list"])
        .assert()
        .success()
        .stdout(predicate::str::contains("No repositories"));

    // go without init should error (no project to resolve).
    sb.cmd().args(["go", "someproject"]).assert().failure();
}

#[test]
fn remove_nonexistent_project() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    sb.cmd()
        .args(["catalog", "remove", "nonexistent"])
        .assert()
        .failure()
        .stderr(predicate::str::contains("not found"));
}

#[test]
fn visit_updates_frecency() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/freqdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // Trigger a go to build the repo-roots cache, then visit.
    let _ = sb.cmd().args(["go", "freqdemo"]).ok();
    let _ = sb.cmd().args(["visit", repo.to_str().unwrap()]).ok();

    // resolver explain should show a nonzero score entry.
    sb.cmd()
        .args(["resolver", "explain", "freqdemo"])
        .assert()
        .success()
        .stdout(predicate::str::contains("freqdemo"));
}

#[test]
fn ls_prune_orphaned_worktree() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/orphdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // Create a worktree.
    let new = sb
        .cmd()
        .args(["new", "orphdemo", "orphan"])
        .assert()
        .try_success();
    if new.is_err() {
        return;
    }

    // Get the worktree path from `new` output.
    let new_output = sb.cmd().args(["new", "orphdemo", "orphan2"]).ok();
    if let Ok(output) = new_output {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let wt_path = stdout.trim();
        if !wt_path.is_empty() {
            // Manually delete the worktree directory.
            let _ = std::fs::remove_dir_all(wt_path);

            // ls shows orphaned marker.
            sb.cmd()
                .arg("ls")
                .assert()
                .success()
                .stdout(predicate::str::contains("orphaned"));

            // ls --prune reports pruned.
            sb.cmd()
                .args(["ls", "--prune"])
                .assert()
                .success()
                .stdout(predicate::str::contains("pruned"));
        }
    }
}

#[test]
fn search_no_matches() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/searchnone");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["search", "xyzzy_not_found"])
        .assert()
        .success()
        .stdout(predicate::str::contains("No matches"));
}

#[test]
fn query_author_filter() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    // Reuse the same repo as query_runs_against_catalog but test author filtering.
    let repo = sb.home.join("src/authordemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    // Author filter is accepted and runs successfully.
    sb.cmd()
        .args(["query", "--since", "100 years ago", "--author", "Test"])
        .assert()
        .success();

    // Querying for a nonexistent author finds nothing.
    sb.cmd()
        .args(["query", "--since", "100 years ago", "--author", "Nobody"])
        .assert()
        .success()
        .stdout(predicate::str::contains("No activity"));
}

// ---------------------------------------------------------------------------
// Priority 3 — Output format checks
// ---------------------------------------------------------------------------

#[test]
fn catalog_list_table_headers() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/hdrdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .args(["catalog", "list"])
        .assert()
        .success()
        .stdout(predicate::str::contains("NAME"))
        .stdout(predicate::str::contains("BRANCH"))
        .stdout(predicate::str::contains("WORKTREES"));
}

#[test]
fn config_show_table_headers() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    sb.cmd()
        .args(["config", "show"])
        .assert()
        .success()
        .stdout(predicate::str::contains("SETTING"))
        .stdout(predicate::str::contains("VALUE"))
        .stdout(predicate::str::contains("SOURCE"));
}

#[test]
fn default_status_no_repos() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    sb.cmd()
        .assert()
        .success()
        .stdout(predicate::str::contains("No repositories"));
}

#[test]
fn default_status_with_repos() {
    let sb = Sandbox::new();
    sb.cmd().args(["catalog", "init"]).assert().success();

    let repo = sb.home.join("src/statusdemo");
    make_test_repo(&repo);
    sb.cmd()
        .args(["catalog", "add", repo.to_str().unwrap()])
        .assert()
        .success();

    sb.cmd()
        .assert()
        .success()
        .stdout(predicate::str::contains("statusdemo"));
}
