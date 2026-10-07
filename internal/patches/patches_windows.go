//go:build windows

package patches

import (
	"bufio"
	"os/exec"
	"strings"
)

func Listar() PatchPayload {
	var itens []Patch

	script := `
$session = New-Object -ComObject Microsoft.Update.Session
$search = $session.CreateUpdateSearcher().Search("IsInstalled=0 and IsHidden=0")
foreach ($u in $search.Updates) {
    $kb = if ($u.KBArticleIDs.Count -gt 0) { "KB" + $u.KBArticleIDs.Item(0) } else { $u.Identity.UpdateID }
    $sev = if ($u.MsrcSeverity) { $u.MsrcSeverity } else { "media" }
    $cat = if ($u.Categories.Count -gt 0) { $u.Categories.Item(0).Name } else { "Atualização" }
    $sz = try { [math]::Round($u.MaxDownloadSize / 1MB, 1) } catch { 0 }
    Write-Host "$kb|$($u.Title)|$cat|$sev|$sz|$($u.RebootRequired)"
}
`
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return PatchPayload{Atualizacoes: itens}
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "|")
		if len(fields) < 6 {
			continue
		}
		reinicio := fields[5] == "True"
		itens = append(itens, Patch{
			Identificador: fields[0],
			Titulo:        fields[1],
			Categoria:     fields[2],
			Severidade:    strings.ToLower(fields[3]),
			Plataforma:    "windows",
			Reinicio:      reinicio,
		})
		if len(itens) >= 300 {
			break
		}
	}

	return PatchPayload{Atualizacoes: itens}
}
