//go:build linux

package location

import (
	"context"
	"os"
	"os/exec"
	"time"
)

var caminhosWhereAmI = []string{
	"/usr/libexec/geoclue-2.0/demos/where-am-i",
	"/usr/lib/geoclue-2.0/demos/where-am-i",
	"/usr/lib/x86_64-linux-gnu/geoclue-2.0/demos/where-am-i",
	"/usr/lib/aarch64-linux-gnu/geoclue-2.0/demos/where-am-i",
}

// Só funciona com GeoClue instalado e liberado para o agente.
func coletarOS() Posicao {
	bin := ""
	for _, c := range caminhosWhereAmI {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			bin = c
			break
		}
	}
	if bin == "" {
		return Posicao{Status: StatusNaoSuportado, Fonte: "geoclue", Detalhe: "GeoClue não instalado"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, bin, "-t", "30").CombinedOutput()
	return interpretarGeoClue(string(out))
}
