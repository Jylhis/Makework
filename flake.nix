{
  description = "makework – opinionated project scaffolding tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/13043924aaa7375ce482ebe2494338e058282925";
    crate2nix = {
      url = "github:nix-community/crate2nix/0.15.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
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
        import ./nix/checks.nix {
          pkgs = nixpkgs.legacyPackages.${system};
          makework = self.packages.${system}.makework;
        }
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
