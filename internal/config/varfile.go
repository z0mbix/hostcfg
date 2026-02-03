package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// VarFileLoader handles loading variable files
type VarFileLoader struct {
	parser *hclparse.Parser
}

// NewVarFileLoader creates a new variable file loader
func NewVarFileLoader() *VarFileLoader {
	return &VarFileLoader{
		parser: hclparse.NewParser(),
	}
}

// LoadVarFile loads variables from a single .vars.hcl file
// Returns a map of variable names to their cty.Value
func (l *VarFileLoader) LoadVarFile(path string) (map[string]cty.Value, hcl.Diagnostics) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to read variable file",
			Detail:   err.Error(),
		}}
	}

	file, diags := l.parser.ParseHCL(src, path)
	if diags.HasErrors() {
		return nil, diags
	}

	// Variable files are just attribute assignments at the top level
	attrs, attrDiags := file.Body.JustAttributes()
	diags = append(diags, attrDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	result := make(map[string]cty.Value)
	for name, attr := range attrs {
		val, valDiags := attr.Expr.Value(nil)
		diags = append(diags, valDiags...)
		if !valDiags.HasErrors() {
			result[name] = val
		}
	}

	return result, diags
}

// FindAutoLoadFiles finds all auto-load variable files in a directory
// Returns files in load order: hostcfg.vars.hcl, hostcfg.vars.hcl.local, *.auto.vars.hcl
func (l *VarFileLoader) FindAutoLoadFiles(dir string) ([]string, error) {
	var files []string

	// 1. hostcfg.vars.hcl
	defaultFile := filepath.Join(dir, "hostcfg.vars.hcl")
	if _, err := os.Stat(defaultFile); err == nil {
		files = append(files, defaultFile)
	}

	// 2. hostcfg.vars.hcl.local
	localFile := filepath.Join(dir, "hostcfg.vars.hcl.local")
	if _, err := os.Stat(localFile); err == nil {
		files = append(files, localFile)
	}

	// 3. *.auto.vars.hcl (alphabetically sorted)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files, nil // Return what we have, directory read error is not fatal
	}

	var autoFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".auto.vars.hcl") {
			autoFiles = append(autoFiles, filepath.Join(dir, name))
		}
	}
	sort.Strings(autoFiles)
	files = append(files, autoFiles...)

	return files, nil
}

// LoadMultipleVarFiles loads variables from multiple files in order
// Later files override earlier files for the same variable
func (l *VarFileLoader) LoadMultipleVarFiles(paths []string) (map[string]cty.Value, hcl.Diagnostics) {
	result := make(map[string]cty.Value)
	var allDiags hcl.Diagnostics

	for _, path := range paths {
		vars, diags := l.LoadVarFile(path)
		allDiags = append(allDiags, diags...)
		if !diags.HasErrors() {
			for k, v := range vars {
				result[k] = v
			}
		}
	}

	return result, allDiags
}
