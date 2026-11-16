//go:build windows

package main

import (
	"encoding/json"
	"strings"
	"time"
	"unsafe"
)

func (a *app) acaoBotaoPainel(id string) {
	switch id {
	case idPainelMenu:
		a.abrirMenuPainel()
	case idPainelTranslate:
		a.doAction("traduzirnochat")
	case idPainelSend:
		a.doAction("lerpaginaTudo")
	case idPainelFull:
		a.toggleSoIA()
	case idPainelReload:
		a.recarregarPaineisDireita()
	}
}

func (a *app) escolherOndeAbrir(endereco string, origem *pane) {
	endereco = strings.TrimSpace(endereco)
	if endereco == "" || strings.HasPrefix(endereco, "javascript:") || strings.HasPrefix(endereco, "#") {
		return
	}

	a.escolhaURL = endereco
	a.escolhaDestino = origem
	a.menuOrigem = ""

	var pt struct{ x, y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&pt)))
	ancora := rectT{pt.x, pt.y, pt.x + 1, pt.y + 1}

	itens := []menuItem{
		{kind: miHeader, label: txtAbrirLinkEm},
		{kind: miItem, label: txtAbaNovaEsquerda, id: "escolha:esq"},
	}
	site := a.siteAtualIA()

	rotulo := txtAquiNoChat
	if site.aceitaURL(endereco) {
		itens = append(itens, menuItem{kind: miItem, label: rotulo, id: "escolha:dir"})
	}

	itens = append(itens, menuItem{kind: miSep},
		menuItem{kind: miItem, label: txtCancelar, hint: txtHintEsc, id: "escolha:cancelar"})

	menu.items = itens

	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	a.abrirMenuAncorado(ancora)
}

func (a *app) aplicarEscolhaLink(id string) {
	endereco := a.escolhaURL
	a.escolhaURL = ""
	a.closeMenu()

	if endereco == "" {
		return
	}

	switch id {
	case "escolha:esq":
		a.novaAba(endereco, true)
		a.status = txtLinkAbaNova
	case "escolha:cancelar":
		a.status = txtLinkCancelado
	case "escolha:dir":
		a.abrirEscolhaNaDireita(endereco)
	default:
		a.dizerErrof(txtLinkNaoAberto)
	}

	a.invalidateBar()
}

// abrirEscolhaNaDireita abre o link no painel da direita.
func (a *app) abrirEscolhaNaDireita(endereco string) {
	p := a.escolhaDestino

	a.escolhaDestino = nil
	if p == nil {
		p = a.right
	}

	if p != nil && p.ready && p.wv != nil {
		if p == a.right {
			a.chatTravando = false
		}

		p.queriaURL = endereco
		p.wv.Navigate(endereco)

		a.status = txtLinkPainelIA
	}
}

func (a *app) doAction(id string) {
	defer a.limpezaAposAcao(id)

	a.acordarAbaPrincipal()

	if a.doActionExt(id) {
		return
	}

	if a.doActionNavegar(id) {
		return
	}

	a.doActionPainel(id)
}

// doActionUpdateExt trata verificacao e aplicacao de update; true = tratado.
func (a *app) doActionUpdateExt(id string) bool {
	if after, ok := strings.CutPrefix(id, "extatualizar:"); ok {
		a.verificarUpdateExtensao(after)

		return true
	}

	if after, ok := strings.CutPrefix(id, "extaplicar:"); ok {
		a.aplicarUpdateExtensao(after)

		return true
	}

	return false
}

// doActionExtLigaDesliga trata ligar e desligar extensao; true = tratado.
func (a *app) doActionExtLigaDesliga(id string) bool {
	if after, ok := strings.CutPrefix(id, "extligar:"); ok {
		pasta := after
		a.alternarExtensao(pasta, true)
		a.dizerProgressoIndetf(txtExtLigando)

		return true
	}

	if after, ok := strings.CutPrefix(id, "extdesligar:"); ok {
		pasta := after
		a.alternarExtensao(pasta, false)

		return true
	}

	return false
}

// doActionExt trata os ids com prefixo de extensão ou paywall; true = tratado.
func (a *app) doActionExt(id string) bool {
	if a.doActionUpdateExt(id) {
		return true
	}

	if after, ok := strings.CutPrefix(id, "paywall:"); ok {
		a.abrirServicoPaywall(after)

		return true
	}

	if after, ok := strings.CutPrefix(id, "extpainel:"); ok {
		a.abrirPainelExtensao(after)

		return true
	}

	if after, ok := strings.CutPrefix(id, "extfixar:"); ok {
		pasta := after
		a.alternarFixada(pasta, !a.extFixada(pasta))

		return true
	}

	if after, ok := strings.CutPrefix(id, "extremover:"); ok {
		a.removerExtensao(after)

		return true
	}

	if after, ok := strings.CutPrefix(id, "extopcoes:"); ok {
		a.abrirOpcoesExtensao(after)

		return true
	}

	if a.doActionExtLigaDesliga(id) {
		return true
	}

	if strings.HasPrefix(id, "extdom:") {
		a.acaoDominioExt(id)

		return true
	}

	if after, ok := strings.CutPrefix(id, "ext:"); ok {
		a.abrirPainelExtensao(after)

		return true
	}

	return false
}

// saiDoLeitorSeAtivo sai do modo leitor do painel e diz se estava nele.
func (a *app) saiDoLeitorSeAtivo(p *pane) bool {
	if p != nil && p.readerAtivo {
		a.sairDoLeitor(p)

		return true
	}

	return false
}

// doActionNavegar trata voltar..datadir da barra e menus; true = tratado.
func (a *app) doActionNavegar(id string) bool {
	if a.doActionNavegarBase(id) {
		return true
	}

	return a.doActionNavegarFerramentas(id)
}

// navegarVoltarAvancar trata voltar/avancar com saida do leitor; true = tratado.
func (a *app) navegarVoltarAvancar(id string) bool {
	var voltar bool

	switch id {
	case idVoltar:
		voltar = true
	case idAvancar:
		voltar = false
	default:
		return false
	}

	if a.saiDoLeitorSeAtivo(a.left) {
		return true
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtSemPagPronta)

		return true
	}

	if voltar {
		a.voltarPagina()
	} else {
		a.avancarPagina()
	}

	return true
}

// voltarPagina volta uma pagina se houver anterior.
func (a *app) voltarPagina() {
	if !a.left.wv.CanGoBack() {
		a.dizerf(txtSemHistorico)

		return
	}

	a.left.wv.GoBack()
}

// avancarPagina avanca uma pagina se houver proxima.
func (a *app) avancarPagina() {
	if !a.left.wv.CanGoForward() {
		a.dizerf(txtSemHistorico)

		return
	}

	a.left.wv.GoForward()
}

func (a *app) navegarParaHome() bool {
	if a.left != nil {
		limparLeitor(a.left)
	}

	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.NavigateToString(startPageHTML)
		a.loading = false
		a.invalidateBar()
		a.layout()
	} else {
		a.dizerf(txtPagAbrindoAcao)
	}

	return true
}

func (a *app) navegarRecarregar() bool {
	if a.saiDoLeitorSeAtivo(a.left) {
		return true
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtPagAbrindoAcao)

		return true
	}

	if time.Since(a.ultimoReload) < 300*time.Millisecond {
		return true
	}

	a.ultimoReload = time.Now()
	if a.loading {
		a.left.wv.Stop()
		a.loading = false
		a.invalidateBar()
	} else {
		a.loading = true
		a.invalidateBar()
		a.left.wv.Reload()
	}

	return true
}

func (a *app) navegarParar() bool {
	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtPagAbrindoAcao)

		return true
	}

	a.left.wv.Stop()

	return true
}

// doActionNavegarBase trata voltar..go; true = tratado.
func (a *app) doActionNavegarBase(id string) bool {
	if a.navegarVoltarAvancar(id) {
		return true
	}

	switch id {
	case idHome:
		return a.navegarParaHome()
	case idRecarregar:
		return a.navegarRecarregar()
	case idParar:
		return a.navegarParar()
	case "go":
		a.navigateLeft(a.editText())

		return true
	default:
		return false
	}
}

// doActionNavegarFerramentas trata dev..zreset; true = tratado.
func (a *app) doActionNavegarFerramentas(id string) bool {
	if a.doActionNavegarDocs(id) {
		return true
	}

	return a.doActionNavegarZoom(id)
}

// navegarDocsFerramentas trata dev..salvarpdf; true = tratado.
func (a *app) navegarDocsFerramentas(id string) (bool, bool) {
	switch id {
	case idDev:
		if p := a.focused(); p != nil && p.ready && p.wv != nil {
			p.wv.OpenDevTools()
		}

		return true, true
	case idDownloads:
		a.abrirPagina(idDownloads)

		return true, true
	case idImprimirDlg:
		a.imprimirPDF()

		return true, true
	case "salvarpdf":
		a.imprimirPDF()

		return true, true
	default:
		return false, false
	}
}

// navegarDocsSalvar trata salvarpagina..copiarendereco; true = tratado.
func (a *app) navegarDocsSalvar(id string) (bool, bool) {
	switch id {
	case idSalvarPagina:
		a.salvarPagina()

		return true, true
	case idSalvarCompleto:
		a.salvarCompleto()

		return true, true
	case idSalvarUnico:
		a.salvarArquivoUnico()

		return true, true
	case idCopiarEndereco:
		a.copiarEndereco()

		return true, true
	default:
		return false, false
	}
}

// navegarDocsImpressao trata dev..copiarendereco; true = tratado.
func (a *app) navegarDocsImpressao(id string) (bool, bool) {
	if tratado, _ := a.navegarDocsFerramentas(id); tratado {
		return true, true
	}

	return a.navegarDocsSalvar(id)
}

// navegarDocsRecargaSaida trata selecionartudo..sair; true = tratado.
func (a *app) navegarDocsRecargaSaida(id string) bool {
	switch id {
	case idSelecionarTudo:
		a.selecionarTudo()

		return true
	case idRecarregarFrio:
		a.recarregarFrio()

		return true
	case idRecarregarSemLite:
		a.recarregarSemOtimizacoes()

		return true
	case "recarregarsemjs":
		a.recarregarSemJS()

		return true
	case idSair:
		a.saveAndQuit()

		return true
	default:
		return false
	}
}

// doActionNavegarDocs trata dev..sair; true = tratado.
func (a *app) doActionNavegarDocs(id string) bool {
	if tratado, _ := a.navegarDocsImpressao(id); tratado {
		return true
	}

	return a.navegarDocsRecargaSaida(id)
}

// doActionNavegarZoom trata limparDados..zreset; true = tratado.
func (a *app) doActionNavegarZoom(id string) bool {
	const passoZoomAcao = 0.05

	switch id {
	case idLimparDados:
		a.abrirPagina("dados")

		return true
	case "zin":
		a.setZoom(a.st.Zoom - passoZoomAcao)

		return true
	case "zout":
		a.setZoom(a.st.Zoom + passoZoomAcao)

		return true
	case "zreset":
		a.setZoom(1.0)

		return true
	default:
		return false
	}
}

// doActionPainel trata chat..datadir da barra e menus; true = tratado.
func (a *app) doActionPainel(id string) bool {
	if a.doActionPainelMenus(id) {
		return true
	}

	if a.doActionPainelLeitura(id) {
		return true
	}

	return a.doActionPainelConta(id)
}

// doActionPainelMenus trata chat..print da barra e menus; true = tratado.
func (a *app) doActionPainelMenus(id string) bool {
	if a.doActionPainelIA(id) {
		return true
	}

	return a.doActionPainelPrint(id)
}

// doActionPainelIA trata chat..seguranca; true = tratado.
func (a *app) doActionPainelIA(id string) bool {
	switch id {
	case idChat:
		a.toggleChat()

		return true
	case idChatHome:
		a.voltarPainelIAInicial()

		return true
	case idUbo:
		a.abrirPopupUbo()

		return true
	case idSenhas:
		a.abrirMenuSenhas()

		return true
	case idExtensoes:
		a.abrirMenuExtensoes()

		return true
	case idEngine:
		a.abrirMenuBuscador()

		return true
	case idSeguranca:
		a.abrirMenuSeguranca()

		return true
	default:
		return false
	}
}

// doActionPainelPrint trata ferramentas..print; true = tratado.
func (a *app) doActionPainelPrint(id string) bool {
	switch id {
	case idFerramentas:
		a.abrirMenuFerramentas()

		return true
	case idImprimir:
		a.abrirMenuPrint()

		return true
	case "printagora":
		a.iniciarRecorte()

		return true
	case "print1", "print2", "print3", "print4", "print5":
		if segundos := segundosTimerPrint(id); segundos > 0 {
			a.iniciarRecorteComTimer(segundos)
		}

		return true
	default:
		return false
	}
}

// doActionPainelLeitura trata clipboard..tr da barra e menus; true = tratado.
func (a *app) doActionPainelLeitura(id string) bool {
	if a.doActionPainelLeituraA(id) {
		return true
	}

	if a.doActionSenhas(id) {
		return true
	}

	return a.doActionPainelLeituraB(id)
}

// doActionPainelLeituraA trata clipboard..vpn; true = tratado.
func (a *app) doActionPainelLeituraA(id string) bool {
	switch id {
	case idClipboard:
		a.alternarClipboard()

		return true
	case idPaywall:
		a.abrirMenuPaywall()

		return true
	case idReader:
		if a.left != nil && a.left.readerAtivo {
			a.sairDoLeitor(a.left)
		} else {
			a.abrirModoLeitura()
		}

		return true
	case idCopiarLeitura:
		a.copiarArtigoDaBarra()

		return true
	case idCookies:
		a.limparCookiesDoDominio()

		return true
	case idVPN:
		a.abrirPagina(idVPN)

		return true
	default:
		return false
	}
}

// doActionPainelLeituraB trata sleep..tr; true = tratado.
func (a *app) doActionPainelLeituraB(id string) bool {
	switch id {
	case idSleep:
		a.toggleSonoLite()

		return true
	case idWebviewUnica:
		a.toggleWebviewUnica()

		return true
	case idLivreClique:
		a.toggleLivreClique()

		return true
	case idNojs:
		a.recarregarSemJS()

		return true
	case idYoutube:
		a.alternarYouTube()

		return true
	case "tr":
		a.abrirMenuIdiomas()

		return true
	default:
		return false
	}
}

// doActionPainelConta trata conta..datadir da barra e menus; true = tratado.
func (a *app) doActionPainelConta(id string) bool {
	if a.doActionPainelTroca(id) {
		return true
	}

	return a.doActionPainelJanela(id)
}

// doActionPainelTroca trata conta..traduzirnochat; true = tratado.
func (a *app) doActionPainelTroca(id string) bool {
	switch id {
	case "conta0":
		a.trocarConta(0)

		return true
	case "conta1":
		a.trocarConta(1)

		return true
	case "conta2":
		const indiceContaTerceira = 2
		a.trocarConta(indiceContaTerceira)

		return true
	case "conta3":
		const indiceContaQuarta = 3
		a.trocarConta(indiceContaQuarta)

		return true
	case "trocaconta":
		a.trocarConta(proximaConta())

		return true
	case idContaDeepseek:
		if _, ok := contaAtiva(); ok {
			a.trocarConta(proximaConta())
		} else {
			a.dizerf(txtSemContaLogin)
		}

		return true
	case "traduzirnochat":
		a.traduzirNoChat()

		return true
	default:
		return false
	}
}

// doActionPainelJanela trata novaJanela..datadir; true = tratado.
func (a *app) doActionPainelJanela(id string) bool {
	switch id {
	case idNovaJanela:
		a.novaJanela("")

		return true
	case idNovaPrivada:
		a.novaJanela(flagPrivado)

		return true
	case "lerpaginaTudo":
		a.lerPaginaParaChat(modoTudo)

		return true
	case idLiberarMemoria:
		a.liberarMemoriaAgora()

		return true
	case "zoomfit":
		a.toggleMaximizar()

		return true
	case idMenu:
		a.toggleMenu()

		return true
	case "datadir":
		pShellExecuteW.Call(a.hwnd,
			uintptr(unsafe.Pointer(utf16("open"))),
			uintptr(unsafe.Pointer(utf16(dataDir))),
			0, 0, swShow)

		return true
	default:
		return false
	}
}

func (a *app) irPara(u string) {
	a.navigateLeft(u)
}

func (a *app) tratarMsgApp(p *pane, msg string) bool {
	if !strings.Contains(msg, `"t":"app"`) {
		return false
	}

	endereco := ""
	if p != nil && p.ready && p.wv != nil {
		endereco = p.wv.Source()
	}

	if !ehPaginaInterna(endereco) {
		return true
	}

	var mensagem msgApp
	if err := json.Unmarshal([]byte(msg), &mensagem); err != nil {
		return true
	}

	if mensagem.Acao != "" {
		a.executarAcaoPagina(p, mensagem)

		return true
	}

	return true
}

func (a *app) executarAcaoPagina(_ *pane, mensagem msgApp) {
	var args map[string]any
	if len(mensagem.Args) > 0 {
		_ = json.Unmarshal(mensagem.Args, &args)
	}

	str := func(k string) string {
		if v, ok := args[k].(string); ok {
			return v
		}

		return ""
	}

	switch mensagem.Acao {
	case "navegar":
		if u := str("url"); u != "" {
			a.irPara(u)
		}
	case "pesquisar":
		if q := strings.TrimSpace(str("q")); q != "" {
			a.irPara(a.resolverInput(q))
		}
	}
}
