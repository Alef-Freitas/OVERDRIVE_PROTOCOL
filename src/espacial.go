//go:build windows

package main

import (
	"slices"
	"strings"
	"time"
)

const (
	espCardLarguraMin = 40
	espCardAlturaMin  = 30
	espCardRaio       = 10
	espBorda          = 2
	espFundoCard      = 0xD0182028
	espBordaCard      = 0xFF3A4A5A
	espBordaFoco      = 0xFF00D0E0
	espCorSelecao     = 0x8000D0E0
	espCorTitulo      = 0xFFE8F0F4
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
)

type cardEspacial struct {
	rect   rectT
	titulo string
	url    string
	cr     uint32
}

type estadoEspacial struct {
	cartoes     []cardEspacial
	selecao     rectT
	arrastando  bool
	movendo     bool
	origemX     int32
	origemY     int32
	cardOrigem  rectT
	foco        int
	aberto      int
	editando    int
	textoEdicao string
	zoom        float64
	offsetX     int32
	offsetY     int32
	criacao     time.Time
}

//nolint:gochecknoglobals // Estado unico do canvas espacial: UI single-window.
var espCanvas estadoEspacial

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
		cartoes:     nil,
		selecao:     rectT{},
		arrastando:  false,
		movendo:     false,
		origemX:     0,
		origemY:     0,
		cardOrigem:  rectT{},
		foco:        -1,
		aberto:      -1,
		editando:    -1,
		textoEdicao: "",
		zoom:        1,
		offsetX:     0,
		offsetY:     0,
		criacao:     time.Time{},
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
	espCanvas.textoEdicao = ""
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

func espFinalizarEdicao() {
	indice := espCanvas.editando
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		espCanvas.editando = -1

		return
	}

	url := strings.TrimSpace(espCanvas.textoEdicao)
	espCanvas.cartoes[indice].url = url
	espCanvas.cartoes[indice].titulo = url
	espCanvas.editando = -1
	espCanvas.textoEdicao = ""
}

func espInserirCaractere(car rune) {
	espCanvas.textoEdicao += string(car)
}

func espApagarCaractere() {
	texto := []rune(espCanvas.textoEdicao)
	if len(texto) == 0 {
		return
	}

	espCanvas.textoEdicao = string(texto[:len(texto)-1])
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

func espMouseDown(app *app, x, y int32) {
	if app == nil {
		return
	}

	if espDentro(espBotaoSair(app), x, y) {
		pPostQuitMessage.Call(0)

		return
	}

	if espCanvas.editando >= 0 {
		espFinalizarEdicao()
		pInvalidateRect.Call(app.hwnd, 0, 1)
	}

	indice := espCartaoEm(x, y)
	if indice >= 0 {
		espCanvas.foco = indice

		if espDentro(espAreaURL(indice), x, y) {
			espCanvas.editando = indice
			espCanvas.textoEdicao = espCanvas.cartoes[indice].url

			pInvalidateRect.Call(app.hwnd, 0, 1)

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

func espTeclaChar(app *app, wparam uintptr) bool {
	if espCanvas.editando < 0 {
		return false
	}

	car := rune(wparam & espMascaraPalavra)
	if car >= espPrimeiroCar {
		espInserirCaractere(car)
	}

	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

func espTeclaEdicao(tecla uint32) bool {
	switch tecla {
	case vkReturn:
		espFinalizarEdicao()
	case vkBackspace:
		espApagarCaractere()
	case vkEscape:
		espCanvas.editando = -1
		espCanvas.textoEdicao = ""
	default:
		return false
	}

	return true
}

func espTecla(app *app, mensagem uint32, wparam uintptr) bool {
	if app == nil {
		return false
	}

	if mensagem == wmChar {
		return espTeclaChar(app, wparam)
	}

	if mensagem != wmKeyDown {
		return false
	}

	tecla := uint32(wparam & espMascaraPalavra)
	if espCanvas.editando >= 0 {
		if !espTeclaEdicao(tecla) {
			return false
		}

		pInvalidateRect.Call(app.hwnd, 0, 1)

		return true
	}

	if tecla == vkDelete {
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

	textoURL := espCanvas.cartoes[indice].url
	if indice == espCanvas.editando {
		textoURL = espCanvas.textoEdicao
	}

	if textoURL == "" {
		textoURL = espURLPlaceholder
	}

	areaTexto := rectT{
		areaURL.Left + espMargem, areaURL.Top,
		areaURL.Right - espMargem, areaURL.Bottom,
	}
	disp.textoUI(textoURL, areaTexto, espCorTitulo, true, false)
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
}
