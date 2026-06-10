package cmdcli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
)

func newDoctorCmd() *cobra.Command {
	var configOnly bool

	cmd := &cobra.Command{
		Use:   "doctor [config]",
		Short: "Validate config, SSH access, and remote-host writability.",
		Long: `Lints the config, attempts an SSH connection, checks that release_root and
shared/ paths are writable. Optionally dry-runs hooks (lint only, no execution).

--config-only stops after the YAML parse and validation pass — useful in CI
where SSH access is not configured.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}

			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			cfg, err := config.Load(path)
			if err != nil {
				var verr *config.ValidationError
				if errors.As(err, &verr) {
					fmt.Fprintln(errOut, err)
					return errNotYetImplemented // exit non-zero
				}
				return fmt.Errorf("loading config: %w", err)
			}

			fmt.Fprintf(out, "config: %s — OK\n", cfg.SourcePath())
			fmt.Fprintf(out, "  app:               %s\n", cfg.App)
			fmt.Fprintf(out, "  host:              %s\n", cfg.Host.SSH)
			fmt.Fprintf(out, "  release_root:      %s\n", cfg.Host.ReleaseRoot)
			fmt.Fprintf(out, "  artifact:          %s (%s)\n", cfg.Artifact.Source, cfg.Artifact.Format)
			fmt.Fprintf(out, "  releases.keep:     %d\n", cfg.Releases.Keep)
			fmt.Fprintf(out, "  health_check.url:  %s\n", emptyDash(cfg.HealthCheck.URL))
			fmt.Fprintf(out, "  lock.enabled:      %t\n", cfg.Lock.IsEnabled())

			if configOnly {
				return nil
			}

			fmt.Fprintln(errOut, "doctor: SSH + remote-host checks land in Phase 6")
			return errNotYetImplemented
		},
	}

	cmd.Flags().BoolVar(&configOnly, "config-only", false, "validate the YAML config; skip SSH and remote checks")

	return cmd
}

func emptyDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
