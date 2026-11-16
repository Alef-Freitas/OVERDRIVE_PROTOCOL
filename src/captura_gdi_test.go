//go:build windows

package main

import (
	"testing"
)

func TestContarPixelsAcesos(t *testing.T) {
	t.Parallel()

	if got := contarPixelsAcesos(nil); got != 0 {
		t.Errorf("vazio devia dar 0, deu %d", got)
	}

	if got := contarPixelsAcesos(make([]byte, 400)); got != 0 {
		t.Errorf("preto devia dar 0, deu %d", got)
	}

	buf := make([]byte, 3*4)
	buf[0], buf[1], buf[2] = 10, 20, 30
	buf[8] = 1

	if got := contarPixelsAcesos(buf); got != 2 {
		t.Errorf("dois acesos, deu %d", got)
	}

	if got := contarPixelsAcesos([]byte{1, 2}); got != 0 {
		t.Error("curto devia dar 0 sem panico")
	}
}

func TestFotografarTelaGDI(t *testing.T) {
	t.Parallel()

	pix, origemX, origemY, larg, alt, err := fotografarTelaGDI()
	if err != nil {
		t.Skipf("gdi indisponivel: %v", err)
	}

	if int32(len(pix)) != larg*alt*bytesPorPixelCaptura {
		t.Errorf("buf %d difere de %dx%d", len(pix), larg, alt)
	}

	if origemX != 0 && origemY != 0 && larg <= 0 {
		t.Error("geometria invalida")
	}
}

func TestLarguraPreviaClip(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so DPI importa (96 sem janela).
	aplicativo := &app{}

	area := rectT{Left: 0, Top: 0, Right: 470, Bottom: 100}

	larga := aplicativo.larguraPreviaClip(area, 440)
	if larga <= 0 {
		t.Errorf("larga devia ser positiva, deu %d", larga)
	}

	// Meta comeca em 470-210=260; texto nao pode passar de 252.
	if larga > 252-14 {
		t.Errorf("texto invadia a meta: %d", larga)
	}

	estreita := aplicativo.larguraPreviaClip(rectT{Left: 0, Top: 0, Right: 200, Bottom: 100}, 190)
	if estreita < aplicativo.sc(40) {
		t.Errorf("estreita devia ter piso, deu %d", estreita)
	}
}
