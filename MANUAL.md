# Makework User Manual

`mw` (makework) is a command-line **git worktree manager** for developers who
juggle many repositories and many branches at once. It keeps **one bare clone
per repository** and **one worktree per branch**, then layers fuzzy navigation,
frecency ranking, monorepo awareness, and automatic Nix environment activation
on top. The result is that switching projects or branches becomes a `cd`, not a
`git checkout`/stash/rebuild dance.

- **Version:** 0.1.0
- **Language / toolchain:** Go (module `github.com/jylhis/makework`, Go 1.26)
- **Binary:** `mw` (built from `cmd/mw`)
- **License:** MIT © 2026 Markus Jylhänkangas

> This manual is generated from a full read of the source tree. Where the
> bundled `docs/` site uses older command spellings (`mw catalog`, `mw new`,
> `mw sync`, bare `mw`), this manual documents the **actual compiled command
> tree** as wired in `internal/cli/root.go`, and notes the legacy names inline.

---

## Table of Contents

1. [Concepts & Mental Model](#1-concepts--mental-model)
2. [Installation](#2-installation)
3. [First-Run Setup](#3-first-run-setup)
4. [Shell Integration (required for `cd`)](#4-shell-integration-required-for-cd)
5. [Command Reference](#5-command-reference)
6. [The Resolver: Fuzzy + Frecency Navigation](#6-the-resolver-fuzzy--frecency-navigation)
7. [Worktrees & On-Disk Layout](#7-worktrees--on-disk-layout)
8. [The Catalog](#8-the-catalog)
9. [Nix Auto-Activation](#9-nix-auto-activation)
10. [Monorepos: Subprojects & Sparse Checkout](#10-monorepos-subprojects--sparse-checkout)
11. [Configuration Reference](#11-configuration-reference)
12. [Per-Project `.makework.toml`](#12-per-project-makeworktoml)
13. [Status, Integration State & Caching](#13-status-integration-state--caching)
14. [Hooks & Templates](#14-hooks--templates)
15. [Integrations (Emacs, AI, Nix modules)](#15-integrations)
16. [Security Model](#16-security-model)
17. [Architecture & Package Map](#17-architecture--package-map)
18. [Development & Contributing](#18-development--contributing)
19. [Troubleshooting](#19-troubleshooting)
20. [Quick Reference Card](#20-quick-reference-card)

---

## 1. Concepts & Mental Model

### The bare-clone + worktree split

Most git tooling assumes one clone with one working tree. Makework instead uses
git's [`worktree`](https://git-scm.com/docs/git-worktree) feature:

- Each repository is cloned **once** as a **bare repository** (no working files,
  just the object store and refs).
- Each branch you work on becomes its own **worktree** — a real directory of
  checked-out files that points back at the shared bare clone.

Why this matters:

- **Switching branches is `cd`, not `git checkout`.** No stashing, no rebuilds,
  no clobbered untracked files.
- **Each worktree is independent.** Its own untracked files, build cache, editor
  session, and dev environment.
- **Disk scales with branches, not repos.** Only the working files are
  duplicated; the git object store stays in the single bare clone.

### Key terms

| Term | Meaning |
|------|---------|
| **Catalog** | The registry of every repo Makework manages (`catalog.toml`). |
| **Bare clone** | The single `--bare` clone of a repo, under `bare_root`. |
| **Worktree** | A checked-out branch directory, under `worktree_root`. |
| **Project** | A registered repo, or a named subproject inside one. |
| **Subproject** | A logical project living in a subdirectory of a monorepo. |
| **Resolver** | The weighted scorer behind `mw go`'s fuzzy matching. |
| **Frecency** | Frequency × recency ranking of visited `repo:branch` pairs. |

### The shape of a session

```sh
mw repo add ~/Developer/myrepo   # register a repo (bare clone created)
mw go myrepo                     # cd into its main-branch worktree (created on demand)
mw go feature@new-ui             # fuzzy repo match + branch routing
mw ls                            # status across all worktrees
```

---

## 2. Installation

### Requirements

- **Go 1.26+** for source builds — `go.mod` declares `go 1.26`, so `go install`
  / `go build` needs a Go 1.26 toolchain (or it will auto-download one). Older
  toolchains (1.23–1.25) will fail unless toolchain auto-download is available.
- **git 2.5+** for worktrees; **git 2.25+** if you use sparse-checkout.

### Optional runtime dependencies

| Tool | Used by | Purpose |
|------|---------|---------|
| `git` | all commands | worktrees, fetch, log, sparse-checkout |
| `fzf` | interactive picker | fuzzy TUI for disambiguation / `mw go` with no arg |
| `ripgrep`/`grep` | `mw search` | cross-worktree text search (shells out to `grep -rn`) |
| `direnv` | `.envrc` detection | auto-loading per-project shells |
| `devenv` | `devenv.nix` detection | activating devenv environments |
| Nix | `flake.nix`/`shell.nix` detection | activating flake / classic shells |
| `gh` | `pr:N` shortcuts | resolve a GitHub PR number to its branch |
| `glab` | `mr:N` shortcuts | resolve a GitLab MR number to its branch |

### Install with Nix (recommended for Nix users)

Build straight from the flake:

```sh
nix build github:Jylhis/makework
nix profile install github:Jylhis/makework
```

Wire it into a NixOS / nix-darwin configuration:

```nix
inputs.makework = {
  url = "github:Jylhis/makework";
  inputs.nixpkgs.follows = "nixpkgs";
};

# in a module
environment.systemPackages = [ inputs.makework.packages.${pkgs.system}.default ];
```

Prebuilt binaries are served from the `jylhis` Cachix cache:

```sh
cachix use jylhis
```

### Install with Go

```sh
go install github.com/jylhis/makework/cmd/mw@latest
# or from a clone:
go build -o "$(go env GOPATH)/bin/mw" ./cmd/mw
mw --version
```

### Uninstall

```sh
rm "$(which mw)"
rm -rf ~/.config/makework ~/.local/share/makework ~/.local/state/makework
```

---

## 3. First-Run Setup

```sh
mw init            # create config, catalog, and state directories
mw config show     # verify the effective configuration
```

`mw init` (with no shell argument) creates the config, catalog, and state
directories and reports each path it created or found. In some bundled docs this
is spelled `mw catalog init`; the compiled command is `mw init`.

Register your repositories one of two ways:

```sh
# Explicitly, by path or URL
mw repo add ~/Developer/my-project
mw repo add https://github.com/owner/repo

# Or bulk-discover from scan roots
mw config set scan_roots ~/Developer
mw repo sync
```

---

## 4. Shell Integration (required for `cd`)

A child process cannot change its parent shell's working directory. `mw go`
therefore only **prints** the destination path (and an optional Nix activation
command); a small shell wrapper named `mw` reads that output and performs the
actual `cd` + activation.

### Install the wrapper + completions

```sh
# bash
eval "$(mw completions bash)"        # add to ~/.bashrc
# zsh
eval "$(mw completions zsh)"         # add to ~/.zshrc
# fish
mw completions fish | source         # add to ~/.config/fish/config.fish
```

`mw completions <shell>` emits **both** native tab-completions and the `mw()`
wrapper function. The wrapper only intercepts `mw go`; every other subcommand is
passed through verbatim.

### Install the visit-tracking hook (frecency)

```sh
# bash
mw init bash >> ~/.bashrc
# zsh
mw init zsh  >> ~/.zshrc
```

`mw init <shell>` outputs completions, the navigation wrapper, and (for bash/zsh
only) a directory-change hook. The hook calls `mw visit "$PWD"` on every `cd`:

- **zsh** registers a `chpwd` hook (`add-zsh-hook chpwd ...`).
- **bash** appends to `PROMPT_COMMAND` (guarded by a sentinel to avoid double
  installation).
- **fish / powershell** get no visit hook — frecency only updates through
  successful `mw go` navigations on those shells.

`mw visit` is a fast path: it consults the cached list of bare-clone roots and
returns immediately if you are not inside a known repo, so it adds only
milliseconds to each prompt. This lets the resolver learn your habits even when
you `cd` around manually rather than through `mw go`.

### How the `mw go` wrapper parses output

`mw go` prints up to three lines:

```
/path/to/worktree/services/api   # line 1: destination cd target (subproject path, or worktree root)
nix develop                      # line 2: activation command (optional)
/path/to/worktree                # line 3: worktree root — only emitted for subprojects
```

The wrapper `cd`s into line 1. If line 2 is present, it then `cd`s into line 3
when given (the worktree root) and `eval`s the activation command there. So for a
subproject you land in the subproject directory, but the activation itself runs
at the worktree root — see [§9](#9-nix-auto-activation) for the implications.

### Bypassing the wrapper

`command mw ...` skips the shell function and prints raw output — useful for
scripting or to get a path without the activation side effect:

```sh
worktree="$(command mw go myrepo | head -1)"
```

---

## 5. Command Reference

Top-level commands, exactly as registered in `internal/cli/root.go`:

```
go  switch  rm  merge  ls  prune  fetch  repo  project  maintenance  config
search  query  resolver  ai  init  completions  wt  visit  mcp  man  generate-texi
```

Global behavior: errors print as `Error: <message>` to stderr and exit non-zero;
usage is silenced on error. `mw --version` prints the build version; `mw --help`
or `mw <cmd> --help` prints usage.

> Bare `mw` with no subcommand prints help. For a status overview, use `mw ls`
> (or `mw wt list` inside a repo). Some docs refer to bare `mw` as the status
> command; the implemented status surface is `mw ls`.

### 5.1 `mw go [project] [ref]` — navigate to a worktree

The primary command. Resolves a project, ensures a worktree exists for the ref,
and prints the path (plus Nix activation) for the shell wrapper to consume.

- **`project`** — a catalog name, a fuzzy query, or explicit `repo@branch`.
- **`ref`** — branch/tag/commit; defaults to the repo's main branch. Supports
  `pr:N` and `mr:N` shortcuts (see [§6](#6-the-resolver-fuzzy--frecency-navigation)).

| Flag | Default | Description |
|------|---------|-------------|
| `--list` | false | Print the top 10 scored matches instead of navigating. |
| `--allow-hooks` | false | Run post-create hooks from `.makework.toml` (off by default). |

**Resolution order:**

1. **No argument:** with a TTY, opens an interactive picker over all project
   names; without a TTY, lists projects on stderr and exits non-zero.
2. **Explicit `repo@branch`:** routes directly, no fuzzy matching.
3. **Exact catalog name:** routes directly (fast path).
4. **Fuzzy query:** runs the resolver. If the top two results are within 10% and
   a TTY is present, opens a disambiguation picker. Otherwise, **`mw go` refuses
   to auto-navigate on a fuzzy-only match** and prints the suggested
   `mw go name@branch` to confirm. This is a deliberate safety guard against
   landing in the wrong repo.

A worktree is created on first navigation to a `(repo, ref)` pair; subsequent
visits reuse it. On creation Makework also applies sparse-checkout, copies the
template directory, runs post-create hooks (if allowed), and records a visit.

> **`mw go` does not create new branches.** It runs `git worktree add <path> <ref>`,
> so `<ref>` must already resolve — an existing local branch, or a name that
> matches exactly one remote-tracking branch (git's DWIM creates a local tracking
> branch in that case). A brand-new branch name that exists nowhere will fail.
> To start a *new* branch, use `mw switch <project> <branch> -c`.

```sh
mw go myapp                  # main branch
mw go myapp feature/auth     # check out an EXISTING ref (worktree created if missing)
mw go myapp@develop          # explicit repo@branch
mw go api                    # fuzzy match
mw go --list api             # show scored candidates, don't move
mw go myapp pr:42            # check out the branch behind GitHub PR #42
```

### 5.2 `mw switch <project> <ref>` — create a worktree (no implicit navigation)

Creates a worktree and prints its path. Scriptable analog to `mw go` without
fuzzy matching. In older docs this is `mw new`.

Without `-c`, `switch` runs `git worktree add <path> <ref>`, so `<ref>` must
already exist (or match a single remote branch); it does **not** create an
arbitrary new branch, and it will error if the worktree path already exists. Use
`-c` to create a new branch (off `--base`, defaulting to the main branch).

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--create` | `-c` | false | Create a new branch for the worktree. |
| `--base` | `-b` | "" | Base branch for `-c` (defaults to the repo's main branch). |

```sh
mw switch myapp feature/auth          # worktree for an EXISTING ref
mw switch myapp fix/login -c          # new branch off main
mw switch myapp hotfix -c -b release/v2  # new branch off release/v2
```

### 5.3 `mw rm <target>` — remove a worktree (and its branch)

`target` must be `<project>/<ref>` or `<project>@<ref>`. Removes the worktree
first, then — unless told otherwise — deletes the branch **if it is integrated**
(merged) into the default branch.

> Note: `rm` does **not** refuse to remove the default branch's *worktree*. It
> removes the worktree regardless; if `<ref>` is the default branch it then keeps
> the *branch* (printing `Branch <ref> kept (default branch)`). Diverged branches
> are kept unless you pass `-D`.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--keep-branch` | | false | Remove the worktree but keep the branch. |
| `--force-delete` | `-D` | false | Force-delete the branch even if it has diverged. |

```sh
mw rm myapp/feature/auth
mw rm myapp@feature/auth --keep-branch
```

### 5.4 `mw merge` — rebase + fast-forward + clean up

Run from **inside** a worktree. Rebases the current branch onto the target,
fast-forwards the target to the rebased HEAD, then removes the worktree and
deletes the branch.

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | repo default branch | Branch to merge into. |
| `--keep-worktree` | false | Don't remove the worktree afterward. |
| `--keep-branch` | false | Don't delete the branch afterward. |
| `--force` | false | Proceed even with uncommitted changes. |

Aborts the rebase automatically on conflict; fails fast if the worktree is dirty
and `--force` is not set.

### 5.5 `mw ls` — list active worktrees

Lists every active worktree across all repos.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `table` | `table` or `json`. |

Table columns: `REPO`, `BRANCH` (or `(detached)`), `REMOTE↕` (ahead/behind vs
upstream), `PATH` (with ` (orphaned)` when the directory is gone). JSON emits a
list of `{repo, status{path,branch,ahead,behind,is_orphaned}}`.

### 5.6 `mw prune` — remove orphaned worktree entries

Scans every repo and prunes git worktree entries whose directories no longer
exist on disk. Reports the count pruned per repo.

### 5.7 `mw fetch [project]` — fetch one or all repos

Runs `git fetch --all --prune --tags` on a single repo (if named) or every
registered repo. Warns when a repo's upstream default branch has changed.

```sh
mw fetch          # all repos
mw fetch myapp    # one repo
```

### 5.8 `mw repo` — manage the catalog

The repository registry commands. Older docs spell this group `mw catalog`.

| Subcommand | Description |
|------------|-------------|
| `mw repo add <source>` | Register a repo from a local path or URL (HTTPS/SSH/`git@`). Creates the bare clone. |
| `mw repo list` | Table of `NAME`, `URL` (or `(local)`), `BRANCH`, `WORKTREES`. |
| `mw repo rm <project>` | Unregister; **keeps** files on disk. Warns if worktrees exist. |
| `mw repo purge <project>` | Delete worktrees + bare clone (only within configured roots) and unregister. |
| `mw repo edit` | Open `catalog.toml` in `$EDITOR` (falls back to `vi`). |
| `mw repo sync` | Discover and register repos under `scan_roots`. |

`mw repo sync` flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--depth` | from config (`sync_max_depth`, default 1) | Max directory depth to scan. |
| `--exclude` (repeatable) | merged with `sync_exclude` | Directory names to skip. |

Sync walks each scan root, registers working repos it finds (`.git` directory),
and skips bare repos and submodules.

```sh
mw repo add https://github.com/jylhis/makework.git
mw repo add ~/Developer/side-project
mw repo add .
mw repo sync --depth 3 --exclude node_modules --exclude target
mw repo purge old-project      # destructive: deletes clone + worktrees
```

### 5.9 `mw project` — per-project configuration

| Subcommand | Description |
|------------|-------------|
| `mw project init` | Write a commented `.makework.toml` template into the current directory. |
| `mw project show [project]` | Print resolved metadata (path, URL, main branch, remotes, resolved Nix config). |

### 5.10 `mw config` — global settings

| Subcommand | Description |
|------------|-------------|
| `mw config show` | Table of `SETTING`, `VALUE`, `SOURCE` (file path or builtin). |
| `mw config set <key> <value>` | Set one key (comma-separated for list values). |
| `mw config edit` | Open `config.toml` in `$EDITOR`. |

See [§11](#11-configuration-reference) for all keys.

### 5.11 `mw search <pattern>` (alias `mw grep`) — search across worktrees

Greps the **main-branch worktree** of each registered repo (shells out to
`grep -rn`) and groups results by repo.

> Scope caveat: `mw search` computes one worktree path per repo using the repo's
> main branch and searches only that directory — it does **not** iterate every
> active worktree. Matches that live only in a feature-branch worktree are not
> found. Repos whose main-branch worktree doesn't exist on disk are skipped.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--glob` | | "" | File filter, e.g. `*.go`. |
| `--ignore-case` | `-i` | false | Case-insensitive search. |
| `--max` | | 0 | Cap results per repo (0 = unlimited). |

```sh
mw search "TODO"
mw search --glob "*.go" -i "fixme"
mw grep --max 5 "error"
```

### 5.12 `mw query` — recent commit activity across repos

Aggregates `git log` across every active worktree, de-duplicates commits (by
repo + hash), and summarizes per repo.

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `7 days ago` | Anything `git log --since` accepts. |
| `--until` | "" | Upper bound on commit date. |
| `--author` | "" | Filter by author name. |
| `--format` | `short` | `short` (abbrev hash) or `full` (full hash). |

```sh
mw query
mw query --since yesterday --format full
mw query --since 2026-04-01 --until 2026-04-15 --author Alice
```

Author names and commit messages are sanitized (control/format characters
stripped) before printing to defend against terminal-injection.

### 5.13 `mw resolver explain <query>` — debug fuzzy scoring

Prints the active weights and a per-signal breakdown for the top candidates:

```
Query: 'auth'
Weights: fuzzy=0.35 frecency=0.35 activity=0.15 context=0.15

REPO            BRANCH        FUZZY  FREC  ACT   CTX   TOTAL
auth-service    main          0.950 0.812 0.000 0.500 0.692
backend         auth-rewrite  0.620 0.450 0.000 0.500 0.449
```

Notes when disambiguation would trigger (top two within 10%).

### 5.14 `mw wt` — worktrunk-compatible commands (current repo)

A subset of the `worktrunk` CLI surface scoped to the repo
that owns `$PWD` (auto-detected, so no project argument). Useful when you already
`cd`'d into a repo.

- **`mw wt switch <branch>`** — switch to / create a worktree for `<branch>`.
  Flags: `-c/--create`, `-b/--base`.
- **`mw wt list`** — list this repo's worktrees. Flags: `--branches` (include
  local branches without worktrees), `--remotes` (include remote-tracking refs),
  `--format table|json`. `--full`, `--progressive`, `--no-progressive` are
  accepted for compatibility but currently render the same table (CI/LLM
  summaries and streaming are not yet implemented).
- **`mw wt remove [<branch>...]`** — remove worktrees (current branch if none
  given). Flags: `-f/--force` (allow untracked files), `--no-delete-branch`,
  `-D/--force-delete`.

The `wt list` JSON/table carries richer status than `mw ls`: `STATUS` symbols,
`MAIN↕` (vs default branch), `REMOTE↕`, integration state, and dirty counts.

### 5.15 `mw maintenance` — git maintenance registration

| Subcommand | Description |
|------------|-------------|
| `mw maintenance start` | Register the repo of the **current directory** with `git maintenance`. |
| `mw maintenance stop` | Unregister the repo of the current directory. |
| `mw maintenance status` | Report registered / not registered for the current directory's repo. |

> Scope caveat: despite the command help text saying "all bare repos", these
> subcommands operate only on the git repository containing your current working
> directory (`os.Getwd()`); they do not load the catalog or iterate every repo.
> Run them from inside each worktree/clone you want registered.

### 5.16 `mw ai init` — output the Claude Code skill

Prints the bundled `SKILL.md` to stdout. Install it with:

```sh
mkdir -p .claude/skills && mw ai init > .claude/skills/makework.md
```

### 5.17 `mw init [shell]` — initialize or emit shell integration

- **No argument:** create config, catalog, and state directories.
- **With a shell argument:** emit completions plus, depending on the shell:
  - **`bash` / `zsh`** — native completions, the `mw go` wrapper, **and** the
    visit-tracking hook (full integration).
  - **`fish`** — native completions and the `mw go` wrapper, but **no** visit
    hook (frecency-on-`cd` is not installed for fish yet).
  - **`powershell`** — native completions **only**: no `mw go` wrapper and no
    visit hook.

### 5.18 `mw completions <shell>` — completions + wrapper

Prints native completions and the `mw()` navigation wrapper for
`bash`/`zsh`/`fish`/`powershell`.

### 5.19 Hidden / utility commands

| Command | Purpose |
|---------|---------|
| `mw visit <path>` | Record a frecency visit. Called by the shell hook; not for manual use. |
| `mw mcp` | Deprecated MCP server stub; prints a message directing you to Claude Code skills. |
| `mw man [--output-dir DIR]` | Generate man pages for all commands. |
| `mw generate-texi [--output FILE]` | Generate Texinfo source for info pages. |

---

## 6. The Resolver: Fuzzy + Frecency Navigation

`mw go <query>` does not require the exact catalog name. A multi-signal resolver
scores every project/branch candidate and picks the best — like `zoxide` for
repos.

### Signals and default weights

| Signal | Default weight | Measures |
|--------|----------------|----------|
| **fuzzy** | 0.35 | Normalized Levenshtein with prefix & substring bonuses |
| **frecency** | 0.35 | Visit frequency × time decay (with sibling-branch boost) |
| **activity** | 0.15 | `log1p(commits in last 30 days)` on the default branch |
| **context** | 0.15 | Whether the candidate path overlaps your current directory |

Final score = `fuzzy·0.35 + frecency·0.35 + activity·0.15 + context·0.15`.
Weights are configurable (see [§11](#11-configuration-reference)); they need not
sum to 1, but at least one must be positive.

### Fuzzy matching details

For each candidate, the best score across the repo name, project name, branch
name, and path basename is used. The raw fuzzy score is:

```
similarity = 1 - levenshtein(query, name) / max(len(query), len(name))   # rune-based
raw        = (similarity + substringBonus) * prefixMultiplier
```

- **Substring bonus:** `+0.2` if the query is a (case-insensitive) substring.
- **Prefix multiplier:** `×1.3` if the query is a case-insensitive prefix.
- **Cap:** non-exact matches are capped at `0.95`, so an exact name always wins
  ties.

### Frecency details

Every successful navigation records a visit keyed `repo:branch` in
`visits.json`. Score for a candidate is:

```
visitScore × timeWeight,  where  timeWeight = 1 / (1 + elapsedSeconds / 3600)
```

so weight halves after ~1 hour of inactivity. Other branches of a heavily
visited repo (sharing the `repo:` prefix) receive a partial **sibling boost**
(0.5×). The database self-compacts: when the summed score exceeds `MaxAge`
(10000), all scores are rescaled and entries below 1.0 are dropped (a z-frecency
aging trick).

### Disambiguation & the no-auto-navigate guard

- If the **top two** candidates are within **10%** and a TTY is present, `mw go`
  opens an interactive picker.
- Otherwise, on a fuzzy-only match `mw go` **refuses to move** and prints the
  exact `mw go name@branch` to confirm. Only exact catalog matches, explicit
  `repo@branch`, or a chosen picker entry navigate automatically.

### Branch shortcuts

The `ref` argument accepts:

- `pr:N` → resolves the GitHub PR's head branch via `gh pr view N --json headRefName`
  (requires `gh`).
- `mr:N` → resolves the GitLab MR's source branch via `glab mr show N --output json`
  (requires `glab`).

The repo slug (`host/owner/name`) is derived from the catalog URL and passed via
`--repo`, so resolution works without a local checkout.

---

## 7. Worktrees & On-Disk Layout

### Where things live (XDG)

| Path | Contents |
|------|----------|
| `$XDG_CONFIG_HOME/makework/config.toml` | Global config |
| `$XDG_CONFIG_HOME/makework/catalog.toml` | Repository registry |
| `$XDG_DATA_HOME/makework/repos/...` | Bare clones |
| `$XDG_DATA_HOME/makework/worktrees/...` | Worktrees |
| `$XDG_STATE_HOME/makework/visits.json` | Frecency database |
| `$XDG_STATE_HOME/makework/catalog.toml.lock` | Save-time advisory lock |
| `$XDG_STATE_HOME/makework/` (cache, repo-roots) | Status cache, cached bare-clone roots |

Defaults (Linux): config `~/.config`, data `~/.local/share`, state
`~/.local/state`. On macOS, config and data resolve to
`~/Library/Application Support/makework`; state stays at `~/.local/state`.

### Path scheme

Remote-sourced repos are partitioned by URL host and path segments; local repos
live under `local/`:

```
repos/
├── github.com/Jylhis/makework.git       # bare clone (remote)
└── local/side-project.git               # bare clone (local path)

worktrees/
├── github.com/Jylhis/makework/main/      # remote repo, branch "main"
├── github.com/Jylhis/makework/feature-x/ # branch "feature/x" sanitized
└── local/side-project/main/              # local repo
```

Branch names are **sanitized** for the filesystem: `refs/tags/` prefixes are
stripped, unsafe characters (`: \ ? * < > | "`) become `_`, and `.`/`..`/empty
path segments are dropped to prevent traversal (e.g. `feature/../../etc` →
`feature_etc`).

### Lifecycle on creation

When `mw go`/`mw switch` first materializes a `(repo, ref)`:

1. `git worktree add` creates the directory.
2. If the subproject defines `sparse_paths`, `git sparse-checkout init --cone`
   then `set` is applied.
3. If `template_dir` is set, its files are copied in (never overwriting).
4. Post-create hooks run (only with `--allow-hooks`/`allow_hooks`).
5. Nix is detected and the activation command is emitted.
6. A visit is recorded.

---

## 8. The Catalog

`catalog.toml` is the registry of every managed repo. Each entry records the
bare-clone path, optional remote URL, default branch, named remotes, and any
nested projects/subprojects.

```toml
[repos.makework]
path = "/home/me/.local/share/makework/repos/github.com/Jylhis/makework.git"
url = "https://github.com/Jylhis/makework"
main_branch = "main"

[repos.makework.remotes.origin]
url = "https://github.com/Jylhis/makework"
```

### Name resolution order

When a name is given to `mw go`, `mw project show`, etc., the catalog resolves it
by: (1) exact **repo** name, (2) **project** name, (3) **subproject** name.
Ambiguous names (e.g. the same subproject in two repos) raise an error unless an
explicit `repo@branch` form is used.

### Accepted URL forms

`mw repo add` parses:

- `https://host[:port]/owner/repo.git`
- `http://host[:port]/owner/repo.git`
- `ssh://[user@]host[:port]/owner/repo.git`
- `git@host:owner/repo.git` (scp-style)

The `.git` suffix is stripped, hosts and segments are validated against control
characters and path separators, and the host/segments become the on-disk layout.

### Concurrency & atomicity

Catalog writes take an exclusive POSIX flock on `catalog.toml.lock`, write to a
temp file, then atomically rename — safe under concurrent invocations.

---

## 9. Nix Auto-Activation

After navigating, Makework prints an activation command that the shell wrapper
`eval`s, dropping you straight into the project's dev environment.

### Detection order (first match wins)

1. **Explicit `[nix]` table** in `.makework.toml` (project or subproject).
2. **`flake.nix`** → `nix develop` (or `nix develop .#<devshell>` if a devshell
   is named).
3. **`shell.nix`** → `nix-shell`.
4. **`devenv.nix`** → `devenv shell`.
5. **`.envrc`** containing `use flake` / `use nix` / `use devenv` → the matching
   command.

If nothing matches, no activation is printed and you keep your parent shell
environment.

### Where detection and activation happen

Detection runs at the **worktree root** (`nix.Detect(<worktreeRoot>, …)`), and
the shell wrapper `eval`s the activation command at the worktree root too. For a
subproject, `mw go` emits three lines — the subproject path, the activation
command, and the worktree root — and the wrapper `cd`s into the subproject, then
`cd`s back to the worktree root before running the activation.

> Practical consequence: a Nix file that exists **only** under a subdirectory
> (e.g. `services/api/devenv.nix`) is **not** auto-detected — detection looks at
> the worktree root, and `nix develop` / `devenv shell` then run there. To drive
> a subproject environment you must set an explicit `[nix]` table (with a `type`)
> on the subproject in the catalog so the activation command is chosen, and the
> Nix tool itself must be able to resolve the environment from the worktree root.
> Repo-root Nix files work as described in the detection order above.

Inspect what will run before navigating:

```sh
mw project show api    # prints the resolved Nix configuration
```

---

## 10. Monorepos: Subprojects & Sparse Checkout

A **subproject** is a logical project inside a larger repository, defined in the
repo-root `.makework.toml`.

> **Important — subprojects are not auto-imported (as of 0.1.0).** `mw repo add`
> and `mw repo sync` register the repo with an **empty** project map and never
> parse `.makework.toml`. The only thing `mw` reads `.makework.toml` for is
> **post-create hooks** at worktree creation. The resolver builds its targets
> from `catalog.toml` only, so subproject-by-name navigation (`mw go api`),
> per-subproject sparse-checkout, and per-subproject Nix take effect **only for
> subprojects that exist in `catalog.toml`** — which today means adding them by
> hand via `mw repo edit`. The schema below is what a catalog subproject entry
> looks like; committing it as `.makework.toml` documents intent but does not, on
> its own, make `mw go api` resolve. (The bundled `docs/` site currently
> overstates this; it is tracked as a gap.)

```toml
main_branch = "main"
tags = ["work"]

[subprojects.api]
subproject_path = "services/api"
sparse_paths = ["services/api", "shared/proto"]

[subprojects.api.nix]
type = "devenv"
path = "services/api"

[subprojects.web]
subproject_path = "apps/web"

[subprojects.web.nix]
type = "flake"
path = "apps/web"
devshell = "node"
```

Once a subproject is present in the catalog:

- **Navigate by name:** `mw go api` routes into `services/api` of the parent
  monorepo; `mw go web` into `apps/web`.
- **Sparse checkout:** when `sparse_paths` is set, only those directories are
  populated (`git sparse-checkout init --cone` + `set`). Requires git 2.25+.
  Different subprojects in the same repo get separate worktrees with their own
  sparse configuration.
- **Per-subproject Nix:** a subproject can declare its own `[nix]` table to
  select the activation command. As noted in [§9](#9-nix-auto-activation),
  detection and activation still run at the worktree root, not the subproject
  directory — so the `[nix]` `type` chooses the command, but the Nix files/flake
  must resolve from the worktree root.

Inspect a resolved subproject:

```sh
mw project show api    # path, sparse paths, resolved Nix, inherited tags
```

---

## 11. Configuration Reference

### Global config (`config.toml`, `[config]` table)

| Key | Type | Default | Purpose |
|-----|------|---------|---------|
| `worktree_root` | path | `$XDG_DATA_HOME/makework/worktrees` | Where worktrees are created. Required, absolute. |
| `bare_root` | path | `$XDG_DATA_HOME/makework/repos` | Where bare clones live. Required, absolute, **must differ** from `worktree_root`. |
| `scan_roots` | path list | — | Directories `mw repo sync` walks. |
| `sync_max_depth` | uint | 1 | Max recursion depth for `mw repo sync`. |
| `sync_exclude` | string list | — | Directory names skipped during scan. |
| `template_dir` | path | — | Files copied into every new worktree. |
| `allow_hooks` | bool | false | Run `.makework.toml` post-create hooks by default. |
| `resolver.weight_fuzzy` | float | 0.35 | Fuzzy-match weight. |
| `resolver.weight_frecency` | float | 0.35 | Frecency weight. |
| `resolver.weight_activity` | float | 0.15 | Activity weight. |
| `resolver.weight_context` | float | 0.15 | Context weight. |

`~` is expanded in path values. Validation requires absolute, distinct
`worktree_root`/`bare_root` and a positive resolver-weight sum.

### Setting values

```sh
mw config set worktree_root ~/work/wts
mw config set scan_roots ~/Developer,~/work       # comma-separated list
mw config set sync_exclude node_modules,target,.cache
mw config set sync_max_depth 3
mw config set allow_hooks true
mw config edit                                     # open in $EDITOR
mw config show                                     # effective config + source
```

Keys accepted by `mw config set`: `worktree_root`, `bare_root`, `scan_roots`,
`sync_max_depth`, `sync_exclude`, `template_dir`, `allow_hooks`.

### Example `config.toml`

```toml
[config]
worktree_root = "/home/me/code/worktrees"
bare_root     = "/home/me/code/repos"
scan_roots    = ["/home/me/Developer"]
sync_exclude  = ["node_modules", "target"]
allow_hooks   = false

[config.resolver]
weight_fuzzy    = 0.40
weight_frecency = 0.40
weight_activity = 0.05
weight_context  = 0.15
```

### Environment variables

| Variable | Used for |
|----------|----------|
| `XDG_CONFIG_HOME` | config + catalog location |
| `XDG_DATA_HOME` | bare clones + worktrees |
| `XDG_STATE_HOME` | visits, status cache, lock |
| `HOME` | XDG fallbacks and `~` expansion |
| `EDITOR` | `mw config edit`, `mw repo edit` (falls back to `vi`) |

---

## 12. Per-Project `.makework.toml`

Placed at a repository root (committed to the repo). Schema:

| Key | Type | Purpose |
|-----|------|---------|
| `main_branch` | string | Override the repo's default branch. |
| `tags` | string list | Free-form grouping/filtering tags. |
| `[nix]` | table | Repo-wide Nix override (see below). |
| `[hooks]` `post-create` | string list | Commands run after a worktree is created. |
| `[subprojects.<id>]` | table | One per subproject. |
| `subprojects.<id>.subproject_path` | path | Path within the repo (relative). |
| `subprojects.<id>.sparse_paths` | string list | Directories to populate via sparse-checkout. |
| `subprojects.<id>.docs` | string | Optional docs pointer. |
| `[subprojects.<id>.nix]` | table | Per-subproject Nix override. |

`[nix]` tables (project or subproject):

| Key | Type | Purpose |
|-----|------|---------|
| `type` | string | `flake`, `classic`/`shell`, `devenv`, or `custom`. |
| `devshell` | string | Named devshell → `nix develop .#<name>`. |
| `path` | path | Directory to run the activation command in. |

Generate a starter file with `mw project init`:

```toml
# main_branch = "main"
# tags = ["work"]

# [subprojects.example]
# subproject_path = "services/example"

# [subprojects.example.nix]
# type = "devenv"
# path = "services/example"
```

---

## 13. Status, Integration State & Caching

`mw ls` and `mw wt list` compute worktree status. The full status record
includes:

- `branch`, `path`
- `dirty_count`, `modified`, `untracked`, `conflicts`
- `ahead`/`behind` vs upstream tracking branch
- `main_ahead`/`main_behind` vs the default branch
- `integration` state, `last_commit_ts`, `is_orphaned`

`mw wt list` renders single-character **STATUS symbols**:

| Symbol | Meaning |
|--------|---------|
| `!` | modified files |
| `?` | untracked files |
| `✘` | conflicts |
| `⊂` | integrated (merged) |
| `_` | clean, same commit |
| `-` | no status |

### Integration classification

When deciding whether a branch is safe to delete, Makework classifies it against
the default branch with a 5-step ladder (first match wins):

1. **same_commit** — branch HEAD equals default HEAD.
2. **ancestor** — branch is in the default's history.
3. **no_changes** — the three-dot diff (`default...branch`) is empty.
4. **patch_id** — every unique branch commit has an exact patch-text match on
   the default (squash/cherry-pick detection, via SHA-256 of patch text).
5. **diverged** — none of the above; the branch carries unmerged work.

`mw rm`/`mw merge` delete the branch automatically only when it is integrated;
diverged branches are kept unless you pass `-D`.

### Caching

Status snapshots are cached under the state directory so the overview stays cheap
across many repos. The cache and `visits.json` are safe to delete; Makework
regenerates them on demand.

---

## 14. Hooks & Templates

### Post-create hooks

`.makework.toml` may declare commands to run after a worktree is created:

```toml
[hooks]
post-create = [
  "direnv allow",
  "cp ../shared/.env .env",
]
```

- Commands run sequentially via `sh -c` in the new worktree root; execution
  stops at the first non-zero exit.
- Each command is echoed (`$ <cmd>`) before running.
- The environment is the inherited process env plus:
  `MW_WORKTREE_PATH`, `MW_BRANCH`, `MW_REPO`.
- **Disabled by default.** Because hooks come from repo contents, they are
  untrusted: unless you pass `--allow-hooks` or set `allow_hooks = true`, the
  hooks are skipped with a one-line notice explaining how to opt in.

### Templates

Set `template_dir` and Makework copies its tree into every new worktree after
creation:

```sh
mw config set template_dir ~/dotfiles/worktree-template
```

Copying never overwrites existing files, validates that destinations stay inside
the worktree (no traversal), and refuses symlinked destinations. Handy for shared
`.envrc`, editor configs, or local-only scripts.

---

## 15. Integrations

### Emacs (`editors/emacs/makework.el`)

A single-file package wrapping the `mw` binary.

```elisp
(add-to-list 'load-path "~/path/to/makework/editors/emacs")
(require 'makework)

(setq makework-binary "mw"     ; or an absolute path
      makework-use-nix t)      ; run Nix activation after navigating
```

Interactive commands:

| Command | Effect |
|---------|--------|
| `M-x makework-go` | `completing-read` over projects, set `default-directory`, optionally activate Nix via `compile`. |
| `M-x makework-status` | Run `mw` into a read-only `*makework-status*` buffer. |
| `M-x makework-sync` | Run `mw repo sync`, report newly registered repos. |
| `M-x makework-fetch` | Run `mw fetch` in a compilation buffer. |

> Caveat (0.1.0): `makework.el` is partly out of sync with the current CLI. Its
> project picker shells out to `mw catalog list`, but the command is now
> `mw repo list` (`catalog` was renamed to `repo`), so `makework-go`'s completion
> list comes back empty. `makework-status` runs bare `mw`, which prints help
> rather than a status overview (use `mw ls` for status). `makework-sync` and
> `makework-fetch` work as written. Treat the package as a starting point until
> it is updated.

Run the ERT tests with:

```sh
emacs -batch -L editors/emacs -l makework-test.el -f ert-run-tests-batch-and-exit
```

### AI / Claude Code skill

`mw ai init` prints a Claude Code skill describing how to navigate repos, manage
the catalog, and run cross-project ops with `mw`:

```sh
mkdir -p .claude/skills && mw ai init > .claude/skills/makework.md
```

Place it in a repo's `.claude/skills/` (project-scoped) or your home directory
(global). Regenerate after upgrading `mw`. The legacy `mw mcp` server is
deprecated in favor of this skill.

### Nix modules (`programs.makework.*`)

Modules ship for **home-manager**, **NixOS**, and **nix-darwin**.

Shared options:

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enable` | bool | false | Install Makework. |
| `package` | package | flake default | Package to install. |
| `enableBashIntegration` | bool | true | Source the bash visit hook. |
| `enableZshIntegration` | bool | true | Source the zsh visit hook. |
| `settings` | attrset | {} | Written to `config.toml` (**home-manager only**). |

`settings` maps directly to the `[config]` table (`worktree_root`, `bare_root`,
`scan_roots`, `sync_max_depth`, `sync_exclude`, `template_dir`, and a `resolver`
sub-attrset with the four weights). The NixOS/darwin modules install the binary
and shell hooks but reject `settings` (config is per-user by design).

Example (home-manager):

```nix
programs.makework = {
  enable = true;
  settings = {
    worktree_root = "/home/alice/code/worktrees";
    bare_root     = "/home/alice/code/repos";
    scan_roots    = [ "/home/alice/code" ];
    resolver = {
      weight_fuzzy = 0.4; weight_frecency = 0.4;
      weight_activity = 0.05; weight_context = 0.15;
    };
  };
};
```

---

## 16. Security Model

Makework handles untrusted inputs (repo contents, commit metadata, remote URLs)
defensively:

- **Hooks are opt-in.** Post-create hooks from `.makework.toml` never run unless
  you pass `--allow-hooks` or set `allow_hooks = true`.
- **Terminal-injection defense.** Author names, commit messages, and other
  attacker-influenceable text are sanitized (control/format characters replaced)
  before display.
- **Path-traversal defense.** Branch names and URL segments are sanitized; `..`,
  empty, and `.` path segments are dropped. Template application validates that
  destinations stay inside the worktree and rejects symlinked destinations.
- **Shell-field validation.** Paths and activation commands emitted for the shell
  wrapper are rejected if they contain control characters.
- **Branch-name validation.** Branch names are checked with
  `git check-ref-format` and rejected if they start with `-`.
- **Shell quoting.** Suggested commands are single-quote escaped with the POSIX
  `'\''` trick.
- **Bounded blast radius for destructive ops.** `mw repo purge` only deletes
  paths under the configured `bare_root`/`worktree_root`.

---

## 17. Architecture & Package Map

Single Go module (`github.com/jylhis/makework`):

```
cmd/mw/                  # binary entry point
internal/
├── cli/                 # Cobra command tree & dispatch
├── config/              # config.toml load/validate/save
├── catalog/             # catalog.toml registry, add/sync/resolve
├── resolver/            # weighted fuzzy resolution + visits/frecency + filelock
├── worktree/            # worktree path computation, create/list/sparse
├── repo/                # git shell-out wrappers, URL parsing
├── project/             # .makework.toml schema
├── nix/                 # Nix environment detection
├── status/              # worktree status + caching
├── integration/         # branch integration-state classification
├── search/              # grep across worktrees
├── query/               # git-log activity queries
├── template/            # template file application
├── hook/                # post-create hook runner
├── maintenance/         # git maintenance registration
├── refshortcut/         # pr:N / mr:N resolution
├── picker/              # fzf / stdin interactive picker
├── terminal/            # output sanitization
├── xdgpath/             # XDG directory resolution
├── fsx/                 # filesystem helpers
└── buildinfo/           # version via ldflags
editors/emacs/           # makework.el integration
nix/                     # flake-parts module, packaging, home-manager/NixOS/darwin modules
docs/                    # Mintlify documentation site
```

Key dependencies: `spf13/cobra` (CLI), `pelletier/go-toml/v2` (config),
`agnivade/levenshtein` (fuzzy), `cobra/doc` (man/texi generation).

Command → package mapping (selected):

| Command | Primary package |
|---------|-----------------|
| `go`, `switch`, `rm`, `ls`, `prune`, `wt` | `internal/worktree`, `internal/status` |
| `repo *` | `internal/catalog` |
| `config *` | `internal/config` |
| `fetch`, `merge` | `internal/repo` |
| `search` | `internal/search` |
| `query` | `internal/query` |
| `go` (fuzzy) / `resolver explain` | `internal/resolver` |
| `project *` | `internal/project` |
| `maintenance *` | `internal/maintenance` |

---

## 18. Development & Contributing

The dev environment is defined with [devenv](https://devenv.sh) and provides Go,
golangci-lint, gopls, `just`, and treefmt.

```sh
just dev          # enter the dev shell (devenv shell)
just build        # nix build (flake)
just test         # go test -race ./...
just lint         # golangci-lint run
just fmt          # treefmt (gofmt + nixfmt)
just check        # nix flake check (build, lint, tests, fmt)

go build ./cmd/mw                              # raw Go build
go test ./...                                  # all tests
go test -run TestName ./internal/pkg/...       # single test
```

Lock-file hygiene (`flake.lock` ↔ `devenv.lock` shared inputs):

```sh
just sync         # rewrite devenv.yaml pins to match flake.lock
just verify       # assert shared inputs are in sync
just update       # update flake inputs, devenv pins, and Go deps
```

CI runs two workflows: **CI** (`devenv test` on linux+macos, Go lint/test/build)
and a **SonarCloud** analysis with Go coverage. Conventions: Conventional Commit
messages; `treefmt` formatting; table-driven tests next to the code; route all
git through `internal/repo` rather than ad-hoc `os/exec`. See `AGENTS.md` and
`CONTRIBUTING.md` for the full contributor guide.

---

## 19. Troubleshooting

| Symptom | Likely cause / fix |
|---------|--------------------|
| `mw go` prints a path but the shell doesn't move | Shell wrapper not installed — `eval "$(mw completions <shell>)"`. |
| "Refusing to auto-navigate on fuzzy match" | Intended guard. Run the suggested `mw go name@branch`, or use a more exact query. |
| Wrapper navigates but doesn't activate Nix | No matching Nix file in the worktree/subproject, or wrong `path` in an explicit `[nix]` table. |
| `direnv: error .envrc is blocked` | Run `direnv allow` once inside the worktree. |
| Wrong devshell selected for a flake | Set `devshell` in the `[nix]` table. |
| Post-create hooks skipped | By design — pass `--allow-hooks` or set `allow_hooks = true`. |
| `pr:N` / `mr:N` fails | Install `gh` / `glab` and authenticate; ensure the catalog URL is set. |
| Sparse checkout has no effect | Requires git 2.25+ and `sparse_paths` on the subproject. |
| Frecency never improves | Install the visit hook (`mw init <shell> >> ~/.<shell>rc`). |
| Stale status / odd cache | Delete the state cache and `visits.json`; they regenerate. |
| Resolver picks the wrong repo | Inspect with `mw resolver explain <query>`; tune `resolver.*` weights. |

---

## 20. Quick Reference Card

```sh
# Setup
mw init                              # create dirs
eval "$(mw completions zsh)"         # shell wrapper + completions
mw init zsh >> ~/.zshrc              # visit hook (frecency)

# Register repos
mw repo add ~/Developer/myrepo       # by path
mw repo add https://github.com/o/r   # by URL
mw config set scan_roots ~/Developer
mw repo sync --depth 2               # bulk discover

# Navigate
mw go myrepo                         # main branch
mw go myrepo feature/x               # specific branch (created if new)
mw go repo@branch                    # explicit, no fuzzy
mw go api                            # fuzzy
mw go myrepo pr:42                   # PR branch
mw go --list api                     # scored candidates

# Create / remove / merge
mw switch myrepo fix/bug -c          # new branch worktree
mw rm myrepo/fix/bug                 # remove worktree (+branch if merged)
mw merge --target main               # rebase+ff+cleanup (run inside worktree)

# Inspect
mw ls                                # all worktrees
mw ls --format json
mw wt list --branches --remotes      # rich status (current repo)
mw query --since "yesterday"         # recent activity
mw search --glob '*.go' -i TODO      # cross-repo grep
mw resolver explain api              # debug scoring

# Maintain
mw fetch                             # fetch all repos
mw prune                             # drop orphaned worktrees
mw maintenance start                 # background git maintenance
mw repo purge old-repo               # delete everything for a repo

# Config / project
mw config show
mw config set worktree_root ~/wts
mw project init                      # .makework.toml template
mw project show api                  # resolved metadata + Nix
```

---

*This manual reflects the source at the time of writing (version 0.1.0). For the
rendered documentation site, see [`docs/`](./docs/); for the day-to-day cheat
sheet, see [`.claude/skills/makework.md`](./.claude/skills/makework.md).*
