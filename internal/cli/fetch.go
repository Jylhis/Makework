package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/repo"
	"github.com/spf13/cobra"
)

func newFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [project]",
		Short: "Fetch updates for one or all repos",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat := loadState()
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			var repos []*repo.Repository
			if len(args) > 0 {
				r, ok := cat.Repos[args[0]]
				if !ok {
					return fmt.Errorf("repository not found: %s", args[0])
				}
				repos = []*repo.Repository{r}
			} else {
				for _, r := range cat.Repos {
					repos = append(repos, r)
				}
			}

			hadError := false
			for _, r := range repos {
				fmt.Fprintf(out, "Fetching %s... ", r.Name)
				if err := repo.Fetch(r.Path); err != nil {
					fmt.Fprintf(out, "error: %v\n", err)
					hadError = true
					continue
				}
				fmt.Fprintln(out, "done")
				if newBranch, err := repo.CheckDefaultBranchChanged(r.Path, r.MainBranch); err == nil && newBranch != "" {
					fmt.Fprintf(errOut, "  Note: default branch appears to have changed from '%s' to '%s'\n", r.MainBranch, newBranch)
				}
			}
			if hadError {
				return fmt.Errorf("one or more repos failed to fetch")
			}
			return nil
		},
	}
	return silenceSubcommand(cmd)
}
