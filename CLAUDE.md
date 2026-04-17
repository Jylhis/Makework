# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

Requires [devenv](https://devenv.sh/) — the dev environment is defined in `devenv.nix` and provides Go, golangci-lint, gopls, `just`, and treefmt.

```sh
devenv shell                        # enter development environment
just                                # list all available commands
just build                          # build via nix (flake)
just check                          # run all flake checks (build, lint, tests, fmt)
just fmt                            # format all code (Go + Nix)
just test                           # run Go tests
just lint                           # run golangci-lint
just sync                           # sync devenv.lock to flake.lock nixpkgs pin
just verify                         # verify lock files are in sync
go build ./cmd/mw                   # build the mw binary
go test ./...                       # run all tests
go test -run TestName ./internal/pkg/...  # run a single test
golangci-lint run                   # lint
```

## Formatting

Managed by treefmt with shared config in `nix/treefmt.nix` (used by both `nix fmt` and devenv). Runs:
- **gofmt** — formats `.go` files
- **nixfmt** — formats `.nix` files

Use `just fmt` (or `nix fmt` / `treefmt`) to format everything.

## CI

Two GitHub Actions workflows:
- **CI** (`ci.yml`): `devenv test` (matrix: linux + macos), Go lint, Go test, Go build
- **Build** (`build.yml`): SonarQube/SonarCloud analysis with Go coverage

## Project Overview

Single Go module (`github.com/jylhis/makework`) with:
- **`cmd/mw/`** — binary entry point
- **`internal/cli/`** — Cobra CLI definitions and dispatch
- **`internal/config/`** — config.toml loading and management
- **`internal/catalog/`** — repo registry (catalog.toml), sync, add, resolution
- **`internal/resolver/`** — weighted fuzzy project resolution (fuzzy + frecency + activity + context)
- **`internal/worktree/`** — git worktree path computation, creation, listing, sparse-checkout
- **`internal/repo/`** — git shell-out wrappers and URL parsing
- **`internal/nix/`** — nix environment detection
- **`internal/project/`** — per-project .makework.toml schema
- **`internal/status/`** — worktree status and caching
- **`internal/search/`** — grep across worktrees
- **`internal/query/`** — git log activity queries
- **`internal/template/`** — template file application
- **`internal/maintenance/`** — git maintenance registration
- **`internal/xdgpath/`** — XDG directory resolution
- **`internal/buildinfo/`** — version info via ldflags

Key dependencies: cobra, pelletier/go-toml/v2, agnivade/levenshtein, cobra/doc.

## Configuration Files
- `$XDG_CONFIG_HOME/makework/config.toml` — global config
- `$XDG_CONFIG_HOME/makework/catalog.toml` — repo registry
- Per-project `.makework.toml` — subprojects, sparse-checkout paths
- `$XDG_STATE_HOME/makework/` — status cache, visits.json
