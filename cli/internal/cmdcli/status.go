package cmdcli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/deploy"
)

func newStatusCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "status [config]",
		Short: "Show current release, last N releases, and lock state.",
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

			s, err := deploy.GetStatus(ctx, cfg, buildLogger(cmd))
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch format {
			case "json":
				return json.NewEncoder(out).Encode(s)
			default:
				return renderStatusPretty(out, s)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "pretty", "output format: pretty | json")

	return cmd
}

func renderStatusPretty(w interface{ Write([]byte) (int, error) }, s *deploy.Status) error {
	fmt.Fprintf(w, "app:           %s\n", s.App)
	fmt.Fprintf(w, "host:          %s\n", s.Host)
	fmt.Fprintf(w, "release_root:  %s\n", s.ReleaseRoot)
	if s.CurrentID == "" {
		fmt.Fprintln(w, "current:       (none — never deployed)")
	} else {
		fmt.Fprintf(w, "current:       %s  (%s)\n", s.CurrentID, s.CurrentID.FormatHuman())
	}
	if s.LockHeld {
		if s.LockInfo != nil {
			fmt.Fprintf(w, "lock:          held by %s@%s pid=%d since %s\n",
				s.LockInfo.HeldBy, s.LockInfo.Hostname, s.LockInfo.PID,
				s.LockInfo.AcquiredAt.Format(time.RFC3339))
		} else {
			fmt.Fprintln(w, "lock:          held (unknown holder)")
		}
	} else {
		fmt.Fprintln(w, "lock:          (none)")
	}
	fmt.Fprintln(w)
	if len(s.Releases) == 0 {
		fmt.Fprintln(w, "no releases on remote")
		return nil
	}
	fmt.Fprintf(w, "%-15s  %-25s  %s\n", "release", "modified", "")
	for _, r := range s.Releases {
		marker := "  "
		if r.IsCurrent {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%-13s  %-25s\n", marker, r.ID, r.ModifiedAt.UTC().Format(time.RFC3339))
	}
	return nil
}
