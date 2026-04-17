package cli

import (
	"fmt"
	"os"

	"github.com/jylhis/makework/internal/maintenance"
	"github.com/spf13/cobra"
)

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
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				if err := maintenance.Register(cwd); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Registered for maintenance")
				return nil
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "stop",
			Short: "Unregister all bare repos from git maintenance",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				if err := maintenance.Unregister(cwd); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Unregistered from maintenance")
				return nil
			},
		}),
		silenceSubcommand(&cobra.Command{
			Use:   "status",
			Short: "Show maintenance status for each bare repo",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				registered, err := maintenance.Status(cwd)
				if err != nil {
					return err
				}
				status := "not registered"
				if registered {
					status = "registered"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Maintenance: %s\n", status)
				return nil
			},
		}),
	)
	return silenceSubcommand(cmd)
}
