package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "new <project> <ref>",
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
			if err := worktree.Create(resolved.Repo.Path, ref, wtPath); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
			return nil
		},
	})
}
