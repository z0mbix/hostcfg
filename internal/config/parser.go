package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Parser handles parsing HCL configuration files
type Parser struct {
	parser      *hclparse.Parser
	variables   map[string]cty.Value
	resources   map[string]map[string]cty.Value // type -> name -> attributes
	baseDir     string                          // directory containing HCL files
	roleBaseDir string                          // current role's directory (empty if not in role)
}

// NewParser creates a new HCL parser
func NewParser() *Parser {
	return &Parser{
		parser:    hclparse.NewParser(),
		variables: make(map[string]cty.Value),
		resources: make(map[string]map[string]cty.Value),
	}
}

// SetVariable sets a variable value for use during parsing
func (p *Parser) SetVariable(name string, value string) {
	p.variables[name] = cty.StringVal(value)
}

// SetVariableValue sets a variable with a cty.Value directly
func (p *Parser) SetVariableValue(name string, value cty.Value) {
	p.variables[name] = value
}

// GetBaseDir returns the base directory for the parser
func (p *Parser) GetBaseDir() string {
	return p.baseDir
}

// SetRoleContext sets the base directory for role-relative paths
func (p *Parser) SetRoleContext(roleDir string) {
	p.roleBaseDir = roleDir
}

// ClearRoleContext resets to main config context
func (p *Parser) ClearRoleContext() {
	p.roleBaseDir = ""
}

// getEffectiveBaseDir returns roleBaseDir if set, otherwise baseDir
func (p *Parser) getEffectiveBaseDir() string {
	if p.roleBaseDir != "" {
		return p.roleBaseDir
	}
	return p.baseDir
}

// SetResourceAttributes sets resource attributes for use in expressions
// This allows resources to reference attributes of other resources
func (p *Parser) SetResourceAttributes(resourceType, resourceName string, attrs map[string]cty.Value) {
	if p.resources[resourceType] == nil {
		p.resources[resourceType] = make(map[string]cty.Value)
	}
	p.resources[resourceType][resourceName] = cty.ObjectVal(attrs)
}

// ParseFile parses a single HCL file
func (p *Parser) ParseFile(filename string) (*Config, hcl.Diagnostics) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to read file",
			Detail:   err.Error(),
		}}
	}

	// Set base directory to the directory containing the HCL file
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to resolve file path",
			Detail:   err.Error(),
		}}
	}
	p.baseDir = filepath.Dir(absPath)

	file, diags := p.parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, diags
	}

	return p.decodeConfig(file.Body)
}

// ParseDirectory parses all .hcl files in a directory
func (p *Parser) ParseDirectory(dir string) (*Config, hcl.Diagnostics) {
	// Set base directory for resolving relative paths in templates
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to resolve directory path",
			Detail:   err.Error(),
		}}
	}
	p.baseDir = absDir

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to read directory",
			Detail:   err.Error(),
		}}
	}

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
		// Skip variable files (*.vars.hcl) - these are loaded separately
		if strings.HasSuffix(name, ".vars.hcl") {
			continue
		}

		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			allDiags = append(allDiags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   err.Error(),
			})
			continue
		}

		file, diags := p.parser.ParseHCL(src, path)
		allDiags = append(allDiags, diags...)
		if file != nil {
			files = append(files, file)
		}
	}

	if allDiags.HasErrors() {
		return nil, allDiags
	}

	if len(files) == 0 {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "No HCL files found",
			Detail:   fmt.Sprintf("No .hcl files found in directory: %s", dir),
		}}
	}

	// Merge all file bodies
	body := hcl.MergeFiles(files)
	return p.decodeConfig(body)
}

// decodeConfig decodes the HCL body into a Config struct
func (p *Parser) decodeConfig(body hcl.Body) (*Config, hcl.Diagnostics) {
	var config Config

	// First pass: extract variable definitions and their defaults
	ctx := p.buildEvalContext(nil)
	diags := gohcl.DecodeBody(body, ctx, &config)
	if diags.HasErrors() {
		return nil, diags
	}

	// Process variable defaults
	for _, v := range config.Variables {
		if v.Default != nil {
			val, valDiags := v.Default.Value(ctx)
			if !valDiags.HasErrors() {
				// Only set if not already overridden
				if _, exists := p.variables[v.Name]; !exists {
					p.variables[v.Name] = val
				}
			}
		}
	}

	return &config, nil
}

// buildEvalContext creates the evaluation context for HCL expressions
func (p *Parser) buildEvalContext(extra map[string]cty.Value) *hcl.EvalContext {
	vars := make(map[string]cty.Value)
	for k, v := range p.variables {
		vars[k] = v
	}
	for k, v := range extra {
		vars[k] = v
	}

	// Build the context variables map
	ctxVars := map[string]cty.Value{
		"var": cty.ObjectVal(vars),
	}

	// Add resource references (e.g., directory.web_root_dir.path)
	for resourceType, resources := range p.resources {
		if len(resources) > 0 {
			ctxVars[resourceType] = cty.ObjectVal(resources)
		}
	}

	// Build functions map with template having access to context
	// Use the effective base dir (role's base dir if in role context)
	funcs := standardFunctions()
	funcs["template"] = makeTemplateFunc(ctxVars, p.getEffectiveBaseDir())

	return &hcl.EvalContext{
		Variables: ctxVars,
		Functions: funcs,
	}
}

// GetEvalContext returns the evaluation context for resource parsing
func (p *Parser) GetEvalContext() *hcl.EvalContext {
	return p.buildEvalContext(nil)
}

// DecodeResourceBody decodes a resource body into the given target struct
func (p *Parser) DecodeResourceBody(body hcl.Body, target interface{}) hcl.Diagnostics {
	ctx := p.buildEvalContext(nil)
	return gohcl.DecodeBody(body, ctx, target)
}
