package inventory

// Inventario representa o payload enviado à nuvem.
type Inventario struct {
	Hardware  Hardware   `json:"hardware"`
	Programas []Programa `json:"programas"`
}

type Hardware struct {
	Fabricante  interface{} `json:"fabricante"`
	Modelo      interface{} `json:"modelo"`
	Serie       interface{} `json:"serie"`
	Processador interface{} `json:"processador"`
	CPUNucleos  interface{} `json:"cpuNucleos"`
	MemoriaMb   interface{} `json:"memoriaMb"`
	DiscoTotalGb interface{} `json:"discoTotalGb"`
	BiosData    interface{} `json:"biosData"`
	Sistema     string      `json:"sistema"`
	GPU         interface{} `json:"gpu"`
	TPM         interface{} `json:"tpm"`
	Discos      []Disco     `json:"discos"`
	MemoriaSlots []Slot     `json:"memoriaSlots"`
	Rede        []Iface     `json:"rede"`
	Bateria     interface{} `json:"bateria"`
	Monitores   []Monitor   `json:"monitores"`
}

type Disco struct {
	Nome      string  `json:"nome"`
	TamanhoGb float64 `json:"tamanhoGb"`
	LivreGb   float64 `json:"livreGb"`
	Tipo      interface{} `json:"tipo"`
}

type Slot struct {
	Slot      string  `json:"slot"`
	TamanhoMb int     `json:"tamanhoMb"`
	Tipo      interface{} `json:"tipo"`
	Velocidade interface{} `json:"velocidade"`
}

type Iface struct {
	Nome string      `json:"nome"`
	MAC  string      `json:"mac"`
	IP   interface{} `json:"ip"`
}

type Monitor struct {
	Nome      interface{} `json:"nome"`
	Serie     interface{} `json:"serie"`
	Resolucao interface{} `json:"resolucao"`
}

type Programa struct {
	Nome        string `json:"nome"`
	Versao      string `json:"versao"`
	Editor      string `json:"editor,omitempty"`
	InstaladoEm string `json:"instaladoEm,omitempty"`
}
