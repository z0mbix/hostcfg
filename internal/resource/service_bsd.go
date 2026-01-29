package resource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// FreeBSDServiceManager implements ServiceManager for FreeBSD rc.d
type FreeBSDServiceManager struct{}

func (m *FreeBSDServiceManager) Name() string { return "rc.d" }

func (m *FreeBSDServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	// Check if rc script exists
	rcScript := "/etc/rc.d/" + name
	if _, err := os.Stat(rcScript); err == nil {
		return true, nil
	}
	// Also check /usr/local/etc/rc.d for ports
	rcScript = "/usr/local/etc/rc.d/" + name
	if _, err := os.Stat(rcScript); err == nil {
		return true, nil
	}
	return false, nil
}

func (m *FreeBSDServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "service", name, "status")
	err := cmd.Run()
	return err == nil, nil
}

func (m *FreeBSDServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "service", name, "enabled")
	err := cmd.Run()
	return err == nil, nil
}

func (m *FreeBSDServiceManager) Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "service", name, "start")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("service start failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *FreeBSDServiceManager) Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "service", name, "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("service stop failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *FreeBSDServiceManager) Enable(ctx context.Context, name string) error {
	// Add service_enable="YES" to /etc/rc.conf
	cmd := exec.CommandContext(ctx, "sysrc", name+"_enable=YES")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysrc enable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *FreeBSDServiceManager) Disable(ctx context.Context, name string) error {
	// Set service_enable="NO" in /etc/rc.conf
	cmd := exec.CommandContext(ctx, "sysrc", name+"_enable=NO")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysrc disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// OpenBSDServiceManager implements ServiceManager for OpenBSD rcctl
type OpenBSDServiceManager struct{}

func (m *OpenBSDServiceManager) Name() string { return "rcctl" }

func (m *OpenBSDServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "rcctl", "ls", "all")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *OpenBSDServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "rcctl", "check", name)
	err := cmd.Run()
	return err == nil, nil
}

func (m *OpenBSDServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "rcctl", "get", name, "flags")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	// If flags is "NO", the service is disabled
	return strings.TrimSpace(string(output)) != "NO", nil
}

func (m *OpenBSDServiceManager) Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "rcctl", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcctl start failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *OpenBSDServiceManager) Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "rcctl", "stop", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcctl stop failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *OpenBSDServiceManager) Enable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "rcctl", "enable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcctl enable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *OpenBSDServiceManager) Disable(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "rcctl", "disable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rcctl disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// NetBSDServiceManager implements ServiceManager for NetBSD rc.d
type NetBSDServiceManager struct{}

func (m *NetBSDServiceManager) Name() string { return "rc.d" }

func (m *NetBSDServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	// Check if rc script exists
	rcScript := "/etc/rc.d/" + name
	if _, err := os.Stat(rcScript); err == nil {
		return true, nil
	}
	// Also check /usr/pkg/share/examples/rc.d for pkgsrc services
	rcScript = "/usr/pkg/share/examples/rc.d/" + name
	if _, err := os.Stat(rcScript); err == nil {
		return true, nil
	}
	return false, nil
}

func (m *NetBSDServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "/etc/rc.d/"+name, "status")
	err := cmd.Run()
	return err == nil, nil
}

func (m *NetBSDServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	// Check /etc/rc.conf for service=YES
	data, err := os.ReadFile("/etc/rc.conf")
	if err != nil {
		return false, nil
	}
	// Look for service=YES or service=yes
	content := string(data)
	return strings.Contains(content, name+"=YES") || strings.Contains(content, name+"=yes"), nil
}

func (m *NetBSDServiceManager) Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "/etc/rc.d/"+name, "start")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rc.d start failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *NetBSDServiceManager) Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "/etc/rc.d/"+name, "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rc.d stop failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (m *NetBSDServiceManager) Enable(ctx context.Context, name string) error {
	// Append service=YES to /etc/rc.conf if not already present
	data, err := os.ReadFile("/etc/rc.conf")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read /etc/rc.conf: %w", err)
	}

	content := string(data)
	entry := name + "=YES"

	// Check if already enabled
	if strings.Contains(content, entry) {
		return nil
	}

	// Remove any existing entry for this service
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), name+"=") {
			newLines = append(newLines, line)
		}
	}

	// Add the new entry
	newLines = append(newLines, entry)
	newContent := strings.Join(newLines, "\n")

	if err := os.WriteFile("/etc/rc.conf", []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/rc.conf: %w", err)
	}
	return nil
}

func (m *NetBSDServiceManager) Disable(ctx context.Context, name string) error {
	data, err := os.ReadFile("/etc/rc.conf")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read /etc/rc.conf: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove entries like service=YES or service=yes
		if strings.HasPrefix(trimmed, name+"=") {
			continue
		}
		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile("/etc/rc.conf", []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/rc.conf: %w", err)
	}
	return nil
}
