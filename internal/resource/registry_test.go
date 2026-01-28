package resource

import (
	"context"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/z0mbix/hostcfg/internal/config"
)

// testResource is a simple test resource for registry tests
type testResource struct {
	name      string
	typ       string
	deps      []string
}

func (r *testResource) Type() string                                              { return r.typ }
func (r *testResource) Name() string                                              { return r.name }
func (r *testResource) Read(ctx context.Context) (*State, error)                  { return nil, nil }
func (r *testResource) Diff(ctx context.Context, s *State) (*Plan, error)         { return nil, nil }
func (r *testResource) Apply(ctx context.Context, p *Plan, apply bool) error      { return nil }
func (r *testResource) Validate() error                                           { return nil }
func (r *testResource) Dependencies() []string                                    { return r.deps }

func testFactory(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	return &testResource{
		name: name,
		typ:  "test",
		deps: dependsOn,
	}, nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.factories == nil {
		t.Error("factories map not initialized")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	r.Register("test", testFactory)

	if len(r.factories) != 1 {
		t.Errorf("expected 1 factory, got %d", len(r.factories))
	}
	if _, ok := r.factories["test"]; !ok {
		t.Error("test factory not registered")
	}
}

func TestRegistry_Create(t *testing.T) {
	r := NewRegistry()
	r.Register("test", testFactory)

	block := &config.ResourceBlock{
		Type:      "test",
		Name:      "mytest",
		DependsOn: []string{"other.resource"},
	}

	resource, err := r.Create(block, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if resource.Type() != "test" {
		t.Errorf("expected type 'test', got %q", resource.Type())
	}
	if resource.Name() != "mytest" {
		t.Errorf("expected name 'mytest', got %q", resource.Name())
	}
	if len(resource.Dependencies()) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(resource.Dependencies()))
	}
}

func TestRegistry_Create_UnknownType(t *testing.T) {
	r := NewRegistry()
	r.Register("test", testFactory)

	block := &config.ResourceBlock{
		Type: "unknown",
		Name: "mytest",
	}

	_, err := r.Create(block, nil)
	if err == nil {
		t.Error("expected error for unknown resource type")
	}
}

func TestRegistry_CreateWithDeps(t *testing.T) {
	r := NewRegistry()
	r.Register("test", testFactory)

	block := &config.ResourceBlock{
		Type:      "test",
		Name:      "mytest",
		DependsOn: []string{"explicit.dep"},
	}

	// CreateWithDeps uses the deps parameter instead of block.DependsOn
	mergedDeps := []string{"explicit.dep", "implicit.dep"}
	resource, err := r.CreateWithDeps(block, mergedDeps, nil)
	if err != nil {
		t.Fatalf("CreateWithDeps failed: %v", err)
	}

	deps := resource.Dependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(deps))
	}
}

func TestDefaultRegistry(t *testing.T) {
	// DefaultRegistry should already have file, directory, link registered via init()
	// We can't easily test this without side effects, so just verify it exists
	if DefaultRegistry == nil {
		t.Error("DefaultRegistry is nil")
	}
}

func TestDefaultRegistry_HasBuiltinTypes(t *testing.T) {
	// These types are registered in init() functions
	types := []string{"file", "directory", "link"}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			_, ok := DefaultRegistry.factories[typ]
			if !ok {
				t.Errorf("expected %q to be registered in DefaultRegistry", typ)
			}
		})
	}
}
