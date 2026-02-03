package resource

import (
	"context"
)

// State represents the current state of a resource
type State struct {
	Exists     bool
	Attributes map[string]interface{}
}

// NewState creates a new State with initialized attributes map
func NewState() *State {
	return &State{
		Attributes: make(map[string]interface{}),
	}
}

// Change represents a single attribute change
type Change struct {
	Attribute string
	Old       interface{}
	New       interface{}
}

// Action represents the type of change being made
type Action int

const (
	ActionNoop Action = iota
	ActionCreate
	ActionUpdate
	ActionDelete
	ActionSkip
)

func (a Action) String() string {
	switch a {
	case ActionNoop:
		return "noop"
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionDelete:
		return "delete"
	case ActionSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// Plan represents the planned changes for a resource
type Plan struct {
	Action     Action
	Changes    []Change
	Before     *State
	After      *State
	SkipReason string // Reason for skipping: "when condition false" or "dependency skipped"
}

// HasChanges returns true if there are any changes in the plan
func (p *Plan) HasChanges() bool {
	return p.Action != ActionNoop
}

// Resource is the interface that all resources must implement
type Resource interface {
	// Type returns the resource type (e.g., "file", "directory")
	Type() string

	// Name returns the resource name (the identifier from HCL)
	Name() string

	// Read retrieves the current state of the resource
	Read(ctx context.Context) (*State, error)

	// Diff compares the desired state with the current state
	Diff(ctx context.Context, current *State) (*Plan, error)

	// Apply makes the changes (or performs a dry-run if apply is false)
	Apply(ctx context.Context, plan *Plan, apply bool) error

	// Validate checks that the resource configuration is valid
	Validate() error

	// Dependencies returns the list of resource references this resource depends on
	Dependencies() []string
}

// ID returns the fully qualified resource ID (type.name)
func ID(r Resource) string {
	return r.Type() + "." + r.Name()
}
