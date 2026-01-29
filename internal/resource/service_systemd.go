package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SystemdServiceManager implements ServiceManager for systemd (Linux)
type SystemdServiceManager struct{}

func (m *SystemdServiceManager) Name() string { return "systemd" }

func (m *SystemdServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-unit-files", name+".service")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(output), name), nil
}

func (m *SystemdServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", name)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "active", nil
}

func (m *SystemdServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-enabled", name)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "enabled", nil
}

func (m *SystemdServiceManager) Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl start failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SystemdServiceManager) Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "stop", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SystemdServiceManager) Enable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "enable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SystemdServiceManager) Disable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "disable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
