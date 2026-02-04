package facts

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// CPUFacts contains CPU information
type CPUFacts struct {
	Physical int // Number of physical CPUs/sockets
	Cores    int // Total number of CPU cores (logical processors)
}

// gatherCPUFacts collects CPU-related facts
func gatherCPUFacts() CPUFacts {
	facts := CPUFacts{}

	switch runtime.GOOS {
	case "darwin":
		facts.Physical = getSysctlInt("hw.physicalcpu")
		facts.Cores = getSysctlInt("hw.logicalcpu")

	case "linux":
		facts.Physical, facts.Cores = getLinuxCPUInfo()

	case "freebsd", "openbsd", "netbsd", "dragonfly":
		// BSD systems: ncpu gives logical CPUs
		facts.Cores = getSysctlInt("hw.ncpu")
		// Physical CPUs harder to get on BSD, default to cores
		facts.Physical = facts.Cores

	default:
		// Fallback to runtime.NumCPU() for unknown systems
		facts.Cores = runtime.NumCPU()
		facts.Physical = facts.Cores
	}

	// Ensure we have at least 1 for both values
	if facts.Physical < 1 {
		facts.Physical = 1
	}
	if facts.Cores < 1 {
		facts.Cores = runtime.NumCPU()
		if facts.Cores < 1 {
			facts.Cores = 1
		}
	}

	return facts
}

// getSysctlInt retrieves an integer value from sysctl
func getSysctlInt(key string) int {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return 0
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return val
}

// getLinuxCPUInfo parses /proc/cpuinfo to get physical and logical CPU counts
func getLinuxCPUInfo() (physical, cores int) {
	// Try nproc for logical CPUs first (most reliable)
	if out, err := exec.Command("nproc").Output(); err == nil {
		if val, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			cores = val
		}
	}

	// For physical CPUs, try lscpu
	if out, err := exec.Command("lscpu").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var sockets, coresPerSocket int
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Socket(s)":
				sockets, _ = strconv.Atoi(value)
			case "Core(s) per socket":
				coresPerSocket, _ = strconv.Atoi(value)
			case "CPU(s)":
				if cores == 0 {
					cores, _ = strconv.Atoi(value)
				}
			}
		}
		if sockets > 0 && coresPerSocket > 0 {
			physical = sockets * coresPerSocket
		} else if sockets > 0 {
			physical = sockets
		}
	}

	// Fallback to runtime.NumCPU if we couldn't get the values
	if cores == 0 {
		cores = runtime.NumCPU()
	}
	if physical == 0 {
		physical = cores
	}

	return physical, cores
}
