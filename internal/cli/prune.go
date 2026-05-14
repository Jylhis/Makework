package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:   "prune",
		Short: "Prune orphaned worktrees whose directories no longer exist",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat, err := loadState()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			anyPruned := false
			for name, r := range cat.Repos {
				count, err := worktree.Prune(r.Path)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: error pruning: %v\n", name, err)
					continue
				}
				if count > 0 {
					fmt.Fprintf(out, "%s: pruned %d orphaned worktree(s)\n", name, count)
					anyPruned = true
				}
			}
			if !anyPruned {
				fmt.Fprintln(out, "No orphaned worktrees found.")
			}
			return nil
		},
	})
}
