//go:build windows

package inventory

import (
	"os/exec"
	"strings"
)

// NumeroSerie lê só o número de série (leve; usado no registro do agente).
func NumeroSerie() string {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", "(Get-CimInstance Win32_BIOS).SerialNumber").Output()
	if err != nil {
		return ""
	}
	return serieValida(strings.TrimSpace(string(out)))
}
