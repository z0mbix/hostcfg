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
	Register("user", NewUserResource)
}

// UserResource manages system users
type UserResource struct {
	name      string
	config    config.UserResourceConfig
	dependsOn []string
}

// NewUserResource creates a new user resource from HCL
func NewUserResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.UserResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode user resource: %s", diags.Error())
	}

	return &UserResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *UserResource) Type() string { return "user" }
func (r *UserResource) Name() string { return r.name }

func (r *UserResource) Validate() error {
	if r.config.Name == "" {
		return fmt.Errorf("user.%s: name is required", r.name)
	}
	return nil
}

func (r *UserResource) Dependencies() []string {
	return r.dependsOn
}

func (r *UserResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Check if user exists by reading /etc/passwd
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/passwd: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) >= 7 && fields[0] == r.config.Name {
			state.Exists = true
			state.Attributes["name"] = fields[0]
			state.Attributes["uid"] = fields[2]
			state.Attributes["gid"] = fields[3]
			state.Attributes["comment"] = fields[4]
			state.Attributes["home"] = fields[5]
			state.Attributes["shell"] = fields[6]

			// Get supplementary groups
			groups, _ := r.getUserGroups(r.config.Name)
			state.Attributes["groups"] = groups

			return state, nil
		}
	}

	state.Exists = false
	return state, nil
}

func (r *UserResource) getUserGroups(username string) ([]string, error) {
	cmd := exec.Command("id", "-nG", username)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	groups := strings.Fields(strings.TrimSpace(string(output)))
	return groups, nil
}

func (r *UserResource) Diff(ctx context.Context, current *State) (*Plan, error) {
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

	// User should be present
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.Changes = append(plan.Changes, Change{
			Attribute: "name",
			Old:       nil,
			New:       r.config.Name,
		})
		if r.config.UID != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "uid",
				Old:       nil,
				New:       *r.config.UID,
			})
		}
		if r.config.GID != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "gid",
				Old:       nil,
				New:       *r.config.GID,
			})
		}
		if r.config.Home != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "home",
				Old:       nil,
				New:       *r.config.Home,
			})
		}
		if r.config.Shell != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "shell",
				Old:       nil,
				New:       *r.config.Shell,
			})
		}
		if r.config.Comment != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "comment",
				Old:       nil,
				New:       *r.config.Comment,
			})
		}
		return plan, nil
	}

	// User exists - check for changes
	plan.After.Exists = true

	if r.config.Shell != nil {
		currentShell, _ := current.Attributes["shell"].(string)
		if currentShell != *r.config.Shell {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "shell",
				Old:       currentShell,
				New:       *r.config.Shell,
			})
		}
	}

	if r.config.Home != nil {
		currentHome, _ := current.Attributes["home"].(string)
		if currentHome != *r.config.Home {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "home",
				Old:       currentHome,
				New:       *r.config.Home,
			})
		}
	}

	if r.config.Comment != nil {
		currentComment, _ := current.Attributes["comment"].(string)
		if currentComment != *r.config.Comment {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "comment",
				Old:       currentComment,
				New:       *r.config.Comment,
			})
		}
	}

	if r.config.Groups != nil {
		currentGroups, _ := current.Attributes["groups"].([]string)
		if !stringSlicesEqual(currentGroups, r.config.Groups) {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "groups",
				Old:       strings.Join(currentGroups, ","),
				New:       strings.Join(r.config.Groups, ","),
			})
		}
	}

	if len(plan.Changes) > 0 {
		plan.Action = ActionUpdate
	}

	return plan, nil
}

func (r *UserResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		cmd := exec.CommandContext(ctx, "userdel", r.config.Name)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to delete user: %w\nOutput: %s", err, string(output))
		}

	case ActionCreate:
		args := []string{}

		if r.config.UID != nil {
			args = append(args, "-u", *r.config.UID)
		}
		if r.config.GID != nil {
			args = append(args, "-g", *r.config.GID)
		}
		if r.config.Home != nil {
			args = append(args, "-d", *r.config.Home)
		}
		if r.config.Shell != nil {
			args = append(args, "-s", *r.config.Shell)
		}
		if r.config.Comment != nil {
			args = append(args, "-c", *r.config.Comment)
		}
		if r.config.Groups != nil && len(r.config.Groups) > 0 {
			args = append(args, "-G", strings.Join(r.config.Groups, ","))
		}
		if r.config.System != nil && *r.config.System {
			args = append(args, "-r")
		}
		if r.config.CreateHome != nil && *r.config.CreateHome {
			args = append(args, "-m")
		}

		args = append(args, r.config.Name)

		cmd := exec.CommandContext(ctx, "useradd", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create user: %w\nOutput: %s", err, string(output))
		}

	case ActionUpdate:
		args := []string{}

		for _, change := range plan.Changes {
			switch change.Attribute {
			case "shell":
				args = append(args, "-s", change.New.(string))
			case "home":
				args = append(args, "-d", change.New.(string))
			case "comment":
				args = append(args, "-c", change.New.(string))
			case "groups":
				groupStr := change.New.(string)
				if groupStr != "" {
					args = append(args, "-G", groupStr)
				}
			}
		}

		if len(args) > 0 {
			args = append(args, r.config.Name)
			cmd := exec.CommandContext(ctx, "usermod", args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to modify user: %w\nOutput: %s", err, string(output))
			}
		}
	}

	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}
	return true
}
