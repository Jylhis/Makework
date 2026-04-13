# TODO

Each phase follows TDD: write failing tests first, then implement until green, then validate.
Test files live in `crates/mw-core/tests/<feature>.rs` (integration) and inline `#[cfg(test)]` (unit).
Shared helper `setup_temp_git_repo()` is duplicated per test file (existing convention).

---

## Phase 9 — `mw sync` auto-discovery with heuristics ✅

### Step 1: Write failing tests (RED)

#### Unit tests — `catalog.rs` inline `#[cfg(test)]`
- [x] `sync_options_default_values` — `SyncOptions::default()` has `max_depth: 1` and empty `exclude`

#### Integration tests — `crates/mw-core/tests/sync.rs` (extend existing file)

- [x] `sync_respects_max_depth`
- [x] `sync_excludes_patterns`
- [x] `sync_skips_submodules`
- [x] `sync_skips_bare_repos`
- [x] Update existing `sync_discovers_git_repos` and `sync_is_idempotent` to pass `SyncOptions::default()` as third arg (backward compat)

### Step 2: Make tests compile (scaffold types)

#### Config (`config.rs`)
- [x] Add `sync_max_depth: Option<u32>` to `MakeworkConfig`
- [x] Add `sync_exclude: Vec<String>` to `MakeworkConfig`
- [x] Add `"sync_max_depth"` and `"sync_exclude"` arms in `config_set()`

#### Core (`catalog.rs`)
- [x] Add `SyncOptions` struct with `Default` impl
- [x] Change `Catalog::sync()` signature: add `options: &SyncOptions` parameter

### Step 3: Implement until GREEN

#### Core (`catalog.rs`)
- [x] Replace `read_dir` loop with recursive `walk_for_repos(dir, current_depth, max_depth, exclude)` helper
- [x] In walker: skip entries matching exclude patterns
- [x] In walker: detect submodule (`.git` is a file, not dir), skip
- [x] In walker: detect bare repo (HEAD + objects/ exist, no .git), skip
- [x] In walker: `.git` is_dir → real repo, do NOT recurse deeper
- [x] In walker: otherwise recurse if within depth limit

#### CLI (`commands/mod.rs`)
- [x] Add `--depth <N>` optional flag to `Sync` command struct
- [x] Add `--exclude <PATTERN>` repeatable flag to `Sync` command struct
- [x] In `Sync` handler: build `SyncOptions` from config + CLI overrides

### Step 4: Validate

- [x] `cargo test -p mw-core` — all new + existing sync tests green
- [x] `cargo test` — full suite green
- [x] `cargo clippy --all-targets -- -D warnings` — clean
- [x] `cargo fmt -- --check` — clean
- [ ] Manual: `mw sync --depth 2 --exclude node_modules` works on real filesystem

---

## Phase 10 — Sparse-checkout for monorepo worktrees ✅

All steps complete:
- [x] `sparse_paths: Option<Vec<String>>` added to `Subproject`
- [x] `enable_sparse_checkout` / `disable_sparse_checkout` implemented in `worktree.rs`
- [x] `sparse_paths` field added to `ResolvedProject` and populated from subproject
- [x] `go()` applies sparse-checkout when configured
- [x] Unit tests: `enable_sparse_checkout_runs_git_commands`, `disable_sparse_checkout_restores_full`, `subproject_sparse_paths_roundtrip`
- [x] All tests green, clippy clean, fmt clean

---

## Phase 11 — Worktree template files ✅

All steps complete:
- [x] `template.rs` module with `apply_template()` — recursive copy, no-overwrite, graceful no-op
- [x] `template_dir: Option<PathBuf>` added to `MakeworkConfig` with tilde expansion and `config_set` support
- [x] `go()` applies template after worktree creation
- [x] Unit tests: copies files, no-overwrite, nested dirs, missing dir noop
- [x] All tests green, clippy clean, fmt clean

---

## Phase 12 — Query projects (activity log) ✅

All steps complete:
- [x] `query.rs` module with ActivityEntry, parse_git_log_line, dedup_entries, query_activity, query_activity_summary
- [x] CLI Query command with --since, --until, --author, --format flags
- [x] Unit tests: parse valid/invalid, dedup same/different repos, summary grouping
- [x] All tests green, clippy clean, fmt clean

---

## Phase 13 — Cross-project ripgrep ✅

All steps complete:
- [x] `search.rs` module with SearchResult, SearchOptions, search_all (grep backend), search_grouped, parse_grep_line
- [x] CLI Search command (alias: grep) with --glob, --ignore-case, --max, optional project filter
- [x] Unit tests: parse grep line, search grouping
- [x] All tests green, clippy clean, fmt clean

---

## Phase 14 — Fuzzy picker for `mw go` with no args ✅

All steps complete:
- [x] `Catalog::all_project_names()` — sorted, deduplicated list of all navigable names
- [x] `Go` command project argument is now optional
- [x] Non-terminal: exits with error message
- [x] Terminal without interactive feature: lists available projects
- [x] Unit tests: all_project_names with repos-only, projects+subprojects, dedup
- [x] All tests green, clippy clean, fmt clean

---

## Phase 15 — flake-parts module ❌

Not implemented — `nix/flake-parts-module.nix` and `flake.nix` do not exist.
- [ ] `nix/flake-parts-module.nix` with perSystem options: enable, package, settings
- [ ] Exposed in `flake.nix` as `flakeModules.default`

---

## Phase 16 — Emacs package `makework.el` ✅

All steps complete:
- [x] `editors/emacs/makework.el` with go, status, sync, fetch commands
- [x] Output parsing for go and status, completing-read navigation
- [x] ERT test suite in `editors/emacs/makework-test.el`

---

## Phase 17 — MCP server crate `mw-mcp` ✅

All steps complete:
- [x] New `mw-mcp` crate in workspace
- [x] Resources: `makework://catalog`, `makework://status`
- [x] Tools: `go`, `sync`, `catalog_add`, `fetch`
- [x] JSON-RPC stdio transport server
- [x] CLI `mw mcp` subcommand
- [x] Unit tests: initialize, tools/list, resources/list, tool error handling, resource responses
- [x] All tests green, clippy clean, fmt clean
