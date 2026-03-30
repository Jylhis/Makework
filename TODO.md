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

## Phase 10 — Sparse-checkout for monorepo worktrees

### Step 1: Write failing tests (RED)

#### Unit tests — `worktree.rs` inline `#[cfg(test)]`
- [ ] `enable_sparse_checkout_runs_git_commands`
  - Setup: tempdir with `git init`, add a commit
  - Call `enable_sparse_checkout(&path, &["src", "docs"])`
  - Assert: `git -C <path> sparse-checkout list` output contains "src" and "docs"

- [ ] `disable_sparse_checkout_restores_full`
  - Setup: same as above, enable then disable
  - Assert: `git -C <path> config core.sparseCheckout` is false or sparse-checkout is off

#### Unit tests — `project.rs` inline `#[cfg(test)]`
- [ ] `subproject_sparse_paths_roundtrip`
  - Create `Subproject` with `sparse_paths: Some(vec!["src/api", "libs/shared"])`
  - Serialize to TOML, deserialize back
  - Assert: `sparse_paths` survived round-trip
  - Assert: `Subproject` with `sparse_paths: None` serializes without the field

#### Integration tests — `crates/mw-core/tests/go.rs` (extend)
- [ ] `go_applies_sparse_checkout_for_subproject`
  - Setup: temp git repo with files in `services/api/` and `services/web/` and `shared/`
  - Create catalog entry with subproject `api` having `sparse_paths: Some(vec!["services/api", "shared"])`
  - Call `go(catalog, config, "api", None)`
  - Assert: worktree exists
  - Assert: `git -C <worktree> sparse-checkout list` contains expected paths

- [ ] `go_skips_sparse_checkout_when_unset`
  - Setup: temp git repo, subproject with `sparse_paths: None`
  - Call `go(catalog, config, "sub", None)`
  - Assert: `git -C <worktree> config core.sparseCheckout` is not set or false

### Step 2: Make tests compile (scaffold types)

#### Data model (`project.rs`)
- [ ] Add `sparse_paths: Option<Vec<String>>` to `Subproject` with `#[serde(default, skip_serializing_if = "Option::is_none")]`

#### Git operations (`worktree.rs`)
- [ ] Add `enable_sparse_checkout(worktree_path: &Path, paths: &[String]) -> Result<(), GitError>` — stub with `todo!()`
- [ ] Add `disable_sparse_checkout(worktree_path: &Path) -> Result<(), GitError>` — stub with `todo!()`

#### Catalog resolution (`catalog.rs`)
- [ ] Add `sparse_paths: Option<&'a [String]>` field to `ResolvedProject<'a>`
- [ ] Populate it in `find_project_unambiguous()` from subproject's `sparse_paths`

### Step 3: Implement until GREEN

#### Git operations (`worktree.rs`)
- [ ] `enable_sparse_checkout`: shell `git -C <path> sparse-checkout init --cone` then `git -C <path> sparse-checkout set <paths...>`
- [ ] `disable_sparse_checkout`: shell `git -C <path> sparse-checkout disable`
- [ ] In `go()`: after worktree exists, if `resolved.sparse_paths.is_some()`, call `enable_sparse_checkout`

#### CLI (`commands/mod.rs`)
- [ ] In `ProjectAction::Show` handler: print `sparse_paths` when set

### Step 4: Validate

- [ ] `cargo test -p mw-core` — all new + existing tests green
- [ ] `cargo test` — full suite green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: create `.makework.toml` with `[subprojects.x]\nsparse_paths = ["src"]`, run `mw go x`, verify sparse checkout

---

## Phase 11 — Worktree template files

### Step 1: Write failing tests (RED)

#### Unit tests — `crates/mw-core/src/template.rs` inline `#[cfg(test)]`
- [ ] `apply_template_copies_files`
  - Setup: tempdir with `template_dir/` containing `.envrc` and `.editorconfig`; empty `worktree_dir/`
  - Call `apply_template(&template_dir, &worktree_dir)`
  - Assert: `worktree_dir/.envrc` and `worktree_dir/.editorconfig` exist with correct contents
  - Assert: returned vec has 2 entries

- [ ] `apply_template_does_not_overwrite`
  - Setup: `template_dir/.envrc` with content "template"; `worktree_dir/.envrc` with content "existing"
  - Call `apply_template(&template_dir, &worktree_dir)`
  - Assert: `worktree_dir/.envrc` still reads "existing"
  - Assert: returned vec has 0 entries (skipped)

- [ ] `apply_template_copies_nested_dirs`
  - Setup: `template_dir/.config/settings.json`; empty `worktree_dir/`
  - Call `apply_template(&template_dir, &worktree_dir)`
  - Assert: `worktree_dir/.config/settings.json` exists

- [ ] `apply_template_noop_when_dir_missing`
  - Call `apply_template(&nonexistent_dir, &worktree_dir)`
  - Assert: returns `Ok(vec![])` (graceful no-op, not an error)

#### Unit tests — `config.rs` inline `#[cfg(test)]`
- [ ] `config_set_template_dir`
  - Call `MakeworkConfig::config_set("template_dir", "~/templates")` in temp env
  - Reload config, assert `template_dir` is `Some` and path is expanded

- [ ] `config_roundtrip_with_template_dir`
  - Serialize `MakeworkConfig` with `template_dir: Some(PathBuf::from("/path"))` to TOML
  - Deserialize back, assert field survives
  - Serialize with `template_dir: None`, assert field absent from TOML string

#### Integration tests — `crates/mw-core/tests/template.rs` (new file)
- [ ] `go_applies_template`
  - Setup: temp git repo, `MakeworkConfig` with `template_dir: Some(template_path)`
  - Template dir has `.envrc` file
  - Call `go(catalog, config, project, None)`
  - Assert: worktree contains `.envrc`

- [ ] `go_skips_template_when_unconfigured`
  - Setup: temp git repo, `MakeworkConfig` with `template_dir: None`
  - Call `go(catalog, config, project, None)`
  - Assert: worktree does NOT contain `.envrc`

- [ ] `per_project_template_overrides_global`
  - Setup: global `template_dir` with `global.txt`; per-project template_dir with `project.txt`
  - Catalog entry has per-project override
  - Call `go(catalog, config, project, None)`
  - Assert: worktree has `project.txt` but NOT `global.txt`

### Step 2: Make tests compile (scaffold types)

#### Config (`config.rs`)
- [ ] Add `template_dir: Option<PathBuf>` to `MakeworkConfig` with `#[serde(default, skip_serializing_if = "Option::is_none")]`
- [ ] Add `"template_dir"` arm in `config_set()`
- [ ] Add `template_dir` to `defaults()` as `None`

#### New module (`template.rs`)
- [ ] Create `crates/mw-core/src/template.rs`
- [ ] Add `pub mod template;` to `lib.rs`
- [ ] Define `TemplateError` enum: `Io(PathBuf, std::io::Error)`
- [ ] Stub `apply_template()` returning `todo!()`

#### Per-project config (`catalog.rs`)
- [ ] Add `template_dir: Option<String>` to `PerProjectConfig`
- [ ] Thread it through to `ResolvedProject` or `go()` logic

### Step 3: Implement until GREEN

#### Template module (`template.rs`)
- [ ] `apply_template`: walk `template_dir` recursively, for each file compute relative path, check if dest exists, if not create parent dirs and copy, collect into result vec
- [ ] Handle missing template_dir gracefully (return empty vec)

#### Integration (`worktree.rs`)
- [ ] In `go()`: after `create_worktree`, determine effective template_dir (per-project > global config), call `apply_template` if present
- [ ] Pass `config` (or template_dir) through to `go()` — currently `go()` does not receive config's template_dir, so add it

#### Integration (`catalog.rs`)
- [ ] In `catalog_add()`: after worktree creation, call `apply_template` if `config.template_dir` is set

### Step 4: Validate

- [ ] `cargo test -p mw-core` — all new + existing tests green
- [ ] `cargo test` — full suite green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: `mw config set template_dir ~/.config/makework/templates`, add files there, `mw go <project>`, verify files present

---

## Phase 12 — Query projects (activity log)

### Step 1: Write failing tests (RED)

#### Unit tests — `crates/mw-core/src/query.rs` inline `#[cfg(test)]`
- [ ] `parse_git_log_line`
  - Input: `"abc1234\tTest User\t2026-03-29T10:00:00+00:00\tfix: something"`
  - Assert: parsed into `ActivityEntry` with correct fields

- [ ] `dedup_entries_same_repo`
  - Input: two `ActivityEntry` with same `commit_hash` and `repo_name` but different `worktree_path`
  - Call `dedup_entries(&mut entries)`
  - Assert: result has 1 entry

- [ ] `dedup_keeps_different_repos`
  - Input: two entries with same `commit_hash` but different `repo_name`
  - Assert: result still has 2 entries (different repos can share cherry-picked commits)

- [ ] `summary_groups_by_repo`
  - Input: 3 entries across 2 repos
  - Call `query_activity_summary(&entries)`
  - Assert: BTreeMap has 2 keys, correct counts

#### Integration tests — `crates/mw-core/tests/query.rs` (new file)
- [ ] `query_returns_recent_commits`
  - Setup: temp git repo, add to catalog, make 2 commits with known messages
  - Call `query_activity(catalog, config, "7 days ago", None, None)`
  - Assert: result contains 2+ entries (initial + test commits)
  - Assert: entries sorted by date descending

- [ ] `query_filters_by_author`
  - Setup: temp git repo with user.name "Alice", commit as Alice; create second worktree, reconfigure user.name "Bob", commit as Bob
  - Call `query_activity(catalog, config, "7 days ago", None, Some("Alice"))`
  - Assert: only Alice's commits returned

- [ ] `query_deduplicates_across_worktrees`
  - Setup: temp git repo, add to catalog, `go` to create default worktree, `go` with new branch (shares history)
  - Call `query_activity(catalog, config, "7 days ago", None, None)`
  - Assert: initial commit appears only once per repo despite being visible from 2 worktrees

- [ ] `query_empty_when_no_commits_in_range`
  - Setup: temp git repo with old commits
  - Call `query_activity(catalog, config, "1 second ago", None, None)`
  - Assert: empty result

### Step 2: Make tests compile (scaffold types)

#### New module (`query.rs`)
- [ ] Create `crates/mw-core/src/query.rs`
- [ ] Add `pub mod query;` to `lib.rs`
- [ ] Define `ActivityEntry` struct: `repo_name: String, branch: String, commit_hash: String, author: String, date: String, message: String, worktree_path: PathBuf`
- [ ] Stub `query_activity()` returning `vec![]`
- [ ] Stub `query_activity_summary()` returning empty BTreeMap
- [ ] Stub `parse_git_log_line()` returning `None`
- [ ] Stub `dedup_entries()` as no-op

### Step 3: Implement until GREEN

#### Query module (`query.rs`)
- [ ] `parse_git_log_line(line: &str) -> Option<ActivityEntry>` — split on `\t`, populate fields
- [ ] `dedup_entries(entries: &mut Vec<ActivityEntry>)` — sort by `(repo_name, commit_hash)`, dedup_by same pair
- [ ] `query_activity()`:
  - List worktrees for each repo via `worktree::list_worktrees`
  - For each non-bare worktree: `git -C <path> log --since=<since> [--until=<until>] [--author=<author>] --format="%H\t%an\t%aI\t%s"` + `--no-merges` (optional)
  - Parse each line, tag with `repo_name` and `worktree_path`
  - Collect all, dedup, sort by date desc
- [ ] `query_activity_summary()` — `fold` into `BTreeMap<String, Vec<ActivityEntry>>`

#### CLI (`commands/mod.rs`)
- [ ] Add `Query` subcommand with `--since`, `--until`, `--author`, `--format`
- [ ] `Query` handler: load config/catalog, call `query_activity`, format output grouped by repo
- [ ] Short format: `<hash_short> <date> <message>`
- [ ] Full format: `<hash> <author> <date>\n  <message>`

### Step 4: Validate

- [ ] `cargo test -p mw-core` — all new tests green
- [ ] `cargo test` — full suite green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: `mw query --since yesterday` on real repos

---

## Phase 13 — Cross-project ripgrep

### Step 1: Write failing tests (RED)

#### Unit tests — `crates/mw-core/src/search.rs` inline `#[cfg(test)]`
- [ ] `parse_rg_json_match`
  - Input: raw JSON line from `rg --json` output (type "match")
  - Assert: parsed into `SearchResult` with correct file, line, content

- [ ] `parse_rg_json_skips_non_match`
  - Input: JSON line with type "begin" or "summary"
  - Assert: returns `None`

- [ ] `search_grouped_organizes_by_repo`
  - Input: 4 results across 2 repos
  - Assert: `search_grouped()` returns BTreeMap with 2 keys, correct counts

#### Integration tests — `crates/mw-core/tests/search.rs` (new file)
- [ ] `search_finds_matches`
  - Setup: temp git repo with file containing "FINDME_MARKER", add to catalog, create worktree
  - Call `search_all(catalog, config, "FINDME_MARKER", SearchOptions::default())`
  - Assert: result has 1+ entries with correct `line_content` containing "FINDME_MARKER"

- [ ] `search_respects_glob_filter`
  - Setup: temp git repo with `foo.rs` containing "MARKER" and `bar.txt` containing "MARKER"
  - Call `search_all(... , SearchOptions { file_glob: Some("*.rs"), .. })`
  - Assert: only `foo.rs` result returned

- [ ] `search_limits_results`
  - Setup: temp git repo with file containing "MARKER" on 10 lines
  - Call `search_all(... , SearchOptions { max_results: Some(3), .. })`
  - Assert: at most 3 results returned

- [ ] `search_case_insensitive`
  - Setup: temp git repo with "Hello World" in a file
  - Call `search_all(... "hello world", SearchOptions { case_insensitive: true, .. })`
  - Assert: match found

- [ ] `search_returns_empty_for_no_match`
  - Setup: temp git repo with known content
  - Call `search_all(... "NONEXISTENT_PATTERN_XYZ", ...)`
  - Assert: empty vec

### Step 2: Make tests compile (scaffold types)

#### New module (`search.rs`)
- [ ] Create `crates/mw-core/src/search.rs`
- [ ] Add `pub mod search;` to `lib.rs`
- [ ] Define `SearchResult` struct: `repo_name: String, worktree_path: PathBuf, file_path: String, line_number: u32, line_content: String`
- [ ] Define `SearchOptions` struct with `Default`: `file_glob: Option<String>, case_insensitive: bool, max_results: Option<usize>`
- [ ] Stub `search_all()` returning `vec![]`
- [ ] Stub `search_grouped()` returning empty BTreeMap
- [ ] Stub `parse_rg_json_match()` returning `None`

### Step 3: Implement until GREEN

#### Search module (`search.rs`)
- [ ] `find_rg_binary() -> Option<PathBuf>` — check `which rg` / `rg --version`
- [ ] `search_worktree_rg(worktree_path, pattern, options) -> Vec<SearchResult>` — shell: `rg --json [--glob] [-i] [-m max] <pattern> <path>`, parse JSON lines
- [ ] `search_worktree_grep(worktree_path, pattern, options) -> Vec<SearchResult>` — fallback: `grep -rn [--include=<glob>] [-i] <pattern> <path>`, parse `file:line:content` lines
- [ ] `parse_rg_json_match(line: &str) -> Option<SearchResult>` — deserialize `rg --json` match type
- [ ] `search_all()`:
  - For each repo, compute main-branch worktree path via `worktree_path()`
  - Skip if worktree doesn't exist on disk
  - Call `search_worktree_rg` (or `_grep` fallback)
  - Tag results with `repo_name`
  - Collect all
- [ ] `search_grouped()` — fold into BTreeMap by `repo_name`

#### CLI (`commands/mod.rs`)
- [ ] Add `Search` subcommand (alias `Grep` via `#[command(alias = "grep")]`)
- [ ] Args: `pattern: String`, `--glob`, `-i`/`--ignore-case`, `--max`, `project: Option<String>`
- [ ] Handler: load config/catalog, optionally filter to single project, call `search_all`, format output

### Step 4: Validate

- [ ] `cargo test -p mw-core` — all new tests green
- [ ] `cargo test` — full suite green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: `mw search "fn main" --glob "*.rs"` on real repos

---

## Phase 14 — Fuzzy picker for `mw go` with no args

### Step 1: Write failing tests (RED)

#### Unit tests — `catalog.rs` inline `#[cfg(test)]`
- [ ] `all_project_names_returns_repos`
  - Setup: catalog with 2 repos ("alpha", "beta"), no projects/subprojects
  - Assert: `all_project_names()` returns `["alpha", "beta"]`

- [ ] `all_project_names_includes_projects_and_subprojects`
  - Setup: catalog with repo "myrepo", project "myproject" with subproject "api"
  - Assert: `all_project_names()` contains "myrepo", "myproject", "api"

- [ ] `all_project_names_deduplicates`
  - Setup: repo name same as project name (common: repo "foo" with project "foo")
  - Assert: "foo" appears only once

### Step 2: Make tests compile (scaffold types)

#### Core (`catalog.rs`)
- [ ] Add `Catalog::all_project_names(&self) -> Vec<String>` — stub returning `vec![]`

### Step 3: Implement until GREEN

#### Core (`catalog.rs`)
- [ ] Collect repo names, project names, subproject names into `BTreeSet<String>` (dedup + sorted)
- [ ] Return as `Vec<String>`

#### Dependencies
- [ ] Add `nucleo-picker` (or `skim`) to `mw-cli/Cargo.toml` under `[dependencies]` gated by `interactive` feature
- [ ] `mw-cli/Cargo.toml`: `[features]\ninteractive = ["dep:nucleo-picker"]`

#### CLI (`commands/mod.rs`)
- [ ] Change `Go { project: String }` to `Go { project: Option<String> }`
- [ ] When `project.is_none()`:
  - [ ] Check `std::io::stdin().is_terminal()` (use `std::io::IsTerminal`)
  - [ ] If not terminal: `eprintln!("provide a project name or run interactively"); exit(1)`
  - [ ] If terminal + feature `interactive`: collect `all_project_names`, launch picker, use selection
  - [ ] If terminal + NOT feature `interactive`: print message about enabling `interactive` feature, list projects as fallback

### Step 4: Validate

- [ ] `cargo test -p mw-core` — all new tests green
- [ ] `cargo test -p mw-cli` — compiles with and without `interactive` feature
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Manual: `mw go` (no args) in terminal shows picker; piped input shows error

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
