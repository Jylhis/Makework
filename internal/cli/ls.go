package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all active worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat := loadState()
			out := cmd.OutOrStdout()

			empty := true
			for name, r := range cat.Repos {
				wts, err := worktree.List(r.Path)
				if err != nil {
					continue
				}
				fmt.Fprintf(out, "%s:\n", name)
				empty = false
				for _, wt := range wts {
					if wt.IsBare {
						continue
					}
					branch := wt.Branch
					if branch == "" {
						branch = "(detached)"
					}
					orphan := ""
					if !fileExistsCli(wt.Path) {
						orphan = " (orphaned)"
					}
					fmt.Fprintf(out, "  %-30s %s%s\n", branch, wt.Path, orphan)
				}
			}
			if empty {
				fmt.Fprintln(out, "No active worktrees.")
			}
			return nil
		},
	}
	return silenceSubcommand(cmd)
}
