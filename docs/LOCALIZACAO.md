# Localização dos computadores (a partir da 2.1.5)

O agente só coleta quando a resposta do poll traz `localizacao.ativa = true`.
Isso acontece apenas quando a organização ligou, no InsightManager
(Configurações › Agente), a localização geral e a "Localização dos
computadores". Desligada é o padrão: o agente não consulta nada.

- Intervalo: `localizacao.intervaloMin` (padrão 15, limitado a 5..240 min).
- Envio: `POST /api/public/device-agent/location` com
  `{status, lat, lon, precisaoM, capturadaEm, fonte, detalhe}`.
- Status: `ok`, `negada`, `indisponivel`, `nao_suportado`, `erro`.

| Sistema | Como | Requisito no equipamento |
|---|---|---|
| Windows | `System.Device.Location.GeoCoordinateWatcher` via PowerShell 5.1 (Wi-Fi/GPS) | "Serviços de localização" e "Permitir que aplicativos da área de trabalho acessem sua localização" ligados (pode ser por GPO). |
| Linux | `where-am-i` do GeoClue 2 | GeoClue instalado e liberado para o agente. |
| macOS | ainda não suportado (`nao_suportado`) | Exige app assinado com permissão do usuário; depende da conta Apple. |

Nada é guardado no equipamento; a retenção é a da organização no InsightManager.
