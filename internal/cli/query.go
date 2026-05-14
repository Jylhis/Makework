package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/fsx"
	"github.com/jylhis/makework/internal/query"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var (
		since  string
		until  string
		author string
		format string
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query recent activity across projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat, err := loadState()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			var untilPtr, authorPtr *string
			if until != "" {
				untilPtr = &until
			}
			if author != "" {
				authorPtr = &author
			}

			var allEntries []query.Entry
			for repoName, r := range cat.Repos {
				wts, err := worktree.List(r.Path)
				if err != nil {
					continue
				}
				for _, wt := range wts {
					if wt.IsBare || !fsx.PathExists(wt.Path) {
						continue
					}
					branch := wt.Branch
					if branch == "" {
						branch = "(detached)"
					}
					entries := query.LogWorktree(wt.Path, branch, repoName, since, untilPtr, authorPtr)
					allEntries = append(allEntries, entries...)
				}
			}

			allEntries = query.Dedup(allEntries)
			if len(allEntries) == 0 {
				fmt.Fprintln(out, "No activity found.")
				return nil
			}

			summary := query.Summary(allEntries)
			for repoName, entries := range summary {
				fmt.Fprintf(out, "%s:\n", repoName)
				for _, e := range entries {
					switch format {
					case "full":
						fmt.Fprintf(out, "  %s %s %s\n", e.CommitHash, e.Author, e.Date)
						fmt.Fprintf(out, "    %s\n", e.Message)
					default:
						hash := e.CommitHash
						if len(hash) > 7 {
							hash = hash[:7]
						}
						fmt.Fprintf(out, "  %s %s %s\n", hash, e.Date, e.Message)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7 days ago", `Show commits since this date (e.g., "yesterday", "2026-03-01")`)
	cmd.Flags().StringVar(&until, "until", "", "Show commits until this date")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&format, "format", "short", "Output format: short|full")
	return silenceSubcommand(cmd)
}
