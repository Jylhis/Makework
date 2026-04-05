{ pkgs, config, ... }:

let
  crate2nix = config.lib.getInput {
    name = "crate2nix";
    url = "github:nix-community/crate2nix";
    attribute = "languages.rust.import";
    follows = [ "nixpkgs" ];
  };

  crate2nixTools = pkgs.callPackage "${crate2nix}/tools.nix" { };

  cargoNix = pkgs.callPackage
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
  };
}
