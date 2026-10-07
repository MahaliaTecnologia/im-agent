//go:build linux

package inventory

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func Coletar() Inventario {
	hw := Hardware{
		Sistema: sistema(),
	}

	// DMI
	hw.Fabricante = lerDMI("sys_vendor")
	hw.Modelo = lerDMI("product_name")
	hw.Serie = lerDMI("product_serial")
	hw.BiosData = lerDMI("bios_date")

	// CPU
	if v := lerCPUInfo("model name"); v != "" {
		hw.Processador = v
	}
	if out, _ := exec.Command("nproc").Output(); len(out) > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			hw.CPUNucleos = n
		}
	}

	// Memória
	if v := lerProcMeminfo("MemTotal"); v > 0 {
		hw.MemoriaMb = v / 1024
	}

	// Disco raiz
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) == nil {
		total := float64(st.Blocks*uint64(st.Bsize)) / (1024 * 1024 * 1024)
		hw.DiscoTotalGb = roundf(total)
	}

	// GPU
	if out, _ := exec.Command("lspci").Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "VGA compatible controller") {
				parts := strings.SplitN(l, ":", 3)
				if len(parts) >= 3 {
					hw.GPU = strings.TrimSpace(parts[2])
					break
				}
			}
		}
	}

	// TPM
	if _, err := os.Stat("/dev/tpm0"); err == nil {
		hw.TPM = "presente"
	} else if _, err := os.Stat("/dev/tpmrm0"); err == nil {
		hw.TPM = "presente"
	}

	// Bateria
	for _, bat := range globFirst("/sys/class/power_supply/BAT*") {
		pct, _ := lerArquivo(bat + "/capacity")
		ciclos, _ := lerArquivo(bat + "/cycle_count")
		status, _ := lerArquivo(bat + "/status")
		hw.Bateria = map[string]interface{}{
			"saude":                status,
			"ciclos":               toIntOr(ciclos, nil),
			"capacidadePercentual": toIntOr(pct, nil),
		}
		break
	}

	// Discos
	if out, _ := exec.Command("df", "-k", "-P").Output(); len(out) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		scanner.Scan() // pula header
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 6 || !strings.HasPrefix(fields[0], "/") {
				continue
			}
			sz, _ := strconv.ParseFloat(fields[1], 64)
			fr, _ := strconv.ParseFloat(fields[3], 64)
			hw.Discos = append(hw.Discos, Disco{
				Nome:      fields[5],
				TamanhoGb: roundf(sz / 1048576),
				LivreGb:   roundf(fr / 1048576),
			})
		}
	}

	// Rede
	entries, _ := os.ReadDir("/sys/class/net")
	for _, e := range entries {
		n := e.Name()
		if n == "lo" {
			continue
		}
		mac, _ := lerArquivo("/sys/class/net/" + n + "/address")
		if mac == "" {
			continue
		}
		iface := Iface{Nome: n, MAC: mac}
		if out, _ := exec.Command("ip", "-4", "-o", "addr", "show", n).Output(); len(out) > 0 {
			fields := strings.Fields(string(out))
			for i, f := range fields {
				if f == "inet" && i+1 < len(fields) {
					ip := strings.Split(fields[i+1], "/")[0]
					iface.IP = ip
					break
				}
			}
		}
		hw.Rede = append(hw.Rede, iface)
	}

	// Programas instalados
	var progs []Programa
	if out, _ := exec.Command("dpkg-query", "-W", "-f=${Package}\t${Version}\n").Output(); len(out) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), "\t", 2)
			if len(parts) == 2 && parts[0] != "" {
				progs = append(progs, Programa{Nome: parts[0], Versao: parts[1]})
				if len(progs) >= 500 {
					break
				}
			}
		}
	} else if out, _ := exec.Command("rpm", "-qa", "--qf", "%{NAME}\t%{VERSION}\n").Output(); len(out) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), "\t", 2)
			if len(parts) == 2 && parts[0] != "" {
				progs = append(progs, Programa{Nome: parts[0], Versao: parts[1]})
				if len(progs) >= 500 {
					break
				}
			}
		}
	}

	return Inventario{Hardware: hw, Programas: progs}
}

func sistema() string {
	var u syscall.Utsname
	syscall.Uname(&u)
	s := charsToString(u.Sysname[:])
	r := charsToString(u.Release[:])
	return s + " " + r
}

func charsToString(arr []int8) string {
	b := make([]byte, len(arr))
	for i, v := range arr {
		if v == 0 {
			break
		}
		b[i] = byte(v)
	}
	return strings.TrimRight(string(b), "\x00")
}

func lerDMI(campo string) interface{} {
	v, err := lerArquivo("/sys/class/dmi/id/" + campo)
	if err != nil || v == "" {
		return nil
	}
	return v
}

func lerCPUInfo(campo string) string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	prefix := campo + "\t: "
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, campo) && strings.Contains(l, ":") {
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	_ = prefix
	return ""
}

func lerProcMeminfo(campo string) int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, campo) {
			fields := strings.Fields(l)
			if len(fields) >= 2 {
				v, _ := strconv.Atoi(fields[1])
				return v
			}
		}
	}
	return 0
}

func lerArquivo(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.TrimRight(string(b), "\x00")), nil
}

func globFirst(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	return m
}

func toIntOr(s string, def interface{}) interface{} {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

func roundf(f float64) float64 {
	return float64(int(f*10)) / 10
}
