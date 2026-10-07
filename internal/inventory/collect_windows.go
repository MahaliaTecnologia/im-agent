//go:build windows

package inventory

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func Coletar() Inventario {
	hw := Hardware{}

	script := `
$cs = Get-CimInstance Win32_ComputerSystem
$bios = Get-CimInstance Win32_BIOS
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$os = Get-CimInstance Win32_OperatingSystem
$disco = Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"
$gpu = (Get-CimInstance Win32_VideoController | Select-Object -First 1).Name
Write-Host "$($cs.Manufacturer)|$($cs.Model)|$($bios.SerialNumber)|$($cpu.Name)|$($cpu.NumberOfCores)|$($cs.TotalPhysicalMemory)|$($disco.Size)|$($bios.ReleaseDate)|$($os.Caption)|$gpu"
`
	if out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output(); err == nil {
		fields := strings.Split(strings.TrimSpace(string(out)), "|")
		if len(fields) >= 10 {
			hw.Fabricante = nullStr(fields[0])
			hw.Modelo = nullStr(fields[1])
			hw.Serie = nullStr(fields[2])
			hw.Processador = nullStr(fields[3])
			if v, err := strconv.Atoi(fields[4]); err == nil {
				hw.CPUNucleos = v
			}
			if v, err := strconv.ParseFloat(fields[5], 64); err == nil {
				hw.MemoriaMb = int(v / 1024 / 1024)
			}
			if v, err := strconv.ParseFloat(fields[6], 64); err == nil {
				hw.DiscoTotalGb = roundf(v / 1024 / 1024 / 1024)
			}
			hw.BiosData = nullStr(fields[7])
			hw.Sistema = fields[8]
			hw.GPU = nullStr(fields[9])
		}
	}

	// Discos
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ForEach-Object { Write-Host "$($_.DeviceID)|$($_.Size)|$($_.FreeSpace)" }`).Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			fields := strings.Split(strings.TrimSpace(l), "|")
			if len(fields) < 3 {
				continue
			}
			sz, _ := strconv.ParseFloat(fields[1], 64)
			fr, _ := strconv.ParseFloat(fields[2], 64)
			hw.Discos = append(hw.Discos, Disco{
				Nome:      fields[0],
				TamanhoGb: roundf(sz / 1024 / 1024 / 1024),
				LivreGb:   roundf(fr / 1024 / 1024 / 1024),
			})
		}
	}

	// Rede
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`Get-CimInstance Win32_NetworkAdapterConfiguration | Where-Object { $_.MACAddress } | ForEach-Object { $ip = if ($_.IPAddress) { ($_.IPAddress | Where-Object { $_ -notmatch ':' } | Select-Object -First 1) } else { "" }; Write-Host "$($_.Description)|$($_.MACAddress)|$ip" }`).Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			fields := strings.Split(strings.TrimSpace(l), "|")
			if len(fields) < 2 || fields[1] == "" {
				continue
			}
			i := Iface{Nome: fields[0], MAC: fields[1]}
			if len(fields) >= 3 && fields[2] != "" {
				i.IP = fields[2]
			}
			hw.Rede = append(hw.Rede, i)
			if len(hw.Rede) >= 32 {
				break
			}
		}
	}

	// TPM
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`try { $t = Get-CimInstance -Namespace "root\CIMV2\Security\MicrosoftTpm" -ClassName Win32_Tpm -EA Stop; Write-Host $t.SpecVersion } catch { }`).Output(); len(out) > 0 {
		v := strings.TrimSpace(string(out))
		if v != "" {
			hw.TPM = v
		}
	}

	// Memória slots
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`Get-CimInstance Win32_PhysicalMemory | ForEach-Object { Write-Host "$($_.DeviceLocator)|$($_.Capacity)|$($_.SMBIOSMemoryType)|$($_.Speed)" }`).Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			fields := strings.Split(strings.TrimSpace(l), "|")
			if len(fields) < 4 || fields[0] == "" {
				continue
			}
			cap, _ := strconv.ParseFloat(fields[1], 64)
			hw.MemoriaSlots = append(hw.MemoriaSlots, Slot{
				Slot:       fields[0],
				TamanhoMb:  int(cap / 1024 / 1024),
				Tipo:       fmt.Sprintf("Tipo %s", fields[2]),
				Velocidade: fields[3] + " MHz",
			})
			if len(hw.MemoriaSlots) >= 32 {
				break
			}
		}
	}

	// Bateria
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`$b = Get-CimInstance Win32_Battery | Select-Object -First 1; if ($b) { Write-Host "$($b.BatteryStatus)|$($b.EstimatedChargeRemaining)" }`).Output(); len(out) > 0 {
		fields := strings.Split(strings.TrimSpace(string(out)), "|")
		if len(fields) == 2 {
			status := "Em uso"
			if fields[0] == "2" {
				status = "Na tomada"
			}
			pct, _ := strconv.Atoi(fields[1])
			hw.Bateria = map[string]interface{}{
				"saude":                status,
				"ciclos":               nil,
				"capacidadePercentual": pct,
			}
		}
	}

	// Monitores
	if out, _ := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`Get-CimInstance -Namespace "root\wmi" -ClassName WmiMonitorID -EA SilentlyContinue | ForEach-Object { $n = if ($_.UserFriendlyName) { -join ($_.UserFriendlyName | Where-Object { $_ -gt 0 } | ForEach-Object { [char]$_ }) } else { "" }; Write-Host $n }`).Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				hw.Monitores = append(hw.Monitores, Monitor{Nome: l})
			}
			if len(hw.Monitores) >= 16 {
				break
			}
		}
	}

	// Programas via registro
	var progs []Programa
	chaves := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, base := range chaves {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, base, registry.READ)
		if err != nil {
			continue
		}
		subs, _ := k.ReadSubKeyNames(-1)
		for _, sub := range subs {
			sk, err := registry.OpenKey(k, sub, registry.READ)
			if err != nil {
				continue
			}
			nome, _, _ := sk.GetStringValue("DisplayName")
			versao, _, _ := sk.GetStringValue("DisplayVersion")
			editor, _, _ := sk.GetStringValue("Publisher")
			data, _, _ := sk.GetStringValue("InstallDate")
			sk.Close()
			if nome == "" {
				continue
			}
			progs = append(progs, Programa{
				Nome:        nome,
				Versao:      versao,
				Editor:      editor,
				InstaladoEm: data,
			})
			if len(progs) >= 600 {
				break
			}
		}
		k.Close()
	}

	return Inventario{Hardware: hw, Programas: progs}
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func roundf(f float64) float64 {
	return float64(int(f*10)) / 10
}
