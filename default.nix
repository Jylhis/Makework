# Non-flake entry point.  `nix-build` uses npins; flake.nix has its
# own pure inputs.  Both call nix/package.nix so the build logic is
# defined once.
{
  pkgs ? import (import ./npins).nixpkgs { },
}:
let
  sources = import ./npins;
in
{
  makework = import ./nix/package.nix {
    inherit pkgs;
    crate2nixSrc = sources.crate2nix;
  };
}
