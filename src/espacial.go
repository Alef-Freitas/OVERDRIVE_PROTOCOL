//go:build windows

package main

import (
	"slices"
	"strings"
	"time"
	"unsafe"
)

const (
	espCardLarguraMin = 40
	espCardAlturaMin  = 30
	espCardRaio       = 10
	espBorda          = 2
	espFundoCard      = 0xE0252521
	espBordaCard      = 0xFF35302E
	espBordaFoco      = 0xFFB6C42E
	espCorSelecao     = 0x80B6C42E
	espCorTitulo      = 0xFFF0EEED
	espCorDim         = 0xFFBAB4B0
	espTituloAlt      = 24
	espMargem         = 8
	espZoomMin        = 0.25
	espZoomMax        = 3.0
	espZoomPasso      = 1.12
	espURLPlaceholder = "Digite uma URL..."
	espBarraAlt       = 26
	vkBackspace       = 0x08
	espMascaraPalavra = 0xFFFF
	espPrimeiroCar    = 32
	espRaioURL        = 6
	espAlphaInicial   = 205
	espAlphaMin       = 40
	espAlphaMax       = 255
	espSliderLargura  = 160
	espSliderAltura   = 8
	espSliderMargem   = 16
	espSliderTopo     = 40
)

type cardEspacial struct {
	rect   rectT
	titulo string
	url    string
	cr     uint32
}

type estadoEspacial struct {
	cartoes          []cardEspacial
	selecao          rectT
	arrastando       bool
	movendo          bool
	origemX          int32
	origemY          int32
	cardOrigem       rectT
	foco             int
	aberto           int
	editando         int
	zoom             float64
	offsetX          int32
	offsetY          int32
	criacao          time.Time
	alpha            uint32
	arrastandoSlider bool
}

//nolint:gochecknoglobals // Estado unico do canvas espacial: UI single-window.
var espCanvas estadoEspacial

//nolint:gochecknoglobals // Flag de edicao de URL via campo nativo do OVERDRIVE.
var espEditAtivo bool

func espFecharPaineisInicio(app *app) {
	if app == nil {
		return
	}

	app.st.RightVisible = false
	app.st.LeftPanelVisible = false
	app.st.ImgVisible = false
	clipAutoLimpar = app.st.LimparClip
}

func espIniciar() {
	espCanvas = estadoEspacial{
		cartoes:          nil,
		selecao:          rectT{},
		arrastando:       false,
		movendo:          false,
		origemX:          0,
		origemY:          0,
		cardOrigem:       rectT{},
		foco:             -1,
		aberto:           -1,
		editando:         -1,
		zoom:             1,
		offsetX:          0,
		offsetY:          0,
		criacao:          time.Time{},
		alpha:            espAlphaInicial,
		arrastandoSlider: false,
	}
}

func espRectValido(r rectT) bool {
	return r.Right-r.Left >= espCardLarguraMin && r.Bottom-r.Top >= espCardAlturaMin
}

func espNormalizar(x1, y1, x2, y2 int32) rectT {
	retangulo := rectT{Left: x1, Top: y1, Right: x2, Bottom: y2}
	if retangulo.Left > retangulo.Right {
		retangulo.Left, retangulo.Right = retangulo.Right, retangulo.Left
	}

	if retangulo.Top > retangulo.Bottom {
		retangulo.Top, retangulo.Bottom = retangulo.Bottom, retangulo.Top
	}

	return retangulo
}

func espDentro(retangulo rectT, x, y int32) bool {
	return retangulo.Width() > 0 && x >= retangulo.Left && x < retangulo.Right &&
		y >= retangulo.Top && y < retangulo.Bottom
}

func espCartaoEm(x, y int32) int {
	for indice := range slices.Backward(espCanvas.cartoes) {
		if espDentro(espCanvas.cartoes[indice].rect, x, y) {
			return indice
		}
	}

	return -1
}

func espAdicionar(retangulo rectT) {
	if !espRectValido(retangulo) {
		return
	}

	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		rect:   retangulo,
		titulo: "",
		url:    "",
		cr:     espBordaCard,
	})

	espCanvas.foco = len(espCanvas.cartoes) - 1
	espCanvas.editando = espCanvas.foco
}

func espAreaURL(indice int) rectT {
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return rectT{}
	}

	area := espCanvas.cartoes[indice].rect

	return rectT{
		area.Left, area.Top - espBarraAlt,
		area.Right, area.Top,
	}
}

func espOcultarEdit(app *app) {
	if app == nil || app.urlEdit == 0 {
		return
	}

	pShowWindow.Call(app.urlEdit, swHide)

	espEditAtivo = false
}

func espMostrarEdit(app *app, indice int) {
	if app == nil || app.urlEdit == 0 || indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	area := espAreaURL(indice)
	if area.Width() <= 0 {
		return
	}

	pSetWindowPos.Call(app.urlEdit, 0,
		uintptr(area.Left), uintptr(area.Top),
		uintptr(area.Width()), uintptr(area.Height()),
		swpNoZOrder|swpNoActivate|swpShowWindow)

	pSetWindowTextW.Call(app.urlEdit, uintptr(unsafe.Pointer(utf16(espCanvas.cartoes[indice].url))))
	app.invalidaEditCache()
	pShowWindow.Call(app.urlEdit, swShow)
	pSetFocus.Call(app.urlEdit)

	espEditAtivo = true
}

func espFinalizarEdicao(app *app) {
	indice := espCanvas.editando
	if app != nil && indice >= 0 && indice < len(espCanvas.cartoes) {
		url := strings.TrimSpace(app.editText())
		espCanvas.cartoes[indice].url = url
		espCanvas.cartoes[indice].titulo = url
	}

	espOcultarEdit(app)

	espCanvas.editando = -1
}

func espEditAplicar(app *app, texto string) {
	indice := espCanvas.editando
	if app == nil || indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	url := strings.TrimSpace(texto)
	espCanvas.cartoes[indice].url = url
	espCanvas.cartoes[indice].titulo = url

	espOcultarEdit(app)

	espCanvas.editando = -1

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espRemoverFoco() {
	indice := espCanvas.foco
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	espCanvas.cartoes = slices.Delete(espCanvas.cartoes, indice, indice+1)
	if espCanvas.aberto == indice {
		espCanvas.aberto = -1
	}

	if espCanvas.editando == indice {
		espCanvas.editando = -1
	}

	espCanvas.foco = -1
}

func espBotaoSair(app *app) rectT {
	if app == nil {
		return rectT{}
	}

	area := app.clientRect()
	margem := app.sc(16)
	largura := app.sc(72)
	altura := app.sc(30)

	return rectT{
		area.Right - margem - largura, margem,
		area.Right - margem, margem + altura,
	}
}

func espPintarBotaoSair(disp *d2dMotor, app *app) {
	botao := espBotaoSair(app)
	if botao.Width() <= 0 {
		return
	}

	disp.redondo(botao, espCardRaio, espFundoCard, espBordaCard)
	disp.textoUI("SAIR", botao, espCorTitulo, true, true)
}

func espSliderRect(app *app) rectT {
	if app == nil {
		return rectT{}
	}

	area := app.clientRect()
	esq := area.Left + espSliderMargem
	topo := area.Top + espSliderTopo

	return rectT{
		esq, topo,
		esq + app.sc(espSliderLargura), topo + app.sc(espSliderAltura),
	}
}

func espAplicarAlpha(app *app) {
	if app == nil || app.hwnd == 0 {
		return
	}

	pSetLayeredWindowAttributes.Call(app.hwnd, 0, uintptr(espCanvas.alpha), lwaAlpha)
}

func espAjustarAlphaSlider(app *app, x int32) {
	area := espSliderRect(app)
	if area.Width() <= 0 {
		return
	}

	fracao := float64(x-area.Left) / float64(area.Width())
	if fracao < 0 {
		fracao = 0
	}

	if fracao > 1 {
		fracao = 1
	}

	espCanvas.alpha = espAlphaMin + uint32(float64(espAlphaMax-espAlphaMin)*fracao)

	espAplicarAlpha(app)

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espPintarSlider(disp *d2dMotor, app *app) {
	area := espSliderRect(app)
	if area.Width() <= 0 {
		return
	}

	disp.ret(area, espBordaCard)

	fracao := float64(espCanvas.alpha-espAlphaMin) / float64(espAlphaMax-espAlphaMin)
	if fracao < 0 {
		fracao = 0
	}

	if fracao > 1 {
		fracao = 1
	}

	posicao := area.Left + int32(float64(area.Width())*fracao)
	marcador := rectT{posicao - 3, area.Top - 4, posicao + 3, area.Bottom + 4}
	disp.ret(marcador, espBordaFoco)
}

func espMouseDown(app *app, x, y int32) {
	if app == nil {
		return
	}

	if espDentro(espSliderRect(app), x, y) {
		espCanvas.arrastandoSlider = true

		espAjustarAlphaSlider(app, x)

		return
	}

	if espDentro(espBotaoSair(app), x, y) {
		pPostQuitMessage.Call(0)

		return
	}

	if espCanvas.editando >= 0 {
		espFinalizarEdicao(app)
		pInvalidateRect.Call(app.hwnd, 0, 1)
	}

	indice := espCartaoEm(x, y)
	if indice >= 0 {
		espCanvas.foco = indice

		if espDentro(espAreaURL(indice), x, y) {
			espCanvas.editando = indice
			espMostrarEdit(app, indice)

			return
		}

		espCanvas.movendo = true
		espCanvas.origemX = x
		espCanvas.origemY = y
		espCanvas.cardOrigem = espCanvas.cartoes[indice].rect

		pSetCapture.Call(app.hwnd)

		return
	}

	espCanvas.foco = -1
	espCanvas.aberto = -1

	espOcultarWebView(app)

	espCanvas.arrastando = true
	espCanvas.origemX = x
	espCanvas.origemY = y
	espCanvas.selecao = rectT{Left: x, Top: y, Right: x, Bottom: y}

	pSetCapture.Call(app.hwnd)

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espMouseMove(app *app, x, y int32) {
	if app == nil {
		return
	}

	if espCanvas.arrastandoSlider {
		espAjustarAlphaSlider(app, x)

		return
	}

	if espCanvas.movendo {
		dx := x - espCanvas.origemX
		dy := y - espCanvas.origemY
		origem := espCanvas.cardOrigem
		espCanvas.cartoes[espCanvas.foco].rect = rectT{
			origem.Left + dx, origem.Top + dy,
			origem.Right + dx, origem.Bottom + dy,
		}

		espPosicionarWebView(app)

		pInvalidateRect.Call(app.hwnd, 0, 1)

		return
	}

	if espCanvas.arrastando {
		espCanvas.selecao = espNormalizar(espCanvas.origemX, espCanvas.origemY, x, y)

		pInvalidateRect.Call(app.hwnd, 0, 1)
	}
}

func espMouseUp(app *app, x, y int32) {
	if app == nil {
		return
	}

	if espCanvas.arrastandoSlider {
		espCanvas.arrastandoSlider = false

		pReleaseCapture.Call()

		return
	}

	if espCanvas.movendo {
		espCanvas.movendo = false

		pReleaseCapture.Call()

		return
	}

	if !espCanvas.arrastando {
		return
	}

	espCanvas.arrastando = false

	pReleaseCapture.Call()

	retangulo := espNormalizar(espCanvas.origemX, espCanvas.origemY, x, y)
	if espRectValido(retangulo) {
		espAdicionar(retangulo)
		espMostrarEdit(app, espCanvas.editando)
	}

	espCanvas.selecao = rectT{}

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espDuploClique(app *app, x, y int32) {
	if app == nil {
		return
	}

	indice := espCartaoEm(x, y)
	if indice < 0 {
		return
	}

	espCanvas.foco = indice
	espAbrirCard(app, indice)
	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espTeclaEditCard(app *app, wparam uintptr) bool {
	switch uint32(wparam) {
	case vkReturn:
		espEditAplicar(app, app.editText())

		return true
	case vkEscape:
		espOcultarEdit(app)

		espCanvas.editando = -1

		return true
	}

	return false
}

func espTecla(app *app, mensagem uint32, wparam uintptr) bool {
	if app == nil || mensagem != wmKeyDown {
		return false
	}

	tecla := uint32(wparam & espMascaraPalavra)
	if tecla == vkDelete && espCanvas.editando < 0 {
		espRemoverFoco()
		pInvalidateRect.Call(app.hwnd, 0, 1)

		return true
	}

	return false
}

func espAbrirCard(app *app, indice int) bool {
	if app == nil || indice < 0 || indice >= len(espCanvas.cartoes) {
		return false
	}

	card := espCanvas.cartoes[indice]
	if card.url == "" {
		return false
	}

	espCanvas.aberto = indice

	if app.left == nil || app.left.wv == nil || !app.left.ready {
		app.novaAba(card.url, true)

		return true
	}

	app.left.wv.Navigate(card.url)
	espPosicionarWebView(app)

	return true
}

func espPosicionarWebView(app *app) {
	if app == nil || app.left == nil || app.left.wv == nil || !app.left.ready {
		return
	}

	indice := espCanvas.aberto
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	area := espCanvas.cartoes[indice].rect

	nova := Rect(area)
	if app.left.boundsApplied != nova {
		app.left.wv.SetBounds(nova)
		app.left.boundsApplied = nova
		app.left.boundsSet = true
	}

	setPaneVisible(app.left, true)
}

func espOcultarWebView(app *app) {
	if app == nil || app.left == nil || app.left.wv == nil || !app.left.ready {
		return
	}

	setPaneVisible(app.left, false)
}

func espPintarBarraURL(disp *d2dMotor, indice int) {
	areaURL := espAreaURL(indice)
	if areaURL.Width() <= 0 {
		return
	}

	corURL := uint32(espBordaCard)
	if indice == espCanvas.editando {
		corURL = espBordaFoco
	}

	disp.redondo(areaURL, espRaioURL, espFundoCard, corURL)

	if indice == espCanvas.editando {
		return
	}

	textoURL := espCanvas.cartoes[indice].url
	if textoURL == "" {
		textoURL = espURLPlaceholder
	}

	areaTexto := rectT{
		areaURL.Left + espMargem, areaURL.Top,
		areaURL.Right - espMargem, areaURL.Bottom,
	}
	disp.textoUI(textoURL, areaTexto, espCorDim, true, false)
}

func espPintar(disp *d2dMotor, app *app) {
	if disp == nil || app == nil {
		return
	}

	for indice, card := range espCanvas.cartoes {
		corBorda := uint32(espBordaCard)
		if indice == espCanvas.foco {
			corBorda = espBordaFoco
		}

		espPintarBarraURL(disp, indice)
		disp.redondo(card.rect, espCardRaio, espFundoCard, corBorda)
	}

	if espCanvas.selecao.Width() > 0 && espCanvas.selecao.Height() > 0 {
		disp.contorno(espCanvas.selecao, espCorSelecao)
	}

	espPintarSlider(disp, app)
}
