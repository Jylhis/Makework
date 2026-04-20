# flake-parts module for makework.
#
# Usage in a flake-parts flake:
#
#   imports = [ makework.flakeModules.default ];
#
#   perSystem = { ... }: {
#     makework.enable = true;
#     makework.settings.worktree_root = "/home/user/wt";
#   };
{
  self,
  lib,
  flake-parts-lib,
  ...
}:
let
  inherit (flake-parts-lib) mkPerSystemOption;
in
{
  options.perSystem = mkPerSystemOption (
    {
      config,
      pkgs,
      system,
      ...
    }:
    let
      common = import ./modules/common.nix { inherit lib pkgs; };
      cfg = config.makework;
    in
    {
      options.makework = common.mkOptions {
        defaultPackage = self.packages.${system}.default;
      };

      config = lib.mkIf cfg.enable {
        devShells = lib.mkDefault {
          default = pkgs.mkShell {
            packages = [ cfg.package ];
          };
        };
      };
    }
  );
}
