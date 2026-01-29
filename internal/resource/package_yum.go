package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// YumPackageManager implements PackageManager for yum (RHEL 7/CentOS)
type YumPackageManager struct{}

func (m *YumPackageManager) Name() string { return "yum" }

func (m *YumPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "rpm", "-q", "--queryformat", "%{VERSION}-%{RELEASE}", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	return true, strings.TrimSpace(string(output)), nil
}

func (m *YumPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yum install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *YumPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "yum", "remove", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yum remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
