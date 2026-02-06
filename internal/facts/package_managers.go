package facts

import (
	"os/exec"
	"sort"
)

var packageManagerCommands = []string{
	"apt", "apt-get", "apk", "dnf", "dpkg", "emerge",
	"pacman", "rpm", "xbps-install", "yum", "zypper",
	"brew", "port",
	"pkg", "pkg_add", "pkgin",
	"flatpak", "snap", "nix-env",
}

func gatherPackageManagerFacts() []string {
	var available []string
	for _, cmd := range packageManagerCommands {
		if _, err := exec.LookPath(cmd); err == nil {
			available = append(available, cmd)
		}
	}
	sort.Strings(available)
	return available
}
