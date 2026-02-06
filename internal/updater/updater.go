package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repoOwner = "z0mbix"
	repoName  = "hostcfg"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a GitHub release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Updater handles self-update logic
type Updater struct {
	CurrentVersion string
	HTTPClient     *http.Client
	APIURL         string
}

// New creates a new Updater
func New(currentVersion string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		HTTPClient:     http.DefaultClient,
		APIURL:         apiURL,
	}
}

// CheckResult contains the result of checking for updates
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpdateNeeded   bool
	AssetURL       string
	AssetName      string
	ChecksumURL    string
}

// Check fetches the latest release and determines if an update is available
func (u *Updater) Check() (*CheckResult, error) {
	req, err := http.NewRequest("GET", u.APIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(u.CurrentVersion, "v")

	result := &CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		UpdateNeeded:   latestVersion != currentVersion,
	}

	expectedName := fmt.Sprintf("hostcfg_%s_%s_%s.tar.gz", latestVersion, runtime.GOOS, runtime.GOARCH)
	for _, asset := range release.Assets {
		if asset.Name == expectedName {
			result.AssetURL = asset.BrowserDownloadURL
			result.AssetName = asset.Name
		}
		if asset.Name == "checksums.txt" {
			result.ChecksumURL = asset.BrowserDownloadURL
		}
	}

	if result.AssetURL == "" {
		return nil, fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return result, nil
}

// Update downloads and installs the latest release
func (u *Updater) Update(result *CheckResult) error {
	// Download checksum file
	expectedChecksum, err := u.fetchChecksum(result.ChecksumURL, result.AssetName)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}

	// Download archive to temp file
	archivePath, err := u.downloadFile(result.AssetURL)
	if err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}
	defer func() { _ = os.Remove(archivePath) }()

	// Verify checksum
	if err := verifyChecksum(archivePath, expectedChecksum); err != nil {
		return err
	}

	// Extract binary from archive
	binaryData, err := extractBinary(archivePath, "hostcfg")
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// Replace current binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determining executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	if err := replaceBinary(execPath, binaryData); err != nil {
		return err
	}

	return nil
}

func (u *Updater) fetchChecksum(checksumURL, assetName string) (string, error) {
	if checksumURL == "" {
		return "", fmt.Errorf("no checksums.txt asset found in release")
	}

	resp, err := u.HTTPClient.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", assetName)
}

func (u *Updater) downloadFile(url string) (string, error) {
	resp, err := u.HTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "hostcfg-update-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer func() { _ = tmpFile.Close() }()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func verifyChecksum(filePath, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

func extractBinary(archivePath, binaryName string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("binary %q not found in archive", binaryName)
}

func replaceBinary(execPath string, newBinary []byte) error {
	// Get current file info to preserve permissions
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}

	dir := filepath.Dir(execPath)

	// Check we can write to the directory
	tmpFile, err := os.CreateTemp(dir, ".hostcfg-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (try running with sudo): %w", dir, err)
	}
	tmpPath := tmpFile.Name()

	// Write new binary
	if _, err := tmpFile.Write(newBinary); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing new binary: %w", err)
	}
	_ = tmpFile.Close()

	// Set permissions
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, execPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing binary (try running with sudo): %w", err)
	}

	return nil
}
