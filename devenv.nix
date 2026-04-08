{ pkgs, config, ... }:

let
  crate2nix = config.lib.getInput {
    name = "crate2nix";
    url = "github:nix-community/crate2nix";
    attribute = "languages.rust.import";
    follows = [ "nixpkgs" ];
  };

  crate2nixTools = pkgs.callPackage "${crate2nix}/tools.nix" { };

  cargoNix =
    pkgs.callPackage
      (crate2nixTools.generatedCargoNix {
        name = "makework";
        src = pkgs.lib.cleanSource ./.;
      })
      {
        buildRustCrateForPkgs =
          _:
          pkgs.buildRustCrate.override {
            rustc = config.languages.rust.toolchainPackage;
            cargo = config.languages.rust.toolchainPackage;
          };
      };

  makework = cargoNix.workspaceMembers.mw-cli.build;
in
{
  packages = [
    pkgs.cargo-mutants
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
    config.programs = {
      nixfmt.enable = true;
      rustfmt.enable = true;
    };
  };

  # Tests that validate the dev environment. Run with `devenv test`.
  enterTest = ''
    set -euo pipefail

    echo "1/7: cargo is available"
    cargo --version

    echo "2/7: rustc is available and reports a stable version"
    rustc --version | grep -v nightly

    echo "3/7: cargo-mutants is installed"
    cargo mutants --version

    echo "4/7: treefmt is available"
    treefmt --version

    echo "5/7: project type-checks via cargo check"
    cargo check --workspace --all-targets

    echo "6/7: workspace formatting is clean"
    treefmt --ci

    echo "7/7: DEVENV_ROOT is exported and points at this repo"
    test -n "''${DEVENV_ROOT:-}"
    test -f "$DEVENV_ROOT/Cargo.toml"

    echo "All 7 dev environment checks passed."
  '';
}
