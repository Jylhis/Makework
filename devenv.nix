{ pkgs, config, ... }:

{
  packages = [
    pkgs.cargo-mutants
  ];
  claude.code = {

    enable = true;
    mcpServers = {
      # Local devenv MCP server
      devenv = {
        type = "stdio";
        command = "devenv";
        args = [ "mcp" ];
        env = {
          DEVENV_ROOT = config.devenv.root;
        };
      };
    };
  };

  languages.rust = {
    enable = true;
    channel = "stable";
  };

  pre-commit.hooks = {
    clippy.enable = true;
    rustfmt.enable = true;
    nixfmt.enable = true;
  };

}
