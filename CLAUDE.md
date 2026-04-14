# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

Requires [devenv](https://devenv.sh/) — the dev environment is defined in `devenv.nix` and provides the Rust stable toolchain, `cargo-mutants`, `just`, and treefmt.

```sh
devenv shell                        # enter development environment
just                                # list all available commands
just build                          # build via nix (flake)
just check                          # run all flake checks (build, clippy, tests, fmt)
just fmt                            # format all code (Rust + Nix)
just sync                           # sync devenv.lock to flake.lock nixpkgs pin
just verify                         # verify lock files are in sync
cargo build                         # build entire workspace
cargo run -p mw-cli                 # run the mw binary
cargo test                          # run all tests across workspace
cargo test -p mw-core               # run mw-core tests only
cargo test <test_name>              # run a single test
cargo clippy --all-targets -- -D warnings  # lint (CI runs with -D warnings)
cargo mutants                       # mutation testing
```

## Formatting

Managed by treefmt with shared config in `nix/treefmt.nix` (used by both `nix fmt` and devenv). Runs:
- **rustfmt** — formats `.rs` files
- **nixfmt** — formats `.nix` files

Use `just fmt` (or `nix fmt` / `treefmt`) to format everything, or `cargo fmt` for Rust only.

## CI

Two GitHub Actions workflows:
- **CI** (`ci.yml`): `devenv test` (matrix: linux + macos), format check, clippy lint (`-D warnings`), build & test
- **Build** (`build.yml`): SonarQube/SonarCloud analysis with clippy reports and code coverage via `cargo-llvm-cov`

## Project Overview

Cargo workspace with three crates:
- **`mw-core`** (`crates/mw-core/`) — library crate with all business logic (config, catalog, repository, resolver, worktree, project, maintenance, nix detection, status, search, query, template)
- **`mw-cli`** (`crates/mw-cli/`) — binary crate (`mw`) with clap CLI definitions and thin dispatch to mw-core
- **`mw-mcp`** (`crates/mw-mcp/`) — MCP server crate exposing makework functionality over JSON-RPC stdio transport for AI assistant integration

Edition 2024. Key dependencies: clap (derive) + clap_complete, serde + serde_json + toml, etcetera (XDG), gix (gitoxide), strsim (fuzzy matching).

## Configuration Files
- `$XDG_CONFIG_HOME/makework/config.toml` — global config
- `$XDG_CONFIG_HOME/makework/catalog.toml` — repo registry
- Per-project `.makework.toml` — subprojects, sparse-checkout paths
- `$XDG_STATE_HOME/makework/` — status cache
