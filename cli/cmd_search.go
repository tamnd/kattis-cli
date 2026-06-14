package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search Kattis problems by name",
		Long: `Search Kattis problems whose name or ID contains <query>
(case-insensitive substring match).

Examples:
  kattis search sorting
  kattis search "hello world"
  kattis search graph --output json
  kattis search tree --limit 5`,
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
