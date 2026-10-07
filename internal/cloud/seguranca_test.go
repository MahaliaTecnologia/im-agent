package cloud

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestValidarURLNuvem(t *testing.T) {
	PermitirHTTP = false
	if _, err := ValidarURLNuvem("http://insightmanager.com.br"); err == nil {
		t.Fatal("http deveria ser recusado")
	}
	if u, err := ValidarURLNuvem("https://insightmanager.com.br/"); err != nil || u != "https://insightmanager.com.br" {
		t.Fatalf("https deveria passar sem barra final: %q %v", u, err)
	}
	PermitirHTTP = true
	defer func() { PermitirHTTP = false }()
	if _, err := ValidarURLNuvem("http://localhost:3000"); err != nil {
		t.Fatal("com --permitir-http, http deveria passar")
	}
}

func TestValidarURLPacote(t *testing.T) {
	PermitirHTTP = false
	if ValidarURLPacote("http://x.com/a.msi", false) == nil {
		t.Fatal("http sem hash deveria ser recusado")
	}
	if ValidarURLPacote("http://x.com/a.msi", true) != nil {
		t.Fatal("http com hash pode passar (o hash protege)")
	}
	if ValidarURLPacote("https://x.com/a.msi", false) != nil {
		t.Fatal("https sem hash deveria passar")
	}
}

func servidor(t *testing.T, status int, corpo []byte) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write(corpo)
	}))
}

func TestBaixar(t *testing.T) {
	corpo := make([]byte, 5000)
	for i := range corpo {
		corpo[i] = byte(i)
	}
	h := sha256.Sum256(corpo)
	hash := hex.EncodeToString(h[:])
	dir := t.TempDir()

	erro := servidor(t, 404, []byte("<html>"+string(make([]byte, 3000))+"</html>"))
	defer erro.Close()
	http.DefaultTransport = erro.Client().Transport
	if err := Baixar(erro.URL, filepath.Join(dir, "a"), ""); err == nil {
		t.Fatal("HTTP 404 deveria ser recusado mesmo com corpo grande")
	}

	ok := servidor(t, 200, corpo)
	defer ok.Close()
	http.DefaultTransport = ok.Client().Transport
	if err := Baixar(ok.URL, filepath.Join(dir, "b"), "00"+hash[2:]); err == nil {
		t.Fatal("hash divergente deveria ser recusado")
	}
	if err := Baixar(ok.URL, filepath.Join(dir, "c"), hash); err != nil {
		t.Fatalf("download válido falhou: %v", err)
	}
}

func TestVerificarAssinatura(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	ChavePublicaAtualizacao = base64.StdEncoding.EncodeToString(pub)
	defer func() { ChavePublicaAtualizacao = "" }()
	hash := "ABCDEF"
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("abcdef")))
	if err := VerificarAssinatura(hash, sig); err != nil {
		t.Fatalf("assinatura válida recusada: %v", err)
	}
	if VerificarAssinatura("abcdee", sig) == nil {
		t.Fatal("assinatura de outro hash deveria ser recusada")
	}
	if VerificarAssinatura(hash, "") == nil {
		t.Fatal("sem assinatura deveria ser recusado quando há chave embutida")
	}
	ChavePublicaAtualizacao = ""
	if VerificarAssinatura(hash, "") != nil {
		t.Fatal("sem chave embutida, assinatura não é exigida")
	}
}

func TestURLAtualizacao(t *testing.T) {
	c := New("https://insightmanager.com.br", "x", "h", "linux", "amd64")
	if u, _ := c.urlAtualizacao("", "linux", "amd64"); u != "https://insightmanager.com.br/agente-endpoint/im-agent?os=linux&arch=amd64" {
		t.Fatalf("url padrão errada: %s", u)
	}
	if _, err := c.urlAtualizacao("https://outro.com/im-agent", "linux", "amd64"); err == nil {
		t.Fatal("host diferente deveria ser recusado")
	}
	if u, err := c.urlAtualizacao("/agente-endpoint/im-agent-linux-amd64", "linux", "amd64"); err != nil || u != "https://insightmanager.com.br/agente-endpoint/im-agent-linux-amd64" {
		t.Fatalf("caminho relativo deveria usar o host da nuvem: %s %v", u, err)
	}
}

func TestAutoatualizarSemHash(t *testing.T) {
	c := New("https://insightmanager.com.br", "x", "h", "linux", "amd64")
	if err := c.Autoatualizar(AgenteInfo{Versao: "9.9.9", Atualizar: true}, "linux", "amd64", nil); err == nil {
		t.Fatal("atualização sem SHA-256 deveria ser recusada")
	}
}

func TestTrocarExecutavel(t *testing.T) {
	dir := t.TempDir()
	self, novo := filepath.Join(dir, "im-agent"), filepath.Join(dir, "im-agent.novo")
	os.WriteFile(self, []byte("velho"), 0755)
	os.WriteFile(novo, []byte("novo"), 0755)
	if err := trocarExecutavel(self, novo); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(self); string(b) != "novo" {
		t.Fatal("executável não foi trocado")
	}
	if b, _ := os.ReadFile(self + ".anterior"); string(b) != "velho" {
		t.Fatal("versão anterior não foi guardada")
	}
}

func TestAutoatualizarMesmaVersao(t *testing.T) {
	c := New("https://insightmanager.com.br", "2.1.0", "h", "linux", "amd64")
	if err := c.Autoatualizar(AgenteInfo{Versao: "2.1.0", Atualizar: true}, "linux", "amd64", nil); err != nil {
		t.Fatal("mesma versão deve ser ignorada sem erro")
	}
}

func TestAutoatualizarNaoRepeteFalha(t *testing.T) {
	c := New("https://insightmanager.com.br", "2.0.0", "h", "linux", "amd64")
	info := AgenteInfo{Versao: "2.1.0", Atualizar: true}
	if c.Autoatualizar(info, "linux", "amd64", nil) == nil {
		t.Fatal("primeira tentativa sem hash deveria falhar")
	}
	if c.Autoatualizar(info, "linux", "amd64", nil) != nil {
		t.Fatal("segunda tentativa igual deveria ser adiada sem erro")
	}
}
