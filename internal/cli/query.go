package cli

import "github.com/spf13/cobra"

func newQueryCmd() *cobra.Command {
	var (
		since  string
		until  string
		author string
		format string
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query recent activity across projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("query")
		},
	}
	cmd.Flags().StringVar(&since, "since", "7 days ago", `Show commits since this date (e.g., "yesterday", "2026-03-01")`)
	cmd.Flags().StringVar(&until, "until", "", "Show commits until this date")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&format, "format", "short", "Output format: short|full")
	return silenceSubcommand(cmd)
}
