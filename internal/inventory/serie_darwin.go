//go:build darwin

package inventory

import (
	"os/exec"
	"strings"
)

// NumeroSerie lê só o número de série (leve; usado no registro do agente).
func NumeroSerie() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "IOPlatformSerialNumber") {
			if p := strings.SplitN(l, "= \"", 2); len(p) == 2 {
				return serieValida(strings.TrimRight(strings.TrimSpace(p[1]), "\""))
			}
		}
	}
	return ""
}
