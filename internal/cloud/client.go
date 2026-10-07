package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// TamanhoMaximoDownload limita qualquer arquivo baixado (agente ou pacote).
const TamanhoMaximoDownload = 2 << 30 // 2 GiB

type Client struct {
	baseURL string
	versao  string
	hostID  string
	sistema string
	arch    string

	mu          sync.RWMutex
	token       string
	falhasNuvem int
	ultimoErro  string

	// Atualização que falhou: não repetir o mesmo download antes de 1 hora.
	atualizacaoFalhou   string
	atualizacaoFalhouEm time.Time
}

func New(baseURL, versao, hostID, sistema, arch string) *Client {
	return &Client{
		baseURL: baseURL,
		versao:  versao,
		hostID:  hostID,
		sistema: sistema,
		arch:    arch,
	}
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) SetToken(t string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = t
}

func (c *Client) RegistrarFalha(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.falhasNuvem++
	c.ultimoErro = msg
}

func (c *Client) LimparFalhas() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.falhasNuvem = 0
	c.ultimoErro = ""
}

type Saude struct {
	OpsiAtivo    interface{} `json:"opsiAtivo"`
	DiscoLivreGb interface{} `json:"discoLivreGb"`
	UptimeHoras  interface{} `json:"uptimeHoras"`
	FalhasNuvem  int         `json:"falhasNuvem"`
	Erro         interface{} `json:"erro"`
}

type enrollReq struct {
	OpsiID      string `json:"opsiId"`
	Hostname    string `json:"hostname"`
	Sistema     string `json:"sistema"`
	Arquitetura string `json:"arquitetura"`
	Versao      string `json:"versao"`
}

type enrollResp struct {
	Token string `json:"token"`
}

func (c *Client) Registrar(chave string) (string, error) {
	if chave == "" {
		return "", fmt.Errorf("sem credencial e sem chave de registro")
	}
	body, _ := json.Marshal(enrollReq{
		OpsiID:      c.hostID,
		Hostname:    c.hostID,
		Sistema:     c.sistema,
		Arquitetura: c.arch,
		Versao:      c.versao,
	})
	req, _ := http.NewRequest("POST", c.baseURL+"/api/public/device-agent/enroll", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+chave)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("registro recusado: HTTP %d", resp.StatusCode)
	}
	var r enrollResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Token == "" {
		return "", fmt.Errorf("registro recusado pela nuvem")
	}
	fmt.Println("Equipamento registrado no Insight Manager.")
	return r.Token, nil
}

type pollReq struct {
	Versao   string `json:"versao"`
	Hostname string `json:"hostname"`
	Sistema  string `json:"sistema"`
	Saude    Saude  `json:"saude"`
}

type Payload struct {
	Produto           string   `json:"produto"`
	Termos            []string `json:"termos"`
	URL               string   `json:"url"`
	SHA256            string   `json:"sha256"`
	Argumentos        string   `json:"argumentos"`
	ArgumentosRemover string   `json:"argumentosRemover"`
	Gerenciador       string   `json:"gerenciador"`
	Versao            string   `json:"versao"`
}

type Ordem struct {
	ID      string  `json:"id"`
	Tipo    string  `json:"tipo"`
	Payload Payload `json:"payload"`
}

// AgenteInfo é a instrução de autoatualização enviada pela nuvem no poll.
// Sem SHA256 a atualização é recusada.
type AgenteInfo struct {
	Versao     string `json:"versao"`
	Atualizar  bool   `json:"atualizar"`
	URL        string `json:"url"`        // opcional; padrão /agente-endpoint/im-agent?os=&arch=
	SHA256     string `json:"sha256"`     // obrigatório para atualizar
	Assinatura string `json:"assinatura"` // Ed25519 (base64) do SHA-256; obrigatória se o binário tiver chave pública embutida
}

type pollResp struct {
	Ordens    []Ordem     `json:"ordens"`
	Intervalo int         `json:"intervalo"`
	Agente    *AgenteInfo `json:"agente"`
}

func (c *Client) Poll(saude Saude) (*pollResp, error) {
	body, _ := json.Marshal(pollReq{
		Versao:   c.versao,
		Hostname: c.hostID,
		Sistema:  c.sistema,
		Saude:    saude,
	})
	req, _ := http.NewRequest("POST", c.baseURL+"/api/public/device-agent/poll", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.Token())
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, &ErrNaoAutorizado{}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poll HTTP %d", resp.StatusCode)
	}
	var r pollResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

type resultReq struct {
	CommandID string `json:"commandId"`
	OK        bool   `json:"ok"`
	Erro      string `json:"erro,omitempty"`
}

func (c *Client) Responder(id string, ok bool, erro string) {
	body, _ := json.Marshal(resultReq{CommandID: id, OK: ok, Erro: erro})
	req, _ := http.NewRequest("POST", c.baseURL+"/api/public/device-agent/result", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.Token())
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *Client) EnviarInventario(inv interface{}) error {
	return c.post("/api/public/device-agent/inventory", inv, 120*time.Second)
}

func (c *Client) EnviarPatches(pt interface{}) error {
	return c.post("/api/public/device-agent/patches", pt, 180*time.Second)
}

func (c *Client) EnviarTelemetria(tel interface{}) error {
	return c.post("/api/public/device-agent/telemetry", tel, 60*time.Second)
}

func (c *Client) IniciarBatida(tarefa string) chan struct{} {
	stop := make(chan struct{})
	go func() {
		tipo := struct {
			Versao   string `json:"versao"`
			Hostname string `json:"hostname"`
			Tarefa   string `json:"tarefa"`
		}{c.versao, c.hostID, tarefa}
		for {
			select {
			case <-stop:
				return
			case <-time.After(45 * time.Second):
				_ = c.post("/api/public/device-agent/heartbeat", tipo, 20*time.Second)
			}
		}
	}()
	return stop
}

// Autoatualizar baixa a nova versão, confere integridade (SHA-256 e, quando
// configurado, assinatura Ed25519), faz um autoteste e só então troca o
// executável, guardando o anterior em <executável>.anterior.
func (c *Client) Autoatualizar(info AgenteInfo, goos, goarch string, args []string) error {
	if info.Versao != "" && info.Versao == c.versao {
		return nil // já está nesta versão
	}
	chave := info.Versao + "|" + info.SHA256
	if chave == c.atualizacaoFalhou && time.Since(c.atualizacaoFalhouEm) < time.Hour {
		return nil // falhou há pouco; espera antes de tentar de novo
	}
	err := c.autoatualizar(info, goos, goarch, args)
	if err != nil {
		c.atualizacaoFalhou, c.atualizacaoFalhouEm = chave, time.Now()
	}
	return err
}

func (c *Client) autoatualizar(info AgenteInfo, goos, goarch string, args []string) error {
	if info.SHA256 == "" {
		return fmt.Errorf("atualização recusada: a nuvem não informou o SHA-256 da versão %s", info.Versao)
	}
	if err := VerificarAssinatura(info.SHA256, info.Assinatura); err != nil {
		return err
	}
	url, err := c.urlAtualizacao(info.URL, goos, goarch)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	tmp := self + ".novo"
	_ = os.Remove(tmp)
	if err := Baixar(url, tmp, info.SHA256); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download da atualização: %w", err)
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := autoteste(tmp, info.Versao); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("a nova versão não passou no autoteste: %w", err)
	}
	if err := trocarExecutavel(self, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	fmt.Printf("Agente atualizado para %s; reiniciando.\n", info.Versao)
	return execRestart(self, args)
}

// urlAtualizacao aceita só HTTPS e só o mesmo host da nuvem configurada.
func (c *Client) urlAtualizacao(informada, goos, goarch string) (string, error) {
	if informada == "" {
		return fmt.Sprintf("%s/agente-endpoint/im-agent?os=%s&arch=%s", c.baseURL, goos, goarch), nil
	}
	if len(informada) > 0 && informada[0] == '/' {
		return c.baseURL + informada, nil
	}
	if !mesmoHost(informada, c.baseURL) {
		return "", fmt.Errorf("atualização recusada: endereço fora do domínio da nuvem")
	}
	return informada, nil
}

func (c *Client) post(path string, payload interface{}, timeout time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.Token())
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

type ErrNaoAutorizado struct{}

func (e *ErrNaoAutorizado) Error() string { return "não autorizado" }

func EhNaoAutorizado(err error) bool {
	_, ok := err.(*ErrNaoAutorizado)
	return ok
}

func LerToken(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	t := string(bytes.TrimSpace(b))
	return t
}

func SalvarToken(path, token string) {
	_ = os.WriteFile(path, []byte(token), 0600)
}

// Baixar grava url em dest, exigindo HTTP 200, tamanho máximo e, se
// sha256esperado não for vazio, conferindo o hash. Redirecionamentos para
// HTTP simples são recusados.
func Baixar(url, dest, sha256esperado string) error {
	client := &http.Client{
		Timeout: 900 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("redirecionamentos demais")
			}
			if req.URL.Scheme != "https" && !PermitirHTTP {
				return fmt.Errorf("redirecionamento para endereço sem HTTPS recusado")
			}
			return nil
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("o servidor respondeu HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, TamanhoMaximoDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > TamanhoMaximoDownload {
		return fmt.Errorf("arquivo maior que o limite permitido")
	}
	if n == 0 {
		return fmt.Errorf("arquivo vazio")
	}
	if sha256esperado != "" {
		calc, err := SHA256Arquivo(dest)
		if err != nil {
			return err
		}
		if !iguaisHex(calc, sha256esperado) {
			return fmt.Errorf("SHA-256 não confere")
		}
	}
	return nil
}
