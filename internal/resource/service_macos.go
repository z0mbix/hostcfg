package resource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LaunchctlServiceManager implements ServiceManager for launchctl (macOS)
type LaunchctlServiceManager struct{}

func (m *LaunchctlServiceManager) Name() string { return "launchctl" }

// getServiceLabel returns the launchd service label for a given service name
// Services can be specified as:
// - Full label: "com.example.myservice"
// - Homebrew service: "nginx" -> looks for homebrew.mxcl.nginx
// - System service: "com.apple.xxx"
func (m *LaunchctlServiceManager) getServiceLabel(name string) string {
	// If it already looks like a full label, return as-is
	if strings.Contains(name, ".") {
		return name
	}
	// Assume it's a Homebrew service
	return "homebrew.mxcl." + name
}

// getPlistPath returns the path to the launchd plist file for a service
func (m *LaunchctlServiceManager) getPlistPath(name string) string {
	label := m.getServiceLabel(name)

	// Check common locations
	locations := []string{
		// User LaunchAgents (Homebrew services typically go here)
		filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents", label+".plist"),
		// System LaunchAgents
		"/Library/LaunchAgents/" + label + ".plist",
		// System LaunchDaemons
		"/Library/LaunchDaemons/" + label + ".plist",
		// Apple LaunchAgents
		"/System/Library/LaunchAgents/" + label + ".plist",
		// Apple LaunchDaemons
		"/System/Library/LaunchDaemons/" + label + ".plist",
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to user LaunchAgents for Homebrew services
	return filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents", label+".plist")
}

func (m *LaunchctlServiceManager) Exists(ctx context.Context, name string) (bool, error) {
	plistPath := m.getPlistPath(name)

	// Check if the plist file exists
	if _, err := os.Stat(plistPath); err == nil {
		return true, nil
	}

	// Also check if it's loaded in launchctl
	label := m.getServiceLabel(name)
	cmd := exec.CommandContext(ctx, "launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, label) {
			return true, nil
		}
	}

	return false, nil
}

func (m *LaunchctlServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	label := m.getServiceLabel(name)

	// launchctl list shows running services
	// Format: PID	Status	Label
	// If PID is "-", the service is not running
	cmd := exec.CommandContext(ctx, "launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasSuffix(line, label) || strings.Contains(line, "\t"+label) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				// First field is PID, "-" means not running
				if fields[0] != "-" {
					return true, nil
				}
			}
			return false, nil
		}
	}

	return false, nil
}

func (m *LaunchctlServiceManager) IsEnabled(ctx context.Context, name string) (bool, error) {
	// On macOS, a service is "enabled" if its plist is present in a LaunchAgents/LaunchDaemons directory
	// and doesn't have the Disabled key set to true
	plistPath := m.getPlistPath(name)

	if _, err := os.Stat(plistPath); err != nil {
		return false, nil
	}

	// Check if the service is disabled using launchctl print
	// For modern macOS, we can check the disabled status
	label := m.getServiceLabel(name)
	cmd := exec.CommandContext(ctx, "launchctl", "print-disabled", "gui/"+fmt.Sprint(os.Getuid()))
	output, _ := cmd.Output()

	// Look for the service in the disabled list
	// Format: "label" => true/false
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "\""+label+"\"") {
			if strings.Contains(line, "=> true") || strings.Contains(line, "=> disabled") {
				return false, nil
			}
		}
	}

	// If plist exists and not explicitly disabled, consider it enabled
	return true, nil
}

func (m *LaunchctlServiceManager) Start(ctx context.Context, name string) error {
	label := m.getServiceLabel(name)
	plistPath := m.getPlistPath(name)

	// Try bootstrap first (modern macOS), fall back to load
	cmd := exec.CommandContext(ctx, "launchctl", "bootstrap", "gui/"+fmt.Sprint(os.Getuid()), plistPath)
	if err := cmd.Run(); err != nil {
		// Fall back to legacy load command
		cmd = exec.CommandContext(ctx, "launchctl", "load", plistPath)
		if err := cmd.Run(); err != nil {
			// Service might already be loaded, try kickstart
			cmd = exec.CommandContext(ctx, "launchctl", "kickstart", "gui/"+fmt.Sprint(os.Getuid())+"/"+label)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("launchctl start failed: %w\nOutput: %s", err, string(output))
			}
		}
	}
	return nil
}

func (m *LaunchctlServiceManager) Stop(ctx context.Context, name string) error {
	label := m.getServiceLabel(name)
	plistPath := m.getPlistPath(name)

	// Try bootout first (modern macOS), fall back to unload
	cmd := exec.CommandContext(ctx, "launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid())+"/"+label)
	if err := cmd.Run(); err != nil {
		// Fall back to legacy unload command
		cmd = exec.CommandContext(ctx, "launchctl", "unload", plistPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("launchctl stop failed: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}

func (m *LaunchctlServiceManager) Enable(ctx context.Context, name string) error {
	label := m.getServiceLabel(name)

	// Enable the service (remove from disabled list)
	cmd := exec.CommandContext(ctx, "launchctl", "enable", "gui/"+fmt.Sprint(os.Getuid())+"/"+label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Might not be in disabled list, which is fine
		if !strings.Contains(string(output), "No such process") {
			return fmt.Errorf("launchctl enable failed: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}

func (m *LaunchctlServiceManager) Disable(ctx context.Context, name string) error {
	label := m.getServiceLabel(name)

	// First stop the service if running
	_ = m.Stop(ctx, name)

	// Disable the service
	cmd := exec.CommandContext(ctx, "launchctl", "disable", "gui/"+fmt.Sprint(os.Getuid())+"/"+label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl disable failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
