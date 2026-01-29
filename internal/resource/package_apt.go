package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AptPackageManager implements PackageManager for apt (Debian/Ubuntu)
type AptPackageManager struct{}

func (m *AptPackageManager) Name() string { return "apt" }

func (m *AptPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status} ${Version}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "install ok installed") {
		parts := strings.Fields(outputStr)
		if len(parts) >= 4 {
			return true, parts[3], nil
		}
		return true, "", nil
	}

	return false, "", nil
}

func (m *AptPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s=%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *AptPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "apt-get", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
