package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PacmanPackageManager implements PackageManager for pacman (Arch Linux)
type PacmanPackageManager struct{}

func (m *PacmanPackageManager) Name() string { return "pacman" }

func (m *PacmanPackageManager) IsInstalled(ctx context.Context, name string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "pacman", "-Q", name)
	output, err := cmd.Output()
	if err != nil {
		return false, "", nil
	}
	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return true, parts[1], nil
	}
	return true, "", nil
}

func (m *PacmanPackageManager) Install(ctx context.Context, name, version string) error {
	// pacman doesn't support installing specific versions easily
	// version parameter is ignored
	cmd := exec.CommandContext(ctx, "pacman", "-S", "--noconfirm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacman install failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *PacmanPackageManager) Remove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "pacman", "-R", "--noconfirm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacman remove failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
