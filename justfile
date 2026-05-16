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

# Update all flake inputs, devenv pins, and Go dependencies
update:
    nix flake update
    just sync
    just verify
    just deps-update

# Rewrite devenv.yaml so every shared input pins the same commit as flake.lock
sync:
    #!/usr/bin/env bash
    set -euo pipefail
    safe='^[A-Za-z0-9._-]+$'
    for name in {{ _shared_inputs }}; do
        node=$(jq -r --arg name "$name" '.nodes.root.inputs[$name]' flake.lock)
        if ! [[ "$node" =~ $safe ]]; then
            echo "ERROR: unexpected node name for $name: $node" >&2
            exit 1
        fi
        rev=$(jq -r --arg node "$node" '.nodes[$node].locked.rev' flake.lock)
        if ! [[ "$rev" =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ ]]; then
            echo "ERROR: unexpected revision for $name: $rev" >&2
            exit 1
        fi
        owner=$(jq -r --arg node "$node" '.nodes[$node].locked.owner' flake.lock)
        repo=$(jq -r --arg node "$node" '.nodes[$node].locked.repo' flake.lock)
        if ! [[ "$owner" =~ $safe ]] || ! [[ "$repo" =~ $safe ]]; then
            echo "ERROR: unexpected owner/repo for $name: $owner/$repo" >&2
            exit 1
        fi
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
        f_node=$(jq -r --arg name "$name" '.nodes.root.inputs[$name]' flake.lock)
        f_rev=$(jq -r --arg node "$f_node" '.nodes[$node].locked.rev' flake.lock)
        d_node=$(jq -r --arg name "$name" '.nodes.root.inputs[$name]' devenv.lock)
        d_rev=$(jq -r --arg node "$d_node" '.nodes[$node].locked.rev' devenv.lock)
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

# Update Go dependencies
deps-update:
    go get -u ./...
    go mod tidy

# Update a specific Go dependency (usage: just deps-get github.com/foo/bar@latest)
deps-get pkg:
    go get {{quote(pkg)}}
    go mod tidy

# Generate Go coverage report
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out
