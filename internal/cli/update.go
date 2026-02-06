package cli

import (
	"fmt"
	"os"

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

			fmt.Fprintf(os.Stderr, "Checking for updates...\n")
			result, err := u.Check()
			if err != nil {
				return fmt.Errorf("checking for updates: %w", err)
			}

			if !result.UpdateNeeded && !force {
				fmt.Fprintf(os.Stderr, "Already up to date (%s)\n", result.CurrentVersion)
				return nil
			}

			if result.UpdateNeeded {
				fmt.Fprintf(os.Stderr, "Update available: %s → %s\n", result.CurrentVersion, result.LatestVersion)
			} else {
				fmt.Fprintf(os.Stderr, "Forcing update to %s\n", result.LatestVersion)
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run: skipping download and install\n")
				return nil
			}

			fmt.Fprintf(os.Stderr, "Downloading %s...\n", result.AssetName)
			if err := u.Update(result); err != nil {
				return fmt.Errorf("updating: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully updated to %s\n", result.LatestVersion)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force update even if already up to date")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Check for updates without installing")

	return cmd
}
