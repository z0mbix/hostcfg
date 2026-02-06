package resource

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

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
	name        string
	description string
	config      config.PackageResourceConfig
	dependsOn   []string
	pm          PackageManager
}

// NewPackageResource creates a new package resource from HCL
func NewPackageResource(name string, body hcl.Body, dependsOn []string, description string, ctx *hcl.EvalContext) (Resource, error) {
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
		name:        name,
		description: description,
		config:      cfg,
		dependsOn:   dependsOn,
		pm:          pm,
	}, nil
}

func (r *PackageResource) Type() string        { return "package" }
func (r *PackageResource) Name() string        { return r.name }
func (r *PackageResource) Description() string { return r.description }

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
	// Check OS first for BSD and macOS systems
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return &HomebrewPackageManager{}, nil
		}
		return nil, fmt.Errorf("no supported package manager found for macOS (Homebrew not installed)")
	case "freebsd":
		if _, err := exec.LookPath("pkg"); err == nil {
			return &PkgPackageManager{}, nil
		}
	case "openbsd":
		if _, err := exec.LookPath("pkg_add"); err == nil {
			return &OpenBSDPackageManager{}, nil
		}
	case "netbsd":
		// Prefer pkgin if available
		if _, err := exec.LookPath("pkgin"); err == nil {
			return &PkginPackageManager{}, nil
		}
		if _, err := exec.LookPath("pkg_add"); err == nil {
			return &NetBSDPackageManager{}, nil
		}
	case "illumos":
		// Prefer pkgin if available (SmartOS), fall back to IPS pkg (OmniOS)
		if _, err := exec.LookPath("pkgin"); err == nil {
			return &PkginPackageManager{}, nil
		}
		if _, err := exec.LookPath("pkg"); err == nil {
			return &IPSPackageManager{}, nil
		}
	}

	// Linux package managers
	if runtime.GOOS == "linux" {
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
	}

	return nil, fmt.Errorf("no supported package manager found for %s", runtime.GOOS)
}
