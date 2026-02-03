//go:build linux || openbsd

package engine

import "syscall"

// getAtime returns the access time in seconds from a syscall.Stat_t.
// On Linux, the field is named Atim.
func getAtime(stat *syscall.Stat_t) int64 {
	return stat.Atim.Sec
}
