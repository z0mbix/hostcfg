package facts

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OSFacts contains operating system information
type OSFacts struct {
	Name                string // OS name (linux, darwin)
	Family              string // OS family (debian, redhat, arch, darwin)
	Distribution        string // Distribution name (Ubuntu, Fedora)
	DistributionVersion string // Version (22.04, 39)
}

// gatherOSFacts collects OS-related facts
func gatherOSFacts() (OSFacts, error) {
	facts := OSFacts{
		Name: runtime.GOOS,
	}

	switch runtime.GOOS {
	case "linux":
		osRelease, err := parseOSRelease()
		if err != nil {
			facts.Family = "unknown"
			return facts, nil
		}

		facts.Distribution = osRelease["NAME"]
		// Remove quotes if present
		facts.Distribution = strings.Trim(facts.Distribution, "\"")

		facts.DistributionVersion = osRelease["VERSION_ID"]
		facts.DistributionVersion = strings.Trim(facts.DistributionVersion, "\"")

		facts.Family = detectLinuxFamily(osRelease)

	case "darwin":
		facts.Family = "darwin"
		facts.Distribution = "macOS"
		facts.DistributionVersion = getMacOSVersion()

	case "freebsd":
		facts.Family = "freebsd"
		facts.Distribution = "FreeBSD"
		facts.DistributionVersion = getUnameVersion()

	case "openbsd":
		facts.Family = "openbsd"
		facts.Distribution = "OpenBSD"
		facts.DistributionVersion = getUnameVersion()

	case "netbsd":
		facts.Family = "netbsd"
		facts.Distribution = "NetBSD"
		facts.DistributionVersion = getUnameVersion()

	case "illumos":
		facts.Family = "illumos"
		facts.Distribution = getIllumosDistribution()
		facts.DistributionVersion = getUnameVersion()

	case "dragonfly":
		facts.Family = "dragonfly"
		facts.Distribution = "DragonFly BSD"
		facts.DistributionVersion = getUnameVersion()

	default:
		facts.Family = runtime.GOOS
	}

	return facts, nil
}

// parseOSRelease reads and parses /etc/os-release
func parseOSRelease() (map[string]string, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "\"")
		result[key] = value
	}

	return result, scanner.Err()
}

// detectLinuxFamily determines the OS family from /etc/os-release
func detectLinuxFamily(osRelease map[string]string) string {
	// First check the ID field
	id := strings.ToLower(osRelease["ID"])

	// Direct matches
	switch id {
	case "debian", "ubuntu", "linuxmint", "pop", "elementary", "kali", "raspbian":
		return "debian"
	case "fedora", "rhel", "centos", "rocky", "alma", "oracle", "amzn", "amazon":
		return "redhat"
	case "arch", "manjaro", "endeavouros", "garuda":
		return "arch"
	case "opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles":
		return "suse"
	case "alpine":
		return "alpine"
	case "gentoo":
		return "gentoo"
	case "void":
		return "void"
	case "nixos":
		return "nixos"
	}

	// Check ID_LIKE for derivatives
	idLike := strings.ToLower(osRelease["ID_LIKE"])
	if idLike != "" {
		likes := strings.Fields(idLike)
		for _, like := range likes {
			switch like {
			case "debian", "ubuntu":
				return "debian"
			case "fedora", "rhel", "centos":
				return "redhat"
			case "arch":
				return "arch"
			case "suse", "opensuse":
				return "suse"
			}
		}
	}

	return "unknown"
}

// getMacOSVersion gets the macOS version using sw_vers
func getMacOSVersion() string {
	out, err := exec.Command("sw_vers", "--productVersion").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getUnameVersion gets the OS version using uname -r (for BSDs)
func getUnameVersion() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getIllumosDistribution detects the illumos distribution name
func getIllumosDistribution() string {
	// Try /etc/os-release first (OmniOS, OpenIndiana)
	osRelease, err := parseOSRelease()
	if err == nil {
		if name := strings.Trim(osRelease["NAME"], "\""); name != "" {
			return name
		}
	}

	// Fall back to /etc/release (SmartOS)
	data, err := os.ReadFile("/etc/release")
	if err == nil {
		line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
		if line != "" {
			return line
		}
	}

	return "illumos"
}
