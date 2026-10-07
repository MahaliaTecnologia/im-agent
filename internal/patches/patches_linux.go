//go:build linux

package patches

import (
	"bufio"
	"os/exec"
	"strings"
)

func Listar() PatchPayload {
	var itens []Patch

	// apt
	if out, err := exec.Command("apt", "list", "--upgradable").CombinedOutput(); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			l := scanner.Text()
			if !strings.Contains(l, "/") || strings.HasPrefix(l, "Listing") {
				continue
			}
			fields := strings.Fields(l)
			pkg := strings.Split(fields[0], "/")[0]
			origem := strings.Split(fields[0], "/")[1]
			ver := ""
			if len(fields) > 1 {
				ver = fields[1]
			}
			sev := "media"
			cat := "Atualização"
			if strings.Contains(origem, "-security") {
				sev = "alta"
				cat = "Segurança"
			}
			itens = append(itens, Patch{
				Identificador: pkg,
				Titulo:        pkg + " " + ver,
				Categoria:     cat,
				Severidade:    sev,
				Plataforma:    "linux",
			})
			if len(itens) >= 300 {
				break
			}
		}
	}

	// dnf como fallback
	if len(itens) == 0 {
		if out, err := exec.Command("dnf", "-q", "check-update").CombinedOutput(); err != nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) < 2 {
					continue
				}
				itens = append(itens, Patch{
					Identificador: fields[0],
					Titulo:        fields[0] + " " + fields[1],
					Categoria:     "Atualização",
					Severidade:    "media",
					Plataforma:    "linux",
				})
				if len(itens) >= 300 {
					break
				}
			}
		}
	}

	return PatchPayload{Atualizacoes: itens}
}
