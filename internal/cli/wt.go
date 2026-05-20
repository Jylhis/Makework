package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/integration"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/status"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

// newWtCmd is the worktrunk-compatible command group. Subcommands
// (`switch`, `list`, `remove`) operate on the catalog repo that owns
// $PWD, rather than taking an explicit project argument like the
// top-level `mw switch / rm / ls`.
func newWtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wt",
		Short: "worktrunk-compatible worktree commands scoped to the current repo",
		Long: "wt provides a subset of the worktrunk CLI surface backed by the " +
			"makework catalog. The current repo is detected from $PWD, so commands " +
			"do not take a project argument.",
	}
	cmd.AddCommand(newWtSwitchCmd(), newWtListCmd(), newWtRemoveCmd())
	return silenceSubcommand(cmd)
}

func newWtSwitchCmd() *cobra.Command {
	var createBranch bool
	var baseBranch string
	cmd := &cobra.Command{
		Use:   "switch <branch>",
		Short: "Switch to (or create) a worktree for <branch> in the current repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat, err := loadState()
			if err != nil {
				return err
			}
			resolved, _, err := currentRepo(cat)
			if err != nil {
				return wtRepoHint(err)
			}
			ref := args[0]
			ref, err = resolveBranchShortcut(ref, resolved)
			if err != nil {
				return err
			}
			wtPath := resolvedWorktreePath(cfg, resolved, ref)
			if createBranch {
				base := baseBranch
				if base == "" {
					base = resolved.Repo.MainBranch
				}
				if err := worktree.CreateBranch(resolved.Repo.Path, ref, wtPath, base); err != nil {
					return err
				}
			} else {
				if err := worktree.Create(resolved.Repo.Path, ref, wtPath); err != nil {
					return err
				}
			}
			recordVisit(resolved.Repo.Name, ref)
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&createBranch, "create", "c", false, "Create a new branch for the worktree")
	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch for -c (defaults to repo's main branch)")
	return silenceSubcommand(cmd)
}

func newWtListCmd() *cobra.Command {
	var includeBranches bool
	var includeRemotes bool
	var progressive bool
	var noProgressive bool
	var full bool
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List worktrees for the current repo (worktrunk-compatible)",
		Long: "Lists the worktrees of the catalog repository that owns $PWD.\n\n" +
			"--branches adds orphan local branches (no worktree).\n" +
			"--remotes adds remote-tracking refs.\n" +
			"--full and --progressive are accepted for worktrunk compatibility " +
			"but currently render the same table; CI/LLM summaries and streaming " +
			"updates are not yet implemented.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat, err := loadState()
			if err != nil {
				return err
			}
			resolved, _, err := currentRepo(cat)
			if err != nil {
				return wtRepoHint(err)
			}
			out := cmd.OutOrStdout()

			entries, seen := collectWtListEntries(resolved)
			if includeBranches {
				if branches, err := repo.ListBranches(resolved.Repo.Path); err == nil {
					for _, b := range branches {
						if seen[b] {
							continue
						}
						entries = append(entries, lsEntry{
							Repo:   resolved.Repo.Name,
							Status: status.WorktreeStatus{Branch: b},
						})
						seen[b] = true
					}
				}
			}
			if includeRemotes {
				if remotes, err := repo.ListRemoteBranches(resolved.Repo.Path); err == nil {
					for _, b := range remotes {
						entries = append(entries, lsEntry{
							Repo:   resolved.Repo.Name,
							Status: status.WorktreeStatus{Branch: b},
						})
					}
				}
			}
			_ = progressive
			_ = noProgressive
			_ = full

			if format == "json" {
				return json.NewEncoder(out).Encode(entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "No active worktrees.")
				return nil
			}
			writeLsTable(out, entries)
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeBranches, "branches", false, "Include local branches without worktrees")
	cmd.Flags().BoolVar(&includeRemotes, "remotes", false, "Include remote-tracking branches")
	cmd.Flags().BoolVar(&progressive, "progressive", false, "Show fast info first, update with slow info (no-op for now)")
	cmd.Flags().BoolVar(&noProgressive, "no-progressive", false, "Force buffered rendering (no-op for now)")
	cmd.Flags().BoolVar(&full, "full", false, "Show CI status and LLM summaries (not yet implemented)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table or json")
	return silenceSubcommand(cmd)
}

func newWtRemoveCmd() *cobra.Command {
	var force bool
	var noDeleteBranch bool
	var forceDelete bool
	cmd := &cobra.Command{
		Use:   "remove [<branch>...]",
		Short: "Remove one or more worktrees in the current repo",
		Long: "Removes worktrees by branch name. When no branches are given, " +
			"removes the worktree for the current branch.\n\n" +
			"By default, the corresponding branch is also deleted when " +
			"integration.Check classifies it as merged. --no-delete-branch " +
			"preserves the branch; -D force-deletes branches that have diverged.\n\n" +
			"Refuses to remove a worktree containing untracked files unless --force " +
			"is passed.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat, err := loadState()
			if err != nil {
				return err
			}
			resolved, currentBranch, err := currentRepo(cat)
			if err != nil {
				return wtRepoHint(err)
			}
			targets := args
			if len(targets) == 0 {
				if currentBranch == "" {
					return fmt.Errorf("no branch specified and current HEAD is detached")
				}
				targets = []string{currentBranch}
			}
			out := cmd.OutOrStdout()
			var firstErr error
			for _, branch := range targets {
				if err := removeOneWorktree(out, cfg, resolved, branch, force, noDeleteBranch, forceDelete); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					fmt.Fprintf(out, "Error removing %s: %v\n", branch, err)
				}
			}
			return firstErr
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Remove even if the worktree contains untracked files")
	cmd.Flags().BoolVar(&noDeleteBranch, "no-delete-branch", false, "Keep the branch after removing the worktree")
	cmd.Flags().BoolVarP(&forceDelete, "force-delete", "D", false, "Force-delete the branch even if it has diverged")
	return silenceSubcommand(cmd)
}

func removeOneWorktree(
	out io.Writer,
	cfg *config.Config,
	resolved *catalog.ResolvedProject,
	branch string,
	force, noDeleteBranch, forceDelete bool,
) error {
	wtPath := resolvedWorktreePath(cfg, resolved, branch)

	if !force {
		if dirty, err := repo.RunGitCapture("-C", wtPath, "status", "--porcelain"); err == nil {
			if strings.TrimSpace(dirty) != "" {
				return fmt.Errorf("worktree %s has uncommitted or untracked changes; use --force to remove anyway", wtPath)
			}
		}
		// If the worktree path doesn't exist git status fails — that's
		// fine, worktree.Remove will produce a clearer error below.
	}

	removeFn := worktree.Remove
	if force {
		removeFn = worktree.RemoveForce
	}
	if err := removeFn(resolved.Repo.Path, wtPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed worktree: %s\n", wtPath)

	if noDeleteBranch {
		return nil
	}

	defaultBranch := resolved.Repo.MainBranch
	if defaultBranch == "" {
		if d, err := repo.GetDefaultBranch(resolved.Repo.Path); err == nil {
			defaultBranch = d
		}
	}

	state, err := integration.Check(resolved.Repo.Path, branch, defaultBranch)
	if err != nil {
		fmt.Fprintf(out, "Branch %s kept (integration check failed: %v)\n", branch, err)
		return nil
	}
	if state == integration.StateDiverged && !forceDelete {
		fmt.Fprintf(out, "Branch %s kept (diverged from %s; use -D to force delete)\n",
			branch, defaultBranch)
		return nil
	}
	if err := repo.DeleteBranch(resolved.Repo.Path, branch, forceDelete); err != nil {
		fmt.Fprintf(out, "Branch %s not deleted: %v\n", branch, err)
		return nil
	}
	fmt.Fprintf(out, "Deleted branch: %s (%s)\n", branch, state)
	return nil
}

// collectWtListEntries returns the entries for the current repo's
// live worktrees plus a set of branch names already represented.
func collectWtListEntries(resolved *catalog.ResolvedProject) ([]lsEntry, map[string]bool) {
	seen := map[string]bool{}
	var entries []lsEntry
	wts, err := worktree.List(resolved.Repo.Path)
	if err != nil {
		return entries, seen
	}
	for _, wt := range wts {
		if wt.IsBare {
			continue
		}
		st := status.GetFull(wt.Path, resolved.Repo.Path, resolved.Repo.MainBranch)
		if st.Branch != "" && st.Branch != "unknown" {
			seen[st.Branch] = true
		}
		entries = append(entries, lsEntry{Repo: resolved.Repo.Name, Status: st})
	}
	return entries, seen
}

// wtRepoHint annotates an ErrRepoNotFound with guidance to run
// `mw repo add` when the cwd is a git repo that isn't catalogued.
func wtRepoHint(err error) error {
	var notFound catalog.ErrRepoNotFound
	if errors.As(err, &notFound) {
		return fmt.Errorf("current directory is not a catalog repository (git-dir: %s); run `mw repo add <path>` to register it",
			notFound.Name)
	}
	return err
}
