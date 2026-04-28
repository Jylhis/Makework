# makework (`mw`) - Usage Scenarios

Example scenarios showing how to use the `mw` CLI for managing git worktrees across multiple projects.

---

## 1. First-time setup

Initialize makework and configure where your repos live:

```shell
$ mw init
Initialized makework:
  created /Users/you/.config/makework/config.toml
  created /Users/you/.config/makework/catalog.toml
  created /Users/you/.local/state/makework/

$ mw config set scan_roots ~/Developer
Set scan_roots = ~/Developer
```

Set up shell integration (completions + visit tracking for frecency scoring):

```shell
# Add to your .zshrc:
eval "$(mw init zsh)"

# Or for bash, add to your .bashrc:
eval "$(mw init bash)"
```

---

## 2. Discover and register existing repos

Scan your Developer directory to auto-register all git repos:

```shell
$ mw repo sync
Scanning: /Users/you/Developer
Registered 5 new repo(s):
  myapp
  dotfiles
  api-server
  frontend
  shared-lib

$ mw repo sync --depth 3
Scanning: /Users/you/Developer
No new repositories found.
```

---

## 3. Manually add a repo

Register a repo from a remote URL or local path:

```shell
$ mw repo add https://github.com/jylhis/makework.git
Registered repository: makework

$ mw repo add /Users/you/Developer/side-project
Registered repository: side-project

$ mw repo add .
Registered repository: side-project2
```

---

## 4. List registered repos

```shell
$ mw repo list
NAME                 URL                                      BRANCH     WORKTREES
myapp                https://github.com/org/myapp.git         main       2
api-server           https://github.com/org/api-server.git    main       1
frontend             (local)                                  main       0
makework             https://github.com/jylhis/makework.git   main       3
```

---

## 5. Navigate to a project worktree

Jump to a project's main branch worktree (creates it if needed):

```shell
$ mw go myapp
/Users/you/.local/share/makework/worktrees/org/myapp/main
```

Navigate to a specific branch:

```shell
$ mw go myapp feature/auth
/Users/you/.local/share/makework/worktrees/org/myapp/feature/auth
```

Use fuzzy matching - you don't need the full name:

```shell
$ mw go api
/Users/you/.local/share/makework/worktrees/org/api-server/main
```

Use `repo@branch` syntax for precise routing:

```shell
$ mw go myapp@develop
/Users/you/.local/share/makework/worktrees/org/myapp/develop
```

```shell
$ mw go myapp@aaaaaa # NOTE: hash
/Users/you/.local/share/makework/worktrees/org/myapp/aaaaaaaa
```

When multiple repos match, disambiguation kicks in:

```shell
$ mw go app
Multiple matches for 'app':
  1. myapp (main)
  2. api-server (main)
Use repo@branch syntax for precise routing.
Error: ambiguous match
```

See all fuzzy match results with scores:

```shell
$ mw go --list api
  1. api-server         main            0.950  api-server
  2. myapp              main            0.320  myapp
```

---

## 6. Create a new branch worktree

Create a worktree with a new branch (branched from main):

```shell
$ mw switch myapp fix/login-bug -c
/Users/you/.local/share/makework/worktrees/org/myapp/fix/login-bug

# NOTE: If inside a git repo
$ mw switch fix/login-bug -c
/Users/you/.local/share/makework/worktrees/org/myapp/fix/login-bug

```

Create a branch from a specific base:

```shell
$ mw switch myapp hotfix/urgent -c -b release/v2
/Users/you/.local/share/makework/worktrees/org/myapp/hotfix/urgent
```

---

## 7. List active worktrees

```shell
$ mw ls
myapp:
  main                           /Users/you/.local/share/makework/worktrees/org/myapp/main
  feature/auth                   /Users/you/.local/share/makework/worktrees/org/myapp/feature/auth
  fix/login-bug                  /Users/you/.local/share/makework/worktrees/org/myapp/fix/login-bug
api-server:
  main                           /Users/you/.local/share/makework/worktrees/org/api-server/main
makework:
  main                           /Users/you/.local/share/makework/worktrees/jylhis/makework/main
  feature/resolver               /Users/you/.local/share/makework/worktrees/jylhis/makework/feature/resolver
```

Prune orphaned worktrees (directories deleted but git refs remain):

```shell
$ mw prune
myapp: pruned 1 orphaned worktree(s)
```

---

## 8. Remove a worktree

```shell
$ mw rm myapp/feature/auth
Removed worktree: /Users/you/.local/share/makework/worktrees/org/myapp/feature/auth

# NOTE: if inside repo
$ mw rm feature/auth
Removed worktree: /Users/you/.local/share/makework/worktrees/org/myapp/feature/auth
```

# NOTE: All commands work on the current repo if the commands are run inside repo
---

## 9. Fetch updates

Fetch all repos at once:

```shell
$ mw fetch
Fetching myapp... done
Fetching api-server... done
Fetching frontend... done
Fetching makework... done
```

Fetch a single repo:

```shell
$ mw fetch myapp
Fetching myapp... done
```

---

## 10. Search across all projects

Grep across all worktrees:

```shell
# TODO make sure we are able to search files, folder and file content )maybe find and search)
# alias these specific search types to rg and fd
# Also add support for searching with ast-grep (sg)
$ mw search "TODO" # NOTE: alias search to also rg
myapp:
  src/auth.go:42:// TODO: add rate limiting
  src/handler.go:15:// TODO: validate input
api-server:
  cmd/main.go:8:// TODO: add graceful shutdown

$ mw search --glob "*.go" --ignore-case "fixme"
myapp:
  internal/db/conn.go:33:// FIXME: connection pool size

$ mw search --max 5 "error"
myapp:
  src/auth.go:12:	return fmt.Errorf("auth error: %w", err)
  src/handler.go:20:	log.Printf("error handling request: %v", err)
```

---

## 11. Query recent activity

See what happened across all projects in the last week:

```shell
# TODO: search is there any existing query language/ systems we can use
$ mw query
myapp:
  a1b2c3d 2026-04-27 fix: resolve login redirect loop
  e4f5g6h 2026-04-25 feat: add OAuth2 provider support
api-server:
  i7j8k9l 2026-04-26 refactor: extract middleware chain

$ mw query --since "yesterday" --format full
myapp:
  a1b2c3d4e5f6 Alice <alice@example.com> 2026-04-27
    fix: resolve login redirect loop

$ mw query --since "2026-04-01" --until "2026-04-15" --author "Alice"
myapp:
  b2c3d4e 2026-04-10 feat: add session management
  c3d4e5f 2026-04-03 fix: token refresh race condition
```

---

## 12. Debug the fuzzy resolver

See how the resolver scores each project for a query:

```shell
$ mw resolver explain "app"
Query: 'app'
Weights: fuzzy=0.50 frecency=0.20 activity=0.15 context=0.15

REPO                 BRANCH        FUZZY   FREC    ACT    CTX    TOTAL
myapp                main          0.800  0.600  0.300  0.100    0.595
api-server           main          0.400  0.200  0.500  0.100    0.340
frontend             main          0.100  0.000  0.200  0.000    0.080
```

---

## 13. Manage configuration

View effective config:

```shell
$ mw config show
SETTING              VALUE                                              SOURCE
bare_root            /Users/you/.local/share/makework/bare               default
worktree_root        /Users/you/.local/share/makework/worktrees          default
scan_roots           [/Users/you/Developer]                              config.toml
```

Edit config in your editor:

```shell
$ mw config edit
# Opens $EDITOR with config.toml
```

---

## 14. Remove a repo from the catalog

Unregister (keeps files on disk):

```shell
$ mw repo rm frontend
Warning: frontend has 1 active worktree(s):
  main /Users/you/.local/share/makework/worktrees/frontend/main
These worktrees will become orphaned.
Removed catalog entry: frontend
```

Purge (removes bare clone + all worktrees + unregisters):

```shell
$ mw repo purge old-project
Purged: old-project
```

---

## 15. Git maintenance

Register repos for automatic background git maintenance (prefetch, gc, etc.):

```shell
$ mw maintenance start
Registered for maintenance

$ mw maintenance status
Maintenance: registered

$ mw maintenance stop
Unregistered from maintenance
```

---

## Typical workflow

A day-in-the-life workflow switching between projects and branches:

```shell
# Morning: fetch everything
$ mw fetch

# Check what happened while you were away
$ mw query --since "yesterday"

# Jump to your main project
$ cd $(mw go myapp)

# Start a new feature branch
$ cd $(mw go myapp feature/new-thing -c)

# Quick context switch to fix a bug in another project
$ cd $(mw go api fix/urgent -c)

# Done with the fix, back to the feature
$ cd $(mw go myapp feature/new-thing)

# Search for something across all projects
$ mw search "deprecated_function"

# End of day: clean up finished branches
$ mw rm api-server/fix/urgent
$ mw ls
```
