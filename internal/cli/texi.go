package cli

import "github.com/spf13/cobra"

func newGenerateTexiCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:    "generate-texi",
		Short:  "Generate Texinfo source for info pages",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("generate-texi")
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Output file (default: stdout)")
	return silenceSubcommand(cmd)
}
