package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/engine"
)

// NewValidateCmd creates the validate command
func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the configuration syntax",
		Long: `The validate command checks the HCL syntax and validates
all resource configurations without connecting to the system.

This is useful for CI/CD pipelines or pre-commit hooks.`,
		RunE: runValidate,
	}

	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
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
	executor := engine.NewExecutor(os.Stdout, !noColor)

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

	// Validate
	if err := executor.Validate(); err != nil {
		return err
	}

	fmt.Println("Configuration is valid.")
	return nil
}
