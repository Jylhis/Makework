package cli

import "github.com/spf13/cobra"

func newManCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("man")
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory")
	return silenceSubcommand(cmd)
}
