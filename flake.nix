{
  description = "makework – opinionated project scaffolding tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/13043924aaa7375ce482ebe2494338e058282925";
    crate2nix = {
      url = "github:nix-community/crate2nix/0.15.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      crate2nix,
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
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
    };
}
