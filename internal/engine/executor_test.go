package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if e == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if e.parser == nil {
		t.Error("parser not initialized")
	}
	if e.graph == nil {
		t.Error("graph not initialized")
	}
	if e.printer == nil {
		t.Error("printer not initialized")
	}
}

func TestExecutor_SetVariable(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	e.SetVariable("test", "value")
	// Variable is set in the parser, we can't directly test it without
	// exposing internals, but we can verify it doesn't panic
}

func TestExecutor_LoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")

	content := `
resource "file" "test" {
  path    = "/tmp/test"
  content = "test content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadFile(hclPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Verify resource was added to graph
	all := e.graph.All()
	if len(all) != 1 {
		t.Errorf("expected 1 resource in graph, got %d", len(all))
	}
}

func TestExecutor_LoadFile_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "invalid.hcl")

	content := `this is not valid { HCL`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadFile(hclPath)
	if err == nil {
		t.Error("expected error for invalid HCL")
	}
}

func TestExecutor_LoadFile_NonExistent(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadFile("/nonexistent/file.hcl")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestExecutor_LoadDirectory(t *testing.T) {
	tmpDir := t.TempDir()

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

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadDirectory failed: %v", err)
	}

	all := e.graph.All()
	if len(all) != 2 {
		t.Errorf("expected 2 resources in graph, got %d", len(all))
	}
}

func TestExecutor_LoadDirectory_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadDirectory(tmpDir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestExecutor_Validate(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")

	content := `
resource "file" "test" {
  path    = "/tmp/test"
  content = "test content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if err := e.LoadFile(hclPath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if err := e.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestExecutor_Validate_InvalidDependency(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")

	content := `
resource "file" "test" {
  path       = "/tmp/test"
  content    = "test content"
  depends_on = ["nonexistent.resource"]
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	err := e.LoadFile(hclPath)
	// The error happens during load when graph is validated
	if err == nil {
		t.Error("expected error for invalid dependency")
	}
}

func TestExecutor_Plan(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	filePath := filepath.Join(tmpDir, "output.txt")

	content := `
resource "file" "test" {
  path    = "` + filePath + `"
  content = "test content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if err := e.LoadFile(hclPath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	ctx := context.Background()
	result, err := e.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if result.ToAdd != 1 {
		t.Errorf("expected 1 to add, got %d", result.ToAdd)
	}
	if !result.HasChanges() {
		t.Error("expected plan to have changes")
	}
}

func TestExecutor_Apply(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	filePath := filepath.Join(tmpDir, "created.txt")

	content := `
resource "file" "test" {
  path    = "` + filePath + `"
  content = "created content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if err := e.LoadFile(hclPath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	ctx := context.Background()
	result, err := e.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if err := e.Apply(ctx, result, false); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != "created content" {
		t.Errorf("file content mismatch: got %q", string(data))
	}
}

func TestExecutor_Apply_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	filePath := filepath.Join(tmpDir, "dryrun.txt")

	content := `
resource "file" "test" {
  path    = "` + filePath + `"
  content = "content"
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if err := e.LoadFile(hclPath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	ctx := context.Background()
	result, err := e.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if err := e.Apply(ctx, result, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify file was NOT created
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not have been created in dry run")
	}
}

func TestExecutor_DependencyOrder(t *testing.T) {
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "test.hcl")
	dirPath := filepath.Join(tmpDir, "testdir")
	filePath := filepath.Join(dirPath, "file.txt")

	content := `
resource "directory" "parent" {
  path = "` + dirPath + `"
}

resource "file" "child" {
  path       = "` + filePath + `"
  content    = "child content"
  depends_on = ["directory.parent"]
}
`
	if err := os.WriteFile(hclPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	if err := e.LoadFile(hclPath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	ctx := context.Background()
	result, err := e.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if err := e.Apply(ctx, result, false); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify both were created
	if _, err := os.Stat(dirPath); err != nil {
		t.Errorf("directory should have been created: %v", err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file should have been created: %v", err)
	}
}

func TestExecutor_MergeDependencies(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf, false)

	tests := []struct {
		name     string
		explicit []string
		implicit []string
		want     []string
	}{
		{
			name:     "no duplicates",
			explicit: []string{"a", "b"},
			implicit: []string{"c", "d"},
			want:     []string{"a", "b", "c", "d"},
		},
		{
			name:     "with duplicates",
			explicit: []string{"a", "b"},
			implicit: []string{"b", "c"},
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "empty explicit",
			explicit: nil,
			implicit: []string{"a", "b"},
			want:     []string{"a", "b"},
		},
		{
			name:     "empty implicit",
			explicit: []string{"a", "b"},
			implicit: nil,
			want:     []string{"a", "b"},
		},
		{
			name:     "both empty",
			explicit: nil,
			implicit: nil,
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.mergeDependencies(tt.explicit, tt.implicit)
			if len(got) != len(tt.want) {
				t.Errorf("mergeDependencies() got %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("mergeDependencies()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		path      string
		wantIsDir bool
		wantErr   bool
	}{
		{
			name: "explicit file path",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				path := filepath.Join(tmpDir, "config.hcl")
				os.WriteFile(path, []byte(""), 0644)
				return path
			},
			path:      "", // set in test
			wantIsDir: false,
			wantErr:   false,
		},
		{
			name: "explicit directory path",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			path:      "", // set in test
			wantIsDir: true,
			wantErr:   false,
		},
		{
			name: "non-existent path",
			setup: func(t *testing.T) string {
				return "/nonexistent/path"
			},
			path:      "/nonexistent/path",
			wantIsDir: false,
			wantErr:   true,
		},
		{
			name: "empty path defaults to current directory",
			setup: func(t *testing.T) string {
				return ""
			},
			path:      "",
			wantIsDir: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			if tt.path == "" && path != "" {
				tt.path = path
			}

			result, isDir, err := FindConfigFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindConfigFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && isDir != tt.wantIsDir {
				t.Errorf("FindConfigFile() isDir = %v, want %v", isDir, tt.wantIsDir)
			}
			if !tt.wantErr && tt.path == "" && result != "." {
				t.Errorf("FindConfigFile() with empty path should return '.', got %q", result)
			}
		})
	}
}

func TestPlanResult_HasChanges(t *testing.T) {
	tests := []struct {
		name      string
		toAdd     int
		toChange  int
		toDestroy int
		want      bool
	}{
		{"no changes", 0, 0, 0, false},
		{"add only", 1, 0, 0, true},
		{"change only", 0, 1, 0, true},
		{"destroy only", 0, 0, 1, true},
		{"all changes", 1, 1, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PlanResult{
				ToAdd:     tt.toAdd,
				ToChange:  tt.toChange,
				ToDestroy: tt.toDestroy,
			}
			if got := r.HasChanges(); got != tt.want {
				t.Errorf("PlanResult.HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
