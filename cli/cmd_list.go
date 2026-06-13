package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all CP-Algorithms articles",
		Long: `List every article from the CP-Algorithms navigation page.

Each record includes the article's rank, title, category, full URL, and
site-relative path. Use --output to change the format.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(0)
			a.progressf("fetching article list...")
			articles, err := a.client.List(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
}
