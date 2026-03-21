{ pkgs, lib, config, inputs, ... }:

{
  packages = [
    pkgs.cargo-mutants
  ];

  languages.rust.enable = true;

  pre-commit.hooks = {
    clippy.enable = true;
    rustfmt.enable = true;
  };
}
