package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jylhis/makework/internal/xdgpath"
	"github.com/spf13/cobra"
)

func newVisitCmd() *cobra.Command {
	return silenceSubcommand(&cobra.Command{
		Use:    "visit <path>",
		Short:  "Record a visit (shell hook, not for direct use)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			visitPath := args[0]
			stateDir, err := xdgpath.StateDir()
			if err != nil {
				return nil // silent
			}
			data, err := os.ReadFile(filepath.Join(stateDir, "repo-roots.txt"))
			if err != nil {
				return nil // silent
			}
			var matchedRoot string
			for _, root := range strings.Split(string(data), "\n") {
				root = strings.TrimSpace(root)
				if root != "" && strings.HasPrefix(visitPath, root) {
					matchedRoot = root
					break
				}
			}
			if matchedRoot == "" {
				return nil
			}
			repoName := filepath.Base(matchedRoot)
			branch := "unknown"
			if out, err := exec.Command("git", "-C", visitPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
				branch = strings.TrimSpace(string(out))
			}
			recordVisit(repoName, branch)
			return nil
		},
	})
}
