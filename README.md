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

```sh
devenv shell                # enter dev environment
cargo install --path crates/mw-cli
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
devenv shell -- cargo test
devenv shell -- cargo clippy --all-targets -- -D warnings
devenv shell -- cargo fmt
```

See [`CLAUDE.md`](./CLAUDE.md) for the workspace layout.
