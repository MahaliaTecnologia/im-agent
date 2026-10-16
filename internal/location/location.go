// Package location obtém a posição do equipamento pelo serviço de localização
// do sistema operacional. Só roda quando a nuvem pede (localização ligada na
// organização); o agente nunca coleta por conta própria.
package location

import (
	"strconv"
	"strings"
	"time"
)

// Status enviados à nuvem.
const (
	StatusOK           = "ok"
	StatusNegada       = "negada"        // serviço de localização desligado ou sem permissão
	StatusIndisponivel = "indisponivel"  // sem posição (sem Wi-Fi/GPS, tempo esgotado)
	StatusNaoSuportado = "nao_suportado" // sistema ainda sem coleta (macOS)
	StatusErro         = "erro"
)

// Posicao é o corpo enviado a /api/public/device-agent/location.
type Posicao struct {
	Status      string   `json:"status"`
	Lat         *float64 `json:"lat,omitempty"`
	Lon         *float64 `json:"lon,omitempty"`
	PrecisaoM   *float64 `json:"precisaoM,omitempty"`
	CapturadaEm string   `json:"capturadaEm,omitempty"`
	Fonte       string   `json:"fonte,omitempty"`
	Detalhe     string   `json:"detalhe,omitempty"`
}

// Coletar consulta o serviço de localização do sistema (até ~40 s).
func Coletar() Posicao { return coletarOS() }

func ok(lat, lon, prec float64, fonte string) Posicao {
	p := Posicao{Status: StatusOK, Fonte: fonte, CapturadaEm: time.Now().UTC().Format(time.RFC3339)}
	if !coordenadaValida(lat, lon) {
		return Posicao{Status: StatusErro, Fonte: fonte, Detalhe: "coordenada inválida"}
	}
	p.Lat, p.Lon = &lat, &lon
	if prec > 0 && prec < 100000 {
		p.PrecisaoM = &prec
	}
	return p
}

func coordenadaValida(lat, lon float64) bool {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	return !(lat == 0 && lon == 0)
}

func numero(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "°")
	s = strings.Fields(s + " ")[0]
	s = strings.ReplaceAll(s, ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func curto(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// interpretarWindows lê a saída do script PowerShell (GeoCoordinateWatcher):
// "ok|lat|lon|precisao", "negada" ou "indisponivel|<status>".
func interpretarWindows(saida string) Posicao {
	linhas := strings.Split(strings.TrimSpace(saida), "\n")
	ultima := strings.TrimSpace(linhas[len(linhas)-1])
	partes := strings.Split(ultima, "|")
	switch partes[0] {
	case "ok":
		if len(partes) >= 4 {
			lat, a := numero(partes[1])
			lon, b := numero(partes[2])
			prec, _ := numero(partes[3])
			if a && b {
				return ok(lat, lon, prec, "windows")
			}
		}
		return Posicao{Status: StatusErro, Fonte: "windows", Detalhe: "resposta incompleta"}
	case "negada":
		return Posicao{Status: StatusNegada, Fonte: "windows", Detalhe: "Serviços de localização desligados ou sem permissão para aplicativos da área de trabalho"}
	case "indisponivel":
		d := "sem posição"
		if len(partes) > 1 {
			d = "sem posição (" + curto(partes[1]) + ")"
		}
		return Posicao{Status: StatusIndisponivel, Fonte: "windows", Detalhe: d}
	}
	return Posicao{Status: StatusErro, Fonte: "windows", Detalhe: curto(ultima)}
}

// interpretarGeoClue lê a saída do where-am-i do GeoClue (usa o último bloco).
func interpretarGeoClue(saida string) Posicao {
	var lat, lon, prec float64
	var temLat, temLon bool
	for _, l := range strings.Split(saida, "\n") {
		chave, valor, achou := strings.Cut(strings.TrimSpace(l), ":")
		if !achou {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(chave)) {
		case "latitude":
			lat, temLat = numero(valor)
		case "longitude":
			lon, temLon = numero(valor)
		case "accuracy":
			prec, _ = numero(valor)
		}
	}
	if temLat && temLon {
		return ok(lat, lon, prec, "geoclue")
	}
	return Posicao{Status: StatusIndisponivel, Fonte: "geoclue", Detalhe: "GeoClue não retornou posição"}
}
