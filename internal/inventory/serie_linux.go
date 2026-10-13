//go:build linux

package inventory

// NumeroSerie lê só o número de série (leve; usado no registro do agente).
func NumeroSerie() string {
	v, err := lerArquivo("/sys/class/dmi/id/product_serial")
	if err != nil {
		return ""
	}
	return serieValida(v)
}
