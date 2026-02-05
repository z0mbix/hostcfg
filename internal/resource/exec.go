package resource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("exec", NewExecResource)
}

// ExecResource manages exec (command execution) resources
type ExecResource struct {
	name        string
	description string
	config      config.ExecResourceConfig
	dependsOn   []string
}

// NewExecResource creates a new exec resource from HCL
func NewExecResource(name string, body hcl.Body, dependsOn []string, description string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.ExecResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode exec resource: %s", diags.Error())
	}

	return &ExecResource{
		name:        name,
		description: description,
		config:      cfg,
		dependsOn:   dependsOn,
	}, nil
}

func (r *ExecResource) Type() string        { return "exec" }
func (r *ExecResource) Name() string        { return r.name }
func (r *ExecResource) Description() string { return r.description }

func (r *ExecResource) Validate() error {
	if r.config.Command == "" {
		return fmt.Errorf("exec.%s: command is required", r.name)
	}
	return nil
}

func (r *ExecResource) Dependencies() []string {
	return r.dependsOn
}

func (r *ExecResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Check if the "creates" file exists
	if r.config.Creates != nil {
		if _, err := os.Stat(*r.config.Creates); err == nil {
			state.Exists = true
			state.Attributes["creates"] = *r.config.Creates
			return state, nil
		}
	}

	// Check only_if condition
	if r.config.OnlyIf != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", *r.config.OnlyIf)
		if err := cmd.Run(); err != nil {
			// only_if failed, so command should not run
			state.Exists = true
			state.Attributes["only_if_satisfied"] = false
			return state, nil
		}
		state.Attributes["only_if_satisfied"] = true
	}

	// Check unless condition
	if r.config.Unless != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", *r.config.Unless)
		if err := cmd.Run(); err == nil {
			// unless succeeded, so command should not run
			state.Exists = true
			state.Attributes["unless_satisfied"] = true
			return state, nil
		}
		state.Attributes["unless_satisfied"] = false
	}

	state.Exists = false
	return state, nil
}

func (r *ExecResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	// If creates file exists, no action needed
	if r.config.Creates != nil {
		if _, exists := current.Attributes["creates"]; exists {
			plan.Action = ActionNoop
			return plan, nil
		}
	}

	// Check only_if - if not satisfied, no action
	if satisfied, ok := current.Attributes["only_if_satisfied"].(bool); ok && !satisfied {
		plan.Action = ActionNoop
		return plan, nil
	}

	// Check unless - if satisfied (command succeeded), no action
	if satisfied, ok := current.Attributes["unless_satisfied"].(bool); ok && satisfied {
		plan.Action = ActionNoop
		return plan, nil
	}

	// Command should run
	plan.Action = ActionCreate
	plan.After.Exists = true
	plan.Changes = append(plan.Changes, Change{
		Attribute: "command",
		Old:       nil,
		New:       r.config.Command,
	})

	if r.config.Creates != nil {
		plan.Changes = append(plan.Changes, Change{
			Attribute: "creates",
			Old:       nil,
			New:       *r.config.Creates,
		})
	}

	return plan, nil
}

func (r *ExecResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", r.config.Command)

	// Set working directory
	if r.config.Dir != nil {
		cmd.Dir = *r.config.Dir
	}

	// Set user if specified
	if r.config.User != nil {
		u, err := user.Lookup(*r.config.User)
		if err != nil {
			return fmt.Errorf("unknown user: %s", *r.config.User)
		}
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
