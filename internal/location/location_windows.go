//go:build windows

package location

import (
	"context"
	"os/exec"
	"time"
)

// Usa a API de localização do Windows (Wi-Fi/GPS) pelo .NET do PowerShell 5.1.
// Depende de "Serviços de localização" e "Permitir que aplicativos da área de
// trabalho acessem sua localização" ligados no equipamento.
const scriptWindows = `
$ErrorActionPreference = 'Stop'
try {
  Add-Type -AssemblyName System.Device
  $w = New-Object System.Device.Location.GeoCoordinateWatcher([System.Device.Location.GeoPositionAccuracy]::High)
  [void]$w.TryStart($false, [TimeSpan]::FromSeconds(15))
  $fim = (Get-Date).AddSeconds(25)
  while (($w.Status -ne 'Ready') -and ($w.Permission -ne 'Denied') -and ((Get-Date) -lt $fim)) { Start-Sleep -Milliseconds 500 }
  $inv = [Globalization.CultureInfo]::InvariantCulture
  $p = $w.Position.Location
  if ($w.Permission -eq 'Denied') { 'negada' }
  elseif ($p.IsUnknown) { 'indisponivel|' + $w.Status }
  else { 'ok|' + $p.Latitude.ToString($inv) + '|' + $p.Longitude.ToString($inv) + '|' + $p.HorizontalAccuracy.ToString($inv) }
  $w.Stop()
} catch { 'erro|' + $_.Exception.Message }
`

func coletarOS() Posicao {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", scriptWindows).Output()
	if err != nil && len(out) == 0 {
		return Posicao{Status: StatusErro, Fonte: "windows", Detalhe: curto(err.Error())}
	}
	return interpretarWindows(string(out))
}
