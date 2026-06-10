package cmdcli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/deploy"
	"github.com/philiprehberger/shipyard/cli/internal/logger"
	"github.com/philiprehberger/shipyard/cli/internal/release"
)

func newDeployCmd() *cobra.Command {
	var (
		dryRun     bool
		skipHealth bool
		force      bool
		releaseID  string
	)

	cmd := &cobra.Command{
		Use:   "deploy [config]",
		Short: "Run a deploy.",
		Long: `Run a deploy against the host described in [config] (default: shipyard.yaml in the current directory).

Pipeline: parse config → run pre_upload hooks → SSH connect → acquire remote
lock → upload artifact → extract into releases/<timestamp>/ → symlink shared
files → run post_extract hooks → atomic symlink flip → run post_flip hooks →
run health check (rollback on failure) → auto-prune → release lock.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}

			cfg, err := config.Load(path)
			if err != nil {
				return &deploy.CodedError{Code: deploy.ExitUsage, Wrapped: err}
			}

			log := buildLogger(cmd)

			ctx, cancel := signalContext()
			defer cancel()

			opts := deploy.Options{
				Config:            cfg,
				Log:               log,
				DryRun:            dryRun,
				SkipHealth:        skipHealth,
				ForceLockSteal:    force,
				ReleaseIDOverride: releaseID,
			}
			_ = release.ID("") // (silence unused import if we lose the override path)

			_, err = deploy.Run(ctx, opts)
			return err
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen, stop after SSH connect")
	cmd.Flags().BoolVar(&skipHealth, "skip-health", false, "flip without running the health check (don't)")
	cmd.Flags().StringVar(&releaseID, "release-id", "", "override the auto-generated release timestamp (YYYYMMDDhhmmss)")
	cmd.Flags().BoolVar(&force, "force", false, "ignore an active remote lock and steal it")

	return cmd
}

// buildLogger reads --verbose + --no-color from the root command and
// constructs the deploy logger.
func buildLogger(cmd *cobra.Command) *logger.Logger {
	verbose, _ := cmd.Root().PersistentFlags().GetBool("verbose")
	noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")
	if _, set := os.LookupEnv("NO_COLOR"); set {
		noColor = true
	}
	return logger.NewLogger(logger.Options{
		Color:   !noColor,
		Verbose: verbose,
	})
}

// signalContext returns a context that's canceled on Ctrl-C / SIGTERM.
// The deploy package treats cancellation as "abort, run rollback if past
// the flip, release lock."
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case <-ch:
			cancel()
		}
	}()
	return ctx, cancel
}
