package cmdcli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/deploy"
)

func newPruneCmd() *cobra.Command {
	var (
		keep   int
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "prune [config]",
		Short: "Delete old releases. Honors releases.keep.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			cfg, err := config.Load(path)
			if err != nil {
				return &deploy.CodedError{Code: deploy.ExitUsage, Wrapped: err}
			}

			ctx, cancel := signalContext()
			defer cancel()

			res, err := deploy.Prune(ctx, deploy.PruneOptions{
				Config: cfg,
				Log:    buildLogger(cmd),
				Keep:   keep,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if dryRun {
				fmt.Fprintf(out, "dry-run: would delete %d release(s); keep %d\n", len(res.Deleted), len(res.Kept))
			} else {
				fmt.Fprintf(out, "deleted %d release(s); kept %d\n", len(res.Deleted), len(res.Kept))
			}
			for _, id := range res.Deleted {
				fmt.Fprintf(out, "  - %s\n", id)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&keep, "keep", 0, "override config; keep this many releases (0 = use config value)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be deleted, do not delete")

	return cmd
}
