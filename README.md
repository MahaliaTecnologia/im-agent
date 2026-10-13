# im-agent — agente de endpoint do InsightManager

Agente em Go (sem dependências além de `golang.org/x/sys`) que roda nos equipamentos dos clientes e conversa com o InsightManager por HTTPS: registro, consulta de ordens, inventário, patches, telemetria (DEX) e autoatualização com SHA-256.

Versão atual: veja `const versao` em `cmd/main.go`.

## Estrutura

| Pasta | Função |
|---|---|
| `cmd/` | Parâmetros, registro, laço principal e execução das ordens |
| `internal/cloud` | Cliente HTTPS, autoatualização segura (`seguranca.go`) |
| `internal/platform` | Instalar/remover pacotes, reiniciar, limpar disco, por sistema |
| `internal/inventory`, `patches`, `telemetry`, `health` | Coletas |
| `tools/assinar` | Calcula SHA-256 (e assinatura Ed25519 opcional) para o manifesto |
| `docs/` | Histórico das correções 2.1.0 |

## Compilar

```bash
go test ./...
./build.sh        # gera dist/: 6 binários, manifesto.json e SHA256SUMS
```

A compilação é reproduzível (`-trimpath -buildvcs=false`, Go 1.24.7): a mesma fonte gera os mesmos SHA-256.

## Publicar uma versão

1. Alterar `const versao` em `cmd/main.go` e fazer o commit.
2. `git tag vX.Y.Z && git push origin main vX.Y.Z`
3. O GitHub Actions testa, compila e cria a release com os binários, `manifesto.json` e `SHA256SUMS`.
4. Copiar os arquivos da release para `public/agente-endpoint/` do projeto InsightManager no Lovable e conferir os SHA-256 contra o manifesto. Os agentes 2.1.0+ se atualizam sozinhos.

## Regras

- Nunca subir binários, chaves privadas, certificados ou senhas para o repositório (`dist/` está no `.gitignore`).
- `--cloud` exige HTTPS; `--permitir-http` só em desenvolvimento.
- Assinatura de código (Windows Authenticode / macOS Developer ID + notarização): passos comentados em `.github/workflows/release.yml`, a ligar quando as contas existirem.
