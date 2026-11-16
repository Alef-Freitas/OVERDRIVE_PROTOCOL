//go:build windows

package main

import (
	"testing"
)

func TestAbaValidaLimites(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{tabs: []*tab{{url: "a"}, nil, {url: "c"}}}

	if got := aplicativo.abaValida(-1); got != nil {
		t.Error("negativo devia dar nil")
	}

	if got := aplicativo.abaValida(3); got != nil {
		t.Error("estouro devia dar nil")
	}

	if got := aplicativo.abaValida(1); got != nil {
		t.Error("entrada nil devia dar nil")
	}

	if got := aplicativo.abaValida(0); got == nil || got.url != "a" {
		t.Error("primeira devia voltar")
	}

	if got := aplicativo.abaValida(2); got == nil || got.url != "c" {
		t.Error("ultima devia voltar")
	}
}

func TestUrlDescarteAba(t *testing.T) {
	t.Parallel()

	const urlAbaTeste = "https://aba.com"

	comPane := &tab{url: "velha", pane: &pane{ultimaURL: "https://nova.com/x"}}
	if got := urlDescarteAba(comPane); got != "https://nova.com/x" {
		t.Errorf("pane ganha da aba, deu %q", got)
	}

	semPane := &tab{url: urlAbaTeste}
	if got := urlDescarteAba(semPane); got != urlAbaTeste {
		t.Errorf("sem pane usa a aba, deu %q", got)
	}

	paneVazio := &tab{url: urlAbaTeste, pane: &pane{}}
	if got := urlDescarteAba(paneVazio); got != urlAbaTeste {
		t.Errorf("pane sem url usa a aba, deu %q", got)
	}
}

func TestSnapshotTabAbout(t *testing.T) {
	t.Parallel()

	limpa := snapshotTab(&tab{url: "about:blank", titulo: "x"})
	if limpa.URL != "" {
		t.Errorf("about devia zerar, deu %q", limpa.URL)
	}

	if limpa.Titulo != "x" {
		t.Errorf("titulo devia passar, deu %q", limpa.Titulo)
	}

	vazia := snapshotTab(nil)
	if vazia.URL != "" || vazia.Titulo != "" {
		t.Errorf("nil devia dar vazio, deu %+v", vazia)
	}
}

func TestPrenderDestinoMoverAba(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{tabs: []*tab{{}, {}, {}}}

	if got := prenderDestinoMoverAba(aplicativo, -2); got != 0 {
		t.Errorf("negativo devia prender em 0, deu %d", got)
	}

	if got := prenderDestinoMoverAba(aplicativo, 99); got != 3 {
		t.Errorf("estouro devia prender no fim, deu %d", got)
	}

	if got := prenderDestinoMoverAba(aplicativo, 1); got != 1 {
		t.Errorf("meio devia passar, deu %d", got)
	}
}

func TestAbasFixadasComNil(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{tabs: []*tab{{fixa: true}, nil, {fixa: false}}}

	if got := aplicativo.abasFixadas(); got != 1 {
		t.Errorf("so uma fixa, deu %d", got)
	}
}

func TestReancorarAbaAtiva(t *testing.T) {
	t.Parallel()

	alvo := &tab{url: "alvo"}

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{tabs: []*tab{{url: "a"}, alvo, {url: "c"}}, ativa: 0}

	reancorarAbaAtiva(aplicativo, alvo)

	if aplicativo.ativa != 1 {
		t.Errorf("ativa devia ir para 1, deu %d", aplicativo.ativa)
	}

	reancorarAbaAtiva(aplicativo, nil)

	if aplicativo.ativa != 1 {
		t.Error("nil nao devia mover a ativa")
	}
}
