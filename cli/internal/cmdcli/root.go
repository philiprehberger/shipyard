package cmdcli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/philiprehberger/shipyard/cli/internal/deploy"
	"github.com/philiprehberger/shipyard/cli/internal/version"
)

const longDescription = `Shipyard runs zero-downtime SSH deploys with health-gated promotion and automatic rollback.

A typical session:

  shipyard init                # interactive — writes shipyard.yaml
  shipyard doctor              # validates config + SSH + remote writability
  shipyard deploy              # ships, health-checks, rolls back on failure

Exit codes:
  0  success
  1  usage or config error
  2  SSH / transport error
  3  deploy step error (rolled back)
  4  health-check failure (rolled back)
  5  lock held by another process

Docs: https://shipyard.philiprehberger.com
Source: https://github.com/philiprehberger/shipyard`

// NewRootCmd builds the root cobra command and wires every subcommand.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "shipyard",
		Short:         "Atomic-release deploy CLI with health-gated promotion and automatic rollback.",
		Long:          longDescription,
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().Bool("no-color", false, "disable colored output (also honors NO_COLOR env var)")
	root.PersistentFlags().Bool("verbose", false, "verbose logging — emits JSON to stderr in addition to pretty stdout")

	root.AddCommand(
		newDeployCmd(),
		newRollbackCmd(),
		newStatusCmd(),
		newReleasesCmd(),
		newPruneCmd(),
		newInitCmd(),
		newDoctorCmd(),
		newVersionCmd(),
	)

	return root
}

// Execute is the entrypoint called from main. Returns the exit code.
func Execute(stdout, stderr io.Writer, args []string) int {
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "shipyard:", err)
		return exitCodeFor(err)
	}
	return 0
}

// exitCodeFor maps known error sentinels to documented exit codes.
func exitCodeFor(err error) int {
	var coded *deploy.CodedError
	if errors.As(err, &coded) {
		return int(coded.Code)
	}
	return 1
}

// Sentinel for "not yet implemented" stubs. Phase 6 will retire it once
// init / doctor SSH checks land.
var errNotYetImplemented = fmt.Errorf("not yet implemented in this build")

// stub returns a runE that prints which phase will deliver this subcommand,
// then exits 1. Used during scaffold; replaced as we land each feature.
func stub(phase string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s (lands in %s)\n", cmd.Name(), errNotYetImplemented, phase)
		return errNotYetImplemented
	}
}
