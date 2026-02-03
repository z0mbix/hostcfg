package facts

import (
	"bufio"
	"os"
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
		// Could use sw_vers to get version, but keeping it simple
		facts.DistributionVersion = ""

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
