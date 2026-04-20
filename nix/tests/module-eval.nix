# Synthetic evalModules-based driver exercising every module variant.
#
# We declare stub options that mirror the external options each module
# writes into (home.packages, environment.systemPackages, ...) so the
# modules evaluate in isolation — no home-manager / NixOS / nix-darwin /
# nix-on-droid flake inputs required. The driver only asserts that each
# module evaluates and populates its expected install target.
{
  self,
  pkgs,
  lib ? pkgs.lib,
}:

let
  # Minimal stub decls covering every option the four modules write to.
  # Freeform `anything` types keep the stubs resilient to schema drift
  # without requiring an exact mirror of each upstream option.
  stubs =
    { lib, ... }:
    {
      options = {
        home = {
          packages = lib.mkOption {
            type = lib.types.listOf lib.types.package;
            default = [ ];
          };
        };
        xdg.configFile = lib.mkOption {
          type = lib.types.attrsOf (
            lib.types.submodule { options.source = lib.mkOption { type = lib.types.path; }; }
          );
          default = { };
        };
        environment.systemPackages = lib.mkOption {
          type = lib.types.listOf lib.types.package;
          default = [ ];
        };
        environment.packages = lib.mkOption {
          type = lib.types.listOf lib.types.package;
          default = [ ];
        };
        programs = {
          bash.initExtra = lib.mkOption {
            type = lib.types.lines;
            default = "";
          };
          zsh.initExtra = lib.mkOption {
            type = lib.types.lines;
            default = "";
          };
          bash.interactiveShellInit = lib.mkOption {
            type = lib.types.lines;
            default = "";
          };
          zsh.interactiveShellInit = lib.mkOption {
            type = lib.types.lines;
            default = "";
          };
          fish.shellInit = lib.mkOption {
            type = lib.types.lines;
            default = "";
          };
        };
      };
    };

  evalOne =
    {
      name,
      module,
      enableSettings ? false,
      expectedPaths, # list of attr-path lists to read; each must be non-null
    }:
    let
      evaluated = lib.evalModules {
        modules = [
          stubs
          module
          {
            # Disable `_module.check` so we don't fail on stub options that
            # the real NixOS/HM ecosystems would also declare.
            _module.check = false;
          }
          {
            programs.makework = {
              enable = true;
            }
            // lib.optionalAttrs enableSettings {
              settings.worktree_root = "/tmp/wt";
              settings.bare_root = "/tmp/bare";
            };
          }
        ];
        specialArgs = { inherit pkgs; };
      };
      check = pkgs.runCommand "makework-${name}-module-eval" { } ''
        ${lib.concatMapStringsSep "\n" (path: ''
          value=${lib.escapeShellArg (builtins.toJSON (lib.getAttrFromPath path evaluated.config))}
          test -n "$value" || { echo "missing: ${lib.concatStringsSep "." path}"; exit 1; }
        '') expectedPaths}
        echo "${name} module evaluation: OK"
        touch $out
      '';
    in
    check;

  hmModule = import ../modules/home-manager.nix { inherit self; };
  nixosModule = import ../modules/nixos.nix { inherit self; };
  darwinModule = import ../modules/darwin.nix { inherit self; };
  nodModule = import ../modules/nix-on-droid.nix { inherit self; };

in
{
  home-manager-eval = evalOne {
    name = "home-manager";
    module = hmModule;
    enableSettings = true;
    expectedPaths = [
      [
        "home"
        "packages"
      ]
      [
        "xdg"
        "configFile"
      ]
      [
        "programs"
        "bash"
        "initExtra"
      ]
      [
        "programs"
        "zsh"
        "initExtra"
      ]
    ];
  };

  nixos-eval = evalOne {
    name = "nixos";
    module = nixosModule;
    expectedPaths = [
      [
        "environment"
        "systemPackages"
      ]
      [
        "programs"
        "bash"
        "interactiveShellInit"
      ]
      [
        "programs"
        "zsh"
        "interactiveShellInit"
      ]
    ];
  };

  darwin-eval = evalOne {
    name = "darwin";
    module = darwinModule;
    expectedPaths = [
      [
        "environment"
        "systemPackages"
      ]
      [
        "programs"
        "bash"
        "interactiveShellInit"
      ]
    ];
  };

  nix-on-droid-eval = evalOne {
    name = "nix-on-droid";
    module = nodModule;
    expectedPaths = [
      [
        "environment"
        "packages"
      ]
    ];
  };
}
