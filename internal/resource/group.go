package resource

import (
	"bufio"
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
	Register("group", NewGroupResource)
}

// GroupResource manages system groups
type GroupResource struct {
	name      string
	config    config.GroupResourceConfig
	dependsOn []string
}

// NewGroupResource creates a new group resource from HCL
func NewGroupResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.GroupResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode group resource: %s", diags.Error())
	}

	return &GroupResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *GroupResource) Type() string { return "group" }
func (r *GroupResource) Name() string { return r.name }

func (r *GroupResource) Validate() error {
	if r.config.Name == "" {
		return fmt.Errorf("group.%s: name is required", r.name)
	}
	return nil
}

func (r *GroupResource) Dependencies() []string {
	return r.dependsOn
}

func (r *GroupResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Check if group exists by reading /etc/group
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/group: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) >= 4 && fields[0] == r.config.Name {
			state.Exists = true
			state.Attributes["name"] = fields[0]
			state.Attributes["gid"] = fields[2]

			// Get members (field 4, comma-separated)
			members := []string{}
			if fields[3] != "" {
				members = strings.Split(fields[3], ",")
			}
			state.Attributes["members"] = members

			return state, nil
		}
	}

	state.Exists = false
	return state, nil
}

func (r *GroupResource) Diff(ctx context.Context, current *State) (*Plan, error) {
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
				Attribute: "name",
				Old:       r.config.Name,
				New:       nil,
			})
		}
		return plan, nil
	}

	// Group should be present
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.Changes = append(plan.Changes, Change{
			Attribute: "name",
			Old:       nil,
			New:       r.config.Name,
		})
		if r.config.GID != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "gid",
				Old:       nil,
				New:       *r.config.GID,
			})
		}
		return plan, nil
	}

	// Group exists - check for changes
	plan.After.Exists = true

	// Note: GID changes are not supported for existing groups (would require recreating)

	if r.config.Members != nil {
		currentMembers, _ := current.Attributes["members"].([]string)
		if !stringSlicesEqual(currentMembers, r.config.Members) {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "members",
				Old:       strings.Join(currentMembers, ","),
				New:       strings.Join(r.config.Members, ","),
			})
		}
	}

	if len(plan.Changes) > 0 {
		plan.Action = ActionUpdate
	}

	return plan, nil
}

func (r *GroupResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		cmd := exec.CommandContext(ctx, "groupdel", r.config.Name)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to delete group: %w\nOutput: %s", err, string(output))
		}

	case ActionCreate:
		args := []string{}

		if r.config.GID != nil {
			args = append(args, "-g", *r.config.GID)
		}
		if r.config.System != nil && *r.config.System {
			args = append(args, "-r")
		}

		args = append(args, r.config.Name)

		cmd := exec.CommandContext(ctx, "groupadd", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create group: %w\nOutput: %s", err, string(output))
		}

		// Add members if specified
		if len(r.config.Members) > 0 {
			if err := r.setGroupMembers(ctx, r.config.Members); err != nil {
				return err
			}
		}

	case ActionUpdate:
		for _, change := range plan.Changes {
			switch change.Attribute {
			case "members":
				memberStr := change.New.(string)
				var members []string
				if memberStr != "" {
					members = strings.Split(memberStr, ",")
				}
				if err := r.setGroupMembers(ctx, members); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *GroupResource) setGroupMembers(ctx context.Context, members []string) error {
	// Use gpasswd to set group members
	// First remove all members, then add the specified ones
	if len(members) == 0 {
		// Remove all members by setting empty member list
		cmd := exec.CommandContext(ctx, "gpasswd", "-M", "", r.config.Name)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to clear group members: %w\nOutput: %s", err, string(output))
		}
	} else {
		// Set members using gpasswd -M
		cmd := exec.CommandContext(ctx, "gpasswd", "-M", strings.Join(members, ","), r.config.Name)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to set group members: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}
