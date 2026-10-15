# Regras para o Claude neste repositório

## Fluxo de trabalho

- Trabalhar sempre em branch e entregar por PR (`gh pr create`).
- Nunca fazer push direto na `main`.
- Nunca criar nem enviar tags: a tag dispara a release e o piloto dos agentes.
- Em caso de ambiguidade, perguntar antes de implementar.

## Código

- Não alterar `const versao` em `cmd/main.go` sem pedido explícito.
- Rodar `go test ./...` e `./build.sh` antes de cada push; só seguir se os dois passarem.

## Commits

- Commits pequenos, com mensagens em português.
- Nunca fazer commit de binários, `dist/`, chaves, certificados, tokens ou senhas.
