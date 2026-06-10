package cmdcli

import "github.com/spf13/cobra"

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [config]",
		Short: "Validate config, SSH access, and remote-host writability.",
		Long:  `Lints the config, attempts an SSH connection, checks that release_root and shared/ paths are writable. Optionally dry-runs hooks (lint only, no execution).`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  stub("Phase 6"),
	}

	cmd.Flags().Bool("config-only", false, "validate the YAML config; skip SSH and remote checks")

	return cmd
}
