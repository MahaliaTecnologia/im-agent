package cloud

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PermitirHTTP só deve ser ligado em desenvolvimento (flag --permitir-http).
// Em produção a nuvem, os downloads e os redirecionamentos exigem HTTPS.
var PermitirHTTP = false

// ChavePublicaAtualizacao é a chave pública Ed25519 (base64) usada para
// conferir a assinatura das atualizações. É embutida na compilação:
//
//	go build -ldflags "-X github.com/insightmanager/im-agent/internal/cloud.ChavePublicaAtualizacao=<base64>"
//
// Vazia = só o SHA-256 é exigido. Preenchida = a assinatura também é exigida.
var ChavePublicaAtualizacao = ""

// ValidarURLNuvem exige https:// (salvo PermitirHTTP) e um host.
func ValidarURLNuvem(raw string) (string, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("endereço da nuvem inválido: %q", raw)
	}
	if u.Scheme != "https" && !(PermitirHTTP && u.Scheme == "http") {
		return "", fmt.Errorf("o endereço da nuvem precisa começar com https:// (recebido %q)", raw)
	}
	return u.String(), nil
}

// ValidarURLPacote exige HTTPS para downloads de pacotes sem SHA-256.
func ValidarURLPacote(raw string, temHash bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("endereço do pacote inválido")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (temHash || PermitirHTTP) {
		return nil
	}
	return fmt.Errorf("pacote sem SHA-256 só pode ser baixado por HTTPS")
}

func mesmoHost(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(ua.Host, ub.Host) && (ua.Scheme == "https" || PermitirHTTP)
}

func iguaisHex(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// SHA256Arquivo devolve o hash hexadecimal do arquivo.
func SHA256Arquivo(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerificarAssinatura confere a assinatura Ed25519 sobre o SHA-256 (texto
// hexadecimal em minúsculas) quando o binário tem chave pública embutida.
func VerificarAssinatura(sha256hex, assinaturaB64 string) error {
	if ChavePublicaAtualizacao == "" {
		return nil
	}
	pub, err := base64.StdEncoding.DecodeString(ChavePublicaAtualizacao)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("chave pública de atualização embutida é inválida")
	}
	if assinaturaB64 == "" {
		return fmt.Errorf("atualização recusada: sem assinatura")
	}
	sig, err := base64.StdEncoding.DecodeString(assinaturaB64)
	if err != nil {
		return fmt.Errorf("atualização recusada: assinatura ilegível")
	}
	msg := []byte(strings.ToLower(strings.TrimSpace(sha256hex)))
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		return fmt.Errorf("atualização recusada: assinatura não confere")
	}
	return nil
}

// autoteste executa "<novo> --versao" e exige saída com a versão anunciada.
func autoteste(binario, versaoEsperada string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binario, "--versao").Output()
	if err != nil {
		return err
	}
	v := strings.TrimSpace(string(bytes.TrimSpace(out)))
	if versaoEsperada != "" && v != versaoEsperada {
		return fmt.Errorf("versão %q diferente da anunciada %q", v, versaoEsperada)
	}
	return nil
}

// trocarExecutavel guarda o atual em .anterior e coloca o novo no lugar.
// Renomear o executável em uso funciona em Linux, macOS e Windows; sobrescrever
// diretamente não funciona no Windows.
func trocarExecutavel(self, novo string) error {
	anterior := self + ".anterior"
	_ = os.Remove(anterior)
	if err := os.Rename(self, anterior); err != nil {
		return fmt.Errorf("não foi possível guardar a versão anterior: %w", err)
	}
	if err := os.Rename(novo, self); err != nil {
		_ = os.Rename(anterior, self)
		return fmt.Errorf("não foi possível instalar a nova versão: %w", err)
	}
	return nil
}
