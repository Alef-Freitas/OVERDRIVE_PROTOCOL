//go:build windows

package main

import (
	"slices"
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
	espHomeURL        = "https://www.google.com"
)

type cardEspacial struct {
	rect   rectT
	titulo string
	url    string
	cr     uint32
}

type estadoEspacial struct {
	cartoes    []cardEspacial
	selecao    rectT
	arrastando bool
	movendo    bool
	origemX    int32
	origemY    int32
	cardOrigem rectT
	foco       int
	aberto     int
	zoom       float64
	offsetX    int32
	offsetY    int32
	criacao    time.Time
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
		cartoes:    nil,
		selecao:    rectT{},
		arrastando: false,
		movendo:    false,
		origemX:    0,
		origemY:    0,
		cardOrigem: rectT{},
		foco:       -1,
		aberto:     -1,
		zoom:       1,
		offsetX:    0,
		offsetY:    0,
		criacao:    time.Time{},
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

func espAdicionar(retangulo rectT, url string) {
	if !espRectValido(retangulo) {
		return
	}

	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		rect:   retangulo,
		titulo: url,
		url:    url,
		cr:     espBordaCard,
	})
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

	indice := espCartaoEm(x, y)
	if indice >= 0 {
		espCanvas.foco = indice
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

		if !espCanvas.arrastando && espCanvas.foco >= 0 {
			espAbrirCard(app, espCanvas.foco)
		}

		return
	}

	if !espCanvas.arrastando {
		return
	}

	espCanvas.arrastando = false

	pReleaseCapture.Call()

	retangulo := espNormalizar(espCanvas.origemX, espCanvas.origemY, x, y)
	if espRectValido(retangulo) {
		espAdicionar(retangulo, espHomeURL)
	}

	espCanvas.selecao = rectT{}

	pInvalidateRect.Call(app.hwnd, 0, 1)
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

func espPintar(disp *d2dMotor, app *app) {
	if disp == nil || app == nil {
		return
	}

	for indice, card := range espCanvas.cartoes {
		corBorda := uint32(espBordaCard)
		if indice == espCanvas.foco {
			corBorda = espBordaFoco
		}

		disp.redondo(card.rect, espCardRaio, espFundoCard, corBorda)

		titulo := rectT{
			card.rect.Left + espMargem,
			card.rect.Top + espMargem,
			card.rect.Right - espMargem,
			card.rect.Top + espTituloAlt,
		}
		disp.textoUI(card.titulo, titulo, espCorTitulo, true, false)
	}

	if espCanvas.selecao.Width() > 0 && espCanvas.selecao.Height() > 0 {
		disp.contorno(espCanvas.selecao, espCorSelecao)
	}
}
