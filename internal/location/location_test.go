package location

import "testing"

func TestWindowsOK(t *testing.T) {
	p := interpretarWindows("\r\nok|-23.5505|-46.6333|35\r\n")
	if p.Status != StatusOK || *p.Lat != -23.5505 || *p.Lon != -46.6333 || *p.PrecisaoM != 35 || p.CapturadaEm == "" {
		t.Fatalf("%+v", p)
	}
}

func TestWindowsNegadaIndisponivelErro(t *testing.T) {
	if p := interpretarWindows("negada"); p.Status != StatusNegada {
		t.Fatal(p)
	}
	if p := interpretarWindows("indisponivel|NoData"); p.Status != StatusIndisponivel || p.Detalhe != "sem posição (NoData)" {
		t.Fatal(p)
	}
	if p := interpretarWindows("erro|Add-Type falhou"); p.Status != StatusErro {
		t.Fatal(p)
	}
	if p := interpretarWindows("ok|abc|1|2"); p.Status != StatusErro {
		t.Fatal(p)
	}
}

func TestCoordenadaInvalida(t *testing.T) {
	if p := interpretarWindows("ok|0|0|10"); p.Status != StatusErro {
		t.Fatal(p)
	}
	if p := interpretarWindows("ok|91|10|10"); p.Status != StatusErro {
		t.Fatal(p)
	}
	// Precisão absurda é omitida, posição continua válida.
	if p := interpretarWindows("ok|-23|-46|NaN"); p.Status != StatusOK || p.PrecisaoM != nil {
		t.Fatalf("%+v", p)
	}
}

func TestGeoClue(t *testing.T) {
	saida := "Client object: /org/freedesktop/GeoClue2/Client/1\n\nNew location:\nLatitude:    -25.428400°\nLongitude:   -49.273300°\nAccuracy:    40.000000 meters\nAltitude:    Unknown\n"
	p := interpretarGeoClue(saida)
	if p.Status != StatusOK || *p.Lat != -25.4284 || *p.Lon != -49.2733 || *p.PrecisaoM != 40 {
		t.Fatalf("%+v", p)
	}
	if p := interpretarGeoClue("Failed to connect to GeoClue2 service"); p.Status != StatusIndisponivel {
		t.Fatal(p)
	}
}
