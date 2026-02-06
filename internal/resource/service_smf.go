package resource

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SMFServiceManager implements ServiceManager for illumos SMF (svcadm/svcs)
type SMFServiceManager struct{}

func (m *SMFServiceManager) Name() string { return "smf" }

func (m *SMFServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "svcs", "-H", name)
	err := cmd.Run()
	return err == nil, nil
}

func (m *SMFServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "svcs", "-H", "-o", "state", name)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "online", nil
}

func (m *SMFServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	// A service is enabled if it's not in the "disabled" state
	cmd := exec.CommandContext(ctx, "svcs", "-H", "-o", "state", name)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) != "disabled", nil
}

func (m *SMFServiceManager) Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "svcadm", "enable", "-t", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("svcadm enable -t failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "svcadm", "disable", "-t", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("svcadm disable -t failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Enable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "svcadm", "enable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("svcadm enable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Disable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "svcadm", "disable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("svcadm disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
