package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/engine"
)

var (
	dryRun      bool
	autoApprove bool
)

// NewApplyCmd creates the apply command
func NewApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply changes to bring the system to desired state",
		Long: `The apply command reads the configuration and makes the necessary
changes to bring the system to the desired state.

By default, it will show the plan and ask for confirmation before
applying changes. Use -y/--yes or --auto-approve to skip confirmation.`,
		RunE: runApply,
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be done without making changes")
	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false,
		"Skip interactive approval before applying")
	cmd.Flags().Bool("auto-approve", false,
		"Skip interactive approval before applying (alias for --yes)")
	// Make --auto-approve an alias for --yes
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if aa, _ := cmd.Flags().GetBool("auto-approve"); aa {
			autoApprove = true
		}
		return nil
	}

	return cmd
}

func runApply(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Find config
	path, isDir, err := engine.FindConfigFile(configPath)
	if err != nil {
		return err
	}

	// Create executor
	useColors := !noColor && isTerminal()
	executor := engine.NewExecutor(os.Stdout, useColors)

	// Set variables
	vars, err := parseVariables(variables)
	if err != nil {
		return err
	}
	for k, v := range vars {
		executor.SetVariable(k, v)
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

	// Check if there are changes
	if !result.HasChanges() {
		return nil
	}

	// Dry run stops here
	if dryRun {
		return nil
	}

	// Ask for confirmation unless auto-approve
	if !autoApprove {
		fmt.Print("\nDo you want to apply these changes? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			fmt.Println("Apply cancelled.")
			return nil
		}
	}

	fmt.Println()

	// Apply changes
	if err := executor.Apply(ctx, result, false); err != nil {
		return err
	}

	fmt.Printf("\nApply complete! Resources: %d added, %d changed, %d destroyed.\n",
		result.ToAdd, result.ToChange, result.ToDestroy)

	return nil
}
