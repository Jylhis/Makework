package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jylhis/makework/internal/integration"
	"github.com/jylhis/makework/internal/status"
	"github.com/jylhis/makework/internal/worktree"
	"github.com/spf13/cobra"
)

// lsEntry is the per-worktree row emitted by `mw ls`.
type lsEntry struct {
	Repo   string                `json:"repo"`
	Status status.WorktreeStatus `json:"status"`
}

func newLsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all active worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cat, err := loadState()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			var all []lsEntry
			for name, r := range cat.Repos {
				wts, err := worktree.List(r.Path)
				if err != nil {
					continue
				}
				for _, wt := range wts {
					if wt.IsBare {
						continue
					}
					st := status.GetFull(wt.Path, r.Path, r.MainBranch)
					all = append(all, lsEntry{Repo: name, Status: st})
				}
			}

			if format == "json" {
				return json.NewEncoder(out).Encode(all)
			}

			if len(all) == 0 {
				fmt.Fprintln(out, "No active worktrees.")
				return nil
			}
			writeLsTable(out, all)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table or json")
	return silenceSubcommand(cmd)
}

func writeLsTable(out io.Writer, entries []lsEntry) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPO\tBRANCH\tSTATUS\tMAIN↕\tREMOTE↕\tPATH")
	for _, e := range entries {
		branch := e.Status.Branch
		if branch == "" {
			branch = "(detached)"
		}
		sym := statusSymbols(e.Status)
		main := fmt.Sprintf("%d/%d", e.Status.MainAhead, e.Status.MainBehind)
		remote := fmt.Sprintf("%d/%d", e.Status.Ahead, e.Status.Behind)
		path := e.Status.Path
		if e.Status.IsOrphaned {
			path += " (orphaned)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Repo, branch, sym, main, remote, path)
	}
	_ = tw.Flush()
}

// statusSymbols renders a compact subset of worktrunk's symbols:
//
//	! modified   ? untracked   ✘ conflicts   ⊂ integrated   _ same-commit clean
func statusSymbols(s status.WorktreeStatus) string {
	var b []rune
	if s.Conflicts {
		b = append(b, '✘')
	}
	if s.Modified {
		b = append(b, '!')
	}
	if s.Untracked {
		b = append(b, '?')
	}
	if s.Integration == integration.StateSameCommit && s.DirtyCount == 0 {
		b = append(b, '_')
	} else if s.Integration != "" && s.Integration != integration.StateDiverged && s.Integration != integration.StateUnknown {
		b = append(b, '⊂')
	}
	if len(b) == 0 {
		return "-"
	}
	return string(b)
}
