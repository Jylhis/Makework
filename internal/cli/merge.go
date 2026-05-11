package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/jylhis/makework/internal/integration"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/status"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newMergeCmd() *cobra.Command {
	var target string
	var keepWorktree bool
	var keepBranch bool
	var force bool
	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Rebase the current branch onto target, fast-forward target, then clean up",
		Long: "Merge the current worktree's branch into <target> using a rebase + " +
			"fast-forward strategy. Removes the worktree and deletes the branch on success.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			wtPath, err := repo.RunGitCapture("-C", wd, "rev-parse", "--show-toplevel")
			if err != nil {
				return fmt.Errorf("not inside a git worktree: %w", err)
			}
			wtPath = strings.TrimSpace(wtPath)

			barePath, err := repo.RunGitCapture("-C", wtPath, "rev-parse", "--git-common-dir")
			if err != nil {
				return fmt.Errorf("resolve bare repo: %w", err)
			}
			barePath = strings.TrimSpace(barePath)

			branch, err := repo.RunGitCapture("-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return fmt.Errorf("resolve current branch: %w", err)
			}
			branch = strings.TrimSpace(branch)
			if branch == "HEAD" {
				return fmt.Errorf("detached HEAD — cannot determine branch")
			}

			if target == "" {
				t, err := repo.GetDefaultBranch(barePath)
				if err != nil {
					return fmt.Errorf("resolve default branch: %w", err)
				}
				target = t
			}
			if branch == target {
				return fmt.Errorf("already on target branch %s", target)
			}

			if !force {
				st := status.Get(wtPath)
				if st.DirtyCount > 0 {
					return fmt.Errorf("worktree is dirty (%d uncommitted changes); commit or use --force", st.DirtyCount)
				}
			}

			fmt.Fprintf(out, "Rebasing %s onto %s...\n", branch, target)
			if err := repo.Rebase(wtPath, target); err != nil {
				return fmt.Errorf("rebase failed: %w", err)
			}

			featureHead, err := repo.RunGitCapture("-C", wtPath, "rev-parse", "HEAD")
			if err != nil {
				return fmt.Errorf("resolve rebased HEAD: %w", err)
			}
			featureHead = strings.TrimSpace(featureHead)

			if err := fastForwardTarget(barePath, target, branch, featureHead); err != nil {
				return fmt.Errorf("fast-forward %s: %w", target, err)
			}
			fmt.Fprintf(out, "Fast-forwarded %s to %s\n", target, featureHead[:min(8, len(featureHead))])

			if keepWorktree {
				return nil
			}

			if err := worktree.Remove(barePath, wtPath); err != nil {
				return fmt.Errorf("remove worktree: %w", err)
			}
			fmt.Fprintf(out, "Removed worktree: %s\n", wtPath)

			if keepBranch {
				return nil
			}

			state, _ := integration.Check(barePath, branch, target)
			if err := repo.DeleteBranch(barePath, branch, false); err != nil {
				fmt.Fprintf(out, "Branch %s kept: %v\n", branch, err)
				return nil
			}
			fmt.Fprintf(out, "Deleted branch: %s (%s)\n", branch, state)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target branch (default: repo's default branch)")
	cmd.Flags().BoolVar(&keepWorktree, "keep-worktree", false, "Do not remove the worktree after merging")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "Do not delete the branch after removing the worktree")
	cmd.Flags().BoolVar(&force, "force", false, "Merge even if the worktree has uncommitted changes")
	return silenceSubcommand(cmd)
}

// fastForwardTarget moves target to point at featureHead. If target is
// checked out in a worktree, runs `merge --ff-only` there; otherwise
// updates the ref directly.
func fastForwardTarget(barePath, target, sourceBranch, sourceHead string) error {
	wts, err := worktree.List(barePath)
	if err != nil {
		return err
	}
	for _, w := range wts {
		if w.IsBare {
			continue
		}
		if w.Branch == target {
			return repo.MergeFF(w.Path, sourceBranch)
		}
	}
	_, err = repo.RunGitCapture("-C", barePath, "update-ref", "refs/heads/"+target, sourceHead)
	return err
}
