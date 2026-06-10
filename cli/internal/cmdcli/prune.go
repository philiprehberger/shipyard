package cmdcli

import "github.com/spf13/cobra"

func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune [config]",
		Short: "Delete old releases. Honors releases.keep.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  stub("Phase 6"),
	}

	cmd.Flags().Int("keep", 0, "override config; keep this many releases (0 = use config value)")
	cmd.Flags().Bool("dry-run", false, "list what would be deleted, do not delete")

	return cmd
}
