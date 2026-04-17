package cli

import (
	"fmt"
	"strings"

	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "rm <target>",
		Short: "Remove a worktree",
		Long:  "<project>/<ref> or worktree path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			target := args[0]
			parts := strings.SplitN(target, "/", 2)
			if len(parts) != 2 {
				return fmt.Errorf("target must be <project>/<ref> or a worktree path")
			}
			projectName, refName := parts[0], parts[1]
			resolved, err := cat.FindProjectUnambiguous(projectName)
			if err != nil {
				return err
			}
			wtPath := resolvedWorktreePath(cfg, resolved, refName)
			if err := worktree.Remove(resolved.Repo.Path, wtPath); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", wtPath)
			return nil
		},
	})
}
