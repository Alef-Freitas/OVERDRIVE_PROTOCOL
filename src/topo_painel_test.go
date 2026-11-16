//go:build windows

package main

import (
	"testing"
	"time"
)

func TestTravouCriacaoAba(t *testing.T) {
	t.Parallel()

	if travouCriacaoAba(nil, time.Second) {
		t.Error("nil nao trava")
	}

	parada := &tab{}
	if travouCriacaoAba(parada, time.Second) {
		t.Error("sem voo nao trava")
	}

	semHora := &tab{criando: true}
	if travouCriacaoAba(semHora, time.Second) {
		t.Error("sem carimbo nao trava")
	}

	fresca := &tab{criando: true, criandoDesde: time.Now()}
	if travouCriacaoAba(fresca, 45*time.Second) {
		t.Error("voo fresco nao trava")
	}

	velha := &tab{criando: true, criandoDesde: time.Now().Add(-time.Hour)}
	if !travouCriacaoAba(velha, 45*time.Second) {
		t.Error("voo de 1h devia travar")
	}
}

func TestPaneAbaPendente(t *testing.T) {
	t.Parallel()

	if paneAbaPendente(nil) {
		t.Error("nil nao e pendente")
	}

	if paneAbaPendente(&tab{criando: true}) {
		t.Error("voo em curso nao e pendente")
	}

	if !paneAbaPendente(&tab{}) {
		t.Error("sem pane e pendente")
	}

	if !paneAbaPendente(&tab{pane: &pane{}}) {
		t.Error("pane nao pronto e pendente")
	}

	if paneAbaPendente(&tab{pane: &pane{ready: true}}) {
		t.Error("pane pronto nao e pendente")
	}
}

func TestVigiarAbaAtivaNaoAge(t *testing.T) {
	t.Parallel()

	var semApp *app
	semApp.vigiarAbaAtiva()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	quieto := &app{}
	quieto.vigiarAbaAtiva()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	semEnv := &app{ativa: 0, tabs: []*tab{{pane: &pane{}}}}
	semEnv.vigiarAbaAtiva()

	if semEnv.tabs[0].criando {
		t.Error("sem env nao devia iniciar voo")
	}
}

func TestNavegarVoltarAvancarGuardas(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{}

	if tratado := aplicativo.navegarVoltarAvancar("id-qualquer"); tratado {
		t.Error("id desconhecido nao e tratado")
	}

	if tratado := aplicativo.navegarVoltarAvancar(idVoltar); !tratado {
		t.Error("sem pagina devia tratar com aviso")
	}

	if tratado := aplicativo.navegarVoltarAvancar(idAvancar); !tratado {
		t.Error("sem pagina devia tratar com aviso")
	}
}

func TestCanGoHistoricoNil(t *testing.T) {
	t.Parallel()

	var vazio *WebView
	if vazio.CanGoBack() {
		t.Error("webview nil nao volta")
	}

	if vazio.CanGoForward() {
		t.Error("webview nil nao avanca")
	}

	vazio.GoBack()
	vazio.GoForward()

	(&WebView{}).GoBack()
	(&WebView{}).GoForward()
}

// TestTituloNaoCobreAudio garante que o titulo termina antes do icone de
// audio mesmo com o X visivel (o X fica a direita do audio e nao pode
// reexpandir a area do titulo por cima dele).
func TestTituloNaoCobreAudio(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so as faixas da aba importam.
	aplicativo := &app{
		tabs:          []*tab{{}},
		tabAudioRects: []rectT{{Left: 250, Top: 50, Right: 272, Bottom: 72}},
		tabCloseRects: []rectT{{Left: 274, Top: 50, Right: 296, Bottom: 72}},
	}
	area := rectT{Left: 8, Top: 44, Right: 308, Bottom: 82}

	var disp d2dMotor

	_, direita := aplicativo.bordasTituloAbaD2D(&disp, aplicativo.tabs[0], area, 0, corTexto)

	quero := int32(250) - aplicativo.sc(2)
	if direita != quero {
		t.Errorf("titulo devia terminar em %d, terminou em %d", quero, direita)
	}

	if direita >= 272 {
		t.Errorf("titulo invadiu o icone de audio: %d", direita)
	}
}

// TestTituloComSoFechar garante que so com o X a area vai ate ele.
func TestTituloComSoFechar(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so as faixas da aba importam.
	aplicativo := &app{
		tabs:          []*tab{{}},
		tabAudioRects: []rectT{{}},
		tabCloseRects: []rectT{{Left: 274, Top: 50, Right: 296, Bottom: 72}},
	}
	area := rectT{Left: 8, Top: 44, Right: 308, Bottom: 82}

	var disp d2dMotor

	_, direita := aplicativo.bordasTituloAbaD2D(&disp, aplicativo.tabs[0], area, 0, corTexto)

	quero := int32(274) - aplicativo.sc(2)
	if direita != quero {
		t.Errorf("titulo devia terminar em %d, terminou em %d", quero, direita)
	}
}
