package cli

import "github.com/spf13/cobra"

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <target>",
		Short: "Remove a worktree",
		Long:  "<project>/<ref> or worktree path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("rm")
		},
	}
	return silenceSubcommand(cmd)
}
