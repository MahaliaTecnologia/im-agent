package main

import (
	"testing"
	"time"
)

func TestIntervaloLocalizacao(t *testing.T) {
	casos := map[int]time.Duration{0: 15 * time.Minute, -3: 15 * time.Minute, 2: 5 * time.Minute, 30: 30 * time.Minute, 999: 240 * time.Minute}
	for in, esperado := range casos {
		if got := intervaloLocalizacao(in); got != esperado {
			t.Errorf("%d: %s != %s", in, got, esperado)
		}
	}
}
