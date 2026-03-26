flake:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.makework;

  catalogEntryType = lib.types.submodule {
    options = {
      url = lib.mkOption {
        type = lib.types.str;
        description = "Git remote URL for the repository.";
      };
      mainBranch = lib.mkOption {
        type = lib.types.str;
        default = "main";
        description = "Default branch name.";
      };
    };
  };

  catalogToml =
    let
      repoEntries = lib.mapAttrsToList (
        name: entry: ''
          [repos.${name}]
          path = "${cfg.settings.bareRoot}/${name}.git"
          url = "${entry.url}"
          main_branch = "${entry.mainBranch}"
        ''
      ) cfg.catalog;
    in
    lib.concatStringsSep "\n" repoEntries;

  shellIntegrationBash = ''
    # Makework shell integration
    if command -v mw &>/dev/null; then
      mw() {
        local output
        output=$(command mw "$@") || return $?
        case "$1" in
          go)
            local path nix_cmd nix_dir
            path=$(printf '%s' "$output" | sed -n '1p')
            nix_cmd=$(printf '%s' "$output" | sed -n '2p')
            nix_dir=$(printf '%s' "$output" | sed -n '3p')
            [ -n "$path" ] && cd "$path" || return $?
            if [ -n "$nix_cmd" ]; then
              cd "''${nix_dir:-$path}" && eval "$nix_cmd"
            fi
            ;;
          *) [ -n "$output" ] && echo "$output" ;;
        esac
      }
    fi
  '';

  shellIntegrationZsh = shellIntegrationBash;

  shellIntegrationFish = ''
    # Makework shell integration
    if command -v mw &>/dev/null
      function mw
        set -l output (command mw $argv); or return $status
        switch $argv[1]
          case go
            set -l path (echo "$output" | sed -n '1p')
            set -l nix_cmd (echo "$output" | sed -n '2p')
            set -l nix_dir (echo "$output" | sed -n '3p')
            test -n "$path"; and cd $path; or return $status
            if test -n "$nix_cmd"
              cd (test -n "$nix_dir" && echo "$nix_dir" || echo "$path")
              eval $nix_cmd
            end
          case '*'
            test -n "$output"; and echo $output
        end
      end
    end
  '';
in
{
  options.programs.makework = {
    enable = lib.mkEnableOption "Makework git worktree manager";

    package = lib.mkPackageOption pkgs "makework" {
      default = flake.packages.${pkgs.system}.default;
    };

    settings = {
      worktreeRoot = lib.mkOption {
        type = lib.types.str;
        default = "~/.local/share/makework/worktrees";
        description = "Root directory for worktrees.";
      };
      bareRoot = lib.mkOption {
        type = lib.types.str;
        default = "~/.local/share/makework/repos";
        description = "Root directory for bare repositories.";
      };
    };

    catalog = lib.mkOption {
      type = lib.types.attrsOf catalogEntryType;
      default = { };
      description = ''
        Declarative catalog entries. Keys are repository names.
        Note: these are overwritten on each `home-manager switch`.
        Use either declarative catalog or `mw catalog add`, not both.
      '';
      example = lib.literalExpression ''
        {
          my-project = {
            url = "git@github.com:user/my-project.git";
            mainBranch = "main";
          };
        }
      '';
    };

    enableBashIntegration = lib.mkEnableOption "Bash shell integration" // {
      default = true;
    };
    enableZshIntegration = lib.mkEnableOption "Zsh shell integration" // {
      default = true;
    };
    enableFishIntegration = lib.mkEnableOption "Fish shell integration";
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    xdg.configFile."makework/config.toml".text = ''
      [config]
      worktree_root = "${cfg.settings.worktreeRoot}"
      bare_root = "${cfg.settings.bareRoot}"
    '';

    xdg.configFile."makework/catalog.toml" = lib.mkIf (cfg.catalog != { }) { text = catalogToml; };

    programs.bash.initExtra = lib.mkIf cfg.enableBashIntegration shellIntegrationBash;
    programs.zsh.initExtra = lib.mkIf cfg.enableZshIntegration shellIntegrationZsh;
    programs.fish.interactiveShellInit = lib.mkIf cfg.enableFishIntegration shellIntegrationFish;
  };
}
