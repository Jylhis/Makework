# flake-parts module for makework.
#
# Usage in a flake-parts flake:
#
#   imports = [ makework.flakeModules.default ];
#
#   perSystem = { ... }: {
#     makework.enable = true;
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
      cfg = config.makework;
    in
    {
      options.makework = {
        enable = lib.mkEnableOption "makework in the devshell";

        package = lib.mkOption {
          type = lib.types.package;
          default = self.packages.${system}.default;
          defaultText = lib.literalExpression "self.packages.\${system}.default";
          description = "The makework package to use.";
        };

        settings = {
          worktree_root = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            description = "Default root directory for worktrees.";
          };

          bare_root = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            description = "Default root directory for bare clones.";
          };
        };
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
