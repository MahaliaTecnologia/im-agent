// Ferramenta de assinatura das atualizações do im-agent (Ed25519).
//
//	go run ./tools/assinar gerar-chaves            → imprime chave pública e privada (base64)
//	go run ./tools/assinar assinar <arquivo>       → lê IM_AGENT_CHAVE_PRIVADA e imprime sha256 + assinatura
//
// A chave privada NUNCA vai para o repositório nem para o Lovable: fica no
// cofre de quem compila. A pública é embutida no agente pelo build.sh.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: assinar gerar-chaves | assinar <arquivo>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gerar-chaves":
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
		fmt.Println("PUBLICA=" + base64.StdEncoding.EncodeToString(pub))
		fmt.Println("PRIVADA=" + base64.StdEncoding.EncodeToString(priv))
	case "assinar":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "informe o arquivo")
			os.Exit(2)
		}
		f, err := os.Open(os.Args[2])
		if err != nil {
			panic(err)
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			panic(err)
		}
		hash := hex.EncodeToString(h.Sum(nil))
		fmt.Println("sha256=" + hash)
		if k := os.Getenv("IM_AGENT_CHAVE_PRIVADA"); k != "" {
			priv, err := base64.StdEncoding.DecodeString(k)
			if err != nil || len(priv) != ed25519.PrivateKeySize {
				fmt.Fprintln(os.Stderr, "IM_AGENT_CHAVE_PRIVADA inválida")
				os.Exit(1)
			}
			fmt.Println("assinatura=" + base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(priv), []byte(hash))))
		}
	default:
		fmt.Fprintln(os.Stderr, "comando desconhecido")
		os.Exit(2)
	}
}
