package cli

import "github.com/spf13/cobra"

func newSearchCmd() *cobra.Command {
	var (
		glob       string
		ignoreCase bool
		max        uint
	)
	cmd := &cobra.Command{
		Use:     "search <pattern>",
		Aliases: []string{"grep"},
		Short:   "Search across all project worktrees",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("search")
		},
	}
	cmd.Flags().StringVar(&glob, "glob", "", `File glob filter (e.g., "*.go")`)
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Case-insensitive search")
	cmd.Flags().UintVar(&max, "max", 0, "Limit results per repo")
	return silenceSubcommand(cmd)
}
