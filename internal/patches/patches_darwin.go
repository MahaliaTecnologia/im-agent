//go:build darwin

package patches

import (
	"bufio"
	"os/exec"
	"strings"
)

func Listar() PatchPayload {
	var itens []Patch

	out, err := exec.Command("softwareupdate", "-l").CombinedOutput()
	if err != nil && len(out) == 0 {
		return PatchPayload{Atualizacoes: itens}
	}

	var lbl string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		l := scanner.Text()
		if strings.Contains(l, "* Label:") {
			lbl = strings.TrimSpace(strings.SplitN(l, "Label:", 2)[1])
			continue
		}
		if lbl != "" {
			sev := "media"
			cat := "Atualização"
			reinicio := false
			if strings.Contains(l, "Recommended: YES") {
				sev = "alta"
				cat = "Segurança"
			}
			if strings.Contains(l, "Action: restart") {
				reinicio = true
			}
			itens = append(itens, Patch{
				Identificador: lbl,
				Titulo:        lbl,
				Categoria:     cat,
				Severidade:    sev,
				Plataforma:    "macos",
				Reinicio:      reinicio,
			})
			lbl = ""
		}
		if len(itens) >= 300 {
			break
		}
	}

	return PatchPayload{Atualizacoes: itens}
}
