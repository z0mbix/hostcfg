package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/engine"
)

// NewPlanCmd creates the plan command
func NewPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what changes would be made",
		Long: `The plan command reads the configuration and shows what changes
would be made to bring the system to the desired state. No changes
are actually applied.

This is equivalent to 'hostcfg apply --dry-run'.`,
		RunE: runPlan,
	}

	return cmd
}

func runPlan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Find config
	path, isDir, err := engine.FindConfigFile(configPath)
	if err != nil {
		return err
	}

	// Determine config directory for auto-loading var files
	configDir := path
	if !isDir {
		configDir = filepath.Dir(path)
	}

	// Create executor
	useColors := !noColor && isTerminal()
	executor := engine.NewExecutor(os.Stdout, useColors)

	// Load variables (auto-load files, --var-file, -e)
	if err := loadVariables(executor, configDir); err != nil {
		return err
	}

	// Load config
	if isDir {
		if err := executor.LoadDirectory(path); err != nil {
			return err
		}
	} else {
		if err := executor.LoadFile(path); err != nil {
			return err
		}
	}

	// Generate plan
	result, err := executor.Plan(ctx)
	if err != nil {
		return err
	}

	// Print plan
	executor.PrintPlan(result)

	return nil
}

func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
