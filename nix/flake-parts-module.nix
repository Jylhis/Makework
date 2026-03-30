{
  self,
  lib,
  flake-parts-lib,
  ...
}:
let
  inherit (lib) mkEnableOption mkOption types mkIf;
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
    {
      options.makework = {
        enable = mkEnableOption "makework worktree manager";

        package = mkOption {
          type = types.package;
          default = self.packages.${system}.default;
          description = "The makework package to use.";
        };

        settings = {
          worktreeRoot = mkOption {
            type = types.nullOr types.str;
            default = null;
            description = "Default worktree root directory.";
          };
          bareRoot = mkOption {
            type = types.nullOr types.str;
            default = null;
            description = "Default bare repository root directory.";
          };
        };
      };

      config = mkIf config.makework.enable {
        packages.makework = config.makework.package;

        devShells.default = pkgs.mkShell {
          packages = [ config.makework.package ];
        };
      };
    }
  );
}
