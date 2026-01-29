package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DnfPackageManager implements PackageManager for dnf (Fedora/RHEL 8+)
type DnfPackageManager struct{}

func (m *DnfPackageManager) Name() string { return "dnf" }

func (m *DnfPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "rpm", "-q", "--queryformat", "%{VERSION}-%{RELEASE}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	return true, strings.TrimSpace(string(output)), nil
}

func (m *DnfPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "dnf", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dnf install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *DnfPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "dnf", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dnf remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
