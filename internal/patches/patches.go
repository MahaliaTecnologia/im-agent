package patches

type Patch struct {
	Identificador string  `json:"identificador"`
	Titulo        string  `json:"titulo"`
	Categoria     string  `json:"categoria"`
	Severidade    string  `json:"severidade"`
	Plataforma    string  `json:"plataforma"`
	TamanhoMb     *float64 `json:"tamanhoMb,omitempty"`
	Reinicio      bool    `json:"reinicio,omitempty"`
}

type PatchPayload struct {
	Atualizacoes []Patch `json:"atualizacoes"`
}
