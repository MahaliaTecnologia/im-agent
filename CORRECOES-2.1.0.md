# im-agent 2.1.0 — correções de segurança e robustez

Base: fonte 2.0.0 enviada em 07/10/2026. Testes: `go test ./...` (cloud e platform) + teste de ponta a ponta da autoatualização com nuvem simulada (hash errado recusado; hash certo troca a versão e guarda a anterior).

## O que mudou

| # | Antes (2.0.0) | Agora (2.1.0) | Arquivos |
|---|---|---|---|
| 1 | Autoatualização trocava o executável sem conferir nada (só tamanho > 1 KB) | Exige SHA-256 vindo da nuvem; confere assinatura Ed25519 quando o binário tem chave pública embutida; roda `--versao` no arquivo novo (autoteste) antes de trocar; guarda o anterior em `<exe>.anterior`; ignora a própria versão; não repete a mesma atualização falha antes de 1 h | `internal/cloud/client.go`, `internal/cloud/seguranca.go` |
| 2 | Download gravava qualquer resposta (inclusive página de erro) | Exige HTTP 200, limite de 2 GiB, arquivo não vazio, redirecionamento só para HTTPS | `cloud.Baixar` (usado pela atualização e pelos pacotes) |
| 3 | `--cloud` aceitava `http://` | Exige `https://`; `--permitir-http` só para desenvolvimento | `cmd/main.go`, `cloud.ValidarURLNuvem` |
| 4 | Pacote sem SHA-256 era executado de qualquer origem | Com SHA-256: confere. Sem SHA-256: só aceita HTTPS | `platform.instalarPacote`, `cloud.ValidarURLPacote` |
| 5 | Remoção sem comando escolhia o primeiro programa "parecido" (Linux: 12 primeiras letras; macOS: `*nome*.app`; Windows: trecho do nome) | Só nome **exato** (sem diferenciar maiúsculas) e só se houver **um** candidato; com mais de um, recusa e pede o comando de remoção. Winget continua por `--id --exact` | `platform/platform.go`, `os_linux.go`, `os_darwin.go`, `os_windows.go` |
| 6 | Autoatualização no Windows tentava sobrescrever o executável em uso (falha) | Renomeia o atual para `.anterior` e coloca o novo no lugar | `seguranca.go` `trocarExecutavel` |
| 7 | Atualização baixava `/agente-endpoint/im-agent` sem informar sistema (a nuvem responde 400 para o Go) | Usa a URL enviada pela nuvem (mesmo host, HTTPS) ou `/agente-endpoint/im-agent?os=&arch=` | `client.go` `urlAtualizacao` |
| 8 | Registro recusado (ex.: cota de ativos) repetia a cada ciclo | Espera progressiva 1, 2, 4… até 30 min | `cmd/main.go` |
| 9 | `go.mod` sem `golang.org/x/sys` (Windows não compilava a partir desta fonte) | Dependência declarada | `go.mod` |

Novos: `--versao` (imprime a versão e sai), `build.sh` (4 binários + `dist/manifesto.json` + `SHA256SUMS`), `tools/assinar` (gera chaves Ed25519 e assina).

Não alterado: protocolo com a nuvem (enroll/poll/result/inventário/patches/telemetria), ordens aceitas, coleta de dados. O "uptime" da saúde continua sendo o do processo do agente.

## Como compilar

```bash
go mod tidy          # gera go.sum (precisa de internet)
go test ./...
./build.sh           # dist/ com os 4 binários e manifesto.json
```

Com assinatura (recomendado):

```bash
go run ./tools/assinar gerar-chaves          # guarde PRIVADA em cofre; PUBLICA pode ser pública
IM_AGENT_CHAVE_PUBLICA=<PUBLICA> IM_AGENT_CHAVE_PRIVADA=<PRIVADA> ./build.sh
```

Agentes compilados com chave pública passam a **exigir** assinatura em toda atualização futura. Perder a chave privada = só reinstalação manual.

## O que o servidor (InsightManager) precisa fazer

1. No `poll`, quando `atualizar = true`, enviar em `agente`: `url` (ex.: `/agente-endpoint/im-agent-linux-amd64`, conforme `sistema_operacional`/`arquitetura` do equipamento), `sha256` e `assinatura` — lidos do `manifesto.json` publicado junto com os binários.
2. Publicar os 4 binários e o `manifesto.json` em `public/agente-endpoint/` e mudar `AGENTE_VERSAO_ATUAL` para `2.1.0`.
3. Recomendado: calcular e gravar o SHA-256 dos pacotes da biblioteca (hoje os 22 estão sem hash; continuam funcionando porque todos usam HTTPS).

## Transição 2.0.0 → 2.1.0

A 2.0.0 **não consegue se autoatualizar**: no Linux/macOS o download cai no erro 400 (item 7) e no Windows a troca do executável falha (item 6). Os equipamentos em 2.0.0 precisam ser reinstalados uma vez com o binário 2.1.0 (mesmo comando de instalação; a chave/token atual é reaproveitada pelo `canal.token`). A partir da 2.1.0 a autoatualização funciona.
