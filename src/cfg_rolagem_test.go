//go:build windows

package main

import (
	"testing"
)

// TestCfgRolagemPonto garante o arrasto preso nos limites.
func TestCfgRolagemPonto(t *testing.T) {
	t.Parallel()

	if got := cfgRolagemPonto(0, 0, 400, 100, 0); got != 0 {
		t.Errorf("sem viagem devia dar 0, deu %d", got)
	}

	if got := cfgRolagemPonto(10, 200, 400, 10, 0); got != 0 {
		t.Errorf("topo devia dar 0, deu %d", got)
	}

	if got := cfgRolagemPonto(10, 200, 400, 210, 0); got != 400 {
		t.Errorf("base devia prender em 400, deu %d", got)
	}

	if got := cfgRolagemPonto(10, 200, 400, 310, 0); got != 400 {
		t.Errorf("alem devia prender em 400, deu %d", got)
	}

	if got := cfgRolagemPonto(10, 200, 400, 110, 0); got != 200 {
		t.Errorf("meio devia dar 200, deu %d", got)
	}

	if got := cfgRolagemPonto(10, 200, 400, 0, 0); got != 0 {
		t.Errorf("acima devia prender em 0, deu %d", got)
	}
}

// TestCfgBarraGeometria garante trilha, polegar e zonas na janela de config.
func TestCfgBarraGeometria(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	salvo := cfgUI
	defer func() { cfgUI = salvo }()

	//exhaustruct:ignore // App de teste: so a geometria importa.
	aplicativo := &app{}
	cfgUI.w = 600
	cfgUI.h = 500
	cfgUI.headH = 52
	cfgUI.scroll = 0
	cfgUI.maxScroll = 400

	trilho := aplicativo.trilhoBarraCfg()
	if trilho.Width() <= 0 || trilho.Height() <= 0 {
		t.Fatalf("trilha invalida: %+v", trilho)
	}

	polegada := aplicativo.polegadaBarraCfg(trilho)
	if polegada.Height() >= trilho.Height() {
		t.Errorf("polegar devia ser menor que a trilha: %+v", polegada)
	}

	if aplicativo.cfgZonaBarra(0, 0) != 0 {
		t.Error("fora devia dar 0")
	}

	meio := (polegada.Top + polegada.Bottom) / 2
	if aplicativo.cfgZonaBarra(polegada.Left+1, meio) != 2 {
		t.Error("polegar devia dar 2")
	}

	if aplicativo.cfgZonaBarra(trilho.Left+1, trilho.Bottom-1) != 1 {
		t.Error("trilha devia dar 1")
	}

	cfgUI.maxScroll = 0

	if aplicativo.cfgZonaBarra(trilho.Left+1, trilho.Top+1) != 0 {
		t.Error("sem rolagem devia dar 0")
	}
}
