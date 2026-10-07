//go:build linux

package telemetry

import (
	"bufio"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Coletar(cloudURL string) Telemetria {
	t := Telemetria{}

	// Uptime
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				t.UptimeHoras = v / 3600
			}
		}
	}

	// CPU (load avg / nproc)
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if load, err := strconv.ParseFloat(fields[0], 64); err == nil {
				nproc := 1
				if out, _ := exec.Command("nproc").Output(); len(out) > 0 {
					nproc, _ = strconv.Atoi(strings.TrimSpace(string(out)))
					if nproc < 1 {
						nproc = 1
					}
				}
				t.CPUPct = load * 100 / float64(nproc)
			}
		}
	}

	// Memória
	f, err := os.Open("/proc/meminfo")
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			l := scanner.Text()
			if strings.HasPrefix(l, "MemTotal") {
				fields := strings.Fields(l)
				if len(fields) >= 2 {
					v, _ := strconv.Atoi(fields[1])
					t.MemTotalMb = v / 1024
				}
			}
			if strings.HasPrefix(l, "MemAvailable") {
				fields := strings.Fields(l)
				if len(fields) >= 2 {
					v, _ := strconv.Atoi(fields[1])
					t.MemLivreMb = v / 1024
				}
			}
		}
		f.Close()
	}

	// Disco
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) == nil {
		total := float64(st.Blocks*uint64(st.Bsize)) / (1024 * 1024 * 1024)
		livre := float64(st.Bavail*uint64(st.Bsize)) / (1024 * 1024 * 1024)
		t.DiscoTotalGb = round1(total)
		t.DiscoLivreGb = round1(livre)
	}

	// Boot (systemd-analyze)
	if out, err := exec.Command("systemd-analyze", "time").Output(); err == nil {
		s := string(out)
		// Procura "= X.Xs"
		if idx := strings.Index(s, "= "); idx >= 0 {
			rest := s[idx+2:]
			if idx2 := strings.Index(rest, "s"); idx2 >= 0 {
				v, _ := strconv.ParseFloat(rest[:idx2], 64)
				t.BootSegundos = v
			}
		}
	}

	// Rede — latência para o cloud
	if cloudURL != "" {
		if h, err := url.Parse(cloudURL); err == nil && h.Host != "" {
			t.LatenciaMs, t.PerdaPct, t.JitterMs = ping(h.Hostname())
		}
	}

	// Interface de rede principal
	if out, _ := exec.Command("ip", "route", "get", "1.1.1.1").Output(); len(out) > 0 {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				t.RedeTipo = fields[i+1]
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
