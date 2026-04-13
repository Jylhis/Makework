# Shared build logic for makework.
# Called from devenv.nix, default.nix, and flake.nix — each provides
# its own pkgs and crate2nix source.
{
  pkgs,
  crate2nixSrc,
  rustToolchain ? null,
  src ? pkgs.lib.cleanSource ./..,
}:
let
  crate2nixTools = pkgs.callPackage "${crate2nixSrc}/tools.nix" { };

  cargoNix =
    pkgs.callPackage
      (crate2nixTools.generatedCargoNix {
        name = "makework";
        inherit src;
      })
      (
        pkgs.lib.optionalAttrs (rustToolchain != null) {
          buildRustCrateForPkgs =
            _:
            pkgs.buildRustCrate.override {
              rustc = rustToolchain;
              cargo = rustToolchain;
            };
        }
      );
in
cargoNix.workspaceMembers.mw-cli.build
