# Stub option decls for option-doc eval.
#
# nixosOptionsDoc calls lib.evalModules, which would otherwise fail
# because our modules write into home.packages / environment.systemPackages
# etc. — options declared by home-manager / NixOS / nix-darwin / nix-on-droid
# themselves. We don't want to drag those full option trees in, so we stub
# the minimum with visible = false so they never leak into the final docs.
{ lib, ... }:
let
  stub = lib.mkOption {
    type = lib.types.anything;
    default = null;
    visible = false;
  };
in
{
  options = {
    home.packages = stub;
    xdg.configFile = stub;
    environment.systemPackages = stub;
    environment.packages = stub;
    programs = {
      bash.initExtra = stub;
      zsh.initExtra = stub;
      bash.interactiveShellInit = stub;
      zsh.interactiveShellInit = stub;
      fish.shellInit = stub;
    };
    assertions = stub;
  };
}
