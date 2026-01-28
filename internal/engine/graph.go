package engine

import (
	"fmt"
	"sort"

	"github.com/z0mbix/hostcfg/internal/resource"
)

// Graph represents a dependency graph of resources
type Graph struct {
	resources map[string]resource.Resource
	edges     map[string][]string // resource ID -> list of dependencies
}

// NewGraph creates a new dependency graph
func NewGraph() *Graph {
	return &Graph{
		resources: make(map[string]resource.Resource),
		edges:     make(map[string][]string),
	}
}

// Add adds a resource to the graph
func (g *Graph) Add(r resource.Resource) {
	id := resource.ID(r)
	g.resources[id] = r
	g.edges[id] = r.Dependencies()
}

// Get returns a resource by ID
func (g *Graph) Get(id string) (resource.Resource, bool) {
	r, ok := g.resources[id]
	return r, ok
}

// All returns all resources
func (g *Graph) All() []resource.Resource {
	result := make([]resource.Resource, 0, len(g.resources))
	for _, r := range g.resources {
		result = append(result, r)
	}
	return result
}

// TopologicalSort returns resources in dependency order (dependencies first)
// Returns an error if there are cycles in the graph
func (g *Graph) TopologicalSort() ([]resource.Resource, error) {
	// Check for cycles first
	if err := g.detectCycles(); err != nil {
		return nil, err
	}

	// Kahn's algorithm for topological sorting
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range g.resources {
		inDegree[id] = 0
	}
	for _, deps := range g.edges {
		for _, dep := range deps {
			if _, exists := g.resources[dep]; exists {
				inDegree[dep]++
			}
		}
	}

	// Wait, that's backwards. We want to process dependencies first.
	// Let's reverse the edges logic

	// Build reverse edges (who depends on whom)
	dependedOnBy := make(map[string][]string)
	for id := range g.resources {
		dependedOnBy[id] = nil
	}
	for id, deps := range g.edges {
		for _, dep := range deps {
			if _, exists := g.resources[dep]; exists {
				dependedOnBy[dep] = append(dependedOnBy[dep], id)
			}
		}
	}

	// Calculate in-degree (number of unprocessed dependencies)
	inDegree = make(map[string]int)
	for id, deps := range g.edges {
		count := 0
		for _, dep := range deps {
			if _, exists := g.resources[dep]; exists {
				count++
			}
		}
		inDegree[id] = count
	}

	// Find all nodes with no dependencies
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort the queue for deterministic ordering
	sort.Strings(queue)

	var result []resource.Resource
	for len(queue) > 0 {
		// Take the first item (deterministic since we sorted)
		id := queue[0]
		queue = queue[1:]

		result = append(result, g.resources[id])

		// Decrease in-degree for all nodes that depend on this one
		for _, dependentID := range dependedOnBy[id] {
			inDegree[dependentID]--
			if inDegree[dependentID] == 0 {
				queue = append(queue, dependentID)
				// Re-sort for deterministic ordering
				sort.Strings(queue)
			}
		}
	}

	// Check if we processed all nodes
	if len(result) != len(g.resources) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return result, nil
}

// detectCycles uses DFS to detect cycles in the graph
func (g *Graph) detectCycles() error {
	// State: 0 = unvisited, 1 = visiting, 2 = visited
	state := make(map[string]int)
	path := make([]string, 0)

	var dfs func(id string) error
	dfs = func(id string) error {
		state[id] = 1 // visiting
		path = append(path, id)

		for _, dep := range g.edges[id] {
			// Skip dependencies that don't exist in the graph
			if _, exists := g.resources[dep]; !exists {
				continue
			}

			if state[dep] == 1 {
				// Found a cycle - build the cycle path
				cycleStart := -1
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}
				cyclePath := append(path[cycleStart:], dep)
				return fmt.Errorf("dependency cycle detected: %s", formatCycle(cyclePath))
			}

			if state[dep] == 0 {
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		state[id] = 2 // visited
		path = path[:len(path)-1]
		return nil
	}

	// Get sorted list of resource IDs for deterministic ordering
	ids := make([]string, 0, len(g.resources))
	for id := range g.resources {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if state[id] == 0 {
			if err := dfs(id); err != nil {
				return err
			}
		}
	}

	return nil
}

func formatCycle(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := path[0]
	for i := 1; i < len(path); i++ {
		result += " -> " + path[i]
	}
	return result
}

// Validate checks that all dependencies exist
func (g *Graph) Validate() error {
	for id, deps := range g.edges {
		for _, dep := range deps {
			if _, exists := g.resources[dep]; !exists {
				return fmt.Errorf("resource %s depends on unknown resource: %s", id, dep)
			}
		}
	}
	return nil
}
