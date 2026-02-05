package resource

import (
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/z0mbix/hostcfg/internal/config"
)

// Factory is a function that creates a new resource from an HCL body
type Factory func(name string, body hcl.Body, dependsOn []string, description string, ctx *hcl.EvalContext) (Resource, error)

// Registry holds registered resource types and their factories
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new resource registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a resource factory to the registry
func (r *Registry) Register(resourceType string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[resourceType] = factory
}

// evaluateDescription evaluates the description expression and returns the result as a string.
// Returns empty string if description is nil or evaluation fails.
func evaluateDescription(desc hcl.Expression, ctx *hcl.EvalContext) string {
	if desc == nil {
		return ""
	}
	val, diags := desc.Value(ctx)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

// Create creates a resource from an HCL resource block
func (r *Registry) Create(block *config.ResourceBlock, ctx *hcl.EvalContext) (Resource, error) {
	r.mu.RLock()
	factory, ok := r.factories[block.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", block.Type)
	}

	description := evaluateDescription(block.Description, ctx)
	return factory(block.Name, block.Body, block.DependsOn, description, ctx)
}

// DefaultRegistry is the global default registry
var DefaultRegistry = NewRegistry()

// Register registers a resource factory with the default registry
func Register(resourceType string, factory Factory) {
	DefaultRegistry.Register(resourceType, factory)
}

// Create creates a resource using the default registry
func Create(block *config.ResourceBlock, ctx *hcl.EvalContext) (Resource, error) {
	return DefaultRegistry.Create(block, ctx)
}

// CreateWithDeps creates a resource with explicit dependencies (merging implicit and explicit)
func (r *Registry) CreateWithDeps(block *config.ResourceBlock, deps []string, ctx *hcl.EvalContext) (Resource, error) {
	r.mu.RLock()
	factory, ok := r.factories[block.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", block.Type)
	}

	description := evaluateDescription(block.Description, ctx)
	return factory(block.Name, block.Body, deps, description, ctx)
}

// CreateWithDeps creates a resource with explicit dependencies using the default registry
func CreateWithDeps(block *config.ResourceBlock, deps []string, ctx *hcl.EvalContext) (Resource, error) {
	return DefaultRegistry.CreateWithDeps(block, deps, ctx)
}
