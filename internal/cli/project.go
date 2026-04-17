package cli

import "github.com/spf13/cobra"

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Per-project configuration",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "init",
			Short: "Create a .makework.toml in the current worktree",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("project init")
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "show",
			Short: "Show project metadata resolved from catalog",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("project show")
			},
		}),
	)
	return silenceSubcommand(cmd)
}
