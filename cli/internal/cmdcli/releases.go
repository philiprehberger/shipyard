package cmdcli

import "github.com/spf13/cobra"

func newReleasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "releases [config]",
		Short: "List all releases on the remote host with their timestamps.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  stub("Phase 6"),
	}
}
