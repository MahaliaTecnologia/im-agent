package inventory

import "testing"

func TestSerieValida(t *testing.T) {
	for _, s := range []string{"", " ", "To Be Filled By O.E.M.", "Default string", "0"} {
		if serieValida(s) != "" {
			t.Fatalf("%q deveria ser descartado", s)
		}
	}
	if serieValida(" C02XK1ABJG5H ") != "C02XK1ABJG5H" {
		t.Fatal("série válida perdida")
	}
}
