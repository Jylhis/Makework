---
name: makework
description: Manage git worktrees, project catalogs, and repository discovery with the mw CLI. Use when navigating projects, syncing repositories, checking worktree status, or managing the catalog.
---

# Makework — Git Worktree Manager

`mw` manages bare-clone repos and git worktrees with fuzzy project resolution, frecency-based ranking, and nix environment detection.

## Navigate to a project worktree

```bash
mw go <project> [ref]
```

- `project` can be a repo name, project name, subproject name, or fuzzy query
- `ref` is optional — defaults to the main branch
- Supports `repo@branch` explicit syntax to bypass fuzzy resolution
- Creates the worktree if it doesn't exist
- Returns the worktree path and optional nix activation command

## Discover and register repositories

```bash
mw sync [--depth N] [--exclude pattern]
```

Scans configured `scan_roots` (or `~/Developer` by default) for git repos and registers them in the catalog.

## Register a specific repository

```bash
mw catalog add <source>
```

`source` can be a local path or a git URL (https://, git@, ssh://).

## Fetch updates

```bash
mw fetch [project]
```

Fetches all refs for all repos, or a specific one.

## List catalog (replaces makework://catalog resource)

```bash
mw catalog list
```

Shows all registered repos with URL, branch, and worktree count.

## Check worktree status (replaces makework://status resource)

```bash
mw ls
```

Lists all active worktrees across repos with branch names. Use `--prune` to clean orphaned entries.

## Other useful commands

```bash
mw config show              # show effective config
mw config set <key> <val>   # set a config value
mw search <pattern>         # grep across all worktrees
mw query --since "7 days ago"  # recent git activity
mw new <project> <ref>      # create a worktree explicitly
mw rm <project>/<ref>       # remove a worktree
mw resolver explain <query> # debug resolver scoring
mw project show <name>      # show project metadata
```

## Configuration paths

- Config: `$XDG_CONFIG_HOME/makework/config.toml`
- Catalog: `$XDG_CONFIG_HOME/makework/catalog.toml`
- Per-project: `.makework.toml`
- State: `$XDG_STATE_HOME/makework/` (visits.json, cache)

## Initialization

First-time setup: `mw catalog init` creates config dirs, default config, and empty catalog.
