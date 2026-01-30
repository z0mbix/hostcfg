package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HomebrewPackageManager implements PackageManager for Homebrew (macOS)
type HomebrewPackageManager struct{}

func (m *HomebrewPackageManager) Name() string { return "brew" }

func (m *HomebrewPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	// Check if the package is installed using brew list
	cmd := exec.CommandContext(ctx, "brew", "list", "--versions", name)
	output, err := cmd.Output()
	if err != nil {
		// brew list returns non-zero if package is not installed
		return false, "", nil
	}

	// Output format: "package_name version1 version2 ..."
	// e.g., "nginx 1.25.3"
	line := strings.TrimSpace(string(output))
	if line == "" {
		return false, "", nil
	}

	parts := strings.Fields(line)
	if len(parts) >= 2 {
		// Return the first (usually current) version
		return true, parts[1], nil
	}

	return true, "", nil
}

func (m *HomebrewPackageManager) Install(ctx context.Context, name, version string) error {
	var cmd *exec.Cmd
	if version != "" {
		// Install specific version using @version syntax
		// Note: This works for formulae that support versioned installs
		pkg := fmt.Sprintf("%s@%s", name, version)
		cmd = exec.CommandContext(ctx, "brew", "install", pkg)
	} else {
		cmd = exec.CommandContext(ctx, "brew", "install", name)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *HomebrewPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "brew", "uninstall", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew uninstall failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
