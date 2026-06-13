package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search CP-Algorithms articles by title or category",
		Long: `Search CP-Algorithms articles whose title or category contains <query>
(case-insensitive substring match).

Examples:
  cpa search sorting
  cpa search "shortest path"
  cpa search graph --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(0)
			q := args[0]
			a.progressf("searching for %q...", q)
			hits, err := a.client.Search(cmd.Context(), q, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(hits, len(hits))
		},
	}
}
