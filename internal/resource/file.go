package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("file", NewFileResource)
}

// FileResource manages file resources
type FileResource struct {
	name      string
	config    config.FileResourceConfig
	dependsOn []string
}

// NewFileResource creates a new file resource from HCL
func NewFileResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.FileResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode file resource: %s", diags.Error())
	}

	return &FileResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *FileResource) Type() string { return "file" }
func (r *FileResource) Name() string { return r.name }

func (r *FileResource) Validate() error {
	if r.config.Path == "" {
		return fmt.Errorf("file.%s: path is required", r.name)
	}
	if r.config.Content == nil && r.config.Source == nil {
		ensure := "present"
		if r.config.Ensure != nil {
			ensure = *r.config.Ensure
		}
		if ensure != "absent" {
			return fmt.Errorf("file.%s: either content or source is required", r.name)
		}
	}
	if r.config.Content != nil && r.config.Source != nil {
		return fmt.Errorf("file.%s: cannot specify both content and source", r.name)
	}
	return nil
}

func (r *FileResource) Dependencies() []string {
	return r.dependsOn
}

func (r *FileResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	info, err := os.Stat(r.config.Path)
	if os.IsNotExist(err) {
		state.Exists = false
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
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

	// Get content hash
	content, err := os.ReadFile(r.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	hash := sha256.Sum256(content)
	state.Attributes["content_hash"] = hex.EncodeToString(hash[:])
	state.Attributes["content"] = string(content)

	return state, nil
}

func (r *FileResource) Diff(ctx context.Context, current *State) (*Plan, error) {
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

	// Get desired content
	desiredContent, err := r.getDesiredContent()
	if err != nil {
		return nil, err
	}

	// File doesn't exist - create it
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
		mode := "0644"
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

	// File exists - check for changes
	plan.After.Exists = true
	plan.After.Attributes = make(map[string]interface{})
	for k, v := range current.Attributes {
		plan.After.Attributes[k] = v
	}

	// Check content
	desiredHash := sha256.Sum256([]byte(desiredContent))
	desiredHashStr := hex.EncodeToString(desiredHash[:])
	if currentHash, ok := current.Attributes["content_hash"].(string); ok {
		if currentHash != desiredHashStr {
			plan.Changes = append(plan.Changes, Change{
				Attribute: "content",
				Old:       current.Attributes["content"],
				New:       desiredContent,
			})
		}
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

func (r *FileResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	switch plan.Action {
	case ActionDelete:
		return os.Remove(r.config.Path)

	case ActionCreate, ActionUpdate:
		// Write content
		content, err := r.getDesiredContent()
		if err != nil {
			return err
		}

		mode := os.FileMode(0644)
		if r.config.Mode != nil {
			parsed, err := strconv.ParseUint(*r.config.Mode, 8, 32)
			if err != nil {
				return fmt.Errorf("invalid mode: %w", err)
			}
			mode = os.FileMode(parsed)
		}

		if err := os.WriteFile(r.config.Path, []byte(content), mode); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

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

			if err := os.Chown(r.config.Path, uid, gid); err != nil {
				return fmt.Errorf("failed to set ownership: %w", err)
			}
		}

		// Set mode (again, in case it was overwritten by umask during create)
		if r.config.Mode != nil {
			parsed, _ := strconv.ParseUint(*r.config.Mode, 8, 32)
			if err := os.Chmod(r.config.Path, os.FileMode(parsed)); err != nil {
				return fmt.Errorf("failed to set mode: %w", err)
			}
		}
	}

	return nil
}

func (r *FileResource) getDesiredContent() (string, error) {
	if r.config.Content != nil {
		return *r.config.Content, nil
	}
	if r.config.Source != nil {
		content, err := os.ReadFile(*r.config.Source)
		if err != nil {
			return "", fmt.Errorf("failed to read source file: %w", err)
		}
		return string(content), nil
	}
	return "", nil
}
