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
	espCorFaint       = 0xFF847B77
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

	// html.float do Marble: brilho radial suave no centro + halo de texto.
	// O fundo continua transparente; so o "ground" escuro e composto no meio.
	espCorGround   = 0x001E1B1A // --ground dark (#1a1b1e), AABBGGRR.
	espGlowCamadas = 6          // numero de elipses concentricas do gradiente.
	espGlowAlfa    = 0x29       // alpha por camada (acentua no centro).
	espHaloAlfa    = 0x28       // alpha do halo de legibilidade do texto.
	espHaloRaio1   = 2          // text-shadow: 0 0 2px.
	espHaloRaio2   = 6          // text-shadow: 0 0 6px.
	espHaloRaio3   = 12         // text-shadow: 0 0 12px.

	teclaTeclaT = 0x54
	teclaTeclaS = 0x53
	teclaTeclaF = 0x46
	teclaTeclaZ = 0x5A
	teclaTeclaY = 0x59
	teclaTeclaO = 0x4F
	teclaTeclaK = 0x4B
	teclaTeclaB = 0x42
	teclaTeclaN = 0x4E
	teclaTeclaW = 0x57
	vkF9        = 0x78
)

type cardEspacial struct {
	id        boardID
	rect      rectT
	titulo    string
	url       string
	cr        uint32
	ehNota    bool
	corNota   uint32
	textoNota string
	temSticky bool
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
	sliderVisivel    bool
	moveu            bool
	readMode         bool
	rectAntesRead    rectT
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
		sliderVisivel:    false,
	}

	boardIniciar()
	marbleIniciar()
	marbleCarregarBoard()
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
		id:     0,
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
		espCanvas.cartoes[indice].titulo = marbleNomeSite(url)
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
	espCanvas.cartoes[indice].titulo = marbleNomeSite(url)

	espOcultarEdit(app)

	espCanvas.editando = -1

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func espRemoverFoco() {
	indice := espCanvas.foco
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	idRemovido := espCanvas.cartoes[indice].id
	boardSincronizar()
	boardRemoverNo(&boardAtivo, idRemovido)

	espCanvas.cartoes = slices.Delete(espCanvas.cartoes, indice, indice+1)
	if espCanvas.aberto == indice {
		espCanvas.aberto = -1
	}

	if espCanvas.editando == indice {
		espCanvas.editando = -1
	}

	espCanvas.foco = -1
}

// espTextoHaloUI desenha texto com o text-shadow de 3 camadas do html.float.
func espTextoHaloUI(disp *d2dMotor, texto string, area rectT, cor uint32, pequeno, centro bool) {
	if theApp == nil {
		return
	}

	tamPx := theApp.sc(tamFonteD2D)
	if pequeno {
		tamPx = theApp.sc(tamFontePeqD2D)
	}

	alinhH := int32(0)
	if centro {
		alinhH = 1
	}

	disp.textoComHalo(texto, area, cor, espCorGround, "Segoe UI", tamPx, alinhH, 1, 1)
}

// espPintarBrilho reproduz o radial-gradient do html.float do Marble: um brilho
// suave da cor --ground no centro da janela. O fundo continua transparente.
func espPintarBrilho(disp *d2dMotor, app *app) {
	area := app.clientRect()
	if area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	centroX := float32(area.Left+area.Right) / meioD2D
	centroY := float32(area.Top+area.Bottom) / meioD2D
	raioX := float32(area.Width()) / meioD2D
	raioY := float32(area.Height()) / meioD2D

	// Da maior para a menor: o centro acumula camadas (mais opaco) e as bordas
	// ficam transparentes, equivalente ao closest-side 50% -> 100% do CSS.
	for camada := espGlowCamadas; camada >= 1; camada-- {
		fracao := float32(camada) / float32(espGlowCamadas)
		meiaLargura, meiaAltura := raioX*fracao, raioY*fracao
		disp.elipseAlfa(rectT{
			int32(centroX - meiaLargura), int32(centroY - meiaAltura),
			int32(centroX + meiaLargura), int32(centroY + meiaAltura),
		}, espCorGround, espGlowAlfa)
	}
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

	// Com wallpaper o board e opaco: sem composicao com o desktop (menos CPU).
	// Sem wallpaper, o alpha do slider controla a transparencia.
	alfa := espCanvas.alpha
	if marbleFundoFoto {
		alfa = espAlphaMax
	}

	pSetLayeredWindowAttributes.Call(app.hwnd, 0, uintptr(alfa), lwaAlpha)
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

	if marbleClique(app, x, y) {
		return
	}

	if espCanvas.sliderVisivel && espDentro(espSliderRect(app), x, y) {
		espCanvas.arrastandoSlider = true

		espAjustarAlphaSlider(app, x)

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
		espCanvas.moveu = false
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
		if dx*dx+dy*dy > 9 {
			espCanvas.moveu = true
		}
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
		if espCanvas.moveu {
			boardRegistrarMovimentoFoco()
		} else {
			espAbrirCard(app, espCanvas.foco)
		}

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
		focoAnterior := espCanvas.foco
		espAdicionar(retangulo)
		boardSincronizar()
		if focoAnterior >= 0 && focoAnterior < len(espCanvas.cartoes) {
			boardConectarFoco(espCanvas.cartoes[focoAnterior].id, espCanvas.cartoes[espCanvas.foco].id)
		}
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
	editando := espCanvas.editando
	ehNota := editando >= 0 && editando < len(espCanvas.cartoes) && espCanvas.cartoes[editando].ehNota

	switch uint32(wparam) {
	case vkReturn:
		if ehNota {
			marbleAplicarTextoNota(app, app.editText())
		} else {
			espEditAplicar(app, app.editText())
		}

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

	if marbleTecla(app, uint32(wparam&espMascaraPalavra), uint32(wparam)) {
		return true
	}

	tecla := uint32(wparam & espMascaraPalavra)
	if tecla == vkDelete && espCanvas.editando < 0 {
		espRemoverFoco()
		pInvalidateRect.Call(app.hwnd, 0, 1)

		return true
	}

	if espCanvas.editando >= 0 {
		return false
	}

	return espAtalhosBoard(app, tecla)
}

func espAtalhosBoard(app *app, tecla uint32) bool {
	ctrl := getKeyDown(vkControl)
	shift := getKeyDown(vkShift)

	if ctrl && shift && tecla == teclaTeclaO {
		espCanvas.sliderVisivel = !espCanvas.sliderVisivel
		pInvalidateRect.Call(app.hwnd, 0, 1)

		return true
	}

	if ctrl && shift && tecla == teclaTeclaB {
		marbleAlternarFundo(app)

		return true
	}

	if ctrl {
		return espAtalhosCtrl(app, tecla, shift)
	}

	if shift {
		return false
	}

	return espAtalhosSimples(app, tecla)
}

func espAtalhosCtrl(app *app, tecla uint32, shift bool) bool {
	switch {
	case tecla == vkTab && shift:
		return boardNavegar(app, true)
	case tecla == teclaTeclaZ:
		return boardDesfazer(app)
	case tecla == teclaTeclaY:
		return boardRefazer(app)
	}

	if tecla >= 0x31 && tecla <= 0x39 {
		marbleTrocarSpace(app, int(tecla-0x31))

		return true
	}

	return false
}

func espAtalhosSimples(app *app, tecla uint32) bool {
	switch tecla {
	case teclaTeclaT:
		return boardTidy()
	case vkTab:
		return boardNavegar(app, false)
	case teclaTeclaS:
		return boardDobrar()
	case teclaTeclaF:
		return boardAjustarView(app)
	case teclaTeclaN:
		marbleCriarNota(app)

		return true
	case teclaTeclaW:
		marbleNovaCard(app)

		return true
	case vkF9:
		marbleModoDemo(app)

		return true
	case vkReturn:
		marbleAlternarReadMode(app)

		return true
	}

	return false
}

func espAbrirCard(app *app, indice int) {
	if app == nil || indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	card := espCanvas.cartoes[indice]
	if card.url == "" {
		return
	}

	espCanvas.aberto = indice

	if app.left == nil || app.left.wv == nil || !app.left.ready {
		app.novaAba(card.url, true)

		return
	}

	app.left.wv.Navigate(card.url)
	espPosicionarWebView(app)
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

	textoURL := espCanvas.cartoes[indice].titulo
	if textoURL == "" {
		textoURL = espCanvas.cartoes[indice].url
	}
	if textoURL == "" {
		textoURL = espURLPlaceholder
	}

	areaTexto := rectT{
		areaURL.Left + espMargem, areaURL.Top,
		areaURL.Right - espMargem, areaURL.Bottom,
	}
	espTextoHaloUI(disp, textoURL, areaTexto, espCorDim, true, false)
}

func espPintar(disp *d2dMotor, app *app) {
	if disp == nil || app == nil {
		return
	}

	marblePintarWallpaper(disp, app)
	if !marbleFundoFoto {
		espPintarBrilho(disp, app)
	}
	marblePintarTrilhas(disp)

	for indice, card := range espCanvas.cartoes {
		if card.ehNota {
			continue
		}

		corBorda := uint32(espBordaCard)
		if indice == espCanvas.foco {
			corBorda = espBordaFoco
		}

		espPintarBarraURL(disp, indice)
		disp.redondo(card.rect, espCardRaio, espFundoCard, corBorda)

		if card.temSticky {
			lado := app.sc(24)
			sticky := rectT{
				card.rect.Right - lado - app.sc(4), card.rect.Bottom - lado - app.sc(4),
				card.rect.Right - app.sc(4), card.rect.Bottom - app.sc(4),
			}
			disp.ret(sticky, 0xFF8AE6FD)
		}
	}

	marblePintarNotas(disp, app)

	if espCanvas.selecao.Width() > 0 && espCanvas.selecao.Height() > 0 {
		disp.contorno(espCanvas.selecao, espCorSelecao)
	}

	if espCanvas.sliderVisivel {
		espPintarSlider(disp, app)
	}

	marblePintar(disp, app)
}
