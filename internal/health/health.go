package health

import (
	"time"

	"github.com/insightmanager/im-agent/internal/cloud"
)

var startTime = time.Now()

func Coletar() cloud.Saude {
	s := cloud.Saude{OpsiAtivo: nil}

	// Disco livre na raiz
	if gb := discoLivre(); gb > 0 {
		s.DiscoLivreGb = gb
	}

	// Uptime em horas
	s.UptimeHoras = time.Since(startTime).Hours()

	return s
}
