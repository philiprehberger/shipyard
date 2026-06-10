package cmdcli

import "github.com/spf13/cobra"

func newRollbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback [config]",
		Short: "Swap the symlink back to the previous release.",
		Long:  `Atomically swap the "current" symlink back to the release that was current before the most recent deploy. Runs on_rollback hooks after the flip.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  stub("Phase 6"),
	}

	cmd.Flags().String("to", "", "roll back to a specific release timestamp (default: previous)")

	return cmd
}
