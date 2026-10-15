#!/usr/bin/env bash
# Envia a versão compilada em dist/ ao InsightManager, que a coloca em piloto
# (equipamentos marcados como "Piloto de atualização") e libera para todos
# depois de 24 h se os pilotos atualizarem sem erro.
#
#   IM_RELEASE_TOKEN  segredo do GitHub (mesmo valor de AGENT_RELEASE_TOKEN no InsightManager)
#   IM_RELEASE_URL    opcional; padrão https://insightmanager.com.br
#   GITHUB_SHA        commit (preenchido pelo GitHub Actions)
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -z "${IM_RELEASE_TOKEN:-}" ]]; then
  echo "::warning::IM_RELEASE_TOKEN não configurado; versão não enviada ao InsightManager."
  exit 0
fi
BASE="${IM_RELEASE_URL:-https://insightmanager.com.br}"
VERSAO=$(sed -n 's/^const versao = "\(.*\)"/\1/p' cmd/main.go)
COMMIT="${GITHUB_SHA:-$(git rev-parse HEAD)}"
AUTH="Authorization: Bearer ${IM_RELEASE_TOKEN}"

# Confere localmente antes de enviar.
(cd dist && sha256sum -c SHA256SUMS)

corpo=$(jq -n --arg v "$VERSAO" --arg c "$COMMIT" --slurpfile m dist/manifesto.json \
  '{versao:$v, tag:("v"+$v), commit:$c, manifesto:$m[0]}')

chamar() { # método caminho [args curl...]
  local metodo=$1 caminho=$2; shift 2
  local codigo
  codigo=$(curl -sS -o /tmp/resp.txt -w '%{http_code}' --retry 3 --retry-all-errors \
    -X "$metodo" -H "$AUTH" "$@" "${BASE}${caminho}")
  echo "$metodo $caminho -> HTTP $codigo"
  if [[ "$codigo" -ge 300 ]]; then cat /tmp/resp.txt; echo; return 1; fi
}

chamar POST /api/public/agent-releases -H 'Content-Type: application/json' --data "$corpo"

for arquivo in $(jq -r '.binarios[].arquivo' dist/manifesto.json); do
  chamar PUT "/api/public/agent-releases/${VERSAO}/${arquivo}" \
    -H 'Content-Type: application/octet-stream' --data-binary "@dist/${arquivo}"
done

chamar POST "/api/public/agent-releases/${VERSAO}/concluir"
echo "Versão ${VERSAO} enviada ao InsightManager e em piloto."
