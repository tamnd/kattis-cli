package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) problemsCmd() *cobra.Command {
	var sort string
	cmd := &cobra.Command{
		Use:   "problems",
		Short: "List Kattis problems",
		Long: `List problems from the Kattis Online Judge.

By default fetches one page (up to ~100 problems). Use --limit to fetch more.

Each record includes the problem's ID, name, difficulty, category, solve rate,
and URL. Use --output to change the format and --limit to cap results.

Examples:
  kattis problems
  kattis problems --sort difficulty
  kattis problems --sort -difficulty --limit 20
  kattis problems --output json
  kattis problems --fields id,name,difficulty,url`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(0)
			a.progressf("fetching problem list...")
			problems, err := a.client.Problems(cmd.Context(), sort, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(problems, len(problems))
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "name", "sort order: name, -name, difficulty, -difficulty")
	return cmd
}
