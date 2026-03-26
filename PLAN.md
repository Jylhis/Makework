# Implementation Plan

## Current State

Single-file Rust binary crate (`src/main.rs`) with hello-world, no dependencies. Build tooling in place: devenv with Rust stable, pre-commit hooks (clippy, rustfmt, nixfmt), CI with SonarQube.

## Target State (MVP v0.1)

Multi-crate workspace CLI tool (`mw`) that manages a catalog of git projects, orchestrates bare repos + git worktrees, detects Nix dev shells, and provides a status overview.

---

## Phase 0: Project Restructure

Convert from single-crate binary to a Cargo workspace with two crates.

### Steps

1. Create workspace directory structure:
   ```
   crates/mw-core/src/lib.rs
   crates/mw-cli/src/main.rs
   ```
2. Convert root `Cargo.toml` to a workspace manifest with `members = ["crates/mw-core", "crates/mw-cli"]`
3. Create `crates/mw-core/Cargo.toml` (library crate, no deps yet)
4. Create `crates/mw-cli/Cargo.toml` (binary crate, depends on `mw-core`)
5. Move existing test to `mw-cli` or remove (placeholder test)
6. Verify `cargo build`, `cargo test`, `cargo clippy`, `cargo fmt` all pass
7. Update `CLAUDE.md` if needed to reflect workspace structure
8. Update `sonar-project.properties` for new source paths

### Key Decisions
- Binary name is `mw` (set via `[[bin]]` in mw-cli or package name)
- `mw-core` is the shared library for CLI, future MCP server, and Emacs integration
- Remove `src/main.rs` after migration

---

## Phase 1: Core Data Model & Configuration

Implement the catalog, project, and config data structures in `mw-core`.

### Dependencies to Add (mw-core)
- `serde`, `serde_derive` — serialization
- `toml` — TOML parsing/writing
- `dirs` or `xdg` — XDG directory resolution

### Steps

1. **Config module** (`mw-core/src/config.rs`):
   - `MakeworkConfig` struct: `worktree_root`, `bare_root`, XDG paths
   - Load from `$XDG_CONFIG_HOME/makework/config.toml` with defaults
   - Path expansion (`~` → home dir)

2. **Project model** (`mw-core/src/project.rs`):
   - `Project` struct: name, tags
   - `Subproject` struct: name, subproject_path, docs, nix config
   - `NixConfig` struct: type (flake/classic/devenv/custom), devshell path, nix path

3. **Repository model** (`mw-core/src/repository.rs`):
   - `Repository` struct: name, path, url, main_branch, remotes, projects map

4. **Catalog module** (`mw-core/src/catalog.rs`):
   - `Catalog` struct containing `config` section and `repos` map
   - Load/save from `$XDG_CONFIG_HOME/makework/catalog.toml`
   - Per-project `.makework.toml` parsing and merge (per-project wins)
   - Unique subproject name validation (FR-010)
   - Project lookup by name (searching across all repos and subprojects)

5. **Tests**:
   - Round-trip serialize/deserialize catalog TOML
   - Merge per-project `.makework.toml` over catalog entry
   - Reject duplicate subproject names
   - Project lookup by name across repos

---

## Phase 2: CLI Scaffolding

Set up clap with the command structure from Appendix C.

### Dependencies to Add (mw-cli)
- `clap` (derive feature) — CLI argument parsing

### Steps

1. Define top-level `Cli` struct with subcommands:
   - `Go { project: String, ref_: Option<String> }`
   - `New { project: String, ref_: String }`
   - `Rm { target: String }`
   - `Ls`
   - `Fetch { project: Option<String> }`
   - `Sync`
   - `Catalog` (subcommands: `List`, `Add`, `Remove`, `Edit`)
   - `Project` (subcommands: `Init`, `Show`)
   - `Maintenance` (subcommands: `Start`, `Stop`, `Status`)
   - `Config` (subcommands: `Show`, `Set`, `Edit`)
   - `Completions { shell: Shell }`
   - No subcommand → status overview

2. Each command module in `crates/mw-cli/src/commands/` dispatches to `mw-core`
3. Implement `mw completions <shell>` (pure clap, no core logic)
4. Verify `mw --help` shows the full command tree

---

## Phase 3: Git Operations — Bare Repos & Worktrees

Implement git operations in `mw-core`. Shell out to `git` CLI for worktree and maintenance operations (gitoxide doesn't cover these yet).

### Dependencies to Add (mw-core)
- `gix` (gitoxide) — git repo inspection (branches, status, refs)
- `std::process::Command` — shell out to `git` for worktree/maintenance ops

### Steps

1. **Repository module** (`mw-core/src/repository.rs`):
   - `clone_bare(url, path)` — `git clone --bare <url> <path>`
   - `fetch(bare_path)` — `git fetch --all --prune --tags`
   - `list_branches(bare_path)` — use gitoxide to enumerate refs
   - `get_default_branch(bare_path)` — resolve HEAD or configured main_branch

2. **Worktree module** (`mw-core/src/worktree.rs`):
   - Path convention: `<worktree_root>/<remote_base>/<group>/<repo>/<branch>/`
   - `create_worktree(bare_path, branch, worktree_path)` — `git worktree add`
   - `remove_worktree(bare_path, worktree_path)` — `git worktree remove`
   - `list_worktrees(bare_path)` — `git worktree list --porcelain`
   - `prune_worktrees(bare_path)` — `git worktree prune`
   - Branch name sanitization for directory paths (FR-006): `feature/auth` → `feature/auth/` (nested dirs, matching git ref layout per A-004)
   - Handle detached-HEAD worktrees for tags/commits

3. **Maintenance module** (`mw-core/src/maintenance.rs`):
   - `register(bare_path)` — `git maintenance register`
   - `unregister(bare_path)` — `git maintenance unregister`
   - `status(bare_path)` — check if registered
   - `run_prefetch(bare_path)` — `git maintenance run --task=prefetch`

4. **Tests**:
   - Integration tests using temp dirs with real git repos
   - Create bare clone → create worktree → verify branch → remove worktree
   - Register/unregister maintenance

---

## Phase 4: Catalog Commands

Wire up the catalog CLI commands to core logic.

### Steps

1. **`mw catalog add <path>`**:
   - Detect if path is a git repo
   - Read `.makework.toml` if present
   - Create bare clone (if not already bare)
   - Create default-branch worktree
   - Register with `git maintenance`
   - Add to catalog, save
   - Idempotent: update existing entry without duplication

2. **`mw catalog list`**: Print all catalog entries with metadata

3. **`mw catalog remove <project>`**: Unregister (warn about existing worktrees)

4. **`mw catalog edit`**: Open `$EDITOR` on catalog.toml

5. **`mw sync`**: Scan `~/Developer` for git repos, register undiscovered ones

6. **`mw project init`**: Create `.makework.toml` template in current repo

7. **`mw project show`**: Print resolved config (catalog merged with per-project)

---

## Phase 5: Worktree Commands

Wire up worktree CLI commands.

### Steps

1. **`mw new <project> <ref>`**:
   - Resolve project → bare repo path
   - Compute worktree path from convention
   - Create branch from default if it doesn't exist
   - `git worktree add`

2. **`mw rm <project>/<ref>`**:
   - Resolve to worktree path
   - `git worktree remove` + prune

3. **`mw ls`**:
   - For each registered repo, list active worktrees
   - Group by project name
   - Show branch, dirty state, path

4. **`mw go <project>[/<ref>]`**:
   - Resolve project (may be subproject name)
   - If worktree exists, print path (shell function does `cd`)
   - If worktree doesn't exist, create it on-the-fly, then print path
   - For subprojects, print subproject path within worktree

5. **Shell integration**:
   - Provide shell function `mw()` that wraps the binary
   - For `go` command: capture path output, `cd` to it
   - For all other commands: pass through
   - Generate via `mw completions` or document for manual setup

---

## Phase 6: Status Overview

Implement the default `mw` command (no args) showing project status.

### Steps

1. For each registered repo:
   - Get active worktrees
   - For each worktree: dirty file count, ahead/behind counts (vs prefetch refs)
   - Show upstream changes available (comparing prefetch refs to local)

2. **`mw fetch [project]`**: Foreground fetch, then update status cache

3. Output format: compact table or grouped list, suitable for terminal

4. Cache status in `$XDG_STATE_HOME/makework/` for fast repeat display (FR-014)

---

## Phase 7: Nix Dev Shell Detection

Implement Nix environment detection and activation delegation.

### Steps

1. **Nix detection module** (`mw-core/src/nix.rs`):
   - Detection order (per spec): explicit `.makework.toml` `nix.devshell` → `flake.nix` → `shell.nix` → `devenv.nix` → `.envrc` with nix directives
   - Return detected type and activation command

2. **Integration with `mw go`**:
   - After cd, if nix config detected, spawn activation command
   - Respect `nix.path` for subprojects (relative to repo root)
   - No nix config → just cd, no activation attempt

---

## Phase 8: Nix Packaging

Package the tool as a Nix flake with modules.

### Steps

1. Create `flake.nix` at repo root:
   - Package `mw` binary
   - Dev shell (current devenv setup can coexist)

2. NixOS module (`nix/module.nix`):
   - Option to install `mw` system-wide
   - Option to configure default catalog settings

3. Home-manager module (`nix/home-manager.nix`):
   - Option to install `mw` for user
   - Option to declare catalog entries declaratively
   - Shell integration (add `mw` function to bashrc/zshrc)

---

## Phase Ordering & Dependencies

```
Phase 0 (restructure)
  └→ Phase 1 (data model)
       ├→ Phase 2 (CLI scaffolding)
       └→ Phase 3 (git operations)
            ├→ Phase 4 (catalog commands)    ← needs Phase 2 + 3
            └→ Phase 5 (worktree commands)   ← needs Phase 2 + 3
                 └→ Phase 6 (status)         ← needs Phase 5
                      └→ Phase 7 (nix)       ← needs Phase 5
                           └→ Phase 8 (packaging) ← needs all above
```

Phases 2 and 3 can be developed in parallel after Phase 1.
Phases 4 and 5 can be developed in parallel after Phases 2+3.

---

## Out of Scope (Follow-up)

- Sparse-checkout for monorepo worktrees (v0.2)
- MCP server crate `mw-mcp` (v0.2)
- Emacs package `makework.el` (v0.2)
- flake-parts module (v0.2)
- `mw sync` auto-discovery with heuristics (v0.2)
- Fuzzy picker for `mw go` with no args (future)
- Cross-project ripgrep (future)
- Worktree template files (future)
- Query projects (for example to answer: what did I work on yesterday)
- init catalog
