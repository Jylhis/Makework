# Implementation Plan

## Current State

Phases 0–14 and 16–17 are complete. Phase 15 (flake-parts module) is not implemented — the `nix/flake-parts-module.nix` file and `flake.nix` do not exist. The project is a Cargo workspace with three crates (`mw-core`, `mw-cli`, `mw-mcp`), full CLI, git worktree management, nix detection, status overview, crate2nix packaging via devenv, cross-project search, activity query, worktree templates, sparse-checkout support, Emacs integration, and MCP server.

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
