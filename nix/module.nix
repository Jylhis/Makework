flake:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.makework;
in
{
  options.programs.makework = {
    enable = lib.mkEnableOption "Makework git worktree manager";

    package = lib.mkPackageOption pkgs "makework" {
      default = flake.packages.${pkgs.system}.default;
    };

    settings = {
      worktreeRoot = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Default worktree root directory.";
      };
      bareRoot = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Default bare repository root directory.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];
  };
}
