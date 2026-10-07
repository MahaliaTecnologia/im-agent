//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/insightmanager/im-agent/internal/cloud"
)

func Sistema() string {
	out, _ := exec.Command("sw_vers", "-productVersion").Output()
	v := strings.TrimSpace(string(out))
	if v == "" {
		v = "unknown"
	}
	return "Darwin " + v
}

func Arch() string { return syscallArch() }

func HostnameFQDN() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("hostname", "-f").Output()
	if err == nil {
		fqdn := strings.TrimSpace(string(out))
		if fqdn != "" {
			return strings.ToLower(fqdn), nil
		}
	}
	return strings.ToLower(h), nil
}

func DataDir() string { return "/var/lib/insightmanager" }

func brewExec(args ...string) error {
	brewPath := ""
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if _, err := os.Stat(p); err == nil {
			brewPath = p
			break
		}
	}
	if brewPath == "" {
		if p, err := exec.LookPath("brew"); err == nil {
			brewPath = p
		} else {
			return fmt.Errorf("homebrew não encontrado")
		}
	}

	// Roda como o dono do brew (não root)
	fi, err := os.Stat(brewPath)
	_ = fi
	_ = err
	uid := os.Getuid()
	if uid == 0 {
		// Detecta o dono do binário
		var stat syscall.Stat_t
		if err := syscall.Stat(brewPath, &stat); err == nil && stat.Uid != 0 {
			cmd := exec.Command("sudo", append([]string{"-u", fmt.Sprintf("#%d", stat.Uid), brewPath}, args...)...)
			return cmd.Run()
		}
	}
	return exec.Command(brewPath, args...).Run()
}

func programaInstaladoOS(produto string, termos []string, gerenciador string) bool {
	alvos := append(termos, produto)
	curto := strings.Fields(produto)
	nome := produto
	if len(curto) > 0 {
		nome = curto[0]
	}

	// /Applications
	entries, _ := filepath.Glob("/Applications/*.app")
	entries2, _ := filepath.Glob("/Applications/*/*.app")
	entries = append(entries, entries2...)
	for _, entry := range entries {
		base := strings.TrimSuffix(filepath.Base(entry), ".app")
		for _, alvo := range alvos {
			if strings.Contains(strings.ToLower(base), strings.ToLower(alvo)) {
				return true
			}
		}
		if strings.Contains(strings.ToLower(base), strings.ToLower(nome)) {
			return true
		}
	}

	// Brew
	if gerenciador != "" {
		if brewExec("list", "--formula", gerenciador) == nil {
			return true
		}
		if brewExec("list", "--cask", gerenciador) == nil {
			return true
		}
	}
	return false
}

func instalarGerenciador(ger string) error {
	if ger == "" {
		return fmt.Errorf("sem instalador próprio e sem nome no gerenciador do sistema")
	}
	if brewExec("install", "--cask", ger) == nil {
		return nil
	}
	if brewExec("install", ger) == nil {
		return nil
	}
	return fmt.Errorf("o Homebrew não conseguiu instalar '%s'", ger)
}

func executarInstalador(arq, comando string) error {
	ext := strings.ToLower(filepath.Ext(arq))
	if comando != "" {
		linha := strings.ReplaceAll(comando, "%ARQUIVO%", arq)
		cmd := exec.Command("sh", "-c", linha)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("a instalação retornou erro: %s", truncar(string(out), 160))
		}
		return nil
	}
	switch ext {
	case ".pkg":
		return exec.Command("installer", "-pkg", arq, "-target", "/").Run()
	case ".dmg":
		out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-noverify", "-readonly", arq).CombinedOutput()
		if err != nil {
			return fmt.Errorf("não foi possível abrir a imagem: %w", err)
		}
		// Extrai o ponto de montagem
		ponto := ""
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "/Volumes/") {
				parts := strings.Fields(l)
				for _, p := range parts {
					if strings.HasPrefix(p, "/Volumes/") {
						ponto = p
					}
				}
			}
		}
		if ponto == "" {
			return fmt.Errorf("não foi possível determinar o ponto de montagem")
		}
		defer exec.Command("hdiutil", "detach", ponto, "-quiet").Run()

		// Procura .app ou .pkg
		apps, _ := filepath.Glob(filepath.Join(ponto, "*.app"))
		pkgs, _ := filepath.Glob(filepath.Join(ponto, "*.pkg"))
		if len(apps) > 0 {
			dest := "/Applications/" + filepath.Base(apps[0])
			os.RemoveAll(dest)
			return exec.Command("cp", "-R", apps[0], "/Applications/").Run()
		}
		if len(pkgs) > 0 {
			return exec.Command("installer", "-pkg", pkgs[0], "-target", "/").Run()
		}
		return fmt.Errorf("nenhum .app ou .pkg encontrado na imagem")
	case ".sh":
		os.Chmod(arq, 0755)
		return exec.Command(arq).Run()
	default:
		return fmt.Errorf("formato de pacote não suportado: %s", ext)
	}
}

func removerPacoteOS(p cloud.Payload) error {
	if p.ArgumentosRemover != "" {
		cmd := exec.Command("sh", "-c", p.ArgumentosRemover)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("a remoção retornou erro: %s", truncar(string(out), 160))
		}
		return nil
	}

	// Procura em /Applications pelo nome EXATO do app (Produto.app); nunca
	// por trecho do nome.
	apps, _ := filepath.Glob("/Applications/*.app")
	var nomes []string
	for _, a := range apps {
		nomes = append(nomes, strings.TrimSuffix(filepath.Base(a), ".app"))
	}
	if alvo, err := candidatoUnico(nomes, alvosRemocao(p)); err == nil {
		if err := os.RemoveAll(filepath.Join("/Applications", alvo+".app")); err == nil {
			return nil
		}
	} else if strings.HasPrefix(err.Error(), "mais de um") {
		return err
	}

	// Brew
	if p.Gerenciador != "" {
		if brewExec("uninstall", "--cask", p.Gerenciador) == nil {
			return nil
		}
		if brewExec("uninstall", p.Gerenciador) == nil {
			return nil
		}
	}

	return fmt.Errorf("programa não encontrado no equipamento (%s)", p.Produto)
}

func Reiniciar() {
	exec.Command("shutdown", "-r", "+1", "Reinício solicitado pelo Insight Manager").Start()
}

func Desligar() {
	exec.Command("shutdown", "-h", "+1", "Desligamento solicitado pelo Insight Manager").Start()
}

func LimparDisco() {
	exec.Command("find", "/Library/Caches", "-type", "f", "-mtime", "+7", "-delete").Run()
	exec.Command("find", "/tmp", "-type", "f", "-mtime", "+1", "-delete").Run()
}

func AplicarAtualizacoesSistema() error {
	return exec.Command("softwareupdate", "--install", "--all", "--agree-to-license").Run()
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
