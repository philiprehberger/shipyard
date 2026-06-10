package cmdcli

import "github.com/spf13/cobra"

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [config]",
		Short: "Show current release, last N releases, and lock state.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  stub("Phase 6"),
	}

	cmd.Flags().String("format", "pretty", "output format: pretty | json")

	return cmd
}
