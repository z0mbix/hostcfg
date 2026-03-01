package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/engine"
)

// NewVerifyCmd creates the verify command
func NewVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify that resources match their desired state",
		Long: `The verify command checks that the current system state matches
the desired state defined in the configuration. This is useful for:

- Testing that roles configured the host correctly
- Detecting configuration drift
- Continuous compliance monitoring
- Post-apply validation

Unlike 'plan' which shows what would change, 'verify' confirms
resources are already in their desired state.

Exit codes:
  0 - All resources verified successfully
  1 - One or more resources have drifted from desired state
  2 - Error during verification`,
		RunE: runVerify,
	}

	return cmd
}

func runVerify(cmd *cobra.Command, args []string) error {
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

	// Parse default timeout
	timeout, err := time.ParseDuration(defaultTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", defaultTimeout, err)
	}

	// Create executor
	executor := engine.NewExecutor(out, verbose, timeout)

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

	// Run verification
	result, err := executor.Verify(ctx)
	if err != nil {
		return err
	}

	// Print results
	executor.PrintVerifyResult(result)

	// Exit with appropriate code
	if result.HasDrift() {
		os.Exit(1)
	}

	return nil
}
