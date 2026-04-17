package cli

import "github.com/spf13/cobra"

func newGoCmd() *cobra.Command {
	var list bool
	cmd := &cobra.Command{
		Use:   "go [project] [ref]",
		Short: "Navigate to a project worktree (supports fuzzy matching)",
		Long:  "Project name, fuzzy query, or repo@branch (interactive picker if omitted)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("go")
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "Show all matches with scores instead of navigating")
	return silenceSubcommand(cmd)
}
