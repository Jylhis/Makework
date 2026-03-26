{
  description = "Makework - git worktree manager";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      crane,
      rust-overlay,
      ...
    }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      pkgsFor =
        system:
        import nixpkgs {
          inherit system;
          overlays = [
            rust-overlay.overlays.default
            self.overlays.default
          ];
        };

      mkCraneLib =
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ rust-overlay.overlays.default ];
          };
          rustToolchain = pkgs.rust-bin.stable.latest.minimal;
        in
        (crane.mkLib pkgs).overrideToolchain rustToolchain;

      mkCommonArgs =
        system:
        let
          craneLib = mkCraneLib system;
          pkgs = import nixpkgs { inherit system; };
        in
        {
          pname = "makework";
          version = "0.1.0";
          src = craneLib.cleanCargoSource ./.;
          strictDeps = true;
          nativeBuildInputs = [ pkgs.git ];
          buildInputs =
            [ ]
            ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
              pkgs.darwin.apple_sdk.frameworks.Security
              pkgs.darwin.apple_sdk.frameworks.SystemConfiguration
            ];
        };
    in
    {
      overlays.default = final: _prev: {
        makework = self.packages.${final.system}.default;
      };

      packages = forAllSystems (
        system:
        let
          craneLib = mkCraneLib system;
          commonArgs = mkCommonArgs system;
          cargoArtifacts = craneLib.buildDepsOnly commonArgs;
          makework = craneLib.buildPackage (
            commonArgs
            // {
              inherit cargoArtifacts;
              # Integration tests call catalog_add which writes to XDG dirs
              preCheck = ''
                export HOME=$(mktemp -d)
              '';
            }
          );
        in
        {
          default = makework;
          makework = makework;
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = pkgsFor system;
          rustToolchain = pkgs.rust-bin.stable.latest.default;
        in
        {
          default = pkgs.mkShell {
            inputsFrom = [ self.packages.${system}.default ];
            packages = [
              rustToolchain
              pkgs.cargo-mutants
            ];
          };
        }
      );

      checks = forAllSystems (
        system:
        let
          craneLib = mkCraneLib system;
          commonArgs = mkCommonArgs system;
          cargoArtifacts = craneLib.buildDepsOnly commonArgs;
        in
        {
          makework-clippy = craneLib.cargoClippy (
            commonArgs
            // {
              inherit cargoArtifacts;
              cargoClippyExtraArgs = "--all-targets -- -D warnings";
            }
          );
          makework-fmt = craneLib.cargoFmt { src = craneLib.cleanCargoSource ./.; };
          makework-test = craneLib.cargoTest (
            commonArgs
            // {
              inherit cargoArtifacts;
              # Integration tests call catalog_add which writes to XDG dirs
              preCheck = ''
                export HOME=$(mktemp -d)
              '';
            }
          );
        }
      );

      nixosModules.default = import ./nix/module.nix self;
      homeManagerModules.default = import ./nix/home-manager.nix self;
    };
}
