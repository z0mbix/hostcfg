package role

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/zclconf/go-cty/cty"
)

// Loader handles loading roles from directories
type Loader struct {
	mainParser  *config.Parser
	mainBaseDir string
	cliVars     map[string]cty.Value
}

// NewLoader creates a new role loader
func NewLoader(parser *config.Parser, baseDir string, cliVars map[string]cty.Value) *Loader {
	if cliVars == nil {
		cliVars = make(map[string]cty.Value)
	}
	return &Loader{
		mainParser:  parser,
		mainBaseDir: baseDir,
		cliVars:     cliVars,
	}
}

// LoadRole loads a role from its source directory
func (l *Loader) LoadRole(block *config.RoleBlock) (*Role, error) {
	// 1. Resolve source path relative to main config
	roleDir := block.Source
	if !filepath.IsAbs(roleDir) {
		roleDir = filepath.Join(l.mainBaseDir, block.Source)
	}

	absRoleDir, err := filepath.Abs(roleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve role path: %w", err)
	}

	// Verify role directory exists
	info, err := os.Stat(absRoleDir)
	if err != nil {
		return nil, fmt.Errorf("role source not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("role source is not a directory: %s", absRoleDir)
	}

	role := &Role{
		Name:      block.Name,
		Source:    block.Source,
		BaseDir:   absRoleDir,
		Defaults:  make(map[string]cty.Value),
		Variables: make(map[string]cty.Value),
		DependsOn: block.DependsOn,
	}

	// 2. Load defaults from defaults/variables.hcl
	if err := l.loadDefaults(role); err != nil {
		return nil, fmt.Errorf("failed to load role defaults: %w", err)
	}

	// 3. Evaluate instantiation variables
	if block.Variables != nil {
		ctx := l.mainParser.GetEvalContext()
		val, diags := block.Variables.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("failed to evaluate role variables: %s", diags.Error())
		}

		if val.Type().IsObjectType() || val.Type().IsMapType() {
			for k, v := range val.AsValueMap() {
				role.Variables[k] = v
			}
		}
	}

	// 4. Build final variable scope with precedence
	finalVars := role.BuildVariableScope(l.cliVars)

	// 5. Set role context for template path resolution
	l.mainParser.SetRoleContext(absRoleDir)
	defer l.mainParser.ClearRoleContext()

	// Set variables for role parsing
	for name, value := range finalVars {
		l.mainParser.SetVariableValue(name, value)
	}

	// 6. Parse role's HCL files
	resources, err := l.parseRoleResources(absRoleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse role resources: %w", err)
	}

	// 7. Prefix all resource names, transform dependencies, and set role base dir
	for _, res := range resources {
		originalName := res.Name
		res.Name = role.PrefixResourceName(originalName)

		// Transform internal dependencies to use prefixes
		res.DependsOn = l.transformDependencies(res.DependsOn, role.Name)

		// Set the role base directory for template path resolution
		res.RoleBaseDir = absRoleDir
	}

	role.Resources = resources

	return role, nil
}

// loadDefaults loads default variables from defaults/variables.hcl
func (l *Loader) loadDefaults(role *Role) error {
	defaultsPath := filepath.Join(role.BaseDir, "defaults", "variables.hcl")

	// If defaults file doesn't exist, that's okay
	if _, err := os.Stat(defaultsPath); os.IsNotExist(err) {
		return nil
	}

	src, err := os.ReadFile(defaultsPath)
	if err != nil {
		return err
	}

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, defaultsPath)
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse defaults: %s", diags.Error())
	}

	// Decode variable blocks
	type defaultsConfig struct {
		Variables []*config.Variable `hcl:"variable,block"`
	}
	var defaults defaultsConfig
	diags = gohcl.DecodeBody(file.Body, nil, &defaults)
	if diags.HasErrors() {
		return fmt.Errorf("failed to decode defaults: %s", diags.Error())
	}

	// Extract default values
	for _, v := range defaults.Variables {
		if v.Default != nil {
			val, valDiags := v.Default.Value(nil)
			if !valDiags.HasErrors() {
				role.Defaults[v.Name] = val
			}
		}
	}

	return nil
}

// parseRoleResources parses HCL files in the role directory
func (l *Loader) parseRoleResources(roleDir string) ([]*config.ResourceBlock, error) {
	var resources []*config.ResourceBlock

	// Find all .hcl files in role directory (not recursive, excluding defaults/)
	entries, err := os.ReadDir(roleDir)
	if err != nil {
		return nil, err
	}

	parser := hclparse.NewParser()
	var files []*hcl.File
	var allDiags hcl.Diagnostics

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".hcl") {
			continue
		}

		path := filepath.Join(roleDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			allDiags = append(allDiags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   err.Error(),
			})
			continue
		}

		file, diags := parser.ParseHCL(src, path)
		allDiags = append(allDiags, diags...)
		if file != nil {
			files = append(files, file)
		}
	}

	if allDiags.HasErrors() {
		return nil, fmt.Errorf("failed to parse role HCL: %s", allDiags.Error())
	}

	if len(files) == 0 {
		// No HCL files in role directory is okay (might just have files/)
		return resources, nil
	}

	// Merge all file bodies
	body := hcl.MergeFiles(files)

	// Decode resources using main parser's context (with role variables)
	type roleConfig struct {
		Resources []*config.ResourceBlock `hcl:"resource,block"`
	}
	var cfg roleConfig
	ctx := l.mainParser.GetEvalContext()
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode role config: %s", diags.Error())
	}

	return cfg.Resources, nil
}

// transformDependencies prefixes internal dependencies with role name
func (l *Loader) transformDependencies(deps []string, roleName string) []string {
	result := make([]string, 0, len(deps))
	for _, dep := range deps {
		// Skip role-level dependencies (role.xxx) - they'll be expanded later
		if strings.HasPrefix(dep, "role.") {
			result = append(result, dep)
			continue
		}

		// Prefix internal resource dependencies
		parts := strings.SplitN(dep, ".", 2)
		if len(parts) == 2 {
			// Transform type.name to type.rolename_name
			result = append(result, parts[0]+"."+roleName+"_"+parts[1])
		} else {
			result = append(result, dep)
		}
	}
	return result
}
