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
	parser    *hclparse.Parser
	variables map[string]cty.Value
}

// NewParser creates a new HCL parser
func NewParser() *Parser {
	return &Parser{
		parser:    hclparse.NewParser(),
		variables: make(map[string]cty.Value),
	}
}

// SetVariable sets a variable value for use during parsing
func (p *Parser) SetVariable(name string, value string) {
	p.variables[name] = cty.StringVal(value)
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

	file, diags := p.parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, diags
	}

	return p.decodeConfig(file.Body)
}

// ParseDirectory parses all .hcl files in a directory
func (p *Parser) ParseDirectory(dir string) (*Config, hcl.Diagnostics) {
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

	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(vars),
		},
		Functions: standardFunctions(),
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
