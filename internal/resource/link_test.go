package resource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseLinkHCL(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{})
	if diags.HasErrors() {
		t.Fatalf("failed to parse HCL: %v", diags.Error())
	}
	return file.Body
}

func TestLinkResource_Type(t *testing.T) {
	body := parseLinkHCL(t, `
		path   = "/tmp/link"
		target = "/tmp/target"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Type() != "link" {
		t.Errorf("expected type 'link', got %q", r.Type())
	}
}

func TestLinkResource_Name(t *testing.T) {
	body := parseLinkHCL(t, `
		path   = "/tmp/link"
		target = "/tmp/target"
	`)

	r, err := NewLinkResource("mylink", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Name() != "mylink" {
		t.Errorf("expected name 'mylink', got %q", r.Name())
	}
}

func TestLinkResource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr bool
	}{
		{
			name: "valid link",
			hcl: `
				path   = "/tmp/link"
				target = "/tmp/target"
			`,
			wantErr: false,
		},
		{
			name: "missing path",
			hcl: `
				target = "/tmp/target"
			`,
			wantErr: true,
		},
		{
			name: "missing target",
			hcl: `
				path = "/tmp/link"
			`,
			wantErr: true,
		},
		{
			name: "absent with target",
			hcl: `
				path   = "/tmp/link"
				target = "/tmp/target"
				ensure = "absent"
			`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseLinkHCL(t, tt.hcl)
			r, err := NewLinkResource("test", body, nil, "", nil)
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

func TestLinkResource_Dependencies(t *testing.T) {
	body := parseLinkHCL(t, `
		path   = "/tmp/link"
		target = "/tmp/target"
	`)

	deps := []string{"directory.target"}
	r, err := NewLinkResource("test", body, deps, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	gotDeps := r.Dependencies()
	if len(gotDeps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(gotDeps))
	}
}

func TestLinkResource_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "nonexistent")

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "/tmp/target"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if state.Exists {
		t.Error("expected link to not exist")
	}
}

func TestLinkResource_Read_ExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	// Create target directory
	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !state.Exists {
		t.Error("expected link to exist")
	}
	if state.Attributes["path"] != linkPath {
		t.Errorf("path mismatch: got %v", state.Attributes["path"])
	}
	if state.Attributes["target"] != targetPath {
		t.Errorf("target mismatch: got %v", state.Attributes["target"])
	}
	if state.Attributes["is_symlink"] != true {
		t.Error("expected is_symlink to be true")
	}
}

func TestLinkResource_Read_NotASymlink(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")

	// Create a regular file
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+filePath+`"
		target = "/tmp/target"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !state.Exists {
		t.Error("expected path to exist")
	}
	if state.Attributes["is_symlink"] != false {
		t.Error("expected is_symlink to be false")
	}
}

func TestLinkResource_Diff_Create(t *testing.T) {
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "newlink")
	targetPath := filepath.Join(tmpDir, "target")

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

func TestLinkResource_Diff_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

func TestLinkResource_Diff_TargetChange(t *testing.T) {
	tmpDir := t.TempDir()
	oldTarget := filepath.Join(tmpDir, "oldtarget")
	newTarget := filepath.Join(tmpDir, "newtarget")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(oldTarget, 0755); err != nil {
		t.Fatalf("failed to create old target: %v", err)
	}
	if err := os.Mkdir(newTarget, 0755); err != nil {
		t.Fatalf("failed to create new target: %v", err)
	}
	if err := os.Symlink(oldTarget, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+newTarget+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	var foundTargetChange bool
	for _, c := range plan.Changes {
		if c.Attribute == "target" {
			foundTargetChange = true
			if c.Old != oldTarget {
				t.Errorf("expected old target %q, got %v", oldTarget, c.Old)
			}
			if c.New != newTarget {
				t.Errorf("expected new target %q, got %v", newTarget, c.New)
			}
		}
	}
	if !foundTargetChange {
		t.Error("target change not in plan")
	}
}

func TestLinkResource_Diff_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
		ensure = "absent"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

func TestLinkResource_Diff_NotSymlinkWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+filePath+`"
		target = "/tmp/target"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	_, err = r.Diff(ctx, current)
	if err == nil {
		t.Error("expected error when path is not a symlink without force")
	}
}

func TestLinkResource_Diff_NotSymlinkWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+filePath+`"
		target = "/tmp/target"
		force  = true
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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
		t.Fatalf("Diff should succeed with force=true: %v", err)
	}

	if plan.Action != ActionUpdate {
		t.Errorf("expected ActionUpdate, got %v", plan.Action)
	}
}

func TestLinkResource_Apply_Create(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	// Verify symlink was created
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != targetPath {
		t.Errorf("symlink target mismatch: got %q, want %q", target, targetPath)
	}
}

func TestLinkResource_Apply_Update(t *testing.T) {
	tmpDir := t.TempDir()
	oldTarget := filepath.Join(tmpDir, "oldtarget")
	newTarget := filepath.Join(tmpDir, "newtarget")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(oldTarget, 0755); err != nil {
		t.Fatalf("failed to create old target: %v", err)
	}
	if err := os.Mkdir(newTarget, 0755); err != nil {
		t.Fatalf("failed to create new target: %v", err)
	}
	if err := os.Symlink(oldTarget, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+newTarget+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	// Verify symlink was updated
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != newTarget {
		t.Errorf("symlink target mismatch: got %q, want %q", target, newTarget)
	}
}

func TestLinkResource_Apply_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
		ensure = "absent"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	// Verify symlink was deleted
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should have been deleted")
	}
}

func TestLinkResource_Apply_ForceReplaceFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}
	// Create a file where the link should be
	if err := os.WriteFile(linkPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
		force  = true
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	// Verify symlink replaced the file
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if target != targetPath {
		t.Errorf("symlink target mismatch: got %q, want %q", target, targetPath)
	}
}

func TestLinkResource_Apply_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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

	// Verify symlink was NOT created
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should not have been created in dry run")
	}
}

func TestLinkResource_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	if err := os.Mkdir(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	body := parseLinkHCL(t, `
		path   = "`+linkPath+`"
		target = "`+targetPath+`"
	`)

	r, err := NewLinkResource("test", body, nil, "", nil)
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
