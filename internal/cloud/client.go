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
	Produto          string   `json:"produto"`
	Termos           []string `json:"termos"`
	URL              string   `json:"url"`
	SHA256           string   `json:"sha256"`
	Argumentos       string   `json:"argumentos"`
	ArgumentosRemover string  `json:"argumentosRemover"`
	Gerenciador      string   `json:"gerenciador"`
	Versao           string   `json:"versao"`
}

type Ordem struct {
	ID      string  `json:"id"`
	Tipo    string  `json:"tipo"`
	Payload Payload `json:"payload"`
}

type AgenteInfo struct {
	Versao   string `json:"versao"`
	Atualizar bool  `json:"atualizar"`
}

type pollResp struct {
	Ordens    []Ordem    `json:"ordens"`
	Intervalo int        `json:"intervalo"`
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

func (c *Client) Autoatualizar(novaVersao string, args []string) error {
	ext := ""
	if os.Getenv("OS") == "Windows_NT" {
		ext = ".exe"
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	tmp := self + ".novo"
	if err := download(c.baseURL+"/agente-endpoint/im-agent"+ext, tmp); err != nil {
		return err
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() < 1000 {
		os.Remove(tmp)
		return fmt.Errorf("arquivo baixado inválido")
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, self); err != nil {
		os.Remove(tmp)
		return err
	}
	fmt.Printf("Agente atualizado para %s; reiniciando.\n", novaVersao)
	// Reinicia o processo com os mesmos argumentos
	return execRestart(self, args)
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

func download(url, dest string) error {
	client := &http.Client{Timeout: 900 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
