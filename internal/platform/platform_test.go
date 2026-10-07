package platform

import "testing"

func TestCandidatoUnico(t *testing.T) {
	nomes := []string{"Microsoft Office Professional", "Microsoft Office Tools", "7-Zip 23.01", "google-chrome-stable", "python3", "python3-minimal"}
	if _, err := candidatoUnico(nomes, []string{"Office"}); err == nil {
		t.Fatal("trecho de nome não pode remover nada")
	}
	if _, err := candidatoUnico(nomes, []string{"python"}); err == nil {
		t.Fatal("'python' não pode casar com python3")
	}
	if a, err := candidatoUnico(nomes, []string{"Google Chrome", "google-chrome-stable"}); err != nil || a != "google-chrome-stable" {
		t.Fatalf("nome exato deveria ser encontrado: %q %v", a, err)
	}
	if a, err := candidatoUnico(nomes, []string{"7-ZIP 23.01"}); err != nil || a != "7-Zip 23.01" {
		t.Fatalf("comparação deve ignorar maiúsculas: %q %v", a, err)
	}
	if _, err := candidatoUnico(nomes, []string{"python3", "python3-minimal"}); err == nil {
		t.Fatal("dois candidatos devem gerar erro")
	}
}
