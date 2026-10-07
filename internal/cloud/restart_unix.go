//go:build !windows

package cloud

import "syscall"

func execRestart(self string, args []string) error {
	return syscall.Exec(self, args, nil)
}
