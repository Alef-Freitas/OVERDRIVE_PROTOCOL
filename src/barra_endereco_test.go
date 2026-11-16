//go:build windows

package main

import (
	"testing"
)

// TestMenuFerramentasTemBarraEndereco garante que os botoes de leitura que
// sairam da barra vivem no menu ferramentas (inclusive nos submenus), cada um
// com icone proprio. Print e historico agora ficam na barra.
func TestMenuFerramentasTemBarraEndereco(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so o menu importa.
	aplicativo := &app{}
	itens := aplicativo.buildMenuFerramentas()

	quero := []string{
		idReader, idCopiarLeitura, "tr", idCookies, idPaywall, idLivreClique,
	}

	tem := map[string]menuItem{}

	var achatar func(sub []menuItem)

	achatar = func(sub []menuItem) {
		for _, item := range sub {
			if item.kind == miItem || item.kind == miCheck {
				tem[item.id] = item
			}

			achatar(item.sub)
		}
	}
	achatar(itens)

	vistos := map[string]string{}

	for _, id := range quero {
		item, ok := tem[id]
		if !ok {
			t.Errorf("ferramentas devia ter %q", id)

			continue
		}

		if item.label == "" {
			t.Errorf("item %q sem rotulo", id)
		}

		icone := iconeDoItem(item)
		if icone == "" || icone == "document" {
			t.Errorf("item %q sem icone proprio", id)
		}

		if dono, repetido := vistos[icone]; repetido {
			t.Errorf("icone %q repetido em %q e %q", icone, dono, id)
		}

		vistos[icone] = id
	}
}

// TestMenuFerramentasSubmenus trava a organizacao: acao direta, submenus
// por alvo com as acoes em ordem, interruptores por ultimo.
func TestMenuFerramentasSubmenus(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so a estrutura importa.
	aplicativo := &app{}
	itens := aplicativo.buildMenuFerramentas()

	queroOrdem := []string{
		idSenhas, txtSubPaginaWeb, txtSubSiteRede,
		idNojs, idSleep, idWebviewUnica, idLivreClique, txtSubDiagnostico,
	}

	verificarOrdemMenuFerramentas(t, itens, queroOrdem)

	queroSubs := map[string][]string{
		txtSubPaginaWeb:   {"tr", idReader, idCopiarLeitura},
		txtSubSiteRede:    {idCookies, idPaywall, idVPN},
		txtSubDiagnostico: {idDev, idLiberarMemoria},
	}

	verificarSubmenusFerramentas(t, itens, queroSubs)
}

// verificarOrdemMenuFerramentas compara as linhas de topo do menu.
func verificarOrdemMenuFerramentas(t *testing.T, itens []menuItem, queroOrdem []string) {
	t.Helper()

	if len(itens) != len(queroOrdem) {
		t.Fatalf("menu com %d linhas, quero %d", len(itens), len(queroOrdem))
	}

	rotulo := func(item menuItem) string {
		if item.kind == miSub {
			return item.label
		}

		return item.id
	}

	for i, quero := range queroOrdem {
		if rotulo(itens[i]) != quero {
			t.Errorf("linha %d = %q, quero %q", i, rotulo(itens[i]), quero)
		}
	}
}

// verificarSubmenusFerramentas compara os itens de cada submenu.
func verificarSubmenusFerramentas(t *testing.T, itens []menuItem, queroSubs map[string][]string) {
	t.Helper()

	for _, item := range itens {
		if item.kind != miSub {
			continue
		}

		quero, ok := queroSubs[item.label]
		if !ok {
			t.Errorf("submenu inesperado %q", item.label)

			continue
		}

		if len(item.sub) != len(quero) {
			t.Errorf("submenu %q com %d itens, quero %d", item.label, len(item.sub), len(quero))

			continue
		}

		for i, id := range quero {
			if item.sub[i].id != id {
				t.Errorf("submenu %q pos %d = %q, quero %q", item.label, i, item.sub[i].id, id)
			}
		}
	}
}

// TestBotoesNoQuadroPinturaDedicada garante que print, historico e ferramentas
// nao saem na passada generica (o preenchimento do quadro os cobriria) e que
// os glifos existem.
func TestBotoesNoQuadroPinturaDedicada(t *testing.T) {
	t.Parallel()

	for _, id := range []string{idFerramentas, idImprimir, idClipboard} {
		if iconesFluent[id] == 0 && glifoNav(id) == "" {
			t.Errorf("%q sem glifo: ficaria invisivel na barra", id)
		}
	}

	//exhaustruct:ignore // App de teste: so o roteamento de pintura importa.
	aplicativo := &app{}

	for _, botao := range []tbutton{
		{id: idFerramentas, rect: rectT{0, 0, 24, 24}},
		{id: idImprimir, rect: rectT{30, 0, 54, 24}},
		{id: idClipboard, rect: rectT{60, 0, 84, 24}},
	} {
		if !aplicativo.pularBotaoBarraD2D(botao) {
			t.Errorf("%q devia pular a pintura generica (pintura dedicada apos o quadro)", botao.id)
		}
	}

	fora := tbutton{id: idDownloads, rect: rectT{0, 0, 36, 36}}
	if aplicativo.pularBotaoBarraD2D(fora) {
		t.Error("botao fora do quadro nao devia pular a pintura generica")
	}
}

// TestBarraEnderecoOrdemBotoes garante o layout novo, da direita para a
// esquerda: uBO, ferramentas, historico e print; cadeado e motor a esquerda.
func TestBarraEnderecoOrdemBotoes(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	aplicativo, medida := barraEnderecoTeste()

	aplicativo.montarBotoesBarra(medida)
	aplicativo.posicionarBotoesLeitura(medida)

	verificarBotoesPresentesBarra(t, aplicativo)
	verificarOrdemBarra(t, aplicativo, medida)
	verificarBotoesForaBarra(t, aplicativo)
	verificarClicaveisBarra(t, aplicativo)
}

// barraEnderecoTeste monta a medida minima da barra de endereco.
func barraEnderecoTeste() (*app, *medidaFaixa) {
	//exhaustruct:ignore // App de teste: so o layout da barra importa.
	aplicativo := &app{}

	var medida medidaFaixa

	medida.rx = 1000
	medida.padY = 44
	medida.bh = 38
	medida.addrFrameLeft = 200
	medida.uboBase = "extensions\\uBO"

	return aplicativo, &medida
}

// verificarBotoesPresentesBarra confere cadeado, motor e os tres botoes novos.
func verificarBotoesPresentesBarra(t *testing.T, aplicativo *app) {
	t.Helper()

	if aplicativo.securityBtn.rect.Width() <= 0 {
		t.Error("cadeado devia continuar na barra")
	}

	if aplicativo.engineBtn.rect.Width() <= 0 {
		t.Error("motor de busca devia continuar na barra")
	}

	for id, botao := range map[string]tbutton{
		idFerramentas: aplicativo.toolsBtn,
		idImprimir:    aplicativo.printBtn,
		idClipboard:   aplicativo.clipBtn,
	} {
		if botao.rect.Width() <= 0 {
			t.Errorf("%q devia estar na barra", id)
		}
	}

	if aplicativo.engineBtn.rect.Left < aplicativo.securityBtn.rect.Right {
		t.Error("motor devia ficar a direita do cadeado")
	}
}

// verificarOrdemBarra confere a ordem fisica dentro do quadro do endereco.
func verificarOrdemBarra(t *testing.T, aplicativo *app, medida *medidaFaixa) {
	t.Helper()

	ordem := []tbutton{aplicativo.printBtn, aplicativo.clipBtn, aplicativo.toolsBtn, aplicativo.uboBtn}
	for i := 1; i < len(ordem); i++ {
		if ordem[i-1].rect.Right > ordem[i].rect.Left {
			t.Errorf("%q devia ficar a esquerda de %q", ordem[i-1].id, ordem[i].id)
		}
	}

	if aplicativo.uboBtn.rect.Right > medida.frameRight {
		t.Error("uBO devia ficar dentro da barra de endereco")
	}

	if aplicativo.printBtn.rect.Left < medida.addrFrameLeft {
		t.Error("print devia ficar a direita do cadeado")
	}
}

// verificarBotoesForaBarra garante que os botoes migrados nao ocupam a barra.
func verificarBotoesForaBarra(t *testing.T, aplicativo *app) {
	t.Helper()

	for id, botao := range map[string]tbutton{
		idReader:        aplicativo.readerBtn,
		idCopiarLeitura: aplicativo.readerCopyBtn,
		idPaywall:       aplicativo.paywallBtn,
		"tr":            aplicativo.trBtn,
		idCookies:       aplicativo.cookiesBtn,
	} {
		if botao.rect.Width() > 0 {
			t.Errorf("botao %q devia sair da barra", id)
		}
	}
}

// verificarClicaveisBarra garante o hit-test dos botoes visiveis e ocultos.
func verificarClicaveisBarra(t *testing.T, aplicativo *app) {
	t.Helper()

	clicaveis := map[string]bool{}

	for _, botao := range aplicativo.buttons {
		clicaveis[botao.id] = botao.rect.Width() > 0
	}

	for _, id := range []string{idFerramentas, idImprimir, idClipboard, aplicativo.uboBtn.id} {
		if !clicaveis[id] {
			t.Errorf("botao %q devia ser clicavel na barra", id)
		}
	}

	for _, id := range []string{idReader, idCopiarLeitura, idPaywall, "tr", idCookies} {
		if clicaveis[id] {
			t.Errorf("botao %q nao devia ser clicavel na barra", id)
		}
	}
}
