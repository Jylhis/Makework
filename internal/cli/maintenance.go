package cli

import "github.com/spf13/cobra"

func newMaintenanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Git maintenance management",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "start",
			Short: "Register all bare repos with git maintenance",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("maintenance start")
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "stop",
			Short: "Unregister all bare repos from git maintenance",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("maintenance stop")
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "status",
			Short: "Show maintenance status for each bare repo",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("maintenance status")
			},
		}),
	)
	return silenceSubcommand(cmd)
}
