# Makework

A git worktree manager with fuzzy navigation, monorepo support, and Nix
auto-activation. One bare clone per repo, one worktree per branch — switch
between projects without losing your place.

```sh
mw catalog add ~/Developer/myrepo  # register a repo
mw go myrepo                       # cd into a worktree (creates one if needed)
mw go feature@new-ui               # fuzzy match + branch routing
mw                                 # status across all worktrees
```

## Highlights

- **Fuzzy + frecency routing.** `mw go <query>` ranks by name match, recent
  visits, and current directory context — like `zoxide` for repos.
- **Auto Nix activation.** Detects `flake.nix`, `devenv.nix`, `shell.nix`, and
  `.envrc`; runs the right command after `cd`.
- **Monorepo aware.** Per-project `.makework.toml` defines subprojects with
  optional sparse-checkout.
- **Cross-project ops.** `mw search`, `mw query`, `mw fetch` work across every
  registered repo.
- **Integrations.** Shell wrapper (bash/zsh/fish), Emacs package, MCP server
  for AI assistants.

## Install

### With Nix (NixOS / nix-darwin / standalone Nix)

Build straight from the flake:

```sh
nix build github:Jylhis/makework
nix profile install github:Jylhis/makework
```

Or wire it into a NixOS / nix-darwin configuration:

```nix
# flake input
inputs.makework = {
  url = "github:Jylhis/makework";
  inputs.nixpkgs.follows = "nixpkgs";
};

# in your module
environment.systemPackages = [ inputs.makework.packages.${pkgs.system}.default ];
```

Prebuilt binaries are available from the `jylhis` Cachix cache:

```sh
cachix use jylhis
```

### With Go

```sh
devenv shell                # enter dev environment (or use Go 1.23+)
go install github.com/jylhis/makework/cmd/mw@latest
```

### Post-install

```sh
mw catalog init             # create config + state directories
eval "$(mw completions bash)"  # add to .bashrc / .zshrc
```

## Documentation

Full docs live in [`docs/`](./docs/) and render as a Mintlify site. See
[`docs/quickstart.mdx`](./docs/quickstart.mdx) for a 5-minute walkthrough or
[`docs/cli/reference.mdx`](./docs/cli/reference.mdx) for the complete command
list.

## Development

```sh
devenv shell                # enter dev shell (Go, golangci-lint, gopls, just)
just test                   # go test ./...
just lint                   # golangci-lint run
just fmt                    # treefmt (gofmt + nixfmt)
just check                  # full flake check (build, lint, tests, fmt)
```

See [`CLAUDE.md`](./CLAUDE.md) for the workspace layout.
