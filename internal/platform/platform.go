package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insightmanager/im-agent/internal/cloud"
)

// ProgramaInstalado verifica se um programa está instalado no sistema.
func ProgramaInstalado(produto string, termos []string, gerenciador string) bool {
	return programaInstaladoOS(produto, termos, gerenciador)
}

// AplicarInstalacao tenta o instalador da biblioteca e depois o gerenciador nativo.
func AplicarInstalacao(p cloud.Payload) error {
	if p.URL != "" {
		err := instalarPacote(p.URL, p.SHA256, p.Argumentos)
		if err == nil {
			return nil
		}
		if p.Gerenciador == "" {
			return err
		}
		// Cai no gerenciador nativo
	}
	return instalarGerenciador(p.Gerenciador)
}

// ConferirInstalado aguarda até 30s para o programa aparecer na lista.
func ConferirInstalado(p cloud.Payload) error {
	for i := 0; i < 6; i++ {
		if ProgramaInstalado(p.Produto, p.Termos, p.Gerenciador) {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("a instalação terminou, mas o programa não aparece na lista de instalados")
}

// ConferirRemovido aguarda até 30s para o programa sumir da lista.
func ConferirRemovido(p cloud.Payload) error {
	for i := 0; i < 6; i++ {
		if !ProgramaInstalado(p.Produto, p.Termos, p.Gerenciador) {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("a remoção terminou, mas o programa continua na lista de instalados")
}

// RemoverPacote remove um programa do sistema.
func RemoverPacote(p cloud.Payload) error {
	return removerPacoteOS(p)
}

// instalarPacote baixa e instala um arquivo.
func instalarPacote(url, sha256sum, comando string) error {
	tmpDir, err := os.MkdirTemp("", "im-pkg-")
	if err != nil {
		tmpDir = filepath.Join(os.TempDir(), fmt.Sprintf("im-pkg-%d", time.Now().UnixNano()))
		os.MkdirAll(tmpDir, 0700)
	}
	defer os.RemoveAll(tmpDir)

	// Nome do arquivo da URL
	nome := filepath.Base(strings.Split(url, "?")[0])
	if nome == "" || nome == "." {
		nome = "pacote"
	}
	arq := filepath.Join(tmpDir, nome)

	// Download
	client := &http.Client{Timeout: 900 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar o pacote: %w", err)
	}
	defer resp.Body.Close()
	f, err := os.Create(arq)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("falha ao gravar o pacote: %w", err)
	}
	f.Close()

	// Verifica SHA256 se fornecido
	if sha256sum != "" {
		calc, err := sha256Arquivo(arq)
		if err != nil {
			return err
		}
		if !strings.EqualFold(calc, sha256sum) {
			return fmt.Errorf("assinatura do pacote não confere")
		}
	}

	// Executa o instalador
	return executarInstalador(arq, comando)
}

func sha256Arquivo(path string) (string, error) {
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
