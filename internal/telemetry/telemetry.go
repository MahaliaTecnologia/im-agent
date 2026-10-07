package telemetry

// Telemetria representa as métricas de experiência do equipamento.
type Telemetria struct {
	BootSegundos   interface{} `json:"bootSegundos"`
	CPUPct         interface{} `json:"cpuPct"`
	MemTotalMb     interface{} `json:"memTotalMb"`
	MemLivreMb     interface{} `json:"memLivreMb"`
	DiscoTotalGb   interface{} `json:"discoTotalGb"`
	DiscoLivreGb   interface{} `json:"discoLivreGb"`
	UptimeHoras    interface{} `json:"uptimeHoras"`
	Travamentos24h int         `json:"travamentos24h"`
	Reinicios24h   int         `json:"reinicios24h"`
	LatenciaMs     interface{} `json:"latenciaMs"`
	PerdaPct       interface{} `json:"perdaPct"`
	JitterMs       interface{} `json:"jitterMs"`
	RedeTipo       interface{} `json:"redeTipo"`
	Aplicacoes     []App       `json:"aplicacoes"`
}

type App struct {
	Nome        string  `json:"nome"`
	Execucoes   int     `json:"execucoes"`
	Travamentos int     `json:"travamentos"`
	MemoriaMb   float64 `json:"memoriaMb,omitempty"`
}

// Normalizar garante listas vazias em vez de null no JSON enviado à nuvem.
func Normalizar(t Telemetria) Telemetria {
	if t.Aplicacoes == nil {
		t.Aplicacoes = []App{}
	}
	return t
}
