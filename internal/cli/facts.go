package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/z0mbix/hostcfg/internal/facts"
	"gopkg.in/yaml.v3"
)

var factsFormat string
var noEnv bool

// NewFactsCmd creates the facts command
func NewFactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "facts",
		Short: "Display gathered system facts",
		Long: `The facts command displays all system facts that are available
for use in HCL expressions via the "fact" namespace.

Facts include OS information, architecture, hostname, user details,
CPU information, machine ID, and environment variables.`,
		RunE: runFacts,
	}

	cmd.Flags().StringVar(&factsFormat, "format", "hcl",
		"Output format: hcl, json, or yaml")
	cmd.Flags().BoolVar(&noEnv, "no-env", false,
		"Exclude environment variables from output")

	return cmd
}

func runFacts(cmd *cobra.Command, args []string) error {
	// Gather facts
	f, err := facts.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather facts: %w", err)
	}

	if noEnv {
		f.Env = nil
	}

	// Output in requested format
	switch strings.ToLower(factsFormat) {
	case "json":
		return outputJSON(f)
	case "yaml", "yml":
		return outputYAML(f)
	case "hcl":
		return outputHCL(f)
	default:
		return fmt.Errorf("unsupported format: %s (supported: hcl, json, yaml)", factsFormat)
	}
}

// factsOutput is a structured representation of facts for serialization
type factsOutput struct {
	OS              osOutput          `json:"os" yaml:"os"`
	Arch            string            `json:"arch" yaml:"arch"`
	Hostname        string            `json:"hostname" yaml:"hostname"`
	FQDN            string            `json:"fqdn" yaml:"fqdn"`
	MachineID       string            `json:"machine_id" yaml:"machine_id"`
	PackageManagers []string          `json:"package_managers" yaml:"package_managers"`
	CPU             cpuOutput         `json:"cpu" yaml:"cpu"`
	User            userOutput        `json:"user" yaml:"user"`
	Env             map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

type osOutput struct {
	Name                string `json:"name" yaml:"name"`
	Family              string `json:"family" yaml:"family"`
	Distribution        string `json:"distribution" yaml:"distribution"`
	DistributionVersion string `json:"distribution_version" yaml:"distribution_version"`
}

type cpuOutput struct {
	Physical int `json:"physical" yaml:"physical"`
	Cores    int `json:"cores" yaml:"cores"`
}

type userOutput struct {
	Name string `json:"name" yaml:"name"`
	Home string `json:"home" yaml:"home"`
	UID  string `json:"uid" yaml:"uid"`
	GID  string `json:"gid" yaml:"gid"`
}

func toFactsOutput(f *facts.Facts) factsOutput {
	return factsOutput{
		OS: osOutput{
			Name:                f.OS.Name,
			Family:              f.OS.Family,
			Distribution:        f.OS.Distribution,
			DistributionVersion: f.OS.DistributionVersion,
		},
		Arch:            f.Arch,
		Hostname:        f.Hostname,
		FQDN:            f.FQDN,
		MachineID:       f.MachineID,
		PackageManagers: f.PackageManagers,
		CPU: cpuOutput{
			Physical: f.CPU.Physical,
			Cores:    f.CPU.Cores,
		},
		User: userOutput{
			Name: f.User.Name,
			Home: f.User.Home,
			UID:  f.User.UID,
			GID:  f.User.GID,
		},
		Env: f.Env,
	}
}

func outputJSON(f *facts.Facts) error {
	out := toFactsOutput(f)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputYAML(f *facts.Facts) error {
	out := toFactsOutput(f)
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(out); err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Print(buf.String())
	return nil
}

func outputHCL(f *facts.Facts) error {
	var sb strings.Builder

	sb.WriteString("os = {\n")
	sb.WriteString(fmt.Sprintf("  name                 = %q\n", f.OS.Name))
	sb.WriteString(fmt.Sprintf("  family               = %q\n", f.OS.Family))
	sb.WriteString(fmt.Sprintf("  distribution         = %q\n", f.OS.Distribution))
	sb.WriteString(fmt.Sprintf("  distribution_version = %q\n", f.OS.DistributionVersion))
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("arch       = %q\n", f.Arch))
	sb.WriteString(fmt.Sprintf("hostname   = %q\n", f.Hostname))
	sb.WriteString(fmt.Sprintf("fqdn       = %q\n", f.FQDN))
	sb.WriteString(fmt.Sprintf("machine_id = %q\n", f.MachineID))

	// Format package_managers as HCL list
	if len(f.PackageManagers) == 0 {
		sb.WriteString("package_managers = []\n\n")
	} else {
		quoted := make([]string, len(f.PackageManagers))
		for i, pm := range f.PackageManagers {
			quoted[i] = fmt.Sprintf("%q", pm)
		}
		sb.WriteString(fmt.Sprintf("package_managers = [%s]\n\n", strings.Join(quoted, ", ")))
	}

	sb.WriteString("cpu = {\n")
	sb.WriteString(fmt.Sprintf("  physical = %d\n", f.CPU.Physical))
	sb.WriteString(fmt.Sprintf("  cores    = %d\n", f.CPU.Cores))
	sb.WriteString("}\n\n")

	sb.WriteString("user = {\n")
	sb.WriteString(fmt.Sprintf("  name = %q\n", f.User.Name))
	sb.WriteString(fmt.Sprintf("  home = %q\n", f.User.Home))
	sb.WriteString(fmt.Sprintf("  uid  = %q\n", f.User.UID))
	sb.WriteString(fmt.Sprintf("  gid  = %q\n", f.User.GID))
	sb.WriteString("}\n")

	if len(f.Env) > 0 {
		sb.WriteString("\n")

		// Sort env keys for consistent output
		keys := make([]string, 0, len(f.Env))
		for k := range f.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		sb.WriteString("env = {\n")
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("  %q = %q\n", k, f.Env[k]))
		}
		sb.WriteString("}\n")
	}

	fmt.Print(sb.String())
	return nil
}
