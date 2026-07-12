package cli

import (
	"fmt"
	"io"

	"github.com/jylhis/makework/internal/integration"
	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	var keepBranch bool
	var forceDelete bool
	cmd := &cobra.Command{
		Use:   "rm <target>",
		Short: "Remove a worktree (and delete its branch if integrated)",
		Long:  "<project>/<ref> or <project>@<ref>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat, err := loadState()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			target := args[0]

			projectName, refName, ok := splitTarget(target)
			if !ok {
				return fmt.Errorf("target must be <project>/<ref> or <project>@<ref>")
			}
			resolved, err := cat.FindProjectUnambiguous(projectName)
			if err != nil {
				return err
			}
			wtPath := resolvedWorktreePath(cfg, resolved, refName)

			if err := worktree.Remove(resolved.Repo.Path, wtPath); err != nil {
				return err
			}
			fmt.Fprintf(out, "Removed worktree: %s\n", wtPath)

			if keepBranch {
				return nil
			}

			defaultBranch := resolved.Repo.MainBranch
			if defaultBranch == "" {
				if d, err := repo.GetDefaultBranch(resolved.Repo.Path); err == nil {
					defaultBranch = d
				}
			}
			if defaultBranch != "" && refName == defaultBranch {
				fmt.Fprintf(out, "Branch %s kept (default branch)\n", refName)
				return nil
			}

			deleteBranchAfterRemove(out, resolved.Repo.Path, refName, defaultBranch, forceDelete)
			return nil
		},
	}
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "Do not delete the branch after removing the worktree")
	cmd.Flags().BoolVarP(&forceDelete, "force-delete", "D", false, "Force-delete the branch even if not integrated")
	return silenceSubcommand(cmd)
}

// deleteBranchAfterRemove deletes branch once its worktree is gone,
// unless integration.Check classifies it as diverged and forceDelete is
// not set. It reports the outcome to out and never returns an error: a
// kept or undeletable branch is not a failure of the remove itself.
func deleteBranchAfterRemove(out io.Writer, repoPath, branch, defaultBranch string, forceDelete bool) {
	state, err := integration.Check(repoPath, branch, defaultBranch)
	if err != nil {
		fmt.Fprintf(out, "Branch %s kept (integration check failed: %v)\n", branch, err)
		return
	}
	if state == integration.StateDiverged && !forceDelete {
		fmt.Fprintf(out, "Branch %s kept (diverged from %s; use -D to force delete)\n",
			branch, defaultBranch)
		return
	}
	if err := repo.DeleteBranch(repoPath, branch, forceDelete); err != nil {
		fmt.Fprintf(out, "Branch %s not deleted: %v\n", branch, err)
		return
	}
	fmt.Fprintf(out, "Deleted branch: %s (%s)\n", branch, state)
}

// splitTarget accepts either "project/ref" or "project@ref" and returns
// the two components plus an ok flag. The first '/' or '@' wins.
func splitTarget(target string) (project, ref string, ok bool) {
	for i, ch := range target {
		if ch == '/' || ch == '@' {
			project = target[:i]
			ref = target[i+1:]
			if project == "" || ref == "" {
				return "", "", false
			}
			return project, ref, true
		}
	}
	// Strict mode: a single token like "repo" with no '/' or '@'
	// separator is not a valid rm target.
	return "", "", false
}
