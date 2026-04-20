# nix-darwin module for makework.
#
# Usage (flake):
#   imports = [ makework.darwinModules.default ];
#   programs.makework.enable = true;
#
# nix-darwin exposes the same environment.systemPackages and
# programs.{bash,zsh}.interactiveShellInit options as NixOS, so the
# module body is identical. This file re-exports the NixOS module
# verbatim. If nix-darwin ever diverges, split them then.
import ./nixos.nix
