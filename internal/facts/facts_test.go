package facts

import (
	"os"
	"runtime"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestGather(t *testing.T) {
	facts, err := Gather()
	if err != nil {
		t.Fatalf("Gather() returned error: %v", err)
	}

	// Check that OS name matches runtime
	if facts.OS.Name != runtime.GOOS {
		t.Errorf("OS.Name = %q, want %q", facts.OS.Name, runtime.GOOS)
	}

	// Check that Arch matches runtime
	if facts.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", facts.Arch, runtime.GOARCH)
	}

	// Hostname should not be empty
	if facts.Hostname == "" {
		t.Error("Hostname is empty")
	}

	// User name should not be empty
	if facts.User.Name == "" {
		t.Error("User.Name is empty")
	}

	// User home should not be empty
	if facts.User.Home == "" {
		t.Error("User.Home is empty")
	}

	// UID/GID should be valid numbers
	if facts.User.UID == "" {
		t.Error("User.UID is empty")
	}
	if facts.User.GID == "" {
		t.Error("User.GID is empty")
	}

	// CPU facts should have sensible values
	if facts.CPU.Physical < 1 {
		t.Errorf("CPU.Physical = %d, want >= 1", facts.CPU.Physical)
	}
	if facts.CPU.Cores < 1 {
		t.Errorf("CPU.Cores = %d, want >= 1", facts.CPU.Cores)
	}

	// Environment variables should not be empty
	if len(facts.Env) == 0 {
		t.Error("Env map is empty")
	}

	// PATH should typically be set
	if _, ok := facts.Env["PATH"]; !ok {
		t.Error("PATH not found in Env map")
	}
}

func TestToCtyValue(t *testing.T) {
	facts := &Facts{
		OS: OSFacts{
			Name:                "linux",
			Family:              "debian",
			Distribution:        "Ubuntu",
			DistributionVersion: "22.04",
		},
		Arch:      "amd64",
		Hostname:  "testhost",
		FQDN:      "testhost.example.com",
		MachineID: "abc123def456",
		CPU: CPUFacts{
			Physical: 4,
			Cores:    8,
		},
		User: UserFacts{
			Name: "testuser",
			Home: "/home/testuser",
			UID:  "1000",
			GID:  "1000",
		},
		Env: map[string]string{
			"HOME": "/home/testuser",
			"PATH": "/usr/bin:/bin",
		},
	}

	val := facts.ToCtyValue()

	// Check that it's an object
	if !val.Type().IsObjectType() {
		t.Fatalf("ToCtyValue() returned %s, want object", val.Type().FriendlyName())
	}

	// Check top-level attributes
	assertCtyString(t, val, "arch", "amd64")
	assertCtyString(t, val, "hostname", "testhost")
	assertCtyString(t, val, "fqdn", "testhost.example.com")
	assertCtyString(t, val, "machine_id", "abc123def456")

	// Check nested OS object
	osVal := val.GetAttr("os")
	if !osVal.Type().IsObjectType() {
		t.Fatalf("os is %s, want object", osVal.Type().FriendlyName())
	}
	assertCtyString(t, osVal, "name", "linux")
	assertCtyString(t, osVal, "family", "debian")
	assertCtyString(t, osVal, "distribution", "Ubuntu")
	assertCtyString(t, osVal, "distribution_version", "22.04")

	// Check nested CPU object
	cpuVal := val.GetAttr("cpu")
	if !cpuVal.Type().IsObjectType() {
		t.Fatalf("cpu is %s, want object", cpuVal.Type().FriendlyName())
	}
	assertCtyNumber(t, cpuVal, "physical", 4)
	assertCtyNumber(t, cpuVal, "cores", 8)

	// Check nested user object
	userVal := val.GetAttr("user")
	if !userVal.Type().IsObjectType() {
		t.Fatalf("user is %s, want object", userVal.Type().FriendlyName())
	}
	assertCtyString(t, userVal, "name", "testuser")
	assertCtyString(t, userVal, "home", "/home/testuser")
	assertCtyString(t, userVal, "uid", "1000")
	assertCtyString(t, userVal, "gid", "1000")

	// Check nested env object
	envVal := val.GetAttr("env")
	if !envVal.Type().IsObjectType() {
		t.Fatalf("env is %s, want object", envVal.Type().FriendlyName())
	}
	assertCtyString(t, envVal, "HOME", "/home/testuser")
	assertCtyString(t, envVal, "PATH", "/usr/bin:/bin")
}

func assertCtyString(t *testing.T, obj cty.Value, attr, expected string) {
	t.Helper()
	val := obj.GetAttr(attr)
	if val.Type() != cty.String {
		t.Errorf("%s is %s, want string", attr, val.Type().FriendlyName())
		return
	}
	if val.AsString() != expected {
		t.Errorf("%s = %q, want %q", attr, val.AsString(), expected)
	}
}

func assertCtyNumber(t *testing.T, obj cty.Value, attr string, expected int64) {
	t.Helper()
	val := obj.GetAttr(attr)
	if val.Type() != cty.Number {
		t.Errorf("%s is %s, want number", attr, val.Type().FriendlyName())
		return
	}
	bf := val.AsBigFloat()
	got, _ := bf.Int64()
	if got != expected {
		t.Errorf("%s = %d, want %d", attr, got, expected)
	}
}

func TestDetectLinuxFamily(t *testing.T) {
	tests := []struct {
		name     string
		release  map[string]string
		expected string
	}{
		{
			name:     "Ubuntu",
			release:  map[string]string{"ID": "ubuntu", "ID_LIKE": "debian"},
			expected: "debian",
		},
		{
			name:     "Debian",
			release:  map[string]string{"ID": "debian"},
			expected: "debian",
		},
		{
			name:     "Fedora",
			release:  map[string]string{"ID": "fedora"},
			expected: "redhat",
		},
		{
			name:     "CentOS",
			release:  map[string]string{"ID": "centos", "ID_LIKE": "rhel fedora"},
			expected: "redhat",
		},
		{
			name:     "Rocky Linux",
			release:  map[string]string{"ID": "rocky", "ID_LIKE": "rhel centos fedora"},
			expected: "redhat",
		},
		{
			name:     "Arch Linux",
			release:  map[string]string{"ID": "arch"},
			expected: "arch",
		},
		{
			name:     "Manjaro",
			release:  map[string]string{"ID": "manjaro", "ID_LIKE": "arch"},
			expected: "arch",
		},
		{
			name:     "Alpine",
			release:  map[string]string{"ID": "alpine"},
			expected: "alpine",
		},
		{
			name:     "openSUSE",
			release:  map[string]string{"ID": "opensuse-leap", "ID_LIKE": "suse opensuse"},
			expected: "suse",
		},
		{
			name:     "Linux Mint",
			release:  map[string]string{"ID": "linuxmint", "ID_LIKE": "ubuntu debian"},
			expected: "debian",
		},
		{
			name:     "Pop!_OS",
			release:  map[string]string{"ID": "pop", "ID_LIKE": "ubuntu debian"},
			expected: "debian",
		},
		{
			name:     "Unknown distro with debian-like",
			release:  map[string]string{"ID": "custom", "ID_LIKE": "debian"},
			expected: "debian",
		},
		{
			name:     "Unknown distro",
			release:  map[string]string{"ID": "unknowndistro"},
			expected: "unknown",
		},
		{
			name:     "Amazon Linux",
			release:  map[string]string{"ID": "amzn", "ID_LIKE": "centos rhel fedora"},
			expected: "redhat",
		},
		{
			name:     "NixOS",
			release:  map[string]string{"ID": "nixos"},
			expected: "nixos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLinuxFamily(tt.release)
			if result != tt.expected {
				t.Errorf("detectLinuxFamily() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGatherUserFacts(t *testing.T) {
	facts, err := gatherUserFacts()
	if err != nil {
		t.Fatalf("gatherUserFacts() returned error: %v", err)
	}

	// User name should match current user
	expectedUser := os.Getenv("USER")
	if expectedUser != "" && facts.Name != expectedUser {
		t.Errorf("User.Name = %q, want %q", facts.Name, expectedUser)
	}

	// Home should be a valid path
	if facts.Home == "" {
		t.Error("User.Home is empty")
	}

	// UID should be non-empty
	if facts.UID == "" {
		t.Error("User.UID is empty")
	}
}

func TestGatherCPUFacts(t *testing.T) {
	facts := gatherCPUFacts()

	// Physical CPUs should be at least 1
	if facts.Physical < 1 {
		t.Errorf("CPU.Physical = %d, want >= 1", facts.Physical)
	}

	// Cores should be at least 1
	if facts.Cores < 1 {
		t.Errorf("CPU.Cores = %d, want >= 1", facts.Cores)
	}

	// Cores should be >= Physical (logical cores include hyperthreading)
	if facts.Cores < facts.Physical {
		t.Errorf("CPU.Cores (%d) < CPU.Physical (%d), cores should be >= physical", facts.Cores, facts.Physical)
	}
}

func TestGatherEnvFacts(t *testing.T) {
	facts := gatherEnvFacts()

	// Should not be empty
	if len(facts) == 0 {
		t.Error("Env map is empty")
	}

	// PATH should typically be set
	if _, ok := facts["PATH"]; !ok {
		t.Error("PATH not found in Env map")
	}

	// Test that HOME is captured (should be set in test environment)
	if home, ok := facts["HOME"]; ok {
		if home == "" {
			t.Error("HOME is empty string")
		}
	}
}

func TestGetMachineID(t *testing.T) {
	// This test just verifies the function runs without panicking
	// The actual value depends on the system
	machineID := getMachineID()

	// On Linux, machine ID should be a 32-character hex string
	if runtime.GOOS == "linux" && machineID != "" {
		if len(machineID) != 32 {
			t.Logf("Machine ID length = %d (expected 32 for standard format)", len(machineID))
		}
	}
}

func TestGatherOSFacts(t *testing.T) {
	facts, err := gatherOSFacts()
	if err != nil {
		t.Fatalf("gatherOSFacts() returned error: %v", err)
	}

	// OS name should match runtime
	if facts.Name != runtime.GOOS {
		t.Errorf("OS.Name = %q, want %q", facts.Name, runtime.GOOS)
	}

	// Family should not be empty
	if facts.Family == "" {
		t.Error("OS.Family is empty")
	}

	// On Linux, we should have distribution info
	if runtime.GOOS == "linux" {
		// Distribution might be empty if /etc/os-release doesn't exist
		// but we should at least have a family
		if facts.Family == "" {
			t.Error("OS.Family is empty on Linux")
		}
	}

	// On macOS, we should have distribution and version
	if runtime.GOOS == "darwin" {
		if facts.Distribution != "macOS" {
			t.Errorf("OS.Distribution = %q, want %q", facts.Distribution, "macOS")
		}
		if facts.DistributionVersion == "" {
			t.Error("OS.DistributionVersion is empty on macOS")
		}
		if facts.Family != "darwin" {
			t.Errorf("OS.Family = %q, want %q", facts.Family, "darwin")
		}
	}

	// On FreeBSD, we should have distribution and version
	if runtime.GOOS == "freebsd" {
		if facts.Distribution != "FreeBSD" {
			t.Errorf("OS.Distribution = %q, want %q", facts.Distribution, "FreeBSD")
		}
		if facts.DistributionVersion == "" {
			t.Error("OS.DistributionVersion is empty on FreeBSD")
		}
		if facts.Family != "freebsd" {
			t.Errorf("OS.Family = %q, want %q", facts.Family, "freebsd")
		}
	}

	// On OpenBSD, we should have distribution and version
	if runtime.GOOS == "openbsd" {
		if facts.Distribution != "OpenBSD" {
			t.Errorf("OS.Distribution = %q, want %q", facts.Distribution, "OpenBSD")
		}
		if facts.DistributionVersion == "" {
			t.Error("OS.DistributionVersion is empty on OpenBSD")
		}
		if facts.Family != "openbsd" {
			t.Errorf("OS.Family = %q, want %q", facts.Family, "openbsd")
		}
	}

	// On NetBSD, we should have distribution and version
	if runtime.GOOS == "netbsd" {
		if facts.Distribution != "NetBSD" {
			t.Errorf("OS.Distribution = %q, want %q", facts.Distribution, "NetBSD")
		}
		if facts.DistributionVersion == "" {
			t.Error("OS.DistributionVersion is empty on NetBSD")
		}
		if facts.Family != "netbsd" {
			t.Errorf("OS.Family = %q, want %q", facts.Family, "netbsd")
		}
	}
}
