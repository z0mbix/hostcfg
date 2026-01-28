package engine

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/z0mbix/hostcfg/internal/diff"
	"github.com/z0mbix/hostcfg/internal/resource"
)

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
	ctx := e.parser.GetEvalContext()

	for _, block := range cfg.Resources {
		r, err := resource.Create(block, ctx)
		if err != nil {
			return fmt.Errorf("failed to create resource %s.%s: %w",
				block.Type, block.Name, err)
		}

		// Set depends_on from the block
		if len(block.DependsOn) > 0 {
			// The resource already handles depends_on in its factory
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
// 2. hostcfg.hcl in current directory
// 3. *.hcl files in current directory
func FindConfigFile(path string) (string, bool, error) {
	if path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return "", false, fmt.Errorf("cannot access %s: %w", path, err)
		}
		return path, info.IsDir(), nil
	}

	// Try hostcfg.hcl first
	if info, err := os.Stat("hostcfg.hcl"); err == nil && !info.IsDir() {
		return "hostcfg.hcl", false, nil
	}

	// Try current directory
	return ".", true, nil
}
