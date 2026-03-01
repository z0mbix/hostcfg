package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/updater"
)

// NewUpdateCmd creates the update command
func NewUpdateCmd() *cobra.Command {
	var force bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update hostcfg to the latest version",
		Long:  `Downloads the latest release from GitHub and replaces the current binary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "dev" && !force {
				return fmt.Errorf("cannot update a dev build (use --force to override)")
			}

			u := updater.New(version)

			out.Info("Checking for updates...")
			result, err := u.Check()
			if err != nil {
				return fmt.Errorf("checking for updates: %w", err)
			}

			if !result.UpdateNeeded && !force {
				out.Successf("Already up to date (%s)", result.CurrentVersion)
				return nil
			}

			if result.UpdateNeeded {
				out.Infof("Update available: %s → %s", result.CurrentVersion, result.LatestVersion)
			} else {
				out.Infof("Forcing update to %s", result.LatestVersion)
			}

			if dryRun {
				out.Info("Dry run: skipping download and install")
				return nil
			}

			out.Infof("Downloading %s...", result.AssetName)
			if err := u.Update(result); err != nil {
				return fmt.Errorf("updating: %w", err)
			}

			out.Successf("Successfully updated to %s", result.LatestVersion)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force update even if already up to date")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Check for updates without installing")

	return cmd
}
