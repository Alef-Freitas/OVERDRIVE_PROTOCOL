//go:build windows

package main

import (
	"testing"
	"time"
)

const (
	urlAbaRecuperacao  = "https://aba.com"
	urlVivaRecuperacao = "https://viva.com"
	urlAboutBlankTeste = "about:blank"
)

func TestProcessoDerrubaDocumento(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome    string
		kind    uint32
		derruba bool
	}{
		{"navegador", falhaProcessoNavegador, false},
		{"renderizador", falhaProcessoRenderizador, true},
		{"sem resposta", 2, false},
		{"quadro", falhaProcessoQuadro, true},
		{"utilitario", 4, false},
		{"sandbox", 5, false},
		{"gpu", 6, false},
		{"desconhecido", 99, false},
	}

	for _, caso := range casos {
		if got := processoDerrubaDocumento(caso.kind); got != caso.derruba {
			t.Errorf("%s: esperava %v, veio %v", caso.nome, caso.derruba, got)
		}
	}
}

func TestAbaDoPane(t *testing.T) {
	t.Parallel()

	paneA := &pane{}
	paneB := &pane{}
	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{tabs: []*tab{{pane: paneA}, nil, {pane: paneB}}}

	if got := aplicativo.abaDoPane(paneB); got != aplicativo.tabs[2] {
		t.Error("devia achar a aba oculta do painel")
	}

	if got := aplicativo.abaDoPane(&pane{}); got != nil {
		t.Error("painel de outro dono devia dar nil")
	}

	if got := aplicativo.abaDoPane(nil); got != nil {
		t.Error("painel nil devia dar nil")
	}

	var semApp *app
	if got := semApp.abaDoPane(paneA); got != nil {
		t.Error("app nil devia dar nil")
	}
}

func TestPodeRecarregarAbaFalha(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	zerado := &app{}
	if !zerado.podeRecarregarAbaFalha() {
		t.Error("sem recuperacao registrada devia liberar")
	}

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	recente := &app{ultimaRecuperacaoAba: time.Now()}
	if recente.podeRecarregarAbaFalha() {
		t.Error("recuperacao recente devia segurar")
	}

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	vencido := &app{ultimaRecuperacaoAba: time.Now().Add(-periodoRecuperacaoAba - time.Second)}
	if !vencido.podeRecarregarAbaFalha() {
		t.Error("recuperacao vencida devia liberar")
	}
}

func TestEnderecoPaneFalho(t *testing.T) {
	t.Parallel()

	vivo := &pane{ultimaURL: urlVivaRecuperacao + "/x"}
	if got := enderecoPaneFalho(vivo, &tab{url: urlAbaRecuperacao}); got != urlVivaRecuperacao+"/x" {
		t.Errorf("pane devia ganhar da aba, deu %q", got)
	}

	about := &pane{ultimaURL: urlAboutBlankTeste}
	if got := enderecoPaneFalho(about, &tab{url: urlAbaRecuperacao}); got != urlAbaRecuperacao {
		t.Errorf("about devia cair para a aba, deu %q", got)
	}

	if got := enderecoPaneFalho(&pane{}, &tab{url: urlAboutBlankTeste}); got != "" {
		t.Errorf("sem endereco real devia dar vazio, deu %q", got)
	}
}

func abaFalhaTeste() *tab {
	return &tab{
		url:     "https://antiga.com",
		criando: true,
		pane: &pane{
			ready:        true,
			ultimaURL:    urlVivaRecuperacao,
			tocandoAudio: true,
			mudo:         true,
			frioPendente: true,
			nojsPendente: true,
			sonoDormindo: true,
		},
	}
}

func TestEsquecerPaneFalho(t *testing.T) {
	t.Parallel()

	aba := abaFalhaTeste()

	esquecerPaneFalho(aba, urlVivaRecuperacao)

	if aba.pane.ready || aba.pane.wv != nil {
		t.Error("pane devia ficar solto")
	}

	if aba.criando {
		t.Error("voo devia ser cancelado")
	}

	if aba.url != urlVivaRecuperacao || aba.pane.ultimaURL != urlVivaRecuperacao {
		t.Error("endereco vivo devia ser preservado")
	}
}

func TestEsquecerPaneFalhoZeraPendencias(t *testing.T) {
	t.Parallel()

	aba := abaFalhaTeste()

	esquecerPaneFalho(aba, urlVivaRecuperacao)

	if aba.pane.tocandoAudio || aba.pane.mudo {
		t.Error("audio devia zerar")
	}

	if aba.pane.frioPendente || aba.pane.nojsPendente || aba.pane.sonoDormindo {
		t.Error("pendencias deviam zerar")
	}

	esquecerPaneFalho(nil, urlVivaRecuperacao)
	esquecerPaneFalho(&tab{}, urlVivaRecuperacao)
}
