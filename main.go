//go:build windows

// TESTE
package main

import "unsafe"

const (
	botaoLargura = 72
	botaoAltura  = 30
	botaoMargem  = 16
	passoZoom    = 120
	meio         = 2
)

type estado struct {
	api     *api
	motor   motorD2D
	canvas  canvas
	janela  uintptr
	cursor  uintptr
	botao   rectT
	centroX int32
	centroY int32
}

func main() {
	var motor motorD2D

	atual := &estado{
		api:     novaAPI(),
		motor:   motor,
		canvas:  novoCanvas(),
		janela:  0,
		cursor:  0,
		botao:   rectT{Left: 0, Top: 0, Right: 0, Bottom: 0},
		centroX: 0,
		centroY: 0,
	}

	if !criarJanela(atual) {
		return
	}

	loopMensagens(atual)
	finalizar(atual)
}

func criarJanela(atual *estado) bool {
	apiLocal := atual.api
	apiLocal.pSetProcessDPIAware.Call()

	area := apiLocal.areaVirtual()
	atual.centroX = (area.Right - area.Left) / meio
	atual.centroY = (area.Bottom - area.Top) / meio

	atual.botao = rectT{
		area.Right - botaoLargura - botaoMargem,
		area.Top + botaoMargem,
		area.Right - botaoMargem,
		area.Top + botaoMargem + botaoAltura,
	}

	nomeClasse := textoUTF16("OverdriveProtocolWnd")
	instancia := apiLocal.moduloHandle()
	cursor, _, _ := apiLocal.pLoadCursorW.Call(0, idiArrow)
	atual.cursor = cursor

	var classeVazia wndClassExW

	classe := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(classeVazia)),
		style:         0,
		lpfnWndProc:   apiLocal.pDefWindowProcW.Addr(),
		cbClsExtra:    0,
		cbWndExtra:    0,
		hInstance:     instancia,
		hIcon:         0,
		hCursor:       cursor,
		hbrBackground: 0,
		lpszMenuName:  nil,
		lpszClassName: nomeClasse,
		hIconSm:       0,
	}
	apiLocal.pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classe)))

	handle, _, _ := apiLocal.pCreateWindowExW.Call(
		wsExLayered|wsExAppwindow|wsExTopmost,
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(textoUTF16(""))),
		wsPopup,
		uintptr(area.Left), uintptr(area.Top),
		uintptr(area.Right-area.Left), uintptr(area.Bottom-area.Top),
		0, 0, instancia, 0,
	)
	if handle == 0 {
		return false
	}

	atual.janela = handle

	if !atual.motor.criar(apiLocal, handle, area.Right-area.Left, area.Bottom-area.Top) {
		return false
	}

	apiLocal.pShowWindow.Call(handle, swShow)
	redesenhar(atual, handle)

	return true
}

func redesenhar(atual *estado, handle uintptr) {
	atual.motor.iniciar()
	atual.canvas.pintar(&atual.motor, atual.centroX, atual.centroY, atual.botao)
	atual.motor.apresentar(atual.api, handle)
}

func coordenadas(lparam uintptr) (int32, int32) {
	x := int32(int16(lparam & mascaraPalavra))
	y := int32(int16((lparam >> deslocPalavra) & mascaraPalavra))

	return x, y
}

func (atual *estado) tratarMouseMove(mensagem *mensagemW) {
	x, y := coordenadas(mensagem.lParam)

	if !atual.canvas.arrastando {
		return
	}

	atual.canvas.mover(x-atual.canvas.ultX, y-atual.canvas.ultY)
	atual.canvas.ultX = x
	atual.canvas.ultY = y
	atual.canvas.movido = true
	redesenhar(atual, mensagem.hwnd)
}

func (atual *estado) tratarDown(mensagem *mensagemW) {
	x, y := coordenadas(mensagem.lParam)

	botao := atual.canvas.clique(x, y, atual.centroX, atual.centroY, atual.botao)
	if botao {
		atual.api.pPostQuitMessage.Call(0)

		return
	}

	atual.canvas.arrastando = true
	atual.canvas.movido = false
	atual.canvas.ultX = x
	atual.canvas.ultY = y
	atual.api.pSetCapture.Call(mensagem.hwnd)
}

func (atual *estado) tratarWheel(mensagem *mensagemW) {
	x, y := coordenadas(mensagem.lParam)
	delta := int32(int16(uint16(mensagem.wParam >> deslocPalavra)))

	fator := zoomPasso
	if delta < 0 {
		fator = 1 / zoomPasso
	}

	atual.canvas.aproximar(x, y, atual.centroX, atual.centroY, fator)
	redesenhar(atual, mensagem.hwnd)
}

func (atual *estado) processar(mensagem *mensagemW) {
	switch mensagem.message {
	case wmPaint:
		redesenhar(atual, mensagem.hwnd)
		atual.api.pValidateRect.Call(mensagem.hwnd, 0)
	case wmEraseBkgnd:
		return
	case wmSetCursor:
		atual.api.pSetCursor.Call(atual.cursor)
	case wmMouseMove:
		atual.tratarMouseMove(mensagem)
	case wmLButtonDown:
		atual.tratarDown(mensagem)
	case wmLButtonUp:
		atual.canvas.arrastando = false
		atual.api.pReleaseCapture.Call()
	case wmMouseWheel:
		atual.tratarWheel(mensagem)
	case wmClose, wmDestroy:
		atual.api.pPostQuitMessage.Call(0)
	}
}

func loopMensagens(atual *estado) {
	var mensagem mensagemW

	for {
		resultado, _, _ := atual.api.pGetMessageW.Call(ponteiroMensagem(&mensagem), 0, 0, 0)
		if int32(resultado) <= 0 {
			return
		}

		atual.processar(&mensagem)
	}
}

func finalizar(atual *estado) {
	atual.motor.libertar(atual.api)
}
