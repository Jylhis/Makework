# Non-flake entry point.  Uses flake-compat to read flake.lock,
# so `nix-build default.nix -A makework` works without experimental features.
let
  lock = builtins.fromJSON (builtins.readFile ./flake.lock);
  compatNode = lock.nodes.${lock.nodes.root.inputs.flake-compat};
  compat = import (fetchTarball {
    url = compatNode.locked.url;
    sha256 = compatNode.locked.narHash;
  });
in
(compat { src = ./.; }).defaultNix.packages.${builtins.currentSystem}
