// Package facts provides Ansible-style system facts for hostcfg.
// Facts are gathered once at startup and made available in HCL expressions
// via the "fact" namespace.
package facts

import (
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/zclconf/go-cty/cty"
)

// Facts contains all gathered system facts
type Facts struct {
	OS              OSFacts
	Arch            string
	Hostname        string
	FQDN            string
	User            UserFacts
	MachineID       string
	CPU             CPUFacts
	Env             map[string]string
	PackageManagers []string
}

// Gather collects all system facts
func Gather() (*Facts, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	fqdn := lookupFQDN(hostname)

	osFacts, err := gatherOSFacts()
	if err != nil {
		// Use defaults on error
		osFacts = OSFacts{
			Name:   runtime.GOOS,
			Family: "unknown",
		}
	}

	userFacts, err := gatherUserFacts()
	if err != nil {
		// Use defaults on error
		userFacts = UserFacts{
			Name: "unknown",
		}
	}

	machineID := getMachineID()
	cpuFacts := gatherCPUFacts()
	envFacts := gatherEnvFacts()
	packageManagerFacts := gatherPackageManagerFacts()

	return &Facts{
		OS:              osFacts,
		Arch:            runtime.GOARCH,
		Hostname:        hostname,
		FQDN:            fqdn,
		User:            userFacts,
		MachineID:       machineID,
		CPU:             cpuFacts,
		Env:             envFacts,
		PackageManagers: packageManagerFacts,
	}, nil
}

// getMachineID reads the machine ID from /etc/machine-id (Linux)
// or returns an empty string on unsupported platforms
func getMachineID() string {
	// Primary location on most Linux systems
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Fallback location (older systems or systems using dbus)
	if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		return strings.TrimSpace(string(data))
	}

	return ""
}

// lookupFQDN attempts to get the fully qualified domain name
func lookupFQDN(hostname string) string {
	// Try to look up the FQDN via DNS
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		return hostname
	}

	// Reverse lookup the first address
	names, err := net.LookupAddr(addrs[0])
	if err != nil || len(names) == 0 {
		return hostname
	}

	// Remove trailing dot if present
	fqdn := names[0]
	if len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.' {
		fqdn = fqdn[:len(fqdn)-1]
	}

	return fqdn
}

// ToCtyValue converts Facts to a cty.Value for use in HCL expressions
func (f *Facts) ToCtyValue() cty.Value {
	// Convert environment variables map to cty
	envMap := make(map[string]cty.Value)
	for k, v := range f.Env {
		envMap[k] = cty.StringVal(v)
	}

	// Convert package managers slice to cty
	var packageManagersCty cty.Value
	if len(f.PackageManagers) == 0 {
		packageManagersCty = cty.ListValEmpty(cty.String)
	} else {
		pmVals := make([]cty.Value, len(f.PackageManagers))
		for i, pm := range f.PackageManagers {
			pmVals[i] = cty.StringVal(pm)
		}
		packageManagersCty = cty.ListVal(pmVals)
	}

	return cty.ObjectVal(map[string]cty.Value{
		"os": cty.ObjectVal(map[string]cty.Value{
			"name":                 cty.StringVal(f.OS.Name),
			"family":               cty.StringVal(f.OS.Family),
			"distribution":         cty.StringVal(f.OS.Distribution),
			"distribution_version": cty.StringVal(f.OS.DistributionVersion),
		}),
		"arch":             cty.StringVal(f.Arch),
		"hostname":         cty.StringVal(f.Hostname),
		"fqdn":             cty.StringVal(f.FQDN),
		"machine_id":       cty.StringVal(f.MachineID),
		"package_managers": packageManagersCty,
		"cpu": cty.ObjectVal(map[string]cty.Value{
			"physical": cty.NumberIntVal(int64(f.CPU.Physical)),
			"cores":    cty.NumberIntVal(int64(f.CPU.Cores)),
		}),
		"user": cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal(f.User.Name),
			"home": cty.StringVal(f.User.Home),
			"uid":  cty.StringVal(f.User.UID),
			"gid":  cty.StringVal(f.User.GID),
		}),
		"env": cty.ObjectVal(envMap),
	})
}
