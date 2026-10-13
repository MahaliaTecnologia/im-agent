package inventory

import "strings"

// serieValida descarta valores genéricos que alguns fabricantes gravam no BIOS.
func serieValida(s string) string {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "0", "none", "null", "default string", "to be filled by o.e.m.", "system serial number", "not specified", "not applicable", "123456789", "0123456789":
		return ""
	}
	if len(s) > 120 {
		return ""
	}
	return s
}
