package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
	if p.parser == nil {
		t.Error("HCL parser not initialized")
	}
	if p.variables == nil {
		t.Error("variables map not initialized")
	}
	if p.resources == nil {
		t.Error("resources map not initialized")
	}
}

func TestParser_SetVariable(t *testing.T) {
	p := NewParser()
	p.SetVariable("test_var", "test_value")

	if len(p.variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(p.variables))
	}
	if p.variables["test_var"].AsString() != "test_value" {
		t.Errorf("variable value mismatch")
	}
}

func TestParser_SetResourceAttributes(t *testing.T) {
	p := NewParser()
	attrs := map[string]cty.Value{
		"path": cty.StringVal("/tmp/test"),
		"mode": cty.StringVal("0644"),
	}
	p.SetResourceAttributes("file", "myfile", attrs)

	if len(p.resources) != 1 {
		t.Errorf("expected 1 resource type, got %d", len(p.resources))
	}
	if len(p.resources["file"]) != 1 {
		t.Errorf("expected 1 file resource, got %d", len(p.resources["file"]))
	}
}

func TestParser_ParseFile_ValidHCL(t *testing.T) {
	// Create temp file with valid HCL
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	content := `
variable "test" {
  default = "hello"
}

resource "file" "example" {
  path    = "/tmp/test"
  content = "test content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	cfg, diags := p.ParseFile(hclPath)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	if len(cfg.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(cfg.Variables))
	}
	if len(cfg.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(cfg.Resources))
	}
	if cfg.Resources[0].Type != "file" {
		t.Errorf("expected resource type 'file', got %q", cfg.Resources[0].Type)
	}
	if cfg.Resources[0].Name != "example" {
		t.Errorf("expected resource name 'example', got %q", cfg.Resources[0].Name)
	}
}

func TestParser_ParseFile_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "invalid.hcl")
	content := `this is not valid { HCL`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	_, diags := p.ParseFile(hclPath)
	if !diags.HasErrors() {
		t.Error("expected parse error for invalid HCL")
	}
}

func TestParser_ParseFile_NonExistent(t *testing.T) {
	p := NewParser()
	_, diags := p.ParseFile("/nonexistent/file.hcl")
	if !diags.HasErrors() {
		t.Error("expected error for non-existent file")
	}
}

func TestParser_ParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two HCL files
	file1 := `
resource "file" "one" {
  path    = "/tmp/one"
  content = "one"
}
`
	file2 := `
resource "directory" "two" {
  path = "/tmp/two"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "one.hcl"), []byte(file1), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "two.hcl"), []byte(file2), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	// Create a non-HCL file that should be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "ignore.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	p := NewParser()
	cfg, diags := p.ParseDirectory(tmpDir)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	if len(cfg.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(cfg.Resources))
	}
}

func TestParser_ParseDirectory_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	p := NewParser()
	_, diags := p.ParseDirectory(tmpDir)
	if !diags.HasErrors() {
		t.Error("expected error for empty directory")
	}
}

func TestParser_ParseDirectory_NonExistent(t *testing.T) {
	p := NewParser()
	_, diags := p.ParseDirectory("/nonexistent/directory")
	if !diags.HasErrors() {
		t.Error("expected error for non-existent directory")
	}
}

func TestParser_VariableResolution(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	content := `
variable "filename" {
  default = "myfile.txt"
}

resource "file" "example" {
  path    = "/tmp/${var.filename}"
  content = "test"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	cfg, diags := p.ParseFile(hclPath)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	// Check that variable default was captured
	if len(cfg.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(cfg.Variables))
	}
	if cfg.Variables[0].Name != "filename" {
		t.Errorf("expected variable name 'filename', got %q", cfg.Variables[0].Name)
	}
}

func TestParser_VariableOverride(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	content := `
variable "name" {
  default = "default_value"
}

resource "file" "example" {
  path    = "/tmp/test"
  content = var.name
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	// Set variable before parsing (simulates -e flag)
	p.SetVariable("name", "overridden_value")

	cfg, diags := p.ParseFile(hclPath)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	// Verify the override took effect
	if p.variables["name"].AsString() != "overridden_value" {
		t.Error("variable override did not work")
	}
	if len(cfg.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(cfg.Resources))
	}
}

func TestParser_GetEvalContext(t *testing.T) {
	p := NewParser()
	p.SetVariable("myvar", "myvalue")
	p.SetResourceAttributes("file", "myfile", map[string]cty.Value{
		"path": cty.StringVal("/tmp/test"),
	})

	ctx := p.GetEvalContext()

	if ctx == nil {
		t.Fatal("GetEvalContext returned nil")
	}
	if ctx.Variables == nil {
		t.Error("Variables not set in context")
	}
	if ctx.Functions == nil {
		t.Error("Functions not set in context")
	}

	// Check var namespace exists
	if _, ok := ctx.Variables["var"]; !ok {
		t.Error("var namespace not in context")
	}

	// Check resource namespace exists
	if _, ok := ctx.Variables["file"]; !ok {
		t.Error("file namespace not in context")
	}
}

func TestParser_DependsOn(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	content := `
resource "directory" "parent" {
  path = "/tmp/parent"
}

resource "file" "child" {
  path       = "/tmp/parent/child.txt"
  content    = "child content"
  depends_on = ["directory.parent"]
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	cfg, diags := p.ParseFile(hclPath)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	if len(cfg.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(cfg.Resources))
	}

	// Find the file resource
	var fileResource *ResourceBlock
	for _, r := range cfg.Resources {
		if r.Type == "file" && r.Name == "child" {
			fileResource = r
			break
		}
	}

	if fileResource == nil {
		t.Fatal("file resource not found")
	}

	if len(fileResource.DependsOn) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(fileResource.DependsOn))
	}
	if fileResource.DependsOn[0] != "directory.parent" {
		t.Errorf("expected dependency 'directory.parent', got %q", fileResource.DependsOn[0])
	}
}

func TestParser_MultipleResourceTypes(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	content := `
resource "directory" "dir" {
  path = "/tmp/test"
}

resource "file" "file" {
  path    = "/tmp/test/file.txt"
  content = "test"
}

resource "link" "link" {
  path   = "/tmp/link"
  target = "/tmp/test"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewParser()
	cfg, diags := p.ParseFile(hclPath)
	if diags.HasErrors() {
		t.Fatalf("parse failed: %v", diags.Error())
	}

	if len(cfg.Resources) != 3 {
		t.Errorf("expected 3 resources, got %d", len(cfg.Resources))
	}

	types := make(map[string]bool)
	for _, r := range cfg.Resources {
		types[r.Type] = true
	}

	if !types["directory"] || !types["file"] || !types["link"] {
		t.Error("not all resource types found")
	}
}
