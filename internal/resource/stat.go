package resource

import (
	"context"
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
	Register("stat", NewStatResource)
}

// StatResource is a read-only resource that gathers file information
type StatResource struct {
	name      string
	config    config.StatResourceConfig
	dependsOn []string
}

// NewStatResource creates a new stat resource from HCL
func NewStatResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.StatResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode stat resource: %s", diags.Error())
	}

	return &StatResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *StatResource) Type() string { return "stat" }
func (r *StatResource) Name() string { return r.name }

func (r *StatResource) Validate() error {
	if r.config.Path == "" {
		return fmt.Errorf("stat.%s: path is required", r.name)
	}
	return nil
}

func (r *StatResource) Dependencies() []string {
	return r.dependsOn
}

func (r *StatResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	// Determine whether to follow symlinks
	follow := true
	if r.config.Follow != nil {
		follow = *r.config.Follow
	}

	var info os.FileInfo
	var err error

	if follow {
		info, err = os.Stat(r.config.Path)
	} else {
		info, err = os.Lstat(r.config.Path)
	}

	if os.IsNotExist(err) {
		state.Exists = false
		state.Attributes["exists"] = false
		state.Attributes["path"] = r.config.Path
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	state.Exists = true
	state.Attributes["exists"] = true
	state.Attributes["path"] = r.config.Path
	state.Attributes["size"] = info.Size()
	state.Attributes["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())

	// File type flags
	state.Attributes["isdir"] = info.IsDir()
	state.Attributes["isfile"] = info.Mode().IsRegular()

	// Check if it's a symlink (need Lstat for this)
	linfo, lerr := os.Lstat(r.config.Path)
	if lerr == nil {
		state.Attributes["islink"] = linfo.Mode()&os.ModeSymlink != 0
	} else {
		state.Attributes["islink"] = false
	}

	// Get owner and group information
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		state.Attributes["uid"] = int64(stat.Uid)
		state.Attributes["gid"] = int64(stat.Gid)

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

		// Timestamps
		state.Attributes["mtime"] = info.ModTime().Unix()
		state.Attributes["atime"] = getAtime(stat)
	}

	return state, nil
}

func (r *StatResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	// Stat is a read-only resource, always returns ActionNoop
	plan := &Plan{
		Before: current,
		After:  current, // No changes expected
		Action: ActionNoop,
	}
	return plan, nil
}

func (r *StatResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	// Stat is a read-only resource, Apply is a no-op
	return nil
}
