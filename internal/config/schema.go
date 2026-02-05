package config

import (
	"github.com/hashicorp/hcl/v2"
)

// Config represents the top-level configuration structure
type Config struct {
	Variables []*Variable      `hcl:"variable,block"`
	Resources []*ResourceBlock `hcl:"resource,block"`
	Roles     []*RoleBlock     `hcl:"role,block"`
}

// RoleBlock represents a role instantiation in HCL
type RoleBlock struct {
	Name      string         `hcl:"name,label"`
	Source    string         `hcl:"source"`
	Variables hcl.Expression `hcl:"variables,optional"`
	DependsOn []string       `hcl:"depends_on,optional"`
	Body      hcl.Body       `hcl:",remain"`
}

// Variable represents a variable definition in HCL
type Variable struct {
	Name        string         `hcl:"name,label"`
	TypeExpr    hcl.Expression `hcl:"type,optional"`
	Default     hcl.Expression `hcl:"default,optional"`
	Description string         `hcl:"description,optional"`
}

// ResourceBlock represents a resource definition in HCL
type ResourceBlock struct {
	Type        string         `hcl:"type,label"`
	Name        string         `hcl:"name,label"`
	Description string         `hcl:"description,optional"`
	DependsOn   []string       `hcl:"depends_on,optional"`
	ForEach   hcl.Expression `hcl:"for_each,optional"`
	When      hcl.Expression `hcl:"when,optional"`
	Body      hcl.Body       `hcl:",remain"`

	// RoleBaseDir is the base directory for role resources (for template path resolution).
	// This is set by the role loader and is empty for non-role resources.
	// No HCL tag means gohcl ignores this field during decoding.
	RoleBaseDir string

	// ForEachKey stores the iteration key for expanded resources.
	// Empty for non-for_each resources, set during expansion.
	ForEachKey string
}

// FileResourceConfig holds file resource specific attributes
type FileResourceConfig struct {
	Path    string  `hcl:"path"`
	Content *string `hcl:"content,optional"`
	Source  *string `hcl:"source,optional"`
	Owner   *string `hcl:"owner,optional"`
	Group   *string `hcl:"group,optional"`
	Mode    *string `hcl:"mode,optional"`
	Ensure  *string `hcl:"ensure,optional"` // "present" or "absent"
}

// DirectoryResourceConfig holds directory resource specific attributes
type DirectoryResourceConfig struct {
	Path      string  `hcl:"path"`
	Owner     *string `hcl:"owner,optional"`
	Group     *string `hcl:"group,optional"`
	Mode      *string `hcl:"mode,optional"`
	Recursive *bool   `hcl:"recursive,optional"`
	Ensure    *string `hcl:"ensure,optional"` // "present" or "absent"
}

// ExecResourceConfig holds exec resource specific attributes
type ExecResourceConfig struct {
	Command string  `hcl:"command"`
	Creates *string `hcl:"creates,optional"` // File that indicates command has run
	OnlyIf  *string `hcl:"only_if,optional"` // Run only if this command succeeds
	Unless  *string `hcl:"unless,optional"`  // Run unless this command succeeds
	Dir     *string `hcl:"dir,optional"`     // Working directory
	User    *string `hcl:"user,optional"`    // User to run as
}

// HostnameResourceConfig holds hostname resource specific attributes
type HostnameResourceConfig struct {
	Name string `hcl:"name"`
}

// CronResourceConfig holds cron resource specific attributes
type CronResourceConfig struct {
	Command  string  `hcl:"command"`
	Schedule string  `hcl:"schedule"` // cron expression (e.g., "0 * * * *")
	User     *string `hcl:"user,optional"`
	Ensure   *string `hcl:"ensure,optional"` // "present" or "absent"
}

// PackageResourceConfig holds package resource specific attributes
type PackageResourceConfig struct {
	Name    string  `hcl:"name"`
	Version *string `hcl:"version,optional"`
	Ensure  *string `hcl:"ensure,optional"` // "present", "absent", or specific version
}

// ServiceResourceConfig holds service resource specific attributes
type ServiceResourceConfig struct {
	Name    string  `hcl:"name"`
	Ensure  *string `hcl:"ensure,optional"`  // "running" or "stopped"
	Enabled *bool   `hcl:"enabled,optional"` // Start on boot
}

// UserResourceConfig holds user resource specific attributes
type UserResourceConfig struct {
	Name       string   `hcl:"name"`
	UID        *string  `hcl:"uid,optional"`
	GID        *string  `hcl:"gid,optional"`
	Groups     []string `hcl:"groups,optional"`
	Home       *string  `hcl:"home,optional"`
	Shell      *string  `hcl:"shell,optional"`
	Comment    *string  `hcl:"comment,optional"` // GECOS field
	System     *bool    `hcl:"system,optional"`  // Create as system user
	CreateHome *bool    `hcl:"create_home,optional"`
	Ensure     *string  `hcl:"ensure,optional"` // "present" or "absent"
}

// GroupResourceConfig holds group resource specific attributes
type GroupResourceConfig struct {
	Name    string   `hcl:"name"`
	GID     *string  `hcl:"gid,optional"`
	Members []string `hcl:"members,optional"`
	System  *bool    `hcl:"system,optional"` // Create as system group
	Ensure  *string  `hcl:"ensure,optional"` // "present" or "absent"
}

// LinkResourceConfig holds symbolic link resource specific attributes
type LinkResourceConfig struct {
	Path   string  `hcl:"path"`
	Target string  `hcl:"target"`
	Force  *bool   `hcl:"force,optional"`  // Replace existing file/directory
	Ensure *string `hcl:"ensure,optional"` // "present" or "absent"
}

// DownloadResourceConfig holds download resource specific attributes
type DownloadResourceConfig struct {
	URL      string  `hcl:"url"`
	Dest     string  `hcl:"dest"`
	Checksum *string `hcl:"checksum,optional"` // "algorithm:hash" (md5, sha1, sha256, sha512)
	Owner    *string `hcl:"owner,optional"`
	Group    *string `hcl:"group,optional"`
	Mode     *string `hcl:"mode,optional"`
	Force    *bool   `hcl:"force,optional"`   // Force re-download even if checksum matches
	Timeout  *int    `hcl:"timeout,optional"` // HTTP timeout in seconds (default: 30)
}

// StatResourceConfig holds stat resource specific attributes
type StatResourceConfig struct {
	Path   string `hcl:"path"`
	Follow *bool  `hcl:"follow,optional"` // Follow symlinks (default: true)
}
