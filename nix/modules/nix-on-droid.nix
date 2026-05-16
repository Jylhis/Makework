# nix-on-droid module for makework.
#
# Usage (flake):
#   imports = [ makework.nixOnDroidModules.default ];
#   programs.makework.enable = true;
#
# nix-on-droid uses environment.packages (not .systemPackages) and does
# not expose programs.{bash,zsh}.interactiveShellInit — users who want
# the shell hook should run home-manager inside nix-on-droid and use
# this project's homeManagerModules.default there.
{ self }:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  common = import ./common.nix { inherit lib pkgs; };
  cfg = config.programs.makework;
in
{
  options.programs.makework = common.mkOptions {
    defaultPackage = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = common.mkConfigFile cfg == null;
        message = ''
          programs.makework.settings is per-user configuration; set it in
          the home-manager module running inside nix-on-droid instead.
        '';
      }
    ];

    environment.packages = [ cfg.package ];
  };
}
