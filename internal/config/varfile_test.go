package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVarFileLoader_LoadVarFile_BasicTypes(t *testing.T) {
	tmpDir := t.TempDir()
	varFile := filepath.Join(tmpDir, "test.vars.hcl")
	content := `
string_var = "hello"
number_var = 42
bool_var   = true
`
	if err := os.WriteFile(varFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write var file: %v", err)
	}

	loader := NewVarFileLoader()
	vars, diags := loader.LoadVarFile(varFile)
	if diags.HasErrors() {
		t.Fatalf("failed to load var file: %s", diags.Error())
	}

	if vars["string_var"].AsString() != "hello" {
		t.Errorf("string_var: expected 'hello', got '%s'", vars["string_var"].AsString())
	}

	num, _ := vars["number_var"].AsBigFloat().Int64()
	if num != 42 {
		t.Errorf("number_var: expected 42, got %d", num)
	}

	if vars["bool_var"].True() != true {
		t.Error("bool_var: expected true")
	}
}

func TestVarFileLoader_LoadVarFile_ComplexTypes(t *testing.T) {
	tmpDir := t.TempDir()
	varFile := filepath.Join(tmpDir, "complex.vars.hcl")
	content := `
hosts = ["web1", "web2", "web3"]
config = {
  debug = true
  port  = 8080
}
`
	if err := os.WriteFile(varFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write var file: %v", err)
	}

	loader := NewVarFileLoader()
	vars, diags := loader.LoadVarFile(varFile)
	if diags.HasErrors() {
		t.Fatalf("failed to load var file: %s", diags.Error())
	}

	// Verify list/tuple
	if !vars["hosts"].Type().IsListType() && !vars["hosts"].Type().IsTupleType() {
		t.Error("hosts should be a list/tuple type")
	}

	hostsList := vars["hosts"].AsValueSlice()
	if len(hostsList) != 3 {
		t.Errorf("hosts: expected 3 elements, got %d", len(hostsList))
	}
	if hostsList[0].AsString() != "web1" {
		t.Errorf("hosts[0]: expected 'web1', got '%s'", hostsList[0].AsString())
	}

	// Verify object
	if !vars["config"].Type().IsObjectType() {
		t.Error("config should be an object type")
	}
}

func TestVarFileLoader_FindAutoLoadFiles_Order(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in random order
	files := []string{
		"z.auto.vars.hcl",
		"hostcfg.vars.hcl",
		"a.auto.vars.hcl",
		"hostcfg.vars.hcl.local",
		"other.hcl", // Should be ignored
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("x = 1"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	loader := NewVarFileLoader()
	found, err := loader.FindAutoLoadFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindAutoLoadFiles failed: %v", err)
	}

	// Expected order: hostcfg.vars.hcl, hostcfg.vars.hcl.local, a.auto.vars.hcl, z.auto.vars.hcl
	expectedOrder := []string{
		"hostcfg.vars.hcl",
		"hostcfg.vars.hcl.local",
		"a.auto.vars.hcl",
		"z.auto.vars.hcl",
	}

	if len(found) != len(expectedOrder) {
		t.Fatalf("expected %d files, got %d", len(expectedOrder), len(found))
	}

	for i, expected := range expectedOrder {
		if filepath.Base(found[i]) != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, filepath.Base(found[i]))
		}
	}
}

func TestVarFileLoader_FindAutoLoadFiles_Partial(t *testing.T) {
	tmpDir := t.TempDir()

	// Only create hostcfg.vars.hcl
	if err := os.WriteFile(filepath.Join(tmpDir, "hostcfg.vars.hcl"), []byte("x = 1"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	loader := NewVarFileLoader()
	found, err := loader.FindAutoLoadFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindAutoLoadFiles failed: %v", err)
	}

	if len(found) != 1 {
		t.Fatalf("expected 1 file, got %d", len(found))
	}

	if filepath.Base(found[0]) != "hostcfg.vars.hcl" {
		t.Errorf("expected hostcfg.vars.hcl, got %s", filepath.Base(found[0]))
	}
}

func TestVarFileLoader_FindAutoLoadFiles_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewVarFileLoader()
	found, err := loader.FindAutoLoadFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindAutoLoadFiles failed: %v", err)
	}

	if len(found) != 0 {
		t.Errorf("expected 0 files, got %d", len(found))
	}
}

func TestVarFileLoader_LoadMultipleVarFiles_Override(t *testing.T) {
	tmpDir := t.TempDir()

	// First file
	file1 := filepath.Join(tmpDir, "base.vars.hcl")
	if err := os.WriteFile(file1, []byte(`
env = "base"
port = 8080
`), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}

	// Second file overrides env
	file2 := filepath.Join(tmpDir, "override.vars.hcl")
	if err := os.WriteFile(file2, []byte(`
env = "production"
`), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	loader := NewVarFileLoader()
	vars, diags := loader.LoadMultipleVarFiles([]string{file1, file2})
	if diags.HasErrors() {
		t.Fatalf("LoadMultipleVarFiles failed: %s", diags.Error())
	}

	if vars["env"].AsString() != "production" {
		t.Errorf("env: expected 'production' (override), got '%s'", vars["env"].AsString())
	}

	num, _ := vars["port"].AsBigFloat().Int64()
	if num != 8080 {
		t.Errorf("port: expected 8080 (from base), got %d", num)
	}
}

func TestVarFileLoader_LoadVarFile_InvalidSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	varFile := filepath.Join(tmpDir, "invalid.vars.hcl")
	content := `
this is not valid HCL {{{
`
	if err := os.WriteFile(varFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write var file: %v", err)
	}

	loader := NewVarFileLoader()
	_, diags := loader.LoadVarFile(varFile)
	if !diags.HasErrors() {
		t.Error("expected error for invalid syntax")
	}
}

func TestVarFileLoader_LoadVarFile_NotFound(t *testing.T) {
	loader := NewVarFileLoader()
	_, diags := loader.LoadVarFile("/nonexistent/path.vars.hcl")
	if !diags.HasErrors() {
		t.Error("expected error for missing file")
	}
}
