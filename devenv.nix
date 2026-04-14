{ pkgs, config, ... }:

let
  crate2nix = config.lib.getInput {
    name = "crate2nix";
    url = "github:nix-community/crate2nix";
    attribute = "languages.rust.import";
    follows = [ "nixpkgs" ];
  };

  makework = import ./nix/package.nix {
    inherit pkgs;
    crate2nixSrc = crate2nix;
    rustToolchain = config.languages.rust.toolchainPackage;
  };
in
{
  packages = [
    pkgs.cargo-mutants
    pkgs.just
  ];

  outputs = {
    inherit makework;
  };

  # Binary caching via Cachix — pulls prebuilt artifacts from the
  # `jylhis` cache. Pushing is opt-in via `devenv.local.nix` or by
  # setting `CACHIX_AUTH_TOKEN` in CI; see devenv.sh/binary-caching.
  cachix = {
    enable = true;
    pull = [ "jylhis" ];
  };

  claude.code = {
    enable = true;
    mcpServers = {
      devenv = {
        type = "stdio";
        command = "devenv";
        args = [ "mcp" ];
        env = {
          DEVENV_ROOT = config.devenv.root;
        };
      };
    };
  };

  languages.rust = {
    enable = true;
    channel = "stable";
  };

  treefmt = {
    enable = true;
    config.programs = import ./nix/treefmt.nix;
  };

  # Tests that validate the dev environment is correct and functional.
  # Application-level checks (clippy, tests, formatting) live in
  # nix/checks.nix and run via `nix flake check`.
  enterTest = ''
    set -euo pipefail

    echo "1/5: cargo is available"
    cargo --version

    echo "2/5: rustc is available and reports a stable version"
    rustc --version | grep -v nightly

    echo "3/5: cargo-mutants is installed"
    cargo mutants --version

    echo "4/5: treefmt is available"
    treefmt --version

    echo "5/5: DEVENV_ROOT is exported and points at this repo"
    test -n "''${DEVENV_ROOT:-}"
    test -f "$DEVENV_ROOT/Cargo.toml"

    echo "All 5 dev environment checks passed."
  '';
}
