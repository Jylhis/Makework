package cli

import "github.com/spf13/cobra"

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <project> <ref>",
		Short: "Create a new worktree",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("new")
		},
	}
	return silenceSubcommand(cmd)
}
