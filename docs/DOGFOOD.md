# Dogfood Plan (v0)

Makework only earns its keep if it makes Jylhis multi-repo work measurably
faster. This is the v0 dogfood plan: who uses it, how, and how we know it's
working.

## Primary users

- **Markus** — primary developer. Daily driver across ~20+ active Jylhis
  repos plus upstream contributions.
- **Paperclip agents working in Jylhis projects** — every agent that needs to
  navigate, search, or fetch across the Jylhis repo set uses `mw` via the
  [`makework` skill](../.claude/skills/makework.md).

## What "dogfooding" means concretely

The bar is that the workflows below run through `mw` instead of raw
`git`/`cd`/`find`:

| Workflow                                                    | Command                          |
| ----------------------------------------------------------- | -------------------------------- |
| Switch to any registered project                            | `mw go <fuzzy-query>`            |
| Switch to a specific branch (auto-create worktree)          | `mw go <project>@<ref>`          |
| Pull updates across every repo                              | `mw fetch`                       |
| Find a symbol across every active worktree                  | `mw search <pattern>`            |
| See recent activity across every repo                       | `mw query --since "7 days ago"`  |
| Status snapshot                                             | `mw`                             |

When any of these is faster to do _outside_ `mw`, that is a bug — file it.

## Day-1 setup (already in place)

- Catalog seeded from `~/Developer` via `mw repo sync`.
- Shell integration installed (`eval "$(mw init zsh)"`).
- Nix auto-activation enabled (default).
- MCP server registered with Claude Desktop / `claude` CLI (see
  [`docs/integrations/mcp.mdx`](./integrations/mcp.mdx)).
- Emacs package loaded from [`editors/emacs/makework.el`](../editors/emacs/makework.el).

## Health signals

Tracked informally for now; the v1 plan is to expose them via `mw status
--json`:

- `mw go` time-to-first-byte (TTFB) under 100 ms on a warm catalog.
- Resolver chooses the intended repo on the first try (no `--explain` needed).
- No leftover orphan worktrees after a week of normal use (`mw prune` is a
  no-op).
- Agents using the `makework` skill never fall back to raw `git clone`.

## v0 → v1 — what we'll learn

The first month of real use should tell us:

1. Which commands need flag/UX work (data: shell history + agent transcripts).
2. Whether sparse-checkout via `.makework.toml` actually scales to the
   monorepo cases it was built for.
3. Whether frecency scoring matches Markus's mental model, or whether the
   weights need to shift toward recency.
4. What the right release cadence is — `mw` is currently `0.1.0`
   (see [`VERSION`](../VERSION)).

File findings as GitHub issues tagged `dogfood`. Promote recurring pain
points into proper roadmap items on the parent onboarding issue.
