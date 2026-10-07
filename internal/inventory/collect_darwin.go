//go:build darwin

package inventory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Coletar() Inventario {
	hw := Hardware{}

	hw.Fabricante = "Apple"

	if out, _ := exec.Command("sysctl", "-n", "hw.model").Output(); len(out) > 0 {
		hw.Modelo = strings.TrimSpace(string(out))
	}

	// Serial via ioreg
	if out, _ := exec.Command("ioreg", "-l").Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "IOPlatformSerialNumber") {
				parts := strings.SplitN(l, "= \"", 2)
				if len(parts) == 2 {
					hw.Serie = strings.TrimRight(parts[1], "\"")
					break
				}
			}
		}
	}

	if out, _ := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); len(out) > 0 {
		hw.Processador = strings.TrimSpace(string(out))
	}

	if out, _ := exec.Command("sysctl", "-n", "hw.physicalcpu").Output(); len(out) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			hw.CPUNucleos = v
		}
	}

	if out, _ := exec.Command("sysctl", "-n", "hw.memsize").Output(); len(out) > 0 {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			hw.MemoriaMb = int(v / 1024 / 1024)
		}
	}

	if out, _ := exec.Command("sw_vers", "-productVersion").Output(); len(out) > 0 {
		hw.Sistema = "Darwin " + strings.TrimSpace(string(out))
	}

	// GPU
	if out, _ := exec.Command("system_profiler", "SPDisplaysDataType").Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "Chipset Model:") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					hw.GPU = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	// Bateria
	if out, _ := exec.Command("ioreg", "-r", "-c", "AppleSmartBattery").Output(); len(out) > 0 {
		var ciclos, pct interface{}
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "\"CycleCount\"") {
				parts := strings.SplitN(l, "= ", 2)
				if len(parts) == 2 {
					if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						ciclos = v
					}
				}
			}
		}
		if out2, _ := exec.Command("pmset", "-g", "batt").Output(); len(out2) > 0 {
			s := string(out2)
			if idx := strings.Index(s, "%"); idx > 0 {
				start := idx - 1
				for start > 0 && s[start-1] >= '0' && s[start-1] <= '9' {
					start--
				}
				if v, err := strconv.Atoi(s[start:idx]); err == nil {
					pct = v
				}
			}
		}
		hw.Bateria = map[string]interface{}{
			"saude":                nil,
			"ciclos":               ciclos,
			"capacidadePercentual": pct,
		}
	}

	// Rede
	if out, _ := exec.Command("ifconfig", "-l").Output(); len(out) > 0 {
		for _, iface := range strings.Fields(string(out)) {
			if iface == "lo0" {
				continue
			}
			if out2, _ := exec.Command("ifconfig", iface).Output(); len(out2) > 0 {
				mac := ""
				for _, l := range strings.Split(string(out2), "\n") {
					if strings.Contains(l, "ether") {
						fields := strings.Fields(l)
						if len(fields) >= 2 {
							mac = fields[1]
						}
					}
				}
				if mac == "" {
					continue
				}
				ip := ""
				if out3, _ := exec.Command("ipconfig", "getifaddr", iface).Output(); len(out3) > 0 {
					ip = strings.TrimSpace(string(out3))
				}
				i := Iface{Nome: iface, MAC: mac}
				if ip != "" {
					i.IP = ip
				}
				hw.Rede = append(hw.Rede, i)
			}
		}
	}

	// Programas instalados
	var progs []Programa
	entries, _ := filepath.Glob("/Applications/*.app")
	entries2, _ := filepath.Glob("/Applications/*/*.app")
	for _, e := range append(entries, entries2...) {
		nome := strings.TrimSuffix(filepath.Base(e), ".app")
		ver := ""
		if out, _ := exec.Command("defaults", "read",
			e+"/Contents/Info", "CFBundleShortVersionString").Output(); len(out) > 0 {
			ver = strings.TrimSpace(string(out))
		}
		progs = append(progs, Programa{Nome: nome, Versao: ver})
		if len(progs) >= 300 {
			break
		}
	}

	_ = os.Stat

	return Inventario{Hardware: hw, Programas: progs}
}
