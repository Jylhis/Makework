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
    vendorHash = "sha256-l/ugqCaggFCsVwr3ldZxi4lM0LJlBY3sNM2tu7xB0go=";
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
    pkgs.texinfo
  ];
  postBuild = ''
    # Man pages
    mkdir -p man1
    $out/bin/mw man --output-dir man1
    installManPage man1/*.1

    # Info pages (stdenv's install-info hook registers them in fixupPhase)
    $out/bin/mw generate-texi --output mw.texi
    makeinfo mw.texi -o mw.info
    install -D -m 0644 mw.info $out/share/info/mw.info

    # Shell completions (packaging mode — no wrapper functions)
    $out/bin/mw completions bash --output-dir .
    installShellCompletion --bash mw
    $out/bin/mw completions zsh --output-dir .
    installShellCompletion --zsh _mw
    $out/bin/mw completions fish --output-dir .
    installShellCompletion --fish mw.fish
    $out/bin/mw completions powershell --output-dir .
    install -D -m 0644 mw.ps1 $out/share/powershell/mw.ps1

    # Shell init hook snippets — modules source these so hook bodies
    # stay defined once in the Go binary (internal/cli/shellinit.go).
    mkdir -p $out/share/makework/init
    $out/bin/mw init bash > $out/share/makework/init/mw-hook.bash
    $out/bin/mw init zsh  > $out/share/makework/init/mw-hook.zsh
  '';

  meta = {
    description = "Opinionated git worktree manager";
    homepage = "https://github.com/jylhis/makework";
    license = pkgs.lib.licenses.mit;
    mainProgram = "mw";
    platforms = pkgs.lib.platforms.unix;
  };
}
