package cmdcli

import "github.com/spf13/cobra"

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [config]",
		Short: "Run a deploy.",
		Long: `Run a deploy against the host described in [config] (default: shipyard.yaml in the current directory).

Pipeline: parse config → run pre_upload hooks → SSH connect → acquire remote lock → SFTP upload artifact → extract into releases/<timestamp>/ → symlink shared files → run post_extract hooks → atomic symlink flip → run post_flip hooks → run health check (rollback on failure) → auto-prune → release lock.`,
		Args: cobra.MaximumNArgs(1),
		RunE: stub("Phase 4"),
	}

	cmd.Flags().Bool("dry-run", false, "show what would happen, do not transfer")
	cmd.Flags().Bool("skip-health", false, "flip without running the health check (don't)")
	cmd.Flags().String("release-id", "", "override the auto-generated release timestamp")
	cmd.Flags().Bool("force", false, "ignore an active lock and steal it")

	return cmd
}
