package cli

import "github.com/spf13/cobra"

func newResolverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolver",
		Short: "Resolver debugging tools",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "explain <query>",
			Short: "Show resolver scoring breakdown for a query",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return notImplemented("resolver explain")
			},
		}),
	)
	return silenceSubcommand(cmd)
}
