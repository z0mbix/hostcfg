package resource

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("download", NewDownloadResource)
}

// DownloadResource manages file downloads
type DownloadResource struct {
	name      string
	config    config.DownloadResourceConfig
	dependsOn []string
}

// NewDownloadResource creates a new download resource from HCL
func NewDownloadResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.DownloadResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode download resource: %s", diags.Error())
	}

	return &DownloadResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *DownloadResource) Type() string { return "download" }
func (r *DownloadResource) Name() string { return r.name }

func (r *DownloadResource) Validate() error {
	if r.config.URL == "" {
		return fmt.Errorf("download.%s: url is required", r.name)
	}
	if r.config.Dest == "" {
		return fmt.Errorf("download.%s: dest is required", r.name)
	}
	if r.config.Checksum != nil {
		if _, _, err := parseChecksum(*r.config.Checksum); err != nil {
			return fmt.Errorf("download.%s: %w", r.name, err)
		}
	}
	return nil
}

func (r *DownloadResource) Dependencies() []string {
	return r.dependsOn
}

func (r *DownloadResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	info, err := os.Stat(r.config.Dest)
	if os.IsNotExist(err) {
		state.Exists = false
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat dest file: %w", err)
	}

	state.Exists = true
	state.Attributes["dest"] = r.config.Dest
	state.Attributes["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	state.Attributes["size"] = info.Size()

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

	// Calculate checksum if one is expected
	if r.config.Checksum != nil {
		algorithm, _, err := parseChecksum(*r.config.Checksum)
		if err == nil {
			hash, err := computeFileChecksum(r.config.Dest, algorithm)
			if err == nil {
				state.Attributes["checksum"] = algorithm + ":" + hash
			}
		}
	}

	return state, nil
}

func (r *DownloadResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	// File doesn't exist - create it
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.After.Attributes["dest"] = r.config.Dest
		plan.Changes = append(plan.Changes, Change{
			Attribute: "dest",
			Old:       nil,
			New:       r.config.Dest,
		})
		plan.Changes = append(plan.Changes, Change{
			Attribute: "url",
			Old:       nil,
			New:       r.config.URL,
		})
		return plan, nil
	}

	// File exists - check if we need to update

	// Force re-download if force = true
	if r.config.Force != nil && *r.config.Force {
		plan.Action = ActionUpdate
		plan.Changes = append(plan.Changes, Change{
			Attribute: "force",
			Old:       false,
			New:       true,
		})
		return plan, nil
	}

	// Check checksum if provided
	if r.config.Checksum != nil {
		currentChecksum, _ := current.Attributes["checksum"].(string)
		if currentChecksum != *r.config.Checksum {
			plan.Action = ActionUpdate
			plan.Changes = append(plan.Changes, Change{
				Attribute: "checksum",
				Old:       currentChecksum,
				New:       *r.config.Checksum,
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
		if plan.Action == ActionNoop {
			plan.Action = ActionUpdate
		}
	}

	return plan, nil
}

func (r *DownloadResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	// Check if we need to download or just update permissions
	needsDownload := false
	for _, change := range plan.Changes {
		if change.Attribute == "dest" || change.Attribute == "url" ||
			change.Attribute == "checksum" || change.Attribute == "force" {
			needsDownload = true
			break
		}
	}

	if needsDownload || plan.Action == ActionCreate {
		if err := r.download(ctx); err != nil {
			return err
		}
	}

	// Set mode
	if r.config.Mode != nil {
		parsed, err := strconv.ParseUint(*r.config.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode: %w", err)
		}
		if err := os.Chmod(r.config.Dest, os.FileMode(parsed)); err != nil {
			return fmt.Errorf("failed to set mode: %w", err)
		}
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

		if err := os.Chown(r.config.Dest, uid, gid); err != nil {
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	return nil
}

func (r *DownloadResource) download(ctx context.Context) error {
	// Create HTTP client with timeout
	timeout := 30
	if r.config.Timeout != nil {
		timeout = *r.config.Timeout
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.config.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(r.config.Dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create temp file in the same directory for atomic rename
	tmpFile, err := os.CreateTemp(destDir, ".download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on error
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	// Set default mode
	mode := os.FileMode(0644)
	if r.config.Mode != nil {
		parsed, err := strconv.ParseUint(*r.config.Mode, 8, 32)
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("invalid mode: %w", err)
		}
		mode = os.FileMode(parsed)
	}
	if err := tmpFile.Chmod(mode); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to set temp file mode: %w", err)
	}

	// Write to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Verify checksum if provided
	if r.config.Checksum != nil {
		algorithm, expectedHash, err := parseChecksum(*r.config.Checksum)
		if err != nil {
			return err
		}

		actualHash, err := computeFileChecksum(tmpPath, algorithm)
		if err != nil {
			return fmt.Errorf("failed to compute checksum: %w", err)
		}

		if actualHash != expectedHash {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}
	}

	// Atomic rename
	if err := os.Rename(tmpPath, r.config.Dest); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	// Clear tmpPath so deferred cleanup doesn't remove the destination
	tmpPath = ""

	return nil
}

// parseChecksum parses a checksum string in the format "algorithm:hash"
func parseChecksum(checksum string) (algorithm, hash string, err error) {
	parts := strings.SplitN(checksum, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid checksum format, expected 'algorithm:hash'")
	}

	algorithm = strings.ToLower(parts[0])
	hash = strings.ToLower(parts[1])

	switch algorithm {
	case "md5", "sha1", "sha256", "sha512":
		// Valid
	default:
		return "", "", fmt.Errorf("unsupported checksum algorithm: %s (supported: md5, sha1, sha256, sha512)", algorithm)
	}

	return algorithm, hash, nil
}

// computeFileChecksum computes the checksum of a file using the specified algorithm
func computeFileChecksum(path, algorithm string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	switch algorithm {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
