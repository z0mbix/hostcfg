package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseDownloadHCL(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{})
	if diags.HasErrors() {
		t.Fatalf("failed to parse HCL: %v", diags.Error())
	}
	return file.Body
}

func TestDownloadResource_Type(t *testing.T) {
	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "/tmp/file"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Type() != "download" {
		t.Errorf("expected type 'download', got %q", r.Type())
	}
}

func TestDownloadResource_Name(t *testing.T) {
	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "/tmp/file"
	`)

	r, err := NewDownloadResource("mydownload", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	if r.Name() != "mydownload" {
		t.Errorf("expected name 'mydownload', got %q", r.Name())
	}
}

func TestDownloadResource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr bool
	}{
		{
			name: "valid download",
			hcl: `
				url  = "http://example.com/file"
				dest = "/tmp/file"
			`,
			wantErr: false,
		},
		{
			name: "valid download with checksum",
			hcl: `
				url      = "http://example.com/file"
				dest     = "/tmp/file"
				checksum = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
			`,
			wantErr: false,
		},
		{
			name: "missing url",
			hcl: `
				dest = "/tmp/file"
			`,
			wantErr: true,
		},
		{
			name: "missing dest",
			hcl: `
				url = "http://example.com/file"
			`,
			wantErr: true,
		},
		{
			name: "invalid checksum format",
			hcl: `
				url      = "http://example.com/file"
				dest     = "/tmp/file"
				checksum = "invalidchecksum"
			`,
			wantErr: true,
		},
		{
			name: "unsupported checksum algorithm",
			hcl: `
				url      = "http://example.com/file"
				dest     = "/tmp/file"
				checksum = "crc32:12345678"
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseDownloadHCL(t, tt.hcl)
			r, err := NewDownloadResource("test", body, nil, "", nil)
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

func TestDownloadResource_Dependencies(t *testing.T) {
	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "/tmp/file"
	`)

	deps := []string{"directory.downloads"}
	r, err := NewDownloadResource("test", body, deps, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	gotDeps := r.Dependencies()
	if len(gotDeps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(gotDeps))
	}
}

func TestDownloadResource_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "nonexistent")

	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "`+destPath+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

func TestDownloadResource_Read_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("test content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedChecksum := "sha256:" + hex.EncodeToString(hash[:])

	body := parseDownloadHCL(t, `
		url      = "http://example.com/file"
		dest     = "`+destPath+`"
		checksum = "`+expectedChecksum+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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
	if state.Attributes["dest"] != destPath {
		t.Errorf("dest mismatch: got %v", state.Attributes["dest"])
	}
	if state.Attributes["mode"] != "0644" {
		t.Errorf("mode mismatch: got %v", state.Attributes["mode"])
	}
	if state.Attributes["checksum"] != expectedChecksum {
		t.Errorf("checksum mismatch: got %v", state.Attributes["checksum"])
	}
}

func TestDownloadResource_Diff_Create(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "newfile")

	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "`+destPath+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

func TestDownloadResource_Diff_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("test content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	hash := sha256.Sum256(content)
	expectedChecksum := "sha256:" + hex.EncodeToString(hash[:])

	body := parseDownloadHCL(t, `
		url      = "http://example.com/file"
		dest     = "`+destPath+`"
		checksum = "`+expectedChecksum+`"
		mode     = "0644"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

func TestDownloadResource_Diff_ChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("old content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Expected checksum for different content
	newContent := []byte("new content")
	hash := sha256.Sum256(newContent)
	expectedChecksum := "sha256:" + hex.EncodeToString(hash[:])

	body := parseDownloadHCL(t, `
		url      = "http://example.com/file"
		dest     = "`+destPath+`"
		checksum = "`+expectedChecksum+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

	var foundChecksumChange bool
	for _, c := range plan.Changes {
		if c.Attribute == "checksum" {
			foundChecksumChange = true
		}
	}
	if !foundChecksumChange {
		t.Error("checksum change not in plan")
	}
}

func TestDownloadResource_Diff_Force(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("test content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseDownloadHCL(t, `
		url   = "http://example.com/file"
		dest  = "`+destPath+`"
		force = true
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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
		t.Errorf("expected ActionUpdate with force=true, got %v", plan.Action)
	}
}

func TestDownloadResource_Diff_ModeChange(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("test content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "`+destPath+`"
		mode = "0755"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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
		t.Errorf("expected ActionUpdate for mode change, got %v", plan.Action)
	}

	var foundModeChange bool
	for _, c := range plan.Changes {
		if c.Attribute == "mode" {
			foundModeChange = true
			if c.Old != "0644" {
				t.Errorf("expected old mode 0644, got %v", c.Old)
			}
			if c.New != "0755" {
				t.Errorf("expected new mode 0755, got %v", c.New)
			}
		}
	}
	if !foundModeChange {
		t.Error("mode change not in plan")
	}
}

func TestDownloadResource_Apply_Download(t *testing.T) {
	// Create a mock HTTP server
	content := []byte("downloaded content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded")

	hash := sha256.Sum256(content)
	checksum := "sha256:" + hex.EncodeToString(hash[:])

	body := parseDownloadHCL(t, `
		url      = "`+server.URL+`/file"
		dest     = "`+destPath+`"
		checksum = "`+checksum+`"
		mode     = "0755"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

	// Verify file was downloaded
	downloaded, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(downloaded) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", downloaded, content)
	}

	// Verify mode
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode mismatch: got %o, want 0755", info.Mode().Perm())
	}
}

func TestDownloadResource_Apply_ChecksumMismatch(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("actual content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded")

	// Use wrong checksum
	body := parseDownloadHCL(t, `
		url      = "`+server.URL+`/file"
		dest     = "`+destPath+`"
		checksum = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	err = r.Apply(ctx, plan, true)
	if err == nil {
		t.Error("expected checksum mismatch error")
	}

	// Verify file was not created (atomic download should have cleaned up)
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("file should not exist after checksum mismatch")
	}
}

func TestDownloadResource_Apply_HTTPError(t *testing.T) {
	// Create a mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded")

	body := parseDownloadHCL(t, `
		url  = "`+server.URL+`/notfound"
		dest = "`+destPath+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()
	current := NewState()
	plan, err := r.Diff(ctx, current)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	err = r.Apply(ctx, plan, true)
	if err == nil {
		t.Error("expected HTTP error")
	}
}

func TestDownloadResource_Apply_DryRun(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made in dry run")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded")

	body := parseDownloadHCL(t, `
		url  = "`+server.URL+`/file"
		dest = "`+destPath+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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
		t.Fatalf("Apply (dry run) failed: %v", err)
	}

	// Verify file was NOT created
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("file should not exist in dry run")
	}
}

func TestDownloadResource_Apply_UpdateModeOnly(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile")

	content := []byte("test content")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	body := parseDownloadHCL(t, `
		url  = "http://example.com/file"
		dest = "`+destPath+`"
		mode = "0755"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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

	// Verify mode was updated
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode mismatch: got %o, want 0755", info.Mode().Perm())
	}

	// Verify content was not changed
	read, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(read) != string(content) {
		t.Error("content should not have changed for mode-only update")
	}
}

func TestDownloadResource_Apply_CreatesParentDirectory(t *testing.T) {
	// Create a mock HTTP server
	content := []byte("test content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "subdir", "nested", "downloaded")

	body := parseDownloadHCL(t, `
		url  = "`+server.URL+`/file"
		dest = "`+destPath+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
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
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestDownloadResource_Idempotent(t *testing.T) {
	// Create a mock HTTP server
	content := []byte("test content")
	downloadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded")

	hash := sha256.Sum256(content)
	checksum := "sha256:" + hex.EncodeToString(hash[:])

	body := parseDownloadHCL(t, `
		url      = "`+server.URL+`/file"
		dest     = "`+destPath+`"
		checksum = "`+checksum+`"
	`)

	r, err := NewDownloadResource("test", body, nil, "", nil)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	ctx := context.Background()

	// First apply - should download
	current1 := NewState()
	plan1, _ := r.Diff(ctx, current1)
	if plan1.Action != ActionCreate {
		t.Error("first run should create")
	}
	if err := r.Apply(ctx, plan1, true); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if downloadCount != 1 {
		t.Errorf("expected 1 download, got %d", downloadCount)
	}

	// Second apply - should be no-op
	current2, _ := r.Read(ctx)
	plan2, _ := r.Diff(ctx, current2)
	if plan2.Action != ActionNoop {
		t.Errorf("second run should be noop, got %v", plan2.Action)
	}
	if err := r.Apply(ctx, plan2, true); err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if downloadCount != 1 {
		t.Errorf("expected still 1 download, got %d", downloadCount)
	}
}

func TestParseChecksum(t *testing.T) {
	tests := []struct {
		input     string
		algorithm string
		hash      string
		wantErr   bool
	}{
		{
			input:     "sha256:abc123",
			algorithm: "sha256",
			hash:      "abc123",
			wantErr:   false,
		},
		{
			input:     "SHA256:ABC123",
			algorithm: "sha256",
			hash:      "abc123",
			wantErr:   false,
		},
		{
			input:     "md5:d41d8cd98f00b204e9800998ecf8427e",
			algorithm: "md5",
			hash:      "d41d8cd98f00b204e9800998ecf8427e",
			wantErr:   false,
		},
		{
			input:     "sha1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			algorithm: "sha1",
			hash:      "da39a3ee5e6b4b0d3255bfef95601890afd80709",
			wantErr:   false,
		},
		{
			input:     "sha512:abcdef",
			algorithm: "sha512",
			hash:      "abcdef",
			wantErr:   false,
		},
		{
			input:   "invalidformat",
			wantErr: true,
		},
		{
			input:   "unsupported:abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			alg, hash, err := parseChecksum(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseChecksum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if alg != tt.algorithm {
					t.Errorf("algorithm = %q, want %q", alg, tt.algorithm)
				}
				if hash != tt.hash {
					t.Errorf("hash = %q, want %q", hash, tt.hash)
				}
			}
		})
	}
}

func TestComputeFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	// Empty file
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Test SHA256 of empty file
	hash, err := computeFileChecksum(filePath, "sha256")
	if err != nil {
		t.Fatalf("computeFileChecksum failed: %v", err)
	}
	// SHA256 of empty string
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("hash = %q, want %q", hash, expected)
	}

	// Test with content
	content := []byte("test content")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	algorithms := []string{"md5", "sha1", "sha256", "sha512"}
	for _, alg := range algorithms {
		hash, err := computeFileChecksum(filePath, alg)
		if err != nil {
			t.Errorf("computeFileChecksum(%s) failed: %v", alg, err)
		}
		if hash == "" {
			t.Errorf("computeFileChecksum(%s) returned empty hash", alg)
		}
	}

	// Test unsupported algorithm
	_, err = computeFileChecksum(filePath, "unsupported")
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}
