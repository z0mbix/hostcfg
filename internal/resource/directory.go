package resource

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("directory", NewDirectoryResource)
}

// DirectoryResource manages directory resources
type DirectoryResource struct {
	name      string
	config    config.DirectoryResourceConfig
	dependsOn []string
}

// NewDirectoryResource creates a new directory resource from HCL
func NewDirectoryResource(name string, body hcl.Body, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.DirectoryResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode directory resource: %s", diags.Error())
	}

	// Extract depends_on from body
	var wrapper struct {
		DependsOn []string `hcl:"depends_on,optional"`
	}
	gohcl.DecodeBody(body, ctx, &wrapper)

	return &DirectoryResource{
		name:      name,
		config:    cfg,
		dependsOn: wrapper.DependsOn,
	}, nil
}

func (r *DirectoryResource) Type() string { return "directory" }
func (r *DirectoryResource) Name() string { return r.name }

func (r *DirectoryResource) Validate() error {
	if r.config.Path == "" {
		return fmt.Errorf("directory.%s: path is required", r.name)
	}
	return nil
}

func (r *DirectoryResource) Dependencies() []string {
	return r.dependsOn
}

func (r *DirectoryResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	info, err := os.Stat(r.config.Path)
	if os.IsNotExist(err) {
		state.Exists = false
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path exists but is not a directory: %s", r.config.Path)
	}

	state.Exists = true
	state.Attributes["path"] = r.config.Path
	state.Attributes["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())

	// Get owner and group
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if u, err := user.LookupId(strconv.Itoa(int(stat.Uid))); err == nil {
			state.Attributes["owner"] = u.Username
		} else {
			state.Attributes["owner"] = strconv.Itoa(int(stat.Uid))
		}
		if g, err := user.LookupGroupId(strconv.Itoa(int(stat.Gid))); err == nil {
			state.Attributes["group"] = g.Name
		} else {
			state.Attributes["group"] = strconv.Itoa(int(stat.Gid))
		}
	}

	return state, nil
}

func (r *DirectoryResource) Diff(ctx context.Context, current *State) (*Plan, error) {
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

	// Directory doesn't exist - create it
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.After.Attributes["path"] = r.config.Path
		plan.Changes = append(plan.Changes, Change{
			Attribute: "path",
			Old:       nil,
			New:       r.config.Path,
		})
		if r.config.Owner != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "owner",
				Old:       nil,
				New:       *r.config.Owner,
			})
		}
		if r.config.Group != nil {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "group",
				Old:       nil,
				New:       *r.config.Group,
			})
		}
		mode := "0755"
		if r.config.Mode != nil {
			mode = *r.config.Mode
		}
		plan.Changes = append(plan.Changes, Change{
			Attribute: "mode",
			Old:       nil,
			New:       mode,
		})
		return plan, nil
	}

	// Directory exists - check for changes
	plan.After.Exists = true
	plan.After.Attributes = make(map[string]interface{})
	for k, v := range current.Attributes {
		plan.After.Attributes[k] = v
	}

	// Check mode
	if r.config.Mode != nil {
		currentMode, _ := current.Attributes["mode"].(string)
		if currentMode != *r.config.Mode {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "mode",
				Old:       currentMode,
				New:       *r.config.Mode,
			})
		}
	}

	// Check owner
	if r.config.Owner != nil {
		currentOwner, _ := current.Attributes["owner"].(string)
		if currentOwner != *r.config.Owner {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "owner",
				Old:       currentOwner,
				New:       *r.config.Owner,
			})
		}
	}

	// Check group
	if r.config.Group != nil {
		currentGroup, _ := current.Attributes["group"].(string)
		if currentGroup != *r.config.Group {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "group",
				Old:       currentGroup,
				New:       *r.config.Group,
			})
		}
	}

	if len(plan.Changes) > 0 {
		plan.Action = ActionUpdate
	}

	return plan, nil
}

func (r *DirectoryResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		recursive := false
		if r.config.Recursive != nil {
			recursive = *r.config.Recursive
		}
		if recursive {
			return os.RemoveAll(r.config.Path)
		}
		return os.Remove(r.config.Path)

	case ActionCreate:
		mode := os.FileMode(0755)
		if r.config.Mode != nil {
			parsed, err := strconv.ParseUint(*r.config.Mode, 8, 32)
			if err != nil {
				return fmt.Errorf("invalid mode: %w", err)
			}
			mode = os.FileMode(parsed)
		}

		recursive := false
		if r.config.Recursive != nil {
			recursive = *r.config.Recursive
		}

		if recursive {
			if err := os.MkdirAll(r.config.Path, mode); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			if err := os.Mkdir(r.config.Path, mode); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		return r.applyOwnershipAndMode()

	case ActionUpdate:
		return r.applyOwnershipAndMode()
	}

	return nil
}

func (r *DirectoryResource) applyOwnershipAndMode() error {
	// Set ownership
	if r.config.Owner != nil || r.config.Group != nil {
		uid := -1
		gid := -1

		if r.config.Owner != nil {
			u, err := user.Lookup(*r.config.Owner)
			if err != nil {
				return fmt.Errorf("unknown user: %s", *r.config.Owner)
			}
			uid, _ = strconv.Atoi(u.Uid)
		}

		if r.config.Group != nil {
			g, err := user.LookupGroup(*r.config.Group)
			if err != nil {
				return fmt.Errorf("unknown group: %s", *r.config.Group)
			}
			gid, _ = strconv.Atoi(g.Gid)
		}

		// If recursive, apply to all children
		recursive := false
		if r.config.Recursive != nil {
			recursive = *r.config.Recursive
		}

		if recursive {
			err := filepath.Walk(r.config.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				return os.Chown(path, uid, gid)
			})
			if err != nil {
				return fmt.Errorf("failed to set ownership recursively: %w", err)
			}
		} else {
			if err := os.Chown(r.config.Path, uid, gid); err != nil {
				return fmt.Errorf("failed to set ownership: %w", err)
			}
		}
	}

	// Set mode
	if r.config.Mode != nil {
		parsed, _ := strconv.ParseUint(*r.config.Mode, 8, 32)
		if err := os.Chmod(r.config.Path, os.FileMode(parsed)); err != nil {
			return fmt.Errorf("failed to set mode: %w", err)
		}
	}

	return nil
}
