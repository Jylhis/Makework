# Implementation Plan

## Current State

All MVP phases (0–8) and edge-case hardening are complete. The project is a Cargo workspace with two crates (`mw-core`, `mw-cli`), full CLI, git worktree management, nix detection, status overview, and Nix flake packaging.

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
