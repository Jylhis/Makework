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
  version = (builtins.fromTOML (builtins.readFile ../Cargo.toml)).workspace.package.version;

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

  mw-bin = cargoNix.workspaceMembers.mw-cli.build;
in
pkgs.symlinkJoin {
  name = "makework-${version}";
  paths = [ mw-bin ];
  nativeBuildInputs = [
    pkgs.installShellFiles
    pkgs.texinfoInteractive
  ];
  postBuild = ''
    # Man pages
    mkdir -p man1
    $out/bin/mw man --output-dir man1
    installManPage man1/*.1

    # Shell completions (packaging mode — no wrapper functions)
    $out/bin/mw completions bash --output-dir .
    installShellCompletion --bash mw
    $out/bin/mw completions zsh --output-dir .
    installShellCompletion --zsh _mw
    $out/bin/mw completions fish --output-dir .
    installShellCompletion --fish mw.fish

    # Info pages
    $out/bin/mw generate-texi --output mw.texi
    makeinfo mw.texi -o mw.info
    install -Dm644 mw.info $out/share/info/mw.info
  '';
}
