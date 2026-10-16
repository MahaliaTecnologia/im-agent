package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/insightmanager/im-agent/internal/cloud"
	"github.com/insightmanager/im-agent/internal/health"
	"github.com/insightmanager/im-agent/internal/inventory"
	"github.com/insightmanager/im-agent/internal/location"
	"github.com/insightmanager/im-agent/internal/patches"
	"github.com/insightmanager/im-agent/internal/platform"
	"github.com/insightmanager/im-agent/internal/telemetry"
)

const versao = "2.1.5"

func main() {
	cloudURL := flag.String("cloud", "", "URL da nuvem (obrigatório)")
	chave := flag.String("chave", "", "Chave de registro imdev_xxx.yyy")
	hostID := flag.String("host-id", "", "Identificador do equipamento (ex: maquina.dominio)")
	mostrarVersao := flag.Bool("versao", false, "Mostra a versão e sai (usado no autoteste da atualização)")
	mostrarVersaoEn := flag.Bool("version", false, "Igual a --versao (compatibilidade com scripts antigos)")
	permitirHTTP := flag.Bool("permitir-http", false, "Aceita nuvem sem HTTPS (somente desenvolvimento)")
	flag.Parse()

	if *mostrarVersao || *mostrarVersaoEn {
		fmt.Println(versao)
		return
	}

	if *cloudURL == "" {
		fmt.Fprintln(os.Stderr, "Informe --cloud")
		os.Exit(2)
	}
	cloud.PermitirHTTP = *permitirHTTP
	urlNuvem, err := cloud.ValidarURLNuvem(*cloudURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	*cloudURL = urlNuvem

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
	c.SetSerie(inventory.NumeroSerie())

	// Registro com espera progressiva: 1, 2, 4… até 30 minutos entre
	// tentativas recusadas (por exemplo, cota de ativos esgotada).
	var proximoRegistro time.Time
	falhasRegistro := 0
	registrar := func() {
		if time.Now().Before(proximoRegistro) {
			return
		}
		tok, err := c.Registrar(*chave)
		if err != nil {
			falhasRegistro++
			espera := time.Minute << uint(min(falhasRegistro-1, 5))
			if espera > 30*time.Minute {
				espera = 30 * time.Minute
			}
			proximoRegistro = time.Now().Add(espera)
			log.Printf("Registro falhou: %v; nova tentativa em %s.", err, espera)
			return
		}
		falhasRegistro = 0
		proximoRegistro = time.Time{}
		cloud.SalvarToken(tokenPath, tok)
		c.SetToken(tok)
	}

	// Carrega ou obtém token
	c.SetToken(cloud.LerToken(tokenPath))
	if c.Token() == "" {
		registrar()
	}

	fmt.Printf("Canal direto ativo (%s). Consultando a nuvem periodicamente.\n", *hostID)

	intervalo := 60 * time.Second
	// Telemetria e inventário por relógio (não por número de consultas): a
	// nuvem espaça as consultas quando não há ordens, o que atrasaria a coleta.
	const cadaTelemetria = 10 * time.Minute
	const cadaInventario = 6 * time.Hour
	var ultimaTelemetria, ultimoInventario, ultimaLocalizacao time.Time
	var cfgLocalizacao cloud.LocalizacaoCfg
	var coletandoLocalizacao atomic.Bool

	for {
		if c.Token() == "" {
			registrar()
		}

		if c.Token() != "" {
			resp, err := c.Poll(health.Coletar())
			if err != nil {
				c.RegistrarFalha(err.Error())
				if cloud.EhNaoAutorizado(err) && *chave != "" {
					registrar()
				}
			} else {
				c.LimparFalhas()
				if resp.Intervalo > 0 {
					intervalo = time.Duration(resp.Intervalo) * time.Second
				}
				for _, ordem := range resp.Ordens {
					go executar(c, ordem)
				}
				if resp.Localizacao != nil {
					cfgLocalizacao = *resp.Localizacao
				} else {
					cfgLocalizacao = cloud.LocalizacaoCfg{}
				}
				if resp.Agente != nil && resp.Agente.Atualizar {
					if err := c.Autoatualizar(*resp.Agente, runtime.GOOS, runtime.GOARCH, os.Args); err != nil {
						log.Printf("Autoatualização falhou: %v", err)
					}
				}
			}
		}

		agora := time.Now()
		if c.Token() != "" && agora.Sub(ultimaTelemetria) >= cadaTelemetria {
			ultimaTelemetria = agora
			go func() { _ = c.EnviarTelemetria(telemetry.Normalizar(telemetry.Coletar(*cloudURL))) }()
		}
		if c.Token() != "" && agora.Sub(ultimoInventario) >= cadaInventario {
			ultimoInventario = agora
			go func() { _ = c.EnviarInventario(inventory.Coletar()) }()
			go func() { _ = c.EnviarPatches(patches.Listar()) }()
		}

		// Localização só quando a nuvem pede (organização ligou para computadores).
		if c.Token() != "" && cfgLocalizacao.Ativa && agora.Sub(ultimaLocalizacao) >= intervaloLocalizacao(cfgLocalizacao.IntervaloMin) &&
			coletandoLocalizacao.CompareAndSwap(false, true) {
			ultimaLocalizacao = agora
			go func() {
				defer coletandoLocalizacao.Store(false)
				if err := c.EnviarLocalizacao(location.Coletar()); err != nil {
					log.Printf("Envio da localização falhou: %v", err)
				}
			}()
		}

		// Nunca dorme mais que o intervalo da telemetria.
		espera := intervalo
		if espera > cadaTelemetria {
			espera = cadaTelemetria
		}
		time.Sleep(espera)
	}
}

// intervaloLocalizacao limita o pedido da nuvem a 5 min..4 h (padrão 15 min).
func intervaloLocalizacao(min int) time.Duration {
	switch {
	case min <= 0:
		min = 15
	case min < 5:
		min = 5
	case min > 240:
		min = 240
	}
	return time.Duration(min) * time.Minute
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
