{ pkgs, config, ... }:

let
  makework = import ./nix/package.nix {
    inherit pkgs;
  };
in
{
  packages = [
    pkgs.just
    pkgs.golangci-lint
    pkgs.gopls
    pkgs.go-tools
    pkgs.delve
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

  languages.go.enable = true;

  treefmt = {
    enable = true;
    config.programs = import ./nix/treefmt.nix;
  };

  enterTest = ''
    set -euo pipefail

    echo "1/4: go is available"
    go version

    echo "2/4: golangci-lint is available"
    golangci-lint --version

    echo "3/4: treefmt is available"
    treefmt --version

    echo "4/4: DEVENV_ROOT is exported and points at this repo"
    test -n "''${DEVENV_ROOT:-}"
    test -f "$DEVENV_ROOT/go.mod"

    echo "All 4 dev environment checks passed."
  '';
}
