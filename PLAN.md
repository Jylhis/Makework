# Implementation Plan

## Current State

All phases (0–17) are complete. The project is a Cargo workspace with three crates (`mw-core`, `mw-cli`, `mw-mcp`), full CLI, git worktree management, nix detection, status overview, Nix flake packaging, cross-project search, activity query, worktree templates, sparse-checkout support, Emacs integration, flake-parts module, and MCP server.

## Phase 9 — `mw sync` auto-discovery with heuristics

**Goal:** Currently `sync` walks `scan_roots` one level deep for `.git` dirs. Enhance with configurable depth, exclude patterns, and smarter heuristics.

### Config changes (`config.rs`)
- Add `sync_max_depth: Option<u32>` to `MakeworkConfig` (default `1`, backward-compatible)
- Add `sync_exclude: Vec<String>` to `MakeworkConfig` (glob patterns like `node_modules`, `.cache`, `target`)
- Extend `config_set()` to support `sync_max_depth` and `sync_exclude` keys

### Core changes (`catalog.rs`)
- Modify `Catalog::sync()` to accept a `SyncOptions` struct: `{ max_depth: u32, exclude: Vec<String> }`
- Replace single-level `read_dir` walk with recursive walker bounded by `max_depth`
- Skip directories matching any `sync_exclude` glob pattern
- Heuristic: detect nested git repos (submodules), skip them by default
- Heuristic: detect bare repos (presence of `HEAD` + `objects/` + no `.git`) and skip them

### CLI changes (`commands/mod.rs`)
- Add `--depth <N>` flag to `Sync` command (overrides config)
- Add `--exclude <PATTERN>` repeatable flag (merged with config excludes)

### Tests
- `sync_respects_max_depth` — repos at depth 2 found when depth=2, not when depth=1
- `sync_excludes_patterns` — directory matching exclude glob is skipped
- `sync_skips_submodules` — nested `.git` file (submodule) is not registered as separate repo
- `sync_skips_bare_repos` — bare repos in scan_roots are not double-registered

---

## Phase 10 — Sparse-checkout for monorepo worktrees

**Goal:** When navigating to a subproject in a monorepo, only check out the relevant paths to save disk space and speed up operations.

### Data model changes (`project.rs`)
- Add `sparse_paths: Option<Vec<String>>` field to `Subproject`
- Serialized in `.makework.toml` and `catalog.toml`

### Git operations (`worktree.rs`)
- Add `enable_sparse_checkout(worktree_path: &Path, paths: &[String]) -> Result<(), GitError>`
  - Runs `git -C <path> sparse-checkout init --cone`
  - Runs `git -C <path> sparse-checkout set <paths...>`
- Add `disable_sparse_checkout(worktree_path: &Path) -> Result<(), GitError>`
- Modify `go()`: after creating or entering a worktree, if resolved subproject has `sparse_paths`, call `enable_sparse_checkout`

### CLI changes
- No new commands needed — sparse-checkout is driven by `.makework.toml` config
- `mw project show` should display sparse_paths when set

### Tests
- `sparse_checkout_sets_paths` — verify sparse-checkout config written after `go`
- `sparse_checkout_skipped_when_unset` — no sparse-checkout when `sparse_paths` is None

---

## Phase 11 — Worktree template files

**Goal:** Apply template files (e.g., `.envrc`, editor configs, hooks) to newly created worktrees.

### Config changes (`config.rs`)
- Add `template_dir: Option<PathBuf>` to `MakeworkConfig`
- Extend `config_set()` to support `template_dir` key

### Core changes (new module `template.rs`)
- `apply_template(template_dir: &Path, worktree_path: &Path) -> Result<Vec<PathBuf>, TemplateError>`
  - Copy all files from `template_dir` into worktree root
  - Do not overwrite existing files (skip with warning)
  - Return list of files copied

### Integration (`worktree.rs`, `catalog.rs`)
- After `create_worktree` succeeds in `go()`, call `apply_template` if `template_dir` is configured
- After `create_worktree` in `catalog_add`, same
- Per-project override: add `template_dir: Option<String>` to `.makework.toml` (relative path within repo)

### CLI changes
- `mw config set template_dir ~/path` — set global template directory
- `mw project show` — display template_dir when set

### Tests
- `template_copies_files` — files from template_dir appear in new worktree
- `template_does_not_overwrite` — existing files in worktree are not overwritten
- `template_per_project_override` — per-project template_dir takes precedence

---

## Phase 12 — Query projects (activity log)

**Goal:** Answer "what did I work on yesterday?" by querying git log across all worktrees with time filters.

### Core changes (new module `query.rs`)
- `ActivityEntry` struct: `{ repo_name, branch, commit_hash, author, date, message, worktree_path }`
- `query_activity(catalog, config, since: &str, until: Option<&str>, author: Option<&str>) -> Vec<ActivityEntry>`
  - For each repo's worktrees, run `git -C <wt> log --since=<since> --until=<until> --author=<author> --format=<format>`
  - Parse output into `ActivityEntry` list
  - Sort by date descending
  - Deduplicate across worktrees (same commit can appear in multiple worktrees of same repo)
- `query_activity_summary(entries: &[ActivityEntry]) -> BTreeMap<String, Vec<ActivityEntry>>`
  - Group by repo name for display

### CLI changes (`commands/mod.rs`)
- Add `Query` subcommand with:
  - `--since <DATE>` (required, e.g., "yesterday", "2 days ago", "2026-03-01")
  - `--until <DATE>` (optional)
  - `--author <NAME>` (optional, defaults to git user.name)
  - `--format <FORMAT>` (optional: `short` | `full`, default `short`)
- Output grouped by repo: repo name header, then commit list

### Tests
- `query_returns_recent_commits` — commits within range appear
- `query_filters_by_author` — only matching author's commits returned
- `query_deduplicates_across_worktrees` — same commit not listed twice

---

## Phase 13 — Cross-project ripgrep

**Goal:** Search across all project worktrees with a single command, results grouped by project.

### Core changes (new module `search.rs`)
- `SearchResult` struct: `{ repo_name, worktree_path, file_path, line_number, line_content }`
- `search_all(catalog, config, pattern: &str, options: SearchOptions) -> Vec<SearchResult>`
  - `SearchOptions`: `{ file_glob: Option<String>, case_insensitive: bool, max_results: Option<usize> }`
  - For each repo, pick the main-branch worktree
  - Shell out: `rg --json <pattern> [--glob <glob>] [-i] <worktree_path>`
  - Parse JSON output into `SearchResult` list
  - Fall back to `grep -rn` if `rg` is not installed
- `search_grouped(results) -> BTreeMap<String, Vec<SearchResult>>` — group by repo

### CLI changes
- Add `Search` (alias `Grep`) subcommand:
  - `pattern: String` — regex pattern
  - `--glob <GLOB>` — file filter (e.g., `*.rs`)
  - `-i` / `--ignore-case` — case insensitive
  - `--max <N>` — limit results per repo
  - `project: Option<String>` — limit to single project
- Output: grouped by repo, file path relative to worktree, line number + content

### Tests
- `search_finds_matches` — known pattern found in test repo
- `search_respects_glob` — file filter works
- `search_fallback_to_grep` — works when `rg` not in PATH (mock/skip)

---

## Phase 14 — Fuzzy picker for `mw go` with no args

**Goal:** When `mw go` is invoked without arguments, present an interactive fuzzy finder listing all projects and subprojects.

### Dependencies
- Add `nucleo` (or `skim`) to `mw-cli` Cargo.toml for fuzzy matching
- Feature-gate behind `interactive` feature to keep the binary lean by default

### Core changes (`catalog.rs`)
- Add `Catalog::all_project_names(&self) -> Vec<String>` — flat list of all repo names, project names, and subproject names for picker input

### CLI changes (`commands/mod.rs`)
- Modify `Go` command: make `project` optional
- When `project` is `None`:
  - Collect all project names from catalog
  - Launch interactive fuzzy picker (TUI)
  - User selects a project, then proceed with normal `go` flow
- When stdin is not a TTY (piped), error with message to provide project name

### Tests
- `all_project_names_returns_complete_list` — unit test on catalog
- Integration test for fuzzy picker is manual / interactive only

---

## Phase 15 — flake-parts module

**Goal:** Provide a flake-parts module so users can consume makework in flake-parts-based flakes.

### Nix changes (`nix/flake-parts-module.nix`)
- Create `flake-parts-module.nix` exposing:
  - `perSystem.makework.enable` — add package to devshell
  - `perSystem.makework.package` — override
  - `perSystem.makework.settings` — worktree_root, bare_root
- Expose in `flake.nix` outputs as `flakeModules.default`

### Tests
- Nix evaluation test: import the module in a minimal flake-parts flake, assert it evaluates

---

## Phase 16 — Emacs package `makework.el`

**Goal:** Emacs integration for navigating and managing makework projects.

### New directory: `editors/emacs/`
- `makework.el` — Emacs Lisp package
  - `makework-go` — completing-read over project names, `cd` + nix activation via `shell-command`
  - `makework-status` — display `mw` (status) output in a read-only buffer
  - `makework-sync` — run `mw sync` and report results
  - `makework-fetch` — run `mw fetch` with progress
  - `project.el` integration: register makework projects as project.el backends
  - Customization group: `makework-binary` (path to `mw`), `makework-use-nix` (activate nix on go)

### Nix packaging
- Add `makework-el` to flake.nix outputs (trivialBuild or melpaBuild)
- Add to home-manager module: `programs.emacs.extraPackages` option

### Tests
- ERT test suite: test command construction, output parsing
- Manual testing instructions in `editors/emacs/README.md`

---

## Phase 17 — MCP server crate `mw-mcp`

**Goal:** Expose makework functionality over Model Context Protocol for AI assistant integration.

### New crate: `crates/mw-mcp/`
- Add to workspace members
- Dependencies: `mw-core`, MCP server SDK (e.g., `mcp-server` or `rmcp`)

### MCP Resources
- `makework://catalog` — list all repos with metadata
- `makework://project/{name}` — project details (worktrees, status, config)
- `makework://status` — full status overview

### MCP Tools
- `go` — navigate to project (returns path + nix info)
- `sync` — discover and register repos
- `catalog_add` — register repo from URL or path
- `fetch` — fetch updates
- `search` — cross-project grep (depends on Phase 13)
- `query` — activity query (depends on Phase 12)

### CLI integration
- Add `Mcp` subcommand: `mw mcp` starts the MCP server (stdio transport)
- Optionally: `mw mcp --port <N>` for HTTP transport

### Tests
- Unit tests: tool handlers return expected JSON
- Integration test: start server, send MCP request, verify response
