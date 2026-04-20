# home-manager module for makework.
#
# Usage (flake):
#   home.sharedModules = [ makework.homeManagerModules.default ];
#   programs.makework.enable = true;
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
  configFile = common.mkConfigFile cfg;
in
{
  options.programs.makework = common.mkOptions {
    defaultPackage = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
  };

  config = lib.mkIf cfg.enable (
    lib.mkMerge [
      { home.packages = [ cfg.package ]; }

      (lib.mkIf (configFile != null) {
        xdg.configFile."makework/config.toml".source = configFile;
      })

      (lib.mkIf cfg.enableBashIntegration {
        programs.bash.initExtra = ''
          if [ -r ${common.bashHookPath cfg} ]; then
            . ${common.bashHookPath cfg}
          fi
        '';
      })

      (lib.mkIf cfg.enableZshIntegration {
        programs.zsh.initExtra = ''
          if [ -r ${common.zshHookPath cfg} ]; then
            . ${common.zshHookPath cfg}
          fi
        '';
      })
    ]
  );
}
