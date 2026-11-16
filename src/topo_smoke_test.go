//go:build windows

package main

import (
	"testing"
)

// TestDoActionTopoSemPagina dispara todos os botoes da parte de cima sem
// pagina, sem motor e sem janela: nenhum pode dar panico. Usa o mutex do
// clipboard porque o id clipboard toca nos mesmos globais de UI.
func TestDoActionTopoSemPagina(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	ids := []string{
		idVoltar, idAvancar, idRecarregar, idParar,
		idSalvarPagina, idSalvarCompleto, idSalvarUnico, idCopiarEndereco, idSelecionarTudo,
		idRecarregarFrio, idRecarregarSemLite, "recarregarsemjs",
		idLimparDados, idImprimirDlg, "salvarpdf",
		idUbo, idExtensoes, idSeguranca, idFerramentas, idEngine,
		"tr", idReader, idCopiarLeitura, idCookies, idVPN, idYoutube,
		idSleep, idWebviewUnica, idLivreClique, idNojs,
		idClipboard, idImprimir, "printagora", "print1",
		idSenhas, "senhasauto", "senhas:preencher:x.com",
		"senhas:copiaru:x.com", "senhas:copiars:x.com", "senhas:apagar:x.com",
		idContaDeepseek, "conta0", "conta1", "conta2",
		idDownloads, idLiberarMemoria,
		"zin", "zout", "zreset", "zoomfit", idMenu,
	}

	for _, id := range ids {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("doAction(%q) panico: %v", id, r)
				}
			}()

			//exhaustruct:ignore // App de teste: sem pagina, motor e janela.
			aplicativo := &app{}
			aplicativo.doAction(id)
		}()
	}
}

// TestBotoesJanelaSemHwnd garante que os botoes de janela sao no-op sem panico.
func TestBotoesJanelaSemHwnd(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("botoes de janela panico: %v", r)
			}
		}()

		//exhaustruct:ignore // App de teste: sem janela.
		aplicativo := &app{}
		aplicativo.executarBotaoJanela(winMinimiza)
		aplicativo.executarBotaoJanela(winMaximiza)
		aplicativo.executarBotaoJanela(winFecha)
		aplicativo.toggleMaximizar()
	}()
}

// TestHitTesteBarraVazia garante hit-test vazio sem panico.
func TestHitTesteBarraVazia(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: sem botoes montados.
	aplicativo := &app{}

	if got := aplicativo.hitButton(10, 10); got != "" {
		t.Errorf("barra vazia devia dar vazio, deu %q", got)
	}

	if got := aplicativo.panelButtonAt(10, 10); got != "" {
		t.Errorf("painel vazio devia dar vazio, deu %q", got)
	}
}

// TestCriarJanelasSemProc garante que criar janela sem callback
// registrado falha em vez de matar o processo (no teste os callbacks
// nunca sao registrados; CreateWindowEx com WndProc nulo matava o
// processo sem rastro — constatado headless).
func TestCriarJanelasSemProc(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	//exhaustruct:ignore // App de teste: sem janela.
	aplicativo := &app{}

	if aplicativo.createMenuWindowFor(&menu, "ClasseInexistenteDeTeste", 0) {
		t.Error("menu sem proc devia reprovar")
	}

	if aplicativo.createMenuWindowFor(&menuSub, "ClasseInexistenteDeTeste", 0) {
		t.Error("submenu sem proc devia reprovar")
	}

	if clipCriarJanela() {
		t.Error("clip sem proc devia reprovar")
	}

	if recorteCriarJanela() != 0 {
		t.Error("recorte sem proc devia dar 0")
	}

	if cfgCriarJanela() {
		t.Error("cfg sem proc devia reprovar")
	}

	if aplicativo.criarJanelaFloat("teste", 10, 10) {
		t.Error("float sem proc devia reprovar")
	}

	if aplicativo.criarJanelaAviso() {
		t.Error("aviso sem proc devia reprovar")
	}
}
