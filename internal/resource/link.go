package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("link", NewLinkResource)
}

// LinkResource manages symbolic links
type LinkResource struct {
	name      string
	config    config.LinkResourceConfig
	dependsOn []string
}

// NewLinkResource creates a new link resource from HCL
func NewLinkResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.LinkResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode link resource: %s", diags.Error())
	}

	return &LinkResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *LinkResource) Type() string { return "link" }
func (r *LinkResource) Name() string { return r.name }

func (r *LinkResource) Validate() error {
	if r.config.Path == "" {
		return fmt.Errorf("link.%s: path is required", r.name)
	}
	ensure := "present"
	if r.config.Ensure != nil {
		ensure = *r.config.Ensure
	}
	if ensure != "absent" && r.config.Target == "" {
		return fmt.Errorf("link.%s: target is required", r.name)
	}
	return nil
}

func (r *LinkResource) Dependencies() []string {
	return r.dependsOn
}

func (r *LinkResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Check if path exists and is a symlink
	info, err := os.Lstat(r.config.Path)
	if os.IsNotExist(err) {
		state.Exists = false
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat link: %w", err)
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		// Path exists but is not a symlink
		state.Exists = true
		state.Attributes["path"] = r.config.Path
		state.Attributes["is_symlink"] = false
		return state, nil
	}

	// It's a symlink - read the target
	target, err := os.Readlink(r.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read link target: %w", err)
	}

	state.Exists = true
	state.Attributes["path"] = r.config.Path
	state.Attributes["target"] = target
	state.Attributes["is_symlink"] = true

	return state, nil
}

func (r *LinkResource) Diff(ctx context.Context, current *State) (*Plan, error) {
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
				Attribute: "path",
				Old:       r.config.Path,
				New:       nil,
			})
		}
		return plan, nil
	}

	// Link should be present
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.Changes = append(plan.Changes, Change{
			Attribute: "path",
			Old:       nil,
			New:       r.config.Path,
		})
		plan.Changes = append(plan.Changes, Change{
			Attribute: "target",
			Old:       nil,
			New:       r.config.Target,
		})
		return plan, nil
	}

	// Something exists at the path
	isSymlink, _ := current.Attributes["is_symlink"].(bool)

	if !isSymlink {
		// Path exists but is not a symlink - this is an error unless force is set
		if r.config.Force != nil && *r.config.Force {
			plan.Action = ActionUpdate
			plan.Changes = append(plan.Changes, Change{
				Attribute: "type",
				Old:       "file/directory",
				New:       "symlink",
			})
			plan.Changes = append(plan.Changes, Change{
				Attribute: "target",
				Old:       nil,
				New:       r.config.Target,
			})
			return plan, nil
		}
		return nil, fmt.Errorf("path %s exists but is not a symlink (use force = true to replace)", r.config.Path)
	}

	// It's a symlink - check if target matches
	currentTarget, _ := current.Attributes["target"].(string)
	if currentTarget != r.config.Target {
		plan.Action = ActionUpdate
		plan.Changes = append(plan.Changes, Change{
			Attribute: "target",
			Old:       currentTarget,
			New:       r.config.Target,
		})
	}

	return plan, nil
}

func (r *LinkResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		if err := os.Remove(r.config.Path); err != nil {
			return fmt.Errorf("failed to remove link: %w", err)
		}

	case ActionCreate:
		// Ensure parent directory exists
		parentDir := filepath.Dir(r.config.Path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		if err := os.Symlink(r.config.Target, r.config.Path); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}

	case ActionUpdate:
		// Remove existing and recreate
		// Check if we need to force (remove non-symlink)
		info, err := os.Lstat(r.config.Path)
		if err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				// It's not a symlink - need to remove it (only if force is set, which was checked in Diff)
				if info.IsDir() {
					if err := os.RemoveAll(r.config.Path); err != nil {
						return fmt.Errorf("failed to remove directory: %w", err)
					}
				} else {
					if err := os.Remove(r.config.Path); err != nil {
						return fmt.Errorf("failed to remove file: %w", err)
					}
				}
			} else {
				// It's a symlink - just remove it
				if err := os.Remove(r.config.Path); err != nil {
					return fmt.Errorf("failed to remove old symlink: %w", err)
				}
			}
		}

		if err := os.Symlink(r.config.Target, r.config.Path); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	return nil
}
