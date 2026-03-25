# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

Requires [devenv](https://devenv.sh/) — the dev environment is defined in `devenv.nix` and provides the Rust stable toolchain, `cargo-mutants`, and pre-commit hooks.

```sh
devenv shell                        # enter development environment
cargo build                         # build entire workspace
cargo run -p mw-cli                 # run the mw binary
cargo test                          # run all tests across workspace
cargo test -p mw-core               # run mw-core tests only
cargo test <test_name>              # run a single test
cargo fmt                           # format Rust code
cargo clippy --all-targets -- -D warnings  # lint (CI runs with -D warnings)
cargo mutants                       # mutation testing
```

## Pre-commit Hooks

Managed by devenv via `.pre-commit-config.yaml` (auto-generated, do not edit). Runs on every commit:
- **rustfmt** — formats `.rs` files
- **nixfmt** — formats `.nix` files

## CI

Two GitHub Actions workflows:
- **CI** (`ci.yml`): format check, clippy lint (`-D warnings`), build & test
- **Build** (`build.yml`): SonarQube/SonarCloud analysis with clippy reports and code coverage via `cargo-llvm-cov`

## Project Overview

Cargo workspace with two crates:
- **`mw-core`** (`crates/mw-core/`) — library crate with all business logic (config, catalog, repository, worktree, project, maintenance, nix detection, status)
- **`mw-cli`** (`crates/mw-cli/`) — binary crate (`mw`) with clap CLI definitions and thin dispatch to mw-core

Edition 2024. Dependencies: clap (derive), serde + toml, etcetera (XDG), gix (gitoxide).

## Active Technologies
- Rust, edition 2024 (stable toolchain via devenv) + clap (derive) for CLI, serde + toml for serialization, dirs/etcetera for XDG paths, gix for git inspection, std::process::Command for git CLI shelling (makework-mvp)
- TOML files — `$XDG_CONFIG_HOME/makework/config.toml`, `$XDG_CONFIG_HOME/makework/catalog.toml`, per-project `.makework.toml`; status cache in `$XDG_STATE_HOME/makework/` (makework-mvp)

## Recent Changes
- makework-mvp: Added Rust, edition 2024 (stable toolchain via devenv) + clap (derive) for CLI, serde + toml for serialization, dirs/etcetera for XDG paths, gix for git inspection, std::process::Command for git CLI shelling
