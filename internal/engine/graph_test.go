package engine

import (
	"context"
	"testing"

	"github.com/z0mbix/hostcfg/internal/resource"
)

// mockResource is a simple mock for testing the graph
type mockResource struct {
	typ       string
	name      string
	deps      []string
	validated bool
}

func newMockResource(typ, name string, deps []string) *mockResource {
	return &mockResource{
		typ:  typ,
		name: name,
		deps: deps,
	}
}

func (m *mockResource) Type() string                                              { return m.typ }
func (m *mockResource) Name() string                                              { return m.name }
func (m *mockResource) Description() string                                       { return "" }
func (m *mockResource) Read(ctx context.Context) (*resource.State, error)         { return nil, nil }
func (m *mockResource) Diff(ctx context.Context, s *resource.State) (*resource.Plan, error) {
	return nil, nil
}
func (m *mockResource) Apply(ctx context.Context, p *resource.Plan, apply bool) error { return nil }
func (m *mockResource) Validate() error                                           { m.validated = true; return nil }
func (m *mockResource) Dependencies() []string                                    { return m.deps }

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil {
		t.Fatal("NewGraph returned nil")
	}
	if g.resources == nil {
		t.Error("resources map not initialized")
	}
	if g.edges == nil {
		t.Error("edges map not initialized")
	}
}

func TestGraph_Add(t *testing.T) {
	g := NewGraph()
	r := newMockResource("file", "test", nil)

	g.Add(r)

	if len(g.resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(g.resources))
	}
	if _, ok := g.resources["file.test"]; !ok {
		t.Error("resource not added with correct ID")
	}
}

func TestGraph_Get(t *testing.T) {
	g := NewGraph()
	r := newMockResource("file", "test", nil)
	g.Add(r)

	tests := []struct {
		name     string
		id       string
		wantOK   bool
	}{
		{"existing resource", "file.test", true},
		{"non-existing resource", "file.notexist", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := g.Get(tt.id)
			if ok != tt.wantOK {
				t.Errorf("Get(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
			}
			if tt.wantOK && got == nil {
				t.Error("Get returned nil for existing resource")
			}
		})
	}
}

func TestGraph_All(t *testing.T) {
	g := NewGraph()
	g.Add(newMockResource("file", "a", nil))
	g.Add(newMockResource("file", "b", nil))
	g.Add(newMockResource("directory", "c", nil))

	all := g.All()
	if len(all) != 3 {
		t.Errorf("expected 3 resources, got %d", len(all))
	}
}

func TestGraph_TopologicalSort_NoDependencies(t *testing.T) {
	g := NewGraph()
	g.Add(newMockResource("file", "a", nil))
	g.Add(newMockResource("file", "b", nil))
	g.Add(newMockResource("file", "c", nil))

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}
	if len(sorted) != 3 {
		t.Errorf("expected 3 resources, got %d", len(sorted))
	}
}

func TestGraph_TopologicalSort_LinearDependencies(t *testing.T) {
	g := NewGraph()
	// c depends on b, b depends on a
	g.Add(newMockResource("file", "a", nil))
	g.Add(newMockResource("file", "b", []string{"file.a"}))
	g.Add(newMockResource("file", "c", []string{"file.b"}))

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	// Verify order: a before b, b before c
	positions := make(map[string]int)
	for i, r := range sorted {
		positions[resource.ID(r)] = i
	}

	if positions["file.a"] >= positions["file.b"] {
		t.Error("file.a should come before file.b")
	}
	if positions["file.b"] >= positions["file.c"] {
		t.Error("file.b should come before file.c")
	}
}

func TestGraph_TopologicalSort_DiamondDependencies(t *testing.T) {
	g := NewGraph()
	// Diamond: d depends on b and c, both depend on a
	g.Add(newMockResource("file", "a", nil))
	g.Add(newMockResource("file", "b", []string{"file.a"}))
	g.Add(newMockResource("file", "c", []string{"file.a"}))
	g.Add(newMockResource("file", "d", []string{"file.b", "file.c"}))

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	positions := make(map[string]int)
	for i, r := range sorted {
		positions[resource.ID(r)] = i
	}

	// a must come first
	if positions["file.a"] != 0 {
		t.Error("file.a should be first")
	}
	// d must come last
	if positions["file.d"] != 3 {
		t.Error("file.d should be last")
	}
	// b and c must come after a and before d
	if positions["file.b"] <= positions["file.a"] {
		t.Error("file.b should come after file.a")
	}
	if positions["file.c"] <= positions["file.a"] {
		t.Error("file.c should come after file.a")
	}
}

func TestGraph_TopologicalSort_Cycle(t *testing.T) {
	g := NewGraph()
	// Simple cycle: a -> b -> c -> a
	g.Add(newMockResource("file", "a", []string{"file.c"}))
	g.Add(newMockResource("file", "b", []string{"file.a"}))
	g.Add(newMockResource("file", "c", []string{"file.b"}))

	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestGraph_TopologicalSort_SelfReference(t *testing.T) {
	g := NewGraph()
	// Self-referencing resource
	g.Add(newMockResource("file", "a", []string{"file.a"}))

	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("expected cycle detection error for self-reference")
	}
}

func TestGraph_Validate_ValidDependencies(t *testing.T) {
	g := NewGraph()
	g.Add(newMockResource("directory", "parent", nil))
	g.Add(newMockResource("file", "child", []string{"directory.parent"}))

	err := g.Validate()
	if err != nil {
		t.Errorf("Validate should pass: %v", err)
	}
}

func TestGraph_Validate_MissingDependency(t *testing.T) {
	g := NewGraph()
	g.Add(newMockResource("file", "child", []string{"directory.nonexistent"}))

	err := g.Validate()
	if err == nil {
		t.Error("expected error for missing dependency")
	}
}

func TestGraph_Deterministic(t *testing.T) {
	// Run the same test multiple times to verify deterministic ordering
	for i := 0; i < 10; i++ {
		g := NewGraph()
		g.Add(newMockResource("file", "z", nil))
		g.Add(newMockResource("file", "a", nil))
		g.Add(newMockResource("file", "m", nil))

		sorted, err := g.TopologicalSort()
		if err != nil {
			t.Fatalf("TopologicalSort failed: %v", err)
		}

		// With no dependencies, resources should be sorted alphabetically
		expected := []string{"file.a", "file.m", "file.z"}
		for j, r := range sorted {
			if resource.ID(r) != expected[j] {
				t.Errorf("iteration %d: position %d: got %s, want %s", i, j, resource.ID(r), expected[j])
			}
		}
	}
}
