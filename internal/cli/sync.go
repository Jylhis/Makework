package cli

import (
	"fmt"
	"strings"

	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/fsx"
	"github.com/jylhis/makework/internal/xdgpath"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		depth   uint32
		exclude []string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Discover and register repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cat, err := loadState()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			scanRoots := cfg.ScanRoots
			if len(scanRoots) == 0 {
				dev, err := xdgpath.ExpandTilde("~/Developer")
				if err == nil {
					if fsx.PathExists(dev) {
						scanRoots = []string{dev}
					}
				}
			}
			if len(scanRoots) == 0 {
				return fmt.Errorf("no scan roots configured. Set with: mw config set scan_roots ~/Developer")
			}

			maxDepth := depth
			if maxDepth == 0 {
				if cfg.SyncMaxDepth != nil {
					maxDepth = *cfg.SyncMaxDepth
				} else {
					maxDepth = 1
				}
			}
			mergedExclude := append([]string{}, cfg.SyncExclude...)
			for _, pat := range exclude {
				found := false
				for _, existing := range mergedExclude {
					if existing == pat {
						found = true
						break
					}
				}
				if !found {
					mergedExclude = append(mergedExclude, pat)
				}
			}

			fmt.Fprintf(out, "Scanning: %s\n", strings.Join(scanRoots, ", "))
			added, err := cat.Sync(cfg, scanRoots, catalog.SyncOptions{
				MaxDepth: maxDepth,
				Exclude:  mergedExclude,
			})
			if err != nil {
				return err
			}
			if len(added) == 0 {
				fmt.Fprintln(out, "No new repositories found.")
			} else {
				fmt.Fprintf(out, "Registered %d new repo(s):\n", len(added))
				for _, name := range added {
					fmt.Fprintf(out, "  %s\n", name)
				}
			}
			writeRepoRootsCache(cat)
			return nil
		},
	}
	cmd.Flags().Uint32Var(&depth, "depth", 0, "Maximum directory depth to scan (overrides config)")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Directory name patterns to skip (repeatable, merged with config)")
	return silenceSubcommand(cmd)
}
