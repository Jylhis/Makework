package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage settings",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "show",
			Short: "Print the effective config",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("config show")
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a config key (dot-separated)",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("config set")
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "edit",
			Short: "Open the config file in $EDITOR",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("config edit")
			},
		}),
	)
	return silenceSubcommand(cmd)
}
