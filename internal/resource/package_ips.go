package resource

import (
	"context"
	"fmt"
	"strings"
)

// IPSPackageManager implements PackageManager for IPS pkg (OmniOS, OpenIndiana)
type IPSPackageManager struct{}

func (m *IPSPackageManager) Name() string { return "pkg" }

func (m *IPSPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	output, err := RunCmd(ctx, "pkg", "list", "-H", name)
	if err != nil {
		return false, "", nil
	}

	// Output format: NAME (PUBLISHER) VERSION IFO
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) >= 2 {
		return true, fields[1], nil
	}

	return true, "", nil
}

func (m *IPSPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s@%s", name, version)
	}
	output, err := RunCmd(ctx, "pkg", "install", "--accept", pkg)
	if err != nil {
		return fmt.Errorf("pkg install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *IPSPackageManager) Remove(ctx context.Context, name string) error {
	output, err := RunCmd(ctx, "pkg", "uninstall", name)
	if err != nil {
		return fmt.Errorf("pkg uninstall failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
