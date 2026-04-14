default:
    @just --list --justfile {{justfile()}}

_sed_inplace := if os() == "macos" { "sed -i ''" } else { "sed -i" }

# Build the default package via flake
build:
    nix build

# Build via nix-build (non-flake, uses flake-compat)
build-legacy:
    nix-build default.nix -A makework

# Run all flake checks
check:
    nix flake check

# Format all code (Nix + Rust)
fmt:
    nix fmt

# Update all flake inputs and sync devenv lock
update:
    nix flake update
    just sync

# Sync devenv.lock to match flake.lock nixpkgs pin
sync:
    #!/usr/bin/env bash
    set -euo pipefail
    NIXPKGS_NODE=$(jq -r '.nodes.root.inputs.nixpkgs' flake.lock)
    REV=$(jq -r ".nodes.\"$NIXPKGS_NODE\".locked.rev" flake.lock)
    echo "Syncing devenv to nixpkgs $REV"
    {{ _sed_inplace }} "s|url: github:NixOS/nixpkgs/.*|url: github:NixOS/nixpkgs/$REV|" devenv.yaml
    devenv update
    echo "Done. Both locks pinned to $REV"

# Verify both lock files point to the same nixpkgs rev
verify:
    #!/usr/bin/env bash
    set -euo pipefail
    FLAKE_NODE=$(jq -r '.nodes.root.inputs.nixpkgs' flake.lock)
    FLAKE_REV=$(jq -r ".nodes.\"$FLAKE_NODE\".locked.rev" flake.lock)
    DEVENV_NODE=$(jq -r '.nodes.root.inputs.nixpkgs' devenv.lock)
    DEVENV_REV=$(jq -r ".nodes.\"$DEVENV_NODE\".locked.rev" devenv.lock)
    echo "flake:  $FLAKE_REV"
    echo "devenv: $DEVENV_REV"
    if [ "$FLAKE_REV" != "$DEVENV_REV" ]; then
        echo "ERROR: nixpkgs revisions are out of sync"
        exit 1
    fi
    echo "All lock files in sync."

# Run devenv environment validation
test-env:
    devenv test

# Run cargo tests
test:
    cargo test --workspace

# Lint with clippy
lint:
    cargo clippy --all-targets -- -D warnings
