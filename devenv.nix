{ pkgs, ... }:

{
  packages = [
    pkgs.cargo-mutants
  ];

  languages.rust = {
    enable = true;
    channel = "stable";
  };

  pre-commit.hooks = {
    clippy.enable = true;
    rustfmt.enable = true;
  };
}
