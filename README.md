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

Clone the repo and build the package via devenv:

```sh
git clone https://github.com/Jylhis/makework.git
cd makework
devenv build makework
```

The result is a store path containing `bin/mw`. Install it into your profile:

```sh
nix profile install ./devenv/outputs/makework
```

Or add it to your NixOS configuration (e.g. in a module):

```nix
# flake input
inputs.makework = {
  url = "github:Jylhis/makework";
  inputs.nixpkgs.follows = "nixpkgs";
};

# in your NixOS module
environment.systemPackages = [ inputs.makework.devenv.outputs.makework ];
```

Prebuilt binaries are available from the `jylhis` Cachix cache:

```sh
cachix use jylhis
```

### With cargo

```sh
devenv shell                # enter dev environment (or use Rust stable 1.85+)
cargo install --path crates/mw-cli
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
devenv shell -- cargo test
devenv shell -- cargo clippy --all-targets -- -D warnings
devenv shell -- cargo fmt
```

See [`CLAUDE.md`](./CLAUDE.md) for the workspace layout.
