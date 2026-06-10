package cmdcli

import (
	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/deploy"
	"github.com/philiprehberger/shipyard/cli/internal/release"
)

func newRollbackCmd() *cobra.Command {
	var toID string

	cmd := &cobra.Command{
		Use:   "rollback [config]",
		Short: "Swap the symlink back to the previous release.",
		Long:  `Atomically swap the "current" symlink back to the release that was current before the most recent deploy. Runs on_rollback hooks after the flip.`,
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

			_, err = deploy.Rollback(ctx, deploy.RollbackOptions{
				Config: cfg,
				Log:    buildLogger(cmd),
				ToID:   release.ID(toID),
			})
			return err
		},
	}

	cmd.Flags().StringVar(&toID, "to", "", "roll back to a specific release timestamp (default: previous)")

	return cmd
}
