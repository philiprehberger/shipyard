package cmdcli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive config-file generator. Writes shipyard.yaml in the current directory.",
		Args:  cobra.NoArgs,
		RunE:  stub("Phase 6"),
	}
}
