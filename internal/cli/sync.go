package cli

import "github.com/spf13/cobra"

func newSyncCmd() *cobra.Command {
	var (
		depth   uint32
		exclude []string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Discover and register repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("sync")
		},
	}
	cmd.Flags().Uint32Var(&depth, "depth", 0, "Maximum directory depth to scan (overrides config)")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Directory name patterns to skip (repeatable, merged with config)")
	return silenceSubcommand(cmd)
}
