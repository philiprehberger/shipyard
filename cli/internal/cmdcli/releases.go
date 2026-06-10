package cmdcli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/deploy"
)

func newReleasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "releases [config]",
		Short: "List all releases on the remote host, newest first.",
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

			ids, current, err := deploy.ListReleases(ctx, cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(ids) == 0 {
				fmt.Fprintln(out, "(no releases on remote)")
				return nil
			}
			for _, id := range ids {
				marker := "  "
				if id == current {
					marker = "* "
				}
				fmt.Fprintf(out, "%s%s  %s\n", marker, id, id.FormatHuman())
			}
			return nil
		},
	}
}
