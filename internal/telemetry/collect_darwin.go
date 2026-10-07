//go:build darwin

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

	// Memória total
	if out, _ := exec.Command("sysctl", "-n", "hw.memsize").Output(); len(out) > 0 {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			t.MemTotalMb = v / 1024 / 1024
		}
	}

	// Uptime via kern.boottime
	if out, _ := exec.Command("sysctl", "-n", "kern.boottime").Output(); len(out) > 0 {
		s := string(out)
		if idx := strings.Index(s, "sec = "); idx >= 0 {
			rest := s[idx+6:]
			if idx2 := strings.IndexAny(rest, ", }"); idx2 >= 0 {
				if sec, err := strconv.ParseInt(rest[:idx2], 10, 64); err == nil {
					upSec := time.Now().Unix() - sec
					t.UptimeHoras = float64(upSec) / 3600
				}
			}
		}
	}

	// CPU (via ps)
	if out, _ := exec.Command("ps", "-A", "-o", "%cpu").Output(); len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		sum := 0.0
		for _, l := range lines[1:] {
			if v, err := strconv.ParseFloat(strings.TrimSpace(l), 64); err == nil {
				sum += v
			}
		}
		ncpu := 1
		if out2, _ := exec.Command("sysctl", "-n", "hw.ncpu").Output(); len(out2) > 0 {
			ncpu, _ = strconv.Atoi(strings.TrimSpace(string(out2)))
			if ncpu < 1 {
				ncpu = 1
			}
		}
		t.CPUPct = round1(sum / float64(ncpu))
	}

	// Latência
	if cloudURL != "" {
		if h, err := url.Parse(cloudURL); err == nil && h.Host != "" {
			t.LatenciaMs, t.PerdaPct, t.JitterMs = ping(h.Hostname())
		}
	}

	// Interface de rede
	if out, _ := exec.Command("route", "-n", "get", "default").Output(); len(out) > 0 {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "interface:") {
				fields := strings.Fields(l)
				if len(fields) >= 2 {
					t.RedeTipo = fields[1]
				}
				break
			}
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
