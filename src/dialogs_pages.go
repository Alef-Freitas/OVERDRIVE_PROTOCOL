//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func (a *app) mostrarAviso2(texto string, duracao int, temBarra bool, pct int, indet bool) {
	if a.hwnd == 0 || texto == "" {
		return
	}

	duracao, pct = normalizarAviso(duracao, pct)

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return
	}

	if aviso.hwnd == 0 {
		if !a.criarJanelaAviso() {
			return
		}
	}

	aviso.texto = texto
	aviso.visto = texto
	aviso.quando = time.Now()
	aviso.duracao = duracao
	aviso.temBarra = temBarra
	aviso.pct = pct
	aviso.indet = indet

	if a.fontUI == 0 {
		a.initFonts()
	}

	linhas, largura, altura := a.medirAviso(texto, temBarra)

	aviso.linhas = linhas
	aviso.altura = altura

	a.posicionarAviso(largura, altura)
}

// normalizarAviso prende duracao e percentual nas faixas validas.
func normalizarAviso(duracao, pct int) (int, int) {
	const percentualMaximoAviso = 100

	if duracao <= 0 {
		duracao = avisoDuracaoMs
	}

	if pct < 0 {
		pct = 0
	}

	if pct > percentualMaximoAviso {
		pct = percentualMaximoAviso
	}

	return duracao, pct
}

// medirAviso calcula linhas, largura e altura da janela do aviso.
func (a *app) medirAviso(texto string, temBarra bool) ([]string, int32, int32) {
	largMax := int32(float64(a.clientRect().Right) * avisoMaxLargura)
	if largMax < a.sc(220) {
		largMax = a.sc(220)
	}

	larguraTexto := largMax - a.sc(54)
	d := &d2dMain
	linhas := d.quebrar(texto, a.fontUI, larguraTexto, avisoMaxLinhas)

	largura := a.sc(54)
	for _, l := range linhas {
		if w := d.larguraTexto(l, a.fontUI, larguraTexto); w+a.sc(54) > largura {
			largura = w + a.sc(54)
		}
	}

	if largura > largMax {
		largura = largMax
	}

	if largura < a.sc(190) {
		largura = a.sc(190)
	}

	alturaLinha := a.sc(19)

	altura := int32(len(linhas))*alturaLinha + a.sc(16)
	if temBarra {
		altura += a.sc(9)
	}

	if altura < a.sc(34) {
		altura = a.sc(34)
	}

	return linhas, largura, altura
}

func (a *app) avisoOrigem() (int32, int32) {
	posX := a.left.rect.Left + a.sc(12)
	posY := a.left.rect.Top + a.sc(8)

	if a.left.rect.Width() > 0 {
		return posX, posY
	}

	posX = a.sc(12)
	posY = a.tabH + a.tbHeight + a.sc(8)

	if a.soIA {
		posY = a.sc(8)
	}

	return posX, posY
}

func (a *app) avisoEnquadrarX(x, largura int32, area rectT) int32 {
	if area.Right-area.Left <= largura+a.sc(24) {
		return x
	}

	if x+largura > area.Right-a.sc(12) {
		x = area.Right - largura - a.sc(12)
	}

	if x < area.Left+a.sc(12) {
		x = area.Left + a.sc(12)
	}

	return x
}

func (a *app) avisoEnquadrarY(y, altura int32, area rectT) int32 {
	if area.Bottom-area.Top <= altura+a.sc(24) {
		return y
	}

	if y+altura > area.Bottom-a.sc(12) {
		y = area.Bottom - altura - a.sc(12)
	}

	if y < area.Top+a.sc(4) {
		y = area.Top + a.sc(4)
	}

	return y
}

// avisarFalhaSalvar mostra aviso persistente com o motivo e acoes de copiar.
func (a *app) avisarFalhaSalvar(motivo string) {
	if a.hwnd == 0 {
		return
	}

	if aviso.hwnd == 0 && !a.criarJanelaAviso() {
		return
	}

	if a.fontUI == 0 {
		a.initFonts()
	}

	detalhe := resumirErroSalvar(motivo)
	_, largura, _ := a.medirAviso(txtAvisoFalhaTitulo+"\n"+detalhe, false)
	altura := 2*a.sc(19) + a.sc(16) + a.sc(34)

	aviso.texto = txtAvisoFalhaTitulo + "\n" + detalhe
	aviso.visto = aviso.texto
	aviso.quando = time.Now()
	aviso.duracao = 0
	aviso.temBarra = false
	aviso.pct = 0
	aviso.indet = false
	aviso.linhas = []string{txtAvisoFalhaTitulo, detalhe}
	aviso.altura = altura
	aviso.fixo = true
	aviso.copiar = txtAvisoFalhaTitulo + ": " + detalhe

	margem := a.sc(10)
	alturaBotao := a.sc(24)
	yBotao := altura - a.sc(6) - alturaBotao
	larguraBotao := (largura - margem*3) / divisorMetade
	aviso.btnCopiar = rectT{margem, yBotao, margem + larguraBotao, yBotao + alturaBotao}
	aviso.btnFechar = rectT{margem*2 + larguraBotao, yBotao, largura - margem, yBotao + alturaBotao}

	a.posicionarAviso(largura, altura)
	pKillTimer.Call(a.hwnd, timerAviso)
	pKillTimer.Call(a.hwnd, timerStatus)
	a.status = ""
}

func (a *app) avisoMostrarAnimado(_, altura int32) {
	eraVisivel := aviso.visivel
	if !eraVisivel {
		aviso.offsetY = -altura
		aviso.animDe = -altura
		aviso.animAte = 0
		aviso.animIni = time.Now()
		aviso.animando = true
		aviso.saindo = false
		pShowWindow.Call(aviso.hwnd, swShowNoActivate)
		aviso.visivel = true
	} else if aviso.saindo {
		aviso.animDe = aviso.offsetY
		aviso.animAte = 0
		aviso.animIni = time.Now()
		aviso.animando = true
		aviso.saindo = false
	}

	pInvalidateRect.Call(aviso.hwnd, 0, 0)
	pUpdateWindow.Call(aviso.hwnd)

	if aviso.animando && a.hwnd != 0 {
		const intervaloAnimacaoAvisoMs = 15
		pSetTimer.Call(a.hwnd, timerAvisoAnim, intervaloAnimacaoAvisoMs, 0)
	}

	const atrasoOcultacaoAvisoMs = 700
	pSetTimer.Call(a.hwnd, timerAviso, atrasoOcultacaoAvisoMs, 0)
}

// posicionarAviso move a janela do aviso para perto da faixa.
func (a *app) posicionarAviso(largura, altura int32) {
	posX, posY := a.avisoOrigem()

	ponto := struct{ x, y int32 }{posX, posY}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	x := a.avisoEnquadrarX(ponto.x, largura, area)
	y := a.avisoEnquadrarY(ponto.y, altura, area)

	pSetWindowPos.Call(aviso.hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(largura), uintptr(altura),
		swpNoActivate)

	a.avisoMostrarAnimado(largura, altura)
}

func (a *app) esconderAviso() {
	if aviso.hwnd == 0 {
		return
	}

	if !aviso.visivel {
		pShowWindow.Call(aviso.hwnd, swHide)

		return
	}

	if aviso.saindo {
		return
	}

	altura := aviso.altura
	if altura <= 0 {
		var cr rectT
		pGetClientRect.Call(aviso.hwnd, uintptr(unsafe.Pointer(&cr)))
		altura = cr.Bottom
	}

	aviso.saindo = true
	aviso.animDe = aviso.offsetY
	aviso.animAte = -altura
	aviso.animIni = time.Now()
	aviso.animando = true

	if a.hwnd != 0 {
		const intervaloAnimacaoAvisoMs = 15
		pSetTimer.Call(a.hwnd, timerAvisoAnim, intervaloAnimacaoAvisoMs, 0)
	}
}

func (a *app) finalizarAvisoOculto() {
	if aviso.hwnd != 0 {
		pShowWindow.Call(aviso.hwnd, swHide)
	}

	aviso.visivel = false
	aviso.saindo = false
	aviso.animando = false
	aviso.offsetY = 0

	if theApp != nil && theApp.hwnd != 0 {
		pKillTimer.Call(theApp.hwnd, timerAviso)
		pKillTimer.Call(theApp.hwnd, timerAvisoAnim)
	}

	aviso.texto = ""
	aviso.linhas = nil
	aviso.temBarra = false
	aviso.fixo = false
	aviso.copiar = ""
	aviso.btnCopiar = rectT{}
	aviso.btnFechar = rectT{}

	d2dAviso.soltar()
}

func (a *app) animarAviso() {
	if !aviso.animando {
		if a != nil && a.hwnd != 0 {
			pKillTimer.Call(a.hwnd, timerAvisoAnim)
		}

		return
	}

	duracao := time.Duration(avisoAnimMs) * time.Millisecond

	const duracaoPadraoAnimacaoAvisoMs = 150

	if duracao <= 0 {
		duracao = duracaoPadraoAnimacaoAvisoMs * time.Millisecond
	}

	progresso := float64(time.Since(aviso.animIni)) / float64(duracao)
	if progresso < 0 {
		progresso = 0
	}

	if progresso > 1 {
		progresso = 1
	}

	const coeficienteSuavizacaoAviso = 3

	e := progresso * progresso * (coeficienteSuavizacaoAviso - 2*progresso)
	aviso.offsetY = aviso.animDe + int32(float64(aviso.animAte-aviso.animDe)*e)
	pInvalidateRect.Call(aviso.hwnd, 0, 0)
	pUpdateWindow.Call(aviso.hwnd)

	if progresso >= 1 {
		aviso.animando = false

		if a.hwnd != 0 {
			pKillTimer.Call(a.hwnd, timerAvisoAnim)
		}

		if aviso.saindo {
			a.finalizarAvisoOculto()
		}
	}
}

func (a *app) descartarAviso() {
	if a == nil || a.hwnd == 0 {
		return
	}

	pKillTimer.Call(a.hwnd, timerAviso)
	pKillTimer.Call(a.hwnd, timerStatus)
	a.status = ""
	a.esconderAviso()
	a.invalidateFaixa()
	a.invalidateBar()
}

func (a *app) criarJanelaAviso() bool {
	const (
		larguraInicialAviso = 100
		alturaInicialAviso  = 30
	)

	if avisoProcPtr == 0 {
		return false
	}

	if aviso.hwnd != 0 {
		return true
	}

	hInst := getModuleHandle()
	nomeClasse := utf16(avisoClasse)
	classe := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   avisoProcPtr,
		hInstance:     hInst,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: nomeClasse,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classe)))

	h, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExNoActivate|wsExToolWindow,
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(utf16(""))),
		wsPopup,
		0, 0, larguraInicialAviso, alturaInicialAviso,
		0, 0, hInst, 0,
	)
	if h == 0 {
		return false
	}

	pSetWindowTheme.Call(h, uintptr(unsafe.Pointer(utf16("DarkMode_Explorer"))), 0)
	aviso.hwnd = h

	return true
}

func (a *app) vigiarAviso() {
	if aviso.fixo {
		return
	}

	if a.status == "" {
		if aviso.texto != "" {
			a.esconderAviso()
		}

		aviso.visto = ""

		return
	}

	if a.status != aviso.visto {
		aviso.visto = a.status
	}

	if aviso.saindo {
		return
	}

	duracao := aviso.duracao
	if duracao <= 0 {
		duracao = avisoDuracaoMs
	}

	if time.Since(aviso.quando) > time.Duration(duracao)*time.Millisecond {
		a.esconderAviso()

		return
	}

	restante := time.Duration(duracao)*time.Millisecond - time.Since(aviso.quando)

	const atrasoOcultacaoAvisoMs = 700

	proximo := min(restante, atrasoOcultacaoAvisoMs*time.Millisecond)

	proximo = max(proximo, time.Millisecond)

	pSetTimer.Call(a.hwnd, timerAviso, uintptr(proximo.Milliseconds()), 0)
}

func (a *app) avisoPintarJanela(hwnd uintptr) uintptr {
	var cliente rectT
	pGetClientRect.Call(aviso.hwnd, uintptr(unsafe.Pointer(&cliente)))

	if cliente.Right <= 0 || cliente.Bottom <= 0 {
		pValidateRect.Call(hwnd, 0)

		return 0
	}

	if d2dAviso.rt == nil {
		d2dAviso.criar(hwnd, cliente.Right, cliente.Bottom)
	}

	if d2dAviso.rt != nil {
		d2dAviso.iniciar()
		a.paintAvisoD2D()
		d2dAviso.apresentar()
	} else {
		pValidateRect.Call(hwnd, 0)
	}

	return 0
}

func avisoRedimensionar(hwnd uintptr, a *app) uintptr {
	if a != nil {
		var cr rectT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		d2dAviso.redimensionar(cr.Right, cr.Bottom)
	}

	return 0
}

// tratarCliqueAvisoFixo trata os botoes do aviso persistente; true = clicou.
func tratarCliqueAvisoFixo(lparam uintptr) bool {
	if !aviso.fixo {
		return false
	}

	x, y := loword(lparam), hiword(lparam)

	switch {
	case dentroRect(aviso.btnCopiar, x, y):
		_ = copiarTexto(aviso.copiar)

		return true
	case dentroRect(aviso.btnFechar, x, y):
		return true
	default:
		return false
	}
}

func avisoProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		if aplicativo != nil {
			return aplicativo.avisoPintarJanela(hwnd)
		}

		pValidateRect.Call(hwnd, 0)

		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		return avisoRedimensionar(hwnd, aplicativo)
	case wmLButtonDown:
		if aplicativo == nil {
			return 0
		}

		clicou := tratarCliqueAvisoFixo(lparam)
		if clicou || !aviso.fixo {
			aplicativo.depois(func() { aplicativo.descartarAviso() })
		}

		return 0
	case wmNcHitTest:
		return htClient
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func (a *app) avisarNaPagina(p *pane, num int, modo string) {
	if p == nil || !p.ready {
		return
	}

	qual := txtAvisoArtigo
	if modo == modoTudo {
		qual = txtAvisoPagInteira
	}

	texto := fmt.Sprintf(txtFmtAvisoPagina, formataMilhar(num), qual)
	js := strings.ReplaceAll(avisoPaginaJS, "TEXTO_AQUI", texto)
	p.wv.EvalJS(js, func(err error, res string) {
		if err != nil {
			a.dizerErrof(txtFmtAvisoFalhou, err)

			return
		}

		if strings.TrimSpace(res) != `"ok"` {
			a.dizerErrof(txtFmtAvisoFalhou, txtAvisoNaoConf)
		}
	})
}

func (a *app) limparAposCookies() {
	a.permissoesMu.Lock()
	a.permissoes = map[string]map[uint32]bool{}
	a.permissoesMu.Unlock()

	a.st.RightURL = ""
	if a.right != nil && a.right.ready {
		a.right.queriaURL = urlChat
		a.right.wv.Navigate(urlChat)
	}

	a.chatVazioTentou = false

	if p := a.left; p != nil && p.ready {
		p.wv.Reload()
	}

	a.dizerOkf(txtCookiesApagados)
	a.invalidateBar()
}

func ehPaginaInterna(endereco string) bool {
	if strings.HasPrefix(endereco, "overdrive://") {
		return true
	}

	if strings.HasPrefix(endereco, "file://") {
		return true
	}

	if strings.HasPrefix(endereco, "data:text/html") {
		return true
	}

	return endereco == "" || strings.HasPrefix(endereco, "about:")
}

func (a *app) abrirPagina(secao string) {
	a.abrirCfg(secao)
}

func (a *app) dicaParaExtensao(id string) (string, bool) {
	after, ok := strings.CutPrefix(id, "ext:")
	if !ok {
		return "", false
	}

	if orig := a.pastaRealExtensao(after); orig != "" {
		return orig + txtDicaAbrePainel, true
	}

	return txtDicaExtensao, true
}

func (a *app) dicaAbaEspecial(rest string) (string, bool) {
	switch rest {
	case sufAbaNova:
		return txtDicaNovaGuia, true
	case sufAbaFechar:
		return txtDicaFecharGuia, true
	case idAudio:
		return a.dicaAbaAudio(rest)
	case "overflow":
		if a.tabOverflowN > 0 {
			return fmt.Sprintf(txtFmtGuiasOcultas, a.tabOverflowN), true
		}

		return "", true
	default:
		return "", false
	}
}

func (a *app) dicaAbaAudio(rest string) (string, bool) {
	num, err := strconv.Atoi(rest)
	if err != nil || num < 0 || num >= len(a.tabs) || a.tabs[num] == nil || a.tabs[num].pane == nil {
		return txtDicaSilenciar, true
	}

	if a.tabs[num].pane.mudo {
		return txtDicaReativarSom, true
	}

	return txtDicaSilenciar, true
}

func (a *app) dicaAbaNumero(rest string) (string, bool) {
	num, err := strconv.Atoi(rest)
	if err != nil || num < 0 || num >= len(a.tabs) {
		return "", false
	}

	titulo := a.tituloDaAba(a.tabs[num])
	if p := a.tabs[num].pane; p != nil && (p.tocandoAudio || p.mudo) {
		titulo += a.dicaSufixoAudio(p.mudo)
	}

	return titulo, true
}

func (a *app) dicaSufixoAudio(mudo bool) string {
	if mudo {
		return txtDicaSilenciada
	}

	return txtDicaTocando
}

func (a *app) dicaParaAba(id string) (string, bool) {
	after, ok := strings.CutPrefix(id, "tab:")
	if !ok {
		return "", false
	}

	if dica, ok := a.dicaAbaEspecial(after); ok {
		return dica, true
	}

	if dica, ok := a.dicaAbaNumero(after); ok {
		return dica, true
	}

	return "", true
}

func (a *app) dicaParaJanela(id string) (string, bool) {
	switch id {
	case "win:minimizar":
		return txtDicaMinimizar, true
	case "win:maximizar":
		return a.dicaMaximizar()
	case "win:fechar":
		return txtDicaFechar, true
	default:
		return "", false
	}
}

func (a *app) dicaMaximizar() (string, bool) {
	if z, _, _ := pIsZoomed.Call(a.hwnd); z != 0 {
		return txtDicaRestaurar, true
	}

	return txtDicaMaximizar, true
}

func (a *app) dicaParaNavegacaoBase(id string) (string, bool) {
	switch id {
	case idVoltar:
		return txtDicaVoltar, true
	case idAvancar:
		return txtDicaAvancar, true
	case idRecarregar:
		return a.dicaRecarregar()
	case idParar:
		return txtDicaParar, true
	case "go":
		return txtDicaIrEndereco, true
	case idEngine:
		return fmt.Sprintf(txtDicaMotorBusca, a.motorAtual().nome), true
	case idSeguranca:
		return a.dicaSeguranca(), true
	case idFerramentas:
		return txtDicaFerramentas, true
	default:
		return "", false
	}
}

func (a *app) dicaParaNavegacaoMeio(id string) (string, bool) {
	switch id {
	case idImprimir:
		return txtDicaPrint, true
	case idDownloads:
		return txtAbrirDownloads, true
	case idClipboard:
		return txtDicaClipboard, true
	case "tr":
		return a.dicaTraducao()
	case idHome:
		return txtDicaPagInicial, true
	case idCookies:
		return a.dicaCookies()
	case idPaywall:
		return txtDicaPaywall, true
	case idReader:
		return txtDicaLeitura, true
	default:
		return "", false
	}
}

func (a *app) dicaParaNavegacaoFim(id string) (string, bool) {
	switch id {
	case idCopiarLeitura:
		return txtDicaCopiarLeitura, true
	case idNojs:
		return txtDicaSemJS, true
	case idVPN:
		return txtDicaVPN, true
	case idSleep:
		return a.dicaSono()
	default:
		return "", false
	}
}

func (a *app) dicaParaNavegacao(id string) (string, bool) {
	if dica, ok := a.dicaParaNavegacaoBase(id); ok {
		return dica, true
	}

	if dica, ok := a.dicaParaNavegacaoMeio(id); ok {
		return dica, true
	}

	return a.dicaParaNavegacaoFim(id)
}

func (a *app) dicaRecarregar() (string, bool) {
	if a.loading {
		return txtDicaParar, true
	}

	return txtDicaRecarregar, true
}

func (a *app) dicaTraducao() (string, bool) {
	if a.traduzido {
		return txtVerOriginal, true
	}

	return txtDicaTraduzir, true
}

func (a *app) dicaCookies() (string, bool) {
	if dom := dominioDeURL(a.urlDaBarraAtual()); dom != "" {
		return fmt.Sprintf(txtFmtCookiesDom, dom), true
	}

	return txtDicaCookies, true
}

func (a *app) dicaSono() (string, bool) {
	if a.left != nil && a.left.sonoDormindo {
		return txtDicaSonoDormindo, true
	}

	if a.st.SleepLite {
		return txtDicaSonoLigado, true
	}

	return txtDicaSonoDeslig, true
}

func (a *app) dicaParaPaineisBase(id string) (string, bool) {
	switch id {
	case idLeftAux:
		return a.dicaPainelLateral()
	case idLeftAuxFull:
		return a.dicaPainelCheio()
	case idYoutube:
		return a.dicaTube()
	case idExtensoes:
		return txtDicaExtensoes, true
	case idMenu:
		return txtDicaMenu, true
	case "zin":
		return txtDicaZoomMenos, true
	case "zout":
		return txtDicaZoomMais, true
	case "zreset":
		return txtDicaZoomReset, true
	default:
		return "", false
	}
}

func (a *app) dicaParaPaineisMeio(id string) (string, bool) {
	switch id {
	case "zoomfit":
		return txtDicaZoomFit, true
	case idSplit:
		return a.dicaSplit()
	case idImgToggle:
		return a.dicaImgToggle()
	case idChatHome:
		return txtDicaChatHome, true
	case idPainelMenu:
		return txtDicaPanelMenu, true
	case idPainelReload:
		return txtDicaPanelReload, true
	case idPainelTranslate:
		return txtDicaPanelTrad, true
	default:
		return "", false
	}
}

func (a *app) dicaParaPaineisFim(id string) (string, bool) {
	switch id {
	case idPainelSend:
		return txtDicaPanelSend, true
	case idPainelFull:
		return a.dicaPainelFull()
	case "addr":
		return txtDicaEndereco, true
	default:
		return "", false
	}
}

func (a *app) dicaParaPaineis(id string) (string, bool) {
	if dica, ok := a.dicaParaPaineisBase(id); ok {
		return dica, true
	}

	if dica, ok := a.dicaParaPaineisMeio(id); ok {
		return dica, true
	}

	return a.dicaParaPaineisFim(id)
}

func (a *app) dicaPainelLateral() (string, bool) {
	if a.st.LeftPanelVisible {
		return txtDicaRecolherPainel, true
	}

	return txtDicaExpandirPainel, true
}

func (a *app) dicaPainelCheio() (string, bool) {
	if a.painelCheio {
		return txtDicaSairFullPainel, true
	}

	return txtDicaFullPainel, true
}

func (a *app) dicaTube() (string, bool) {
	if a.ytAberto && a.ytOculto {
		return txtDicaTubeFundo, true
	}

	if a.ytAberto {
		return txtDicaTubeTela, true
	}

	return txtDicaTube, true
}

func (a *app) dicaSplit() (string, bool) {
	if a.st.RightVisible {
		return txtDicaSplitChat, true
	}

	return txtDicaSplitAbrir, true
}

func (a *app) dicaImgToggle() (string, bool) {
	if a.st.ImgVisible {
		return txtDicaOcultarIA, true
	}

	return txtDicaMostrarIA, true
}

func (a *app) dicaPainelFull() (string, bool) {
	if a.soIA {
		return txtDicaVoltarNav, true
	}

	return txtDicaPanelFull, true
}

func (a *app) tipFor(id string) string {
	if dica, ok := a.dicaParaExtensao(id); ok {
		return dica
	}

	if dica, ok := a.dicaParaAba(id); ok {
		return dica
	}

	if dica, ok := a.dicaParaJanela(id); ok {
		return dica
	}

	if dica, ok := a.dicaParaNavegacao(id); ok {
		return dica
	}

	if dica, ok := a.dicaParaPaineis(id); ok {
		return dica
	}

	return ""
}

func (a *app) initTooltip() {
	const tamanhoInicialDica = 10

	if a.tipHwnd != 0 {
		return
	}

	if tipProcPtr == 0 {
		return
	}

	className := utf16(tipClassName)
	classe := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         0,
		lpfnWndProc:   tipProcPtr,
		hInstance:     getModuleHandle(),
		lpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classe)))
	a.tipHwnd, _, _ = pCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExNoActivate|wsExTransparent,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16(""))),
		wsPopup,
		0, 0, tamanhoInicialDica, tamanhoInicialDica,
		a.hwnd, 0, getModuleHandle(), 0,
	)
	aplicarCantosDWM(a.tipHwnd)
}

func (a *app) scheduleTip(id string, area rectT) {
	if a.tipHwnd == 0 {
		return
	}

	if id == "" {
		a.hideTip()

		return
	}

	if menu.open || panelMenu.open {
		a.hideTip()

		return
	}

	txt := a.tipFor(id)
	if txt == "" {
		a.hideTip()

		return
	}

	a.tipText = txt
	a.tipAnchor = area

	a.tipPending = true
	if a.tipShown {
		a.showTip()

		return
	}

	const atrasoDicaMs = 420
	pSetTimer.Call(a.hwnd, timerTip, atrasoDicaMs, 0)
}

func (a *app) showTip() {
	if a.tipHwnd == 0 || a.tipText == "" || menu.open || panelMenu.open {
		return
	}

	dmed := &d2dMain
	larguraTexto := dmed.larguraTexto(a.tipText, a.fontSm, 0)
	pad := a.sc(8)
	altura := a.sc(22)

	const ladosPreenchimentoDica = 2

	largura := larguraTexto + ladosPreenchimentoDica*pad
	if largura < a.sc(40) {
		largura = a.sc(40)
	}

	ponto := struct{ x, y int32 }{a.tipAnchor.Left + a.tipAnchor.Width()/2, a.tipAnchor.Bottom + a.sc(4)}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	const divisorCentroDica = 2

	x := ponto.x - largura/divisorCentroDica
	y := ponto.y

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	if area.Right > area.Left {
		if x+largura > area.Right-a.sc(4) {
			x = area.Right - a.sc(4) - largura
		}

		if x < area.Left+a.sc(4) {
			x = area.Left + a.sc(4)
		}
	}

	pSetWindowPos.Call(a.tipHwnd, hwndTopmost, uintptr(x), uintptr(y),
		uintptr(largura), uintptr(altura), swpNoActivate|swpShowWindow)
	a.tipShown = true
	a.tipPending = false
	pInvalidateRect.Call(a.tipHwnd, 0, 1)
}

func (a *app) hideTip() {
	if a.tipHwnd == 0 {
		return
	}

	pKillTimer.Call(a.hwnd, timerTip)
	a.tipPending = false

	d2dTip.soltar()

	if a.tipShown {
		pShowWindow.Call(a.tipHwnd, swHide)
		a.tipShown = false
	}
}

func tipRedimensionar(hwnd uintptr) uintptr {
	var cr rectT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	d2dTip.redimensionar(cr.Right, cr.Bottom)

	return 0
}

func tipPintarConteudo(hwnd uintptr, aplicativo *app, cr rectT) {
	if d2dTip.rt == nil {
		d2dTip.criar(hwnd, cr.Right, cr.Bottom)
	}

	if d2dTip.rt != nil {
		d2dTip.iniciar()
		aplicativo.paintTipD2D()
		d2dTip.apresentar()
	} else {
		pValidateRect.Call(hwnd, 0)
	}
}

func tipAoPintar(hwnd uintptr, aplicativo *app) uintptr {
	if aplicativo == nil {
		pValidateRect.Call(hwnd, 0)

		return 0
	}

	var cliente rectT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cliente)))

	if cliente.Right <= 0 || cliente.Bottom <= 0 {
		pValidateRect.Call(hwnd, 0)

		return 0
	}

	tipPintarConteudo(hwnd, aplicativo, cliente)

	return 0
}

func tipProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmSize:
		return tipRedimensionar(hwnd)
	case wmPaint:
		return tipAoPintar(hwnd, aplicativo)
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func nomeDoIdioma(cod string) string {
	for _, l := range idiomasOferecidos {
		if l.codigo == cod {
			return l.nome
		}
	}

	if cod == "" {
		return txtIdiomaPadrao
	}

	return cod
}

func (a *app) alternarTraducao() {
	p := a.left
	if p == nil || !p.ready {
		a.dizerf(txtSemPagTraduzir)
		a.invalidateBar()

		return
	}

	if traduzindo {
		a.dizerf(txtJaTraduzindo)
		a.invalidateBar()

		return
	}

	if p != nil && p.ready && p.wv != nil && ehProxyGoogle(strings.TrimSpace(p.wv.Source())) {
		a.desfazerTraducao(p)

		return
	}

	jsTradAtivo := "(function(){try{return (window.__odTrad&&window.__odTrad.ativo())?'sim':'nao';}" +
		"catch(e){return 'nao';}})()"
	p.wv.EvalJS(jsTradAtivo,
		func(err error, res string) {
			if err == nil && strings.Contains(res, "sim") {
				a.desfazerTraducao(p)

				return
			}

			a.depois(func() { a.traduzirPagina(p) })
		})
}

func (a *app) desfazerTraducao(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if ehProxyGoogle(strings.TrimSpace(p.wv.Source())) {
		a.voltarDoProxyGoogle(p)

		return
	}

	jsDesfazer := "(function(){try{return window.__odDesfazer?window.__odDesfazer():'{\"ok\":false}';}" +
		"catch(e){return '{\"ok\":false}';}})()"
	p.wv.EvalJS(jsDesfazer, func(error, string) {
		a.depois(func() {
			a.traduzido = false
			a.dizerf(txtPagOriginal)
			a.invalidateBar()
		})
	})
}

func (a *app) modoTraducao() string {
	if a.st.ModoTraducao == idNativo {
		return idNativo
	}

	return chaveGoogle
}

func ehProxyGoogle(u string) bool {
	pu, err := url.Parse(strings.TrimSpace(u))
	if err != nil || pu.Host == "" {
		return false
	}

	return strings.HasSuffix(strings.ToLower(pu.Host), ".translate.goog")
}

func urlTradutorGoogle(endereco, alvo string) string {
	analisado, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || analisado.Host == "" {
		return ""
	}

	if ehProxyGoogle(endereco) {
		return ""
	}

	host := strings.ToLower(analisado.Host)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}

	if host == "" {
		return ""
	}

	analisado.Scheme = esquemaHTTPS
	analisado.Host = strings.ReplaceAll(host, ".", "-") + ".translate.goog"
	q := analisado.Query()
	q.Set("_x_tr_sl", "auto")
	q.Set("_x_tr_tl", alvo)
	q.Set("_x_tr_hl", alvo)
	q.Set("_x_tr_pto", "wapp")
	analisado.RawQuery = q.Encode()

	return analisado.String()
}

func origemDoProxy(endereco string) string {
	analisado, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || analisado.Host == "" || !ehProxyGoogle(endereco) {
		return ""
	}

	base := strings.TrimSuffix(strings.ToLower(analisado.Host), ".translate.goog")
	analisado.Scheme = esquemaHTTPS
	analisado.Host = strings.ReplaceAll(base, "-", ".")
	q := analisado.Query()
	q.Del("_x_tr_sl")
	q.Del("_x_tr_tl")
	q.Del("_x_tr_hl")
	q.Del("_x_tr_pto")
	analisado.RawQuery = q.Encode()

	return analisado.String()
}

func (a *app) traduzirViaGoogle(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtSemPagTraduzir)
		a.invalidateBar()

		return
	}

	alvo := a.st.Idioma
	if alvo == "" {
		alvo = idiomaPtBR
	}

	atual := strings.TrimSpace(p.wv.Source())

	origem := atual
	if ehProxyGoogle(atual) {
		if a.traducaoOrigemURL != "" {
			origem = a.traducaoOrigemURL
		} else if u := origemDoProxy(atual); u != "" {
			origem = u
		}
	} else if atual != "" {
		a.traducaoOrigemURL = atual
	}

	destino := urlTradutorGoogle(origem, alvo)
	if destino == "" {
		a.dizerf(txtNaoAbriuTradG)
		a.invalidateBar()

		return
	}

	a.traduzido = true
	a.status = fmt.Sprintf(txtTraduzindoGoogle, nomeDoIdioma(alvo))
	a.invalidateBar()
	p.wv.Navigate(destino)
}

func (a *app) voltarDoProxyGoogle(p *pane) {
	destino := a.traducaoOrigemURL
	if destino == "" && p != nil && p.ready && p.wv != nil {
		destino = origemDoProxy(strings.TrimSpace(p.wv.Source()))
	}

	a.traducaoOrigemURL = ""

	a.traduzido = false
	if destino == "" {
		a.dizerf(txtPagOriginal)
		a.invalidateBar()

		return
	}

	a.dizerf(txtPagOriginal)
	a.invalidateBar()

	if p != nil && p.ready && p.wv != nil {
		p.wv.Navigate(destino)
	}
}

func (a *app) passarTraducao(p *pane, alvo string, rodada int, gen uint64) {
	if p == nil || !p.ready || p.wv == nil || !tradValida(gen) {
		traduzindo = false

		return
	}

	js := fmt.Sprintf("(function(){try{return window.__odColetar?(window.__odColetar(%d,%d)):'{\"ok\":false}';}"+
		"catch(e){return '{\"ok\":false}';}})()", tradMaxTrechos, tradMaxChars)
	p.wv.EvalJS(js, func(err error, res string) {
		a.traduzirAoColetar(p, alvo, rodada, gen, err, res)
	})
}

func (a *app) traduzirAoColetar(p *pane, alvo string, rodada int, gen uint64, err error, res string) {
	if !tradValida(gen) {
		traduzindo = false

		return
	}

	if err != nil {
		a.fimTraducao(p, erroTrad(err.Error()))

		return
	}

	var resposta struct {
		OK    bool     `json:"ok"`
		Erro  string   `json:"err"`
		Itens []string `json:"itens"`
		Lang  string   `json:"lang"`
		Total int      `json:"total"`
	}
	if e := desembrulharEvalJS(res, &resposta); e != nil || !resposta.OK {
		a.fimTraducao(p, txtTraducaoNaoLi)

		return
	}

	if len(resposta.Itens) == 0 {
		a.traduzirColetaVazia(p, rodada)

		return
	}

	a.depois(func() {
		a.dizerProgressoIndetf(txtFmtTraduzindoN, len(resposta.Itens))
	})

	itens := append([]string{}, resposta.Itens...)

	go a.traduzirLote(p, alvo, rodada, gen, itens)
}

func (a *app) traduzirColetaVazia(p *pane, rodada int) {
	if rodada == 0 {
		a.depois(func() {
			a.dizerf(txtTraducaoSemTexto)
			a.invalidateBar()
		})
	}

	a.fimTraducao(p, "")
}

func (a *app) traduzirAplicarParcial(p *pane, alvo string, gen uint64, mapa map[string]string) {
	if len(mapa) == 0 || !tradValida(gen) {
		return
	}

	//nolint:errchkjson // Marshal de map[string]string e infalivel; sem erro possivel.
	corpo, _ := json.Marshal(mapa)
	scriptAplicar := fmt.Sprintf("(function(){try{return window.__odAplicar?(window.__odAplicar(%q,%q)):'{}';}"+
		"catch(e){return '{}';}})()", string(corpo), alvo)

	a.depois(func() {
		if !tradValida(gen) {
			return
		}

		if p != nil && p.ready && p.wv != nil {
			p.wv.EvalJS(scriptAplicar, func(error, string) {})

			a.traduzido = true
		}
	})
}

func (a *app) traduzirLote(p *pane, alvo string, rodada int, gen uint64, itens []string) {
	cancelado := func() bool { return !tradValida(gen) }

	aplicarParcial := func(mapa map[string]string) {
		a.traduzirAplicarParcial(p, alvo, gen, mapa)
	}

	mapa, origem, terr := traduzirTextos(itens, alvo, aplicarParcial, cancelado)

	if !tradValida(gen) {
		traduzindo = false

		return
	}

	if len(mapa) == 0 {
		a.fimTraducao(p, erroTradDe(terr))

		return
	}

	//nolint:errchkjson // Marshal de map[string]string e infalivel; sem erro possivel.
	corpo, _ := json.Marshal(mapa)
	scriptAplicar := fmt.Sprintf(
		"(function(){try{return window.__odAplicar?(window.__odAplicar(%q,%q)):'{\"ok\":false}';}"+
			"catch(e){return '{\"ok\":false}';}})()",
		string(corpo), alvo,
	)

	a.depois(func() {
		a.traduzirAplicarFinal(p, alvo, origem, rodada, gen, scriptAplicar)
	})
}

func (a *app) traduzirAplicarFinal(p *pane, alvo, origem string, rodada int, gen uint64, scriptAplicar string) {
	if !tradValida(gen) {
		traduzindo = false

		return
	}

	if p == nil || !p.ready || p.wv == nil {
		traduzindo = false

		return
	}

	p.wv.EvalJS(scriptAplicar, func(_ error, resAplicar string) {
		a.traduzirAoAplicar(p, alvo, origem, rodada, gen, resAplicar)
	})
}

func (a *app) traduzirAoAplicar(p *pane, alvo, origem string, rodada int, gen uint64, resAplicar string) {
	var resultado struct {
		OK    bool   `json:"ok"`
		N     int    `json:"n"`
		Total int    `json:"total"`
		Erro  string `json:"err"`
	}

	_ = desembrulharEvalJS(resAplicar, &resultado)

	a.depois(func() {
		a.traduzirConcluirAplicacao(p, alvo, origem, rodada, gen, resultado.OK, resultado.Erro)
	})
}

func (a *app) traduzirConcluirAplicacao(p *pane, alvo, origem string, rodada int, gen uint64, ok bool, erro string) {
	if !tradValida(gen) {
		return
	}

	if !ok && erro != "ocupado" {
		a.fimTraducao(p, txtTraducaoNaoAplica)

		return
	}

	a.traduzido = true
	if origem != "" && origem != alvo {
		a.st.Idioma = alvo
	}

	const maxRodadasTraducao = 2
	if rodada >= maxRodadasTraducao {
		a.fimTraducao(p, "")

		return
	}

	const atrasoProximaRodadaTraducaoMs = 1500
	time.AfterFunc(atrasoProximaRodadaTraducaoMs*time.Millisecond, func() {
		a.depois(func() {
			if tradValida(gen) {
				a.passarTraducao(p, alvo, rodada+1, gen)
			}
		})
	})
}

func erroTradDe(erro error) string {
	if erro == nil {
		return txtTraducaoFalhou
	}

	if strings.Contains(erro.Error(), "429") {
		return txtServicoLimitado
	}

	return fmt.Sprintf(txtFmtFalhaTraducao, erro.Error())
}

func erroTrad(msg string) string {
	if msg == "" {
		return txtTraducaoFalha
	}

	return fmt.Sprintf(txtFmtTraducaoFalhaMsg, msg)
}

func (a *app) fimTraducao(p *pane, problema string) {
	traduzindo = false

	a.depois(func() {
		if problema != "" {
			a.dizerf("%s", problema)
			a.invalidateBar()

			return
		}

		if p != nil && p.ready && p.wv != nil {
			jsMapa := "(function(){try{var m=window.__odTradMapa||{};" +
				"return String(Object.keys(m).length);}catch(e){return '0';}})()"
			p.wv.EvalJS(jsMapa, func(_ error, res string) {
				contagem := strings.TrimSpace(strings.Trim(res, `"`))

				a.depois(func() {
					if contagem == "" || contagem == "0" {
						a.dizerf(txtFmtPagTraduzida, nomeDoIdioma(a.st.Idioma))
					} else {
						a.dizerf(txtFmtPagTrechos, nomeDoIdioma(a.st.Idioma), contagem)
					}

					a.invalidateBar()
				})
			})

			return
		}

		a.dizerf(txtFmtPagTraduzida, nomeDoIdioma(a.st.Idioma))
		a.invalidateBar()
	})
}

func (a *app) traduzirPagina(p *pane) {
	if a.modoTraducao() == chaveGoogle {
		a.traduzirViaGoogle(p)

		return
	}

	if traduzindo {
		return
	}

	a.tradRetentou = false
	if restante := tradEsperaRestante(); restante > time.Second {
		a.dizerf(txtTraducaoCalma, int(restante.Seconds())+1)
		a.invalidateBar()

		return
	}

	traduzindo = true

	a.dizerf(txtProcurandoTexto)
	a.invalidateBar()

	alvo := a.st.Idioma
	if alvo == "" {
		alvo = idiomaPtBR
	}

	gen := tradGeracaoAtual(p)
	a.passarTraducao(p, alvo, 0, gen)
}
