//go:build !windows

package health

import "syscall"

func discoLivre() float64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0
	}
	return float64(st.Bavail*uint64(st.Bsize)) / (1024 * 1024 * 1024)
}
