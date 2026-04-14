# Flake checks for application-level validation.
# Called from flake.nix with { pkgs, makework, src }.
{
  pkgs,
  makework,
  src ? pkgs.lib.cleanSource ./..,
}:
let
  version = (builtins.fromTOML (builtins.readFile ../Cargo.toml)).workspace.package.version;

  rustCheck =
    args:
    pkgs.rustPlatform.buildRustPackage (
      {
        pname = "makework-check";
        inherit version src;
        cargoLock.lockFile = ../Cargo.lock;
        doInstall = false;
      }
      // args
    );
in
{
  build = makework;

  clippy = rustCheck {
    pname = "makework-clippy";
    nativeBuildInputs = [ pkgs.clippy ];
    buildPhase = ''
      cargo clippy --workspace --all-targets -- -D warnings
    '';
    doCheck = false;
    installPhase = "touch $out";
  };

  tests = rustCheck {
    pname = "makework-tests";
    buildPhase = "true";
    doCheck = true;
    installPhase = "touch $out";
  };

  formatting =
    pkgs.runCommand "makework-fmt-check"
      {
        nativeBuildInputs = [
          pkgs.rustfmt
          pkgs.cargo
        ];
        inherit src;
      }
      ''
        cd $src
        cargo fmt --all --check
        touch $out
      '';
}
