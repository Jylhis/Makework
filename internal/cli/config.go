package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "show",
			Short: "Print the effective config",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := loadConfig()
				if err != nil {
					return err
				}
				entries, err := cfg.Show()
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "%-20s %-50s SOURCE\n", "SETTING", "VALUE")
				for _, e := range entries {
					fmt.Fprintf(out, "%-20s %-50s %s\n", e.Key, e.Value, e.Source)
				}
				return nil
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a config key (dot-separated)",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := config.Set(args[0], args[1]); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
				return nil
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "edit",
			Short: "Open the config file in $EDITOR",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				path, err := config.Path()
				if err != nil {
					return err
				}
				return editFile(path)
			},
		}),
	)
	return silenceSubcommand(cmd)
}
