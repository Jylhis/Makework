# NixOS module for makework.
#
# Usage (flake):
#   imports = [ makework.nixosModules.default ];
#   programs.makework.enable = true;
#
# This module installs the binary + completions + man/info pages
# system-wide and optionally appends the shell visit-tracking hook to
# /etc/bashrc and /etc/zshrc. Per-user config.toml is NOT written: the
# file is per-user by design, so declarative config belongs in the
# home-manager module.
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
        assertion = cfg.settings == { };
        message = ''
          programs.makework.settings is per-user configuration and cannot be
          applied system-wide. Move it to the home-manager module instead.
        '';
      }
    ];

    environment.systemPackages = [ cfg.package ];

    programs.bash.interactiveShellInit = lib.mkIf cfg.enableBashIntegration ''
      if [ -r ${common.bashHookPath cfg} ]; then
        . ${common.bashHookPath cfg}
      fi
    '';

    programs.zsh.interactiveShellInit = lib.mkIf cfg.enableZshIntegration ''
      if [ -r ${common.zshHookPath cfg} ]; then
        . ${common.zshHookPath cfg}
      fi
    '';
  };
}
