//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79

	wmRButtonDown    = 0x0204
	wmCaptureChanged = 0x00A2

	cfDib = 8

	recorteClasse = "OverdriveRecorteWnd"

	tamanhoInicialRecorte = 100
	qtdBotoesRecorte      = 4
	intervalosRecorte     = 3
	botaoRecorteSalvar    = 2
	botaoRecorteFechar    = 3
	teclaRecorteCopiar    = 0x43
	teclaRecorteOCR       = 0x4F
	teclaRecorteSalvar    = 0x53
	bytesPorPixelCaptura  = 4
	margemRecorte         = 8
	passoExpansaoRecorte  = 40
	tamanhoMinimoDIB      = 40
	minLadoRecorte        = 4

	flagExploradorRecorte = 0x00080000
	flagOcultarSomenteLe  = 0x00000002
	flagSobrescrever      = 0x00000004
	flagCaminhoAbsoluto   = 0x00000800
	flagNaoMudarDir       = 0x00000008
)

//nolint:gochecknoglobals // Procs user32/comdlg32 + ponto de entrada: handles fixos do processo.
var (
	pGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	comdlg32             = windows.NewLazySystemDLL("comdlg32.dll")
	pGetSaveFileNameW    = comdlg32.NewProc("GetSaveFileNameW")
	recorteProcPtr       uintptr
)

type recorteEstado struct {
	hwnd       uintptr
	ativo      bool
	vx, vy     int32
	w, h       int32
	origPix    []byte
	sel        rectT
	arrastando bool
	ix, iy     int32
	escolhendo bool
	hoverBtn   int
	barra      rectT
	botoes     [4]rectT
}

//nolint:gochecknoglobals // Estado do recorte em voo: app single-window.
var recorte recorteEstado

func (a *app) iniciarRecorteComTimer(segundos int) {
	if segundos <= 0 {
		a.iniciarRecorte()

		return
	}

	if recorte.ativo {
		a.dizerf(txtPrintAberto)

		return
	}

	a.recorteTimerGen++
	gen := a.recorteTimerGen
	a.dizerProgressof(0, txtFmtPrintEm, segundos)

	for i := segundos - 1; i >= 1; i-- {
		restam := i
		espera := time.Duration(segundos-i) * time.Second
		time.AfterFunc(espera, func() {
			a.depois(func() {
				if gen != a.recorteTimerGen {
					return
				}

				a.dizerProgressof((segundos-restam)*100/segundos, txtFmtPrintEmX, restam)
			})
		})
	}

	time.AfterFunc(time.Duration(segundos)*time.Second, func() {
		a.depois(func() {
			if gen != a.recorteTimerGen {
				return
			}

			a.recorteTimerGen = 0
			a.iniciarRecorte()
		})
	})
}

func (a *app) cancelarRecorteTimer() bool {
	if a.recorteTimerGen == 0 || recorte.ativo {
		return false
	}

	a.recorteTimerGen++
	a.dizerf(txtPrintCancelado)
	a.invalidateBar()

	return true
}

func (a *app) iniciarRecorte() {
	if a == nil || a.hwnd == 0 || recorte.ativo {
		return
	}

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		a.dizerf(txtPrintRestaure)

		return
	}

	if err := recortePreparar(); err != nil {
		a.dizerf("%s", txtPrintPrefix+err.Error())

		return
	}

	if recorteCriarJanela() == 0 {
		recorteLiberar()
		a.dizerf(txtPrintSemSelecao)

		return
	}

	recorte.ativo = true
	recorte.arrastando = false
	recorte.escolhendo = false
	recorte.hoverBtn = -1
	recorte.sel = rectT{}
	pSetWindowPos.Call(recorte.hwnd, hwndTopmost,
		uintptr(uint32(recorte.vx)), uintptr(uint32(recorte.vy)),
		uintptr(recorte.w), uintptr(recorte.h),
		swpShowWindow|swpNoActivate)
	pShowWindow.Call(recorte.hwnd, swShow)
	pSetForegroundWindow.Call(recorte.hwnd)
	pSetFocus.Call(recorte.hwnd)
	pSetCursor.Call(loadCursor(idcCross))
}

func recortePreparar() error {
	pix, origemX, origemY, largura, altura, err := fotografarTela()
	if err != nil {
		return err
	}

	if contarPixelsAcesos(pix) == 0 {
		return erroTexto("não consegui capturar a tela")
	}

	recorte.vx, recorte.vy = origemX, origemY
	recorte.w, recorte.h = largura, altura
	recorte.origPix = pix

	return nil
}

func recorteCriarJanela() uintptr {
	if recorteProcPtr == 0 {
		return 0
	}

	if recorte.hwnd != 0 {
		return recorte.hwnd
	}

	hInst := getModuleHandle()
	nomeClasse := utf16(recorteClasse)
	classeJanela := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   recorteProcPtr,
		hInstance:     hInst,
		hCursor:       loadCursor(idcCross),
		hbrBackground: 0,
		lpszClassName: nomeClasse,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classeJanela)))
	handle, _, _ := pCreateWindowExW.Call(
		uintptr(wsExTopmost|wsExToolWindow),
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(utf16(txtTituloPrint))),
		wsPopup,
		0, 0, tamanhoInicialRecorte, tamanhoInicialRecorte, 0, 0, hInst, 0,
	)
	recorte.hwnd = handle

	return handle
}

func recorteProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmPaint:
		return recortePintar(hwnd)
	case wmEraseBkgnd:
		return 1
	case wmLButtonDown:
		return recorteBotaoBaixo(hwnd, lparam)
	case wmMouseMove:
		return recorteMover(hwnd, lparam)
	case wmLButtonUp:
		return recorteBotaoCima(hwnd, lparam)
	case wmKeyDown:
		return recorteTecla(wparam)
	case wmRButtonDown:
		encerrarRecorte()

		return 0
	case wmSetCursor:
		return recorteCursor(hwnd)
	case wmCaptureChanged:
		return recorteCapturaPerdida()
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

// recorteCapturaPerdida encerra o arrasto se a captura do mouse for perdida.
func recorteCapturaPerdida() uintptr {
	if recorte.ativo && recorte.arrastando {
		recorte.arrastando = false

		encerrarRecorte()
	}

	return 0
}

// recortePintar desenha a captura e a barra de acoes.
func recortePintar(hwnd uintptr) uintptr {
	if theApp != nil && recorte.ativo && len(recorte.origPix) > 0 {
		if d2dRec.rt == nil {
			d2dRec.criar(hwnd, recorte.w, recorte.h)
		}

		if d2dRec.rt != nil {
			d2dRec.iniciar()
			theApp.paintRecorteD2D()
			d2dRec.apresentar()
		} else {
			pValidateRect.Call(hwnd, 0)
		}

		return 0
	}

	pValidateRect.Call(hwnd, 0)

	return 0
}

// recorteBotaoBaixo inicia o arrasto ou dispara o botao da barra.
func recorteBotaoBaixo(hwnd uintptr, lparam uintptr) uintptr {
	if !recorte.ativo {
		return 0
	}

	x, y := recorteClamp(loword(lparam), hiword(lparam))
	if recorte.escolhendo {
		if recorteDispararBotao(x, y) {
			return 0
		}

		recorte.escolhendo = false
		recorte.hoverBtn = -1
	}

	recorte.arrastando = true
	recorte.ix, recorte.iy = x, y
	recorte.sel = rectT{x, y, x, y}

	pSetCapture.Call(hwnd)
	pInvalidateRect.Call(hwnd, 0, 0)

	return 0
}

// recorteDispararBotao executa o botao da barra sob o clique; true = tratado.
func recorteDispararBotao(x, y int32) bool {
	switch recorteBotaoEm(x, y) {
	case 0:
		recorteCopiar()

		return true
	case 1:
		recorteOCR()

		return true
	case botaoRecorteSalvar:
		recorteSalvarComo()

		return true
	case botaoRecorteFechar:
		encerrarRecorte()

		return true
	}

	return false
}

// recorteMover atualiza o hover da barra ou o retangulo arrastado.
func recorteMover(hwnd uintptr, lparam uintptr) uintptr {
	if !recorte.ativo {
		return 0
	}

	x, y := recorteClamp(loword(lparam), hiword(lparam))
	if recorte.escolhendo {
		recorteAtualizarHover(hwnd, x, y)

		return 0
	}

	if recorte.arrastando {
		antigo := recorte.sel
		recorte.sel = retanguloDePontos(recorte.ix, recorte.iy, x, y)
		recorteInvalidar(antigo, recorte.sel)
	}

	return 0
}

// recorteAtualizarHover repinta a barra e troca o cursor conforme o botao.
func recorteAtualizarHover(hwnd uintptr, x, y int32) {
	botao := recorteBotaoEm(x, y)
	if botao != recorte.hoverBtn {
		recorte.hoverBtn = botao

		area := recorte.barra
		if area.Width() > 0 {
			pInvalidateRect.Call(hwnd, uintptr(unsafe.Pointer(&area)), 0)
		}
	}

	if botao >= 0 {
		pSetCursor.Call(loadCursor(idcHand))
	} else {
		pSetCursor.Call(loadCursor(idcArrow))
	}
}

// recorteBotaoCima fecha a selecao ou encerra se for pequena demais.
func recorteBotaoCima(hwnd uintptr, lparam uintptr) uintptr {
	if recorte.ativo && recorte.arrastando {
		recorte.arrastando = false

		pReleaseCapture.Call()

		x, y := recorteClamp(loword(lparam), hiword(lparam))

		recorte.sel = retanguloDePontos(recorte.ix, recorte.iy, x, y)
		if recorte.sel.Width() < minLadoRecorte || recorte.sel.Height() < minLadoRecorte {
			encerrarRecorte()

			return 0
		}

		recorte.escolhendo = true
		recorte.hoverBtn = -1

		recortePosBarra()
		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

// recorteTecla trata Esc/Enter e atalhos C/O/S com a barra visivel.
func recorteTecla(wparam uintptr) uintptr {
	if !recorte.ativo {
		return 0
	}

	if recorte.escolhendo {
		switch wparam {
		case vkEscape:
			encerrarRecorte()
		case vkReturn, teclaRecorteCopiar:
			recorteCopiar()
		case teclaRecorteOCR:
			recorteOCR()
		case teclaRecorteSalvar:
			recorteSalvarComo()
		}

		return 0
	}

	if wparam == vkEscape {
		encerrarRecorte()
	}

	return 0
}

// recorteCursor mostra a mao sobre os botoes e a mira no resto.
func recorteCursor(hwnd uintptr) uintptr {
	if recorte.escolhendo {
		var pt struct{ x, y int32 }
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))

		if recorteBotaoEm(pt.x, pt.y) >= 0 {
			pSetCursor.Call(loadCursor(idcHand))

			return 1
		}
	}

	pSetCursor.Call(loadCursor(idcCross))

	return 1
}

func recortePosBarra() {
	aplicativo := theApp
	if aplicativo == nil {
		return
	}

	selecao := recorte.sel
	larguraBotao := aplicativo.sc(104)
	alt := aplicativo.sc(34)
	gap := aplicativo.sc(8)
	total := larguraBotao*qtdBotoesRecorte + gap*intervalosRecorte

	x := selecao.Right - total
	if x < aplicativo.sc(8) {
		x = aplicativo.sc(8)
	}

	if x+total > recorte.w-aplicativo.sc(8) {
		x = recorte.w - total - aplicativo.sc(8)
	}

	if x < aplicativo.sc(8) {
		x = aplicativo.sc(8)
	}

	y := selecao.Bottom + aplicativo.sc(14)
	if y+alt+aplicativo.sc(38) > recorte.h {
		y = selecao.Top - alt - aplicativo.sc(38)
	}

	if y < aplicativo.sc(8) {
		y = aplicativo.sc(8)
	}

	recorte.barra = rectT{
		x - aplicativo.sc(10), y - aplicativo.sc(10), x + total + aplicativo.sc(10), y + alt + aplicativo.sc(30),
	}
	for i := range qtdBotoesRecorte {
		bx := x + int32(i)*(larguraBotao+gap)
		recorte.botoes[i] = rectT{bx, y, bx + larguraBotao, y + alt}
	}
}

func recorteBotaoEm(x, y int32) int {
	for i, botao := range recorte.botoes {
		if x >= botao.Left && x < botao.Right && y >= botao.Top && y < botao.Bottom {
			return i
		}
	}

	return -1
}

func recorteClamp(x, y int32) (int32, int32) {
	if x < 0 {
		x = 0
	}

	if y < 0 {
		y = 0
	}

	if x > recorte.w {
		x = recorte.w
	}

	if y > recorte.h {
		y = recorte.h
	}

	return x, y
}

func retanguloDePontos(xInicio, yInicio, xFim, yFim int32) rectT {
	if xInicio > xFim {
		xInicio, xFim = xFim, xInicio
	}

	if yInicio > yFim {
		yInicio, yFim = yFim, yInicio
	}

	return rectT{xInicio, yInicio, xFim, yFim}
}

func recorteInvalidar(antigo, novo rectT) {
	area := recorteUnir(antigo, novo)
	area = recorteExpandir(area)

	pInvalidateRect.Call(recorte.hwnd, uintptr(unsafe.Pointer(&area)), 0)
}

// recorteUnir une o retangulo antigo ao novo para repintar a area afetada.
func recorteUnir(antigo, novo rectT) rectT {
	area := antigo
	if novo.Width() <= 0 && novo.Height() <= 0 {
		return area
	}

	if area.Width() <= 1 && area.Height() <= 1 {
		return novo
	}

	if novo.Left < area.Left {
		area.Left = novo.Left
	}

	if novo.Top < area.Top {
		area.Top = novo.Top
	}

	if novo.Right > area.Right {
		area.Right = novo.Right
	}

	if novo.Bottom > area.Bottom {
		area.Bottom = novo.Bottom
	}

	return area
}

// recorteExpandir adiciona margem e prende o retangulo a tela.
func recorteExpandir(area rectT) rectT {
	passo := int32(passoExpansaoRecorte)
	if theApp != nil {
		passo = theApp.sc(passoExpansaoRecorte)
	}

	area.Left -= margemRecorte
	area.Top -= passo
	area.Right += margemRecorte
	area.Bottom += margemRecorte

	if area.Left < 0 {
		area.Left = 0
	}

	if area.Top < 0 {
		area.Top = 0
	}

	if area.Right > recorte.w {
		area.Right = recorte.w
	}

	if area.Bottom > recorte.h {
		area.Bottom = recorte.h
	}

	return area
}

// segundosTimerPrint extrai os segundos do id printN; -1 = invalido.
func segundosTimerPrint(id string) int {
	if len(id) != len(idImprimir)+1 {
		return -1
	}

	for i := range len(idImprimir) {
		if id[i] != idImprimir[i] {
			return -1
		}
	}

	segundos := int(id[len(id)-1] - '0')
	if segundos < 1 || segundos > 5 {
		return -1
	}

	return segundos
}

// pixelsCapturaExcedem barra telas virtuais absurdas antes do make([]byte).
func pixelsCapturaExcedem(largura, altura int32) bool {
	if largura <= 0 || altura <= 0 {
		return true
	}

	const maxPixelsCaptura = int64(7680 * 4320)

	return int64(largura)*int64(altura) > maxPixelsCaptura
}

func recortePixels() ([]byte, int32, int32, error) {
	selecao := recorte.sel
	if selecao.Width() < 1 || selecao.Height() < 1 {
		return nil, 0, 0, erroTexto("área vazia")
	}

	if pixelsCapturaExcedem(selecao.Width(), selecao.Height()) {
		return nil, 0, 0, erroTexto(txtFalhaLerPrint)
	}

	if len(recorte.origPix) < int(recorte.w)*int(recorte.h)*bytesPorPixelCaptura {
		return nil, 0, 0, erroTexto(txtFalhaLerPrint)
	}

	buf := make([]byte, int(selecao.Width())*int(selecao.Height())*bytesPorPixelCaptura)
	for y := range int(selecao.Height()) {
		src := ((int(selecao.Top)+y)*int(recorte.w) + int(selecao.Left)) * bytesPorPixelCaptura
		dst := y * int(selecao.Width()) * bytesPorPixelCaptura
		copy(
			buf[dst:dst+int(selecao.Width())*bytesPorPixelCaptura],
			recorte.origPix[src:src+int(selecao.Width())*bytesPorPixelCaptura],
		)
	}

	return buf, selecao.Width(), selecao.Height(), nil
}

func recorteCopiar() {
	buf, largura, altura, err := recortePixels()
	if err != nil {
		if theApp != nil {
			theApp.dizerf("%s", txtPrintPrefix+err.Error())
		}

		return
	}

	linha := int(largura) * bytesPorPixelCaptura

	invertido := make([]byte, len(buf))
	for y := range int(altura) {
		copy(invertido[y*linha:(y+1)*linha], buf[(int(altura)-1-y)*linha:(int(altura)-y)*linha])
	}

	infoBitmap := bitmapInfo{}
	infoBitmap.bmiHeader.biSize = uint32(unsafe.Sizeof(bitmapInfoHeader{}))
	infoBitmap.bmiHeader.biWidth = largura
	infoBitmap.bmiHeader.biHeight = altura
	infoBitmap.bmiHeader.biPlanes = 1
	infoBitmap.bmiHeader.biBitCount = 32
	infoBitmap.bmiHeader.biCompression = biRGB
	infoBitmap.bmiHeader.biSizeImage = uint32(len(invertido))
	hdr := unsafe.Slice((*byte)(unsafe.Pointer(&infoBitmap.bmiHeader)), int(unsafe.Sizeof(bitmapInfoHeader{})))
	dib := make([]byte, len(hdr)+len(invertido))
	copy(dib, hdr)
	copy(dib[len(hdr):], invertido)
	encerrarRecorte()

	if err := clipboardGravarDIB(dib); err != nil {
		if theApp != nil {
			theApp.dizerf("%s", txtPrintPrefix+err.Error())
		}

		return
	}

	if theApp != nil {
		theApp.dizerf(txtFmtPrintCopiado, largura, altura)
	}
}

func recorteOCR() {
	buf, largura, altura, err := recortePixels()
	if err != nil {
		if theApp != nil {
			theApp.dizerf("%s", txtOCRPrefix+err.Error())
		}

		return
	}

	encerrarRecorte()

	if theApp != nil {
		theApp.dizerf(txtOCRLendo)
	}

	go func() {
		txt, err := ocrTexto(buf, largura, altura)

		aplicativo := theApp
		if aplicativo == nil {
			return
		}

		aplicativo.depois(func() {
			if err != nil {
				aplicativo.dizerf("%s", txtOCRPrefix+err.Error())

				return
			}

			if err := clipboardGravarTexto(txt); err != nil {
				aplicativo.dizerf("%s", txtOCRNaoCopiou+err.Error())

				return
			}

			aplicativo.dizerf(txtFmtOCRCopiado, len([]rune(txt)))
		})
	}()
}

func recorteSalvarComo() {
	buf, largura, altura, err := recortePixels()
	if err != nil {
		if theApp != nil {
			theApp.dizerf("%s", txtPrintPrefix+err.Error())
		}

		return
	}

	caminho, ok := pedirCaminhoPNG(recorte.hwnd)
	if !ok {
		return
	}

	img := bgraParaNRGBA(buf, int(largura), int(altura))
	if err := gravarPNG(caminho, img); err != nil {
		if theApp != nil {
			theApp.dizerf("%s", txtPrintPrefix+err.Error())
		}

		return
	}

	encerrarRecorte()

	if theApp != nil {
		theApp.dizerf(txtFmtPrintSalvoEm, caminho)
	}
}

func clipboardGravarDIB(dib []byte) error {
	if len(dib) < tamanhoMinimoDIB {
		return erroTexto("DIB invalido")
	}

	if aberto, _, _ := pOpenClipboard.Call(0); aberto == 0 {
		return erroTexto("área de transferência ocupada")
	}
	defer pCloseClipboard.Call()

	pEmptyClipboard.Call()

	handle, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(len(dib)))
	if handle == 0 {
		return erroTexto("sem memória para copiar")
	}

	ptr, _, _ := pGlobalLock.Call(handle)
	if ptr == 0 {
		pGlobalFree.Call(handle)

		return erroTexto("falha ao copiar")
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(dib))
	copy(dst, dib)
	pGlobalUnlock.Call(handle)

	if colocado, _, _ := pSetClipboardData.Call(cfDib, handle); colocado == 0 {
		pGlobalFree.Call(handle)

		return erroTexto("falha ao copiar")
	}

	return nil
}

func bgraParaNRGBA(buf []byte, largura, altura int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, largura, altura))
	if largura <= 0 || altura <= 0 {
		return img
	}

	if len(buf) < largura*altura*bytesPorPixelCaptura {
		return img
	}

	for y := range altura {
		src := buf[y*largura*bytesPorPixelCaptura : (y+1)*largura*bytesPorPixelCaptura]

		dst := img.Pix[y*img.Stride : y*img.Stride+largura*bytesPorPixelCaptura]
		for x := range largura {
			dst[x*bytesPorPixelCaptura+0] = src[x*bytesPorPixelCaptura+2]
			dst[x*bytesPorPixelCaptura+1] = src[x*bytesPorPixelCaptura+1]
			dst[x*bytesPorPixelCaptura+2] = src[x*bytesPorPixelCaptura+0]
			dst[x*bytesPorPixelCaptura+3] = 255
		}
	}

	return img
}

func gravarPNG(caminho string, img image.Image) error {
	// GOD: caminho escolhido pelo usuario no dialogo Salvar; por desenho.
	arquivo, err := os.Create(caminho) // #nosec G304
	if err != nil {
		return erroTextoF("criar imagem: %w", err)
	}

	//exhaustruct:ignore // BufferPool padrao da stdlib: zero-value intencional.
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(arquivo, img); err != nil {
		arquivo.Close()

		return erroTextoF("codificar PNG: %w", err)
	}

	if err := arquivo.Close(); err != nil {
		return erroTextoF("fechar imagem: %w", err)
	}

	return nil
}

//exhaustruct:ignore // Espelho da API Win32 (OPENFILENAMEW): campos nao usados ficam zero.
type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}

func pastaImagensPNG() string {
	casa, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	p := filepath.Join(casa, "Pictures")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return p
	}

	return casa
}

func pedirCaminhoSalvar(dono uintptr, titulo, nome, filtro, ext, pastaInicial string) (string, bool) {
	var buf [1024]uint16

	if len(nome) > 0 {
		if textoUtf16, err := windows.UTF16FromString(nome); err == nil {
			copy(buf[:], textoUtf16)
		}
	}

	var dir *uint16

	if pastaInicial != "" {
		if st, err := os.Stat(pastaInicial); err == nil && st.IsDir() {
			dir = utf16(pastaInicial)
		}
	}

	ofn := openFileNameW{
		lStructSize:     uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:       dono,
		lpstrFilter:     utf16(filtro),
		nFilterIndex:    1,
		lpstrFile:       &buf[0],
		nMaxFile:        uint32(len(buf)),
		lpstrInitialDir: dir,
		lpstrTitle:      utf16(titulo),
		flags: flagExploradorRecorte | flagOcultarSomenteLe | flagSobrescrever |
			flagCaminhoAbsoluto | flagNaoMudarDir,
		lpstrDefExt: utf16(ext),
	}

	ret, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", false
	}

	return windows.UTF16ToString(buf[:]), true
}

func pedirCaminhoPNG(dono uintptr) (string, bool) {
	nome := fmt.Sprintf("print-%s.png", time.Now().Format("20060102-150405"))
	filtro := "Imagens PNG (*.png)\x00*.png\x00Todos os arquivos (*.*)\x00*.*\x00\x00"

	return pedirCaminhoSalvar(dono, txtTituloSalvarPrint, nome, filtro, "png", pastaImagensPNG())
}

func encerrarRecorte() {
	if !recorte.ativo && !recorte.arrastando {
		return
	}

	recorte.ativo = false
	recorte.arrastando = false
	recorte.escolhendo = false
	recorte.hoverBtn = -1
	recorte.barra = rectT{}
	recorte.sel = rectT{}

	pReleaseCapture.Call()

	if recorte.hwnd != 0 {
		pShowWindow.Call(recorte.hwnd, swHide)
	}

	recorteLiberar()

	if theApp != nil && theApp.hwnd != 0 {
		pSetForegroundWindow.Call(theApp.hwnd)
		theApp.invalidateBar()
	}
}

func recorteLiberar() {
	recorte.origPix = nil

	d2dRec.soltar()

	if d2dRecOrig != nil {
		d2dRecOrig.Release()
		d2dRecOrig = nil
	}
}
