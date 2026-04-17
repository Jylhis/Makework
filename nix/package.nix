# Shared build logic for makework (Go).
# Called from devenv.nix, default.nix, and flake.nix.
{
  pkgs,
  src ? pkgs.lib.cleanSource ./..,
}:
let
  version = pkgs.lib.removeSuffix "\n" (builtins.readFile ../VERSION);

  mw-bin = pkgs.buildGoModule {
    pname = "mw";
    inherit version src;
    vendorHash = "sha256-azOw274pz3XCh8wHJLlWmg7OynutjuvSI7x/Kf63U9M=";
    subPackages = [ "cmd/mw" ];
    ldflags = [
      "-s"
      "-w"
      "-X github.com/jylhis/makework/internal/buildinfo.Version=${version}"
    ];
    # Tests shell out to git and need HOME; run via checks instead.
    doCheck = false;
  };
in
pkgs.symlinkJoin {
  name = "makework-${version}";
  paths = [ mw-bin ];
  nativeBuildInputs = [
    pkgs.installShellFiles
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
  '';
}
