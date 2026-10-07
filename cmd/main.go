package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/insightmanager/im-agent/internal/cloud"
	"github.com/insightmanager/im-agent/internal/health"
	"github.com/insightmanager/im-agent/internal/inventory"
	"github.com/insightmanager/im-agent/internal/patches"
	"github.com/insightmanager/im-agent/internal/platform"
	"github.com/insightmanager/im-agent/internal/telemetry"
)

const versao = "2.0.0"

func main() {
	cloudURL := flag.String("cloud", "", "URL da nuvem (obrigatório)")
	chave := flag.String("chave", "", "Chave de registro imdev_xxx.yyy")
	hostID := flag.String("host-id", "", "Identificador do equipamento (ex: maquina.dominio)")
	flag.Parse()

	if *cloudURL == "" {
		fmt.Fprintln(os.Stderr, "Informe --cloud")
		os.Exit(2)
	}

	if *hostID == "" {
		h, err := platform.HostnameFQDN()
		if err != nil {
			h, _ = os.Hostname()
		}
		*hostID = h
	}

	dataDir := platform.DataDir()
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		dataDir = os.TempDir()
	}
	tokenPath := dataDir + "/canal.token"

	c := cloud.New(*cloudURL, versao, *hostID, platform.Sistema(), platform.Arch())

	// Carrega ou obtém token
	token := cloud.LerToken(tokenPath)
	if token == "" {
		var err error
		token, err = c.Registrar(*chave)
		if err != nil {
			log.Printf("Registro inicial falhou: %v; tentando novamente no laço.", err)
		} else {
			cloud.SalvarToken(tokenPath, token)
		}
	}
	c.SetToken(token)

	fmt.Printf("Canal direto ativo (%s). Consultando a nuvem periodicamente.\n", *hostID)

	intervalo := 60 * time.Second
	ciclo := 0

	for {
		if c.Token() == "" {
			tok, err := c.Registrar(*chave)
			if err == nil {
				cloud.SalvarToken(tokenPath, tok)
				c.SetToken(tok)
			}
		}

		if c.Token() != "" {
			resp, err := c.Poll(health.Coletar())
			if err != nil {
				c.RegistrarFalha(err.Error())
				if cloud.EhNaoAutorizado(err) && *chave != "" {
					tok, rerr := c.Registrar(*chave)
					if rerr == nil {
						cloud.SalvarToken(tokenPath, tok)
						c.SetToken(tok)
					}
				}
			} else {
				c.LimparFalhas()
				if resp.Intervalo > 0 {
					intervalo = time.Duration(resp.Intervalo) * time.Second
				}
				for _, ordem := range resp.Ordens {
					go executar(c, ordem)
				}
				if resp.Agente != nil && resp.Agente.Atualizar {
					if err := c.Autoatualizar(resp.Agente.Versao, os.Args); err != nil {
						log.Printf("Autoatualização falhou: %v", err)
					}
				}
			}
		}

		// Telemetria a cada 10 ciclos (~10 min)
		if ciclo%10 == 0 {
			go func() { _ = c.EnviarTelemetria(telemetry.Coletar(*cloudURL)) }()
		}
		// Inventário e patches a cada 360 ciclos (~6 horas)
		if ciclo%360 == 0 {
			go func() { _ = c.EnviarInventario(inventory.Coletar()) }()
			go func() { _ = c.EnviarPatches(patches.Listar()) }()
		}

		ciclo++
		time.Sleep(intervalo)
	}
}

func executar(c *cloud.Client, ordem cloud.Ordem) {
	// Inicia heartbeat para tarefas longas
	var stopBatida chan struct{}
	longas := map[string]bool{
		"instalar_pacote": true, "remover_pacote": true, "reinstalar_pacote": true,
		"aplicar_pendencias": true, "limpar_disco": true, "inventariar": true, "coletar": true,
	}
	if longas[ordem.Tipo] {
		stopBatida = c.IniciarBatida(ordem.Tipo)
		defer close(stopBatida)
	}

	switch ordem.Tipo {
	case "reiniciar":
		c.Responder(ordem.ID, true, "")
		time.Sleep(3 * time.Second)
		platform.Reiniciar()

	case "desligar":
		c.Responder(ordem.ID, true, "")
		time.Sleep(3 * time.Second)
		platform.Desligar()

	case "limpar_disco":
		platform.LimparDisco()
		c.Responder(ordem.ID, true, "")

	case "inventariar":
		inv := inventory.Coletar()
		if err := c.EnviarInventario(inv); err != nil {
			c.Responder(ordem.ID, false, "falha ao enviar o inventário")
		} else {
			c.Responder(ordem.ID, true, "")
		}

	case "coletar":
		inv := inventory.Coletar()
		pt := patches.Listar()
		e1 := c.EnviarInventario(inv)
		e2 := c.EnviarPatches(pt)
		if e1 != nil || e2 != nil {
			c.Responder(ordem.ID, false, "falha ao atualizar os dados do equipamento")
		} else {
			c.Responder(ordem.ID, true, "")
		}

	case "aplicar_pendencias":
		if err := platform.AplicarAtualizacoesSistema(); err != nil {
			c.Responder(ordem.ID, false, err.Error())
		} else {
			_ = c.EnviarPatches(patches.Listar())
			c.Responder(ordem.ID, true, "")
		}

	case "instalar_pacote":
		p := ordem.Payload
		if err := platform.AplicarInstalacao(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		if err := platform.ConferirInstalado(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		c.Responder(ordem.ID, true, "")
		go func() { _ = c.EnviarInventario(inventory.Coletar()) }()

	case "remover_pacote":
		p := ordem.Payload
		if err := platform.RemoverPacote(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		if err := platform.ConferirRemovido(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		c.Responder(ordem.ID, true, "")
		go func() { _ = c.EnviarInventario(inventory.Coletar()) }()

	case "reinstalar_pacote":
		p := ordem.Payload
		if platform.ProgramaInstalado(p.Produto, p.Termos, p.Gerenciador) {
			_ = platform.RemoverPacote(p)
			time.Sleep(5 * time.Second)
		}
		if err := platform.AplicarInstalacao(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		if err := platform.ConferirInstalado(p); err != nil {
			c.Responder(ordem.ID, false, err.Error())
			return
		}
		c.Responder(ordem.ID, true, "")
		go func() { _ = c.EnviarInventario(inventory.Coletar()) }()

	default:
		c.Responder(ordem.ID, false, "ordem não suportada: "+ordem.Tipo)
	}
}
