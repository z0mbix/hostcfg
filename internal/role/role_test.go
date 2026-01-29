package role

import (
	"testing"

	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/zclconf/go-cty/cty"
)

func TestRole_PrefixResourceName(t *testing.T) {
	r := &Role{Name: "redis"}

	tests := []struct {
		name string
		want string
	}{
		{"config", "redis_config"},
		{"service", "redis_service"},
		{"myfile", "redis_myfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.PrefixResourceName(tt.name)
			if got != tt.want {
				t.Errorf("PrefixResourceName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRole_BuildVariableScope_Defaults(t *testing.T) {
	r := &Role{
		Name: "redis",
		Defaults: map[string]cty.Value{
			"port":      cty.NumberIntVal(6379),
			"maxmemory": cty.StringVal("256mb"),
		},
		Variables: make(map[string]cty.Value),
	}

	result := r.BuildVariableScope(nil)

	if !result["port"].Equals(cty.NumberIntVal(6379)).True() {
		t.Errorf("expected port=6379, got %v", result["port"])
	}
	if result["maxmemory"].AsString() != "256mb" {
		t.Errorf("expected maxmemory=256mb, got %v", result["maxmemory"])
	}
}

func TestRole_BuildVariableScope_InstantiationOverridesDefaults(t *testing.T) {
	r := &Role{
		Name: "redis",
		Defaults: map[string]cty.Value{
			"port":      cty.NumberIntVal(6379),
			"maxmemory": cty.StringVal("256mb"),
		},
		Variables: map[string]cty.Value{
			"port": cty.NumberIntVal(6380),
		},
	}

	result := r.BuildVariableScope(nil)

	if !result["port"].Equals(cty.NumberIntVal(6380)).True() {
		t.Errorf("expected port=6380, got %v", result["port"])
	}
	if result["maxmemory"].AsString() != "256mb" {
		t.Errorf("expected maxmemory=256mb, got %v", result["maxmemory"])
	}
}

func TestRole_BuildVariableScope_CLIOverridesAll(t *testing.T) {
	r := &Role{
		Name: "redis",
		Defaults: map[string]cty.Value{
			"port":      cty.NumberIntVal(6379),
			"maxmemory": cty.StringVal("256mb"),
		},
		Variables: map[string]cty.Value{
			"port": cty.NumberIntVal(6380),
		},
	}

	cliVars := map[string]cty.Value{
		"port":     cty.NumberIntVal(6381),
		"unrelated": cty.StringVal("ignored"), // not in role vars, should be ignored
	}

	result := r.BuildVariableScope(cliVars)

	if !result["port"].Equals(cty.NumberIntVal(6381)).True() {
		t.Errorf("expected port=6381, got %v", result["port"])
	}
	if result["maxmemory"].AsString() != "256mb" {
		t.Errorf("expected maxmemory=256mb, got %v", result["maxmemory"])
	}
	if _, exists := result["unrelated"]; exists {
		t.Error("unrelated CLI var should not be in result")
	}
}

func TestRole_BuildVariableScope_EmptyDefaults(t *testing.T) {
	r := &Role{
		Name:      "redis",
		Defaults:  make(map[string]cty.Value),
		Variables: make(map[string]cty.Value),
	}

	result := r.BuildVariableScope(nil)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestRole_GetResourceIDs(t *testing.T) {
	r := &Role{
		Name: "redis",
		Resources: []*config.ResourceBlock{
			{Type: "package", Name: "redis_redis"},
			{Type: "file", Name: "redis_config"},
			{Type: "service", Name: "redis_redis"},
		},
	}

	ids := r.GetResourceIDs()

	expected := []string{"package.redis_redis", "file.redis_config", "service.redis_redis"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ID[%d] = %q, want %q", i, id, expected[i])
		}
	}
}

func TestRole_GetResourceIDs_Empty(t *testing.T) {
	r := &Role{
		Name:      "empty",
		Resources: nil,
	}

	ids := r.GetResourceIDs()

	if len(ids) != 0 {
		t.Errorf("expected empty IDs, got %d", len(ids))
	}
}
