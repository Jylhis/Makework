package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newManCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			header := &doc.GenManHeader{
				Title:   "MW",
				Section: "1",
			}
			return doc.GenManTree(root, header, outputDir)
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory")
	return silenceSubcommand(cmd)
}
