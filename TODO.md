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

## Phase 15 — flake-parts module

### Step 1: Write failing test (RED)

#### Nix eval test
- [ ] Create `nix/tests/flake-parts-eval.nix` (or use `nix eval`)
  - Import `flake-parts-module.nix` in a minimal flake-parts config
  - Assert: evaluation succeeds without errors
  - Assert: enabling makework adds the package to `config.packages`

### Step 2: Scaffold

- [ ] Create `nix/flake-parts-module.nix` — minimal module skeleton that fails the eval test with a clear error

### Step 3: Implement until GREEN

#### `nix/flake-parts-module.nix`
- [ ] `perSystem` options:
  - `makework.enable` — `mkEnableOption "makework worktree manager"`
  - `makework.package` — `mkPackageOption` defaulting to `self'.packages.${system}.default`
  - `makework.settings.worktreeRoot` — optional string
  - `makework.settings.bareRoot` — optional string
- [ ] `perSystem` config:
  - When enabled: add package to `config.packages`
  - When settings provided: generate `config.toml` (as a derivation or shell hook)

#### `flake.nix`
- [ ] Add `flake-parts` to inputs (or make it optional)
- [ ] Add `flakeModules.default = ./nix/flake-parts-module.nix;` to outputs

### Step 4: Validate

- [ ] `nix flake check` — passes
- [ ] `nix eval .#flakeModules.default` — resolves to the module
- [ ] Manual: create test flake using `inputs.makework.flakeModules.default`, verify `nix develop` has `mw` in PATH

---

## Phase 16 — Emacs package `makework.el`

### Step 1: Write failing tests (RED)

#### ERT tests — `editors/emacs/makework-test.el`
- [ ] `makework-test-parse-go-output`
  - Input: `"/path/to/worktree\nnix develop\n/path/to/nix"`
  - Assert: parsed to `(path . "/path/to/worktree")`, `(nix-cmd . "nix develop")`, `(nix-dir . "/path/to/nix")`

- [ ] `makework-test-parse-go-output-no-nix`
  - Input: `"/path/to/worktree\n"`
  - Assert: parsed to `(path . "/path/to/worktree")`, `(nix-cmd . nil)`, `(nix-dir . nil)`

- [ ] `makework-test-build-command`
  - Assert: `(makework--build-command "go" "myproject")` returns `("mw" "go" "myproject")`
  - Assert: custom `makework-binary` is respected

- [ ] `makework-test-parse-status-output`
  - Input: multiline status output string
  - Assert: parsed into structured list of `(repo . ((branch . status) ...))` entries

### Step 2: Scaffold

- [ ] Create `editors/emacs/makework.el` with `provide`, `defgroup`, `defcustom`, stub functions

### Step 3: Implement until GREEN

#### `editors/emacs/makework.el`
- [ ] `defgroup makework` with customization variables:
  - `makework-binary` (default `"mw"`)
  - `makework-use-nix` (default `t`)
- [ ] `makework--build-command (&rest args)` — prepend `makework-binary`
- [ ] `makework--parse-go-output (output)` — split on newlines, extract path/nix-cmd/nix-dir
- [ ] `makework--project-list ()` — call `mw catalog list`, parse names
- [ ] `makework-go ()` — `interactive`, `completing-read` from project list, call `mw go`, cd + nix
- [ ] `makework-status ()` — run `mw`, display in `*makework-status*` buffer (read-only, special-mode)
- [ ] `makework-sync ()` — run `mw sync`, message results
- [ ] `makework-fetch ()` — run `mw fetch`, display in compilation-like buffer
- [ ] `project.el` backend: `makework-project-find-function` for `project-find-functions`

#### Nix packaging
- [ ] Add `makework-el` package to `flake.nix` using `trivialBuild` or `emacsPackages.trivialBuild`
- [ ] Optionally: add to home-manager module as `programs.emacs.extraPackages` option

### Step 4: Validate

- [ ] `emacs -batch -l makework.el -l makework-test.el -f ert-run-tests-batch-and-exit` — all ERT tests pass
- [ ] `nix build .#makework-el` — builds
- [ ] Manual: open Emacs, `M-x makework-go`, verify navigation works

---

## Phase 17 — MCP server crate `mw-mcp`

### Step 1: Write failing tests (RED)

#### Unit tests — `crates/mw-mcp/src/` inline `#[cfg(test)]`
- [ ] `catalog_resource_returns_repo_list`
  - Setup: temp catalog with 2 repos
  - Call catalog resource handler
  - Assert: JSON response has 2 entries with names and URLs

- [ ] `status_resource_returns_worktree_status`
  - Setup: temp catalog with 1 repo that has a worktree
  - Call status resource handler
  - Assert: JSON response includes worktree path, branch, dirty count

- [ ] `go_tool_returns_path`
  - Setup: temp catalog/config with registered repo
  - Call go tool handler with `{ "project": "my-repo" }`
  - Assert: response includes `path` field pointing to worktree

- [ ] `go_tool_error_on_unknown_project`
  - Call go tool handler with `{ "project": "nonexistent" }`
  - Assert: error response with descriptive message

- [ ] `sync_tool_discovers_repos`
  - Setup: temp scan_root with git repos
  - Call sync tool handler
  - Assert: response includes list of newly added repos

#### Integration tests — `crates/mw-mcp/tests/mcp_protocol.rs` (new file)
- [ ] `mcp_initialize_handshake`
  - Spawn `mw mcp` subprocess with stdin/stdout pipes
  - Send MCP `initialize` request
  - Assert: valid `initialize` response with server info, capabilities
  - Send `initialized` notification
  - Assert: no error

- [ ] `mcp_list_tools`
  - After init handshake: send `tools/list` request
  - Assert: response lists `go`, `sync`, `catalog_add`, `fetch` tools

- [ ] `mcp_list_resources`
  - After init handshake: send `resources/list` request
  - Assert: response lists `makework://catalog`, `makework://status` resources

### Step 2: Make tests compile (scaffold types)

#### New crate setup
- [ ] Create `crates/mw-mcp/Cargo.toml`:
  - `[dependencies]`: `mw-core = { path = "../mw-core" }`, MCP SDK crate (e.g., `rmcp`), `serde`, `serde_json`, `tokio`
  - `[[bin]]`: `name = "mw-mcp"` (or make it a library used by `mw-cli`)
- [ ] Add `"crates/mw-mcp"` to workspace `members` in root `Cargo.toml`
- [ ] Create `crates/mw-mcp/src/lib.rs` with module declarations
- [ ] Stub resource handlers returning `todo!()`
- [ ] Stub tool handlers returning `todo!()`

### Step 3: Implement until GREEN

#### MCP Resources (`resources.rs`)
- [ ] `handle_catalog_resource(config) -> serde_json::Value` — load catalog, serialize repos
- [ ] `handle_project_resource(config, name) -> serde_json::Value` — resolve project, serialize details + worktrees
- [ ] `handle_status_resource(config) -> serde_json::Value` — get_all_status, serialize

#### MCP Tools (`tools.rs`)
- [ ] `handle_go(config, params) -> serde_json::Value` — `worktree::go()`, return path + nix info
- [ ] `handle_sync(config, params) -> serde_json::Value` — `catalog::sync()`, return added list
- [ ] `handle_catalog_add(config, params) -> serde_json::Value` — dispatch URL vs path add
- [ ] `handle_fetch(config, params) -> serde_json::Value` — `repository::fetch()` one or all
- [ ] `handle_search(config, params) -> serde_json::Value` — `search::search_all()` (Phase 13 dependency)
- [ ] `handle_query(config, params) -> serde_json::Value` — `query::query_activity()` (Phase 12 dependency)

#### MCP Server (`server.rs`)
- [ ] Register all tools with MCP SDK router
- [ ] Register all resources with MCP SDK router
- [ ] Start stdio transport server

#### CLI integration (`mw-cli/commands/mod.rs`)
- [ ] Add `Mcp` subcommand: `mw mcp` — starts MCP server (delegate to `mw-mcp` lib)
- [ ] Optional: `--port <N>` for HTTP/SSE transport

### Step 4: Validate

- [ ] `cargo test -p mw-mcp` — all unit tests green
- [ ] `cargo test` — full workspace green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}' | mw mcp` returns valid response
- [ ] Manual: configure Claude Code to use `mw mcp` as MCP server, verify tool listing works
