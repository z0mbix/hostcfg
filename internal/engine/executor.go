package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/z0mbix/hostcfg/internal/diff"
	"github.com/z0mbix/hostcfg/internal/facts"
	"github.com/z0mbix/hostcfg/internal/resource"
	"github.com/z0mbix/hostcfg/internal/role"
	"github.com/zclconf/go-cty/cty"
)

// knownResourceTypes lists all resource types that can be referenced
var knownResourceTypes = map[string]bool{
	"file":      true,
	"directory": true,
	"exec":      true,
	"hostname":  true,
	"cron":      true,
	"package":   true,
	"service":   true,
	"user":      true,
	"group":     true,
	"link":      true,
	"download":  true,
	"stat":      true,
}

// Executor runs the configuration management process
type Executor struct {
	parser    *config.Parser
	graph     *Graph
	printer   *diff.Printer
	out       io.Writer
	useColors bool
	roles     map[string]*role.Role
	cliVars   map[string]cty.Value

	// for_each tracking
	forEachValues        map[string]cty.Value  // resourceID -> each.value
	forEachOriginalNames map[string][]string   // originalID -> []expandedIDs
}

// NewExecutor creates a new executor
func NewExecutor(out io.Writer, useColors bool) *Executor {
	parser := config.NewParser()

	// Gather system facts and inject into parser
	if f, err := facts.Gather(); err == nil {
		parser.SetFacts(f.ToCtyValue())
	}

	return &Executor{
		parser:               parser,
		graph:                NewGraph(),
		printer:              diff.NewPrinter(out, useColors),
		out:                  out,
		useColors:            useColors,
		roles:                make(map[string]*role.Role),
		cliVars:              make(map[string]cty.Value),
		forEachValues:        make(map[string]cty.Value),
		forEachOriginalNames: make(map[string][]string),
	}
}

// SetVariable sets a variable for use during execution
func (e *Executor) SetVariable(name, value string) {
	e.parser.SetVariable(name, value)
	e.cliVars[name] = cty.StringVal(value)
}

// SetVariableValue sets a variable with a cty.Value directly (for non-string types from var files)
func (e *Executor) SetVariableValue(name string, value cty.Value) {
	e.parser.SetVariableValue(name, value)
	e.cliVars[name] = value
}

// LoadFile loads and parses an HCL configuration file
func (e *Executor) LoadFile(filename string) error {
	cfg, diags := e.parser.ParseFile(filename)
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse file: %s", diags.Error())
	}

	return e.loadConfig(cfg)
}

// LoadDirectory loads and parses all HCL files in a directory
func (e *Executor) LoadDirectory(dir string) error {
	cfg, diags := e.parser.ParseDirectory(dir)
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse directory: %s", diags.Error())
	}

	return e.loadConfig(cfg)
}

func (e *Executor) loadConfig(cfg *config.Config) error {
	// Phase 0: Load all roles
	if len(cfg.Roles) > 0 {
		roleLoader := role.NewLoader(e.parser, e.parser.GetBaseDir(), e.cliVars)
		for _, roleBlock := range cfg.Roles {
			r, err := roleLoader.LoadRole(roleBlock)
			if err != nil {
				return fmt.Errorf("failed to load role %s: %w", roleBlock.Name, err)
			}
			e.roles[roleBlock.Name] = r

			// Append role resources to main resource list
			cfg.Resources = append(cfg.Resources, r.Resources...)
		}
	}

	// Phase 0.5: Expand for_each resources
	expandedResources, err := e.expandForEachResources(cfg.Resources)
	if err != nil {
		return fmt.Errorf("failed to expand for_each resources: %w", err)
	}
	cfg.Resources = expandedResources

	// First pass: extract resource attributes so they can be referenced
	// by other resources. We decode each resource to get its attribute values.
	for _, block := range cfg.Resources {
		// Set role context if this is a role resource
		if block.RoleBaseDir != "" {
			e.parser.SetRoleContext(block.RoleBaseDir)
		}
		attrs := e.extractResourceAttributes(block)
		if len(attrs) > 0 {
			e.parser.SetResourceAttributes(block.Type, block.Name, attrs)
		}
		// Clear role context
		if block.RoleBaseDir != "" {
			e.parser.ClearRoleContext()
		}
	}

	// Second pass: create resources with full context (including resource references)
	for _, block := range cfg.Resources {
		// Set role context if this is a role resource (for template path resolution)
		if block.RoleBaseDir != "" {
			e.parser.SetRoleContext(block.RoleBaseDir)
		}

		// Build context with each.key/each.value if this is an expanded for_each resource
		var ctx *hcl.EvalContext
		if block.ForEachKey != "" {
			eachKey := cty.StringVal(block.ForEachKey)
			eachValue := e.forEachValues[block.Type+"."+block.Name]
			if eachValue.IsNull() {
				eachValue = eachKey // Fallback for sets where key == value
			}
			ctx = e.parser.BuildEvalContextWithEach(eachKey, eachValue)
		} else {
			ctx = e.parser.GetEvalContext()
		}

		// Extract implicit dependencies from resource references in expressions
		implicitDeps := e.extractImplicitDependencies(block)

		// Merge explicit depends_on with implicit dependencies
		allDeps := e.mergeDependencies(block.DependsOn, implicitDeps)

		// Expand role-level dependencies (role.xxx -> all resources in that role)
		allDeps = e.expandRoleDependencies(allDeps)

		// Expand for_each dependencies (reference to base name -> all expanded instances)
		allDeps = e.expandForEachDependencies(allDeps)

		r, err := resource.CreateWithDeps(block, allDeps, ctx)

		// Clear role context
		if block.RoleBaseDir != "" {
			e.parser.ClearRoleContext()
		}

		if err != nil {
			return fmt.Errorf("failed to create resource %s.%s: %w",
				block.Type, block.Name, err)
		}

		if err := r.Validate(); err != nil {
			return err
		}

		e.graph.Add(r)
	}

	// Validate the dependency graph
	if err := e.graph.Validate(); err != nil {
		return err
	}

	return nil
}

// expandRoleDependencies converts "role.redis" to all resources in that role
func (e *Executor) expandRoleDependencies(deps []string) []string {
	var result []string
	for _, dep := range deps {
		if strings.HasPrefix(dep, "role.") {
			roleName := strings.TrimPrefix(dep, "role.")
			if r, ok := e.roles[roleName]; ok {
				result = append(result, r.GetResourceIDs()...)
			}
		} else {
			result = append(result, dep)
		}
	}
	return result
}

// expandForEachResources expands any resource with for_each into multiple resources
func (e *Executor) expandForEachResources(resources []*config.ResourceBlock) ([]*config.ResourceBlock, error) {
	var result []*config.ResourceBlock

	for _, block := range resources {
		// Evaluate the for_each expression (returns nil if not present or null)
		iterations, diags := e.parser.EvaluateForEach(block.ForEach)
		if diags.HasErrors() {
			return nil, fmt.Errorf("failed to evaluate for_each for %s.%s: %s",
				block.Type, block.Name, diags.Error())
		}

		// If no for_each or it evaluates to null, keep the resource as-is
		if iterations == nil {
			result = append(result, block)
			continue
		}

		if len(iterations) == 0 {
			// Empty for_each - no resources created
			continue
		}

		originalID := block.Type + "." + block.Name
		var expandedIDs []string

		// Create an expanded resource for each iteration
		for key, value := range iterations {
			expandedName := fmt.Sprintf("%s[\"%s\"]", block.Name, key)
			expanded := &config.ResourceBlock{
				Type:        block.Type,
				Name:        expandedName,
				DependsOn:   block.DependsOn, // Will be expanded in second pass
				Body:        block.Body,      // Same body, will be decoded with each context
				RoleBaseDir: block.RoleBaseDir,
				ForEachKey:  key,
			}

			expandedID := block.Type + "." + expandedName
			e.forEachValues[expandedID] = value
			expandedIDs = append(expandedIDs, expandedID)

			result = append(result, expanded)
		}

		e.forEachOriginalNames[originalID] = expandedIDs
	}

	return result, nil
}

// expandForEachDependencies expands references to for_each base names to all their instances
func (e *Executor) expandForEachDependencies(deps []string) []string {
	var result []string
	for _, dep := range deps {
		if expanded, ok := e.forEachOriginalNames[dep]; ok {
			// This dependency references a for_each resource - depend on all instances
			result = append(result, expanded...)
		} else {
			result = append(result, dep)
		}
	}
	return result
}

// mergeDependencies combines explicit and implicit dependencies, removing duplicates
func (e *Executor) mergeDependencies(explicit, implicit []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(explicit)+len(implicit))

	for _, dep := range explicit {
		if !seen[dep] {
			seen[dep] = true
			result = append(result, dep)
		}
	}

	for _, dep := range implicit {
		if !seen[dep] {
			seen[dep] = true
			result = append(result, dep)
		}
	}

	return result
}

// extractResourceAttributes extracts attribute values from a resource block
// that can be referenced by other resources
func (e *Executor) extractResourceAttributes(block *config.ResourceBlock) map[string]cty.Value {
	// Build context with each.key/each.value if this is an expanded for_each resource
	var ctx *hcl.EvalContext
	if block.ForEachKey != "" {
		eachKey := cty.StringVal(block.ForEachKey)
		eachValue := e.forEachValues[block.Type+"."+block.Name]
		if eachValue.IsNull() {
			eachValue = eachKey // Fallback for sets where key == value
		}
		ctx = e.parser.BuildEvalContextWithEach(eachKey, eachValue)
	} else {
		ctx = e.parser.GetEvalContext()
	}

	attrs := make(map[string]cty.Value)

	switch block.Type {
	case "file":
		var cfg config.FileResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["path"] = cty.StringVal(cfg.Path)
			if cfg.Content != nil {
				attrs["content"] = cty.StringVal(*cfg.Content)
			}
			if cfg.Mode != nil {
				attrs["mode"] = cty.StringVal(*cfg.Mode)
			}
			if cfg.Owner != nil {
				attrs["owner"] = cty.StringVal(*cfg.Owner)
			}
			if cfg.Group != nil {
				attrs["group"] = cty.StringVal(*cfg.Group)
			}
		}

	case "directory":
		var cfg config.DirectoryResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["path"] = cty.StringVal(cfg.Path)
			if cfg.Mode != nil {
				attrs["mode"] = cty.StringVal(*cfg.Mode)
			}
			if cfg.Owner != nil {
				attrs["owner"] = cty.StringVal(*cfg.Owner)
			}
			if cfg.Group != nil {
				attrs["group"] = cty.StringVal(*cfg.Group)
			}
		}

	case "exec":
		var cfg config.ExecResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["command"] = cty.StringVal(cfg.Command)
			if cfg.Creates != nil {
				attrs["creates"] = cty.StringVal(*cfg.Creates)
			}
			if cfg.Dir != nil {
				attrs["dir"] = cty.StringVal(*cfg.Dir)
			}
		}

	case "hostname":
		var cfg config.HostnameResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["name"] = cty.StringVal(cfg.Name)
		}

	case "cron":
		var cfg config.CronResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["command"] = cty.StringVal(cfg.Command)
			attrs["schedule"] = cty.StringVal(cfg.Schedule)
			if cfg.User != nil {
				attrs["user"] = cty.StringVal(*cfg.User)
			}
		}

	case "package":
		var cfg config.PackageResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["name"] = cty.StringVal(cfg.Name)
			if cfg.Version != nil {
				attrs["version"] = cty.StringVal(*cfg.Version)
			}
		}

	case "service":
		var cfg config.ServiceResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["name"] = cty.StringVal(cfg.Name)
		}

	case "user":
		var cfg config.UserResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["name"] = cty.StringVal(cfg.Name)
			if cfg.UID != nil {
				attrs["uid"] = cty.StringVal(*cfg.UID)
			}
			if cfg.GID != nil {
				attrs["gid"] = cty.StringVal(*cfg.GID)
			}
			if cfg.Home != nil {
				attrs["home"] = cty.StringVal(*cfg.Home)
			}
			if cfg.Shell != nil {
				attrs["shell"] = cty.StringVal(*cfg.Shell)
			}
		}

	case "group":
		var cfg config.GroupResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["name"] = cty.StringVal(cfg.Name)
			if cfg.GID != nil {
				attrs["gid"] = cty.StringVal(*cfg.GID)
			}
		}

	case "link":
		var cfg config.LinkResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["path"] = cty.StringVal(cfg.Path)
			attrs["target"] = cty.StringVal(cfg.Target)
		}

	case "download":
		var cfg config.DownloadResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["url"] = cty.StringVal(cfg.URL)
			attrs["dest"] = cty.StringVal(cfg.Dest)
			if cfg.Checksum != nil {
				attrs["checksum"] = cty.StringVal(*cfg.Checksum)
			}
			if cfg.Mode != nil {
				attrs["mode"] = cty.StringVal(*cfg.Mode)
			}
			if cfg.Owner != nil {
				attrs["owner"] = cty.StringVal(*cfg.Owner)
			}
			if cfg.Group != nil {
				attrs["group"] = cty.StringVal(*cfg.Group)
			}
		}

	case "stat":
		var cfg config.StatResourceConfig
		if diags := gohcl.DecodeBody(block.Body, ctx, &cfg); !diags.HasErrors() {
			attrs["path"] = cty.StringVal(cfg.Path)

			// Perform stat operation to populate runtime attributes
			follow := true
			if cfg.Follow != nil {
				follow = *cfg.Follow
			}

			var info os.FileInfo
			var statErr error
			if follow {
				info, statErr = os.Stat(cfg.Path)
			} else {
				info, statErr = os.Lstat(cfg.Path)
			}

			if os.IsNotExist(statErr) {
				attrs["exists"] = cty.BoolVal(false)
				attrs["isdir"] = cty.BoolVal(false)
				attrs["isfile"] = cty.BoolVal(false)
				attrs["islink"] = cty.BoolVal(false)
				attrs["size"] = cty.NumberIntVal(0)
				attrs["mode"] = cty.StringVal("")
				attrs["owner"] = cty.StringVal("")
				attrs["group"] = cty.StringVal("")
				attrs["uid"] = cty.NumberIntVal(-1)
				attrs["gid"] = cty.NumberIntVal(-1)
				attrs["mtime"] = cty.NumberIntVal(0)
				attrs["atime"] = cty.NumberIntVal(0)
			} else if statErr == nil {
				attrs["exists"] = cty.BoolVal(true)
				attrs["isdir"] = cty.BoolVal(info.IsDir())
				attrs["isfile"] = cty.BoolVal(info.Mode().IsRegular())
				attrs["size"] = cty.NumberIntVal(info.Size())
				attrs["mode"] = cty.StringVal(fmt.Sprintf("%04o", info.Mode().Perm()))
				attrs["mtime"] = cty.NumberIntVal(info.ModTime().Unix())

				// Check if it's a symlink
				linfo, lerr := os.Lstat(cfg.Path)
				if lerr == nil {
					attrs["islink"] = cty.BoolVal(linfo.Mode()&os.ModeSymlink != 0)
				} else {
					attrs["islink"] = cty.BoolVal(false)
				}

				// Get owner/group info
				if stat, ok := info.Sys().(*syscall.Stat_t); ok {
					attrs["uid"] = cty.NumberIntVal(int64(stat.Uid))
					attrs["gid"] = cty.NumberIntVal(int64(stat.Gid))
					attrs["atime"] = cty.NumberIntVal(getAtime(stat))

					if u, err := user.LookupId(strconv.Itoa(int(stat.Uid))); err == nil {
						attrs["owner"] = cty.StringVal(u.Username)
					} else {
						attrs["owner"] = cty.StringVal(strconv.Itoa(int(stat.Uid)))
					}
					if g, err := user.LookupGroupId(strconv.Itoa(int(stat.Gid))); err == nil {
						attrs["group"] = cty.StringVal(g.Name)
					} else {
						attrs["group"] = cty.StringVal(strconv.Itoa(int(stat.Gid)))
					}
				}
			}
		}
	}

	return attrs
}

// extractImplicitDependencies analyzes HCL expressions in a resource block
// to find references to other resources, returning them as implicit dependencies
func (e *Executor) extractImplicitDependencies(block *config.ResourceBlock) []string {
	deps := make(map[string]bool)

	// Get all attributes from the body
	attrs, _ := block.Body.JustAttributes()

	for _, attr := range attrs {
		// Skip depends_on as it's explicit
		if attr.Name == "depends_on" {
			continue
		}

		// Get all variable references in this expression
		for _, traversal := range attr.Expr.Variables() {
			if len(traversal) >= 2 {
				// Check if the root is a known resource type
				rootName := traversal.RootName()
				if knownResourceTypes[rootName] {
					// This is a resource reference like directory.web_root_dir.path
					// Extract the resource type and name
					if nameStep, ok := traversal[1].(hcl.TraverseAttr); ok {
						resourceRef := rootName + "." + nameStep.Name
						// Don't add self-references
						selfRef := block.Type + "." + block.Name
						if resourceRef != selfRef {
							deps[resourceRef] = true
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result
}

// Plan generates and prints the execution plan
func (e *Executor) Plan(ctx context.Context) (*PlanResult, error) {
	result := &PlanResult{
		Plans: make(map[string]*resource.Plan),
	}

	// Get resources in dependency order
	resources, err := e.graph.TopologicalSort()
	if err != nil {
		return nil, err
	}

	for _, r := range resources {
		current, err := r.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", resource.ID(r), err)
		}

		plan, err := r.Diff(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("failed to diff %s: %w", resource.ID(r), err)
		}

		result.Plans[resource.ID(r)] = plan
		result.Resources = append(result.Resources, r)

		if plan.HasChanges() {
			switch plan.Action {
			case resource.ActionCreate:
				result.ToAdd++
			case resource.ActionUpdate:
				result.ToChange++
			case resource.ActionDelete:
				result.ToDestroy++
			}
		}
	}

	return result, nil
}

// PrintPlan prints the execution plan
func (e *Executor) PrintPlan(result *PlanResult) {
	hasChanges := false

	for _, r := range result.Resources {
		plan := result.Plans[resource.ID(r)]
		if plan.HasChanges() {
			hasChanges = true
			e.printer.PrintPlan(r, plan)
		}
	}

	if !hasChanges {
		e.printer.PrintNoChanges()
		return
	}

	e.printer.PrintSummary(result.ToAdd, result.ToChange, result.ToDestroy)
}

// Apply applies the changes
func (e *Executor) Apply(ctx context.Context, result *PlanResult, dryRun bool) error {
	for _, r := range result.Resources {
		plan := result.Plans[resource.ID(r)]
		if !plan.HasChanges() {
			continue
		}

		if dryRun {
			_, _ = fmt.Fprintf(e.out, "Would %s %s\n", plan.Action, resource.ID(r))
			continue
		}

		_, _ = fmt.Fprintf(e.out, "Applying %s...\n", resource.ID(r))
		if err := r.Apply(ctx, plan, true); err != nil {
			return fmt.Errorf("failed to apply %s: %w", resource.ID(r), err)
		}
		_, _ = fmt.Fprintf(e.out, "  Done.\n")
	}

	return nil
}

// Validate validates the loaded configuration
func (e *Executor) Validate() error {
	for _, r := range e.graph.All() {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return e.graph.Validate()
}

// PlanResult holds the results of a plan operation
type PlanResult struct {
	Resources []resource.Resource
	Plans     map[string]*resource.Plan
	ToAdd     int
	ToChange  int
	ToDestroy int
}

// HasChanges returns true if there are any changes in the plan
func (r *PlanResult) HasChanges() bool {
	return r.ToAdd > 0 || r.ToChange > 0 || r.ToDestroy > 0
}

// FindConfigFile looks for configuration in the following order:
// 1. Specified path (file or directory)
// 2. Current directory (all *.hcl files)
func FindConfigFile(path string) (string, bool, error) {
	if path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return "", false, fmt.Errorf("cannot access %s: %w", path, err)
		}
		return path, info.IsDir(), nil
	}

	// Default to current directory (like Terraform)
	return ".", true, nil
}
