package engine

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/z0mbix/hostcfg/internal/diff"
	"github.com/z0mbix/hostcfg/internal/resource"
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
}

// Executor runs the configuration management process
type Executor struct {
	parser    *config.Parser
	graph     *Graph
	printer   *diff.Printer
	out       io.Writer
	useColors bool
}

// NewExecutor creates a new executor
func NewExecutor(out io.Writer, useColors bool) *Executor {
	return &Executor{
		parser:    config.NewParser(),
		graph:     NewGraph(),
		printer:   diff.NewPrinter(out, useColors),
		out:       out,
		useColors: useColors,
	}
}

// SetVariable sets a variable for use during execution
func (e *Executor) SetVariable(name, value string) {
	e.parser.SetVariable(name, value)
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
	// First pass: extract resource attributes so they can be referenced
	// by other resources. We decode each resource to get its attribute values.
	for _, block := range cfg.Resources {
		attrs := e.extractResourceAttributes(block)
		if len(attrs) > 0 {
			e.parser.SetResourceAttributes(block.Type, block.Name, attrs)
		}
	}

	// Second pass: create resources with full context (including resource references)
	ctx := e.parser.GetEvalContext()

	for _, block := range cfg.Resources {
		// Extract implicit dependencies from resource references in expressions
		implicitDeps := e.extractImplicitDependencies(block)

		// Merge explicit depends_on with implicit dependencies
		allDeps := e.mergeDependencies(block.DependsOn, implicitDeps)

		r, err := resource.CreateWithDeps(block, allDeps, ctx)
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
	ctx := e.parser.GetEvalContext()
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
			fmt.Fprintf(e.out, "Would %s %s\n", plan.Action, resource.ID(r))
			continue
		}

		fmt.Fprintf(e.out, "Applying %s...\n", resource.ID(r))
		if err := r.Apply(ctx, plan, true); err != nil {
			return fmt.Errorf("failed to apply %s: %w", resource.ID(r), err)
		}
		fmt.Fprintf(e.out, "  Done.\n")
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
