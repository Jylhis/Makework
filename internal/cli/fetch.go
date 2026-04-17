package cli

import "github.com/spf13/cobra"

func newFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [project]",
		Short: "Fetch updates for one or all repos",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("fetch")
		},
	}
	return silenceSubcommand(cmd)
}
