{
  description = "makework – opinionated git worktree manager";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
          pkgs = nixpkgs.legacyPackages.${system};
          makework = import ./nix/package.nix { inherit pkgs; };
          docs = import ./nix/docs/options.nix { inherit pkgs self; };
        in
        {
          default = makework;
          inherit makework docs;
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

          moduleChecks = import ./nix/tests/module-eval.nix { inherit self pkgs; };
        in
        appChecks
        // {
          inherit flake-parts-module-eval;
          home-manager-module-eval = moduleChecks.home-manager-eval;
          nixos-module-eval = moduleChecks.nixos-eval;
          darwin-module-eval = moduleChecks.darwin-eval;
          nix-on-droid-module-eval = moduleChecks.nix-on-droid-eval;
        }
      );

      flakeModules.default = import ./nix/flake-parts-module.nix;

      nixosModules.default = import ./nix/modules/nixos.nix { inherit self; };
      darwinModules.default = import ./nix/modules/darwin.nix { inherit self; };
      homeManagerModules.default = import ./nix/modules/home-manager.nix { inherit self; };
      nixOnDroidModules.default = import ./nix/modules/nix-on-droid.nix { inherit self; };

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
