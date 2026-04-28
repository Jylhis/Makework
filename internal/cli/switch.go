package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newSwitchCmd() *cobra.Command {
	var createBranch bool
	var baseBranch string
	cmd := &cobra.Command{
		Use:   "switch <project> <ref>",
		Short: "Create a new worktree",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			project, ref := args[0], args[1]
			resolved, err := cat.FindProjectUnambiguous(project)
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
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&createBranch, "create", "c", false, "Create a new branch for the worktree")
	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch for -c (defaults to repo's main branch)")
	return silenceSubcommand(cmd)
}
