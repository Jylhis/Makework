# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```sh
devenv shell                        # enter development environment (provides Rust toolchain + tools)
cargo build                         # build
cargo run                           # run
cargo test                          # run all tests
cargo test <test_name>              # run a single test
cargo fmt                           # format Rust code
cargo clippy --all-targets -- -D warnings  # lint (CI runs with -D warnings)
cargo mutants                       # mutation testing (cargo-mutants available via devenv)
```

## Pre-commit Hooks

Managed by devenv via `.pre-commit-config.yaml` (auto-generated, do not edit). Runs on every commit:
- **rustfmt** — formats `.rs` files
- **clippy** — lints `.rs` files
- **nixfmt** — formats `.nix` files

## CI

Two GitHub Actions workflows:
- **CI** (`ci.yml`): format check, clippy lint (`-D warnings`), build & test
- **Build** (`build.yml`): SonarQube analysis with clippy reports and code coverage via `cargo-llvm-cov`

## Project Overview

Rust binary crate (`makework`), edition 2024. Currently a single-file project at `src/main.rs` with no external dependencies.
