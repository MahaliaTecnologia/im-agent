//go:build windows

package telemetry

import (
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Coletar(cloudURL string) Telemetria {
	t := Telemetria{}

	script := `
$os = Get-CimInstance Win32_OperatingSystem
$cpu = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
$disco = Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='$env:SystemDrive'"
$uptime = ((Get-Date) - $os.LastBootUpTime).TotalHours
Write-Host "$($os.TotalVisibleMemorySize)|$($os.FreePhysicalMemory)|$cpu|$($disco.Size)|$($disco.FreeSpace)|$($uptime)"
`
	if out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output(); err == nil {
		fields := strings.Split(strings.TrimSpace(string(out)), "|")
		if len(fields) >= 6 {
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				t.MemTotalMb = v / 1024
			}
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
				t.MemLivreMb = v / 1024
			}
			if v, err := strconv.ParseFloat(fields[2], 64); err == nil {
				t.CPUPct = v
			}
			if v, err := strconv.ParseFloat(fields[3], 64); err == nil {
				t.DiscoTotalGb = round1(v / 1024 / 1024 / 1024)
			}
			if v, err := strconv.ParseFloat(fields[4], 64); err == nil {
				t.DiscoLivreGb = round1(v / 1024 / 1024 / 1024)
			}
			if v, err := strconv.ParseFloat(fields[5], 64); err == nil {
				t.UptimeHoras = round1(v)
			}
		}
	}

	// Latência
	if cloudURL != "" {
		if h, err := url.Parse(cloudURL); err == nil && h.Host != "" {
			t.LatenciaMs, t.PerdaPct, t.JitterMs = ping(h.Hostname())
		}
	}

	return t
}

func ping(host string) (lat, perda, jitter interface{}) {
	const n = 5
	var tempos []float64
	for i := 0; i < n; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", host+":443", 2*time.Second)
		ms := time.Since(start).Seconds() * 1000
		if err == nil {
			conn.Close()
			tempos = append(tempos, ms)
		}
	}
	if len(tempos) == 0 {
		return nil, 100.0, nil
	}
	sum, mn, mx := 0.0, tempos[0], tempos[0]
	for _, v := range tempos {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	avg := sum / float64(len(tempos))
	lost := float64(n-len(tempos)) / n * 100
	return round1(avg), round1(lost), round1(mx - mn)
}

func round1(f float64) float64 {
	return float64(int(f*10)) / 10
}
