#!/usr/bin/env bash
# Compila os 4 binários do im-agent em dist/ e gera dist/manifesto.json com
# SHA-256 (e assinatura, se IM_AGENT_CHAVE_PRIVADA estiver definida).
#
#   IM_AGENT_CHAVE_PUBLICA=<base64>  embute a chave pública (exige assinatura nas atualizações)
#   IM_AGENT_CHAVE_PRIVADA=<base64>  assina cada binário (guardar em cofre, nunca no repositório)
set -euo pipefail
cd "$(dirname "$0")"
VERSAO=$(sed -n 's/^const versao = "\(.*\)"/\1/p' cmd/main.go)
LDFLAGS="-s -w"
if [[ -n "${IM_AGENT_CHAVE_PUBLICA:-}" ]]; then
  LDFLAGS="$LDFLAGS -X github.com/insightmanager/im-agent/internal/cloud.ChavePublicaAtualizacao=${IM_AGENT_CHAVE_PUBLICA}"
fi
rm -rf dist && mkdir -p dist
alvos=("linux amd64 im-agent-linux-amd64" "linux arm64 im-agent-linux-arm64" "darwin amd64 im-agent-darwin-amd64" "darwin arm64 im-agent-darwin-arm64" "windows amd64 im-agent-windows-amd64.exe" "windows arm64 im-agent-windows-arm64.exe")
echo "{\"versao\":\"$VERSAO\",\"binarios\":{" > dist/manifesto.json
primeiro=1
for a in "${alvos[@]}"; do
  read -r os arch nome <<<"$a"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "dist/$nome" ./cmd
  saida=$(go run ./tools/assinar assinar "dist/$nome")
  sha=$(sed -n 's/^sha256=//p' <<<"$saida"); sig=$(sed -n 's/^assinatura=//p' <<<"$saida")
  [[ $primeiro == 1 ]] || echo "," >> dist/manifesto.json; primeiro=0
  printf '"%s-%s":{"arquivo":"%s","sha256":"%s","assinatura":"%s"}' "$os" "$arch" "$nome" "$sha" "$sig" >> dist/manifesto.json
  echo "$sha  $nome" >> dist/SHA256SUMS
done
echo "}}" >> dist/manifesto.json
echo "Versão $VERSAO compilada em dist/"
