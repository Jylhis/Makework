# AGENTS.md

Guide for agents (human or AI) working in this repository.

## What this repo is

Makework (`mw`) is a Go CLI that manages git worktrees across many repos:
fuzzy + frecency navigation, monorepo-aware sparse-checkout, automatic Nix
environment activation, and cross-project ops (`search`, `query`, `fetch`).
See [`README.md`](./README.md) for the user-facing pitch and
[`CLAUDE.md`](./CLAUDE.md) for the package map.

## Engineering canon

This project follows the Jylhis engineering canon. Source of truth:

- [`Jylhis/virt-corp` `docs/ENGINEERING_PRINCIPLES.md`](https://github.com/Jylhis/virt-corp/blob/main/docs/ENGINEERING_PRINCIPLES.md)
  — 15 principles. Read these before opening a PR.
- [`Jylhis/virt-corp` `docs/WAY_OF_WORKING.md`](https://github.com/Jylhis/virt-corp/blob/main/docs/WAY_OF_WORKING.md)
  — 8 day-to-day norms.

If the canon and a local rule conflict, the canon wins; raise the conflict on
the source repo, do not silently drift.

## Dev loop

`devenv` provides Go, golangci-lint, gopls, `just`, and treefmt. Once you are
in the dev shell:

```sh
just dev      # enter the dev shell (devenv shell)
just test     # go test -race ./...
just lint     # golangci-lint run
just fmt      # treefmt (gofmt + nixfmt)
just check    # full flake check (build, lint, tests, fmt)
just build    # nix build
```

Run `go test -run TestName ./internal/pkg/...` for a single test. Use
`go build ./cmd/mw` if you need a raw Go build without Nix.

## Conventions

- **Commits:** Conventional Commits (`feat:`, `fix:`, `refactor:`, `chore:`,
  `docs:`, `test:`). Scope when it helps: `refactor(status): ...`.
- **Formatting:** treefmt enforces gofmt + nixfmt. Run `just fmt` before
  committing.
- **Linting:** `golangci-lint` config in `.golangci.yml`. CI runs lint, tests,
  and a Nix build on every PR.
- **Testing:** Prefer table-driven Go tests next to the code under test.
  CLI-level flows live in `internal/cli/*_test.go`.

## Working with the CLI

For day-to-day use of `mw` itself (including by agents navigating this and
other Jylhis repos), see
[`.claude/skills/makework.md`](./.claude/skills/makework.md). It is the
authoritative cheat sheet for the command surface.

## Definition of done

- `just check` passes locally (or you have explained why CI will).
- New behaviour has a test that fails without the change.
- Conventional Commit message; PR description links the driving issue.
- No new direct `os/exec` for git — go through `internal/repo` so behaviour
  stays consistent across packages.
