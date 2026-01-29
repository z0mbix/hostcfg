package role

import (
	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/zclconf/go-cty/cty"
)

// Role represents a loaded role with its configuration
type Role struct {
	Name      string                   // Instance name (e.g., "redis")
	Source    string                   // Path to role directory (as specified)
	BaseDir   string                   // Absolute path to role directory
	Defaults  map[string]cty.Value     // From defaults/variables.hcl
	Variables map[string]cty.Value     // From instantiation
	Resources []*config.ResourceBlock  // Prefixed resources
	DependsOn []string                 // Role-level dependencies
}

// PrefixResourceName adds role prefix to resource name
func (r *Role) PrefixResourceName(name string) string {
	return r.Name + "_" + name
}

// BuildVariableScope merges variables with precedence:
// 1. Role defaults (lowest)
// 2. Instantiation variables
// 3. CLI variables (highest, if they match role variable names)
func (r *Role) BuildVariableScope(cliVars map[string]cty.Value) map[string]cty.Value {
	result := make(map[string]cty.Value)

	// 1. Role defaults
	for k, v := range r.Defaults {
		result[k] = v
	}

	// 2. Instantiation variables
	for k, v := range r.Variables {
		result[k] = v
	}

	// 3. CLI variables (if they match role variable names)
	for k, v := range cliVars {
		if _, exists := result[k]; exists {
			result[k] = v
		}
	}

	return result
}

// GetResourceIDs returns all resource identifiers in this role (type.name format)
func (r *Role) GetResourceIDs() []string {
	ids := make([]string, 0, len(r.Resources))
	for _, res := range r.Resources {
		ids = append(ids, res.Type+"."+res.Name)
	}
	return ids
}
