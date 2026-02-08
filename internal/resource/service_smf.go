package resource

import (
	"context"
	"fmt"
	"strings"
)

// SMFServiceManager implements ServiceManager for illumos SMF (svcadm/svcs)
type SMFServiceManager struct{}

func (m *SMFServiceManager) Name() string { return "smf" }

func (m *SMFServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	err := RunCmdSilent(ctx, "svcs", "-H", name)
	return err == nil, nil
}

func (m *SMFServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	output, err := RunCmd(ctx, "svcs", "-H", "-o", "state", name)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "online", nil
}

func (m *SMFServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	output, err := RunCmd(ctx, "svcs", "-H", "-o", "state", name)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) != "disabled", nil
}

func (m *SMFServiceManager) Start(ctx context.Context, name string) error {
	output, err := RunCmd(ctx, "svcadm", "enable", "-t", name)
	if err != nil {
		return fmt.Errorf("svcadm enable -t failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Stop(ctx context.Context, name string) error {
	output, err := RunCmd(ctx, "svcadm", "disable", "-t", name)
	if err != nil {
		return fmt.Errorf("svcadm disable -t failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Enable(ctx context.Context, name string) error {
	output, err := RunCmd(ctx, "svcadm", "enable", name)
	if err != nil {
		return fmt.Errorf("svcadm enable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *SMFServiceManager) Disable(ctx context.Context, name string) error {
	output, err := RunCmd(ctx, "svcadm", "disable", name)
	if err != nil {
		return fmt.Errorf("svcadm disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
