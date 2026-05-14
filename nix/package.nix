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
    vendorHash = "sha256-nDLncfH+2TbcuHqjd4bUXyh8FqgIILZWJP3kBzwzzrA=";
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

    # Shell completions
    installShellCompletion --cmd mw \
      --bash <($out/bin/mw completion bash) \
      --zsh  <($out/bin/mw completion zsh) \
      --fish <($out/bin/mw completion fish)

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
