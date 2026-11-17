//go:build windows

package main

import (
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Constantes locais de entrada (teclas, mascaras, limites e passos sem outro dono).
const (
	teclaRetrocesso         = 0x08
	teclaInserir            = 0x2D
	margemTextoCampo        = 2
	divisorMetadeTrilho     = 2
	mascaraPalavraBaixa     = 0xFFFF
	deslocamentoPalavraAlta = 16
	botaoEsquerdoMouse      = 0x01
	indiceHoverNovaAba      = -2
	digitoUmAba             = 0x31
	digitoNoveAba           = 0x39
	passoZoomTeclado        = 0.05
)

func ehTeclaDeEdicao(tecla uint32, ctrl bool) bool {
	if !ctrl {
		return false
	}

	switch tecla {
	case vkC, vkV, vkX, vkZ, vkY, vkA, teclaRetrocesso, vkDelete, teclaInserir:
		return true
	}

	return false
}

func atalhoValeNoCampoDeEndereco(tecla uint32, ctrl, shift, alt bool) bool {
	if atalhoRefazerComShift(tecla, ctrl, shift) {
		return true
	}

	if ehTeclaDeEdicao(tecla, ctrl) {
		return false
	}

	return ctrl || alt || tecla == vkF3 || tecla == vkF5 || tecla == vkF11 || tecla == vkF12
}

// atalhoRefazerComShift detecta refazer/retrocesso com shift no endereco.
func atalhoRefazerComShift(tecla uint32, ctrl, shift bool) bool {
	return ctrl && shift && (tecla == vkY || tecla == vkDelete || tecla == teclaRetrocesso)
}

func editProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	app := theApp
	if app == nil {
		return defWindowProc(hwnd, msg, wparam, lparam)
	}

	switch msg {
	case wmKeyDown:
		if app.tratarTeclaEdit(hwnd, wparam) {
			return 0
		}
	case wmChar:
		app.invalidaEditCache()

		if wparam == vkReturn {
			return 0
		}
	case wmPaste, wmCut, wmClear, wmUndo:
		app.invalidaEditCache()
	case wmSetFocus:
		app.focarCampoEndereco(hwnd)
	case wmKillFocus:
		app.desfocarCampoEndereco(hwnd)
	}

	r, _, _ := pCallWindowProcW.Call(app.oldEditProc, hwnd, uintptr(msg), wparam, lparam)

	return r
}

func (a *app) tratarTeclaEdit(hwnd, wparam uintptr) bool {
	if espEditAtivo && espTeclaEditCard(a, wparam) {
		return true
	}

	return a.teclaEditProc(hwnd, wparam)
}

// teclaEditProc trata as teclas no campo de endereco; true = consumiu.
func (a *app) teclaEditProc(hwnd uintptr, wparam uintptr) bool {
	a.invalidaEditCache()

	switch uint32(wparam) {
	case vkReturn:
		txt := a.editText()
		a.navigateLeft(txt)

		if a.left != nil && a.left.ready {
			a.left.wv.Focus()
		}

		return true
	case vkEscape:
		a.restaurarEnderecoEdit(hwnd)

		return true
	case vkA:
		if getKeyDown(vkControl) {
			const emSetSel = 0x00B1
			pSendMessageW.Call(hwnd, emSetSel, 0, ^uintptr(0))

			return true
		}
	case vkV:
		if a.colarNavegarEdit(hwnd) {
			return true
		}
	}

	ctrlDw, altDw := getKeyDown(vkControl), getKeyDown(vkMenu)

	return a.tentarAtalhoEdit(uint32(wparam), ctrlDw, altDw)
}

// tentarAtalhoEdit executa o atalho no campo de endereco; true = tratado.
func (a *app) tentarAtalhoEdit(wparam uint32, ctrlDw, altDw bool) bool {
	if atalhoValeNoCampoDeEndereco(wparam, ctrlDw, getKeyDown(vkShift), altDw) {
		return a.handleShortcut(wparam, ctrlDw, getKeyDown(vkShift), altDw)
	}

	return false
}

// restaurarEnderecoEdit volta o texto da pagina ao apertar escape.
func (a *app) restaurarEnderecoEdit(hwnd uintptr) {
	if a.left != nil && a.left.ready && a.left.wv != nil {
		if u := a.left.wv.Source(); u != "" {
			pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16(u))))
			a.invalidaEditCache()
		}

		a.left.wv.Focus()
	}
}

// colarNavegarEdit cola sem formato e navega; true = consumiu.
func (a *app) colarNavegarEdit(hwnd uintptr) bool {
	if !getKeyDown(vkControl) || !getKeyDown(vkShift) {
		return false
	}

	if txt := strings.TrimSpace(lerTextoClipboard()); txt != "" {
		pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16(txt))))
		a.invalidaEditCache()
		a.navigateLeft(txt)

		if a.left != nil && a.left.ready {
			a.left.wv.Focus()
		}
	}

	return true
}

// focarCampoEndereco marca o foco no campo de endereco.
func (a *app) focarCampoEndereco(hwnd uintptr) {
	a.focus = idLeft
	a.urlEditing = true

	pShowWindow.Call(hwnd, swShow)
	a.invalidateBar()
}

// desfocarCampoEndereco sai do campo e sincroniza a barra.
func (a *app) desfocarCampoEndereco(hwnd uintptr) {
	a.urlEditing = false

	pShowWindow.Call(hwnd, swHide)

	if !a.loading {
		a.syncAddressBar()
	}

	a.invalidateBar()
}

func (a *app) editText() string {
	if a.urlEdit == 0 {
		return ""
	}

	if a.editCacheOK {
		return a.editCache
	}

	n, _, _ := pGetWindowTextLengthW.Call(a.urlEdit)
	buf := make([]uint16, n+margemTextoCampo)
	pGetWindowTextW.Call(a.urlEdit, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	a.editCache = windows.UTF16ToString(buf)
	a.editCacheOK = true

	return a.editCache
}

func (a *app) invalidaEditCache() {
	a.editCacheOK = false
}

func (a *app) setAddress(endereco string) {
	if a.urlEdit == 0 {
		return
	}

	if a.urlEditing {
		return
	}

	if endereco == "" || strings.HasPrefix(endereco, "about:") {
		if a.editText() != "" {
			pSetWindowTextW.Call(a.urlEdit, uintptr(unsafe.Pointer(utf16(""))))
			a.invalidaEditCache()
			a.invalidateBar()
		}

		return
	}

	if a.editText() == endereco {
		return
	}

	pSetWindowTextW.Call(a.urlEdit, uintptr(unsafe.Pointer(utf16(endereco))))
	a.invalidaEditCache()
	a.invalidateBar()
}

func (a *app) syncAddressBar() {
	if a.urlEdit == 0 || a.left == nil || !a.left.ready {
		return
	}

	if a.left.readerAtivo {
		a.setAddress(a.left.readerURL)

		return
	}

	if f, _, _ := pGetFocus.Call(); f == a.urlEdit {
		return
	}

	a.setAddress(a.left.wv.Source())
}

// paneComLargura diz se o painel existe e tem largura visivel.
func paneComLargura(p *pane) bool {
	return p != nil && p.rect.Width() > 0
}

func (a *app) overSplit(x, y int32) bool {
	if !a.st.RightVisible || a.rail <= 0 || a.soIA || (a.ytAberto && !a.ytOculto) {
		return false
	}

	if !paneComLargura(a.left) || !paneComLargura(a.right) {
		return false
	}

	if y < a.tabH+a.tbHeight {
		return false
	}

	d := a.rail / divisorMetadeTrilho

	return x >= a.splitX-d && x <= a.splitX+d
}

func (a *app) overLeftAuxSplit(x, y int32) bool {
	if !a.st.LeftPanelVisible || a.leftAuxSplitX <= 0 || a.rail <= 0 || a.soIA || (a.ytAberto && !a.ytOculto) {
		return false
	}

	if !paneComLargura(a.leftAux) {
		return false
	}

	if y < a.tabH+a.tbHeight {
		return false
	}

	d := a.rail / divisorMetadeTrilho

	return x >= a.leftAuxSplitX-d && x <= a.leftAuxSplitX+d
}

func (a *app) overLeftAuxBtn(x, y int32) bool {
	r := a.leftAuxBtn

	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) overLeftAuxFullBtn(x, y int32) bool {
	r := a.leftAuxFullBtn

	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) overSplitButton(x, y int32) bool {
	retangulo := a.splitBtn
	if retangulo.Width() <= 0 {
		return false
	}

	m := a.sc(3)
	if a.overChatHomeButton(x, y) {
		return false
	}

	return x >= retangulo.Left-m && x < retangulo.Right+m && y >= retangulo.Top-m && y < retangulo.Bottom+m
}

func (a *app) overChatHomeButton(x, y int32) bool {
	r := a.chatHomeBtn

	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) overImgSplit(x, y int32) bool {
	if !a.imgSplitPronto() {
		return false
	}

	if a.overImgToggle(x, y) {
		return false
	}

	rail := a.rail / divisorMetadeTrilho
	if x < a.splitX+rail || x >= a.clientRect().Right {
		return false
	}

	return y >= a.imgSplitY-rail && y <= a.imgSplitY+rail
}

// imgSplitPronto diz se a divisoria de imagens pode ser usada.
func (a *app) imgSplitPronto() bool {
	return a.st.RightVisible && a.st.ImgVisible && a.imgPanel != nil && a.right != nil &&
		a.right.ready && a.imgSplitY > 0
}

func (a *app) overImgToggle(x, y int32) bool {
	r := a.imgToggleBtn

	return a.st.RightVisible && r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) panelButtonAt(x, y int32) string {
	for _, botao := range []struct {
		id string
		r  rectT
	}{
		{idPainelMenu, a.panelMenuBtn},
		{idPainelTranslate, a.panelTranslateBtn},
		{idPainelSend, a.panelSendBtn},
		{idPainelFull, a.panelFullBtn},
		{idPainelReload, a.panelReloadBtn},
	} {
		if sobreBotaoPainel(botao.r, x, y) {
			return botao.id
		}
	}

	return ""
}

// sobreBotaoPainel diz se o ponto esta dentro do botao visivel.
func sobreBotaoPainel(r rectT, x, y int32) bool {
	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) hitButton(x, y int32) string {
	if y < a.tabH || y >= a.tabH+a.tbHeight {
		return ""
	}

	for _, b := range a.buttons {
		if x >= b.rect.Left && x < b.rect.Right {
			return b.id
		}
	}

	return ""
}

func (a *app) hitTestNaoCliente(lparam uintptr) uintptr {
	x := int32(int16(lparam & mascaraPalavraBaixa))
	y := int32(int16((lparam >> deslocamentoPalavraAlta) & mascaraPalavraBaixa))

	var retJanela rectT
	pGetWindowRect.Call(a.hwnd, uintptr(unsafe.Pointer(&retJanela)))

	zoom := false
	if z, _, _ := pIsZoomed.Call(a.hwnd); z != 0 {
		zoom = true
	}

	if ht, ok := a.bordaNaoCliente(x, y, retJanela, zoom); ok {
		return ht
	}

	pt := struct{ x, y int32 }{x, y}
	pScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&pt)))

	if pt.y >= 0 && pt.y < a.tabH && !a.sobreControle(pt.x, pt.y) {
		return htCaption
	}

	return htClient
}

// bordaNaoCliente identifica a borda de redimensionar; false = sem borda.
func (a *app) bordaNaoCliente(x, y int32, retJanela rectT, zoom bool) (uintptr, bool) {
	if zoom {
		return 0, false
	}

	borda := a.sc(5)
	esq := x < retJanela.Left+borda
	dir := x >= retJanela.Right-borda
	topo := y < retJanela.Top+borda

	base := y >= retJanela.Bottom-borda
	if ht, ok := cantoNaoCliente(topo, esq, base, dir); ok {
		return ht, true
	}

	switch {
	case esq:
		return htLeft, true
	case dir:
		return htRight, true
	case topo:
		return htTop, true
	case base:
		return htBottom, true
	}

	return 0, false
}

// cantoNaoCliente identifica o canto de redimensionar; false = sem canto.
func cantoNaoCliente(topo, esq, base, dir bool) (uintptr, bool) {
	switch {
	case topo && esq:
		return htTopLeft, true
	case topo && dir:
		return htTopRight, true
	case base && esq:
		return htBottomLeft, true
	case base && dir:
		return htBottomRight, true
	}

	return 0, false
}

// tratarCliqueCalendario abre/fecha o popup no relogio e o fecha ao clicar fora.
// true = o clique foi totalmente consumido.
func (a *app) tratarCliqueCalendario(x, y int32) bool {
	noRelogio := a.winClock.Width() > 0 && dentroRect(a.winClock, x, y)

	if noRelogio {
		if calendarioUI.visivel {
			fecharCalendario()
		} else {
			abrirCalendario(a, time.Now())
		}

		return true
	}

	if calendarioUI.visivel {
		fecharCalendario()
	}

	return false
}

func (a *app) onLButtonDown(x, y int32) {
	a.hideTip()

	if a.tratarCliqueCalendario(x, y) {
		return
	}

	if a.retomarFrioSobCursor(x, y) {
		return
	}

	if a.clicarMenuPainelAberto(x, y) {
		return
	}

	if a.soIA || a.painelCheioAtivo() {
		a.clicarModoCheio(x, y)

		return
	}

	if y < a.tabH {
		a.clicarFaixaAbas(x, y)

		return
	}

	if y >= a.tabH && y < a.tabH+a.tbHeight {
		a.clicarBarraEndereco(x, y)

		return
	}

	a.clicarAreaPaineis(x, y)
}

// retomarFrioSobCursor retoma o chat frio sob o cursor; true = tratado.
func (a *app) retomarFrioSobCursor(x, y int32) bool {
	if a.chatFrio && !a.chatManual && a.pontoNoPainelChat(x, y) {
		a.retomarChatGPTFrio()

		return true
	}

	return false
}

// clicarMenuPainelAberto fecha o menu do painel; true = tratado.
func (a *app) clicarMenuPainelAberto(x, y int32) bool {
	if !panelMenu.open {
		return false
	}

	if r := a.panelMenuBtn; sobreBotaoPainel(r, x, y) {
		a.fecharMenuPainel()

		return true
	}

	a.fecharMenuPainel()

	return true
}

// clicarModoCheio trata o clique em tela cheia de painel ou so-IA.
func (a *app) clicarModoCheio(x, y int32) {
	if a.soIA {
		a.clicarModoSozinho(x, y)

		return
	}

	switch {
	case a.overLeftAuxFullBtn(x, y):
		a.togglePainelCheio()
	case a.overLeftAuxBtn(x, y):
		a.togglePainelEsquerda()
	}
}

// clicarModoSozinho trata o clique com so-IA ativo.
func (a *app) clicarModoSozinho(x, y int32) {
	if id := a.panelButtonAt(x, y); id != "" {
		a.acaoBotaoPainel(id)

		return
	}

	if a.overChatHomeButton(x, y) {
		a.voltarPainelIAInicial()

		return
	}

	if a.overSplitButton(x, y) {
		a.toggleSoIA()
	}
}

// clicarFaixaAbas trata o clique na faixa de abas.
func (a *app) clicarFaixaAbas(x, y int32) {
	if qual := a.hitBotaoJanela(x, y); qual != winNenhum {
		a.executarBotaoJanela(qual)

		return
	}

	if r := a.tabOverflowRect; r.Width() > 0 && x >= r.Left && x < r.Right {
		a.abrirMenuOverflowAbas(x, y)

		return
	}

	if i, fechar, mais, ok := a.hitFaixa(x, y); ok {
		a.clicarItemFaixa(x, y, i, fechar, mais)

		return
	}

	pReleaseCapture.Call()

	pt := struct{ x, y int32 }{x, y}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&pt)))
	pSendMessageW.Call(a.hwnd, wmNcLButtonDown, htCaption, makeLParam(pt.x, pt.y))
}

// clicarItemFaixa trata o clique num item da faixa de abas.
func (a *app) clicarItemFaixa(x, y int32, indice int, fechar, mais bool) {
	if indice >= 0 {
		if ai := a.audioRectAt(x, y); ai >= 0 {
			a.alternarMudoAba(ai)

			return
		}
	}

	switch {
	case mais:
		a.novaAba("", true)
	case fechar:
		a.fecharAba(indice)
	default:
		a.ativarAba(indice)
		a.tabDrag = indice
		a.tabDragStartX = x
		a.tabDragAtivo = false
		a.tabDrop = indice
		pSetCapture.Call(a.hwnd)
	}
}

// clicarBarraEndereco trata o clique na barra de ferramentas.
func (a *app) clicarBarraEndereco(x, y int32) {
	if id := a.hitButton(x, y); id != "" {
		a.doAction(id)

		return
	}

	if a.urlEdit != 0 && x >= a.addrFrame.Left && x < a.addrFrame.Right && !a.urlEditing {
		a.focusAddress()
	}
}

// clicarAreaPaineis trata o clique na area dos paineis e divisorias.
func (a *app) clicarAreaPaineis(x, y int32) {
	if id := a.panelButtonAt(x, y); id != "" {
		a.acaoBotaoPainel(id)

		return
	}

	if a.overLeftAuxBtn(x, y) {
		a.togglePainelEsquerda()

		return
	}

	if a.overLeftAuxSplit(x, y) {
		a.dragLeftAuxSplit = true
		pSetCapture.Call(a.hwnd)

		return
	}

	if a.overChatHomeButton(x, y) {
		a.voltarPainelIAInicial()

		return
	}

	if a.overImgToggle(x, y) {
		a.toggleImagens()

		return
	}

	if a.overImgSplit(x, y) {
		a.dragImgSplit = true
		pSetCapture.Call(a.hwnd)

		return
	}

	if a.overSplitButton(x, y) {
		a.toggleChat()

		return
	}

	if a.overSplit(x, y) {
		a.dragSplit = true
		pSetCapture.Call(a.hwnd)
	}
}

func (a *app) onMouseMove(x, y int32) {
	a.retomarFrioSobCursor(x, y)

	if !a.trackingMouse {
		tme := trackMouseEventT{cbSize: uint32(unsafe.Sizeof(trackMouseEventT{})), dwFlags: tmeLeave, hwndTrack: a.hwnd}
		if r, _, _ := pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme))); r != 0 {
			a.trackingMouse = true
		}
	}

	a.cancelarArrastoSolto()

	if a.acompanharArrastos(x, y) {
		return
	}

	id := a.identificarHover(x, y)

	a.atualizarHoverJanela(x, y)

	if y < a.tabH {
		id = a.atualizarHoverAbas(x, y)
	} else if a.hoverAbasAtivo() {
		a.zerarHoverAbas()
	}

	if id != a.hover {
		a.aplicarHoverNovo(x, y, id)
	}
}

// hoverAbasAtivo diz se algum hover de aba esta ligado.
func (a *app) hoverAbasAtivo() bool {
	return a.tabHover != -1 || a.tabHoverClose != -1 || a.tabHoverAudio != -1
}

// zerarHoverAbas limpa os hovers da faixa de abas.
func (a *app) zerarHoverAbas() {
	a.tabHover, a.tabHoverClose, a.tabHoverAudio = -1, -1, -1
	a.invalidateFaixa()
}

// cancelarArrastoSolto limpa o arrasto quando o botao soltou fora.
func (a *app) cancelarArrastoSolto() {
	if a.tabDrag < 0 || teclaPressionada(botaoEsquerdoMouse) {
		return
	}

	a.tabDrag = -1
	a.tabDragAtivo = false
	a.tabDrop = -1

	pReleaseCapture.Call()
	a.invalidateFaixa()
}

// acompanharArrastos move divisorias e abas arrastadas; true = tratado.
func (a *app) acompanharArrastos(x, y int32) bool {
	if a.dragLeftAuxSplit {
		a.arrastarPainelEsquerdo(x)

		return true
	}

	if a.dragSplit {
		a.arrastarDivisoria(x)

		return true
	}

	if a.dragImgSplit {
		a.arrastarImagens(y)

		return true
	}

	if a.tabDrag >= 0 {
		a.arrastarAba(x)

		return true
	}

	return false
}

// arrastarPainelEsquerdo redimensiona o painel esquerdo auxiliar.
func (a *app) arrastarPainelEsquerdo(x int32) {
	cr := a.clientRect()
	if cr.Right <= 0 {
		return
	}

	s := a.prenderProporcao(float64(x)/float64(cr.Right), leftPanelSplitMin, leftPanelSplitMax)
	a.st.LeftPanelSplit = s
	a.layout()
}

// prenderProporcao limita a proporcao ao intervalo permitido.
func (a *app) prenderProporcao(proporcao, minimo, maximo float64) float64 {
	if proporcao < minimo {
		return minimo
	}

	if proporcao > maximo {
		return maximo
	}

	return proporcao
}

// arrastarDivisoria move a divisoria entre abas e painel.
func (a *app) arrastarDivisoria(x int32) {
	cr := a.clientRect()
	if cr.Right <= 0 {
		return
	}

	a.st.Split = a.prenderProporcao(float64(x)/float64(cr.Right), splitMin, splitMax)
	a.layout()
}

// arrastarImagens move a divisoria do painel de imagens.
func (a *app) arrastarImagens(y int32) {
	top := a.tabH + a.tbHeight + 1
	bottom := a.clientRect().Bottom

	altura := bottom - top
	if altura <= 0 || a.imgPanel == nil {
		return
	}

	a.st.ImgSplit = a.prenderProporcao(float64(y-top)/float64(altura), imgSplitMin, imgSplitMax)
	a.layout()
}

// arrastarAba atualiza a posicao de soltura da aba arrastada.
func (a *app) arrastarAba(x int32) {
	delta := x - a.tabDragStartX
	if delta < 0 {
		delta = -delta
	}

	if !a.tabDragAtivo && delta > a.sc(5) {
		a.tabDragAtivo = true
	}

	if a.tabDragAtivo && a.tabDrag >= 0 && a.tabDrag < len(a.tabs) {
		a.ajustarSolturaAba(x)
	}
}

// ajustarSolturaAba respeita as abas fixadas na soltura.
func (a *app) ajustarSolturaAba(x int32) {
	drop := a.indiceDropAba(x)

	fixas := a.abasFixadas()
	if a.tabs[a.tabDrag].fixa {
		if drop > fixas {
			drop = fixas
		}
	} else if drop < fixas {
		drop = fixas
	}

	if drop != a.tabDrop {
		a.tabDrop = drop
		a.invalidateFaixa()
	}
}

// identificarHover descobre o id sob o cursor.
func (a *app) identificarHover(x, y int32) string {
	id := a.hitButton(x, y)
	if id == "" {
		id = a.panelButtonAt(x, y)
	}

	id = a.completarHoverAuxiliar(id, x, y)

	return a.completarHoverDivisoria(id, x, y)
}

// completarHoverAuxiliar soma os botoes do painel esquerdo ao hover.
func (a *app) completarHoverAuxiliar(id string, x, y int32) string {
	switch {
	case id == "" && a.overLeftAuxFullBtn(x, y):
		return idLeftAuxFull
	case id == "" && a.overLeftAuxBtn(x, y):
		return idLeftAux
	}

	return id
}

// completarHoverDivisoria soma botoes da divisoria ao hover.
func (a *app) completarHoverDivisoria(id string, x, y int32) string {
	switch {
	case id == "" && a.overSplitButton(x, y):
		return idSplit
	case id == "" && a.overChatHomeButton(x, y):
		return idChatHome
	case id == "" && a.overImgToggle(x, y):
		return idImgToggle
	}

	return id
}

// atualizarHoverJanela atualiza o hover dos botoes da janela.
func (a *app) atualizarHoverJanela(x, y int32) {
	if y < a.tabH {
		if qual := a.hitBotaoJanela(x, y); qual != a.winHover {
			a.winHover = qual
			a.invalidateFaixa()
		}
	} else if a.winHover != winNenhum {
		a.winHover = winNenhum
		a.invalidateFaixa()
	}
}

// atualizarHoverAbas atualiza hover de abas e devolve o rotulo.
func (a *app) atualizarHoverAbas(x, y int32) string {
	abaIdx, fechar, mais, _ := a.hitFaixa(x, y)

	novoHover, novoClose, novoAudio := a.calcularHoverAba(x, y, abaIdx, fechar, mais)
	if novoHover != a.tabHover || novoClose != a.tabHoverClose || novoAudio != a.tabHoverAudio {
		a.tabHover, a.tabHoverClose, a.tabHoverAudio = novoHover, novoClose, novoAudio
		a.invalidateFaixa()
	}

	return a.rotuloHoverAba(x, y, mais, novoHover, novoClose, novoAudio)
}

// calcularHoverAba resolve abas e X e audio sob o cursor.
func (a *app) calcularHoverAba(x, y int32, abaIdx int, fechar, mais bool) (int, int, int) {
	novoHover, novoClose, novoAudio := -1, -1, -1
	if mais {
		novoHover = indiceHoverNovaAba
	} else if abaIdx >= 0 {
		novoHover = abaIdx
		if fechar {
			novoClose = abaIdx
		} else if ai := a.audioRectAt(x, y); ai >= 0 {
			novoAudio = ai
		}
	}

	return novoHover, novoClose, novoAudio
}

// rotuloHoverAba escolhe o rotulo do hover na faixa de abas.
func (a *app) rotuloHoverAba(x, y int32, mais bool, novoHover, novoClose, novoAudio int) string {
	r := a.tabOverflowRect

	switch {
	case r.Width() > 0 && x >= r.Left && x < r.Right:
		return "tab:overflow"
	case mais:
		return "tab:nova"
	case novoClose >= 0:
		return "tab:fechar"
	case novoAudio >= 0:
		return "tab:audio"
	case novoHover >= 0:
		return "tab:" + strconv.Itoa(novoHover)
	}

	return a.rotuloHoverJanela(x, y)
}

// rotuloHoverJanela escolhe o rotulo dos botoes da janela.
func (a *app) rotuloHoverJanela(x, y int32) string {
	if qual := a.hitBotaoJanela(x, y); qual != winNenhum {
		switch qual {
		case winMinimiza:
			return "win:minimizar"
		case winMaximiza:
			return "win:maximizar"
		case winFecha:
			return "win:fechar"
		}
	}

	return ""
}

// aplicarHoverNovo troca o hover e agenda dica e repintura.
func (a *app) aplicarHoverNovo(x, y int32, id string) {
	anterior := a.hover
	a.hover = id

	faixa := rectT{0, 0, a.clientRect().Right, a.tabH + a.tbHeight + 1}
	pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&faixa)), 0)

	a.invalidarBotoesHover(id, anterior)
	a.scheduleTip(id, a.retanguloDicaHover(x, y, id))

	if id == "" && y >= a.tabH && y < a.tabH+a.tbHeight && x >= a.addrFrame.Left && x < a.addrFrame.Right {
		a.scheduleTip("addr", a.addrFrame)
	}
}

// invalidarBotoesHover repinta os botoes afetados pela troca de hover.
func (a *app) invalidarBotoesHover(id, anterior string) {
	if strings.HasPrefix(id, idLeftAux) || strings.HasPrefix(anterior, idLeftAux) {
		for _, rr := range []rectT{a.leftAuxBtn, a.leftAuxFullBtn} {
			if rr.Width() > 0 {
				pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rr)), 0)
			}
		}
	}

	for _, r := range []rectT{a.splitBtn, a.chatHomeBtn, a.imgToggleBtn} {
		if r.Width() > 0 {
			pInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
		}
	}
}

// retanguloDicaHover escolhe a area da dica do hover atual.
func (a *app) retanguloDicaHover(x, _ int32, id string) rectT {
	retangulo := rectT{x - a.sc(10), a.tabH, x + a.sc(10), a.tabH + a.tbHeight}

	if nr, ok := a.retanguloDicaPainel(id); ok {
		retangulo = nr
	}

	for _, b := range a.buttons {
		if b.id == id {
			retangulo = b.rect
		}
	}

	return retangulo
}

// retanguloDicaPainel escolhe a area dos botoes do painel; false = outro.
func (a *app) retanguloDicaPainel(id string) (rectT, bool) {
	switch id {
	case idSplit:
		return rectT{a.splitX - a.rail/2, a.tabH, a.splitX + a.rail/2, a.tabH + a.tbHeight}, true
	case idChatHome:
		return a.chatHomeBtn, true
	case idImgToggle:
		return a.imgToggleBtn, true
	case idPainelMenu:
		return a.panelMenuBtn, true
	case idPainelTranslate:
		return a.panelTranslateBtn, true
	}

	return a.retanguloDicaPainelAux(id)
}

// retanguloDicaPainelAux escolhe a area auxiliar; false = padrao.
func (a *app) retanguloDicaPainelAux(id string) (rectT, bool) {
	switch id {
	case idPainelSend:
		return a.panelSendBtn, true
	case idPainelReload:
		return a.panelReloadBtn, true
	case idPainelFull:
		return a.panelFullBtn, true
	case idLeftAux:
		return a.leftAuxBtn, true
	case idLeftAuxFull:
		return a.leftAuxFullBtn, true
	}

	return rectT{}, false
}

func (a *app) handleShortcut(tecla uint32, ctrl, shift, alt bool) bool {
	if a.handleShortcutInner(tecla, ctrl, shift, alt) {
		a.limpezaAposAcao("atalho")

		return true
	}

	return false
}

func (a *app) handleShortcutInner(tecla uint32, ctrl, shift, alt bool) bool {
	switch {
	case a.atalhoAbas(tecla, ctrl, shift, alt):
		return true
	case a.atalhoPainelIA(tecla, ctrl, shift, alt):
		return true
	case a.atalhoPagina(tecla, ctrl, shift):
		return true
	case a.atalhoHistorico(tecla, alt):
		return true
	case a.atalhoSistema(tecla, ctrl, shift):
		return true
	case a.atalhoEscape(tecla):
		return true
	}

	return false
}

// atalhoAbas trata atalhos de abas e janelas; true = tratado.
func (a *app) atalhoAbas(tecla uint32, ctrl, shift, alt bool) bool {
	if a.atalhoAbasCriar(tecla, ctrl, shift) {
		return true
	}

	if a.atalhoAbasFechar(tecla, ctrl, shift) {
		return true
	}

	return a.atalhoAbasNavegar(tecla, ctrl, shift, alt)
}

// atalhoAbasCriar trata criar aba e janela; true = tratado.
func (a *app) atalhoAbasCriar(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && tecla == vkT && !shift:
		a.novaAba("", true)

		return true
	case ctrl && shift && tecla == vkN:
		a.novaJanela(flagPrivado)

		return true
	case ctrl && tecla == vkN:
		a.novaJanela("")

		return true
	}

	return false
}

// atalhoAbasFechar trata fechar aba e sair; true = tratado.
func (a *app) atalhoAbasFechar(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && tecla == vkW && !shift:
		a.fecharAbaAtivaApos()

		return true
	case ctrl && shift && tecla == vkW:
		a.depois(func() { a.saveAndQuit() })

		return true
	}

	return false
}

// fecharAbaAtivaApos agenda o fechamento da aba ativa.
func (a *app) fecharAbaAtivaApos() {
	var alvo *tab
	if a.ativa >= 0 && a.ativa < len(a.tabs) {
		alvo = a.tabs[a.ativa]
	}

	a.depois(func() {
		if alvo == nil {
			return
		}

		if i := a.indiceDaAba(alvo); i >= 0 {
			a.fecharAba(i)
		}
	})
}

// atalhoAbasNavegar trata navegar e reabrir abas; true = tratado.
func (a *app) atalhoAbasNavegar(tecla uint32, ctrl, shift, alt bool) bool {
	if a.atalhoAbasTrocar(tecla, ctrl, shift) {
		return true
	}

	return a.atalhoAbasIrPara(tecla, ctrl, shift, alt)
}

// atalhoAbasTrocar trata trocar de aba; true = tratado.
func (a *app) atalhoAbasTrocar(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && tecla == vkTab:
		if shift {
			a.proximaAba(-1)
		} else {
			a.proximaAba(1)
		}

		return true
	case ctrl && tecla == vkPrior:
		a.proximaAba(-1)

		return true
	case ctrl && tecla == vkNext:
		a.proximaAba(1)

		return true
	}

	return false
}

// atalhoAbasIrPara trata reabrir e ir a aba numerica; true = tratado.
func (a *app) atalhoAbasIrPara(tecla uint32, ctrl, shift, alt bool) bool {
	switch {
	case ctrl && shift && !alt && tecla == vkT:
		a.reabrirGuiaFechada()

		return true
	case ctrl && tecla >= digitoUmAba && tecla <= digitoNoveAba:
		a.ativarAbaNumerica(tecla)

		return true
	}

	return false
}

// ativarAbaNumerica ativa a aba pelo digito do atalho.
func (a *app) ativarAbaNumerica(tecla uint32) {
	indice := int(tecla - digitoUmAba)
	if tecla == digitoNoveAba {
		indice = len(a.tabs) - 1
	}

	if indice >= 0 && indice < len(a.tabs) {
		a.ativarAba(indice)
	}
}

// atalhoPainelIA trata atalhos de paineis e IA; true = tratado.
func (a *app) atalhoPainelIA(tecla uint32, ctrl, shift, alt bool) bool {
	if a.atalhoPainel(tecla, ctrl, shift) {
		return true
	}

	return a.atalhoLeituraIA(tecla, ctrl, shift, alt)
}

// atalhoPainel trata clipboard, YouTube e reabrir painel; true = tratado.
func (a *app) atalhoPainel(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && shift && tecla == vkH:
		a.alternarClipboard()

		return true
	case ctrl && shift && tecla == vkY:
		a.alternarYouTube()

		return true
	}

	return false
}

// atalhoLeituraIA trata traducao e leitura para o chat; true = tratado.
func (a *app) atalhoLeituraIA(tecla uint32, ctrl, shift, alt bool) bool {
	if ctrl && shift && alt && tecla == vkT {
		a.alternarTraducao()

		return true
	}

	return a.atalhoLeituraChat(tecla, ctrl, shift)
}

// atalhoLeituraChat trata leitura da pagina para o chat; true = tratado.
func (a *app) atalhoLeituraChat(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && shift && tecla == vkL:
		a.lerPaginaParaChat("artigo")

		return true
	case ctrl && shift && tecla == vkP:
		a.lerPaginaParaChat(modoTudo)

		return true
	}

	return false
}

// atalhoPagina trata atalhos de pagina e recarga; true = tratado.
func (a *app) atalhoPagina(tecla uint32, ctrl, shift bool) bool {
	if a.atalhoPaginaFoco(tecla, ctrl) {
		return true
	}

	if a.atalhoPaginaImprimir(tecla, ctrl) {
		return true
	}

	return a.atalhoPaginaRecarga(tecla, ctrl, shift)
}

// atalhoPaginaFoco trata foco e salvar pagina; true = tratado.
func (a *app) atalhoPaginaFoco(tecla uint32, ctrl bool) bool {
	switch {
	case ctrl && tecla == vkL:
		a.focusAddress()

		return true
	case ctrl && tecla == vkS:
		a.salvarPagina()

		return true
	}

	return false
}

// atalhoPaginaImprimir trata imprimir, downloads e busca; true = tratado.
func (a *app) atalhoPaginaImprimir(tecla uint32, ctrl bool) bool {
	switch {
	case ctrl && tecla == vkP:
		a.imprimirPDF()

		return true
	case ctrl && tecla == vkJ:
		a.abrirPagina(idDownloads)

		return true
	case ctrl && tecla == vkF:
		a.abrirBuscaNativa()

		return true
	}

	return false
}

// atalhoPaginaRecarga trata dados e recarga; true = tratado.
func (a *app) atalhoPaginaRecarga(tecla uint32, ctrl, shift bool) bool {
	if ctrl && shift && (tecla == vkDelete || tecla == teclaRetrocesso) {
		a.abrirPagina("dados")

		return true
	}

	return a.atalhoRecarregar(tecla, ctrl, shift)
}

// atalhoRecarregar trata recarga fria e normal; true = tratado.
func (a *app) atalhoRecarregar(tecla uint32, ctrl, shift bool) bool {
	switch {
	case ctrl && shift && tecla == vkR:
		a.recarregarFrio()

		return true
	case ctrl && tecla == vkR, tecla == vkF5:
		a.recarregarAtalho()

		return true
	}

	return false
}

// recarregarAtalho recarrega ou para conforme o estado da pagina.
func (a *app) recarregarAtalho() {
	if a.left != nil && a.left.readerAtivo {
		a.sairDoLeitor(a.left)

		return
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtPagAbrindoInput)

		return
	}

	if a.loading {
		a.left.wv.Stop()
		a.loading = false
		a.invalidateBar()
	} else {
		a.loading = true
		a.invalidateBar()
		a.left.wv.Reload()
	}
}

// atalhoHistorico trata voltar, avancar e inicio; true = tratado.
func (a *app) atalhoHistorico(tecla uint32, alt bool) bool {
	switch {
	case alt && tecla == 0x24:
		a.irParaInicio()

		return true
	case alt && tecla == vkLeft:
		a.voltarHistorico()

		return true
	case alt && tecla == vkRight:
		a.avancarHistorico()

		return true
	}

	return false
}

// alvoHistorico resolve o painel do historico, preferindo o YouTube.
func (a *app) alvoHistorico() *pane {
	alvo := a.left
	if a.ytAberto && !a.ytOculto && a.ytPane != nil && a.ytPane.ready && a.ytPane.wv != nil {
		alvo = a.ytPane
	}

	return alvo
}

// sairDoLeitorSeAtivo sai do leitor quando ele esta ativo; true = saiu.
func (a *app) sairDoLeitorSeAtivo(alvo *pane) bool {
	if alvo == a.left && a.left != nil && a.left.readerAtivo {
		a.sairDoLeitor(a.left)

		return true
	}

	return false
}

// voltarHistorico volta uma pagina no painel do historico.
func (a *app) voltarHistorico() {
	alvo := a.alvoHistorico()

	if a.sairDoLeitorSeAtivo(alvo) {
		return
	}

	if alvo != nil && alvo.wv != nil {
		alvo.wv.GoBack()
	}
}

// avancarHistorico avanca uma pagina no painel do historico.
func (a *app) avancarHistorico() {
	alvo := a.alvoHistorico()

	if a.sairDoLeitorSeAtivo(alvo) {
		return
	}

	if alvo != nil && alvo.wv != nil {
		alvo.wv.GoForward()
	}
}

// irParaInicio volta a pagina inicial limpando o leitor.
func (a *app) irParaInicio() {
	if a.left != nil {
		limparLeitor(a.left)
	}

	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.NavigateToString(startPageHTML)
		a.loading = false
		a.invalidateBar()
		a.layout()
	}
}

// atalhoSistema trata devtools, zoom, foco e tela; true = tratado.
func (a *app) atalhoSistema(tecla uint32, ctrl, shift bool) bool {
	if a.atalhoSistemaDevtools(tecla, ctrl, shift) {
		return true
	}

	if a.atalhoSistemaZoom(tecla, ctrl) {
		return true
	}

	return a.atalhoSistemaTela(tecla, ctrl)
}

// atalhoSistemaDevtools abre o devtools; true = tratado.
func (a *app) atalhoSistemaDevtools(tecla uint32, ctrl, shift bool) bool {
	if tecla != vkF12 && (!ctrl || !shift || tecla != 0x49) {
		return false
	}

	if p := a.focused(); p != nil && p.ready && p.wv != nil {
		p.wv.OpenDevTools()
	}

	return true
}

// atalhoSistemaZoom trata zoom da pagina; true = tratado.
func (a *app) atalhoSistemaZoom(tecla uint32, ctrl bool) bool {
	switch {
	case ctrl && (tecla == vkOEMPlus || tecla == vkAdd):
		a.setZoom(a.st.Zoom + passoZoomTeclado)

		return true
	case ctrl && (tecla == vkOEMMinus || tecla == vkSubtract):
		a.setZoom(a.st.Zoom - passoZoomTeclado)

		return true
	case ctrl && tecla == vk0:
		a.setZoom(1.0)

		return true
	}

	return false
}

// atalhoSistemaTela trata chat, UBO, maximizar e foco; true = tratado.
func (a *app) atalhoSistemaTela(tecla uint32, ctrl bool) bool {
	switch {
	case ctrl && tecla == vkB:
		a.doAction(idChat)

		return true
	case ctrl && tecla == vkU:
		a.doAction(idUbo)

		return true
	case tecla == vkF11:
		a.toggleMaximizar()

		return true
	case tecla == vkF6:
		a.ciclarFocoPainel()

		return true
	}

	return false
}

// atalhoEscape trata o escape por camadas; true = tratado.
func (a *app) atalhoEscape(tecla uint32) bool {
	if tecla != vkEscape {
		return false
	}

	if a.atalhoEscapeSobreposicao() {
		return true
	}

	return a.atalhoEscapePainel()
}

// atalhoEscapeSobreposicao fecha sobreposicoes; true = tratado.
func (a *app) atalhoEscapeSobreposicao() bool {
	if a.cancelarRecorteTimer() {
		return true
	}

	if panelMenu.open {
		a.fecharMenuPainel()

		return true
	}

	if clipUI.ativo {
		a.fecharClipboard()

		return true
	}

	if a.escolhaURL != "" {
		a.escolhaURL = ""
		a.closeMenu()
		a.dizerErrof(txtLinkNaoAberto)
		a.invalidateBar()

		return true
	}

	if menu.open {
		if menu.subPai >= 0 {
			a.closeSubMenu()
		} else {
			a.closeMenu()
		}

		return true
	}

	if a.extFloatOn {
		a.depois(func() { a.fecharPainelFlutuante() })

		return true
	}

	if a.popupOn {
		a.depois(func() {
			a.fecharPopup()
			a.layout()
		})

		return true
	}

	if aviso.visivel {
		a.descartarAviso()

		return true
	}

	return false
}

// atalhoEscapePainel fecha paineis cheios; true = tratado.
func (a *app) atalhoEscapePainel() bool {
	if a.ytAberto {
		a.fecharYouTube()

		return true
	}

	if a.soIA {
		a.toggleSoIA()

		return true
	}

	if a.painelCheio {
		a.togglePainelCheio()

		return true
	}

	if a.left != nil && a.left.readerAtivo && (a.focus == idLeft || a.focus == "") {
		a.sairDoLeitor(a.left)

		return true
	}

	return false
}

func (a *app) focusAddress() {
	if a == nil || a.urlEdit == 0 {
		return
	}

	if a.soIA || a.painelCheioAtivo() {
		a.dizerf(txtEndTelaCheia)

		return
	}

	a.syncAddressBar()
	defer a.limpezaAposAcao("focar-endereco")

	pShowWindow.Call(a.urlEdit, swShow)
	pSetFocus.Call(a.urlEdit)

	const emSetSel = 0x00B1
	pSendMessageW.Call(a.urlEdit, emSetSel, 0, ^uintptr(0))

	a.depois(func() {
		a.confirmarSelecaoEndereco()
	})
	a.invalidateBar()
}

// confirmarSelecaoEndereco garante o texto todo selecionado no foco.
func (a *app) confirmarSelecaoEndereco() {
	if a.quitting || a.urlEdit == 0 || !a.urlEditing {
		return
	}

	if f, _, _ := pGetFocus.Call(); f != a.urlEdit {
		return
	}

	const emGetSel = 0x00B0

	var ini, fim uint32
	pSendMessageW.Call(a.urlEdit, emGetSel, uintptr(unsafe.Pointer(&ini)), uintptr(unsafe.Pointer(&fim)))

	if ini != fim || a.editText() == "" {
		return
	}

	const emSetSel = 0x00B1
	pSendMessageW.Call(a.urlEdit, emSetSel, 0, ^uintptr(0))

	const emScrollCaret = 0x00B7
	pSendMessageW.Call(a.urlEdit, emScrollCaret, 0, 0)
}

func (a *app) teclarEnterCDP(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if dn, err := cdpTeclaEnter("keyDown"); err == nil {
		p.wv.CallDevTools("Input.dispatchKeyEvent", dn)
	}

	if up, err := cdpTeclaEnter("keyUp"); err == nil {
		p.wv.CallDevTools("Input.dispatchKeyEvent", up)
	}
}

func (a *app) hitFaixa(x, y int32) (int, bool, bool, bool) {
	if y < 0 || y >= a.tabH {
		return -1, false, false, false
	}

	if a.hitBotaoJanela(x, y) != winNenhum {
		return -1, false, false, false
	}

	for indice := range a.tabs {
		if indice >= len(a.tabRects) {
			continue
		}

		r := a.tabRects[indice]
		if a.pularAbaFaixa(r) {
			continue
		}

		if indice, fechar, ok := a.faixaEmAba(x, y, indice, r); ok {
			return indice, fechar, false, true
		}
	}

	if a.sobreBotaoNovaAba(x, y) {
		return -1, false, true, true
	}

	return -1, false, false, false
}

// pularAbaFaixa diz se a aba sai da area clicavel da faixa.
func (a *app) pularAbaFaixa(r rectT) bool {
	return r.Width() <= 0 || (a.tabLimitePintura > 0 && r.Right > a.tabLimitePintura)
}

// faixaEmAba identifica clique na aba; true = acertou.
func (a *app) faixaEmAba(x, y int32, indice int, retangulo rectT) (int, bool, bool) {
	if indice < len(a.tabCloseRects) {
		cr := a.tabCloseRects[indice]
		if a.mostraFecharAba(indice, retangulo) && a.dentroFecharAba(cr, x, y) {
			return indice, true, true
		}
	}

	if x >= retangulo.Left && x < retangulo.Right && y >= 0 && y < a.tabH {
		return indice, false, true
	}

	return 0, false, false
}

// mostraFecharAba diz se o X da aba aparece no estado atual.
func (a *app) mostraFecharAba(indice int, retangulo rectT) bool {
	if a.tabHoverClose == indice {
		return true
	}

	if indice == a.ativa && retangulo.Width() >= a.sc(112) {
		return true
	}

	return a.tabHover == indice && retangulo.Width() >= a.sc(92)
}

// dentroFecharAba diz se o ponto alcanca o X com margem.
func (a *app) dentroFecharAba(cr rectT, x, y int32) bool {
	return cr.Width() > 0 && x >= cr.Left-a.sc(2) && x < cr.Right+a.sc(2) &&
		y >= cr.Top-a.sc(2) && y < cr.Bottom+a.sc(2)
}

// sobreBotaoNovaAba diz se o ponto esta no botao de nova aba.
func (a *app) sobreBotaoNovaAba(x, y int32) bool {
	r := a.tabNewRect

	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) audioRectAt(x, y int32) int {
	for indice, retangulo := range a.tabAudioRects {
		if retangulo.Width() <= 0 {
			continue
		}

		if indice < len(a.tabRects) && a.tabRects[indice].Width() < a.sc(68) {
			continue
		}

		if x >= retangulo.Left && x < retangulo.Right && y >= retangulo.Top && y < retangulo.Bottom {
			return indice
		}
	}

	return -1
}

func (a *app) onRButtonUp(x, y int32) {
	if a.soIA || a.painelCheioAtivo() {
		return
	}

	if y < 0 || y >= a.tabH || a.hitBotaoJanela(x, y) != winNenhum {
		return
	}

	a.abrirMenuContextoAba(a.indiceAbaEmFaixa(x, y), x, y)
}

// indiceAbaEmFaixa acha a aba sob o ponto; -1 = nenhuma.
func (a *app) indiceAbaEmFaixa(x, y int32) int {
	for i, r := range a.tabRects {
		if x >= r.Left && x < r.Right && y >= 0 && y < a.tabH {
			return i
		}
	}

	return -1
}

func (a *app) hitBotaoJanela(x, y int32) int {
	if y < 0 || y >= a.tabH {
		return winNenhum
	}

	dentro := func(r rectT) bool { return r.Width() > 0 && x >= r.Left && x < r.Right }
	switch {
	case dentro(a.winClose):
		return winFecha
	case dentro(a.winMax):
		return winMaximiza
	case dentro(a.winMin):
		return winMinimiza
	}

	return winNenhum
}

func (a *app) sobreControle(x, y int32) bool {
	if a.hitBotaoJanela(x, y) != winNenhum {
		return true
	}

	if a.winClock.Width() > 0 && dentroRect(a.winClock, x, y) {
		return true
	}

	if _, _, _, ok := a.hitFaixa(x, y); ok {
		return true
	}

	return false
}

func teclaPressionada(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))

	return int16(r) < 0
}
