# Shared option declarations and helpers for every makework module variant
# (NixOS / home-manager / nix-darwin / nix-on-droid / flake-parts).
#
# This file is NOT a module — it returns plain helpers that each wrapper
# imports and uses. Keeping it a library function instead of a module lets
# the wrappers own the `options.programs.makework` path assertion and any
# context-specific merge logic without fighting the merge system.
{ lib, pkgs }:

let
  tomlFormat = pkgs.formats.toml { };

  # Schema mirrors internal/config/config.go (Config struct). Typed keys are
  # optional (null-by-default) so the module only writes a config.toml entry
  # for keys the user actually set; a user who sets nothing gets no file.
  settingsType = lib.types.submodule {
    freeformType = tomlFormat.type;
    options = {
      worktree_root = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        example = "/var/lib/worktrees";
        description = "Root directory for worktrees.";
      };
      bare_root = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        example = "/var/lib/repos";
        description = "Root directory for bare clones.";
      };
      scan_roots = lib.mkOption {
        type = lib.types.nullOr (lib.types.listOf lib.types.str);
        default = null;
        description = "Directories scanned by `mw sync` to discover repos.";
      };
      sync_max_depth = lib.mkOption {
        type = lib.types.nullOr lib.types.ints.unsigned;
        default = null;
        description = "Maximum depth of directory scanning during `mw sync`.";
      };
      sync_exclude = lib.mkOption {
        type = lib.types.nullOr (lib.types.listOf lib.types.str);
        default = null;
        description = "Path patterns excluded from `mw sync` scans.";
      };
      template_dir = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Directory of project templates applied by `mw new`.";
      };
      resolver = lib.mkOption {
        default = null;
        description = "Weighted resolver tuning. Weights must sum > 0.";
        type = lib.types.nullOr (
          lib.types.submodule {
            options = {
              weight_fuzzy = lib.mkOption {
                type = lib.types.float;
                default = 0.35;
                description = "Fuzzy-match weight.";
              };
              weight_frecency = lib.mkOption {
                type = lib.types.float;
                default = 0.35;
                description = "Frecency weight.";
              };
              weight_activity = lib.mkOption {
                type = lib.types.float;
                default = 0.15;
                description = "Git activity weight.";
              };
              weight_context = lib.mkOption {
                type = lib.types.float;
                default = 0.15;
                description = "Working-directory context weight.";
              };
            };
          }
        );
      };
    };
  };

  # Drop null-valued keys from the settings attrset so they don't appear in
  # the rendered TOML as `key = null` (which is not valid TOML anyway).
  pruneNulls = attrs: lib.filterAttrs (_: v: v != null) (lib.mapAttrs (_: pruneValue) attrs);

  pruneValue = v: if builtins.isAttrs v && !(v ? _type) then pruneNulls v else v;

in
{
  inherit settingsType;

  # Build the options attrset installed under `programs.makework` in every
  # wrapper. `defaultPackage` varies by context (self.packages.x or pkgs.mw
  # once upstreamed).
  mkOptions =
    { defaultPackage }:
    {
      enable = lib.mkEnableOption "makework — git worktree manager";

      package = lib.mkOption {
        type = lib.types.package;
        default = defaultPackage;
        defaultText = lib.literalExpression "makework flake package";
        description = "The makework package to install.";
      };

      settings = lib.mkOption {
        type = settingsType;
        default = { };
        description = ''
          Contents written to `$XDG_CONFIG_HOME/makework/config.toml`
          under the `[config]` table. Only applies in user-scope contexts
          (home-manager); system-scope modules ignore this attribute because
          the config file is per-user by design.
        '';
        example = lib.literalExpression ''
          {
            worktree_root = "/home/alice/code/worktrees";
            bare_root     = "/home/alice/code/repos";
            scan_roots    = [ "/home/alice/code" ];
          }
        '';
      };

      enableBashIntegration = lib.mkOption {
        type = lib.types.bool;
        default = true;
        description = "Source the bash visit-tracking hook in interactive shells.";
      };

      enableZshIntegration = lib.mkOption {
        type = lib.types.bool;
        default = true;
        description = "Source the zsh visit-tracking hook in interactive shells.";
      };
    };

  # Returns the generated config.toml derivation, or null if the user wrote
  # no settings. Wrappers should gate `xdg.configFile` on the null check so
  # we don't overwrite a user's hand-written file with an empty one.
  mkConfigFile =
    cfg:
    let
      pruned = pruneNulls cfg.settings;
    in
    if pruned == { } then null else tomlFormat.generate "makework-config.toml" { config = pruned; };

  # Shell init hook paths inside the package. The Go binary owns the hook
  # body (see internal/cli/shellinit.go) and writes it to these files
  # during package.nix postBuild.
  bashHookPath = cfg: "${cfg.package}/share/makework/init/mw-hook.bash";
  zshHookPath = cfg: "${cfg.package}/share/makework/init/mw-hook.zsh";
}
