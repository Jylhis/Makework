{
  description = "makework – opinionated project scaffolding tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    crate2nix = {
      url = "github:nix-community/crate2nix/0.15.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    flake-compat = {
      url = "https://flakehub.com/f/edolstra/flake-compat/1.tar.gz";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      crate2nix,
      treefmt-nix,
      flake-parts,
      flake-compat,
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          makework = import ./nix/package.nix {
            pkgs = nixpkgs.legacyPackages.${system};
            crate2nixSrc = crate2nix;
          };
        in
        {
          default = makework;
          inherit makework;
        }
      );

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          appChecks = import ./nix/checks.nix {
            inherit pkgs;
            makework = self.packages.${system}.makework;
          };

          # Verify the flake-parts module evaluates in a minimal flake-parts flake.
          flake-parts-module-eval =
            let
              testFlake = flake-parts.lib.mkFlake { inputs = { inherit self nixpkgs flake-parts; }; } {
                systems = [ system ];
                imports = [ self.flakeModules.default ];
                perSystem =
                  { pkgs, ... }:
                  {
                    makework.enable = true;
                    # Use a lightweight package to avoid building the real one in tests.
                    makework.package = pkgs.hello;
                  };
              };
            in
            pkgs.runCommand "flake-parts-module-eval" { } ''
              # Assert the test flake produced a devShell for this system.
              test -n "${testFlake.devShells.${system}.default}" \
                && echo "flake-parts module evaluation: OK" \
                && touch $out
            '';
        in
        appChecks // { inherit flake-parts-module-eval; }
      );

      flakeModules.default = import ./nix/flake-parts-module.nix;

      formatter = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          treefmtEval = treefmt-nix.lib.evalModule pkgs {
            projectRootFile = "flake.nix";
            programs = import ./nix/treefmt.nix;
          };
        in
        treefmtEval.config.build.wrapper
      );
    };
}
