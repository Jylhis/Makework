default:
    @just --list --justfile {{justfile()}}

# Build the default package via flake
build:
    nix build

# Build via nix-build (non-flake, uses flake-compat)
build-legacy:
    nix-build default.nix -A makework

# Run all flake checks
check:
    nix flake check

# Format all code (Nix + Go)
fmt:
    nix fmt

# Inputs shared between flake.nix and devenv.yaml that must stay in lockstep.
_shared_inputs := "nixpkgs treefmt-nix"

# Update all flake inputs, rewrite devenv pins, and verify
update:
    nix flake update
    just sync
    just verify

# Rewrite devenv.yaml so every shared input pins the same commit as flake.lock
sync:
    #!/usr/bin/env bash
    set -euo pipefail
    for name in {{ _shared_inputs }}; do
        node=$(jq -r ".nodes.root.inputs.\"$name\"" flake.lock)
        rev=$(jq -r ".nodes.\"$node\".locked.rev" flake.lock)
        owner=$(jq -r ".nodes.\"$node\".locked.owner" flake.lock)
        repo=$(jq -r ".nodes.\"$node\".locked.repo" flake.lock)
        echo "Syncing $name -> github:$owner/$repo/$rev"
        tmp=$(mktemp)
        sed -E "s|url: github:$owner/$repo(/[^[:space:]]*)?|url: github:$owner/$repo/$rev|" \
            devenv.yaml > "$tmp"
        mv "$tmp" devenv.yaml
    done
    devenv update

# Verify every shared input matches between flake.lock and devenv.lock
verify:
    #!/usr/bin/env bash
    set -euo pipefail
    fail=0
    printf '%-14s %-44s %-44s\n' input flake devenv
    for name in {{ _shared_inputs }}; do
        f_node=$(jq -r ".nodes.root.inputs.\"$name\"" flake.lock)
        f_rev=$(jq -r ".nodes.\"$f_node\".locked.rev" flake.lock)
        d_node=$(jq -r ".nodes.root.inputs.\"$name\"" devenv.lock)
        d_rev=$(jq -r ".nodes.\"$d_node\".locked.rev" devenv.lock)
        printf '%-14s %-44s %-44s\n' "$name" "$f_rev" "$d_rev"
        [ "$f_rev" = "$d_rev" ] || fail=1
    done
    if [ "$fail" -ne 0 ]; then
        echo "ERROR: shared inputs are out of sync between flake.lock and devenv.lock" >&2
        exit 1
    fi
    echo "All shared inputs in sync."

# Run devenv environment validation
test-env:
    devenv test

# Run Go tests
test:
    go test -race ./...

# Lint Go code
lint:
    golangci-lint run

# Generate Go coverage report
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out
