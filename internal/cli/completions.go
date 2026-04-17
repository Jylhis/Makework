package cli

import "github.com/spf13/cobra"

func newCompletionsCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:       "completions <shell>",
		Short:     "Generate shell completions and wrapper",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "elvish", "powershell", "nushell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("completions")
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Write completion file to directory (packaging mode, no wrapper)")
	return silenceSubcommand(cmd)
}
