//go:build windows

package cloud

import (
	"os"
	"os/exec"
)

func execRestart(self string, args []string) error {
	cmd := exec.Command(self, args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
