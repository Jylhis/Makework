# Renders programs.makework option docs for every module variant into an
# HTML site consumable by GitHub Pages.
{
  pkgs,
  self,
  lib ? pkgs.lib,
}:

let
  selfRoot = toString self;

  # Rewrite on-disk declaration paths to GitHub URLs so the generated docs
  # link back to the source on the main branch.
  declToGithub =
    d:
    let
      s = toString d;
      rel = lib.removePrefix (selfRoot + "/") s;
    in
    if lib.hasPrefix selfRoot s then "https://github.com/jylhis/makework/blob/main/${rel}" else s;

  evalFor =
    moduleFile:
    (lib.evalModules {
      modules = [
        (import ./stubs.nix)
        moduleFile
        { _module.check = false; }
      ];
      specialArgs = { inherit pkgs; };
    }).options;

  mkDoc =
    { moduleFile }:
    pkgs.nixosOptionsDoc {
      options = (evalFor moduleFile).programs.makework or { };
      transformOptions = opt: opt // { declarations = map declToGithub opt.declarations; };
    };

  hmModule = import ../modules/home-manager.nix { inherit self; };
  nixosModule = import ../modules/nixos.nix { inherit self; };
  darwinModule = import ../modules/darwin.nix { inherit self; };
  nodModule = import ../modules/nix-on-droid.nix { inherit self; };

  docs = {
    home-manager = mkDoc { moduleFile = hmModule; };
    nixos = mkDoc { moduleFile = nixosModule; };
    darwin = mkDoc { moduleFile = darwinModule; };
    nix-on-droid = mkDoc { moduleFile = nodModule; };
  };

  contextLabels = {
    home-manager = "home-manager";
    nixos = "NixOS";
    darwin = "nix-darwin";
    nix-on-droid = "nix-on-droid";
  };

in
pkgs.runCommand "makework-options-site"
  {
    nativeBuildInputs = [ pkgs.pandoc ];
  }
  ''
    mkdir -p $out
    cp ${./index.html} $out/index.html
    cp ${./style.css} $out/style.css

    ${lib.concatMapStringsSep "\n" (ctx: ''
      cp ${docs.${ctx}.optionsCommonMark} $out/${ctx}.md
      pandoc -f commonmark+pipe_tables -t html5 -s \
        --metadata title="makework — ${contextLabels.${ctx}} options" \
        --css style.css \
        -o $out/${ctx}.html $out/${ctx}.md
    '') (lib.attrNames docs)}
  ''
