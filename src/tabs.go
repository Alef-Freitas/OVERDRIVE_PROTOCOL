//go:build windows

package main

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Constantes locais da faixa de abas (limites, esperas e fatores sem outro dono).
const (
	maximoLinksChatVistos     = 64
	esperaLimpezaAposDescarte = 1500 * time.Millisecond
	maximoGuiasFechadas       = 10
	quebraAbasPequenas        = 10
	quebraAbasMedias          = 7
	divisorMetadeFaixa        = 2
	segundosPorMinutoFaixa    = 60
	codigoErroShellExec       = 32
	minimoAbasNavegar         = 2
	digitosMinDia             = 2
)

func (a *app) abaComURL(url string) int {
	url = strings.TrimSpace(url)
	if url == "" {
		return -1
	}

	for indice, aba := range a.tabs {
		if aba == nil {
			continue
		}

		if aba.pane != nil && aba.pane.ultimaURL == url {
			return indice
		}

		if aba.url == url {
			return indice
		}
	}

	return -1
}

func (a *app) podeAbrirAbaDoChat(url string) (bool, int) {
	if i := a.abaComURL(url); i >= 0 {
		return false, i
	}

	agora := time.Now()

	nome := strings.ToLower(strings.TrimSpace(url))
	if t, ok := a.linkChatVistos[nome]; ok && agora.Sub(t) < linkChatMesmaURLJanela {
		return false, -1
	}

	if agora.Sub(a.linkChatJanela) > linkChatMaxJanela {
		a.linkChatJanela = agora
		a.linkChatConta = 0
	}

	limite := linkChatMaxAbas
	if a.linkChatConta >= limite {
		return false, -1
	}

	if a.linkChatVistos == nil {
		a.linkChatVistos = map[string]time.Time{}
	}

	if len(a.linkChatVistos) > maximoLinksChatVistos {
		for k, t := range a.linkChatVistos {
			if agora.Sub(t) > linkChatMaxJanela {
				delete(a.linkChatVistos, k)
			}
		}
	}

	a.linkChatVistos[nome] = agora
	a.linkChatConta++

	return true, -1
}

// guardarEstadoAbaSalva copia URL e titulo do pane para a aba.
func guardarEstadoAbaSalva(aba *tab) {
	if aba.pane.titulo != "" {
		aba.titulo = aba.pane.titulo
	}
}

// paneAbaDescartavel diz se a aba pode ser descartada agora.
func paneAbaDescartavel(aba *tab) bool {
	if aba == nil || aba.pane == nil || !aba.pane.ready || aba.pane.wv == nil {
		return false
	}

	return !aba.pane.tocandoAudio || aba.pane.mudo
}

// salvarAbaAntesDeDescartar copia URL e titulo antes de destruir.
func salvarAbaAntesDeDescartar(aba *tab) {
	if u := aba.pane.wv.Source(); u != "" && !strings.HasPrefix(u, "about:") {
		aba.url = u
	}

	if aba.pane.ultimaURL != "" {
		aba.url = aba.pane.ultimaURL
	}

	guardarEstadoAbaSalva(aba)
}

func (a *app) descartarAbasEsquerdas() {
	total := 0

	for _, aba := range a.tabs {
		if !paneAbaDescartavel(aba) {
			continue
		}

		salvarAbaAntesDeDescartar(aba)
		a.destruirPane(aba.pane)

		total++
	}

	if total > 0 {
		a.pararServicosDeFundo()
		a.agendarLimpezaMemoria(true, esperaLimpezaAposDescarte)
	}
}

func (a *app) acordarAbaPrincipal() {
	if a.left != nil && a.left.frio {
		a.retomarPaneGenerico(a.left)
	}
}

func (a *app) congelarAbaPrincipal(p *pane) {
	if !a.podeCongelarAbaPrincipal(p) {
		return
	}

	p.frioPendente = true

	const jsMidia = `(function(){try{var ms=document.querySelectorAll("video,audio");` +
		`for(var i=0;i<ms.length;i++){var m=ms[i];` +
		`if(!m.paused&&!m.ended&&m.readyState>2)return "playing";}return "idle";}catch(e){return "idle";}})()`
	p.wv.EvalJS(jsMidia, func(err error, res string) {
		a.depois(func() {
			a.concluirCongelamento(p, err, res)
		})
	})
}

// paneCongelavelBasico valida painel e estado global para congelar.
func (a *app) paneCongelavelBasico(p *pane) bool {
	if p == nil || p != a.left || !p.ready || p.wv == nil || p.frio || p.frioPendente {
		return false
	}

	return a.st.EconomiaMax && !a.tudoSuspenso && !a.quitting && !a.ytAberto
}

// paneCongelavelOcioso valida ociosidade e foco para congelar.
func (a *app) paneCongelavelOcioso(p *pane) bool {
	if a.loading || a.urlEditing || a.cursorNoPainel(p) {
		return false
	}

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return false
	}

	if p.ociosoDesde.IsZero() || time.Since(p.ociosoDesde) < 5*time.Minute {
		return false
	}

	u := strings.ToLower(strings.TrimSpace(p.ultimaURL))

	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// podeCongelarAbaPrincipal diz se a aba principal pode ser congelada agora.
func (a *app) podeCongelarAbaPrincipal(p *pane) bool {
	return a.paneCongelavelBasico(p) && a.paneCongelavelOcioso(p)
}

// concluirCongelamento congela a aba se o JS confirmar ociosidade sem midia.
func (a *app) concluirCongelamento(p *pane, err error, res string) {
	if !podeFinalizarCongelamento(a, p, err, res) {
		return
	}

	a.congelarPaneGenerico(p)
}

// consumirSinalCongelamento valida o retorno do JS de midia.
func consumirSinalCongelamento(p *pane, err error, res string) bool {
	if !p.frioPendente {
		return false
	}

	p.frioPendente = false

	if err != nil {
		return false
	}

	if strings.Contains(res, "playing") {
		p.ociosoDesde = time.Now()

		return false
	}

	return true
}

// painelAindaCongelavel revalida o estado global antes de congelar.
func painelAindaCongelavel(a *app, p *pane) bool {
	if p.frio || !a.st.EconomiaMax || a.tudoSuspenso || a.quitting {
		return false
	}

	return !a.loading && !a.urlEditing && !a.cursorNoPainel(p)
}

// podeFinalizarCongelamento revalida tudo antes de congelar de verdade.
func podeFinalizarCongelamento(a *app, p *pane, err error, res string) bool {
	if !consumirSinalCongelamento(p, err, res) {
		return false
	}

	if !painelAindaCongelavel(a, p) {
		return false
	}

	return !p.ociosoDesde.IsZero() && time.Since(p.ociosoDesde) >= 5*time.Minute
}

func (a *app) buildMenuContextoAba(indice int) []menuItem {
	itens := []menuItem{
		{kind: miItem, label: txtNovaGuia, hint: txtHintCtrlT, id: idAbaNova},
		{kind: miItem, label: txtReabrirGuia, hint: txtHintCtrlShiftT, id: idAbaReabrir},
	}

	if indice >= 0 && indice < len(a.tabs) {
		fixLabel := txtFixarGuia
		if a.tabs[indice].fixa {
			fixLabel = txtDesafixarGuia
		}

		itens = append(itens,
			menuItem{kind: miSep},
			menuItem{kind: miItem, label: txtAtivarGuia, id: idAbaAtivar},
			menuItem{kind: miItem, label: fixLabel, id: idAbaFixar},
			menuItem{kind: miItem, label: txtDuplicarGuia, id: idAbaDuplicar},
			menuItem{kind: miItem, label: txtRecarregarGuia, hint: txtHintCtrlR, id: "aba:recarregar"},
			menuItem{kind: miItem, label: txtCopiarEndereco, id: "aba:copiar"},
			menuItem{kind: miSep},
			menuItem{kind: miItem, label: txtFecharGuia, hint: txtHintCtrlW, id: idAbaFechar},
		)
		if len(a.tabs) > 1 {
			itens = append(itens, menuItem{kind: miItem, label: txtFecharOutras, id: idAbaFecharOutras})
		}

		if indice+1 < len(a.tabs) {
			itens = append(itens, menuItem{kind: miItem, label: txtFecharDireita, id: idAbaFecharDireita})
		}
	}

	return itens
}

func (a *app) abrirMenuOverflowAbas(x, y int32) {
	menu.items = a.buildMenuOverflowAbas()
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	a.abrirMenuNoPonto(x, y)
}

func (a *app) buildMenuOverflowAbas() []menuItem {
	itens := []menuItem{
		{kind: miItem, label: txtNovaGuia, hint: txtHintCtrlT, id: idAbaNova},
		{kind: miSep},
	}

	for indice, t := range a.tabs {
		if t == nil {
			continue
		}

		rotulo := a.tituloDaAba(t)
		if indice == a.ativa {
			rotulo = "● " + rotulo
		}

		itens = append(itens, menuItem{kind: miItem, label: rotulo, id: "irtab:" + strconv.Itoa(indice)})
	}

	return itens
}

func (a *app) abrirMenuContextoAba(i int, x, y int32) {
	a.contextAba = i
	a.menuOrigem = ""
	menu.items = a.buildMenuContextoAba(i)
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	a.abrirMenuNoPonto(x, y)
}

func (a *app) novaAba(url string, ativar bool) {
	defer a.limpezaAposAcao("nova-aba")

	t := &tab{url: url, pane: &pane{zoom: a.st.Zoom}}
	if !ativar {
		t.inativoDesde = time.Now()
	}

	a.tabs = append(a.tabs, t)

	i := len(a.tabs) - 1
	if ativar {
		a.ativarAba(i)
	} else {
		a.layoutFaixa()
		a.invalidateFaixa()
	}
}

func (a *app) indiceDaAba(t *tab) int {
	for i, x := range a.tabs {
		if x == t {
			return i
		}
	}

	return -1
}

// estacionarAbaAtiva esconde a aba atual antes da troca.
func (a *app) estacionarAbaAtiva() {
	if a.ativa < 0 || a.ativa >= len(a.tabs) {
		return
	}

	if aba := a.tabs[a.ativa]; aba != nil && aba.pane != nil && aba.pane.ready {
		if aba.pane.ultimaURL != "" {
			aba.url = aba.pane.ultimaURL
		}

		aba.titulo = aba.pane.titulo
		setPaneVisible(aba.pane, false)
		suspenderPane(aba.pane)
		aba.inativoDesde = time.Now()

		a.pararServicosDeFundo()
	}
}

// prepararAbaAlvo troca o ponteiro esquerdo para a aba destino.
func (a *app) prepararAbaAlvo(indice int) *tab {
	aba := a.abaValida(indice)
	if aba == nil {
		return nil
	}

	a.ativa = indice

	aba.inativoDesde = time.Time{}
	if aba.pane == nil {
		aba.pane = &pane{zoom: a.st.Zoom}
	}

	a.left = aba.pane

	a.loading = false
	a.loadPhase = 0
	a.invalidateBar()

	return aba
}

// abaValida devolve a aba do indice ou nil se ausente.
func (a *app) abaValida(indice int) *tab {
	if indice < 0 || indice >= len(a.tabs) {
		return nil
	}

	return a.tabs[indice]
}

func (a *app) ativarAba(indice int) {
	aba := a.abaValida(indice)
	if aba == nil {
		return
	}

	if indice == a.ativa {
		if p := aba.pane; p != nil && p.ready && p.wv != nil {
			// Clicar na aba ja ativa precisa descongelar/reexibir o painel:
			// antes o retorno cedo deixava a pagina presa em segundo plano.
			a.mostrarPaneAba(aba)
			a.invalidateBar()

			return
		}
	}

	if a.extFloatOn {
		a.fecharPainelFlutuante()
	}

	a.estacionarAbaAtiva()
	aba = a.prepararAbaAlvo(indice)

	if aba == nil {
		return
	}

	a.sincronizarSonoPane(aba.pane)
	a.sincronizarForceDark(aba.pane)
	a.sincronizarLivreClique(aba.pane)
	a.mostrarPaneAba(aba)

	if a.st.WebviewUnica && aba.pane.ready {
		a.descartarAbasInativas(0)
	}

	a.layout()
	a.invalidateFaixa()
	a.persist()
}

// sincronizarSonoPane aplica o modo de sono configurado ao painel.
func (a *app) sincronizarSonoPane(p *pane) {
	if p == nil || !p.ready {
		return
	}

	if a.st.SleepLite {
		a.ativarSonoLiteNoPane(p)
	} else {
		a.desativarSonoLiteNoPane(p)
	}
}

// carregarAbaSePendente agenda o carregamento da aba ainda nao pronta.
func (a *app) carregarAbaSePendente(aba *tab) {
	if aba == nil || aba.pane == nil || aba.criando {
		return
	}

	a.carregarAba(aba)
}

// mostrarPaneAba carrega ou exibe o painel da aba ativada.
func (a *app) mostrarPaneAba(aba *tab) {
	if aba == nil || aba.pane == nil || !aba.pane.ready || aba.pane.wv == nil {
		a.carregarAbaSePendente(aba)

		return
	}

	if aba.pane.frio {
		a.retomarPaneGenerico(aba.pane)
	}

	aba.pane.ociosoDesde = time.Time{}
	if aba.pane.descarregado {
		a.recarregarPaneDescarregado(aba.pane)
	}

	aba.pane.wv.Resume()
	a.apenasEstaAbaVisivel(aba)
	setPaneBounds(aba.pane)
	setPaneVisible(aba.pane, true)
	aba.pane.wv.Focus()
	a.aplicarZoomNativo(aba.pane, a.paneZoom(aba.pane))

	if aba.pane.ultimaURL != "" {
		a.setAddress(aba.pane.ultimaURL)
	}

	if aba.pane.titulo != "" {
		a.setTitle(aba.pane.titulo)
	} else {
		a.setTitle(appTitle)
	}

	a.aplicarRegrasDominio()
	a.avaliarLeituraUsavel(aba.pane, aba.pane.ultimaURL)
}

func (a *app) apenasEstaAbaVisivel(alvo *tab) {
	for _, aba := range a.tabs {
		if aba == nil || aba == alvo || aba.pane == nil || !aba.pane.ready {
			continue
		}

		if aba.pane.ultimaURL != "" {
			aba.url = aba.pane.ultimaURL
		}

		aba.titulo = aba.pane.titulo
		setPaneVisible(aba.pane, false)
		suspenderPane(aba.pane)

		if aba.inativoDesde.IsZero() {
			aba.inativoDesde = time.Now()
		}
	}
}

// abaInativaDescartavel diz se a aba inativa pode ser descartada.
func abaInativaDescartavel(a *app, i int, aba *tab, limite time.Duration) bool {
	if aba == nil || i == a.ativa || aba.pane == nil || !aba.pane.ready || aba.pane.wv == nil {
		return false
	}

	if aba.pane.tocandoAudio && !aba.pane.mudo {
		return false
	}

	return !aba.inativoDesde.IsZero() && time.Since(aba.inativoDesde) >= limite
}

// desmontarPaneAba zera o pane da aba descartada.
func desmontarPaneAba(aba *tab) {
	aba.pane.wv.Close()
	aba.pane.wv = nil
	aba.pane.ready = false
	aba.pane.boundsSet = false
	aba.pane.visivel = false
	aba.pane.visivelSet = false
	aba.pane.frio = false
	aba.pane.frioManual = false
	aba.pane.ociosoDesde = time.Time{}

	aba.criando = false
	aba.criandoDesde = time.Time{}
	aba.criandoGen++
}

func (a *app) descartarAbasInativas(limite time.Duration) {
	if len(a.tabs) <= 1 {
		return
	}

	descartou := false

	for i, aba := range a.tabs {
		if !abaInativaDescartavel(a, i, aba, limite) {
			continue
		}

		if aba.pane.ultimaURL != "" {
			aba.url = aba.pane.ultimaURL
		}

		guardarEstadoAbaSalva(aba)
		desmontarPaneAba(aba)

		descartou = true
	}

	if descartou {
		a.pararServicosDeFundo()
		a.agendarLimpezaMemoria(true, esperaLimpezaAposDescarte)
	}
}

func (a *app) toggleWebviewUnica() {
	a.st.WebviewUnica = !a.st.WebviewUnica
	a.persist()

	if a.st.WebviewUnica {
		a.descartarAbasInativas(0)
		a.status = txtVirtLigada
	} else {
		a.status = txtVirtDesligada
	}

	a.layout()
	a.invalidateBar()
}

// travouCriacaoAba diz se a criacao da aba passou do limite sem concluir.
func travouCriacaoAba(aba *tab, limite time.Duration) bool {
	if aba == nil || !aba.criando || aba.criandoDesde.IsZero() {
		return false
	}

	return time.Since(aba.criandoDesde) >= limite
}

// paneAbaPendente diz se a aba precisa de carga sem voo em curso.
func paneAbaPendente(aba *tab) bool {
	if aba == nil || aba.criando {
		return false
	}

	return aba.pane == nil || !aba.pane.ready
}

// vigiarAbaAtiva recupera a aba ativa presa: se a criacao estourou,
// retemta do zero; se o pane precisa de carga sem voo em curso, carrega.
// Cobre o furo entre falha de CreateController e a proxima troca de aba,
// que na virtualizacao vira "url nao carrega, loading infinito".
func (a *app) vigiarAbaAtiva() {
	if a == nil || a.quitting || a.env == nil || a.ativa < 0 || a.ativa >= len(a.tabs) {
		return
	}

	aba := a.tabs[a.ativa]
	if aba == nil {
		return
	}

	const limiteCriacaoAbaVigia = 45 * time.Second

	if travouCriacaoAba(aba, limiteCriacaoAbaVigia) {
		aba.criando = false
		aba.criandoDesde = time.Time{}
		a.status = txtAbaTentandoDeNovo
		a.carregarAba(aba)
		a.invalidateBar()

		return
	}

	if paneAbaPendente(aba) && a.podeRecarregarAbaFalha() {
		a.carregarAba(aba)
	}
}

func (a *app) retentarAbaTravada() {
	if a.env == nil || a.ativa < 0 || a.ativa >= len(a.tabs) {
		return
	}

	aba := a.tabs[a.ativa]
	if aba == nil || aba.pane == nil || aba.pane.ready || !aba.criando {
		return
	}

	if aba.criandoDesde.IsZero() || time.Since(aba.criandoDesde) < 20*time.Second {
		return
	}

	aba.criando = false
	aba.criandoDesde = time.Time{}
	a.carregarAba(aba)
}

// zerarEstadoPaneAba limpa flags do pane recem-criado.
func zerarEstadoPaneAba(p *pane, url string) {
	p.boundsSet = false
	p.visivelSet = false
	p.ready = true
	p.ultimaURL = url
	p.sonoDormindo = false
	p.nojsPendente = false
	p.nojsAtivo = false
	p.nojsCarregando = false
	p.readerAtivo = false
	p.readerCarregando = false
	p.readerPronto = false
	p.readerURL = ""
	p.descarregado = false
	p.urlDescarga = ""
	p.scrollDescarga = 0
	p.tocandoAudio = false
	p.mudo = false
}

// acolherPaneNovo instala o WebView novo no pane da aba.
func (a *app) acolherPaneNovo(aba *tab, p *pane, w *WebView, url string) {
	p.wv = w
	aba.criando = false
	aba.criandoDesde = time.Time{}

	zerarEstadoPaneAba(p, url)
	a.configureWebView(p, false)

	if len(a.st.Extensions) > 0 {
		a.loadExtensionsEm(w, true)
	}

	a.layout()
}

// exibirPaneNovo mostra o pane se for a aba ativa.
func (a *app) exibirPaneNovo(aba *tab, p *pane) {
	if a.ativa != a.indiceDaAba(aba) {
		return
	}

	a.left = p
	a.apenasEstaAbaVisivel(aba)
	setPaneBounds(p)
	setPaneVisible(p, true)
	p.wv.Focus()
	a.aplicarZoomNativo(p, a.paneZoom(p))
}

// navegarPaneNovo navega para a URL ou pagina inicial.
func navegarPaneNovo(w *WebView, url string) {
	if url != "" && !strings.HasPrefix(url, "about:") {
		w.Navigate(url)
	} else {
		// w.NavigateToString(startPageHTML)
		w.Navigate("about:blank")
	}
}

// tratarPaneNovoAba valida e instala o WebView criado para a aba.
func (a *app) tratarPaneNovoAba(aba *tab, gen int, concluir func(), w *WebView) {
	if gen != aba.criandoGen {
		w.Close()

		return
	}

	if a.indiceDaAba(aba) < 0 {
		concluir()
		w.Close()

		return
	}

	p := aba.pane
	if p == nil {
		p = &pane{zoom: a.st.Zoom}
		aba.pane = p
	}

	if p.wv != nil && p.wv != w {
		// WebView velho (ex.: recarga de extensoes deixou o pane sem ready):
		// o novo e valido, entao o antigo sai em vez de descartar a criacao.
		p.wv.Close()
		p.wv = nil
	}

	url := aba.url
	a.acolherPaneNovo(aba, p, w, url)
	a.exibirPaneNovo(aba, p)
	navegarPaneNovo(w, url)
	espPosicionarWebView(a)

	if a.st.WebviewUnica && a.ativa == a.indiceDaAba(aba) {
		a.descartarAbasInativas(0)
	}

	a.invalidateFaixa()
}

func (a *app) carregarAba(aba *tab) {
	if a.env == nil {
		a.status = txtMotorAbrindo
		a.invalidateBar()

		return
	}

	if aba.criando || (aba.pane != nil && aba.pane.ready) {
		return
	}

	aba.criando = true
	aba.criandoDesde = time.Now()
	aba.criandoGen++
	gen := aba.criandoGen
	concluir := func() {
		if aba.criandoGen == gen {
			aba.criando = false
			aba.criandoDesde = time.Time{}
		}
	}

	err := a.env.CreateController(a.hwnd, func(w *WebView, err error) {
		if err != nil {
			concluir()
			a.dizerErrof("%s", txtFalhaCriarAba)

			return
		}

		a.tratarPaneNovoAba(aba, gen, concluir, w)
	})
	if err != nil {
		concluir()
	}
}

func snapshotTab(aba *tab) tabState {
	if aba == nil {
		return tabState{}
	}

	url, titulo := aba.url, aba.titulo
	if aba.pane != nil {
		if aba.pane.ready && aba.pane.wv != nil {
			if atual := aba.pane.wv.Source(); atual != "" {
				url = atual
			}
		}

		if aba.pane.titulo != "" {
			titulo = aba.pane.titulo
		}
	}

	if strings.HasPrefix(url, "about:") {
		url = ""
	}

	return tabState{URL: url, Titulo: titulo}
}

func (a *app) guardarGuiaFechada(t *tab) {
	guia := snapshotTab(t)

	guia.Indice = a.indiceDaAba(t)
	if t != nil {
		guia.Fixa = t.fixa
	}

	a.guiasFechadas = append(a.guiasFechadas, guia)
	if len(a.guiasFechadas) > maximoGuiasFechadas {
		a.guiasFechadas = append([]tabState(nil), a.guiasFechadas[len(a.guiasFechadas)-10:]...)
	}

	a.st.GuiasFechadas = append([]tabState(nil), a.guiasFechadas...)
	a.persist()
}

func (a *app) reabrirGuiaFechada() {
	if len(a.guiasFechadas) == 0 {
		a.dizerf(txtSemGuiaFechada)

		return
	}

	i := len(a.guiasFechadas) - 1
	guia := a.guiasFechadas[i]
	a.guiasFechadas = a.guiasFechadas[:i]
	a.st.GuiasFechadas = append([]tabState(nil), a.guiasFechadas...)

	novaGuia := &tab{url: guia.URL, titulo: guia.Titulo, fixa: guia.Fixa, pane: &pane{zoom: a.st.Zoom}}

	pos := guia.Indice
	if pos < 0 || pos > len(a.tabs) {
		pos = len(a.tabs)
	}

	a.tabs = slices.Insert(a.tabs, pos, novaGuia)
	a.ativarAba(pos)
	a.persist()
}

func (a *app) duplicarGuia(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}

	guia := snapshotTab(a.tabs[i])
	a.novaAba(guia.URL, true)
	a.tabs[a.ativa].titulo = guia.Titulo
	a.persist()
}

func (a *app) fecharOutrasGuias(keep int) {
	if keep < 0 || keep >= len(a.tabs) {
		return
	}

	for indice, v := range slices.Backward(a.tabs) {
		if indice == keep || v == nil || v.fixa {
			continue
		}

		a.fecharAba(indice)

		if indice < keep {
			keep--
		}
	}

	if keep >= 0 && keep < len(a.tabs) {
		a.ativarAba(keep)
	}
}

func (a *app) fecharGuiasADireita(i int) {
	if i < 0 || i >= len(a.tabs)-1 {
		return
	}

	for n := len(a.tabs) - 1; n > i; n-- {
		if n < len(a.tabs) && a.tabs[n] != nil && !a.tabs[n].fixa {
			a.fecharAba(n)
		}
	}
}

// zerarPaneUltimaAba limpa o pane quando sobra so uma aba.
func (a *app) zerarPaneUltimaAba(p *pane) {
	a.limparServiceWorkerDoSite(p.ultimaURL)
	p.ultimaURL = ""
	p.titulo = ""
	p.frioPendente = false
	p.frio = false
	p.frioManual = false
	p.frioDesde = time.Time{}
	p.ociosoDesde = time.Time{}
	p.descarregado = false
	tinhaNojs := p.nojsAtivo
	p.readerAtivo = false
	p.readerCarregando = false
	p.readerPronto = false
	p.readerURL = ""
	p.nojsAtivo = false
	p.nojsPendente = false
	p.nojsCarregando = false
	p.sonoDormindo = false
	p.tocandoAudio = false

	p.mudo = false
	if p.ready && p.wv != nil {
		p.wv.SetFrozen(false)
		p.wv.SetMuted(false)

		if tinhaNojs {
			if params, err := cdpScriptDesligado(false); err == nil {
				p.wv.CallDevTools("Emulation.setScriptExecutionDisabled", params)
			}
		}
	}
}

// reabrirGuiaUnica reseta a unica aba em vez de fecha-la.
func (a *app) reabrirGuiaUnica() {
	a.tabs[0].url = ""
	a.tabs[0].titulo = ""

	if p := a.tabs[0].pane; p != nil {
		a.zerarPaneUltimaAba(p)

		if p.ready && p.wv != nil {
			p.wv.NavigateToString(startPageHTML)
		}
	}

	a.loading = false
	a.layoutFaixa()
	a.invalidateFaixa()
	a.setTitle("")
	a.syncAddressBar()
	a.pararServicosDeFundo()
	a.persist()
	a.agendarLimpezaMemoria(true, esperaLimpezaAposDescarte)
}

// urlDescarteAba devolve a URL para limpar o service worker.
func urlDescarteAba(t *tab) string {
	if t.pane != nil && t.pane.ultimaURL != "" {
		return t.pane.ultimaURL
	}

	return t.url
}

// ativarAposFechar escolhe a proxima aba ativa apos remover.
func (a *app) ativarAposFechar(_ int, eraAtiva bool, novo int) {
	a.ativa = -1
	if eraAtiva {
		a.ativarAba(novo)
	} else {
		a.ativa = novo
		a.layoutFaixa()
		a.invalidateFaixa()
	}
}

func (a *app) fecharAba(indice int) {
	aba := a.abaValida(indice)
	if aba == nil {
		return
	}
	defer a.limpezaAposAcao("fechar-aba")

	if aba.fixa {
		a.dizerf(txtGuiaFixadaMsg)

		return
	}

	a.guardarGuiaFechada(aba)

	if len(a.tabs) == 1 {
		a.reabrirGuiaUnica()

		return
	}

	urlDoSite := urlDescarteAba(aba)

	if aba.pane != nil {
		a.destruirPane(aba.pane)
	}

	a.tabs = append(a.tabs[:indice], a.tabs[indice+1:]...)

	novo := a.ativa
	if indice < a.ativa {
		novo = a.ativa - 1
	} else if indice == a.ativa && novo >= len(a.tabs) {
		novo = len(a.tabs) - 1
	}

	a.ativarAposFechar(indice, indice == a.ativa, novo)

	if urlDoSite != "" {
		a.limparServiceWorkerDoSite(urlDoSite)
	}

	a.pararServicosDeFundo()
	a.persist()
	a.agendarLimpezaMemoria(true, esperaLimpezaAposDescarte)
}

func (a *app) proximaAba(passo int) {
	n := len(a.tabs)
	if n < minimoAbasNavegar {
		return
	}

	a.ativarAba(((a.ativa+passo)%n + n) % n)
}

// urlBaseAba extrai o host para o titulo da guia.
func urlBaseAba(url string) string {
	if url == "" || strings.HasPrefix(url, "about:") {
		return txtNovaGuia
	}

	if i := strings.Index(url, "://"); i > 0 {
		url = url[i+3:]
	}

	if i := strings.IndexAny(url, "/?#"); i > 0 {
		url = url[:i]
	}

	return url
}

func (a *app) tituloDaAba(aba *tab) string {
	if aba == nil {
		return txtNovaGuia
	}

	if aba.pane != nil && aba.pane.titulo != "" {
		return aba.pane.titulo
	}

	if aba.titulo != "" {
		return aba.titulo
	}

	u := aba.url
	if aba.pane != nil && aba.pane.ultimaURL != "" {
		u = aba.pane.ultimaURL
	}

	return urlBaseAba(u)
}

//nolint:gochecknoglobals // Nomes de dias/meses: tabelas fixas, somente leitura.
var (
	diasSemanaRelogio = [...]string{"dom", diaSeg, "ter", "qua", "qui", "sex", "sáb"}
	mesesRelogio      = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
)

func textoRelogio(instante time.Time) string {
	dia := strconv.Itoa(instante.Day())
	if len(dia) < digitosMinDia {
		dia = "0" + dia
	}

	return instante.Format("15:04") + txtSeparador + diasSemanaRelogio[instante.Weekday()] + " " + dia + " " +
		mesesRelogio[instante.Month()-1] + " " + strconv.Itoa(instante.Year())
}

func (a *app) atualizarRelogio() bool {
	if a == nil {
		return false
	}

	agora := time.Now()

	minuto := agora.Unix() / segundosPorMinutoFaixa
	if minuto == a.relogioMin && a.relogioTxt != "" {
		return false
	}

	a.relogioMin = minuto
	if txt := textoRelogio(agora); txt != a.relogioTxt {
		a.relogioTxt = txt

		return true
	}

	return false
}

func (a *app) larguraRelogio() int32 {
	txt := a.relogioTxt
	if txt == "" {
		txt = textoRelogio(time.Now())
	}

	larg := a.sc(190)
	if a.hwnd != 0 {
		d := &d2dMain
		if w := d.larguraTexto(txt, a.fontUI, 0); w > 0 {
			larg = w + a.sc(28)
		}
	}

	if larg < a.sc(150) {
		larg = a.sc(150)
	}

	return larg
}

// prepararFatiasFaixa garante capacidade das fatias da faixa.
func (a *app) prepararFatiasFaixa(total int) {
	if cap(a.tabRects) < total {
		a.tabRects = make([]rectT, total)
	} else {
		a.tabRects = a.tabRects[:total]
	}

	if cap(a.tabCloseRects) < total {
		a.tabCloseRects = make([]rectT, total)
	} else {
		a.tabCloseRects = a.tabCloseRects[:total]
	}

	if cap(a.tabAudioRects) < total {
		a.tabAudioRects = make([]rectT, total)
	} else {
		a.tabAudioRects = a.tabAudioRects[:total]
	}
}

// limiteFaixaAbas calcula o limite horizontal das abas.
func (a *app) limiteFaixaAbas(areaFaixa rectT, relW int32) int32 {
	lim := areaFaixa.Right - a.sc(3*46) - relW - a.sc(6)
	if a.st.RightVisible {
		if x := a.splitX - a.rail/divisorMetadeFaixa; x > 0 && x < lim {
			lim = x
		}
	}

	if lim <= 0 {
		lim = areaFaixa.Right
	}

	return lim
}

// larguraBaseFaixa calcula a largura inicial pelo espaco livre.
func (a *app) larguraBaseFaixa(areaFaixa rectT, relW int32, lim int32, total int) int32 {
	livre := lim - a.sc(4)
	if limBotoes := areaFaixa.Right - a.sc(3*46) - relW - a.sc(6); limBotoes < livre {
		livre = limBotoes - a.sc(9)
	}

	var largura int32
	if total > 0 {
		largura = livre / int32(total+1)
	}

	if largura > a.sc(240) {
		largura = a.sc(240)
	}

	return largura
}

// minimaAbaFaixa devolve a largura minima pelo numero de abas.
func (a *app) minimaAbaFaixa(total int) int32 {
	if total >= quebraAbasPequenas {
		return a.sc(46)
	}

	if total >= quebraAbasMedias {
		return a.sc(58)
	}

	return a.sc(74)
}

// prenderLarguraFaixa limita a largura ao teto da janela.
func (a *app) prenderLarguraFaixa(areaFaixa rectT, relW int32, largura int32, total int) int32 {
	maxAba := areaFaixa.Right - a.sc(3*46) - relW - a.sc(12)
	if total > 0 && largura > maxAba/int32(total) {
		largura = maxAba / int32(total)
	}

	if total > 0 {
		espaco := areaFaixa.Right - a.sc(3*46) - relW - a.sc(6) - a.sc(8) - a.sc(36) - int32(total)*a.sc(1)
		if maxW := espaco / int32(total); maxW > 0 && largura > maxW {
			largura = maxW
		}
	}

	if largura < a.sc(24) {
		return a.sc(24)
	}

	return largura
}

// larguraAbaFaixa calcula a largura de cada aba.
func (a *app) larguraAbaFaixa(areaFaixa rectT, relW int32, lim int32, total int) int32 {
	largura := a.larguraBaseFaixa(areaFaixa, relW, lim, total)

	if minLargura := a.minimaAbaFaixa(total); largura < minLargura {
		largura = minLargura
	}

	return a.prenderLarguraFaixa(areaFaixa, relW, largura, total)
}

// pintarBotoesJanelaFaixa posiciona os botoes da janela na faixa.
func (a *app) pintarBotoesJanelaFaixa(areaFaixa rectT, relW int32) (int32, int32) {
	topo := a.sc(6)
	base := a.tabH - a.sc(4)
	larguraBotao := a.sc(46)
	posX := areaFaixa.Right
	a.winClose = rectT{posX - larguraBotao, 0, posX, a.tabH}
	a.winMax = rectT{posX - 2*larguraBotao, 0, posX - larguraBotao, a.tabH}
	a.winMin = rectT{posX - 3*larguraBotao, 0, posX - 2*larguraBotao, a.tabH}

	a.winClock = rectT{}
	if relW > 0 {
		a.winClock = rectT{posX - 3*larguraBotao - relW, 0, posX - 3*larguraBotao, a.tabH}
	}

	return topo, base
}

// desenharAbaFaixa desenha uma aba na posicao x e devolve o proximo x.
func (a *app) desenharAbaFaixa(indice int, x, topo, base, largura int32) int32 {
	a.tabRects[indice] = rectT{}
	a.tabAudioRects[indice] = rectT{}
	a.tabCloseRects[indice] = rectT{}
	aba := a.tabs[indice]

	larguraAba := largura
	if aba != nil && aba.fixa {
		larguraAba = a.sc(46)
	}

	a.tabRects[indice] = rectT{x, topo, x + larguraAba, base}
	mudaAudio := aba != nil && aba.pane != nil && (aba.pane.tocandoAudio || aba.pane.mudo)

	limite := x + larguraAba - a.sc(5)
	if aba != nil && !aba.fixa {
		cw := a.sc(22)
		a.tabCloseRects[indice] = rectT{
			limite - cw, (topo+base)/2 - cw/2,
			limite, (topo+base)/2 + cw/2,
		}
		limite -= cw + a.sc(2)
	}

	if mudaAudio {
		aw := a.sc(20)
		a.tabAudioRects[indice] = rectT{
			limite - aw, (topo+base)/2 - aw/2,
			limite, (topo+base)/2 + aw/2,
		}
	}

	return x + larguraAba + a.sc(1)
}

// marcarEstouroFaixa conta abas fora do limite visivel.
func (a *app) marcarEstouroFaixa(areaFaixa rectT, relW, topo, base, x int32, total int) {
	a.tabNewRect = rectT{x + a.sc(4), topo + a.sc(3), x + a.sc(34), base - a.sc(3)}

	a.tabOverflowRect = rectT{}

	a.tabOverflowN = 0
	if limite := areaFaixa.Right - a.sc(3*46) - relW - a.sc(6); limite > 0 && x > limite && total > 0 {
		for i := range total {
			if a.tabRects[i].Right > limite {
				a.tabOverflowN++
			}
		}

		if a.tabOverflowN > 0 {
			w := a.sc(30)
			a.tabOverflowRect = rectT{limite - w, topo, limite, base}
		}
	}
}

// relogioFaixaLimitado devolve a largura do relogio sem estourar.
func (a *app) relogioFaixaLimitado(areaFaixa rectT) int32 {
	relW := a.larguraRelogio()
	if disp := areaFaixa.Right - a.sc(3*46) - a.sc(60); relW > disp {
		relW = disp
	}

	if relW < 0 {
		return 0
	}

	return relW
}

func (a *app) layoutFaixa() {
	a.atualizarRelogio()

	total := len(a.tabs)
	a.prepararFatiasFaixa(total)

	if total == 0 || a.tabH <= 0 {
		a.tabNewRect = rectT{}

		return
	}

	areaFaixa := a.clientRect()

	relW := a.relogioFaixaLimitado(areaFaixa)
	lim := a.limiteFaixaAbas(areaFaixa, relW)
	largura := a.larguraAbaFaixa(areaFaixa, relW, lim, total)
	topo, base := a.pintarBotoesJanelaFaixa(areaFaixa, relW)

	x := a.sc(8)
	for i := range total {
		x = a.desenharAbaFaixa(i, x, topo, base, largura)
	}

	a.marcarEstouroFaixa(areaFaixa, relW, topo, base, x, total)
}

func (a *app) alternarMudoAba(i int) {
	if i < 0 || i >= len(a.tabs) || a.tabs[i].pane == nil {
		return
	}

	p := a.tabs[i].pane

	p.mudo = !p.mudo
	if p.wv != nil {
		p.wv.SetMuted(p.mudo)
	}

	if p.mudo {
		a.status = txtGuiaMuda
	} else {
		a.status = txtGuiaSom
	}

	a.layoutFaixa()
	a.invalidateFaixa()
	a.invalidateBar()
}

func (a *app) abasFixadas() int {
	total := 0

	for _, t := range a.tabs {
		if t != nil && t.fixa {
			total++
		}
	}

	return total
}

func (a *app) indiceDropAba(x int32) int {
	for i, r := range a.tabRects {
		if x < (r.Left+r.Right)/2 {
			return i
		}
	}

	return len(a.tabs)
}

// removerAbaDaOrdem tira a aba da posicao de origem.
func removerAbaDaOrdem(a *app, de, para int) (*tab, int) {
	aba := a.tabs[de]

	a.tabs = append(a.tabs[:de], a.tabs[de+1:]...)
	if para > de {
		para--
	}

	return aba, para
}

// prenderDestinoMoverAba limita o destino ao intervalo valido.
func prenderDestinoMoverAba(aplicativo *app, para int) int {
	if para < 0 {
		return 0
	}

	if para > len(aplicativo.tabs) {
		return len(aplicativo.tabs)
	}

	return para
}

// reancorarAbaAtiva redescobre o indice da aba ativa apos mover.
func reancorarAbaAtiva(aplicativo *app, ativaTab *tab) {
	if ativaTab == nil {
		return
	}

	for i, x := range aplicativo.tabs {
		if x == ativaTab {
			aplicativo.ativa = i

			break
		}
	}
}

func (a *app) moverAba(origem, para int) {
	total := len(a.tabs)
	if origem < 0 || origem >= total || para < 0 || para > total || para == origem || para == origem+1 {
		return
	}

	var ativaTab *tab
	if a.ativa >= 0 && a.ativa < total {
		ativaTab = a.tabs[a.ativa]
	}

	aba, destino := removerAbaDaOrdem(a, origem, para)
	destino = prenderDestinoMoverAba(a, destino)

	a.tabs = slices.Insert(a.tabs, destino, aba)
	reancorarAbaAtiva(a, ativaTab)

	a.layoutFaixa()
	a.invalidateFaixa()
}

func (a *app) fixarAba(indice int) {
	aba := a.abaValida(indice)
	if aba == nil {
		return
	}

	fixasAntes := a.abasFixadas()
	aba.fixa = !aba.fixa

	a.moverAba(indice, fixasAntes)

	a.status = txtGuiaFixada
	if !aba.fixa {
		a.status = txtGuiaDesafixada
	}

	a.invalidateBar()
}

func (a *app) invalidateFaixa() {
	if a.hwnd != 0 {
		r := rectT{0, 0, a.clientRect().Right, a.tabH + 1}
		pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
	}
}

func (a *app) restaurarAbas() {
	a.ativa = -1
	// a.novaAba("", true)
}

func (a *app) novaJanela(extra string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	arg := "--perfil janela"
	if extra == flagPrivado {
		arg = flagPrivado
	}

	r, _, _ := pShellExecuteW.Call(a.hwnd,
		uintptr(unsafe.Pointer(utf16("open"))),
		uintptr(unsafe.Pointer(utf16(exe))),
		uintptr(unsafe.Pointer(utf16(arg))),
		0, swShow)
	if r <= codigoErroShellExec {
		a.dizerErrof(txtNaoAbreJanela)
		a.invalidateBar()
	}
}
