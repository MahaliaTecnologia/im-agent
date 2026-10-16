//go:build darwin

package location

// No macOS a localização exige um app assinado (Developer ID) com permissão do
// usuário; o agente roda como serviço sem interface. Fica para quando a conta
// Apple estiver aprovada.
func coletarOS() Posicao {
	return Posicao{Status: StatusNaoSuportado, Fonte: "macos", Detalhe: "macOS ainda não suportado pelo agente"}
}
