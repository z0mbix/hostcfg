package resource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseDirHCL(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{})
	if diags.HasErrors() {
		t.Fatalf("failed to parse HCL: %v", diags.Error())
	}
	return file.Body
}

func TestDirectoryResource_Type(t *testing.T) {
	body := parseDirHCL(t, `path = "/tmp/test"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Type() != "directory" {
		t.Errorf("expected type 'directory', got %q", r.Type())
	}
}

func TestDirectoryResource_Name(t *testing.T) {
	body := parseDirHCL(t, `path = "/tmp/test"`)

	r, err := NewDirectoryResource("mydir", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Name() != "mydir" {
		t.Errorf("expected name 'mydir', got %q", r.Name())
	}
}

func TestDirectoryResource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr bool
	}{
		{
			name:    "valid with path",
			hcl:     `path = "/tmp/test"`,
			wantErr: false,
		},
		{
			name:    "missing path",
			hcl:     `mode = "0755"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseDirHCL(t, tt.hcl)
			r, err := NewDirectoryResource("test", body, nil, nil)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("failed to create resource: %v", err)
				}
				return
			}

			err = r.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDirectoryResource_Dependencies(t *testing.T) {
	body := parseDirHCL(t, `path = "/tmp/test"`)

	deps := []string{"package.prereq"}
	r, err := NewDirectoryResource("test", body, deps, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	gotDeps := r.Dependencies()
	if len(gotDeps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(gotDeps))
	}
	if gotDeps[0] != "package.prereq" {
		t.Errorf("expected dependency 'package.prereq', got %q", gotDeps[0])
	}
}

func TestDirectoryResource_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "nonexistent")

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if state.Exists {
		t.Error("expected directory to not exist")
	}
}

func TestDirectoryResource_Read_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "existing")

	// Create the directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !state.Exists {
		t.Error("expected directory to exist")
	}
	if state.Attributes["path"] != dirPath {
		t.Errorf("path mismatch: got %v", state.Attributes["path"])
	}
	if state.Attributes["mode"] != "0755" {
		t.Errorf("mode mismatch: got %v", state.Attributes["mode"])
	}
}

func TestDirectoryResource_Read_NotADirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")

	// Create a file (not a directory)
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseDirHCL(t, `path = "`+filePath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	_, err = r.Read(ctx)
	if err == nil {
		t.Error("expected error when path is a file, not directory")
	}
}

func TestDirectoryResource_Diff_Create(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "new")

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if plan.Action != ActionCreate {
		t.Errorf("expected ActionCreate, got %v", plan.Action)
	}
}

func TestDirectoryResource_Diff_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "existing")

	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if plan.Action != ActionNoop {
		t.Errorf("expected ActionNoop, got %v", plan.Action)
	}
}

func TestDirectoryResource_Diff_ModeChange(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "existing")

	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	body := parseDirHCL(t, `
		path = "`+dirPath+`"
		mode = "0700"
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if plan.Action != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %v", plan.Action)
	}

	var foundModeChange bool
	for _, c := range plan.Changes {
		if c.Attribute == "mode" {
			foundModeChange = true
			if c.New != "0700" {
				t.Errorf("expected new mode '0700', got %v", c.New)
			}
		}
	}
	if !foundModeChange {
		t.Error("mode change not in plan")
	}
}

func TestDirectoryResource_Diff_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "to_delete")

	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	body := parseDirHCL(t, `
		path   = "`+dirPath+`"
		ensure = "absent"
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if plan.Action != ActionDelete {
		t.Errorf("expected ActionDelete, got %v", plan.Action)
	}
}

func TestDirectoryResource_Apply_Create(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "created")

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("failed to stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestDirectoryResource_Apply_CreateRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "parent", "child", "grandchild")

	body := parseDirHCL(t, `
		path      = "`+dirPath+`"
		recursive = true
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("failed to stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestDirectoryResource_Apply_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "deleted")

	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	body := parseDirHCL(t, `
		path   = "`+dirPath+`"
		ensure = "absent"
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestDirectoryResource_Apply_DeleteRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "deleted")
	childDir := filepath.Join(dirPath, "child")
	childFile := filepath.Join(dirPath, "file.txt")

	// Create directory with contents
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(childFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseDirHCL(t, `
		path      = "`+dirPath+`"
		ensure    = "absent"
		recursive = true
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("directory should have been deleted")
	}
}

func TestDirectoryResource_Apply_Mode(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "moded")

	body := parseDirHCL(t, `
		path = "`+dirPath+`"
		mode = "0700"
	`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, true); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("failed to stat directory: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode mismatch: got %04o, want 0700", info.Mode().Perm())
	}
}

func TestDirectoryResource_Apply_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "dryrun")

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if err := r.Apply(ctx, plan, false); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("directory should not have been created in dry run")
	}
}

func TestDirectoryResource_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "idempotent")

	body := parseDirHCL(t, `path = "`+dirPath+`"`)

	r, err := NewDirectoryResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()

	// First apply
	current1 := NewState()
	plan1, _ := r.Diff(ctx, current1)
	if plan1.Action != ActionCreate {
		t.Error("first run should create")
	}
	if err := r.Apply(ctx, plan1, true); err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	// Second apply (should be no-op)
	current2, _ := r.Read(ctx)
	plan2, _ := r.Diff(ctx, current2)
	if plan2.Action != ActionNoop {
		t.Errorf("second run should be noop, got %v", plan2.Action)
	}
}
