package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PkgPackageManager implements PackageManager for pkg (FreeBSD)
type PkgPackageManager struct{}

func (m *PkgPackageManager) Name() string { return "pkg" }

func (m *PkgPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "pkg", "info", "-e", name)
	err := cmd.Run()
	if err != nil {
		return false, "", nil
	}

	// Get version
	cmd = exec.CommandContext(ctx, "pkg", "query", "%v", name)
	output, err := cmd.Output()
	if err != nil {
		return true, "", nil
	}

	return true, strings.TrimSpace(string(output)), nil
}

func (m *PkgPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "pkg", "install", "-y", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *PkgPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pkg", "delete", "-y", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg delete failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// OpenBSDPackageManager implements PackageManager for pkg_add (OpenBSD)
type OpenBSDPackageManager struct{}

func (m *OpenBSDPackageManager) Name() string { return "pkg_add" }

func (m *OpenBSDPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	// pkg_info -e returns 0 if package is installed
	cmd := exec.CommandContext(ctx, "pkg_info", "-e", name+"-*")
	err := cmd.Run()
	if err != nil {
		// Try exact name match
		cmd = exec.CommandContext(ctx, "pkg_info", "-e", name)
		if err := cmd.Run(); err != nil {
			return false, "", nil
		}
	}

	// Get the installed package info to extract version
	cmd = exec.CommandContext(ctx, "pkg_info", "-q", name)
	output, err := cmd.Output()
	if err != nil {
		// Try with wildcard
		cmd = exec.CommandContext(ctx, "pkg_info")
		output, err = cmd.Output()
		if err != nil {
			return true, "", nil
		}
		// Parse output to find matching package
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, name+"-") {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) > 0 {
					// Extract version from package name (e.g., "nginx-1.24.0" -> "1.24.0")
					pkgName := parts[0]
					if idx := strings.Index(pkgName, name+"-"); idx == 0 {
						version := strings.TrimPrefix(pkgName, name+"-")
						return true, version, nil
					}
				}
			}
		}
		return true, "", nil
	}

	return true, strings.TrimSpace(string(output)), nil
}

func (m *OpenBSDPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "pkg_add", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg_add failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *OpenBSDPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pkg_delete", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg_delete failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// PkginPackageManager implements PackageManager for pkgin (NetBSD preferred)
type PkginPackageManager struct{}

func (m *PkginPackageManager) Name() string { return "pkgin" }

func (m *PkginPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "pkgin", "list")
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}

	// Parse output to find the package
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			// Package format is typically "name-version"
			pkgName := fields[0]
			if strings.HasPrefix(pkgName, name+"-") {
				version := strings.TrimPrefix(pkgName, name+"-")
				return true, version, nil
			}
			if pkgName == name {
				return true, "", nil
			}
		}
	}

	return false, "", nil
}

func (m *PkginPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "pkgin", "-y", "install", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkgin install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *PkginPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pkgin", "-y", "remove", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkgin remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// NetBSDPackageManager implements PackageManager for pkg_add (NetBSD fallback)
type NetBSDPackageManager struct{}

func (m *NetBSDPackageManager) Name() string { return "pkg_add" }

func (m *NetBSDPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "pkg_info", "-e", name+"-[0-9]*")
	err := cmd.Run()
	if err != nil {
		// Try exact match
		cmd = exec.CommandContext(ctx, "pkg_info", "-e", name)
		if err := cmd.Run(); err != nil {
			return false, "", nil
		}
	}

	// Get version info
	cmd = exec.CommandContext(ctx, "pkg_info", "-Q", "PKGVERSION", name)
	output, err := cmd.Output()
	if err != nil {
		return true, "", nil
	}

	return true, strings.TrimSpace(string(output)), nil
}

func (m *NetBSDPackageManager) Install(ctx context.Context, name, version string) error {
	pkg := name
	if version != "" {
		pkg = fmt.Sprintf("%s-%s", name, version)
	}
	cmd := exec.CommandContext(ctx, "pkg_add", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg_add failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *NetBSDPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pkg_delete", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkg_delete failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
