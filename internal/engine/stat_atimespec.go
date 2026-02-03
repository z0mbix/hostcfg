//go:build darwin || freebsd || netbsd

package engine

import "syscall"

// getAtime returns the access time in seconds from a syscall.Stat_t.
// On BSD systems (darwin, freebsd, netbsd, openbsd), the field is named Atimespec.
func getAtime(stat *syscall.Stat_t) int64 {
	return stat.Atimespec.Sec
}
