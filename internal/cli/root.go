package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/z0mbix/hostcfg/internal/engine"
)

var (
	configPath string
	variables  []string
	varFiles   []string
	noColor    bool

	// Version information (set by main)
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetVersionInfo sets the version information from build-time variables
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "hostcfg",
		Short: "An idempotent configuration management tool using HCL2",
		Long: `hostcfg is a configuration management tool that uses HCL2 syntax
to define and manage system resources in an idempotent way.

Resources include files, directories, packages, services, cron jobs,
exec commands, and hostname configuration.`,
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "",
		"Path to config file or directory (default: hostcfg.hcl or current directory)")
	rootCmd.PersistentFlags().StringArrayVarP(&variables, "var", "e", nil,
		"Set a variable (key=value)")
	rootCmd.PersistentFlags().StringArrayVarP(&varFiles, "var-file", "f", nil,
		"Path to variable file (can be used multiple times)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"Disable colored output")

	// Add subcommands
	rootCmd.AddCommand(NewPlanCmd())
	rootCmd.AddCommand(NewApplyCmd())
	rootCmd.AddCommand(NewValidateCmd())
	rootCmd.AddCommand(NewVersionCmd())

	return rootCmd
}

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("hostcfg %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	}
}

// Execute runs the CLI
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// parseVariables parses key=value variable assignments
func parseVariables(vars []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, v := range vars {
		for i := 0; i < len(v); i++ {
			if v[i] == '=' {
				key := v[:i]
				value := v[i+1:]
				if key == "" {
					return nil, fmt.Errorf("invalid variable: %s", v)
				}
				result[key] = value
				break
			}
		}
	}
	return result, nil
}

// loadVariables loads variables from files and CLI flags, applying them to the executor
// Precedence (lowest to highest):
// 1. Auto-loaded var files (hostcfg.vars.hcl, hostcfg.vars.hcl.local, *.auto.vars.hcl)
// 2. Explicit var files (--var-file, in order specified)
// 3. CLI variables (-e, highest priority)
func loadVariables(executor *engine.Executor, configDir string) error {
	loader := config.NewVarFileLoader()

	// 1. Find and load auto-load files
	autoFiles, _ := loader.FindAutoLoadFiles(configDir)

	// 2. Combine auto-load files with explicit var files
	allVarFiles := append(autoFiles, varFiles...)

	// 3. Load all variable files in order
	if len(allVarFiles) > 0 {
		vars, diags := loader.LoadMultipleVarFiles(allVarFiles)
		if diags.HasErrors() {
			return fmt.Errorf("failed to load variable files: %s", diags.Error())
		}
		for name, value := range vars {
			executor.SetVariableValue(name, value)
		}
	}

	// 4. Apply CLI variables (highest precedence)
	cliVars, err := parseVariables(variables)
	if err != nil {
		return err
	}
	for k, v := range cliVars {
		executor.SetVariable(k, v)
	}

	return nil
}
