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
	parser        *hclparse.Parser
	variables     map[string]cty.Value
	variableTypes map[string]cty.Type            // variable type constraints
	resources     map[string]map[string]cty.Value // type -> name -> attributes
	baseDir       string                          // directory containing HCL files
	roleBaseDir   string                          // current role's directory (empty if not in role)
	facts         cty.Value                       // system facts for use in expressions
}

// NewParser creates a new HCL parser
func NewParser() *Parser {
	return &Parser{
		parser:        hclparse.NewParser(),
		variables:     make(map[string]cty.Value),
		variableTypes: make(map[string]cty.Type),
		resources:     make(map[string]map[string]cty.Value),
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

// SetVariableType sets a type constraint for a variable
func (p *Parser) SetVariableType(name string, ty cty.Type) {
	p.variableTypes[name] = ty
}

// GetVariableTypes returns a copy of the variable type constraints map
func (p *Parser) GetVariableTypes() map[string]cty.Type {
	result := make(map[string]cty.Type, len(p.variableTypes))
	for k, v := range p.variableTypes {
		result[k] = v
	}
	return result
}

// ValidateAndSetVariable validates a value against its type constraint and sets it.
// If no type constraint exists, the value is set as-is.
func (p *Parser) ValidateAndSetVariable(name string, value cty.Value) hcl.Diagnostics {
	if constraint, hasType := p.variableTypes[name]; hasType {
		validated, diags := ValidateValue(value, constraint, name, nil)
		if diags.HasErrors() {
			return diags
		}
		p.variables[name] = validated
		return nil
	}
	p.variables[name] = value
	return nil
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

// SetFacts sets the system facts for use during parsing
func (p *Parser) SetFacts(facts cty.Value) {
	p.facts = facts
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

	// Parse type constraints for all variables
	for _, v := range config.Variables {
		if v.TypeExpr != nil {
			ty, typeDiags := ParseTypeConstraint(v.TypeExpr)
			diags = append(diags, typeDiags...)
			if !typeDiags.HasErrors() {
				p.variableTypes[v.Name] = ty
			}
		}
	}

	// Return early if type parsing failed
	if diags.HasErrors() {
		return nil, diags
	}

	// Validate and coerce externally-set variables (from CLI/var files) against type constraints
	for name, constraint := range p.variableTypes {
		if existingVal, exists := p.variables[name]; exists {
			// For string values (from CLI -e flag), attempt type coercion
			if existingVal.Type() == cty.String && constraint != cty.String && constraint != cty.DynamicPseudoType {
				strVal := existingVal.AsString()
				coerced, err := CoerceStringValue(strVal, constraint)
				if err != nil {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("Invalid value for variable %q", name),
						Detail:   fmt.Sprintf("Cannot convert %q to %s: %s", strVal, constraint.FriendlyName(), err.Error()),
					})
					continue
				}
				p.variables[name] = coerced
			} else {
				// Validate non-string values against type constraint
				validated, validateDiags := ValidateValue(existingVal, constraint, name, nil)
				diags = append(diags, validateDiags...)
				if !validateDiags.HasErrors() {
					p.variables[name] = validated
				}
			}
		}
	}

	// Return early if validation failed
	if diags.HasErrors() {
		return nil, diags
	}

	// Process variable defaults with type validation
	for _, v := range config.Variables {
		if v.Default != nil {
			val, valDiags := v.Default.Value(ctx)
			diags = append(diags, valDiags...)
			if valDiags.HasErrors() {
				continue
			}

			// Validate against type constraint if one exists
			if constraint, hasType := p.variableTypes[v.Name]; hasType {
				declRange := v.TypeExpr.Range()
				validated, validateDiags := ValidateValue(val, constraint, v.Name, &declRange)
				diags = append(diags, validateDiags...)
				if validateDiags.HasErrors() {
					continue
				}
				val = validated
			}

			// Only set if not already overridden
			if _, exists := p.variables[v.Name]; !exists {
				p.variables[v.Name] = val
			}
		}
	}

	return &config, diags
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

	// Add system facts (e.g., fact.os.family, fact.hostname)
	if p.facts != cty.NilVal && !p.facts.IsNull() {
		ctxVars["fact"] = p.facts
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

// BuildEvalContextWithEach creates an eval context that includes the "each" variable
// for use when evaluating resource bodies during for_each expansion
func (p *Parser) BuildEvalContextWithEach(eachKey, eachValue cty.Value) *hcl.EvalContext {
	// Start with the base context
	ctx := p.buildEvalContext(nil)

	// Add the "each" object with key and value
	ctx.Variables["each"] = cty.ObjectVal(map[string]cty.Value{
		"key":   eachKey,
		"value": eachValue,
	})

	return ctx
}

// EvaluateWhen evaluates the when expression and returns whether the resource should execute.
// Returns: shouldExecute, failedConditionDescription, error
// - Handles nil expression (returns true, "", nil)
// - Evaluates tuple/list of bools
// - Returns false with description if any condition is false
func (p *Parser) EvaluateWhen(expr hcl.Expression, ctx *hcl.EvalContext) (bool, string, error) {
	if expr == nil {
		return true, "", nil
	}

	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return false, "", fmt.Errorf("failed to evaluate when expression: %s", diags.Error())
	}

	// If the expression evaluates to null, treat it as "no when"
	if val.IsNull() {
		return true, "", nil
	}

	if !val.IsKnown() {
		return false, "", fmt.Errorf("when expression must be known")
	}

	// The when expression should be a tuple/list of booleans
	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		// Single boolean value
		if val.Type() == cty.Bool {
			if val.True() {
				return true, "", nil
			}
			return false, "condition => false", nil
		}
		return false, "", fmt.Errorf("when must be a list of boolean expressions, got %s", val.Type().FriendlyName())
	}

	// Iterate through all conditions
	it := val.ElementIterator()
	conditionIndex := 0
	for it.Next() {
		_, elem := it.Element()

		if elem.Type() != cty.Bool {
			return false, "", fmt.Errorf("when condition[%d] must be a boolean, got %s", conditionIndex, elem.Type().FriendlyName())
		}

		if !elem.True() {
			return false, fmt.Sprintf("condition[%d] => false", conditionIndex), nil
		}
		conditionIndex++
	}

	return true, "", nil
}

// EvaluateForEach evaluates the for_each expression and returns the iteration items
// Returns nil if there is no for_each expression or if it evaluates to null
// For sets: returns map where key == value
// For maps: returns the map directly
func (p *Parser) EvaluateForEach(expr hcl.Expression) (map[string]cty.Value, hcl.Diagnostics) {
	if expr == nil {
		return nil, nil
	}

	ctx := p.GetEvalContext()
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	// If the expression evaluates to null, treat it as "no for_each"
	// This can happen when for_each is not specified in the HCL
	if val.IsNull() {
		return nil, nil
	}

	if !val.IsKnown() {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid for_each value",
			Detail:   "for_each value must be known",
		}}
	}

	result := make(map[string]cty.Value)

	switch {
	case val.Type().IsSetType():
		// For sets: key == value
		it := val.ElementIterator()
		for it.Next() {
			_, elem := it.Element()
			// Sets in for_each must have string elements
			if elem.Type() != cty.String {
				return nil, hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  "Invalid for_each set element",
					Detail:   "for_each set elements must be strings",
				}}
			}
			key := elem.AsString()
			result[key] = elem
		}

	case val.Type().IsMapType() || val.Type().IsObjectType():
		// For maps: key is map key, value is map value
		for k, v := range val.AsValueMap() {
			result[k] = v
		}

	default:
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid for_each type",
			Detail:   fmt.Sprintf("for_each must be a set or map, got %s", val.Type().FriendlyName()),
		}}
	}

	return result, nil
}
