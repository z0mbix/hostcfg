package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string
	variables  []string
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
