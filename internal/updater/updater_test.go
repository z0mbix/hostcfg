package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newTestRelease(version string) Release {
	assetName := fmt.Sprintf("hostcfg_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	return Release{
		TagName: version,
		Assets: []Asset{
			{Name: assetName, BrowserDownloadURL: "ASSET_URL"},
			{Name: "checksums.txt", BrowserDownloadURL: "CHECKSUM_URL"},
		},
	}
}

func TestCheck_UpdateAvailable(t *testing.T) {
	release := newTestRelease("1.2.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		HTTPClient:     server.Client(),
		APIURL:         server.URL,
	}

	result, err := u.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UpdateNeeded {
		t.Error("expected update needed")
	}
	if result.LatestVersion != "1.2.0" {
		t.Errorf("expected latest version 1.2.0, got %s", result.LatestVersion)
	}
}

func TestCheck_AlreadyUpToDate(t *testing.T) {
	release := newTestRelease("1.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		HTTPClient:     server.Client(),
		APIURL:         server.URL,
	}

	result, err := u.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateNeeded {
		t.Error("expected no update needed")
	}
}

func TestCheck_VPrefixHandling(t *testing.T) {
	release := newTestRelease("v1.0.0")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return version with v prefix in tag
		r2 := release
		r2.TagName = "v1.0.0"
		// Fix asset name to match non-prefixed version
		assetName := fmt.Sprintf("hostcfg_%s_%s_%s.tar.gz", "1.0.0", runtime.GOOS, runtime.GOARCH)
		r2.Assets = []Asset{
			{Name: assetName, BrowserDownloadURL: "ASSET_URL"},
			{Name: "checksums.txt", BrowserDownloadURL: "CHECKSUM_URL"},
		}
		_ = json.NewEncoder(w).Encode(r2)
	}))
	defer server.Close()

	u := &Updater{
		CurrentVersion: "v1.0.0",
		HTTPClient:     server.Client(),
		APIURL:         server.URL,
	}

	result, err := u.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateNeeded {
		t.Error("expected no update needed with v prefix")
	}
}

func TestCheck_NoAssetForPlatform(t *testing.T) {
	release := Release{
		TagName: "1.2.0",
		Assets: []Asset{
			{Name: "hostcfg_1.2.0_fakeos_fakegoarch.tar.gz", BrowserDownloadURL: "URL"},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		HTTPClient:     server.Client(),
		APIURL:         server.URL,
	}

	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
}

func TestCheck_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := &Updater{
		CurrentVersion: "1.0.0",
		HTTPClient:     server.Client(),
		APIURL:         server.URL,
	}

	_, err := u.Check()
	if err == nil {
		t.Fatal("expected error on API failure")
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("test content")
	tmpFile, err := os.CreateTemp("", "checksum-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, _ = tmpFile.Write(content)
	_ = tmpFile.Close()

	h := sha256.Sum256(content)
	goodChecksum := hex.EncodeToString(h[:])

	if err := verifyChecksum(tmpFile.Name(), goodChecksum); err != nil {
		t.Fatalf("expected valid checksum: %v", err)
	}

	if err := verifyChecksum(tmpFile.Name(), "badhash"); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestExtractBinary(t *testing.T) {
	binaryContent := []byte("#!/bin/fake-binary")
	archivePath := createTestArchive(t, "hostcfg", binaryContent)
	defer func() { _ = os.Remove(archivePath) }()

	data, err := extractBinary(archivePath, "hostcfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(binaryContent) {
		t.Errorf("expected %q, got %q", binaryContent, data)
	}
}

func TestExtractBinary_NotFound(t *testing.T) {
	archivePath := createTestArchive(t, "other-file", []byte("data"))
	defer func() { _ = os.Remove(archivePath) }()

	_, err := extractBinary(archivePath, "hostcfg")
	if err == nil {
		t.Fatal("expected error for missing binary in archive")
	}
}

func TestReplaceBinary(t *testing.T) {
	// Create a fake current binary
	dir := t.TempDir()
	binPath := filepath.Join(dir, "hostcfg")
	if err := os.WriteFile(binPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	newContent := []byte("new-binary")
	if err := replaceBinary(binPath, newContent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Errorf("expected %q, got %q", newContent, got)
	}

	info, _ := os.Stat(binPath)
	if info.Mode() != 0o755 {
		t.Errorf("expected mode 0755, got %v", info.Mode())
	}
}

// createTestArchive creates a tar.gz with a single file
func createTestArchive(t *testing.T, name string, content []byte) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-archive-*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gw.Close()
	_ = tmpFile.Close()

	return tmpFile.Name()
}
