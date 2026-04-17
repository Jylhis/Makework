package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/repo"
	"github.com/jylhis/makework/internal/search"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		glob       string
		ignoreCase bool
		max        uint
	)
	cmd := &cobra.Command{
		Use:     "search <pattern>",
		Aliases: []string{"grep"},
		Short:   "Search across all project worktrees",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat := loadState()
			pattern := args[0]
			out := cmd.OutOrStdout()

			opts := search.Options{
				FileGlob:        glob,
				CaseInsensitive: ignoreCase,
				MaxResults:      max,
			}

			var allResults []search.Result
			for repoName, r := range cat.Repos {
				var parsedURL *repo.ParsedURL
				if r.URL != nil {
					if p, ok := repo.ParseRemoteURL(*r.URL); ok {
						parsedURL = &p
					}
				}
				wtPath := worktree.Path(cfg.WorktreeRoot, parsedURL, repoName, r.MainBranch)
				if !fileExistsCli(wtPath) {
					continue
				}
				results := search.Worktree(wtPath, pattern, repoName, opts)
				allResults = append(allResults, results...)
			}

			if len(allResults) == 0 {
				fmt.Fprintln(out, "No matches found.")
				return nil
			}

			grouped := search.Grouped(allResults)
			for repoName, results := range grouped {
				fmt.Fprintf(out, "%s:\n", repoName)
				for _, r := range results {
					fmt.Fprintf(out, "  %s:%d:%s\n", r.FilePath, r.LineNumber, r.LineContent)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&glob, "glob", "", `File glob filter (e.g., "*.go")`)
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Case-insensitive search")
	cmd.Flags().UintVar(&max, "max", 0, "Limit results per repo")
	return silenceSubcommand(cmd)
}
