package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizarNaoEnviaNull(t *testing.T) {
	b, _ := json.Marshal(Normalizar(Telemetria{}))
	if !strings.Contains(string(b), `"aplicacoes":[]`) {
		t.Fatalf("aplicacoes deve ir como lista vazia: %s", b)
	}
}
