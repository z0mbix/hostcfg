package resource

import (
	"context"
	"testing"
)

func TestNewState(t *testing.T) {
	s := NewState()
	if s == nil {
		t.Fatal("NewState returned nil")
	}
	if s.Exists {
		t.Error("new state should not exist by default")
	}
	if s.Attributes == nil {
		t.Error("Attributes map not initialized")
	}
}

func TestAction_String(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{ActionNoop, "noop"},
		{ActionCreate, "create"},
		{ActionUpdate, "update"},
		{ActionDelete, "delete"},
		{Action(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.action.String(); got != tt.want {
				t.Errorf("Action.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlan_HasChanges(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{"noop has no changes", ActionNoop, false},
		{"create has changes", ActionCreate, true},
		{"update has changes", ActionUpdate, true},
		{"delete has changes", ActionDelete, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plan{Action: tt.action}
			if got := p.HasChanges(); got != tt.want {
				t.Errorf("Plan.HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockResourceForID struct {
	typ  string
	name string
}

func (m *mockResourceForID) Type() string                                              { return m.typ }
func (m *mockResourceForID) Name() string                                              { return m.name }
func (m *mockResourceForID) Read(ctx context.Context) (*State, error)                  { return nil, nil }
func (m *mockResourceForID) Diff(ctx context.Context, s *State) (*Plan, error)         { return nil, nil }
func (m *mockResourceForID) Apply(ctx context.Context, p *Plan, apply bool) error      { return nil }
func (m *mockResourceForID) Validate() error                                           { return nil }
func (m *mockResourceForID) Dependencies() []string                                    { return nil }

func TestID(t *testing.T) {
	tests := []struct {
		typ  string
		name string
		want string
	}{
		{"file", "myfile", "file.myfile"},
		{"directory", "mydir", "directory.mydir"},
		{"service", "nginx", "service.nginx"},
		{"package", "vim", "package.vim"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			r := &mockResourceForID{typ: tt.typ, name: tt.name}
			if got := ID(r); got != tt.want {
				t.Errorf("ID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChange(t *testing.T) {
	c := Change{
		Attribute: "mode",
		Old:       "0644",
		New:       "0755",
	}

	if c.Attribute != "mode" {
		t.Errorf("expected attribute 'mode', got %q", c.Attribute)
	}
	if c.Old != "0644" {
		t.Errorf("expected old '0644', got %v", c.Old)
	}
	if c.New != "0755" {
		t.Errorf("expected new '0755', got %v", c.New)
	}
}

func TestState_Attributes(t *testing.T) {
	s := NewState()
	s.Exists = true
	s.Attributes["path"] = "/tmp/test"
	s.Attributes["mode"] = "0644"
	s.Attributes["owner"] = "root"

	if s.Attributes["path"] != "/tmp/test" {
		t.Error("path attribute mismatch")
	}
	if s.Attributes["mode"] != "0644" {
		t.Error("mode attribute mismatch")
	}
	if s.Attributes["owner"] != "root" {
		t.Error("owner attribute mismatch")
	}
}
