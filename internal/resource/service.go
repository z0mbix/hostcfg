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
	Register("service", NewServiceResource)
}

// ServiceManager represents a system service manager (init system)
type ServiceManager interface {
	// Name returns the name of the service manager
	Name() string
	// Exists checks if the service exists on the system
	Exists(ctx context.Context, name string) (bool, error)
	// IsRunning checks if the service is currently running
	IsRunning(ctx context.Context, name string) (bool, error)
	// IsEnabled checks if the service is enabled at boot
	IsEnabled(ctx context.Context, name string) (bool, error)
	// Start starts the service
	Start(ctx context.Context, name string) error
	// Stop stops the service
	Stop(ctx context.Context, name string) error
	// Enable enables the service at boot
	Enable(ctx context.Context, name string) error
	// Disable disables the service at boot
	Disable(ctx context.Context, name string) error
}

// ServiceResource manages system services
type ServiceResource struct {
	name        string
	description string
	config      config.ServiceResourceConfig
	dependsOn   []string
	sm          ServiceManager
}

// NewServiceResource creates a new service resource from HCL
func NewServiceResource(name string, body hcl.Body, dependsOn []string, description string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.ServiceResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode service resource: %s", diags.Error())
	}

	sm, err := detectServiceManager()
	if err != nil {
		return nil, err
	}

	return &ServiceResource{
		name:        name,
		description: description,
		config:      cfg,
		dependsOn:   dependsOn,
		sm:          sm,
	}, nil
}

func (r *ServiceResource) Type() string        { return "service" }
func (r *ServiceResource) Name() string        { return r.name }
func (r *ServiceResource) Description() string { return r.description }

func (r *ServiceResource) Validate() error {
	if r.config.Name == "" {
		return fmt.Errorf("service.%s: name is required", r.name)
	}
	if r.config.Ensure != nil {
		ensure := *r.config.Ensure
		if ensure != "running" && ensure != "stopped" {
			return fmt.Errorf("service.%s: ensure must be 'running' or 'stopped'", r.name)
		}
	}
	return nil
}

func (r *ServiceResource) Dependencies() []string {
	return r.dependsOn
}

func (r *ServiceResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	exists, err := r.sm.Exists(ctx, r.config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if service exists: %w", err)
	}

	if !exists {
		state.Exists = false
		return state, nil
	}

	state.Exists = true
	state.Attributes["name"] = r.config.Name

	isRunning, err := r.sm.IsRunning(ctx, r.config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if service is running: %w", err)
	}
	if isRunning {
		state.Attributes["ensure"] = "running"
	} else {
		state.Attributes["ensure"] = "stopped"
	}

	isEnabled, err := r.sm.IsEnabled(ctx, r.config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if service is enabled: %w", err)
	}
	state.Attributes["enabled"] = isEnabled

	return state, nil
}

func (r *ServiceResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	// If service doesn't exist yet (e.g., package not installed), plan to configure it
	// once it becomes available (after dependencies are applied)
	if !current.Exists {
		// Check if we have any desired state to apply
		hasDesiredState := r.config.Ensure != nil || r.config.Enabled != nil
		if !hasDesiredState {
			// No desired state specified, nothing to do
			return plan, nil
		}

		// Plan to configure the service once it exists
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.After.Attributes["name"] = r.config.Name

		if r.config.Ensure != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "ensure",
				Old:       "(service not yet installed)",
				New:       *r.config.Ensure,
			})
		}
		if r.config.Enabled != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "enabled",
				Old:       "(service not yet installed)",
				New:       *r.config.Enabled,
			})
		}
		return plan, nil
	}

	plan.After.Exists = true
	plan.After.Attributes = make(map[string]interface{})
	for k, v := range current.Attributes {
		plan.After.Attributes[k] = v
	}

	// Check ensure (running/stopped)
	if r.config.Ensure != nil {
		currentEnsure, _ := current.Attributes["ensure"].(string)
		if currentEnsure != *r.config.Ensure {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "ensure",
				Old:       currentEnsure,
				New:       *r.config.Ensure,
			})
		}
	}

	// Check enabled
	if r.config.Enabled != nil {
		currentEnabled, _ := current.Attributes["enabled"].(bool)
		if currentEnabled != *r.config.Enabled {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "enabled",
				Old:       currentEnabled,
				New:       *r.config.Enabled,
			})
		}
	}

	if len(plan.Changes) > 0 {
		plan.Action = ActionUpdate
	}

	return plan, nil
}

func (r *ServiceResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	// Re-check if service exists now (dependencies may have installed it)
	exists, err := r.sm.Exists(ctx, r.config.Name)
	if err != nil {
		return fmt.Errorf("failed to check if service exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("service %s does not exist (is the package installed?)", r.config.Name)
	}

	for _, change := range plan.Changes {
		switch change.Attribute {
		case "ensure":
			newEnsure := change.New.(string)
			if newEnsure == "running" {
				if err := r.sm.Start(ctx, r.config.Name); err != nil {
					return fmt.Errorf("failed to start service: %w", err)
				}
			} else {
				if err := r.sm.Stop(ctx, r.config.Name); err != nil {
					return fmt.Errorf("failed to stop service: %w", err)
				}
			}

		case "enabled":
			newEnabled := change.New.(bool)
			if newEnabled {
				if err := r.sm.Enable(ctx, r.config.Name); err != nil {
					return fmt.Errorf("failed to enable service: %w", err)
				}
			} else {
				if err := r.sm.Disable(ctx, r.config.Name); err != nil {
					return fmt.Errorf("failed to disable service: %w", err)
				}
			}
		}
	}

	return nil
}

// detectServiceManager detects and returns the appropriate service manager
func detectServiceManager() (ServiceManager, error) {
	switch runtime.GOOS {
	case "darwin":
		return &LaunchctlServiceManager{}, nil
	case "freebsd":
		return &FreeBSDServiceManager{}, nil
	case "openbsd":
		return &OpenBSDServiceManager{}, nil
	case "netbsd":
		return &NetBSDServiceManager{}, nil
	case "illumos":
		if _, err := exec.LookPath("svcadm"); err == nil {
			return &SMFServiceManager{}, nil
		}
		return nil, fmt.Errorf("no supported service manager found for illumos (SMF not available)")
	case "linux":
		// Check for systemd
		if _, err := exec.LookPath("systemctl"); err == nil {
			return &SystemdServiceManager{}, nil
		}
		return nil, fmt.Errorf("no supported service manager found (systemd not available)")
	default:
		return nil, fmt.Errorf("no supported service manager found for %s", runtime.GOOS)
	}
}
