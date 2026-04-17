package cli

import "github.com/spf13/cobra"

func newLsCmd() *cobra.Command {
	var prune bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all active worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("ls")
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "Prune orphaned worktrees whose directories no longer exist")
	return silenceSubcommand(cmd)
}
