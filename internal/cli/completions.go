package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

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
