package resource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseFileHCL(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{})
	if diags.HasErrors() {
		t.Fatalf("failed to parse HCL: %v", diags.Error())
	}
	return file.Body
}

func TestFileResource_Type(t *testing.T) {
	body := parseFileHCL(t, `
		path    = "/tmp/test"
		content = "hello"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Type() != "file" {
		t.Errorf("expected type 'file', got %q", r.Type())
	}
}

func TestFileResource_Name(t *testing.T) {
	body := parseFileHCL(t, `
		path    = "/tmp/test"
		content = "hello"
	`)

	r, err := NewFileResource("myname", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Name() != "myname" {
		t.Errorf("expected name 'myname', got %q", r.Name())
	}
}

func TestFileResource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr bool
	}{
		{
			name: "valid with content",
			hcl: `
				path    = "/tmp/test"
				content = "hello"
			`,
			wantErr: false,
		},
		{
			name: "valid with source",
			hcl: `
				path   = "/tmp/test"
				source = "/etc/passwd"
			`,
			wantErr: false,
		},
		{
			name: "missing path",
			hcl: `
				content = "hello"
			`,
			wantErr: true,
		},
		{
			name: "missing content and source",
			hcl: `
				path = "/tmp/test"
			`,
			wantErr: true,
		},
		{
			name: "both content and source",
			hcl: `
				path    = "/tmp/test"
				content = "hello"
				source  = "/etc/passwd"
			`,
			wantErr: true,
		},
		{
			name: "absent without content",
			hcl: `
				path   = "/tmp/test"
				ensure = "absent"
			`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseFileHCL(t, tt.hcl)
			r, err := NewFileResource("test", body, nil, "", nil)
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

func TestFileResource_Dependencies(t *testing.T) {
	body := parseFileHCL(t, `
		path    = "/tmp/test"
		content = "hello"
	`)

	deps := []string{"directory.parent", "package.prereq"}
	r, err := NewFileResource("test", body, deps, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	gotDeps := r.Dependencies()
	if len(gotDeps) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(gotDeps))
	}
}

func TestFileResource_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "hello"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if state.Exists {
		t.Error("expected file to not exist")
	}
}

func TestFileResource_Read_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "existing.txt")

	// Create the file
	content := "test content"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "test content"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !state.Exists {
		t.Error("expected file to exist")
	}
	if state.Attributes["path"] != filePath {
		t.Errorf("path mismatch: got %v", state.Attributes["path"])
	}
	if state.Attributes["content"] != content {
		t.Errorf("content mismatch: got %v", state.Attributes["content"])
	}
	if state.Attributes["mode"] != "0644" {
		t.Errorf("mode mismatch: got %v", state.Attributes["mode"])
	}
}

func TestFileResource_Diff_Create(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "new.txt")

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "new content"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState() // file doesn't exist

	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if plan.Action != ActionCreate {
		t.Errorf("expected ActionCreate, got %v", plan.Action)
	}
	if !plan.HasChanges() {
		t.Error("expected plan to have changes")
	}
}

func TestFileResource_Diff_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "existing.txt")
	content := "unchanged content"

	// Create the file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "unchanged content"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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
	if plan.HasChanges() {
		t.Error("expected no changes")
	}
}

func TestFileResource_Diff_ContentChange(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "existing.txt")

	// Create file with old content
	if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "new content"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

	// Find content change
	var foundContentChange bool
	for _, c := range plan.Changes {
		if c.Attribute == "content" {
			foundContentChange = true
			if c.Old != "old content" {
				t.Errorf("expected old content 'old content', got %v", c.Old)
			}
			if c.New != "new content" {
				t.Errorf("expected new content 'new content', got %v", c.New)
			}
		}
	}
	if !foundContentChange {
		t.Error("content change not in plan")
	}
}

func TestFileResource_Diff_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "to_delete.txt")

	// Create file to delete
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path   = "`+filePath+`"
		ensure = "absent"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

func TestFileResource_Apply_Create(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "created.txt")
	content := "created content"

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "`+content+`"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

	// Verify file was created
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

func TestFileResource_Apply_Update(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updated.txt")

	// Create file with old content
	if err := os.WriteFile(filePath, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "updated"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

	// Verify file was updated
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "updated" {
		t.Errorf("file content mismatch: got %q", string(data))
	}
}

func TestFileResource_Apply_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "deleted.txt")

	// Create file to delete
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	body := parseFileHCL(t, `
		path   = "`+filePath+`"
		ensure = "absent"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestFileResource_Apply_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dryrun.txt")

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "content"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	// Apply with apply=false (dry run)
	if err := r.Apply(ctx, plan, false); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify file was NOT created
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not have been created in dry run")
	}
}

func TestFileResource_Apply_Mode(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "moded.txt")

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "content"
		mode    = "0755"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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

	// Verify mode
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode mismatch: got %04o, want 0755", info.Mode().Perm())
	}
}

func TestFileResource_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "idempotent.txt")
	content := "idempotent content"

	body := parseFileHCL(t, `
		path    = "`+filePath+`"
		content = "`+content+`"
	`)

	r, err := NewFileResource("test", body, nil, "", nil)
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
