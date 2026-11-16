//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const (
	esperaLimpezaPainelEsq = 1500 * time.Millisecond
	esperaLimpezaCheio     = 600 * time.Millisecond
	meioPainelEsq          = 2
)

func (a *app) abrirPainelEsquerda() {
	if !a.st.LeftPanelVisible || a.hwnd == 0 {
		return
	}

	if a.leftAux == nil {
		a.leftAux = &pane{}
	}

	a.layout()
	a.invalidateBar()
}

func (a *app) fecharPainelEsquerda() {
	a.painelCheio = false
	if a.leftAux != nil {
		a.leftAux.rect = Rect{}
	}

	a.layout()
	a.invalidateBar()
	a.agendarLimpezaMemoria(true, esperaLimpezaPainelEsq)
}

func (a *app) togglePainelEsquerda() {
	a.st.LeftPanelVisible = !a.st.LeftPanelVisible
	if a.st.LeftPanelVisible {
		a.abrirPainelEsquerda()
	} else {
		a.fecharPainelEsquerda()
	}

	a.layout()
	a.persist()
}

func (a *app) painelCheioAtivo() bool {
	return a.st.LeftPanelVisible && a.painelCheio && !a.soIA &&
		(!a.ytAberto || a.ytOculto)
}

func (a *app) painelCheioLiberado() bool {
	if a.ytAberto && !a.ytOculto {
		a.dizerf(txtPainelEsqFecheModo)

		return false
	}

	return true
}

func (a *app) prepararPainelCheio() {
	if !a.st.LeftPanelVisible {
		a.st.LeftPanelVisible = true
		a.abrirPainelEsquerda()
	}
}

func (a *app) suspenderTudoParaCheio() {
	a.soIA = false
	a.fecharPopup()
	a.hideTip()

	if a.urlEdit != 0 {
		pShowWindow.Call(a.urlEdit, swHide)
		a.urlEditing = false
	}

	for _, t := range a.tabs {
		suspenderPane(t.pane)
	}

	for _, p := range []*pane{a.right, a.imgPanel} {
		suspenderPane(p)
	}

	a.status = txtPainelEsqFull
	a.agendarLimpezaMemoria(true, esperaLimpezaCheio)
}

func (a *app) retomarPainelPrincipalDoCheio() {
	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.resumirPane(a.left)
	} else if a.ativa >= 0 && a.ativa < len(a.tabs) {
		a.ativarAba(a.ativa)
	}
}

func (a *app) retomarPaineisDireitaDoCheio() {
	if !a.st.RightVisible {
		return
	}

	if p := a.right; p != nil && p.ready && p.wv != nil {
		a.resumirPane(p)
	} else {
		a.abrirPainelChat()
	}

	if a.st.ImgVisible && a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil {
		a.resumirPane(a.imgPanel)
	}
}

func (a *app) retomarTudoDoCheio() {
	a.retomarPainelPrincipalDoCheio()
	a.retomarPaineisDireitaDoCheio()
	a.retomarTodosFrios()
	a.status = txtPainelEsqNormal
}

func (a *app) togglePainelCheio() {
	if !a.painelCheioLiberado() {
		return
	}

	a.prepararPainelCheio()

	a.painelCheio = !a.painelCheio
	if a.painelCheio {
		a.suspenderTudoParaCheio()
	} else {
		a.retomarTudoDoCheio()
	}

	a.layout()
	a.persist()
	a.invalidateBar()
}

func (a *app) posicionarBotoesPainel(top, bottom int32) {
	a.leftAuxBtn = rectT{}

	a.leftAuxFullBtn = rectT{}
	if a.leftAuxSplitX <= 0 {
		return
	}

	aberto := a.leftAux != nil && a.leftAux.rect.Width() > 0

	larguraBotao := a.sc(22)
	if !aberto {
		larguraBotao = a.sc(railFinoLogico)
	}

	if larguraBotao > a.rail {
		larguraBotao = a.rail
	}

	alturaBotao := a.sc(30)
	gap := a.sc(4)
	centro := top + (bottom-top)/meioPainelEsq

	x := max(a.leftAuxSplitX-larguraBotao/meioPainelEsq, 0)

	if !aberto {
		a.leftAuxBtn = rectT{
			x, centro - alturaBotao/meioPainelEsq, x + larguraBotao,
			centro - alturaBotao/meioPainelEsq + alturaBotao,
		}

		return
	}

	y := centro - alturaBotao/meioPainelEsq - (alturaBotao + gap)
	a.leftAuxFullBtn = rectT{x, y, x + larguraBotao, y + alturaBotao}
	y += alturaBotao + gap
	a.leftAuxBtn = rectT{x, y, x + larguraBotao, y + alturaBotao}
}

func (a *app) paintEspacoAppsD2D(recorte rectT) {
	mostrar := (a.st.LeftPanelVisible || a.painelCheio) && !a.soIA && (!a.ytAberto || a.ytOculto)
	if !mostrar || a.leftAux == nil {
		return
	}

	area := rectT(a.leftAux.rect)
	if area.Width() <= 10 || area.Height() <= 10 {
		return
	}

	if area.Bottom > recorte.Bottom {
		area.Bottom = recorte.Bottom
	}

	disp := &d2dMain
	disp.retB(area, a.brSurface)
	meio := (area.Top + area.Bottom) / meioPainelEsq
	disp.textoUI(txtEspacoAppsTitulo, rectT{area.Left, meio - a.sc(30), area.Right, meio - a.sc(8)}, corTexto, false, true)
	disp.textoUI(
		txtEspacoAppsSub, rectT{area.Left, meio - a.sc(4), area.Right, meio + a.sc(18)}, corTextoFaint, false, true,
	)
}

func (a *app) urlDaBarraAtual() string {
	if txt := strings.TrimSpace(a.editText()); txt != "" {
		return a.resolverInput(txt)
	}

	if a.left != nil && a.left.ready {
		if endereco := a.left.wv.Source(); urlUtilizavel(endereco) {
			return endereco
		}
	}

	return ""
}

const jsLimparArmazenamentoSite = `(function(){try{localStorage.clear();sessionStorage.clear();` +
	`if(window.caches&&caches.keys){caches.keys().then(function(ks){ks.forEach(function(k){caches.delete(k);});});}` +
	`if(window.indexedDB&&indexedDB.databases){indexedDB.databases().then(function(dbs){` +
	`dbs.forEach(function(db){indexedDB.deleteDatabase(db.name);});});}` +
	`if(navigator.serviceWorker&&navigator.serviceWorker.getRegistrations){` +
	`navigator.serviceWorker.getRegistrations().then(function(rs){rs.forEach(function(r){r.unregister();});});}` +
	`return 'ok';}catch(e){return 'erro';}})()`

func (a *app) alvosLimpezaCookies() []*pane {
	alvos := []*pane{}
	if a.left != nil && a.left.ready && a.left.wv != nil {
		alvos = append(alvos, a.left)
	}

	return alvos
}

// executarLimpezaCookies varre os paineis e soma apagados/restantes.
func (a *app) executarLimpezaCookies(alvos []*pane, endereco string) (int, int, bool) {
	var total, restaram int

	falhou := true

	for _, painel := range alvos {
		apagados, restantes, err := a.limparCookiesDoDominioEm(painel, endereco)
		if err != nil {
			continue
		}

		falhou = false
		total += apagados
		restaram += restantes
	}

	return total, restaram, falhou
}

// anunciarLimpezaCookies mostra o resultado e recarrega quando zerou.
func (a *app) anunciarLimpezaCookies(host string, total, restaram int, falhou bool) {
	if falhou {
		a.dizerErrof(txtFmtLerCookies, host)

		return
	}

	a.limparArmazenamentoDoSite(host)

	if a.left != nil && a.left.ready && a.left.wv != nil &&
		dominioDeURL(a.left.wv.Source()) == host {
		a.left.wv.Reload()
	}

	if restaram > 0 {
		a.dizerf(txtFmtCookiesRestam, host, total, restaram)

		return
	}

	if total == 0 {
		a.dizerOkf(txtCookiesNenhum, host)

		return
	}

	a.dizerOkf(txtFmtCookiesOK, host, total)
}

func (a *app) limparCookiesDoDominio() {
	endereco := a.urlDaBarraAtual()

	host := dominioDeURL(endereco)
	if host == "" {
		a.dizerf(txtCookiesSemDom)

		return
	}

	a.permissoesMu.Lock()
	if a.permissoes != nil {
		delete(a.permissoes, host)
	}
	a.permissoesMu.Unlock()

	alvos := a.alvosLimpezaCookies()
	if len(alvos) == 0 {
		a.dizerf(txtCookiesSemPainel)

		return
	}

	a.dizerProgressoIndetf(txtFmtLimpandoCook, host)

	go func() {
		total, restaram, falhou := a.executarLimpezaCookies(alvos, endereco)

		a.depois(func() { a.anunciarLimpezaCookies(host, total, restaram, falhou) })
	}()
}

func (a *app) limparCookiesDoDominioEm(p *pane, endereco string) (int, int, error) {
	cru, err := a.chamarCDP(p, "Network.getCookies", fmt.Sprintf(`{"urls":[%q]}`, endereco))
	if err != nil {
		return 0, 0, err
	}

	var resposta cdpCookiesResp
	if err := json.Unmarshal([]byte(cru), &resposta); err != nil {
		return 0, 0, erroTextoF("cookies invalidos: %w", err)
	}

	apagados := 0

	for _, cookie := range resposta.Cookies {
		params := fmt.Sprintf(`{"name":%q,"domain":%q}`, cookie.Name, cookie.Domain)
		if cookie.Path != "" {
			params = fmt.Sprintf(`{"name":%q,"domain":%q,"path":%q}`, cookie.Name, cookie.Domain, cookie.Path)
		}

		if _, err := a.chamarCDP(p, "Network.deleteCookies", params); err == nil {
			apagados++
		}
	}

	restantes := 0

	if cru2, err := a.chamarCDP(p, "Network.getCookies", fmt.Sprintf(`{"urls":[%q]}`, endereco)); err == nil {
		var resposta2 cdpCookiesResp
		if json.Unmarshal([]byte(cru2), &resposta2) == nil {
			restantes = len(resposta2.Cookies)
		}
	}

	return apagados, restantes, nil
}

func (a *app) limparArmazenamentoDoSite(host string) {
	if host == "" {
		return
	}

	for _, p := range []*pane{a.left} {
		if p == nil || !p.ready || p.wv == nil {
			continue
		}

		if dominioDeURL(p.wv.Source()) != host {
			continue
		}

		p.wv.EvalJS(jsLimparArmazenamentoSite, func(error, string) {})
	}
}

// prenderNavegacaoEmAbaFria guarda a URL quando a aba ainda nao criou o pane.
func (a *app) prenderNavegacaoEmAbaFria(endereco string) {
	if a.ativa < 0 || a.ativa >= len(a.tabs) {
		return
	}

	if t := a.tabs[a.ativa]; t != nil && t.pane == a.left {
		t.url = endereco
		if t.pane.wv == nil && !t.criando {
			a.carregarAba(t)
		}
	}
}

func (a *app) navigateLeft(input string) {
	endereco := LimparTrackingURL(a.resolverInput(input))
	if endereco == "" || a.left == nil {
		return
	}
	defer a.limpezaAposAcao("navegar")

	if !a.left.ready {
		a.prenderNavegacaoEmAbaFria(endereco)

		return
	}

	a.acordarAbaPrincipal()
	limparLeitor(a.left)
	a.readerBtn.on = false
	a.invalidateBar()
	a.left.wv.Navigate(endereco)
}

func (a *app) layoutPainelCheio(recorte rectT, top, bottom int32) {
	a.splitXAntes = a.splitX
	a.splitX = recorte.Right

	a.leftAuxSplitX = a.rail / meioPainelEsq
	if a.leftAux != nil {
		a.leftAux.rect = Rect{Left: a.rail, Top: top, Right: recorte.Right, Bottom: bottom}
	}

	a.left.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	if a.right != nil {
		a.right.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	if a.imgPanel != nil {
		a.imgPanel.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	if a.popup != nil {
		a.popup.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
	}

	if a.ytPane != nil {
		if a.ytAberto && a.ytOculto {
			esq := -a.sc(4000)
			a.ytPane.rect = Rect{Left: esq, Top: top, Right: esq + a.sc(640), Bottom: top + a.sc(360)}
		} else {
			a.ytPane.rect = Rect{Left: 0, Top: top, Right: 0, Bottom: top}
		}
	}

	a.imgSplitY = 0
	a.splitBtn = rectT{}
	a.chatHomeBtn = rectT{}
	a.panelMenuBtn = rectT{}
	a.panelTranslateBtn = rectT{}
	a.panelSendBtn = rectT{}
	a.panelFullBtn = rectT{}
	a.panelReloadBtn = rectT{}

	a.imgToggleBtn = rectT{}
	for _, t := range a.tabs {
		if t.pane != nil {
			t.pane.rect = a.left.rect
		}
	}

	a.layoutFaixa()
	a.posicionarBotoesPainel(top, bottom)
	a.applyBounds()
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func limparPrivadosAntigos() {
	entradas, err := os.ReadDir(dataDir)
	if err != nil {
		return
	}

	limite := time.Now().Add(-time.Hour)

	for _, entrada := range entradas {
		nome := entrada.Name()
		if !strings.HasPrefix(nome, "privado-") && !strings.HasPrefix(nome, "state-privado-") {
			continue
		}

		info, err := entrada.Info()
		if err != nil {
			continue
		}

		if pid, vivo := pidDoPerfilPrivado(nome); vivo {
			if processoVivo(pid) {
				continue
			}

			_ = os.RemoveAll(filepath.Join(dataDir, nome))

			continue
		}

		if info.ModTime().Before(limite) {
			_ = os.RemoveAll(filepath.Join(dataDir, nome))
		}
	}
}

func pidDoPerfilPrivado(nome string) (int, bool) {
	base := strings.TrimPrefix(strings.TrimPrefix(nome, "state-"), "privado-")

	indice := strings.IndexByte(base, '-')
	if indice <= 0 {
		return 0, false
	}

	rest := base[indice+1:]
	if len(rest) < 9 || rest[8] != '-' {
		return 0, false
	}

	for _, digito := range rest[:8] {
		if digito < '0' || digito > '9' {
			return 0, false
		}
	}

	pid, err := strconv.Atoi(base[:indice])
	if err != nil || pid <= 0 {
		return 0, false
	}

	return pid, true
}

func processoVivo(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var buf [windows.MAX_PATH]uint16

	tamanho := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &tamanho); err != nil {
		return true
	}

	exe := filepath.Base(windows.UTF16ToString(buf[:tamanho]))

	return strings.EqualFold(exe, "WEB Overdrive.exe") || strings.EqualFold(exe, "WebDrive.exe") ||
		strings.EqualFold(exe, "Overdrive.exe")
}
