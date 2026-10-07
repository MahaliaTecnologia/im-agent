//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/insightmanager/im-agent/internal/cloud"
)

func Sistema() string {
	out, _ := exec.Command("cmd", "/c", "ver").Output()
	return strings.TrimSpace(string(out))
}

func Arch() string {
	a := os.Getenv("PROCESSOR_ARCHITECTURE")
	if a == "" {
		return "x86_64"
	}
	return a
}

func HostnameFQDN() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	// Tenta sufixo de domínio
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-DnsClientGlobalSetting).SuffixSearchList | Select-Object -First 1").Output()
	if err == nil {
		sufixo := strings.TrimSpace(string(out))
		if sufixo != "" {
			return strings.ToLower(h + "." + sufixo), nil
		}
	}
	return strings.ToLower(h), nil
}

func DataDir() string {
	d := os.Getenv("ProgramData")
	if d == "" {
		d = `C:\ProgramData`
	}
	return filepath.Join(d, "InsightManager")
}

func listarInstalados() []map[string]string {
	chaves := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	var lista []map[string]string
	for _, base := range chaves {
		for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
			k, err := registry.OpenKey(root, base, registry.READ)
			if err != nil {
				continue
			}
			subs, _ := k.ReadSubKeyNames(-1)
			for _, sub := range subs {
				sk, err := registry.OpenKey(k, sub, registry.READ)
				if err != nil {
					continue
				}
				nome, _, _ := sk.GetStringValue("DisplayName")
				if nome == "" {
					sk.Close()
					continue
				}
				versao, _, _ := sk.GetStringValue("DisplayVersion")
				uninstall, _, _ := sk.GetStringValue("UninstallString")
				quietUninstall, _, _ := sk.GetStringValue("QuietUninstallString")
				sk.Close()
				lista = append(lista, map[string]string{
					"nome":           nome,
					"versao":         versao,
					"uninstall":      uninstall,
					"quietUninstall": quietUninstall,
					"key":            sub,
				})
			}
			k.Close()
		}
	}
	return lista
}

func programaInstaladoOS(produto string, termos []string, gerenciador string) bool {
	alvos := append(termos, produto)
	instalados := listarInstalados()
	for _, prog := range instalados {
		for _, alvo := range alvos {
			if alvo != "" && strings.Contains(strings.ToLower(prog["nome"]), strings.ToLower(alvo)) {
				return true
			}
		}
	}
	return false
}

func temWinget() bool {
	_, err := exec.LookPath("winget.exe")
	return err == nil
}

func instalarGerenciador(ger string) error {
	if ger == "" {
		return fmt.Errorf("sem instalador próprio e sem nome no gerenciador do sistema")
	}
	if !temWinget() {
		return fmt.Errorf("o winget não está disponível neste equipamento")
	}
	cmd := exec.Command("winget.exe", "install", "--id", ger, "--exact",
		"--silent", "--accept-package-agreements", "--accept-source-agreements",
		"--disable-interactivity")
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("o winget retornou erro ao instalar '%s': %w", ger, err)
	}
	return nil
}

func executarInstalador(arq, comando string) error {
	ext := strings.ToLower(filepath.Ext(arq))
	if comando != "" {
		linha := strings.ReplaceAll(comando, "%ARQUIVO%", arq)
		cmd := exec.Command("cmd.exe", "/d", "/s", "/c", linha)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("a instalação retornou erro: %s", truncar(string(out), 160))
		}
		return nil
	}
	var cmd *exec.Cmd
	switch ext {
	case ".msi":
		cmd = exec.Command("msiexec.exe", "/i", arq, "/qn", "/norestart")
	case ".exe":
		cmd = exec.Command(arq, "/S", "/silent", "/qn")
	case ".ps1":
		cmd = exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", arq)
	default:
		return fmt.Errorf("formato de pacote não suportado: %s", ext)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Código 3010 = reinício necessário mas instalação OK
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3010 {
			return nil
		}
		return fmt.Errorf("o instalador retornou erro: %s", truncar(string(out), 160))
	}
	return nil
}

func removerPacoteOS(p cloud.Payload) error {
	if p.ArgumentosRemover != "" {
		cmd := exec.Command("cmd.exe", "/d", "/s", "/c", p.ArgumentosRemover)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("a remoção retornou erro: %s", truncar(string(out), 160))
		}
		return nil
	}

	// Sem comando próprio: só remove o programa cujo nome exibido seja
	// EXATAMENTE o produto ou um termo, e só se houver um único.
	instalados := listarInstalados()
	var nomes []string
	for _, prog := range instalados {
		nomes = append(nomes, prog["nome"])
	}
	alvo, err := candidatoUnico(nomes, alvosRemocao(p))
	if err != nil {
		if strings.HasPrefix(err.Error(), "mais de um") {
			return err
		}
		// Winget por identificador exato como último recurso.
		if p.Gerenciador != "" && temWinget() {
			cmd := exec.Command("winget.exe", "uninstall", "--id", p.Gerenciador,
				"--exact", "--silent", "--disable-interactivity")
			if cmd.Run() == nil {
				return nil
			}
		}
		return fmt.Errorf("programa não encontrado com nome exato (%s); cadastre o comando de remoção", p.Produto)
	}
	for _, prog := range instalados {
		if !strings.EqualFold(strings.TrimSpace(prog["nome"]), alvo) {
			continue
		}
		if strings.HasPrefix(prog["key"], "{") {
			if exec.Command("msiexec.exe", "/x", prog["key"], "/qn", "/norestart").Run() == nil {
				return nil
			}
		}
		if prog["quietUninstall"] != "" {
			if exec.Command("cmd.exe", "/d", "/s", "/c", prog["quietUninstall"]).Run() == nil {
				return nil
			}
		}
		if prog["uninstall"] != "" {
			if exec.Command("cmd.exe", "/d", "/s", "/c", prog["uninstall"]+" /S /silent /qn /norestart").Run() == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("o desinstalador de '%s' retornou erro", alvo)
}

func Reiniciar() {
	exec.Command("shutdown.exe", "/r", "/t", "5", "/c", "Reinício solicitado pelo Insight Manager").Start()
}

func Desligar() {
	exec.Command("shutdown.exe", "/s", "/t", "5", "/c", "Desligamento solicitado pelo Insight Manager").Start()
}

func LimparDisco() {
	dirs := []string{os.Getenv("TEMP"), os.Getenv("SystemRoot") + `\Temp`}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			os.RemoveAll(path)
		}
	}
	exec.Command("cleanmgr.exe", "/verylowdisk").Start()
}

func AplicarAtualizacoesSistema() error {
	// Usa o Windows Update via PowerShell
	script := `
$session = New-Object -ComObject Microsoft.Update.Session
$search = $session.CreateUpdateSearcher().Search("IsInstalled=0 and IsHidden=0")
if ($search.Updates.Count -eq 0) { exit 0 }
$col = New-Object -ComObject Microsoft.Update.UpdateColl
foreach ($u in $search.Updates) { if (-not $u.EulaAccepted) { $u.AcceptEula() }; [void]$col.Add($u) }
$dl = $session.CreateUpdateDownloader(); $dl.Updates = $col; $dl.Download() | Out-Null
$inst = $session.CreateUpdateInstaller(); $inst.Updates = $col; $r = $inst.Install()
if ($r.ResultCode -notin @(2,3)) { exit 1 }
`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", truncar(string(out), 200))
	}
	return nil
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
