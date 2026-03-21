# Makework

A Rust project.

## Development

This project uses [devenv.sh](https://devenv.sh/) for development environment management.

### Setup

```sh
devenv shell
```

### Build & Run

```sh
cargo build
cargo run
```

### Code Quality

```sh
cargo fmt       # format code
cargo clippy    # lint
cargo test      # run tests
```

## Project Structure

- `src/main.rs` — Application entry point
- `Cargo.toml` — Rust package manifest
- `devenv.nix` — Development environment configuration
