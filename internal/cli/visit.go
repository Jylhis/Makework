package cli

import "github.com/spf13/cobra"

func newVisitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "visit <path>",
		Short:  "Record a visit (shell hook, not for direct use)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("visit")
		},
	}
	return silenceSubcommand(cmd)
}
