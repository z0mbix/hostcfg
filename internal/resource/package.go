package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("package", NewPackageResource)
}

// PackageManager represents a system package manager
type PackageManager interface {
	IsInstalled(ctx context.Context, name string) (bool, string, error)
	Install(ctx context.Context, name, version string) error
	Remove(ctx context.Context, name string) error
	Name() string
}

// PackageResource manages system packages
type PackageResource struct {
	name      string
	config    config.PackageResourceConfig
	dependsOn []string
	pm        PackageManager
}

// NewPackageResource creates a new package resource from HCL
func NewPackageResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.PackageResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode package resource: %s", diags.Error())
	}

	pm, err := detectPackageManager()
	if err != nil {
		return nil, err
	}

	return &PackageResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
		pm:        pm,
	}, nil
}

func (r *PackageResource) Type() string { return "package" }
func (r *PackageResource) Name() string { return r.name }

func (r *PackageResource) Validate() error {
	if r.config.Name == "" {
		return fmt.Errorf("package.%s: name is required", r.name)
	}
	return nil
}

func (r *PackageResource) Dependencies() []string {
	return r.dependsOn
}

func (r *PackageResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	installed, version, err := r.pm.IsInstalled(ctx, r.config.Name)
	if err != nil {
		return nil, err
	}

	state.Exists = installed
	if installed {
		state.Attributes["name"] = r.config.Name
		state.Attributes["version"] = version
		state.Attributes["ensure"] = "present"
	}

	return state, nil
}

func (r *PackageResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	ensure := "present"
	if r.config.Ensure != nil {
		ensure = *r.config.Ensure
	}

	// Handle ensure = absent
	if ensure == "absent" {
		if current.Exists {
			plan.Action = ActionDelete
			plan.Changes = append(plan.Changes, Change{
				Attribute: "ensure",
				Old:       "present",
				New:       "absent",
			})
		}
		return plan, nil
	}

	// Package should be present
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.Changes = append(plan.Changes, Change{
			Attribute: "name",
			Old:       nil,
			New:       r.config.Name,
		})
		plan.Changes = append(plan.Changes, Change{
			Attribute: "ensure",
			Old:       "absent",
			New:       "present",
		})
		if r.config.Version != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "version",
				Old:       nil,
				New:       *r.config.Version,
			})
		}
		return plan, nil
	}

	// Package exists - check version if specified
	if r.config.Version != nil {
		currentVersion, _ := current.Attributes["version"].(string)
		if currentVersion != *r.config.Version {
			plan.Action = ActionUpdate
			plan.After.Exists = true
			plan.Changes = append(plan.Changes, Change{
				Attribute: "version",
				Old:       currentVersion,
				New:       *r.config.Version,
			})
		}
	}

	return plan, nil
}

func (r *PackageResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		return r.pm.Remove(ctx, r.config.Name)

	case ActionCreate, ActionUpdate:
		version := ""
		if r.config.Version != nil {
			version = *r.config.Version
		}
		return r.pm.Install(ctx, r.config.Name, version)
	}

	return nil
}

// detectPackageManager detects and returns the appropriate package manager
func detectPackageManager() (PackageManager, error) {
	// Try apt first (Debian/Ubuntu)
	if _, err := exec.LookPath("apt-get"); err == nil {
		return &AptPackageManager{}, nil
	}

	// Try dnf (Fedora/RHEL 8+)
	if _, err := exec.LookPath("dnf"); err == nil {
		return &DnfPackageManager{}, nil
	}

	// Try yum (RHEL 7/CentOS)
	if _, err := exec.LookPath("yum"); err == nil {
		return &YumPackageManager{}, nil
	}

	// Try pacman (Arch Linux)
	if _, err := exec.LookPath("pacman"); err == nil {
		return &PacmanPackageManager{}, nil
	}

	return nil, fmt.Errorf("no supported package manager found")
}

// AptPackageManager implements PackageManager for apt
type AptPackageManager struct{}

func (m *AptPackageManager) Name() string { return "apt" }

func (m *AptPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status} ${Version}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "install ok installed") {
		parts := strings.Fields(outputStr)
		if len(parts) >= 4 {
			return true, parts[3], nil
		}
		return true, "", nil
	}

	return false, "", nil
}

func (m *AptPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s=%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *AptPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "apt-get", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// DnfPackageManager implements PackageManager for dnf
type DnfPackageManager struct{}

func (m *DnfPackageManager) Name() string { return "dnf" }

func (m *DnfPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "rpm", "-q", "--queryformat", "%{VERSION}-%{RELEASE}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	return true, strings.TrimSpace(string(output)), nil
}

func (m *DnfPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "dnf", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dnf install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *DnfPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "dnf", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dnf remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// YumPackageManager implements PackageManager for yum
type YumPackageManager struct{}

func (m *YumPackageManager) Name() string { return "yum" }

func (m *YumPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "rpm", "-q", "--queryformat", "%{VERSION}-%{RELEASE}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	return true, strings.TrimSpace(string(output)), nil
}

func (m *YumPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yum install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *YumPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "yum", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yum remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// PacmanPackageManager implements PackageManager for pacman
type PacmanPackageManager struct{}

func (m *PacmanPackageManager) Name() string { return "pacman" }

func (m *PacmanPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "pacman", "-Q", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return true, parts[1], nil
	}
	return true, "", nil
}

func (m *PacmanPackageManager) Install(ctx context.Context, name, version string) error {
	cmd := exec.CommandContext(ctx, "pacman", "-S", "--noconfirm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacman install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *PacmanPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pacman", "-R", "--noconfirm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacman remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
