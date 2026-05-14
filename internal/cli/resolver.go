package cli

import (
	"fmt"

	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/resolver"
	"github.com/spf13/cobra"
)

func newResolverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolver",
		Short: "Resolver debugging tools",
	}
	cmd.AddCommand(
		silenceSubcommand(&cobra.Command{
			Use:   "explain <query>",
			Short: "Show resolver scoring breakdown for a query",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, cat, err := loadState()
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				query := args[0]

				resolverCfg := config.DefaultResolver()
				if cfg.Resolver != nil {
					resolverCfg = *cfg.Resolver
				}
				index := &resolver.Index{
					Targets: resolver.BuildTargets(cat),
					Visits:  loadVisits(),
				}
				ctx := resolver.DefaultContext()
				results, err := resolver.Resolve(query, index, &resolverCfg, &ctx)
				if err != nil {
					return err
				}

				fmt.Fprintf(out, "Query: '%s'\n", query)
				fmt.Fprintf(out, "Weights: fuzzy=%.2f frecency=%.2f activity=%.2f context=%.2f\n",
					resolverCfg.WeightFuzzy, resolverCfg.WeightFrecency,
					resolverCfg.WeightActivity, resolverCfg.WeightContext)
				fmt.Fprintln(out)
				fmt.Fprintf(out, "%-20s %-12s %6s %6s %6s %6s %8s\n",
					"REPO", "BRANCH", "FUZZY", "FREC", "ACT", "CTX", "TOTAL")
				for i, t := range results {
					if i >= 10 {
						break
					}
					branch := t.Branch
					if branch == "" {
						branch = "-"
					}
					fmt.Fprintf(out, "%-20s %-12s %6.3f %6.3f %6.3f %6.3f %8.3f\n",
						t.RepoName, branch,
						t.SignalScores.Fuzzy, t.SignalScores.Frecency,
						t.SignalScores.Activity, t.SignalScores.Context,
						t.Score)
				}
				if resolver.NeedsDisambiguation(results, 0.10) {
					fmt.Fprintln(out, "\nDisambiguation: TRIGGERED (top scores within 10%)")
				}
				return nil
			},
		}),
	)
	return silenceSubcommand(cmd)
}
