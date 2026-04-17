# Shared treefmt program configuration.
# Imported by both flake.nix (formatter output) and devenv.nix (treefmt module).
{
  nixfmt.enable = true;
  rustfmt.enable = true;
  gofmt.enable = true;
}
