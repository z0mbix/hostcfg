package resource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseStatHCL(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{})
	if diags.HasErrors() {
		t.Fatalf("failed to parse HCL: %v", diags.Error())
	}
	return file.Body
}

func TestStatResource_Type(t *testing.T) {
	body := parseStatHCL(t, `
		path = "/tmp/test"
	`)

	r, err := NewStatResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Type() != "stat" {
		t.Errorf("expected type 'stat', got %q", r.Type())
	}
}

func TestStatResource_Name(t *testing.T) {
	body := parseStatHCL(t, `
		path = "/tmp/test"
	`)

	r, err := NewStatResource("mystat", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Name() != "mystat" {
		t.Errorf("expected name 'mystat', got %q", r.Name())
	}
}

func TestStatResource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr bool
	}{
		{
			name: "valid stat",
			hcl: `
				path = "/tmp/test"
			`,
			wantErr: false,
		},
		{
			name: "valid stat with follow",
			hcl: `
				path   = "/tmp/test"
				follow = false
			`,
			wantErr: false,
		},
		{
			name:    "missing path",
			hcl:     ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseStatHCL(t, tt.hcl)
			r, err := NewStatResource("test", body, nil, nil)
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

func TestStatResource_Dependencies(t *testing.T) {
	body := parseStatHCL(t, `
		path = "/tmp/test"
	`)

	deps := []string{"file.config"}
	r, err := NewStatResource("test", body, deps, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	gotDeps := r.Dependencies()
	if len(gotDeps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(gotDeps))
	}
}

func TestStatResource_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent")

	body := parseStatHCL(t, `
		path = "`+path+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if state.Exists {
		t.Error("expected path to not exist")
	}
	if state.Attributes["exists"] != false {
		t.Error("expected exists attribute to be false")
	}
	if state.Attributes["path"] != path {
		t.Errorf("expected path %q, got %v", path, state.Attributes["path"])
	}
}

func TestStatResource_Read_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	// Create a regular file
	if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+filePath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
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
	if state.Attributes["exists"] != true {
		t.Error("expected exists attribute to be true")
	}
	if state.Attributes["isfile"] != true {
		t.Error("expected isfile to be true")
	}
	if state.Attributes["isdir"] != false {
		t.Error("expected isdir to be false")
	}
	if state.Attributes["islink"] != false {
		t.Error("expected islink to be false")
	}
	if state.Attributes["size"].(int64) != 12 {
		t.Errorf("expected size 12, got %v", state.Attributes["size"])
	}
	if state.Attributes["mode"] != "0644" {
		t.Errorf("expected mode 0644, got %v", state.Attributes["mode"])
	}
}

func TestStatResource_Read_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	// Create a directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+dirPath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
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
	if state.Attributes["isfile"] != false {
		t.Error("expected isfile to be false")
	}
	if state.Attributes["isdir"] != true {
		t.Error("expected isdir to be true")
	}
	if state.Attributes["islink"] != false {
		t.Error("expected islink to be false")
	}
}

func TestStatResource_Read_Symlink_FollowTrue(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	// Create target file
	if err := os.WriteFile(targetPath, []byte("target content"), 0644); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseStatHCL(t, `
		path   = "`+linkPath+`"
		follow = true
	`)

	r, err := NewStatResource("test", body, nil, nil)
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
	// When following symlinks, it should report as a regular file
	if state.Attributes["isfile"] != true {
		t.Error("expected isfile to be true (following symlink)")
	}
	if state.Attributes["isdir"] != false {
		t.Error("expected isdir to be false")
	}
	// islink should still be true since we also check with Lstat
	if state.Attributes["islink"] != true {
		t.Error("expected islink to be true")
	}
	if state.Attributes["size"].(int64) != 14 {
		t.Errorf("expected size 14 (target file size), got %v", state.Attributes["size"])
	}
}

func TestStatResource_Read_Symlink_FollowFalse(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	linkPath := filepath.Join(tmpDir, "link")

	// Create target file
	if err := os.WriteFile(targetPath, []byte("target content"), 0644); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	body := parseStatHCL(t, `
		path   = "`+linkPath+`"
		follow = false
	`)

	r, err := NewStatResource("test", body, nil, nil)
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
	// When not following symlinks, it should not report as a regular file
	if state.Attributes["isfile"] != false {
		t.Error("expected isfile to be false (not following symlink)")
	}
	if state.Attributes["isdir"] != false {
		t.Error("expected isdir to be false")
	}
	if state.Attributes["islink"] != true {
		t.Error("expected islink to be true")
	}
}

func TestStatResource_Read_BrokenSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "brokenlink")

	// Create a symlink to a non-existent target
	if err := os.Symlink("/nonexistent/target", linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Test with follow = true (should report as non-existent)
	body := parseStatHCL(t, `
		path   = "`+linkPath+`"
		follow = true
	`)

	r, err := NewStatResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// When following a broken symlink, it should report as non-existent
	if state.Exists {
		t.Error("expected broken symlink (followed) to report as non-existent")
	}

	// Test with follow = false (should report the symlink itself)
	body2 := parseStatHCL(t, `
		path   = "`+linkPath+`"
		follow = false
	`)

	r2, err := NewStatResource("test2", body2, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	state2, err := r2.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// When not following, the symlink itself exists
	if !state2.Exists {
		t.Error("expected broken symlink (not followed) to report as existing")
	}
	if state2.Attributes["islink"] != true {
		t.Error("expected islink to be true")
	}
}

func TestStatResource_Read_OwnerGroup(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+filePath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check that owner and group are set (values depend on test environment)
	if _, ok := state.Attributes["owner"]; !ok {
		t.Error("expected owner attribute to be set")
	}
	if _, ok := state.Attributes["group"]; !ok {
		t.Error("expected group attribute to be set")
	}
	if _, ok := state.Attributes["uid"]; !ok {
		t.Error("expected uid attribute to be set")
	}
	if _, ok := state.Attributes["gid"]; !ok {
		t.Error("expected gid attribute to be set")
	}
}

func TestStatResource_Read_Timestamps(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+filePath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	state, err := r.Read(ctx)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check that timestamps are set
	if _, ok := state.Attributes["mtime"]; !ok {
		t.Error("expected mtime attribute to be set")
	}
	if _, ok := state.Attributes["atime"]; !ok {
		t.Error("expected atime attribute to be set")
	}
}

func TestStatResource_Diff_AlwaysNoop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+filePath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
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

	// Stat is read-only, should always be ActionNoop
	if plan.Action != ActionNoop {
		t.Errorf("expected ActionNoop, got %v", plan.Action)
	}
	if plan.HasChanges() {
		t.Error("expected no changes for stat resource")
	}
}

func TestStatResource_Apply_Noop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseStatHCL(t, `
		path = "`+filePath+`"
	`)

	r, err := NewStatResource("test", body, nil, nil)
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

	// Apply should be a no-op and not return error
	if err := r.Apply(ctx, plan, true); err != nil {
		t.Errorf("Apply should not fail: %v", err)
	}
}
