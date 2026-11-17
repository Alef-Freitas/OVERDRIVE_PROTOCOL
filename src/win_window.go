//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

// Constantes locais da janela (tempos, fatores e codigos Win32 sem outro dono).
const (
	intervaloFaseMs          = 200
	fasesCarregamento        = 3
	esperaDescarregarOcultos = 10 * time.Minute
	esperaDescartarAbas      = 90 * time.Second
	esperaDescarregarFrios   = 30 * time.Minute
	modControleJanela        = 0x0002
	modAltJanela             = 0x0004
	wmCapturaPerdida         = 0x0215
	fonteUITamanho           = 13
	fontePequenaTamanho      = 11
	divisorMetade            = 2
	dobroMargemFaixa         = 2
	limiteRotuloBotao        = 2
	larguraExtraRotulo       = 14
	preferenciaCantoDWM      = 2
	tamanhoAtributoDWM       = 4
	estiloDuploClique        = 0x0008
	margemLimiteJanela       = 8
	intervaloHoverMs         = 250
	intervaloChatOciosoMs    = 2000
	tamanhoInicialCampo      = 10
	msgDefinirFonte          = 0x0030
	msgDefinirMargens        = 0x00D3
	ambasMargensCampo        = 3
	dpiPadraoJanela          = 96
)

func defWindowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wp, lp)

	return r
}

// ecoSuspender diz se a economia maxima permite suspender paineis agora.
func (a *app) ecoSuspender() bool {
	return a.st.EconomiaMax && !a.tudoSuspenso
}

func (a *app) retomarFriosVisiveis(hwnd uintptr) {
	if !a.janelaAtiva {
		return
	}

	if ic, _, _ := pIsIconic.Call(hwnd); ic == 0 {
		a.retomarFriosSobCursor()
	}
}

func (a *app) piscarFaseCarregamento() {
	if !a.loading {
		a.loadPhase = 0

		return
	}

	fase := int32(time.Now().UnixMilli()/intervaloFaseMs) % fasesCarregamento
	if fase == a.loadPhase {
		return
	}

	a.loadPhase = fase
	r := rectT{a.addrFrame.Left, a.addrFrame.Bottom - a.sc(4), a.addrFrame.Right, a.addrFrame.Bottom}
	pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
}

func (a *app) cuidarPassagemCursor(hwnd uintptr) {
	a.retomarFriosVisiveis(hwnd)
	a.piscarFaseCarregamento()
}

func (a *app) painelImagensOcioso() bool {
	return a.st.RightVisible && a.st.ImgVisible && a.imgPanel != nil &&
		a.imgPanel.ready && a.imgPanel.wv != nil && !a.imgInteragido.IsZero() &&
		time.Since(a.imgInteragido) > 5*time.Minute && a.focus != idImg && !a.cursorNoPainelImagens()
}

func (a *app) cuidarImagensOciosas() {
	if !a.ecoSuspender() || !a.painelImagensOcioso() {
		return
	}

	a.fecharPainelImagens()
	a.st.ImgVisible = false
	a.layout()
	a.persist()
}

func (a *app) cuidarChatOcioso() {
	if !a.ecoSuspender() || !a.chatUsado || a.chatFrio || a.chatGerando || a.chatAtivo ||
		a.focus == idRight || a.cursorNoPainelChat() {
		return
	}

	a.agendarCongelamentoChatGPT()
}

func (a *app) pararCarregamentoTravado() {
	if a.loadStart.IsZero() {
		a.loadStart = time.Now()

		return
	}

	if time.Since(a.loadStart) <= 45*time.Second {
		return
	}

	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.Stop()
	}

	a.loading = false
	a.loadStart = time.Time{}
	a.invalidateBar()
}

func (a *app) cuidarCarregamentoOcioso() {
	if !a.loading {
		if !a.loadStart.IsZero() {
			a.loadStart = time.Time{}
		}

		return
	}

	a.pararCarregamentoTravado()
}

func (a *app) cuidarVarreduraOciosa() {
	// Varreduras pesadas alternadas entre ticks para nao estourar o
	// orcamento do WM_TIMER. Ficam na thread da UI porque o
	// WebView2 e STA (CDP/navegacao exigem ela).
	if a.tickOcioso%2 == 0 {
		a.gerenciarFrios()
		a.descarregarOcultos(esperaDescarregarOcultos)
	} else {
		a.tickPesado++
		if a.st.EconomiaMax && a.tickPesado%3 == 0 {
			a.descartarAbasInativas(esperaDescartarAbas)
			a.descarregarFrios(esperaDescarregarFrios)
		}
	}

	a.tickOcioso++
}

func (a *app) cuidarLembreteAgua() {
	if a.ultimaAgua.IsZero() {
		a.ultimaAgua = time.Now()
	} else if time.Since(a.ultimaAgua) >= lembreteAguaCada {
		a.ultimaAgua = time.Now()
		a.dizerf(txtAgua)
	}
}

func (a *app) cuidarChatOciosoTick() {
	if a.atualizarRelogio() {
		a.invalidateFaixa()
	}

	a.cuidarLembreteAgua()
	a.cuidarImagensOciosas()
	a.cuidarChatOcioso()
	a.cuidarCarregamentoOcioso()
	a.retentarAbaTravada()
	a.vigiarAbaAtiva()
	a.cuidarVarreduraOciosa()
}

func (a *app) cuidarTimerChatOcioso() {
	a.cuidarChatOciosoTick()
}

func (a *app) soltarArrastoAba() bool {
	if a.tabDrag < 0 {
		return false
	}

	origem, para, mover := a.tabDrag, a.tabDrop, a.tabDragAtivo
	a.tabDrag = -1
	a.tabDragAtivo = false
	a.tabDrop = -1

	pReleaseCapture.Call()

	if mover && origem >= 0 && origem < len(a.tabs) && para >= 0 && para <= len(a.tabs) {
		a.moverAba(origem, para)
	}

	a.invalidateFaixa()

	return true
}

func (a *app) soltarArrastoDivisores() {
	switch {
	case a.dragLeftAuxSplit:
		a.dragLeftAuxSplit = false

		pReleaseCapture.Call()
		a.layout()
		a.raisePanes()
		a.persist()
		pInvalidateRect.Call(a.hwnd, 0, 0)
	case a.dragImgSplit:
		a.dragImgSplit = false

		pReleaseCapture.Call()

		if a.right != nil && a.right.ready && a.right.mobile {
			a.aplicarTamanhoChat(a.right)
		}

		a.aplicarTamanhoImagens()
		a.raisePanes()
		a.persist()
		pInvalidateRect.Call(a.hwnd, 0, 0)
	case a.dragSplit:
		a.dragSplit = false

		pReleaseCapture.Call()
		a.raisePanes()

		if a.right != nil && a.right.ready && a.right.mobile {
			a.aplicarTamanhoChat(a.right)
		}

		a.splitXAntes = 0
		pInvalidateRect.Call(a.hwnd, 0, 0)
		a.persist()
	}
}

func (a *app) cursorSobreBotao(pt struct{ x, y int32 }) bool {
	return a.overLeftAuxBtn(pt.x, pt.y) || a.overLeftAuxFullBtn(pt.x, pt.y) || a.overSplitButton(pt.x, pt.y) ||
		a.overChatHomeButton(pt.x, pt.y) || a.overImgToggle(pt.x, pt.y) ||
		(pt.y >= a.tabH && pt.y < a.tabH+a.tbHeight && a.hitButton(pt.x, pt.y) != "")
}

func (a *app) cuidarCursorJanela(hwnd uintptr, lparam uintptr) (uintptr, bool) {
	if loword(lparam) != htClient {
		return 0, false
	}

	var ponto struct{ x, y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&ponto)))
	pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&ponto)))

	if a.cursorSobreBotao(ponto) {
		pSetCursor.Call(loadCursor(idcHand))

		return 1, true
	}

	if a.overLeftAuxSplit(ponto.x, ponto.y) || a.dragLeftAuxSplit {
		pSetCursor.Call(loadCursor(idcSizeWE))

		return 1, true
	}

	if a.overSplit(ponto.x, ponto.y) || a.dragSplit {
		pSetCursor.Call(loadCursor(idcSizeWE))

		return 1, true
	}

	if a.overImgSplit(ponto.x, ponto.y) || a.dragImgSplit {
		pSetCursor.Call(loadCursor(idcSizeNS))

		return 1, true
	}

	pSetCursor.Call(loadCursor(idcArrow))

	return 1, true
}

func (a *app) cuidarAtivacaoJanela(wparam uintptr) {
	if loword(wparam) == waInactive {
		pUnregisterHotKey.Call(a.hwnd, 1)
	} else {
		pRegisterHotKey.Call(a.hwnd, 1, modControleJanela|modAltJanela, vkDelete)
	}
}

func (a *app) cuidarDesativacaoJanela(lparam uintptr) {
	if panelMenu.open && lparam != panelMenu.hwnd {
		a.fecharMenuPainel()
	}

	if a.popupOn {
		a.fecharPopup()
		a.layout()
	}

	a.hideTip()
	a.esconderAviso()

	if a.extFloatOn && a.extFloatHWND != 0 && lparam == a.extFloatHWND {
		a.janelaAtiva = true

		return
	}

	a.janelaAtiva = false
	pSetTimer.Call(a.hwnd, timerOcioso, idleSuspenderMs, 0)

	a.agendarCongelamentoChatGPT()

	if !a.ytAberto {
		aplicarEcoQoS(true)
	}
}

func (a *app) cuidarReativacaoJanela() {
	pKillTimer.Call(a.hwnd, timerOcioso)
	aplicarEcoQoS(false)

	if !a.janelaAtiva {
		a.janelaAtiva = true
		a.retomarTudo()
	}
}

func (a *app) cuidarMensagensPintura(hwnd uintptr, msg uint32, _, _ uintptr) (uintptr, bool) {
	switch msg {
	case wmSize:
		a.cuidarRedimensionar(hwnd)

		return 0, true
	case wmPaint:
		a.cuidarPintura(hwnd)

		return 0, true
	case wmEraseBkgnd:
		return 1, true
	default:
		return 0, false
	}
}

func (a *app) cuidarMensagensSistema(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmNcCalcSize:
		if wparam != 0 {
			if z, _, _ := pIsZoomed.Call(hwnd); z != 0 {
				wa := areaTrabalhoJanela(hwnd)
				if wa.Right > wa.Left && wa.Bottom > wa.Top {
					np := (*ncCalcSizeParams)(unsafe.Pointer(lparam))
					np.rgrc[0] = wa
				}
			}
		}

		return 0, true
	case wmNcHitTest:
		return a.hitTestNaoCliente(lparam), true
	case wmAppTarefa:
		a.drainTarefas()

		return 0, true
	case wmClipboardUpdate:
		a.clipboardAtualizar()

		return 0, true
	default:
		return 0, false
	}
}

func (a *app) cuidarMensagensBase(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	if ret, ok := a.cuidarMensagensPintura(hwnd, msg, wparam, lparam); ok {
		return ret, true
	}

	return a.cuidarMensagensSistema(hwnd, msg, wparam, lparam)
}

func (a *app) cuidarRedimensionar(_ uintptr) {
	a.clientValid = false
	a.layout()

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		a.suspenderTudo()
	} else {
		a.retomarTudo()
	}

	if r := a.clientRect(); r.Right > 0 && r.Bottom > 0 {
		d2dMain.redimensionar(r.Right, r.Bottom)
	}
}

func (a *app) cuidarPintura(hwnd uintptr) {
	r := a.clientRect()
	if d2dMain.rt == nil {
		d2dMain.criar(hwnd, r.Right, r.Bottom)
	}

	if d2dMain.rt != nil {
		d2dMain.iniciar()
		a.paintD2D()
		d2dMain.apresentar()
	} else {
		pValidateRect.Call(hwnd, 0)
	}
}

func (a *app) cuidarTimerStatus(hwnd uintptr) {
	pKillTimer.Call(hwnd, timerStatus)

	a.status = ""
	a.invalidateFaixa()
	a.invalidateBar()
}

func (a *app) cuidarTimerDica(hwnd uintptr) {
	pKillTimer.Call(hwnd, timerTip)

	if a.tipPending {
		a.showTip()
	}
}

func (a *app) cuidarTimerRepetirChat(hwnd uintptr) {
	pKillTimer.Call(hwnd, timerRetryChat)

	if a.right != nil && a.right.ready && a.right.queriaURL != "" {
		a.right.wv.Navigate(a.right.queriaURL)
	}
}

func (a *app) cuidarTimerAvisosJanela(hwnd, wparam uintptr) (uintptr, bool) {
	switch wparam {
	case timerAviso:
		a.vigiarAviso()

		return 0, true
	case timerAvisoAnim:
		a.animarAviso()

		return 0, true
	case timerStatus:
		a.cuidarTimerStatus(hwnd)

		return 0, true
	case timerTip:
		a.cuidarTimerDica(hwnd)

		return 0, true
	case timerRetryChat:
		a.cuidarTimerRepetirChat(hwnd)

		return 0, true
	default:
		return 0, false
	}
}

func (a *app) cuidarTimerJanela(hwnd, wparam uintptr) (uintptr, bool) {
	switch wparam {
	case timerSubDelay:
		a.cuidarTimerSubAtraso(hwnd)

		return 0, true
	case timerOcioso:
		a.cuidarTimerOcioso(hwnd)

		return 0, true
	case timerChatFrio:
		a.cuidarTimerChatFrio(hwnd)

		return 0, true
	case timerHover:
		a.cuidarPassagemCursor(hwnd)

		return 0, true
	case timerChatIdle:
		a.cuidarTimerChatOcioso()

		return 0, true
	default:
		return a.cuidarTimerAvisosJanela(hwnd, wparam)
	}
}

func (a *app) cuidarDuploCliqueJanela(_, lparam uintptr) {
	x, y := loword(lparam), hiword(lparam)
	if !a.overSplit(x, y) {
		return
	}

	a.st.Split = 0.66
	a.layout()
	a.persist()
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) cuidarDuploCliqueNaoCliente(hwnd, wparam, lparam uintptr) {
	if wparam != htCaption {
		return
	}

	var pt struct{ x, y int32 }

	pt.x, pt.y = loword(lparam), hiword(lparam)
	pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))

	if idx, _, _, _ := a.hitFaixa(pt.x, pt.y); idx < 0 {
		a.toggleMaximizar()
	}
}

func (a *app) cuidarCliqueMeioJanela(lparam uintptr) {
	if a.soIA || a.painelCheioAtivo() {
		return
	}

	if idx, _, _, _ := a.hitFaixa(loword(lparam), hiword(lparam)); idx >= 0 {
		a.fecharAba(idx)
	}
}

// cuidarCliqueJanela despacha o clique: uintptr 0 = tratado; o bool diz se consumiu.
func (a *app) cuidarCliqueJanela(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) { //nolint:unparam
	switch msg {
	case wmLButtonDown:
		// a.onLButtonDown(loword(lparam), hiword(lparam))
		espMouseDown(a, loword(lparam), hiword(lparam))

		return 0, true
	case wmLButtonUp:
		// if a.soltarArrastoAba() {
		// 	return 0, true
		// }
		// a.soltarArrastoDivisores()
		espMouseUp(a, loword(lparam), hiword(lparam))

		return 0, true
	case wmRButtonUp:
		a.onRButtonUp(loword(lparam), hiword(lparam))

		return 0, true
	case wmMButtonUp:
		a.cuidarCliqueMeioJanela(lparam)

		return 0, true
	case wmLButtonDblClk:
		// a.cuidarDuploCliqueJanela(hwnd, lparam)
		espDuploClique(a, loword(lparam), hiword(lparam))

		return 0, true
	case wmNcLButtonDblClk:
		a.cuidarDuploCliqueNaoCliente(hwnd, wparam, lparam)

		return 0, true
	default:
		return 0, false
	}
}

func (a *app) cuidarSaidaMouseJanela(hwnd uintptr) {
	a.trackingMouse = false
	a.hideTip()
	mudou := a.hover != "" || a.tabHover != -1 || a.tabHoverClose != -1 ||
		a.tabHoverAudio != -1 || a.winHover != winNenhum
	a.hover = ""
	a.tabHover, a.tabHoverClose, a.tabHoverAudio = -1, -1, -1
	a.winHover = winNenhum

	if mudou {
		pInvalidateRect.Call(hwnd, 0, 0)
	}
}

func (a *app) cuidarMouseJanela(hwnd uintptr, msg uint32, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmMouseMove:
		// a.onMouseMove(loword(lparam), hiword(lparam))
		espMouseMove(a, loword(lparam), hiword(lparam))

		return 0, true
	case wmCapturaPerdida:
		a.dragSplit = false
		a.dragImgSplit = false

		a.dragLeftAuxSplit = false
		if a.tabDrag >= 0 {
			a.tabDrag = -1
		}

		return 0, true
	case wmMouseLeave:
		a.cuidarSaidaMouseJanela(hwnd)

		return 0, true
	case wmSetCursor:
		return a.cuidarCursorJanela(hwnd, lparam)
	default:
		return 0, false
	}
}

// cuidarRodaJanela encaminha a roda ao menu aberto (foco fica na janela
// principal porque o menu abre sem ativar); falso = segue p/ padrao.
func (a *app) cuidarRodaJanela(wparam uintptr) (uintptr, bool) {
	if a == nil || !menu.open {
		return 0, false
	}

	rolarMenuPrincipal(a, wparam)

	return 0, true
}

func (a *app) cuidarTeclaJanela(wparam uint32) (uintptr, bool) {
	if a.handleShortcut(wparam, getKeyDown(vkControl), getKeyDown(vkShift), getKeyDown(vkMenu)) {
		return 0, true
	}

	return 0, false
}

func (a *app) cuidarTeclaSistemaJanela(wparam uint32) (uintptr, bool) {
	if a.handleShortcut(wparam, getKeyDown(vkControl), getKeyDown(vkShift), true) {
		return 0, true
	}

	return 0, false
}

func (a *app) cuidarCorControleJanela(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	if lparam != a.urlEdit {
		return defWindowProc(hwnd, msg, wparam, lparam), true
	}

	pSetBkMode.Call(wparam, opaque)
	pSetBkColor.Call(wparam, corSuperficie)

	if a.urlEditing {
		pSetTextColor.Call(wparam, corTexto)
	} else {
		pSetTextColor.Call(wparam, corTextoFaint)
	}

	return a.brEdit, true
}

func (a *app) cuidarDpiJanela(_, lparam uintptr) {
	a.dpi = 0
	a.clientValid = false

	if lparam != 0 {
		pr := (*rectT)(unsafe.Pointer(lparam))
		if pr.Right > pr.Left && pr.Bottom > pr.Top {
			pSetWindowPos.Call(a.hwnd, 0,
				uintptr(int64(pr.Left)), uintptr(int64(pr.Top)),
				uintptr(pr.Right-pr.Left), uintptr(pr.Bottom-pr.Top),
				swpNoZOrder|swpNoActivate)
		}
	}

	a.layout()

	if a.urlEdit != 0 && a.fontUI != 0 {
		pSendMessageW.Call(a.urlEdit, msgDefinirFonte, a.fontUI, 1)
	}
}

func (a *app) cuidarLimitesJanela(hwnd, lparam uintptr) {
	limites := (*minMaxInfo)(unsafe.Pointer(lparam))
	limites.ptMinTrackSize = struct{ x, y int32 }{a.sc(720), a.sc(460)}

	wa := areaTrabalhoJanela(hwnd)
	if wa.Right > wa.Left && wa.Bottom > wa.Top {
		limites.ptMaxPosition = struct{ x, y int32 }{wa.Left, wa.Top}
		limites.ptMaxSize = struct{ x, y int32 }{wa.Right - wa.Left, wa.Bottom - wa.Top}
	}
}

func (a *app) cuidarSistemaJanela(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmCtlColorEdit, wmCtlColorStatic:
		return a.cuidarCorControleJanela(hwnd, msg, wparam, lparam)
	case wmKillFocus:
		return 0, true
	case wmDpiChanged:
		a.cuidarDpiJanela(hwnd, lparam)

		return 0, true
	case wmGetMinMaxInfo:
		a.cuidarLimitesJanela(hwnd, lparam)

		return 0, true
	case wmCommand:
		return 0, true
	case wmClose:
		marbleSalvarBoard()
		marbleSalvarSpace(marbleEspacoAtual)
		a.saveAndQuit()

		return 0, true
	case wmDestroy:
		pPostQuitMessage.Call(0)

		return 0, true
	default:
		return 0, false
	}
}

func (a *app) cuidarTimerSubAtraso(hwnd uintptr) {
	pKillTimer.Call(hwnd, timerSubDelay)

	idx := a.subAtrasoIdx
	a.subAtrasoIdx = -1

	a.subAtrasoQuando = time.Time{}
	if menu.open && idx >= 0 && menu.hover == idx && menu.subPai != idx &&
		idx < len(menu.items) && len(menu.items[idx].sub) > 0 {
		a.abrirSubMenu(idx)
	}
}

func (a *app) cuidarTimerOcioso(hwnd uintptr) {
	pKillTimer.Call(hwnd, timerOcioso)

	if !a.janelaAtiva {
		a.suspenderTudo()
	}
}

func (a *app) cuidarTimerChatFrio(hwnd uintptr) {
	if ic, _, _ := pIsIconic.Call(hwnd); ic == 0 && a.janelaAtiva {
		if a.chatFrio && !a.chatManual && a.cursorNoPainelChat() {
			a.retomarChatGPTFrio()
		}
	}
}

func (a *app) cuidarAtivacaoJanelaCompleta(wparam, lparam uintptr) {
	a.cuidarAtivacaoJanela(wparam)

	if loword(wparam) == waInactive {
		a.cuidarDesativacaoJanela(lparam)
	} else {
		a.cuidarReativacaoJanela()
	}
}

func (a *app) cuidarAtalhoJanela(wparam uintptr) (uintptr, bool) {
	if wparam != 1 {
		return 0, false
	}

	a.abrirPagina("dados")

	return 0, true
}

func (a *app) cuidarTeclasJanela(msg uint32, wparam uintptr) (uintptr, bool) {
	switch msg {
	case wmHotKey:
		return a.cuidarAtalhoJanela(wparam)
	case wmKeyDown:
		return a.cuidarTeclaJanela(uint32(wparam))
	case wmSysKeyDown:
		return a.cuidarTeclaSistemaJanela(uint32(wparam))
	default:
		return 0, false
	}
}

func (a *app) cuidarFimJanela(msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmActivate:
		a.cuidarAtivacaoJanelaCompleta(wparam, lparam)

		return 0, true
	case wmHotKey, wmKeyDown, wmSysKeyDown:
		return a.cuidarTeclasJanela(msg, wparam)
	default:
		return 0, false
	}
}

func retornoBaseSemApp(hwnd uintptr, msg uint32) uintptr {
	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmNcHitTest:
		return htClient
	case wmPaint:
		pValidateRect.Call(hwnd, 0)
	}

	return 0
}

func retornoSemApp(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmSize, wmPaint, wmEraseBkgnd, wmNcCalcSize, wmNcHitTest, wmAppTarefa, wmClipboardUpdate:
		return retornoBaseSemApp(hwnd, msg)
	default:
		return defWindowProc(hwnd, msg, wparam, lparam)
	}
}

func (a *app) cuidarMensagensIniciais(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmSize, wmPaint, wmEraseBkgnd, wmNcCalcSize, wmNcHitTest, wmAppTarefa, wmClipboardUpdate:
		return a.cuidarMensagensBase(hwnd, msg, wparam, lparam)
	case wmTimer:
		return a.cuidarTimerJanela(hwnd, wparam)
	case wmLButtonDown, wmLButtonUp, wmRButtonUp, wmMButtonUp, wmLButtonDblClk, wmNcLButtonDblClk:
		if ret, ok := a.cuidarCliqueJanela(hwnd, msg, wparam, lparam); ok {
			return ret, true
		}

		return 0, true
	default:
		return 0, false
	}
}

func (a *app) cuidarMensagensFinais(hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmMouseMove, wmCapturaPerdida, wmMouseLeave, wmSetCursor:
		return a.cuidarMouseJanela(hwnd, msg, lparam)
	case wmMouseWheel:
		return a.cuidarRodaJanela(wparam)
	case wmCtlColorEdit, wmCtlColorStatic, wmKillFocus, wmDpiChanged, wmGetMinMaxInfo, wmCommand, wmClose, wmDestroy:
		return a.cuidarSistemaJanela(hwnd, msg, wparam, lparam)
	case wmActivate, wmHotKey, wmKeyDown, wmSysKeyDown, wmChar:
		if espTecla(a, msg, wparam) {
			return 0, true
		}

		return a.cuidarFimJanela(msg, wparam, lparam)
	default:
		return 0, false
	}
}

func wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	app := theApp
	if app == nil {
		return retornoSemApp(hwnd, msg, wparam, lparam)
	}

	if ret, ok := app.cuidarMensagensIniciais(hwnd, msg, wparam, lparam); ok {
		return ret
	}

	if ret, ok := app.cuidarMensagensFinais(hwnd, msg, wparam, lparam); ok {
		return ret
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func (a *app) toggleMaximizar() {
	if z, _, _ := pIsZoomed.Call(a.hwnd); z != 0 {
		pShowWindow.Call(a.hwnd, swRestore)
	} else {
		pShowWindow.Call(a.hwnd, swMaximize)
	}
}

func (a *app) DPI() int {
	if a.dpi == 0 {
		a.dpi = dpiForWindow(a.hwnd)
		if a.dpi == 0 {
			a.dpi = 96
		}
	}

	return a.dpi
}

func (a *app) sc(v int32) int32 { return scale(v, a.DPI()) }

func (a *app) setTitle(titulo string) {
	if a.hwnd == 0 {
		return
	}

	sufixo := appTitle
	if perfilPrivado {
		sufixo += txtSufixoPrivado
	}

	switch {
	case titulo == "":
		titulo = appTitle + " " + appVersion
		if perfilPrivado {
			titulo += txtSufixoPrivado
		}
	case strings.HasPrefix(titulo, appTitle):
		if perfilPrivado && !strings.Contains(titulo, txtSufixoPrivado) {
			titulo += txtSufixoPrivado
		}
	default:
		titulo = titulo + " - " + sufixo
	}

	pSetWindowTextW.Call(a.hwnd, uintptr(unsafe.Pointer(utf16(titulo))))
}

func (a *app) initBrushes() {
	if a.brBg != 0 {
		return
	}

	a.brBg = corFundo
	a.brChrome = 0x00151515
	a.brSurface = corSuperficie
	a.brHover = 0x00262626
	a.brActive = 0x00303030
	a.brSep = 0x002a2a2a
	a.brBorder = corBordaForte
	a.brTipBg = corSuperficie
	a.brTipBorder = corBordaForte

	a.brTabOn = 0x00262626
	a.brAccent = corAccentSoft

	a.brFechar = 0x007f1d1d
	a.brIcon = corTextoFraco

	a.brIconHover = corTexto
	if a.brEdit == 0 {
		h, _, _ := pCreateSolidBrush.Call(uintptr(corSuperficie))
		a.brEdit = h
	}
}

func (a *app) initFonts() {
	if dpi := a.DPI(); a.fontUI != 0 && a.fontDPI == dpi {
		return
	}

	if a.fontUI != 0 {
		pDeleteObject.Call(a.fontUI)
		pDeleteObject.Call(a.fontSm)
		limparD2D()
	}

	a.fontDPI = a.DPI()
	montar := func(sizeLogical int32, weight int32) uintptr {
		fonte, _, _ := pCreateFontW.Call(
			uintptr(-a.sc(sizeLogical)), 0, 0, 0,
			uintptr(weight), 0, 0, 0,
			defaultCharset, outTtPrecis, clipDefaultPrecis, cleartypeQuality, defaultPitch,
			uintptr(unsafe.Pointer(utf16("Segoe UI"))),
		)

		return fonte
	}
	a.fontUI = montar(fonteUITamanho, fwNormal)
	a.fontSm = montar(fontePequenaTamanho, fwNormal)
	a.initIconFace()
}

func (a *app) clientRect() rectT {
	if a.clientValid {
		return a.clientCached
	}

	var r rectT
	pGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)))
	a.clientCached = r
	a.clientValid = true

	return r
}

func (a *app) larguraAuxiliar(areaCliente rectT) (int32, bool) {
	mostrarAux := a.st.LeftPanelVisible && !a.soIA && (!a.ytAberto || a.ytOculto)
	auxW := int32(0)

	if !mostrarAux {
		return auxW, mostrarAux
	}

	if a.st.LeftPanelSplit <= 0 {
		a.st.LeftPanelSplit = leftPanelSplitPadrao
	}

	auxW = int32(float64(areaCliente.Right) * a.st.LeftPanelSplit)
	minAux := a.sc(leftPanelMinLogical)
	maxAux := int32(float64(areaCliente.Right) * leftPanelSplitMax)

	if auxW < minAux {
		auxW = minAux
	}

	if auxW > maxAux {
		auxW = maxAux
	}

	return auxW, mostrarAux
}

func (a *app) baseAuxiliar(mostrarAux bool, auxW int32) int32 {
	railAux := !a.soIA && (!a.ytAberto || a.ytOculto)
	a.leftAuxSplitX = 0
	mainLeft := int32(0)

	if railAux {
		a.leftAuxSplitX = a.sc(railFinoLogico) / divisorMetade
		mainLeft = a.sc(railFinoLogico)
	}

	if mostrarAux {
		a.leftAuxSplitX = auxW
		mainLeft = auxW + a.rail/divisorMetade
	}

	return mainLeft
}

func (a *app) limitarDivisaoPrincipal(areaCliente rectT, mainLeft, auxW int32) (int32, int32) {
	posDivisoria := int32(float64(areaCliente.Right) * a.st.Split)
	minLeft := a.sc(painelMinEsq)
	minRight := a.sc(painelMinDir)

	if posDivisoria < mainLeft+minLeft {
		posDivisoria = mainLeft + minLeft
	}

	if posDivisoria > areaCliente.Right-minRight {
		posDivisoria = areaCliente.Right - minRight
	}

	if auxW > 0 {
		maxAuxByMain := posDivisoria - a.rail - minLeft
		if maxAuxByMain < a.sc(leftPanelMinLogical) {
			maxAuxByMain = a.sc(leftPanelMinLogical)
		}

		if auxW > maxAuxByMain {
			auxW = maxAuxByMain
		}

		mainLeft = auxW + a.rail/divisorMetade
		if posDivisoria < mainLeft+minLeft {
			posDivisoria = mainLeft + minLeft
		}
	}

	return posDivisoria, mainLeft
}

func (a *app) ajustarDivisaoDireita(areaCliente rectT, mainLeft, sx int32) int32 {
	a.splitXAntes = a.splitX

	a.splitX = sx
	if !a.st.RightVisible {
		return a.splitX
	}

	if a.soIA {
		a.splitX = a.rail / divisorMetade

		return a.splitX
	}

	maxChat := a.sc(chatMaxWidthLogical)
	if w := areaCliente.Right - (a.splitX + a.rail/divisorMetade); w > maxChat {
		a.splitX = areaCliente.Right - a.rail/divisorMetade - maxChat
	}

	if a.splitX < mainLeft+a.sc(painelMinEsq) {
		a.splitX = mainLeft + a.sc(painelMinEsq)
	}

	return a.splitX
}

func (a *app) alturaImagensPane(bottom, top int32) (int32, bool) {
	mostrarImagens := a.st.ImgVisible && a.imgPanel != nil && !a.soIA
	imgH := int32(0)

	if !mostrarImagens {
		return imgH, mostrarImagens
	}

	if a.st.ImgSplit <= 0 {
		a.st.ImgSplit = imgSplitPadrao
	}

	imgH = int32(float64(bottom-top) * a.st.ImgSplit)
	if imgH < a.sc(imgAlturaMin) {
		imgH = a.sc(imgAlturaMin)
	}

	if imgH > bottom-top-a.sc(imgAlturaMin) {
		imgH = bottom - top - a.sc(imgAlturaMin)
	}

	return imgH, mostrarImagens
}

func (a *app) posicionarPaineisDireita(areaCliente rectT, top, bottom, mainLeft, posDivisoria int32) {
	if !a.st.RightVisible {
		a.left.rect = Rect{Left: mainLeft, Top: top, Right: areaCliente.Right - a.sc(railFinoLogico), Bottom: bottom}

		a.imgSplitY = 0
		if a.right != nil {
			a.right.rect = Rect{Left: areaCliente.Right, Top: top, Right: areaCliente.Right, Bottom: top}
		}

		if a.imgPanel != nil {
			a.imgPanel.rect = Rect{Left: areaCliente.Right, Top: top, Right: areaCliente.Right, Bottom: top}
		}

		return
	}

	a.left.rect = Rect{Left: mainLeft, Top: top, Right: posDivisoria - a.rail/divisorMetade, Bottom: bottom}
	imgH, mostrarImagens := a.alturaImagensPane(bottom, top)

	imgRail := a.rail

	a.imgSplitY = top + imgRail/divisorMetade
	if mostrarImagens {
		a.imgSplitY = top + imgH
		a.imgPanel.rect = Rect{
			Left: posDivisoria + a.rail/divisorMetade, Top: top,
			Right: areaCliente.Right, Bottom: a.imgSplitY - imgRail/divisorMetade,
		}
	} else if a.imgPanel != nil {
		a.imgPanel.rect = Rect{Left: areaCliente.Right, Top: top, Right: areaCliente.Right, Bottom: top}
	}

	chatTop := a.topoChatPane(top, mostrarImagens)

	if a.right != nil {
		a.right.rect = Rect{Left: posDivisoria + a.rail/divisorMetade, Top: chatTop, Right: areaCliente.Right, Bottom: bottom}
	}
}

func (a *app) topoPainelLayout(areaCliente rectT) (int32, int32) {
	top := a.tabH + a.tbHeight + 1
	if a.soIA {
		top = 0
	}

	bottom := areaCliente.Bottom

	return top, bottom
}

func (a *app) alinharPaineisAuxiliares(top, bottom int32, mostrarAux bool, auxW int32) {
	if a.leftAux != nil {
		if mostrarAux {
			a.leftAux.rect = Rect{Left: 0, Top: top, Right: auxW - a.rail/2, Bottom: bottom}
		} else {
			a.leftAux.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
		}
	}

	for _, t := range a.tabs {
		if t.pane != nil {
			t.pane.rect = a.left.rect
		}
	}
}

func (a *app) aplicarAreaPrincipal(areaCliente rectT, top, bottom int32) {
	auxW, mostrarAux := a.larguraAuxiliar(areaCliente)

	mainLeft := a.baseAuxiliar(mostrarAux, auxW)

	sx, mainLeft := a.limitarDivisaoPrincipal(areaCliente, mainLeft, auxW)

	sx = a.ajustarDivisaoDireita(areaCliente, mainLeft, sx)

	a.posicionarPaineisDireita(areaCliente, top, bottom, mainLeft, sx)
	a.alinharPaineisAuxiliares(top, bottom, mostrarAux, auxW)
}

func (a *app) topoChatPane(top int32, mostrarImagens bool) int32 {
	if a.soIA {
		a.imgSplitY = 0

		return top
	}

	chatTop := top + a.rail
	if mostrarImagens {
		chatTop = a.imgSplitY + a.rail/divisorMetade
	}

	return chatTop
}

func (a *app) esconderPaineisYouTube(top int32) {
	a.left.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	if a.right != nil {
		a.right.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	if a.imgPanel != nil {
		a.imgPanel.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	if a.leftAux != nil {
		a.leftAux.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	a.splitBtn = rectT{}
	a.chatHomeBtn = rectT{}
	a.panelMenuBtn = rectT{}
	a.panelTranslateBtn = rectT{}
	a.panelSendBtn = rectT{}
	a.panelFullBtn = rectT{}
	a.panelReloadBtn = rectT{}
	a.imgToggleBtn = rectT{}
}

func (a *app) posicionarPainelYouTube(areaCliente rectT, top, bottom int32) {
	if a.ytPane == nil {
		return
	}

	switch {
	case a.ytAberto && !a.ytOculto:
		a.ytPane.rect = Rect{Left: 0, Top: top, Right: areaCliente.Right, Bottom: bottom}
	case a.ytAberto && a.ytOculto:
		esq := -a.sc(4000)
		a.ytPane.rect = Rect{Left: esq, Top: top, Right: esq + a.sc(640), Bottom: top + a.sc(360)}
	default:
		a.ytPane.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}
}

func (a *app) posicionarPopupLayout(top int32) {
	if a.popup == nil || !a.popupOn || a.soIA {
		if a.popup != nil {
			a.popup.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
		}

		return
	}

	larguraPopup := a.sc(360)

	alturaPopup := a.popupAltura
	if alturaPopup <= 0 {
		alturaPopup = a.sc(580)
	}

	if larguraPopup > a.left.rect.Width()-a.sc(20) {
		larguraPopup = a.left.rect.Width() - a.sc(20)
	}

	if alturaPopup > a.left.rect.Height()-a.sc(20) {
		alturaPopup = a.left.rect.Height() - a.sc(20)
	}

	a.popup.rect = Rect{
		Left:   a.left.rect.Right - larguraPopup - a.sc(8),
		Top:    top + a.sc(8),
		Right:  a.left.rect.Right - a.sc(8),
		Bottom: top + a.sc(8) + alturaPopup,
	}
}

func (a *app) medirBotaoDivisor(areaCliente rectT) (int32, int32, int32) {
	larguraBotao := a.sc(14)

	bh2 := a.sc(34)
	if a.st.RightVisible && a.right != nil && a.right.rect.Width() > 0 && (!a.ytAberto || a.ytOculto) {
		larguraBotao = a.sc(botaoPainelLargura)
		bh2 = a.sc(botaoPainelAltura)
	}

	if a.soIA && larguraBotao > a.rail {
		larguraBotao = a.rail
	}

	var posBotao int32
	if a.st.RightVisible {
		posBotao = max(a.splitX-larguraBotao/divisorMetade, 0)
	} else {
		fino := a.sc(railFinoLogico)
		posBotao = areaCliente.Right - fino + (fino-larguraBotao)/divisorMetade
	}

	return larguraBotao, bh2, posBotao
}

func (a *app) posicionarBotoesColunaPainel(top, bottom, posBotao, larguraBotao int32) {
	a.chatHomeBtn = rectT{}
	a.panelMenuBtn = rectT{}
	a.panelTranslateBtn = rectT{}
	a.panelSendBtn = rectT{}
	a.panelFullBtn = rectT{}
	a.panelReloadBtn = rectT{}

	if !a.st.RightVisible || a.right == nil || a.right.rect.Width() <= 0 || (a.ytAberto && !a.ytOculto) {
		return
	}

	inset := a.sc(botaoPainelInset)
	rowH := a.sc(botaoPainelLinha)

	centroY := top + (bottom-top)/divisorMetade

	nFileiras := 4
	if a.right.ready {
		nFileiras = 6
	}

	y := centroY - a.sc(12) - int32(nFileiras)*rowH
	topoColuna := y - a.sc(4)
	a.panelMenuBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}
	y += rowH
	a.panelTranslateBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}
	y += rowH
	a.panelSendBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}
	y += rowH
	a.chatHomeBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}

	y += rowH
	if a.right.ready {
		a.panelReloadBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}
		y += rowH
		a.panelFullBtn = rectT{posBotao + inset, y, posBotao + larguraBotao - inset, y + rowH}
	}

	a.splitBtn = rectT{posBotao, topoColuna, posBotao + larguraBotao, centroY + a.sc(12)}
}

func (a *app) posicionarBotoesDivisores(areaCliente rectT, top, bottom int32) {
	bw, bh2, bx := a.medirBotaoDivisor(areaCliente)
	top2 := top + (bottom-top)/divisorMetade - bh2/divisorMetade

	a.splitBtn = rectT{bx, top2, bx + bw, top2 + bh2}
	a.posicionarBotoesPainel(top, bottom)
	a.posicionarBotoesColunaPainel(top, bottom, bx, bw)

	a.imgToggleBtn = rectT{}
	if a.st.RightVisible && a.imgSplitY > 0 {
		bs := a.sc(botaoImagem)
		bx2 := areaCliente.Right - a.sc(botaoImagemMargem)

		by2 := max(a.imgSplitY-bs/2, top)

		a.imgToggleBtn = rectT{bx2, by2, bx2 + bs, by2 + bs}
	}
}

func (a *app) finalizarLayout(areaCliente rectT) {
	a.applyBounds()

	if !a.dragSplit && !a.dragImgSplit {
		r := a.rectDoChat()

		k := layoutKey{
			areaCliente.Right, areaCliente.Bottom, a.tbHeight, a.st.Split,
			a.st.RightVisible, a.left.rect, r, a.popupOn,
		}
		if !a.lastLayoutSet || k != a.lastLayout {
			a.lastLayout = k
			a.lastLayoutSet = true
		}
	}

	if a.dragSplit && a.splitXAntes != 0 {
		x0, x1 := a.splitXAntes, a.splitX
		if x0 > x1 {
			x0, x1 = x1, x0
		}

		r := rectT{x0 - a.rail - 2, a.tbHeight, x1 + a.rail + 2, areaCliente.Bottom}
		pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
	} else {
		pInvalidateRect.Call(a.hwnd, 0, 0)
	}
}

type medidaFaixa struct {
	pad, bh, padY                         int32
	addrFrameLeft, frameRight, barY, barH int32
	rx, extW, addrLeft                    int32
	uboBase                               string
}

func (a *app) montarBotoesNavegacao(areaCliente rectT, medida *medidaFaixa) {
	a.tbHeight = a.sc(46)
	a.tabH = a.sc(40)
	a.initBrushes()
	a.initFonts()
	medida.pad = a.sc(4)
	medida.bh = a.tbHeight - dobroMargemFaixa*medida.pad
	medida.padY = a.tabH + medida.pad
	x := medida.pad
	montar := func(id, label string) tbutton {
		larg := a.sc(36)
		if len(label) > limiteRotuloBotao {
			larg = a.sc(int32(9*len(label) + larguraExtraRotulo))
		}

		botao := tbutton{id: id, label: label, rect: rectT{x, medida.padY, x + larg, medida.padY + medida.bh}}
		x += larg + a.sc(2)

		return botao
	}
	a.buttons = a.buttons[:0]
	a.buttons = append(a.buttons, montar(idVoltar, "<"))
	a.buttons = append(a.buttons, montar(idAvancar, ">"))
	a.buttons = append(a.buttons, montar(idRecarregar, "R"))
	a.buttons = append(a.buttons, montar(idHome, "H"))
	medida.addrFrameLeft = x + a.sc(6)
	medida.rx = areaCliente.Right - medida.pad
	menuW := a.sc(38)
	medida.rx -= menuW
	a.menuBtn = tbutton{
		id:   idMenu,
		rect: rectT{medida.rx, medida.padY, medida.rx + menuW, medida.padY + medida.bh}, on: menu.open,
	}
	medida.rx -= a.sc(2)
	ytW := a.sc(36)
	medida.rx -= ytW
	a.ytBtn = tbutton{
		id:   idYoutube,
		rect: rectT{medida.rx, medida.padY, medida.rx + ytW, medida.padY + medida.bh}, on: a.ytAberto,
	}
	medida.rx -= a.sc(2)
	medida.extW = a.sc(36)
	medida.rx -= medida.extW
	a.extBtn = tbutton{
		id: idExtensoes, rect: rectT{medida.rx, medida.padY, medida.rx + medida.extW, medida.padY + medida.bh},
		on: menu.open && a.menuOrigem == idExtensoes,
	}

	downW := a.sc(36)
	medida.rx -= a.sc(2)
	medida.rx -= downW
	a.downBtn = tbutton{id: idDownloads, rect: rectT{medida.rx, medida.padY, medida.rx + downW, medida.padY + medida.bh}}

	a.extPinBtns = a.extPinBtns[:0]

	medida.uboBase = a.baseUbo()
}

func (a *app) anexarExtensaoFixa(posDireita, padY, bh, extW int32, base string) int32 {
	posDireita -= a.sc(2)
	posDireita -= extW
	a.extPinBtns = append(a.extPinBtns, tbutton{
		id:   "ext:" + base,
		rect: rectT{posDireita, padY, posDireita + extW, padY + bh},
		on:   a.extFloatOn && a.pastaDoFloat() == base,
	})

	return posDireita
}

func (a *app) anexarExtensoesFixas(medida *medidaFaixa) {
	for _, extensao := range a.st.ExtFixadas {
		if len(a.extPinBtns) >= maxExtFixadas {
			break
		}

		if !a.extensaoLigada(extensao) {
			continue
		}

		base := strings.ToLower(strings.TrimSpace(extensao))
		if base == "" {
			continue
		}

		if medida.uboBase != "" && strings.EqualFold(base, medida.uboBase) {
			continue
		}

		medida.rx = a.anexarExtensaoFixa(medida.rx, medida.padY, medida.bh, medida.extW, base)
	}
}

func (a *app) montarBotoesBarra(medida *medidaFaixa) {
	medida.rx -= a.sc(2)
	medida.frameRight = medida.rx - a.sc(6)
	medida.barY = medida.padY + a.sc(2)
	medida.barH = medida.bh - a.sc(4)
	a.toolsBtn = tbutton{
		id: idFerramentas,
		on: menu.open && a.menuOrigem == idFerramentas,
	}
	a.printBtn = tbutton{
		id: idImprimir,
		on: menu.open && a.menuOrigem == idImprimir,
	}
	a.clipBtn = tbutton{
		id: idClipboard,
		on: len(clipHist) > 0,
	}

	tr := rectT{}
	a.nojsBtn = tbutton{id: idNojs, rect: tr}
	a.sleepBtn = tbutton{id: idSleep, rect: tr, on: a.st.SleepLite}
	a.vpnBtn = tbutton{id: idVPN, rect: tr, on: a.vpnAtiva()}
	segW := a.sc(26)
	engW := a.sc(48)
	a.securityBtn = tbutton{
		id:   idSeguranca,
		rect: rectT{medida.addrFrameLeft, medida.barY, medida.addrFrameLeft + segW, medida.barY + medida.barH},
	}
	a.engineBtn = tbutton{
		id: idEngine,
		rect: rectT{
			medida.addrFrameLeft + segW + a.sc(2), medida.barY,
			medida.addrFrameLeft + segW + a.sc(2) + engW, medida.barY + medida.barH,
		},
	}
	medida.addrLeft = a.engineBtn.rect.Right + a.sc(4)

	a.privadoBadge = rectT{}
	if perfilPrivado {
		bw := a.sc(68)
		a.privadoBadge = rectT{medida.addrLeft, medida.barY, medida.addrLeft + bw, medida.barY + medida.barH}
		medida.addrLeft += bw + a.sc(4)
	}

	a.buttons = append(a.buttons, a.securityBtn, a.engineBtn, a.downBtn, a.ytBtn, a.extBtn, a.menuBtn)
	a.buttons = append(a.buttons, a.extPinBtns...)
	a.uboBtn = tbutton{}
	a.readerBtn = tbutton{id: idReader, on: a.left != nil && a.left.readerAtivo}
	a.readerCopyBtn = tbutton{id: idCopiarLeitura}
	a.paywallBtn = tbutton{id: idPaywall, on: a.menuOrigem == idPaywall}
	a.trBtn = tbutton{id: "tr", on: a.traduzido}
	a.cookiesBtn = tbutton{id: idCookies}
}

// esconderBotoesEnderecoNaBarra limpa os botoes que migraram para o menu ferramentas.
func (a *app) esconderBotoesEnderecoNaBarra() {
	a.readerBtn.rect = rectT{}
	a.readerCopyBtn.rect = rectT{}
	a.paywallBtn.rect = rectT{}
	a.trBtn.rect = rectT{}
	a.cookiesBtn.rect = rectT{}
}

func (a *app) posicionarIconesLeitura(posIcone int32) int32 {
	a.esconderBotoesEnderecoNaBarra()

	return posIcone
}

func (a *app) posicionarCampoEndereco(medida *medidaFaixa, posIcone, bordaDir int32) {
	if a.urlEdit == 0 {
		return
	}

	addrRight := posIcone - a.sc(2)
	if addrRight < medida.addrLeft+a.sc(40) {
		addrRight = medida.addrLeft + a.sc(40)
	}

	editH := a.sc(26)
	if editH > medida.barH-a.sc(2) {
		editH = medida.barH - a.sc(2)
	}

	editTop := medida.barY + (medida.barH-editH)/divisorMetade

	novo := rectT{medida.addrLeft, editTop, addrRight, editTop + editH}
	if novo != a.editRect {
		a.editRect = novo
		pSetWindowPos.Call(a.urlEdit, 0,
			uintptr(novo.Left), uintptr(novo.Top),
			uintptr(novo.Right-novo.Left), uintptr(novo.Bottom-novo.Top),
			swpNoZOrder)
	}

	frameR := bordaDir + a.sc(4)
	a.addrFrame = rectT{medida.addrFrameLeft - 1, medida.padY + a.sc(2) - 1, frameR, medida.padY + medida.bh - a.sc(4) + 1}
}

// posicionarBotoesLeitura posiciona da direita para a esquerda, dentro do
// quadro do endereco: uBO, ferramentas, historico e print. Print e historico
// ficam a esquerda de ferramentas; ferramentas fica a esquerda do uBO.
func (a *app) posicionarBotoesLeitura(medida *medidaFaixa) {
	iconW := a.sc(24)
	bordaDir := medida.frameRight - a.sc(4)
	posIcone := bordaDir

	colocar := func(botao *tbutton) {
		botao.rect = rectT{posIcone - iconW, medida.barY, posIcone, medida.barY + medida.barH}
		posIcone -= iconW + a.sc(2)
		a.buttons = append(a.buttons, *botao)
	}

	if medida.uboBase != "" {
		a.uboBtn = tbutton{
			id:   "ext:" + medida.uboBase,
			rect: rectT{},
			on:   a.extFloatOn && a.pastaDoFloat() == medida.uboBase,
		}
		colocar(&a.uboBtn)
	}

	colocar(&a.toolsBtn)
	colocar(&a.clipBtn)
	colocar(&a.printBtn)

	posIcone = a.posicionarIconesLeitura(posIcone)
	a.posicionarCampoEndereco(medida, posIcone, bordaDir)
}

func (a *app) layout() {
	areaCliente := a.clientRect()
	if areaCliente.Right <= 0 || areaCliente.Bottom <= 0 {
		return
	}

	var m medidaFaixa
	a.montarBotoesNavegacao(areaCliente, &m)
	a.anexarExtensoesFixas(&m)
	a.montarBotoesBarra(&m)
	a.posicionarBotoesLeitura(&m)

	top, bottom := a.topoPainelLayout(areaCliente)

	a.rail = a.sc(railLogico)
	if a.painelCheioAtivo() {
		a.layoutPainelCheio(areaCliente, 0, bottom)

		return
	}

	a.aplicarAreaPrincipal(areaCliente, top, bottom)

	if a.ytAberto && !a.ytOculto {
		a.esconderPaineisYouTube(top)
	}

	a.posicionarPainelYouTube(areaCliente, top, bottom)

	if !a.soIA {
		a.layoutFaixa()
	}

	a.posicionarPopupLayout(top)

	a.posicionarBotoesDivisores(areaCliente, top, bottom)

	a.finalizarLayout(areaCliente)
}

func (a *app) limitarFrequenciaBounds() bool {
	if !a.dragSplit && !a.dragImgSplit && !a.dragLeftAuxSplit {
		return true
	}

	agora := time.Now()
	if agora.Sub(a.ultimoBounds) < 20*time.Millisecond {
		return false
	}

	a.ultimoBounds = agora

	return true
}

func (a *app) aplicarVisibilidadePrincipal(acordado bool) {
	setPaneBounds(a.left)
	setPaneVisible(a.left, acordado && a.left != nil && a.left.rect.Width() > 0)
	setPaneBounds(a.right)
	setPaneVisible(a.right, acordado && a.st.RightVisible && a.right != nil && a.right.rect.Width() > 0)
	setPaneBounds(a.imgPanel)
	setPaneVisible(a.imgPanel, acordado && a.st.RightVisible && a.st.ImgVisible && a.imgPanel != nil &&
		a.imgPanel.rect.Width() > 0)
	a.aplicarTamanhoImagens()
}

func (a *app) aplicarVisibilidadePopup(acordado bool) {
	if a.popup == nil || !a.popup.ready || a.popup.wv == nil {
		return
	}

	quer := acordado && a.popupOn && a.popup.rect.Width() > 0
	if quer {
		if precisaMexerVisibilidade(a.popup, true) {
			a.popup.wv.Resume()
		}
	} else {
		suspenderPane(a.popup)
	}

	setPaneBounds(a.popup)
	setPaneVisible(a.popup, quer)
}

func (a *app) aplicarTamanhoChatAtivo() {
	if a.right == nil || !a.right.ready || !a.right.mobile {
		return
	}

	if a.dragSplit {
		a.escalaRapidaChat()
	} else {
		a.aplicarTamanhoChat(a.right)
	}
}

func (a *app) applyBounds() {
	if !a.limitarFrequenciaBounds() {
		return
	}

	if espCanvas.aberto >= 0 {
		espPosicionarWebView(a)

		return
	}

	acordado := !a.tudoSuspenso
	a.aplicarVisibilidadePrincipal(acordado)
	a.aplicarVisibilidadePopup(acordado)

	if a.ytPane != nil && a.ytPane.ready && a.ytPane.wv != nil {
		setPaneBounds(a.ytPane)
		setPaneVisible(a.ytPane, a.ytAberto && a.ytPane.rect.Width() > 0)
	}

	a.aplicarTamanhoChatAtivo()

	if !a.dragSplit {
		a.raisePanes()
	}
}

func aplicarCantosDWM(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	pref := int32(preferenciaCantoDWM)
	pDwmSetWindowAttribute.Call(hwnd, dwmwaWindowCorner, uintptr(unsafe.Pointer(&pref)), tamanhoAtributoDWM)
}

func registrarClasseOverdrive(hInst uintptr, className *uint16, hIcon uintptr) error {
	classeJanela := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         estiloDuploClique,
		lpfnWndProc:   wndProcPtr,
		hInstance:     hInst,
		hIcon:         hIcon,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: className,
		hIconSm:       hIcon,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classeJanela))); r == 0 {
		return erroTextoF("RegisterClassExW: %w", err)
	}

	return nil
}

func dpiDoSistema() int {
	dpi := 96
	if r, _, _ := pGetDpiForSystem.Call(); r > 0 {
		dpi = int(r)
	}

	return dpi
}

func dimensionarJanelaInicial(aplicativo *app, dpi int) (int32, int32) {
	temTamanho := aplicativo.st.WinW > 0 && aplicativo.st.WinH > 0
	if !temTamanho {
		aplicativo.st.WinW = 1400
		aplicativo.st.WinH = 900
	}

	larguraJanela, alturaJanela := scale(aplicativo.st.WinW, dpi), scale(aplicativo.st.WinH, dpi)
	if !temTamanho {
		wr := rectT{0, 0, larguraJanela, alturaJanela}
		pAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&wr)), wsJanelaApp, 0, 0, uintptr(dpi))
		larguraJanela, alturaJanela = wr.Right-wr.Left, wr.Bottom-wr.Top
	}

	return larguraJanela, alturaJanela
}

func areaDisponivelJanela(a *app) (rectT, int32, int32) {
	var area rectT
	if a.st.WinX != 0 || a.st.WinY != 0 {
		area = areaTrabalhoPonto(a.st.WinX+a.st.WinW/divisorMetade, a.st.WinY+a.st.WinH/divisorMetade)
	} else {
		area = areaTrabalhoJanela(0)
	}

	availW := area.Right - area.Left
	availH := area.Bottom - area.Top

	return area, availW, availH
}

func limitarJanelaArea(availW, availH int32, larguraJanela, alturaJanela int32, dpi int) (int32, int32) {
	if availW > 0 && larguraJanela > availW-scale(margemLimiteJanela, dpi) {
		larguraJanela = availW - scale(margemLimiteJanela, dpi)
	}

	if availH > 0 && alturaJanela > availH-scale(margemLimiteJanela, dpi) {
		alturaJanela = availH - scale(margemLimiteJanela, dpi)
	}

	return larguraJanela, alturaJanela
}

func centralizarJanelaOD(a *app, area rectT, availW, availH, larguraJanela, alturaJanela int32) (int32, int32) {
	x, y := a.st.WinX, a.st.WinY
	if x == 0 && y == 0 {
		return area.Left + (availW-larguraJanela)/divisorMetade, area.Top + (availH-alturaJanela)/divisorMetade
	}

	return confinarJanelaOD(area, larguraJanela, alturaJanela, x, y)
}

func confinarJanelaOD(area rectT, larguraJanela, alturaJanela, x, y int32) (int32, int32) {
	if x+larguraJanela > area.Right {
		x = area.Right - larguraJanela
	}

	if y+alturaJanela > area.Bottom {
		y = area.Bottom - alturaJanela
	}

	if x < area.Left {
		x = area.Left
	}

	if y < area.Top {
		y = area.Top
	}

	return x, y
}

func tituloJanelaOD() string {
	tituloJanela := appTitle + " " + appVersion
	if perfilPrivado {
		tituloJanela += txtSufixoPrivado
	}

	return tituloJanela
}

func aplicarEstiloJanela(hwnd uintptr) {
	one := int32(1)
	pDwmSetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&one)), tamanhoAtributoDWM)
	aplicarCantosDWM(hwnd)
	pDwmSetWindowAttribute.Call(hwnd, dwmwaCaptionColor,
		uintptr(unsafe.Pointer(&[]uint32{0x00151515}[0])), tamanhoAtributoDWM)
	pDwmSetWindowAttribute.Call(hwnd, dwmwaBorderColor,
		uintptr(unsafe.Pointer(&[]uint32{0x003a3a3a}[0])), tamanhoAtributoDWM)
	pSetWindowTheme.Call(hwnd, uintptr(unsafe.Pointer(utf16("DarkMode_Explorer"))), 0)
}

func (a *app) finalizarCriacaoJanela(hwnd, hInst, hIcon uintptr) {
	a.dpi = dpiForWindow(hwnd)
	a.initBrushes()
	a.initFonts()
	a.tbHeight = a.sc(28)
	a.criarCampoEndereco(hwnd, hInst)
	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)
	pSetFocus.Call(hwnd)

	if hIcon != 0 {
		pSendMessageW.Call(hwnd, wmSetIcon, iconBig, hIcon)
		pSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hIcon)
	}

	if a.st.Maximized {
		pShowWindow.Call(hwnd, swMaximize)
	}
}

func (a *app) createWindow() error {
	hInst := getModuleHandle()
	className := utf16("OverdriveWndClass")
	hIcon := loadIconFile(filepath.Join(exeDir, "WEB Overdrive.ico"))

	if err := registrarClasseOverdrive(hInst, className, hIcon); err != nil {
		return err
	}

	dpi := dpiDoSistema()
	larguraJanela, alturaJanela := dimensionarJanelaInicial(a, dpi)
	wa, availW, availH := areaDisponivelJanela(a)
	larguraJanela, alturaJanela = limitarJanelaArea(availW, availH, larguraJanela, alturaJanela, dpi)
	x, y := centralizarJanelaOD(a, wa, availW, availH, larguraJanela, alturaJanela)
	tituloJanela := tituloJanelaOD()

	hwnd, _, err := pCreateWindowExW.Call(
		wsExLayered,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16(tituloJanela))),
		wsJanelaApp,
		uintptr(x), uintptr(y),
		uintptr(larguraJanela), uintptr(alturaJanela),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return erroTextoF("CreateWindowExW: %w", err)
	}

	a.hwnd = hwnd
	// pSetLayeredWindowAttributes.Call(hwnd, 0, alfaJanela, lwaAlpha)
	espAplicarAlpha(a)
	pSetWindowLongPtrW.Call(hwnd, gwlpUserData, uintptr(unsafe.Pointer(a)))
	clipboardIniciar(hwnd)
	a.initTooltip()
	pSetTimer.Call(hwnd, timerHover, intervaloHoverMs, 0)
	pSetTimer.Call(hwnd, timerChatIdle, intervaloChatOciosoMs, 0)

	aplicarEstiloJanela(hwnd)
	a.finalizarCriacaoJanela(hwnd, hInst, hIcon)

	return nil
}

func (a *app) criarCampoEndereco(hwnd, hInst uintptr) {
	a.urlEdit, _, _ = pCreateWindowExW.Call(
		wsExLayered, // permite esconder o fundo quando editando nota.
		uintptr(unsafe.Pointer(utf16("EDIT"))),
		uintptr(unsafe.Pointer(utf16(""))),
		wsChild|wsVisible|wsTabStop|esAutoHScroll|esLeft,
		0, 0, tamanhoInicialCampo, tamanhoInicialCampo,
		hwnd, 0, hInst, 0,
	)
	if a.urlEdit == 0 {
		return
	}

	pSendMessageW.Call(a.urlEdit, msgDefinirFonte, a.fontUI, 1)

	pSendMessageW.Call(a.urlEdit, msgDefinirMargens, ambasMargensCampo, uintptr(uint32(a.sc(8))|uint32(a.sc(6))<<16))

	pSetWindowTheme.Call(a.urlEdit, uintptr(unsafe.Pointer(utf16(""))), uintptr(unsafe.Pointer(utf16(""))))
	a.oldEditProc, _, _ = pSetWindowLongPtrW.Call(a.urlEdit, gwlpWndProc, editProcPtr)

	a.urlEditing = false
	pShowWindow.Call(a.urlEdit, swHide)
}

func (a *app) invalidateBar() {
	if a == nil || a.hwnd == 0 {
		return
	}

	pInvalidateRect.Call(a.hwnd, 0, 0)

	if a.urlEdit != 0 {
		pInvalidateRect.Call(a.urlEdit, 0, 1)
	}
}

func (a *app) invalidateDownBar() {
	if a == nil || a.hwnd == 0 || a.addrFrame.Width() <= 0 {
		return
	}

	r := rectT{a.addrFrame.Left, a.addrFrame.Bottom - a.sc(6), a.addrFrame.Right, a.addrFrame.Bottom}
	pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
}

func (a *app) executarBotaoJanela(qual int) {
	switch qual {
	case winMinimiza:
		pShowWindow.Call(a.hwnd, swMinimize)
	case winMaximiza:
		a.toggleMaximizar()
	case winFecha:
		pPostMessageW.Call(a.hwnd, wmClose, 0, 0)
	}
}

func scale(v int32, dpi int) int32 {
	if dpi == dpiPadraoJanela {
		return v
	}

	return int32(int64(v) * int64(dpi) / dpiPadraoJanela)
}
