//go:build linux

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
	var u syscall.Utsname
	syscall.Uname(&u)
	s := make([]byte, len(u.Sysname))
	for i, v := range u.Sysname {
		if v == 0 {
			break
		}
		s[i] = byte(v)
	}
	r := make([]byte, len(u.Release))
	for i, v := range u.Release {
		if v == 0 {
			break
		}
		r[i] = byte(v)
	}
	return strings.TrimRight(string(s), "\x00") + " " + strings.TrimRight(string(r), "\x00")
}

func Arch() string { return syscallArch() }

func HostnameFQDN() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	// Tenta expandir para FQDN
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

func programaInstaladoOS(produto string, termos []string, gerenciador string) bool {
	alvos := append(termos, produto)
	curto := strings.Fields(produto)
	nome := produto
	if len(curto) > 0 {
		nome = curto[0]
	}

	// dpkg
	if _, err := exec.LookPath("dpkg-query"); err == nil {
		out, _ := exec.Command("dpkg-query", "-W", "-f=${Package} ${Status}\n").Output()
		linhas := strings.Split(string(out), "\n")
		for _, l := range linhas {
			if !strings.Contains(l, "install ok installed") {
				continue
			}
			for _, alvo := range alvos {
				if strings.Contains(strings.ToLower(l), strings.ToLower(alvo)) {
					return true
				}
			}
		}
		// Verifica pelo nome no gerenciador
		if gerenciador != "" {
			out, _ := exec.Command("dpkg-query", "-W", "-f=${Status}", gerenciador).Output()
			if strings.Contains(string(out), "install ok installed") {
				return true
			}
		}
	}

	// rpm
	if _, err := exec.LookPath("rpm"); err == nil {
		out, _ := exec.Command("rpm", "-qa").Output()
		for _, alvo := range alvos {
			if strings.Contains(strings.ToLower(string(out)), strings.ToLower(alvo)) {
				return true
			}
		}
	}

	// Binário no PATH
	if _, err := exec.LookPath(strings.ToLower(nome)); err == nil {
		return true
	}

	return false
}

func instalarGerenciador(ger string) error {
	if ger == "" {
		return fmt.Errorf("sem instalador próprio e sem nome no gerenciador do sistema")
	}
	// apt-get
	if _, err := exec.LookPath("apt-get"); err == nil {
		exec.Command("apt-get", "update").Run()
		cmd := exec.Command("apt-get", "-y", "install", ger)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			_ = out
		}
	}
	// dnf
	if _, err := exec.LookPath("dnf"); err == nil {
		if err := exec.Command("dnf", "-y", "install", ger).Run(); err == nil {
			return nil
		}
	}
	// yum
	if _, err := exec.LookPath("yum"); err == nil {
		if err := exec.Command("yum", "-y", "install", ger).Run(); err == nil {
			return nil
		}
	}
	// snap
	if _, err := exec.LookPath("snap"); err == nil {
		if err := exec.Command("snap", "install", ger).Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("o gerenciador do sistema não conseguiu instalar '%s'", ger)
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
	case ".deb":
		if err := exec.Command("dpkg", "-i", arq).Run(); err != nil {
			exec.Command("apt-get", "-y", "-f", "install").Run()
			if err2 := exec.Command("dpkg", "-i", arq).Run(); err2 != nil {
				return fmt.Errorf("falha ao instalar .deb: %w", err2)
			}
		}
	case ".rpm":
		if err := exec.Command("rpm", "-Uvh", "--quiet", arq).Run(); err != nil {
			if err2 := exec.Command("yum", "-y", "install", arq).Run(); err2 != nil {
				return fmt.Errorf("falha ao instalar .rpm: %w", err2)
			}
		}
	case ".sh":
		os.Chmod(arq, 0755)
		if err := exec.Command(arq).Run(); err != nil {
			return fmt.Errorf("falha ao executar .sh: %w", err)
		}
	case ".gz", ".bz2", ".xz":
		// tar.gz — extrai e instala com dpkg/rpm
		dir := arq + "_extracted"
		os.MkdirAll(dir, 0755)
		defer os.RemoveAll(dir)
		if err := exec.Command("tar", "-xzf", arq, "-C", dir, "--strip-components=1").Run(); err != nil {
			return fmt.Errorf("falha ao extrair arquivo: %w", err)
		}
		// Procura DEBs
		debs, _ := filepath.Glob(filepath.Join(dir, "**", "*.deb"))
		if len(debs) == 0 {
			debs, _ = filepath.Glob(filepath.Join(dir, "*.deb"))
		}
		if len(debs) > 0 {
			args := append([]string{"-i"}, debs...)
			if err := exec.Command("dpkg", args...).Run(); err != nil {
				exec.Command("apt-get", "-y", "-f", "install").Run()
			}
			return nil
		}
		return fmt.Errorf("formato de pacote não suportado dentro do arquivo")
	default:
		return fmt.Errorf("formato de pacote não suportado: %s", ext)
	}
	return nil
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
	if p.Produto == "" {
		return fmt.Errorf("sem nome do produto")
	}
	nome := strings.Fields(p.Produto)[0]
	// apt-get
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Tenta encontrar o pacote pelo nome parcial
		out, _ := exec.Command("dpkg-query", "-W", "-f=${Package}\n").Output()
		for _, pkg := range strings.Split(string(out), "\n") {
			if strings.Contains(strings.ToLower(pkg), strings.ToLower(nome[:min(len(nome), 12)])) {
				if exec.Command("apt-get", "-y", "remove", strings.TrimSpace(pkg)).Run() == nil {
					return nil
				}
			}
		}
	}
	// yum/dnf
	if _, err := exec.LookPath("dnf"); err == nil {
		if exec.Command("dnf", "-y", "remove", p.Produto).Run() == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("yum"); err == nil {
		if exec.Command("yum", "-y", "remove", p.Produto).Run() == nil {
			return nil
		}
	}
	// Gerenciador nativo
	if p.Gerenciador != "" {
		if exec.Command("apt-get", "-y", "remove", p.Gerenciador).Run() == nil {
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
	exec.Command("find", "/tmp", "-type", "f", "-mtime", "+1", "-delete").Run()
	exec.Command("find", "/var/tmp", "-type", "f", "-mtime", "+1", "-delete").Run()
	exec.Command("journalctl", "--vacuum-size=200M").Run()
	exec.Command("apt-get", "clean").Run()
}

func AplicarAtualizacoesSistema() error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("apt-get", "-y", "upgrade")
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s", truncar(string(out), 200))
		}
		return nil
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return exec.Command("dnf", "-y", "upgrade", "--refresh").Run()
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return exec.Command("yum", "-y", "update").Run()
	}
	return fmt.Errorf("nenhum gerenciador de atualizações compatível foi encontrado")
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
