//go:build windows

package main

import "testing"

func TestCanvasInicial(t *testing.T) {
	t.Parallel()

	canvasLocal := novoCanvas()
	if len(canvasLocal.cartoes) < 2 || len(canvasLocal.trilhas) < 1 {
		t.Error("canvas inicial vazio")
	}

	if canvasLocal.zoom <= 0 {
		t.Error("zoom invalido")
	}
}

func TestZoomLimites(t *testing.T) {
	t.Parallel()

	canvasLocal := novoCanvas()
	for range 100 {
		canvasLocal.aproximar(0, 0, 0, 0, zoomPasso)
	}

	if canvasLocal.zoom > zoomMaximo {
		t.Errorf("zoom estourou: %f", canvasLocal.zoom)
	}

	for range 100 {
		canvasLocal.aproximar(0, 0, 0, 0, 1/zoomPasso)
	}

	if canvasLocal.zoom < zoomMinimo {
		t.Errorf("zoom abaixo do minimo: %f", canvasLocal.zoom)
	}
}

func TestConversaoCoordenadas(t *testing.T) {
	t.Parallel()

	canvasLocal := novoCanvas()

	p := canvasLocal.mundoParaTela(10, 20, 100, 100)
	if p.X != 110 || p.Y != 120 {
		t.Errorf("conversao errada: %+v", p)
	}
}
