package resource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("hostname", NewHostnameResource)
}

// HostnameResource manages the system hostname
type HostnameResource struct {
	name      string
	config    config.HostnameResourceConfig
	dependsOn []string
}

// NewHostnameResource creates a new hostname resource from HCL
func NewHostnameResource(name string, body hcl.Body, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.HostnameResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode hostname resource: %s", diags.Error())
	}

	// Extract depends_on from body
	var wrapper struct {
		DependsOn []string `hcl:"depends_on,optional"`
	}
	gohcl.DecodeBody(body, ctx, &wrapper)

	return &HostnameResource{
		name:      name,
		config:    cfg,
		dependsOn: wrapper.DependsOn,
	}, nil
}

func (r *HostnameResource) Type() string { return "hostname" }
func (r *HostnameResource) Name() string { return r.name }

func (r *HostnameResource) Validate() error {
	if r.config.Name == "" {
		return fmt.Errorf("hostname.%s: name is required", r.name)
	}
	return nil
}

func (r *HostnameResource) Dependencies() []string {
	return r.dependsOn
}

func (r *HostnameResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Read from /etc/hostname
	content, err := os.ReadFile("/etc/hostname")
	if os.IsNotExist(err) {
		state.Exists = false
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/hostname: %w", err)
	}

	state.Exists = true
	state.Attributes["name"] = strings.TrimSpace(string(content))

	return state, nil
}

func (r *HostnameResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	currentName, _ := current.Attributes["name"].(string)

	if currentName == r.config.Name {
		plan.Action = ActionNoop
		return plan, nil
	}

	if !current.Exists {
		plan.Action = ActionCreate
	} else {
		plan.Action = ActionUpdate
	}

	plan.After.Exists = true
	plan.After.Attributes["name"] = r.config.Name

	plan.Changes = append(plan.Changes, Change{
		Attribute: "name",
		Old:       currentName,
		New:       r.config.Name,
	})

	return plan, nil
}

func (r *HostnameResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	// Write to /etc/hostname
	if err := os.WriteFile("/etc/hostname", []byte(r.config.Name+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/hostname: %w", err)
	}

	// Set the hostname immediately using hostnamectl if available
	if _, err := exec.LookPath("hostnamectl"); err == nil {
		cmd := exec.CommandContext(ctx, "hostnamectl", "set-hostname", r.config.Name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set hostname with hostnamectl: %w", err)
		}
	} else {
		// Fallback to hostname command
		cmd := exec.CommandContext(ctx, "hostname", r.config.Name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set hostname: %w", err)
		}
	}

	return nil
}
