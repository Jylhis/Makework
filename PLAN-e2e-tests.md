# E2E Test Improvements Plan

## Context

The `mw` CLI has a solid E2E test suite in `crates/mw-cli/tests/cli_e2e.rs` (~20 tests) using a manual `Sandbox` pattern that isolates XDG directories and spawns the real binary. The tests work but have boilerplate (manual `String::from_utf8_lossy` + `.contains()` assertions, a custom `assert_success` helper) and lack multi-step workflow coverage. This plan adds `assert_cmd` + `predicates` for ergonomics, migrates existing tests, and adds missing lifecycle/edge-case tests.

**Not adding:** `trycmd` (snapshot testing) — the CLI requires dynamic temp directories for XDG isolation, which `trycmd`'s static `.toml` test files can't express. `assert_fs` — redundant with the existing `Sandbox` + `tempfile` pattern.

---

## Step 1: Add dependencies

**File:** `crates/mw-cli/Cargo.toml`

Add to `[dev-dependencies]`:
```toml
assert_cmd = "2"
predicates = "3"
```

## Step 2: Extend `Sandbox` with `assert_cmd` methods

**File:** `crates/mw-cli/tests/cli_e2e.rs`

Add two new methods alongside the existing `run`/`run_in` (keep those during migration):

```rust
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
```

## Step 3: Migrate existing tests to `assert_cmd`

**File:** `crates/mw-cli/tests/cli_e2e.rs`

Migrate all 20 tests from `sb.run(args)` + manual assertions to `sb.cmd().args([...]).assert().success()` / `.failure()` + `predicates::str::contains()`.

Example transformation:
```rust
// Before
let out = sb.run(&["go", "definitely-not-here"]);
assert!(!out.status.success(), "unknown go target should error");

// After
sb.cmd().args(["go", "definitely-not-here"])
    .assert()
    .failure()
    .stderr(predicates::str::contains("not found"));
```

After all tests are migrated, remove: `Sandbox::run()`, `Sandbox::run_in()`, `assert_success()`, `mw_bin()`.

## Step 4: Add workflow/lifecycle tests

**File:** `crates/mw-cli/tests/cli_e2e.rs`

### Priority 1 — Core lifecycles

1. **`full_add_go_new_ls_rm_purge_lifecycle`** — Chain: `catalog init` -> `catalog add` -> `go <repo>` (prints path) -> `new <repo> feature/x` (worktree dir exists) -> `ls` (both branches listed) -> `rm <repo>/feature/x` -> `ls` (feature gone) -> `catalog purge` (bare clone removed) -> `catalog list` (repo gone)

2. **`sync_discovers_repos_then_go_works`** — `catalog init` -> create 2 repos under scan root -> `config set scan_roots` -> `sync` (both appear) -> `catalog list` (both registered) -> `go <discovered>`

3. **`config_round_trip_with_file_verification`** — `catalog init` -> `config show` (check defaults) -> `config set worktree_root /custom` -> `config show` (verify new value + source `config-file`) -> read TOML file on disk to verify

### Priority 2 — Edge cases and error paths

4. **`double_add_is_idempotent`** — `catalog add` same repo twice; second should say "Already registered"

5. **`operations_without_catalog_init`** — `catalog list` and `go someproject` without `catalog init`; verify clear error messages

6. **`remove_nonexistent_project`** — `catalog remove nonexistent` -> exit 1, stderr contains "not found"

7. **`visit_updates_frecency`** — `catalog init` -> add repo -> `visit <repo-path>` -> `resolver explain <repo>` -> verify score is nonzero

8. **`ls_prune_orphaned_worktree`** — `catalog init` -> add repo -> `new <repo> orphan` -> manually delete worktree dir on disk -> `ls` (shows orphaned marker) -> `ls --prune` (reports pruned)

9. **`search_no_matches`** — `search "xyzzy_not_found"` -> success exit code, output indicates no matches

10. **`query_author_filter`** — Repo with commits from "Test" author -> `query --author Test` finds them -> `query --author Nobody` finds nothing

### Priority 3 — Output format checks

11. **`catalog_list_table_headers`** — Verify header contains `NAME`, `BRANCH`, `WORKTREES`
12. **`config_show_table_headers`** — Verify header contains `SETTING`, `VALUE`, `SOURCE`
13. **`default_status_no_repos`** — `mw` with no args and empty catalog; verify "No repositories" message
14. **`default_status_with_repos`** — `mw` with repos registered; verify repo names appear

---

## Files to modify

| File | Change |
|------|--------|
| `crates/mw-cli/Cargo.toml` | Add `assert_cmd`, `predicates` dev-deps |
| `crates/mw-cli/tests/cli_e2e.rs` | Add `cmd()`/`cmd_in()`, migrate tests, add new tests |

## Reference files (read-only)

- `crates/mw-cli/src/commands/mod.rs` — all error messages use `die()` which prints `Error: {msg}` to stderr, exit 1
- `crates/mw-cli/src/lib.rs` — clap `Cli` and `Command` definitions
- `crates/mw-core/tests/common/mod.rs` — `setup_temp_git_repo` pattern (E2E tests have their own `make_test_repo`)

## Verification

```sh
cargo test -p mw-cli           # all E2E tests pass
cargo test                     # full workspace still green
cargo clippy --all-targets -- -D warnings  # no new warnings
treefmt                        # formatting clean
```
