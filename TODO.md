# TODO

## Phase 0: Project Restructure
- [ ] Create `crates/mw-core/` with `Cargo.toml` and `src/lib.rs`
- [ ] Create `crates/mw-cli/` with `Cargo.toml` and `src/main.rs` (binary name: `mw`)
- [ ] Convert root `Cargo.toml` to workspace manifest
- [ ] Remove `src/main.rs`
- [ ] Update `sonar-project.properties` for new source paths
- [ ] Verify `cargo build`, `cargo test`, `cargo clippy`, `cargo fmt` pass

## Phase 1: Core Data Model & Configuration
- [ ] Add dependencies: `serde`, `toml`, `dirs`/`xdg` to `mw-core`
- [ ] Implement `MakeworkConfig` with XDG path resolution and defaults
- [ ] Implement `NixConfig` struct (type, devshell, path)
- [ ] Implement `Project` and `Subproject` structs
- [ ] Implement `Repository` struct (name, path, url, main_branch, remotes, projects)
- [ ] Implement `Catalog` struct with load/save from TOML
- [ ] Implement per-project `.makework.toml` parsing
- [ ] Implement catalog + per-project config merge (per-project wins)
- [ ] Implement unique subproject name validation (FR-010)
- [ ] Implement project lookup by name across all repos
- [ ] Tests: TOML round-trip, merge logic, duplicate rejection, lookup

## Phase 2: CLI Scaffolding
- [ ] Add `clap` (derive) dependency to `mw-cli`
- [ ] Define `Cli` struct with all subcommands per Appendix C
- [ ] Create command module files: `go.rs`, `new.rs`, `rm.rs`, `ls.rs`, `fetch.rs`, `sync.rs`, `status.rs`, `catalog.rs`
- [ ] Implement `mw completions <shell>`
- [ ] Implement no-args dispatch to status overview (stub)
- [ ] Verify `mw --help` shows full command tree

## Phase 3: Git Operations
- [ ] Add `gix` dependency to `mw-core`
- [ ] Implement `clone_bare(url, path)` — shell out to `git clone --bare`
- [ ] Implement `fetch(bare_path)` — `git fetch --all --prune --tags`
- [ ] Implement `list_branches(bare_path)` via gitoxide
- [ ] Implement `get_default_branch(bare_path)`
- [ ] Implement worktree path convention: `<root>/<remote_base>/<group>/<repo>/<branch>/`
- [ ] Implement branch name sanitization for paths (FR-006)
- [ ] Implement `create_worktree(bare_path, branch, worktree_path)`
- [ ] Implement `remove_worktree(bare_path, worktree_path)`
- [ ] Implement `list_worktrees(bare_path)` — parse `git worktree list --porcelain`
- [ ] Implement `prune_worktrees(bare_path)`
- [ ] Implement detached-HEAD worktree creation (tags/commits)
- [ ] Implement `maintenance register/unregister/status`
- [ ] Integration tests: temp git repos, bare clone → worktree lifecycle

## Phase 4: Catalog Commands
- [ ] Implement `mw catalog add <path>` (detect repo, read .makework.toml, bare clone, default worktree, register maintenance, save catalog, idempotent)
- [ ] Implement `mw catalog list`
- [ ] Implement `mw catalog remove <project>` (warn about existing worktrees)
- [ ] Implement `mw catalog edit` (open $EDITOR)
- [ ] Implement `mw sync` (scan ~/Developer, discover repos, register new ones)
- [ ] Implement `mw project init` (create .makework.toml template)
- [ ] Implement `mw project show` (resolved merged config)

## Phase 5: Worktree Commands
- [ ] Implement `mw new <project> <ref>` (resolve project, compute path, create branch if needed, create worktree)
- [ ] Implement `mw rm <project>/<ref>` (resolve, remove, prune)
- [ ] Implement `mw ls` (list worktrees grouped by project, show branch/dirty/path)
- [ ] Implement `mw go <project>[/<ref>]` (resolve, create if needed, print path; handle subproject paths)
- [ ] Create shell integration function for `mw go` (bash/zsh/fish)
- [ ] Handle edge case: subproject name in multiple worktrees → prompt user

## Phase 6: Status Overview
- [ ] Implement `mw` (no args): dirty count, ahead/behind per worktree, upstream changes
- [ ] Implement `mw fetch [project]` (foreground fetch + status update)
- [ ] Implement status cache in `$XDG_STATE_HOME/makework/`
- [ ] Output formatting: compact, terminal-friendly

## Phase 7: Nix Dev Shell Detection
- [ ] Implement detection chain: `.makework.toml` → `flake.nix` → `shell.nix` → `devenv.nix` → `.envrc`
- [ ] Return detected type and activation command
- [ ] Integrate with `mw go`: activate after cd, respect `nix.path` for subprojects
- [ ] Skip activation gracefully when no nix config found

## Phase 8: Nix Packaging
- [ ] Create `flake.nix` for building `mw` binary
- [ ] Create NixOS module (`nix/module.nix`)
- [ ] Create home-manager module (`nix/home-manager.nix`) with shell integration
- [ ] Verify `nix build` works in sandbox

## Edge Cases (address during relevant phases)
- [ ] Handle remote no longer exists on `catalog add` (warn, register local)
- [ ] Reject duplicate subproject names across repos
- [ ] Detect orphaned worktrees (manually deleted) in `mw ls`, offer prune
- [ ] Detect default branch rename (master → main) on fetch, warn
- [ ] Surface git lock errors clearly on concurrent operations
- [ ] Handle `.makework.toml` vs catalog conflicts (per-project wins, log override)
