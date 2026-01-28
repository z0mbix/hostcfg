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
	Register("service", NewServiceResource)
}

// ServiceResource manages systemd services
type ServiceResource struct {
	name      string
	config    config.ServiceResourceConfig
	dependsOn []string
}

// NewServiceResource creates a new service resource from HCL
func NewServiceResource(name string, body hcl.Body, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.ServiceResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode service resource: %s", diags.Error())
	}

	// Extract depends_on from body
	var wrapper struct {
		DependsOn []string `hcl:"depends_on,optional"`
	}
	gohcl.DecodeBody(body, ctx, &wrapper)

	return &ServiceResource{
		name:      name,
		config:    cfg,
		dependsOn: wrapper.DependsOn,
	}, nil
}

func (r *ServiceResource) Type() string { return "service" }
func (r *ServiceResource) Name() string { return r.name }

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

	// Check if service exists
	cmd := exec.CommandContext(ctx, "systemctl", "list-unit-files", r.config.Name+".service")
	output, err := cmd.Output()
	if err != nil {
		// Service doesn't exist
		state.Exists = false
		return state, nil
	}

	// Check if the service is actually listed (not just headers)
	if !strings.Contains(string(output), r.config.Name) {
		state.Exists = false
		return state, nil
	}

	state.Exists = true
	state.Attributes["name"] = r.config.Name

	// Check if running
	cmd = exec.CommandContext(ctx, "systemctl", "is-active", r.config.Name)
	output, _ = cmd.Output()
	isActive := strings.TrimSpace(string(output)) == "active"
	if isActive {
		state.Attributes["ensure"] = "running"
	} else {
		state.Attributes["ensure"] = "stopped"
	}

	// Check if enabled
	cmd = exec.CommandContext(ctx, "systemctl", "is-enabled", r.config.Name)
	output, _ = cmd.Output()
	isEnabled := strings.TrimSpace(string(output)) == "enabled"
	state.Attributes["enabled"] = isEnabled

	return state, nil
}

func (r *ServiceResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	if !current.Exists {
		return nil, fmt.Errorf("service %s does not exist", r.config.Name)
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

	for _, change := range plan.Changes {
		switch change.Attribute {
		case "ensure":
			newEnsure := change.New.(string)
			var cmd *exec.Cmd
			if newEnsure == "running" {
				cmd = exec.CommandContext(ctx, "systemctl", "start", r.config.Name)
			} else {
				cmd = exec.CommandContext(ctx, "systemctl", "stop", r.config.Name)
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to %s service: %w\nOutput: %s",
					newEnsure, err, string(output))
			}

		case "enabled":
			newEnabled := change.New.(bool)
			var cmd *exec.Cmd
			if newEnabled {
				cmd = exec.CommandContext(ctx, "systemctl", "enable", r.config.Name)
			} else {
				cmd = exec.CommandContext(ctx, "systemctl", "disable", r.config.Name)
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				action := "enable"
				if !newEnabled {
					action = "disable"
				}
				return fmt.Errorf("failed to %s service: %w\nOutput: %s",
					action, err, string(output))
			}
		}
	}

	return nil
}
