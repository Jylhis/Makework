package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCompletionsCmd() *cobra.Command {
	var outputDir string
	cmd := &cobra.Command{
		Use:       "completions <shell>",
		Short:     "Generate shell completions and wrapper",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "elvish", "powershell", "nushell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			shell := args[0]

			if outputDir != "" {
				return generateCompletionFile(root, shell, outputDir)
			}
			return generateCompletionStdout(root, shell, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Write completion file to directory (packaging mode, no wrapper)")
	return silenceSubcommand(cmd)
}

func generateCompletionFile(root *cobra.Command, shell, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	switch shell {
	case "bash":
		return root.GenBashCompletionFileV2(filepath.Join(dir, "mw"), true)
	case "zsh":
		return root.GenZshCompletionFile(filepath.Join(dir, "_mw"))
	case "fish":
		return root.GenFishCompletionFile(filepath.Join(dir, "mw.fish"), true)
	case "powershell":
		return root.GenPowerShellCompletionFile(filepath.Join(dir, "mw.ps1"))
	default:
		return fmt.Errorf("packaging mode not supported for shell: %s", shell)
	}
}

func generateCompletionStdout(root *cobra.Command, shell string, out interface{ Write([]byte) (int, error) }) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "zsh":
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, true)
	case "powershell":
		return root.GenPowerShellCompletion(out)
	default:
		return fmt.Errorf("completions not supported for shell: %s", shell)
	}
}
