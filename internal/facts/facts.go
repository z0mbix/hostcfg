// Package facts provides Ansible-style system facts for hostcfg.
// Facts are gathered once at startup and made available in HCL expressions
// via the "fact" namespace.
package facts

import (
	"net"
	"os"
	"runtime"

	"github.com/zclconf/go-cty/cty"
)

// Facts contains all gathered system facts
type Facts struct {
	OS       OSFacts
	Arch     string
	Hostname string
	FQDN     string
	User     UserFacts
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

	return &Facts{
		OS:       osFacts,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		FQDN:     fqdn,
		User:     userFacts,
	}, nil
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
	return cty.ObjectVal(map[string]cty.Value{
		"os": cty.ObjectVal(map[string]cty.Value{
			"name":                 cty.StringVal(f.OS.Name),
			"family":               cty.StringVal(f.OS.Family),
			"distribution":         cty.StringVal(f.OS.Distribution),
			"distribution_version": cty.StringVal(f.OS.DistributionVersion),
		}),
		"arch":     cty.StringVal(f.Arch),
		"hostname": cty.StringVal(f.Hostname),
		"fqdn":     cty.StringVal(f.FQDN),
		"user": cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal(f.User.Name),
			"home": cty.StringVal(f.User.Home),
			"uid":  cty.StringVal(f.User.UID),
			"gid":  cty.StringVal(f.User.GID),
		}),
	})
}
