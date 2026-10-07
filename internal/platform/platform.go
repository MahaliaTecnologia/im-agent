package platform

import (
	"fmt"
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

// instalarPacote baixa e instala um arquivo. Exige HTTP 200; com SHA-256
// informado, confere o hash; sem SHA-256, só aceita HTTPS.
func instalarPacote(url, sha256sum, comando string) error {
	if err := cloud.ValidarURLPacote(url, sha256sum != ""); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "im-pkg-")
	if err != nil {
		return fmt.Errorf("não foi possível criar pasta temporária: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	nome := filepath.Base(strings.Split(url, "?")[0])
	if nome == "" || nome == "." || nome == "/" {
		nome = "pacote"
	}
	arq := filepath.Join(tmpDir, nome)

	if err := cloud.Baixar(url, arq, sha256sum); err != nil {
		if strings.Contains(err.Error(), "SHA-256") {
			return fmt.Errorf("assinatura do pacote não confere")
		}
		return fmt.Errorf("falha ao baixar o pacote: %w", err)
	}
	return executarInstalador(arq, comando)
}

// candidatoUnico devolve o único item cujo nome corresponde exatamente a um
// dos alvos (sem diferenciar maiúsculas). Nenhum ou mais de um = erro: a
// remoção nunca escolhe "o primeiro parecido".
func candidatoUnico(nomes []string, alvos []string) (string, error) {
	vistos := map[string]bool{}
	var achados []string
	for _, n := range nomes {
		nn := strings.ToLower(strings.TrimSpace(n))
		if nn == "" {
			continue
		}
		for _, a := range alvos {
			if aa := strings.ToLower(strings.TrimSpace(a)); aa != "" && nn == aa && !vistos[nn] {
				vistos[nn] = true
				achados = append(achados, strings.TrimSpace(n))
			}
		}
	}
	switch len(achados) {
	case 0:
		return "", fmt.Errorf("nenhum programa com nome exato encontrado")
	case 1:
		return achados[0], nil
	default:
		return "", fmt.Errorf("mais de um programa corresponde (%s); informe o comando de remoção", strings.Join(achados, ", "))
	}
}

// alvosRemocao junta produto e termos sem alterar o slice recebido.
func alvosRemocao(p cloud.Payload) []string {
	out := make([]string, 0, len(p.Termos)+2)
	out = append(out, p.Termos...)
	if p.Produto != "" {
		out = append(out, p.Produto)
	}
	return out
}
