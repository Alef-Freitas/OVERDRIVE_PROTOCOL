//go:build windows

package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// IDs de acao de aba: criados nos menus e despachados no executarItemMenu.
const (
	idAbaReabrir       = "aba:reabrir"
	idAbaDuplicar      = "aba:duplicar"
	idAbaFechar        = "aba:fechar"
	idAbaFecharOutras  = "aba:fecharoutras"
	idAbaFecharDireita = "aba:fechardireita"
	idAbaNova          = "aba:nova"
	idAbaAtivar        = "aba:ativar"
	idAbaFixar         = "aba:fixar"
	idPainelMenu       = "panelMenu"
	idPainelTranslate  = "panelTranslate"
	idPainelSend       = "panelSend"
	idPainelFull       = "panelFull"
	idPainelReload     = "panelReload"
	idChatHome         = "chatHome"
	idLeftAux          = "leftAux"
	idLeftAuxFull      = "leftAuxFull"
	idImgToggle        = "imgToggle"
)

// IDs de acao: criados nos menus/botoes e despachados no doAction/executarItemMenu.
const (
	idVoltar            = "back"
	idAvancar           = "fwd"
	idRecarregar        = "reload"
	idParar             = "stop"
	idSair              = "sair"
	idUbo               = "ubo"
	idExtensoes         = "extensoes"
	idSeguranca         = "security"
	idFerramentas       = "ferramentas"
	idImprimir          = "print"
	idClipboard         = "clipboard"
	idSenhas            = "senhas"
	idDownloads         = "downloads"
	idSalvarPagina      = "salvarpagina"
	idSalvarCompleto    = "salvarcompleto"
	idSalvarUnico       = "salvarunico"
	idCopiarEndereco    = "copiarendereco"
	idSelecionarTudo    = "selecionartudo"
	idRecarregarFrio    = "recarregarfrio"
	idRecarregarSemLite = "recarregarsemlite"
	idLimparDados       = "limpardados"
	idImprimirDlg       = "imprimir"
	idContaDeepseek     = "contadeepseek"
	idCopiarLeitura     = "copiarleitura"
	idLiberarMemoria    = "liberarmemoria"
	idBuscador          = "buscador"
	idNovaJanela        = "novajanela"
	idNovaPrivada       = "novaprivada"
	idChatgptLimpar     = "chatgpt:limpar"
	idTranslate         = "translate"
	idTrOriginal        = "troriginal"
	idTrash             = "trash"
	idShield            = "shield"
)

// IDs de provedor/busca/rede/idioma: chaves e valores compartilhados por UI e dispatch.
const (
	idEngine            = "engine"
	idSleep             = "sleep"
	idLivreClique       = "livreclique"
	idReader            = "reader"
	idCookies           = "cookies"
	idVPN               = "vpn"
	idYoutube           = "youtube"
	chaveGoogle         = "google"
	chaveDeepseek       = "deepseek"
	chaveChatgpt        = "chatgpt"
	esquemaHTTPS        = "https"
	esquemaHTTP         = "http"
	hostLocal           = "localhost"
	ytParamsPadrao      = "EgIQAQ%3D%3D"
	idiomaPtBR          = "pt-BR"
	paginaPopup         = "popup.html"
	flagPrivado         = "--privado"
	modoTudo            = "tudo"
	sufAbaNova          = "nova"
	sufAbaReabrir       = "reabrir"
	sufAbaAtivar        = "ativar"
	sufAbaFixar         = "fixar"
	sufAbaDuplicar      = "duplicar"
	sufAbaRecarregar    = "recarregar"
	sufAbaCopiar        = "copiar"
	sufAbaFechar        = "fechar"
	sufAbaFecharOutras  = "fecharoutras"
	sufAbaFecharDireita = "fechardireita"
	idChat              = "chat"
	idHome              = "home"
	idMenu              = "menu"
	idSend              = "send"
	idSplit             = "split"
)

// IDs de acao/botao/icone/provedor: mesma palavra, mesmo significado em dispatch, mapa e lookup.
const (
	idCopy         = "copy"
	idSave         = "save"
	idDelete       = "delete"
	idLink         = "link"
	idCode         = "code"
	idVideo        = "video"
	idWatch        = "watch"
	idAudio        = "audio"
	idZoom         = "zoom"
	idPin          = "pin"
	idPlus         = "plus"
	idPower        = "power"
	idRobot        = "robot"
	idSearch       = "search"
	idSettings     = "settings"
	idUnlock       = "unlock"
	idDownload     = "download"
	idEdge         = "edge"
	idExpand       = "expand"
	idGlobe        = "globe"
	idLock         = "lock"
	idMute         = "mute"
	modoNao        = "nao"
	modoNaow       = "naow"
	idNativo       = "nativo"
	idNojs         = "nojs"
	idNoPreference = "no-preference"
	idOpenpane     = "openpane"
	fonteOriginal  = "original"
	idPaywall      = "paywall"
	idPanels       = "panels"
	idRadioOff     = "radiooff"
	idRadioOn      = "radioon"
	idRight        = "right"
	idImg          = "img"
	idLeft         = "left"
	idLibrary      = "library"
	idImagens      = "imagens"
	iconNovaJanela = "newwindow"
)

// IDs de dispatch/icone/provedor restantes + sufíxos e chaves de API.
const (
	idDev          = "dev"
	idWebviewUnica = "webviewunica"
	idCamera       = "camera"
	idChevronLeft  = "chevronleft"
	idChevronRight = "chevronright"
	idDictionary   = "dictionary"
	idPage         = "page"
	opDial         = "dial"
	redePing       = "ping"
	chaveContexto  = "context"
	sufHome        = ".home"
	sufLocal       = ".local"
	sufInternal    = ".internal"
	sufLan         = ".lan"
	idSeg          = "seg"
	diaSeg         = "seg"
)

// Chave "client" das APIs do Google (Translate e InnerTube do YouTube).
const chaveClient = "client"

const (
	itensFixosMenuBarra  = 4
	meioMenu             = 2
	atrasoPrint2s        = 2
	atrasoPrint3s        = 3
	atrasoPrint4s        = 4
	atrasoPrint5s        = 5
	zonaZoomPctMenu      = 2
	zonaZoomMaisMenu     = 3
	zonaZoomAjusteMenu   = 4
	tamanhoInicialMenu   = 10
	deltaRodaMenu        = 120
	linhasRolagemMenu    = 3
	deslocWordAltoMenu   = 16
	atrasoSubMenuMs      = 200
	tamanhoInicialJanela = 10
)

func (a *app) buildMenuItems() []menuItem {
	return a.buildMenuItemsWithIA(false)
}

func (a *app) buildMenuIAItems() []menuItem {
	return a.buildMenuItemsWithIA(true)
}

func (a *app) buildMenuItemsWithIA(soIA bool) []menuItem {
	subIA := []menuItem{{kind: miHeader, label: txtHdrAssistente}}

	atual := a.siteAtualIA().chave
	for _, s := range sitesIA {
		subIA = append(subIA, menuItem{kind: miIA, label: s.nome, id: s.chave})
	}

	if atual == chaveDeepseek && len(loginAtual.Contas) > 0 {
		subIA = append(subIA, menuItem{kind: miSep}, menuItem{kind: miHeader, label: txtHdrContaDS + rotuloContaAtiva()})
		subIA = append(subIA, subContas()...)
	} else if atual == chaveDeepseek {
		subIA = append(subIA, menuItem{kind: miSep}, menuItem{kind: miHeader, label: txtHdrContaVazia})
	}

	if atual == chaveChatgpt {
		subIA = append(subIA,
			menuItem{kind: miSep},
			menuItem{kind: miItem, label: txtApagarChatsGPT, hint: txtHintAbreDados, id: idChatgptLimpar},
		)
	}

	if soIA {
		return subIA
	}

	subAbas := []menuItem{
		{kind: miItem, label: txtNovaGuia, hint: txtHintCtrlT, id: idAbaNova},
		{kind: miItem, label: txtReabrirGuia, hint: txtHintCtrlShiftT, id: idAbaReabrir},
		{kind: miSep},
		{kind: miItem, label: txtDuplicarAtual, id: idAbaDuplicar},
		{kind: miItem, label: txtFecharAtual, hint: txtHintCtrlW, id: idAbaFechar},
		{kind: miItem, label: txtFecharOutras, id: idAbaFecharOutras},
		{kind: miItem, label: txtFecharDireita, id: idAbaFecharDireita},
	}
	subJanelas := []menuItem{
		{kind: miItem, label: txtNovaJanela, hint: txtHintCtrlN, id: idNovaJanela},
		{kind: miItem, label: txtNovaPrivada, hint: txtHintCtrlShiftN, id: idNovaPrivada},
	}

	principal := subMenuPrincipal()
	itens := make([]menuItem, 0, itensFixosMenuBarra+len(principal))
	itens = append(itens,
		menuItem{kind: miSub, label: txtSubGuias, sub: subAbas},
		menuItem{kind: miSub, label: txtSubJanelas, sub: subJanelas},
		menuItem{kind: miZoom},
	)

	return append(itens, principal...)
}

func (a *app) abrirMenuPainel() {
	visible := false

	if panelMenu.hwnd != 0 {
		v, _, _ := pIsWindowVisible.Call(panelMenu.hwnd)
		visible = v != 0
	}

	if panelMenu.open || visible {
		a.fecharMenuPainel()

		return
	}

	panelMenu.items = a.buildMenuIAItems()
	a.menuMetricsIn(&panelMenu, false)

	if panelMenu.hwnd == 0 && !a.createMenuWindowFor(&panelMenu, panelMenuClassName, panelMenuProcPtr) {
		return
	}

	panelMenu.open = true

	a.posicionarMenuPainel(true)
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) posicionarMenuPainel(mostrar bool) {
	if panelMenu.hwnd == 0 {
		return
	}

	ponto := struct{ x, y int32 }{a.panelMenuBtn.Right + a.sc(4), (a.panelMenuBtn.Top + a.panelMenuBtn.Bottom) / meioMenu}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	x := a.ajustarMenuPainelX(ponto.x, area)
	y := a.ajustarMenuPainelY(ponto.y, area)

	flags := uintptr(swpNoActivate)
	if mostrar {
		flags |= swpShowWindow
	}

	pSetWindowPos.Call(panelMenu.hwnd, 0, uintptr(x), uintptr(y), uintptr(panelMenu.w), uintptr(panelMenu.h), flags)

	if mostrar {
		pShowWindow.Call(panelMenu.hwnd, swShowNoActivate)
	}

	panelMenu.dx, panelMenu.dy = x, y
	if mostrar {
		pUpdateWindow.Call(panelMenu.hwnd)
	}
}

// ajustarMenuPainelX mantem o menu dentro da area de trabalho.
func (a *app) ajustarMenuPainelX(x int32, area rectT) int32 {
	if area.Right > area.Left && x+panelMenu.w > area.Right-a.sc(4) {
		x = x - a.sc(8) - (a.panelMenuBtn.Right - a.panelMenuBtn.Left) - panelMenu.w
	}

	return x
}

// ajustarMenuPainelY mantem o menu dentro da area vertical.
func (a *app) ajustarMenuPainelY(y int32, area rectT) int32 {
	y -= panelMenu.h / meioMenu
	if area.Top > 0 && y < area.Top+a.sc(4) {
		y = area.Top + a.sc(4)
	}

	if area.Bottom > 0 && y+panelMenu.h > area.Bottom-a.sc(4) {
		y = area.Bottom - a.sc(4) - panelMenu.h
	}

	return y
}

func (a *app) fecharMenuPainel() {
	if !panelMenu.open {
		return
	}

	panelMenu.open = false

	panelMenu.hover = -1
	if panelMenu.hwnd != 0 {
		pShowWindow.Call(panelMenu.hwnd, swHide)
	}

	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) atualizarMenuPainel() {
	if !panelMenu.open {
		return
	}

	panelMenu.items = a.buildMenuIAItems()
	a.menuMetricsIn(&panelMenu, false)
	panelMenu.hover = -1

	a.posicionarMenuPainel(true)
	pInvalidateRect.Call(panelMenu.hwnd, 0, 0)
}

func (a *app) executarItemMenuPainel(id string) {
	manterAberto := strings.HasPrefix(id, "ia:") || strings.HasPrefix(id, "conta") || id == idContaDeepseek
	a.executarItemMenu(id)

	if manterAberto && panelMenu.open {
		a.atualizarMenuPainel()

		return
	}

	a.fecharMenuPainel()
}

func subMenuPrincipal() []menuItem {
	return []menuItem{
		{kind: miSep},
		{kind: miSub, label: txtSubPagAtual, sub: []menuItem{
			{kind: miItem, label: txtSelecionarTudo, id: idSelecionarTudo},
			{kind: miItem, label: txtImprimir, hint: txtHintCtrlP, id: idImprimirDlg},
			{kind: miItem, label: txtCopiarEndereco, id: idCopiarEndereco},
			{kind: miItem, label: txtRecSemCacheM, hint: txtHintCtrlShiftR, id: idRecarregarFrio},
			{kind: miItem, label: txtRecSemOtimM, id: idRecarregarSemLite},
		}},
		{kind: miSub, label: txtSubSalvarPagina, sub: []menuItem{
			{kind: miSub, label: txtSubSalvarPagina2, sub: []menuItem{
				{kind: miItem, label: txtSalvarPagHTML, hint: txtHintCtrlS, id: idSalvarPagina},
				{kind: miItem, label: txtSalvarPagFull, id: idSalvarCompleto},
				{kind: miItem, label: txtSalvarPagUnica, id: idSalvarUnico},
			}},
		}},
		{kind: miSub, label: txtSubPrivacidade, sub: []menuItem{
			{kind: miItem, label: txtCockpitDados, hint: txtHintCtrlShiftDel, id: idLimparDados},
			{kind: miItem, label: txtAbrirPastaDados, id: "abrirpasta:dados"},
		}},
		{kind: miSep},
		{kind: miItem, label: txtConfiguracoes, id: "pagina:config"},
		{kind: miItem, label: txtSair, id: idSair},
	}
}

func (a *app) abrirMenuIdiomas() {
	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtSemPagTraduzir)

		return
	}

	itens := a.itensModoTraducao()
	itens = append(itens, menuItem{kind: miHeader, label: txtHdrTraduzirPara})

	itens = anexarIdiomasOferecidos(itens, idiomaAtualTraducao(a))

	a.menuOrigem = "tr"
	menu.items = itens
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		a.dizerf(txtMenuIdiomas)

		return
	}

	if menu.open {
		a.closeMenu()
	}

	a.abrirMenuAncorado(a.toolsBtn.rect)
	a.trBtn.on = true
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

// itensModoTraducao monta cabecalho e modos de traducao.
func (a *app) itensModoTraducao() []menuItem {
	itens := []menuItem{}
	if a.traduzido {
		itens = append(itens, menuItem{kind: miItem, label: txtVerOriginal, id: idTrOriginal})
		itens = append(itens, menuItem{kind: miSep})
	}

	itens = append(itens, menuItem{kind: miHeader, label: txtHdrModoTraducao})

	rotNativo, rotGoogle := a.rotulosModoTraducao()

	return append(itens,
		menuItem{kind: miItem, label: rotNativo, id: "trmodo:nativo"},
		menuItem{kind: miItem, label: rotGoogle, id: "trmodo:google"},
		menuItem{kind: miSep},
	)
}

// rotulosModoTraducao marca o modo de traducao atual.
func (a *app) rotulosModoTraducao() (string, string) {
	modo := a.modoTraducao()

	rotNativo := txtMotorOverdrive
	if modo == idNativo {
		rotNativo += txtSufixoAtual
	}

	rotGoogle := txtGoogleProxy
	if modo == chaveGoogle {
		rotGoogle += txtSufixoAtual
	}

	return rotNativo, rotGoogle
}

// idiomaAtualTraducao resolve o idioma atual com padrao pt-BR.
func idiomaAtualTraducao(a *app) string {
	if atual := a.st.Idioma; atual != "" {
		return atual
	}

	return idiomaPtBR
}

// anexarIdiomasOferecidos adiciona os idiomas com o atual marcado.
func anexarIdiomasOferecidos(itens []menuItem, atual string) []menuItem {
	for _, idioma := range idiomasOferecidos {
		rotulo := idioma.nome
		if idioma.codigo == atual {
			rotulo += txtSufixoAtual
		}

		itens = append(itens, menuItem{kind: miItem, label: rotulo, id: "idioma:" + idioma.codigo})
	}

	return itens
}

func (a *app) acaoDeIdioma(id string) bool {
	if after, ok := strings.CutPrefix(id, "trmodo:"); ok {
		a.aplicarModoTraducao(after)

		return true
	}

	if after, ok := strings.CutPrefix(id, "idioma:"); ok {
		a.aplicarIdiomaTraducao(after)

		return true
	}

	if id == idTrOriginal {
		if a.left != nil && a.left.ready {
			a.desfazerTraducao(a.left)
		}

		return true
	}

	return false
}

// aplicarModoTraducao troca o modo de traducao e avisa.
func (a *app) aplicarModoTraducao(modo string) {
	if modo != chaveGoogle {
		modo = idNativo
	}

	if a.modoTraducao() == modo {
		return
	}

	if a.traduzido && a.left != nil && a.left.ready {
		a.desfazerTraducao(a.left)
	}

	a.st.ModoTraducao = modo
	a.persist()
	a.anunciarModoTraducao(modo)

	a.invalidateBar()
}

// anunciarModoTraducao informa o modo de traducao ativo.
func (a *app) anunciarModoTraducao(modo string) {
	if modo == chaveGoogle {
		a.dizerf(txtModoTrGoogle)
	} else {
		a.dizerf(txtModoTrNativo)
	}
}

// aplicarIdiomaTraducao troca o idioma e retraduz a pagina.
func (a *app) aplicarIdiomaTraducao(cod string) {
	a.st.Idioma = cod
	a.persist()

	if a.left != nil && a.left.ready {
		a.depois(func() { a.traduzirPagina(a.left) })
	}
}

// Critério do menu ferramentas: ação direta mais usada, submenus por alvo
// (página, site/rede, diagnóstico) com as ações por frequência de uso,
// interruptores de comportamento por último. O submenu abre ao lado e
// continua aberto para usar várias ferramentas em sequência.
func (a *app) buildMenuFerramentas() []menuItem {
	return []menuItem{
		{kind: miItem, label: txtSenhasVer, id: idSenhas},
		{kind: miSub, label: txtSubPaginaWeb, sub: []menuItem{
			{kind: miItem, label: txtTraduzirPagina, id: "tr"},
			{kind: miItem, label: txtModoLeitura, id: idReader},
			{kind: miItem, label: txtFerramentasCopiar, id: idCopiarLeitura},
		}},
		{kind: miSub, label: txtSubSiteRede, sub: []menuItem{
			{kind: miItem, label: txtLimparCookies, id: idCookies},
			{kind: miItem, label: txtPaywall, id: idPaywall},
			{kind: miCheck, label: txtVPN, id: idVPN, check: a.vpnAtiva()},
		}},
		{kind: miItem, label: txtSemJS, id: idNojs},
		{kind: miCheck, label: txtSonoLeve, id: idSleep, check: a.st.SleepLite},
		{kind: miCheck, label: txtVirtAbas, id: idWebviewUnica, check: a.st.WebviewUnica},
		{kind: miCheck, label: txtLivreClique, id: idLivreClique, check: a.st.LivreClique},
		{kind: miSub, label: txtSubDiagnostico, sub: []menuItem{
			{kind: miItem, label: txtDevTools, hint: txtHintF12, id: idDev},
			{kind: miItem, label: txtLiberarMemoria, id: idLiberarMemoria},
		}},
	}
}

func (a *app) abrirMenuFerramentas() {
	if menu.open && a.menuOrigem == idFerramentas {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	a.contextAba = -1
	a.menuOrigem = idFerramentas
	menu.items = a.buildMenuFerramentas()
	menu.ignorarSoltura = true

	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		menu.ignorarSoltura = false

		return
	}

	if menuSub.hwnd == 0 && !a.createSubMenuWindow() {
		menu.ignorarSoltura = false

		return
	}

	a.abrirMenuAncorado(a.toolsBtn.rect)
}

func (a *app) abrirMenuPrint() {
	if menu.open && a.menuOrigem == idImprimir {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	a.contextAba = -1
	a.menuOrigem = idImprimir
	menu.items = []menuItem{
		{kind: miHeader, label: txtHdrCaptura},
		{kind: miItem, label: txtCapturarAgora, hint: txtHintPadrao, id: "printagora"},
		{kind: miSep},
		{kind: miItem, label: txtPrintEm1, id: "print1"},
		{kind: miItem, label: fmt.Sprintf(txtFmtPrintEmN, atrasoPrint2s), id: "print2"},
		{kind: miItem, label: fmt.Sprintf(txtFmtPrintEmN, atrasoPrint3s), id: "print3"},
		{kind: miItem, label: fmt.Sprintf(txtFmtPrintEmN, atrasoPrint4s), id: "print4"},
		{kind: miItem, label: fmt.Sprintf(txtFmtPrintEmN, atrasoPrint5s), id: "print5"},
	}

	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	a.abrirMenuAncorado(a.printBtn.rect)
}

func (a *app) abrirMenuPaywall() {
	alvo, ok := a.alvoPaywallAtual()
	if !ok {
		a.dizerf(txtPaywallHTTP)

		return
	}

	a.paywallAlvo = alvo
	a.contextAba = -1
	a.menuOrigem = idPaywall
	menu.items = itensMenuPaywall()
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		a.menuOrigem = ""
		a.dizerf(txtPaywallSemMenu)

		return
	}

	if menuSub.hwnd == 0 && !a.createSubMenuWindow() {
		a.menuOrigem = ""
		a.dizerf(txtPaywallSemMenu)

		return
	}

	a.abrirMenuAncorado(a.toolsBtn.rect)
}

// alvoPaywallAtual valida a URL da barra para o paywall.
func (a *app) alvoPaywallAtual() (string, bool) {
	alvo := strings.TrimSpace(a.urlDaBarraAtual())
	enderecoParse, err := url.Parse(alvo)

	host := ""
	if err == nil {
		host = strings.ToLower(enderecoParse.Hostname())
	}

	if err != nil || enderecoParse.Host == "" || !esquemaPaywallValido(enderecoParse.Scheme) || hostProibidoPaywall(host) {
		return "", false
	}

	enderecoParse.Fragment = ""

	return enderecoParse.String(), true
}

// esquemaPaywallValido aceita apenas http e https.
func esquemaPaywallValido(esquema string) bool {
	return esquema == esquemaHTTP || esquema == esquemaHTTPS
}

// hostProibidoPaywall bloqueia enderecos locais no paywall.
func hostProibidoPaywall(host string) bool {
	return host == hostLocal || host == "::1" || strings.HasPrefix(host, "127.")
}

// itensMenuPaywall monta os servicos de paywall por grupo.
func itensMenuPaywall() []menuItem {
	return []menuItem{
		{kind: miHeader, label: txtHdrPaywallAbrir},
		{kind: miSub, label: txtSubArquivos, sub: []menuItem{
			{kind: miItem, label: "archive.ph", hint: txtHintArquivo, id: "paywall:archiveph"},
			{kind: miItem, label: "archive.is", hint: txtHintArquivo, id: "paywall:archiveis"},
			{kind: miItem, label: "Wayback Machine", hint: "web.archive.org", id: "paywall:wayback"},
		}},
		{kind: miSub, label: txtSubLeitores, sub: []menuItem{
			{kind: miItem, label: "smry.ai", hint: txtHintLeitor, id: "paywall:smry"},
			{kind: miItem, label: "Bypass Paywall Reader", hint: txtHintLeitor, id: "paywall:bypass"},
			{kind: miItem, label: "The Radical Party", hint: txtHintLeitor, id: "paywall:radical"},
			{kind: miItem, label: "RemovePaywall", hint: txtHintArquivos, id: "paywall:removepaywall"},
			{kind: miItem, label: "Marreta", hint: "marreta.link", id: "paywall:marreta"},
		}},
		{kind: miSub, label: txtSubTextoLimpo, sub: []menuItem{
			{kind: miItem, label: "Jina Reader", hint: txtHintTexto, id: "paywall:jina"},
			{kind: miItem, label: "Textise", hint: txtHintTexto, id: "paywall:textise"},
			{kind: miItem, label: "PrintFriendly", hint: txtHintImpressao, id: "paywall:printfriendly"},
			{kind: miItem, label: "Outline", hint: txtHintTexto, id: "paywall:outline"},
			{kind: miItem, label: "Freedium", hint: "Medium", id: "paywall:freedium"},
		}},
	}
}

func (a *app) abrirServicoPaywall(chave string) {
	link := linkPaywall(chave, a.paywallAlvo)
	if link == "" {
		a.dizerf(txtPaywallSemURL)

		return
	}

	a.novaAba(link, true)
}

func linkPaywall(chave, endereco string) string {
	enderecoParse, err := url.Parse(endereco)
	if err != nil || enderecoParse.Host == "" {
		return ""
	}

	if link, ok := linkPaywallDireto(chave, endereco); ok {
		return link
	}

	if link, ok := linkPaywallCodificado(chave, endereco); ok {
		return link
	}

	if chave == "marreta" {
		return "https://marreta.link/p/" + caminhoPaywallSemEsquema(enderecoParse)
	}

	return ""
}

// linkPaywallDireto cola a URL direta no servico de arquivo ou leitor.
func linkPaywallDireto(chave, endereco string) (string, bool) {
	switch chave {
	case "archiveph":
		return "https://archive.ph/" + endereco, true
	case "archiveis":
		return "https://archive.is/" + endereco, true
	case "wayback":
		return "https://web.archive.org/web/*/" + endereco, true
	case "smry":
		return "https://smry.ai/" + endereco, true
	case "jina":
		return "https://r.jina.ai/" + endereco, true
	case "outline":
		return "https://outline.com/" + endereco, true
	case "freedium":
		return "https://freedium.cfd/" + endereco, true
	}

	return "", false
}

// linkPaywallCodificado cola a URL escapada no servico de leitura.
func linkPaywallCodificado(chave, endereco string) (string, bool) {
	enc := url.QueryEscape(endereco)

	switch chave {
	case "bypass":
		return "https://www.bypasspaywallreader.com/search.html?url=" + enc, true
	case "radical":
		return "https://paywall.theradicalparty.com/read?url=" + enc, true
	case "removepaywall":
		return "https://www.removepaywall.com/search?url=" + enc, true
	case "textise":
		return "https://www.textise.net/showText.aspx?strURL=" + enc, true
	case "printfriendly":
		return "https://www.printfriendly.com/print?url=" + enc, true
	}

	return "", false
}

// caminhoPaywallSemEsquema monta host+caminho+query para o marreta.
func caminhoPaywallSemEsquema(pu *url.URL) string {
	semEsquema := pu.Host + pu.EscapedPath()
	if pu.RawQuery != "" {
		semEsquema += "?" + pu.RawQuery
	}

	return semEsquema
}

func (a *app) menuMetrics() { a.menuMetricsIn(&menu, true) }

func (a *app) menuItemH(janela *menuWin, item menuItem) int32 {
	switch item.kind {
	case miHeader:
		if item.desc != "" {
			return janela.headerH + a.sc(20) + a.sc(8)
		}

		return janela.headerH
	case miSep:
		return janela.sepY
	default:
		return janela.itemH
	}
}

func (a *app) menuRowTop(janela *menuWin, idx int) int32 {
	y := janela.padY
	for i := 0; i < idx && i < len(janela.items); i++ {
		y += a.menuItemH(janela, janela.items[i])
	}

	return y - janela.scrollY
}

func (a *app) extRowZona(janela *menuWin, yTop int32) rectT {
	padX := a.sc(16)

	return rectT{janela.w - padX - a.sc(30), yTop, janela.w - padX, yTop + janela.itemH}
}

func dentroRect(r rectT, x, y int32) bool {
	return r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

const csDropShadow = 0x00020000

func (a *app) centroZona(r rectT) rectT {
	lado := a.sc(16)
	posX := (r.Left + r.Right) / meioMenu
	posY := (r.Top + r.Bottom) / meioMenu

	return rectT{posX - lado/meioMenu, posY - lado/meioMenu, posX + lado/meioMenu, posY + lado/meioMenu}
}

func (a *app) menuMetricsIn(janela *menuWin, principal bool) {
	janela.zRow = rectT{}
	janela.itemH = a.sc(38)
	janela.headerH = a.sc(28)
	janela.sepY = a.sc(12)
	janela.padY = a.sc(8)

	janela.w = a.sc(390)
	if !principal {
		janela.w = a.sc(390)
	}

	total := janela.padY
	for _, item := range janela.items {
		switch item.kind {
		case miZoom:
			total += janela.itemH
			y := total - janela.itemH
			padX := a.sc(16)
			janela.zRow = rectT{0, y, janela.w, y + janela.itemH}
			janela.zIcon = rectT{padX, y + a.sc(1), padX + a.sc(20), y + janela.itemH - a.sc(1)}
			janela.zLabel = rectT{padX + a.sc(24), y, padX + a.sc(80), y + janela.itemH}
			right := janela.w - padX
			janela.zFit = rectT{right - a.sc(24), y + a.sc(2), right, y + janela.itemH - a.sc(2)}
			right -= a.sc(26)
			janela.zSep = rectT{right + a.sc(8), y + a.sc(5), right + a.sc(9), y + janela.itemH - a.sc(5)}
			right -= a.sc(6)
			janela.zPlus = rectT{right - a.sc(22), y + a.sc(2), right, y + janela.itemH - a.sc(2)}
			right -= a.sc(22)
			janela.zPct = rectT{right - a.sc(48), y + a.sc(2), right, y + janela.itemH - a.sc(2)}
			right -= a.sc(48)
			janela.zMinus = rectT{right - a.sc(22), y + a.sc(2), right, y + janela.itemH - a.sc(2)}
		default:
			total += a.menuItemH(janela, item)
		}
	}

	janela.contentH = total + janela.padY
	janela.h = janela.contentH
	janela.scrollY = 0

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	if maxH := area.Height() - a.sc(8); maxH > 0 && janela.h > maxH {
		janela.h = maxH
	}
}

func (a *app) menuZoomAt(x, y int32) int {
	if menu.zRow.Width() <= 0 || y < menu.zRow.Top || y >= menu.zRow.Bottom {
		return 0
	}

	dentro := func(r rectT) bool { return r.Width() > 0 && x >= r.Left && x < r.Right }
	switch {
	case dentro(menu.zMinus):
		return 1
	case dentro(menu.zPct):
		return zonaZoomPctMenu
	case dentro(menu.zPlus):
		return zonaZoomMaisMenu
	case dentro(menu.zFit):
		return zonaZoomAjusteMenu
	}

	return 0
}

func (a *app) createMenuWindowFor(janela *menuWin, classe string, proc uintptr) bool {
	if proc == 0 {
		return false
	}

	hInst := getModuleHandle()
	nomeClasse := utf16(classe)
	classeJanela := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csDropShadow,
		lpfnWndProc:   proc,
		hInstance:     hInst,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: nomeClasse,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classeJanela)))

	h, _, _ := pCreateWindowExW.Call(
		wsExToolWindow,
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(utf16(""))),
		wsPopup,
		0, 0, tamanhoInicialMenu, tamanhoInicialMenu,
		0, 0, hInst, 0,
	)
	if h == 0 {
		return false
	}

	aplicarCantosDWM(h)
	pSetWindowTheme.Call(h, uintptr(unsafe.Pointer(utf16("DarkMode_Explorer"))), 0)
	janela.hwnd = h

	return true
}

func (a *app) createMenuWindow() bool {
	return a.createMenuWindowFor(&menu, menuClassName, menuProcPtr)
}

func (a *app) createSubMenuWindow() bool {
	return a.createMenuWindowFor(&menuSub, menuSubClassName, menuSubProcPtr)
}

func (a *app) createSubMenu2Window() bool {
	return a.createMenuWindowFor(&menuSub2, menuSub2ClassName, menuSub2ProcPtr)
}

func (a *app) toggleMenu() {
	if menu.open {
		a.closeMenu()

		return
	}

	a.openMenu()
}

func (a *app) openMenu() {
	a.contextAba = -1
	a.menuOrigem = idMenu
	menu.items = a.buildMenuItems()
	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		a.dizerf(txtMenuNaoAbriu)

		return
	}

	if menuSub.hwnd == 0 && !a.createSubMenuWindow() {
		a.dizerf(txtMenuNaoAbriu)

		return
	}

	a.abrirMenuAncorado(a.menuBtn.rect)
}

func (a *app) abrirMenuAncorado(ancora rectT) {
	botao := ancora
	ponto := struct{ x, y int32 }{botao.Right, botao.Bottom + a.sc(3)}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	x := ponto.x - menu.w
	if x < a.sc(4) {
		x = a.sc(4)
	}

	y := ponto.y

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	if area.Bottom > 0 && y+menu.h > area.Bottom-a.sc(4) {
		topo := struct{ x, y int32 }{botao.Left, botao.Top}
		pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&topo)))

		y = topo.y - menu.h - a.sc(3)
		if y < area.Top {
			y = area.Top + a.sc(4)
		}
	}

	a.hideTip()
	a.closeSubMenu()

	menu.hover = -1
	menu.hoverZona = 0
	menu.hoverZoom = 0
	menu.subPai = -1
	menu.open = true
	menu.dx, menu.dy = x, y

	pSetWindowPos.Call(menu.hwnd, 0, uintptr(x), uintptr(y), uintptr(menu.w), uintptr(menu.h),
		swpNoZOrder|swpNoActivate)
	pShowWindow.Call(menu.hwnd, swShow)
	pUpdateWindow.Call(menu.hwnd)

	pSetCapture.Call(menu.hwnd)

	a.menuBtn.on = a.menuOrigem == idMenu
	a.paywallBtn.on = a.menuOrigem == idPaywall
	a.extBtn.on = a.menuOrigem == idExtensoes
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) abrirMenuNoPonto(x, y int32) {
	ponto := struct{ x, y int32 }{x, y}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))
	x, y = ponto.x, ponto.y

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	margem := a.sc(4)
	if area.Right > area.Left {
		if x+menu.w > area.Right-margem {
			x = area.Right - menu.w - margem
		}

		if x < area.Left+margem {
			x = area.Left + margem
		}
	}

	if area.Bottom > area.Top {
		if y+menu.h > area.Bottom-margem {
			y -= menu.h
		}

		if y < area.Top+margem {
			y = area.Top + margem
		}
	}

	a.hideTip()
	a.closeSubMenu()

	menu.hover = -1
	menu.hoverZona = 0
	menu.hoverZoom = 0
	menu.subPai = -1
	menu.open = true
	menu.dx, menu.dy = x, y
	pSetWindowPos.Call(menu.hwnd, 0, uintptr(x), uintptr(y), uintptr(menu.w), uintptr(menu.h), swpNoZOrder|swpNoActivate)
	pShowWindow.Call(menu.hwnd, swShow)
	pUpdateWindow.Call(menu.hwnd)
	pSetCapture.Call(menu.hwnd)

	a.menuBtn.on = a.menuOrigem == idMenu
	a.paywallBtn.on = a.menuOrigem == idPaywall
	a.toolsBtn.on = a.menuOrigem == idFerramentas
	a.printBtn.on = a.menuOrigem == idImprimir
	a.extBtn.on = a.menuOrigem == idExtensoes
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) closeMenu() {
	if !menu.open {
		return
	}

	a.closeSubMenu()

	menu.open = false
	menu.ignorarSoltura = false
	menu.hoverZona = 0
	menu.headerX = false
	a.subAtrasoIdx = -1
	a.subAtrasoQuando = time.Time{}
	pKillTimer.Call(a.hwnd, timerSubDelay)

	pReleaseCapture.Call()
	pShowWindow.Call(menu.hwnd, swHide)

	a.menuBtn.on = false
	a.paywallBtn.on = false
	a.toolsBtn.on = false
	a.printBtn.on = false
	a.extBtn.on = false
	a.menuOrigem = ""
	pInvalidateRect.Call(a.hwnd, 0, 0)

	if p := a.focused(); p != nil && p.ready {
		p.wv.Focus()
	}
}

// nivelMenu descreve um nivel de submenu para as funcoes genericas.
type nivelMenu struct {
	pai    *int
	janela *menuWin
	anca   *menuWin
}

func nivelSubMenu1() nivelMenu {
	return nivelMenu{pai: &menu.subPai, janela: &menuSub, anca: &menu}
}

func nivelSubMenu2() nivelMenu {
	return nivelMenu{pai: &menu.sub2Pai, janela: &menuSub2, anca: &menuSub}
}

// abrirNivelMenu abre um nivel de submenu a partir do item indicado.
func (a *app) abrirNivelMenu(nivel nivelMenu, idx int) {
	itens := nivel.anca.items
	if idx < 0 || idx >= len(itens) {
		return
	}

	item := itens[idx]
	if len(item.sub) == 0 {
		a.fecharNivelMenu(nivel)

		return
	}

	*nivel.pai = idx
	nivel.janela.items = item.sub
	a.menuMetricsIn(nivel.janela, false)

	if nivel.janela.hwnd == 0 && !a.criarJanelaNivel(nivel) {
		return
	}

	y := a.topoNivelMenu(nivel, idx)
	nivel.janela.hover = -1
	eraAberto := nivel.janela.open

	posX, posY := a.posicaoNivelMenu(nivel, y)
	nivel.janela.dx = posX - menu.dx
	nivel.janela.dy = posY - menu.dy

	pSetWindowPos.Call(nivel.janela.hwnd, 0, uintptr(posX), uintptr(posY),
		uintptr(nivel.janela.w), uintptr(nivel.janela.h), swpNoActivate|swpShowWindow)
	pInvalidateRect.Call(nivel.janela.hwnd, 0, 1)

	if !eraAberto {
		pUpdateWindow.Call(nivel.janela.hwnd)
	}

	nivel.janela.open = true
}

// criarJanelaNivel cria a janela do nivel conforme o no.
func (a *app) criarJanelaNivel(nivel nivelMenu) bool {
	if nivel.janela == &menuSub2 {
		return a.createSubMenu2Window()
	}

	return a.createSubMenuWindow()
}

// topoNivelMenu calcula o topo do nivel a partir do item pai.
func (a *app) topoNivelMenu(nivel nivelMenu, idx int) int32 {
	y := nivel.anca.padY
	for i := range idx {
		y += a.menuItemH(nivel.anca, nivel.anca.items[i])
	}

	return y - nivel.anca.scrollY
}

// posicaoNivelMenu ajusta o submenu para dentro da area de trabalho.
func (a *app) posicaoNivelMenu(nivel nivelMenu, y int32) (int32, int32) {
	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	ancaX, ancaY := origemTelaNivel(nivel)
	posX := posXNivelMenu(nivel, ancaX, a.sc(4), a.sc(6), area.Left, area.Right)

	posY := ancaY + y - a.sc(5)
	if area.Bottom > 0 && posY+nivel.janela.h > area.Bottom-a.sc(4) {
		posY = area.Bottom - a.sc(4) - nivel.janela.h
	}

	if area.Top > 0 && posY < area.Top+a.sc(4) {
		posY = area.Top + a.sc(4)
	}

	return posX, posY
}

// posXNivelMenu calcula o X de tela do submenu. O primeiro nivel abre a direita
// do menu raiz; o terceiro nivel abre a esquerda do item pai.
func posXNivelMenu(nivel nivelMenu, ancaX, margem, sobreposicao, areaLeft, areaRight int32) int32 {
	posX := ancaX + nivel.anca.w - sobreposicao
	if nivel.janela == &menuSub2 {
		posX = ancaX - nivel.janela.w + sobreposicao

		if areaLeft > 0 && posX < areaLeft+margem {
			posX = areaLeft + margem
		}

		return posX
	}

	if areaRight > 0 && posX+nivel.janela.w > areaRight-margem {
		posX = ancaX - nivel.janela.w + sobreposicao
	}

	return posX
}

// origemTelaNivel devolve a origem em coordenadas de tela da janela pai do nivel.
// O menu raiz ja guarda coordenadas de tela; os submenus guardam offsets relativos a ele.
func origemTelaNivel(nivel nivelMenu) (int32, int32) {
	if nivel.anca == &menu {
		return nivel.anca.dx, nivel.anca.dy
	}

	return menu.dx + nivel.anca.dx, menu.dy + nivel.anca.dy
}

func (a *app) fecharNivelMenu(nivel nivelMenu) {
	*nivel.pai = -1

	if nivel.janela.open || nivel.janela.hwnd != 0 {
		nivel.janela.open = false
		nivel.janela.hover = -1

		if nivel.janela.hwnd != 0 {
			pShowWindow.Call(nivel.janela.hwnd, swHide)
		}
	}
}

func (a *app) dentroNivelMenu(nivel nivelMenu, x, y int32) bool {
	if *nivel.pai < 0 || nivel.janela.hwnd == 0 {
		return false
	}

	posX, posY := x-nivel.janela.dx, y-nivel.janela.dy

	return posX >= 0 && posY >= 0 && posX < nivel.janela.w && posY < nivel.janela.h
}

func (a *app) abrirSubMenu(idx int) {
	a.abrirNivelMenu(nivelSubMenu1(), idx)
}

func (a *app) closeSubMenu() {
	a.fecharNivelMenu(nivelSubMenu1())
	a.fecharNivelMenu(nivelSubMenu2())
}

func (a *app) dentroSubMenu(x, y int32) bool {
	return a.dentroNivelMenu(nivelSubMenu1(), x, y)
}

func (a *app) abrirSubMenu2(idx int) {
	a.abrirNivelMenu(nivelSubMenu2(), idx)
}

func (a *app) closeSubMenu2() {
	a.fecharNivelMenu(nivelSubMenu2())
}

func (a *app) dentroSubMenu2(x, y int32) bool {
	return a.dentroNivelMenu(nivelSubMenu2(), x, y)
}

func (a *app) menuItemAtIn(janela *menuWin, y int32) int {
	y += janela.scrollY

	posY := janela.padY
	for indice, item := range janela.items {
		altura := a.menuItemH(janela, item)
		if item.kind == miHeader {
			if y >= posY && y < posY+altura {
				return -1
			}

			posY += altura

			continue
		}

		switch item.kind {
		case miSep:
			posY += altura
		case miZoom:
			posY += janela.itemH
		default:
			if y >= posY && y < posY+janela.itemH {
				return indice
			}

			posY += janela.itemH
		}
	}

	return -1
}

func (a *app) menuItemAt(y int32) int { return a.menuItemAtIn(&menu, y) }

func (a *app) menuItemAtXY(x, y int32) int {
	if x < 0 || y < 0 || x >= menu.w || y >= menu.h {
		return -1
	}

	return a.menuItemAt(y)
}

func iconeDoItem(item menuItem) string {
	if item.kind == miIA {
		return idRobot
	}

	if item.kind == miSub {
		return iconeDoSub(item.label)
	}

	if ic := iconeDoExt(item.id); ic != "" {
		return ic
	}

	if ic := iconeDaPagina(item.id); ic != "" {
		return ic
	}

	if ic := iconeDaAcao(item.id); ic != "" {
		return ic
	}

	return "document"
}

// iconeDoSub escolhe o icone dos submenus pelo rotulo.
func iconeDoSub(rotulo string) string {
	switch {
	case strings.Contains(rotulo, "Guias"):
		return "list"
	case strings.Contains(rotulo, "Janelas"):
		return iconNovaJanela
	case strings.Contains(rotulo, "Biblioteca"):
		return idLibrary
	case strings.Contains(rotulo, "Página"):
		return idPage
	case strings.Contains(rotulo, "Salvar"):
		return idSave
	case strings.Contains(rotulo, "Site e rede"):
		return idShield
	case strings.Contains(rotulo, "Diagnóstico"):
		return idSettings
	case strings.Contains(rotulo, "Privacidade"):
		return idShield
	case strings.Contains(rotulo, "Configura"):
		return idSettings
	}

	return idChevronRight
}

// iconeDoExt escolhe o icone das acoes de extensao; vazio = nao e extensao.
func iconeDoExt(id string) string {
	switch {
	case id == idExtensoes:
		return "puzzle"
	case strings.HasPrefix(id, "extpainel:") || strings.HasPrefix(id, "ext:"):
		return idOpenpane
	case strings.HasPrefix(id, "extfixar:"):
		return idPin
	case strings.HasPrefix(id, "extremover:"):
		return idDelete
	case strings.HasPrefix(id, "extligar:"):
		return idPlus
	case strings.HasPrefix(id, "extdesligar:"):
		return idPower
	}

	if ic := iconeDoExtAba(id); ic != "" {
		return ic
	}

	return iconeDoExtJanela(id)
}

// iconeDoExtAba escolhe o icone das acoes de guia; vazio = nao e guia.
func iconeDoExtAba(id string) string {
	switch id {
	case idAbaNova:
		return idPlus
	case idAbaReabrir:
		return "history"
	case idAbaDuplicar:
		return idCopy
	case idAbaFechar, idAbaFecharOutras, idAbaFecharDireita:
		return "close"
	}

	return ""
}

// iconeDoExtJanela escolhe o icone das acoes de janela; vazio = sem icone.
func iconeDoExtJanela(id string) string {
	switch id {
	case idClipboard:
		return "paste"
	case idNovaJanela:
		return iconNovaJanela
	case idNovaPrivada:
		return "private"
	}

	return ""
}

// iconeDaPagina escolhe o icone das paginas internas; vazio = nao e pagina.
func iconeDaPagina(id string) string {
	switch id {
	case "pagina:downloads":
		return idDownload
	case "pagina:config":
		return idSettings
	case "pagina:traducao":
		return idDictionary
	case "pagina:dados":
		return idDelete
	case "pagina:vpn":
		return idVPN
	}

	if ic := iconeDaPaginaDoc(id); ic != "" {
		return ic
	}

	return iconeDaPaginaFerramenta(id)
}

// iconeDaPaginaDoc escolhe o icone das acoes de documento; vazio = outro.
func iconeDaPaginaDoc(id string) string {
	switch id {
	case idSelecionarTudo:
		return "text"
	case idImprimirDlg:
		return idImprimir
	case idSalvarPagina:
		return idSave
	case idSalvarCompleto:
		return idSave
	case idSalvarUnico:
		return idSave
	case idCopiarEndereco:
		return idLink
	case idRecarregarFrio, idRecarregarSemLite:
		return "refresh"
	}

	return ""
}

// iconeDaPaginaFerramenta escolhe o icone das ferramentas; vazio = outro.
func iconeDaPaginaFerramenta(id string) string {
	switch id {
	case idSenhas:
		return idLock
	case idDev:
		return idCode
	case idLiberarMemoria:
		return "sync"
	case "abrirpasta:dados":
		return "folderopen"
	case idLimparDados:
		return idDelete
	case idSair:
		return idPower
	}

	return ""
}

// iconeDaAcao escolhe o icone das acoes de leitura/IA; vazio = sem icone proprio.
func iconeDaAcao(id string) string {
	if ic := iconeDaAcaoTraducao(id); ic != "" {
		return ic
	}

	if ic := iconeDaAcaoLeitura(id); ic != "" {
		return ic
	}

	return iconeDaAcaoIA(id)
}

// iconeDaAcaoTraducao escolhe o icone das acoes de traducao; vazio = outro.
func iconeDaAcaoTraducao(id string) string {
	switch {
	case id == idTrOriginal, strings.HasPrefix(id, "idioma:"), strings.HasPrefix(id, "trmodo:"):
		return idDictionary
	case id == idReader:
		return idReader
	case id == "tr":
		return idTranslate
	case id == idNojs:
		return "blocked"
	case id == idSleep:
		return idSleep
	case id == idLivreClique:
		return idLivreClique
	}

	return ""
}

// iconeDaAcaoLeitura escolhe o icone das acoes de pagina; vazio = outro.
func iconeDaAcaoLeitura(id string) string {
	switch id {
	case idWebviewUnica:
		return idOpenpane
	case idCookies:
		return idCookies
	case idCopiarLeitura:
		return idCopy
	case idPaywall:
		return idUnlock
	case idVPN:
		return idVPN
	}

	return ""
}

// iconeDaAcaoIA escolhe o icone das acoes de IA e links; vazio = sem icone.
func iconeDaAcaoIA(id string) string {
	switch {
	case strings.HasPrefix(id, idImprimir):
		return idCamera
	case strings.HasPrefix(id, "buscador:"):
		return idSearch
	case strings.HasPrefix(id, "seg:"), strings.HasPrefix(id, "perm:"):
		return idShield
	case strings.HasPrefix(id, "conta"), id == idContaDeepseek:
		return "people"
	case id == idChatgptLimpar:
		return idDelete
	case strings.HasPrefix(id, "ia:"):
		return idRobot
	case strings.HasPrefix(id, "ir:"):
		return idLink
	case strings.HasPrefix(id, "pagina:"):
		return idPage
	}

	return ""
}

func (a *app) menuRevelar(janela *menuWin, idx int) {
	if janela == nil || idx < 0 || idx >= len(janela.items) || janela.contentH <= janela.h {
		return
	}

	y := a.topoItemMenuRevelar(janela, idx)

	if y < janela.scrollY {
		janela.scrollY = y
	} else if y+janela.itemH > janela.scrollY+janela.h {
		janela.scrollY = y + janela.itemH - janela.h
	}

	a.limitarRolagemRevelar(janela)
}

// topoItemMenuRevelar soma a altura ate o item revelado.
func (a *app) topoItemMenuRevelar(janela *menuWin, idx int) int32 {
	y := janela.padY
	for i, item := range janela.items {
		if i == idx {
			break
		}

		y += a.menuItemH(janela, item)
	}

	return y
}

// limitarRolagemRevelar prende a rolagem nos limites do menu.
func (a *app) limitarRolagemRevelar(janela *menuWin) {
	if janela.scrollY < 0 {
		janela.scrollY = 0
	}

	if maxScroll := janela.contentH - janela.h; janela.scrollY > maxScroll {
		janela.scrollY = maxScroll
	}
}

func (a *app) scrollMenu(janela *menuWin, delta int32) {
	if janela == nil || janela.contentH <= janela.h || delta == 0 {
		return
	}

	passos := -delta / deltaRodaMenu
	if passos == 0 {
		passos = -1
		if delta > 0 {
			passos = 1
		}
	}

	janela.scrollY += passos * janela.itemH * linhasRolagemMenu
	if janela.scrollY < 0 {
		janela.scrollY = 0
	}

	if maxScroll := janela.contentH - janela.h; janela.scrollY > maxScroll {
		janela.scrollY = maxScroll
	}

	janela.hover = -1
	pInvalidateRect.Call(janela.hwnd, 0, 0)
}

func destinoMenu(id string) string {
	if destino, ok := destinoMenuRecurso(id); ok {
		return destino
	}

	return destinoMenuAcao(id)
}

// destinoMenuRecurso roteia recursos e guias; false = e acao.
func destinoMenuRecurso(id string) (string, bool) {
	switch {
	case strings.HasPrefix(id, "buscador:"):
		return idBuscador, true
	case strings.HasPrefix(id, "perm:"):
		return "perm", true
	case strings.HasPrefix(id, "seg:"):
		return idSeg, true
	case strings.HasPrefix(id, "aba:"):
		return destinoAbaMenu(id), true
	case strings.HasPrefix(id, "escolha:"):
		return "escolha", true
	case strings.HasPrefix(id, "irtab:"):
		return "irtab", true
	case id == "":
		return "", true
	}

	return "", false
}

// destinoAbaMenu roteia o sufixo de acao de guia.
func destinoAbaMenu(id string) string {
	switch strings.TrimPrefix(id, "aba:") {
	case sufAbaNova, sufAbaReabrir, sufAbaAtivar, sufAbaFixar, sufAbaDuplicar, sufAbaRecarregar,
		sufAbaCopiar, sufAbaFechar, sufAbaFecharOutras, sufAbaFecharDireita:
		return "aba"
	}

	return "aba:?"
}

// destinoMenuAcao roteia idiomas, IA, paginas e acoes.
func destinoMenuAcao(id string) string {
	switch {
	case strings.HasPrefix(id, "idioma:"), id == idTrOriginal, strings.HasPrefix(id, "trmodo:"):
		return "idioma"
	case id == idChatgptLimpar:
		return "chatgpt-limpar"
	case strings.HasPrefix(id, "ia:"):
		return "ia"
	case strings.HasPrefix(id, "pagina:"):
		return "pagina"
	case strings.HasPrefix(id, "ir:"):
		return "ir"
	case strings.HasPrefix(id, "abrirpasta:"):
		return "abrirpasta"
	default:
		return "acao"
	}
}

func (a *app) executarItemMenu(id string) {
	defer a.limpezaAposAcao("menu:" + id)

	if a.executarRecursoMenu(id) {
		return
	}

	switch destinoMenu(id) {
	case "chatgpt-limpar":
		a.abrirExclusaoChatGPT()
	case "ia":
		a.acaoDeIA(id)
	case "pagina":
		a.abrirPagina(id[7:])
	case "ir":
		a.irPara(id[3:])
	case "abrirpasta":
		a.abrirPasta(dataDir)
	default:
		a.doAction(id)
	}
}

// executarRecursoMenu executa recurso de menu; true = tratado.
func (a *app) executarRecursoMenu(id string) bool {
	switch destinoMenu(id) {
	case idBuscador:
		a.escolherMotor(strings.TrimPrefix(id, "buscador:"))
	case "perm":
		a.aplicarPermissaoMenu(id)
	case idSeg:
		a.executarAcaoSeguranca(strings.TrimPrefix(id, "seg:"))
	case "aba", "aba:?":
		a.executarAcaoAba(a.alvoAcaoAba(), strings.TrimPrefix(id, "aba:"))
	case "escolha":
		a.aplicarEscolhaLink(id)
	case "irtab":
		a.ativarAbaIrMenu(id)
	case "":
		return true
	case "idioma":
		a.acaoDeIdioma(id)
	default:
		return false
	}

	return true
}

// alvoAcaoAba resolve a guia alvo do menu de contexto.
func (a *app) alvoAcaoAba() int {
	if a.contextAba >= 0 {
		return a.contextAba
	}

	return a.ativa
}

// ativarAbaIrMenu ativa a aba indicada pelo item irtab.
func (a *app) ativarAbaIrMenu(id string) {
	if n, err := strconv.Atoi(strings.TrimPrefix(id, "irtab:")); err == nil {
		a.ativarAba(n)
	}
}

// executarAcaoAba executa a acao de guia do menu de contexto.
func (a *app) executarAcaoAba(alvo int, suf string) {
	if a.executarAcaoGuiaNova(alvo, suf) {
		return
	}

	switch suf {
	case sufAbaCopiar:
		a.copiarURLGuiaAlvo(alvo)
	case sufAbaFechar:
		a.fecharAba(alvo)
	case sufAbaFecharOutras:
		a.fecharOutrasGuias(alvo)
	case sufAbaFecharDireita:
		a.fecharGuiasADireita(alvo)
	default:
	}
}

// executarAcaoGuiaNova executa criacao e navegacao de guia; true = tratado.
func (a *app) executarAcaoGuiaNova(alvo int, suf string) bool {
	switch suf {
	case sufAbaNova:
		a.novaAba("", true)
	case sufAbaReabrir:
		a.reabrirGuiaFechada()
	case sufAbaAtivar:
		a.ativarAba(alvo)
	case sufAbaFixar:
		a.fixarAba(alvo)
	case sufAbaDuplicar:
		a.duplicarGuia(alvo)
	case sufAbaRecarregar:
		a.recarregarGuiaAlvo(alvo)
	default:
		return false
	}

	return true
}

// recarregarGuiaAlvo recarrega a guia alvo se o painel estiver pronto.
func (a *app) recarregarGuiaAlvo(alvo int) {
	if alvo >= 0 && alvo < len(a.tabs) {
		p := a.tabs[alvo].pane
		if p != nil && p.ready && p.wv != nil {
			p.wv.Reload()
		}
	}
}

// copiarURLGuiaAlvo copia o URL da guia alvo para o clipboard.
func (a *app) copiarURLGuiaAlvo(alvo int) {
	if alvo >= 0 && alvo < len(a.tabs) {
		if u := snapshotTab(a.tabs[alvo]).URL; u != "" && copiarTexto(u) {
			a.dizerf(txtSegEndCopiado)
		}
	}
}

func menuProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		return pintarMenuPrincipal(aplicativo, hwnd)
	case wmSize:
		d2dMenu.redimensionar(menu.w, menu.h)

		return 0

	case wmEraseBkgnd:
		return 1

	case wmMouseWheel:
		rolarMenuPrincipal(aplicativo, wparam)

		return 0

	case wmMouseMove:
		moverMenuPrincipal(aplicativo, hwnd, lparam)

		return 0

	case wmLButtonDown:
		pressionarMenuPrincipal(aplicativo, hwnd, lparam)

		return 0

	case wmLButtonUp:
		soltarMenuPrincipal(aplicativo, hwnd, lparam)

		return 0

	case wmKeyDown:
		teclaMenuPrincipal(aplicativo, wparam)

		return 0

	case wmActivate:
		ativarMenuPrincipal(aplicativo, wparam)

		return 0
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

// ativarMenuPrincipal fecha o menu ao perder a ativacao.
func ativarMenuPrincipal(aplicativo *app, wparam uintptr) {
	if loword(wparam) == waInactive && aplicativo != nil && menu.open {
		aplicativo.closeMenu()
	}
}

// pintarMenuPrincipal pinta a janela do menu principal.
func pintarMenuPrincipal(aplicativo *app, hwnd uintptr) uintptr {
	if aplicativo != nil {
		if disp := d2dMotorDe(hwnd, menu.w, menu.h); disp != nil {
			disp.iniciar()
			aplicativo.paintMenuWinD2D(&menu, true)
			disp.apresentar()
		} else {
			pValidateRect.Call(hwnd, 0)
		}

		return 0
	}

	pValidateRect.Call(hwnd, 0)

	return 0
}

// rolarMenuPrincipal rola menu ou submenu conforme o pai aberto.
func rolarMenuPrincipal(aplicativo *app, wparam uintptr) {
	if aplicativo == nil {
		return
	}

	delta := int32(int16(uint16(wparam >> deslocWordAltoMenu)))
	if menu.subPai >= 0 {
		aplicativo.scrollMenu(&menuSub, delta)
	} else {
		aplicativo.scrollMenu(&menu, delta)
	}
}

// moverMenuPrincipal trata o hover do mouse no menu.
func moverMenuPrincipal(aplicativo *app, hwnd uintptr, lparam uintptr) {
	if aplicativo == nil {
		return
	}

	x, y := loword(lparam), hiword(lparam)

	aplicativo.moverZoomMenuPrincipal(hwnd, x, y)

	if idx := aplicativo.menuItemAtXY(x, y); idx >= 0 {
		aplicativo.moverItemMenuPrincipal(hwnd, x, y, idx)
	}

	aplicativo.moverSubmenusPrincipal(hwnd, x, y)
}

// moverSubmenusPrincipal mantem cada nivel ativo apenas quando o pai mais
// proximo esta sob o cursor (ou o cursor esta dentro do proprio submenu).
func (a *app) moverSubmenusPrincipal(hwnd uintptr, x, y int32) {
	dentro2 := a.dentroSubMenu2(x, y)
	dentro1 := a.dentroSubMenu(x, y)

	switch {
	case dentro2:
		a.moverHoverNivel(&menuSub2, x, y)
		a.realcarPaiSubMenu2(hwnd)
	case dentro1:
		a.limparHoverNivel(&menuSub2)
		a.moverHoverNivel(&menuSub, x, y)
		a.realcarPaiSubMenu(hwnd)
		a.atualizarSubMenu2Hover()
	default:
		a.limparHoverNivel(&menuSub2)
		a.limparHoverNivel(&menuSub)
		a.closeSubMenu2()
	}
}

// moverHoverNivel atualiza o item sob o cursor dentro de um nivel do menu.
func (a *app) moverHoverNivel(janela *menuWin, x, y int32) {
	_, sy := x-janela.dx, y-janela.dy
	if i := a.menuItemAtIn(janela, sy); i != janela.hover {
		janela.hover = i
		pInvalidateRect.Call(janela.hwnd, 0, 0)
	}
}

// limparHoverNivel remove o realce de um nivel quando o cursor sai dele.
func (a *app) limparHoverNivel(janela *menuWin) {
	if janela.hover == -1 {
		return
	}

	janela.hover = -1
	if janela.hwnd != 0 {
		pInvalidateRect.Call(janela.hwnd, 0, 0)
	}
}

// realcarPaiSubMenu mantem o item do menu raiz realcado enquanto o primeiro nivel esta ativo.
func (a *app) realcarPaiSubMenu(hwnd uintptr) {
	if menu.hover != menu.subPai {
		menu.hover = menu.subPai

		pInvalidateRect.Call(hwnd, 0, 0)
	}
}

// realcarPaiSubMenu2 mantem os dois pais realcados enquanto o terceiro nivel esta ativo.
func (a *app) realcarPaiSubMenu2(hwnd uintptr) {
	a.realcarPaiSubMenu(hwnd)

	if menuSub.hover != menu.sub2Pai && menuSub.hwnd != 0 {
		menuSub.hover = menu.sub2Pai
		pInvalidateRect.Call(menuSub.hwnd, 0, 0)
	}
}

// moverZoomMenuPrincipal atualiza o hover da faixa de zoom.
func (a *app) moverZoomMenuPrincipal(hwnd uintptr, x, y int32) {
	if hz := a.menuZoomAt(x, y); hz > 0 || menu.hoverZoom > 0 {
		if hz != menu.hoverZoom {
			menu.hoverZoom = hz

			pInvalidateRect.Call(hwnd, 0, 0)
		}
	}
}

// moverItemMenuPrincipal atualiza hover e submenu do item.
func (a *app) moverItemMenuPrincipal(hwnd uintptr, x, y int32, idx int) {
	if idx != menu.hover {
		menu.hover = idx
		menu.hoverZona = 0

		pInvalidateRect.Call(hwnd, 0, 0)
	}

	if item := menu.items[idx]; item.kind == miExt {
		a.moverItemExtMenuPrincipal(hwnd, x, y, idx, item)
	} else if menu.hoverZona != 0 {
		menu.hoverZona = 0

		pInvalidateRect.Call(hwnd, 0, 0)
	}

	if menu.items[idx].kind != miExt {
		a.agendarSubMenuPrincipal(idx)
	}
}

// moverItemExtMenuPrincipal trata hover do pino e abertura do submenu.
func (a *app) moverItemExtMenuPrincipal(hwnd uintptr, x, y int32, idx int, item menuItem) {
	pinArea := a.extRowZona(&menu, a.menuRowTop(&menu, idx))

	zona := 0
	if dentroRect(pinArea, x, y) {
		zona = 1
	}

	if zona != menu.hoverZona {
		menu.hoverZona = zona

		pInvalidateRect.Call(hwnd, 0, 0)
	}

	if len(item.sub) > 0 {
		if menu.subPai != idx {
			a.abrirSubMenu(idx)
		}
	} else if menu.subPai >= 0 {
		a.closeSubMenu()
	}
}

// agendarSubMenuPrincipal agenda ou cancela a abertura do submenu.
func (a *app) agendarSubMenuPrincipal(idx int) {
	if len(menu.items[idx].sub) > 0 {
		a.armarAtrasoSubMenu(idx)

		return
	}

	if menu.subPai >= 0 {
		a.cancelarAtrasoSubMenu()
	}
}

// armarAtrasoSubMenu agenda a abertura atrasada do submenu.
func (a *app) armarAtrasoSubMenu(idx int) {
	if menu.subPai != idx {
		if a.subAtrasoIdx != idx || a.subAtrasoQuando.IsZero() {
			a.subAtrasoIdx = idx
			a.subAtrasoQuando = time.Now()
			pSetTimer.Call(a.hwnd, timerSubDelay, atrasoSubMenuMs, 0)
		}

		return
	}

	a.subAtrasoIdx = -1
	a.subAtrasoQuando = time.Time{}
}

// cancelarAtrasoSubMenu cancela o atraso e fecha o submenu.
func (a *app) cancelarAtrasoSubMenu() {
	a.subAtrasoIdx = -1
	a.subAtrasoQuando = time.Time{}
	pKillTimer.Call(a.hwnd, timerSubDelay)
	a.closeSubMenu()
}

// atualizarSubMenu2Hover abre ou fecha o terceiro nivel conforme o item.
func (a *app) atualizarSubMenu2Hover() {
	idx := menuSub.hover
	if idx < 0 || idx >= len(menuSub.items) {
		a.closeSubMenu2()

		return
	}

	item := menuSub.items[idx]
	if len(item.sub) == 0 {
		a.closeSubMenu2()

		return
	}

	if menu.sub2Pai != idx {
		a.abrirSubMenu2(idx)
	}
}

// pressionarMenuPrincipal fecha o menu ao clicar fora dele.
func pressionarMenuPrincipal(aplicativo *app, _ uintptr, lparam uintptr) {
	x, y := loword(lparam), hiword(lparam)

	dentro := (x >= 0 && y >= 0 && x < menu.w && y < menu.h) ||
		aplicativo.dentroSubMenu(x, y) || aplicativo.dentroSubMenu2(x, y)
	if !dentro && aplicativo != nil {
		var ponto struct{ x, y int32 }
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&ponto)))
		pScreenToClient.Call(aplicativo.hwnd, uintptr(unsafe.Pointer(&ponto)))
		panelID := aplicativo.panelButtonAt(ponto.x, ponto.y)
		aplicativo.closeMenu()

		if panelID == idPainelMenu {
			aplicativo.onLButtonDown(ponto.x, ponto.y)
		}
	}
}

// soltarMenuPrincipal trata o clique que escolhe o item.
func soltarMenuPrincipal(aplicativo *app, hwnd uintptr, lparam uintptr) {
	if aplicativo == nil {
		return
	}

	// A soltada que abre o menu pela barra cai aqui pelo capture: consome
	// uma vez para nao executar o item sob o cursor.
	if menu.ignorarSoltura {
		menu.ignorarSoltura = false

		return
	}

	x, y := loword(lparam), hiword(lparam)
	if soltarSubMenu2Principal(aplicativo, x, y) {
		return
	}

	if soltarSubMenuPrincipal(aplicativo, x, y) {
		return
	}

	if soltarZoomMenuPrincipal(aplicativo, x, y) {
		return
	}

	soltarItemMenuPrincipal(aplicativo, hwnd, x, y)
}

// soltarSubMenuPrincipal executa o item do submenu; true = tratado.
func soltarSubMenuPrincipal(aplicativo *app, x, y int32) bool {
	if !aplicativo.dentroSubMenu(x, y) {
		return false
	}

	if idx := aplicativo.menuItemAtIn(&menuSub, y-menuSub.dy); idx >= 0 && idx < len(menuSub.items) {
		item := menuSub.items[idx]
		if len(item.sub) > 0 {
			aplicativo.abrirSubMenu2(idx)

			return true
		}

		id := item.id
		if item.kind == miIA {
			id = "ia:" + item.id
		}

		aplicativo.closeMenu()
		aplicativo.executarItemMenu(id)
	}

	return true
}

// soltarSubMenu2Principal executa o item do terceiro nivel; true = tratado.
func soltarSubMenu2Principal(aplicativo *app, x, y int32) bool {
	if !aplicativo.dentroSubMenu2(x, y) {
		return false
	}

	if idx := aplicativo.menuItemAtIn(&menuSub2, y-menuSub2.dy); idx >= 0 && idx < len(menuSub2.items) {
		id := menuSub2.items[idx].id

		aplicativo.closeMenu()
		aplicativo.executarItemMenu(id)
	}

	return true
}

// soltarZoomMenuPrincipal executa a acao de zoom; true = tratado.
func soltarZoomMenuPrincipal(aplicativo *app, x, y int32) bool {
	switch aplicativo.menuZoomAt(x, y) {
	case 1:
		aplicativo.doAction("zin")

		return true
	case zonaZoomPctMenu:
		aplicativo.doAction("zreset")

		return true
	case zonaZoomMaisMenu:
		aplicativo.doAction("zout")

		return true
	case zonaZoomAjusteMenu:
		aplicativo.doAction("zoomfit")
		aplicativo.closeMenu()

		return true
	}

	return false
}

// soltarItemMenuPrincipal executa o item clicado do menu principal.
func soltarItemMenuPrincipal(aplicativo *app, hwnd uintptr, x, y int32) {
	if idx := aplicativo.menuItemAtXY(x, y); idx >= 0 && idx < len(menu.items) {
		item := menu.items[idx]
		if item.kind == miExt {
			soltarItemExtMenuPrincipal(aplicativo, hwnd, x, y, idx, item)

			return
		}

		if len(item.sub) > 0 {
			aplicativo.abrirSubMenu(idx)

			return
		}

		aplicativo.closeMenu()
		aplicativo.executarItemMenu(item.id)
	}
}

// soltarItemExtMenuPrincipal trata pino ou abertura da extensao.
func soltarItemExtMenuPrincipal(aplicativo *app, hwnd uintptr, x, y int32, idx int, item menuItem) {
	pinArea := aplicativo.extRowZona(&menu, aplicativo.menuRowTop(&menu, idx))
	if dentroRect(pinArea, x, y) {
		if strings.ToLower(strings.TrimSpace(item.ext)) != idUbo {
			aplicativo.alternarFixada(item.ext, !aplicativo.extFixada(item.ext))
			menu.items = aplicativo.buildMenuExtensoes()
			aplicativo.menuMetrics()
			pInvalidateRect.Call(hwnd, 0, 1)
		}

		return
	}

	aplicativo.closeMenu()
	aplicativo.abrirPainelExtensao(item.ext)
}

// teclaMenuPrincipal trata as teclas com o menu aberto.
func teclaMenuPrincipal(aplicativo *app, wparam uintptr) {
	if aplicativo == nil {
		return
	}

	switch uint32(wparam) {
	case vkEscape:
		if menu.subPai >= 0 {
			aplicativo.closeSubMenu()

			return
		}

		aplicativo.closeMenu()

		return
	case vkRight:
		if menu.hover >= 0 {
			aplicativo.abrirSubMenu(menu.hover)
		}

		return
	case vkLeft:
		aplicativo.closeSubMenu()

		return
	case vkUp, vkDown:
		aplicativo.teclaSetaMenuPrincipal(uint32(wparam) == vkUp)

		return
	case vkReturn:
		aplicativo.teclaEnterMenuPrincipal()

		return
	}
}

// teclaSetaMenuPrincipal move o hover para o proximo item valido.
func (a *app) teclaSetaMenuPrincipal(paraCima bool) {
	step := 1
	if paraCima {
		step = -1
	}

	idx := menu.hover
	for range menu.items {
		idx = proximoIndiceMenu(idx, step)

		if itemNavegavelMenu(menu.items[idx].kind) {
			break
		}
	}

	menu.hover = idx
	a.menuRevelar(&menu, idx)
	a.closeSubMenu()
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

// proximoIndiceMenu avanca o indice com volta ao inicio/fim.
func proximoIndiceMenu(idx, step int) int {
	idx += step
	if idx < 0 {
		return len(menu.items) - 1
	}

	if idx >= len(menu.items) {
		return 0
	}

	return idx
}

// itemNavegavelMenu diz se o tipo entra na navegacao por teclado.
func itemNavegavelMenu(kind int) bool {
	return kind == miItem || kind == miSub ||
		kind == miCheck || kind == miRadio ||
		kind == miExt
}

// teclaEnterMenuPrincipal executa o item em hover.
func (a *app) teclaEnterMenuPrincipal() {
	if menu.hover < 0 || menu.hover >= len(menu.items) {
		return
	}

	item := menu.items[menu.hover]
	if item.kind == miExt {
		a.closeMenu()
		a.abrirPainelExtensao(item.ext)

		return
	}

	if len(item.sub) > 0 {
		a.abrirSubMenu(menu.hover)

		return
	}

	a.closeMenu()
	a.executarItemMenu(item.id)
}

func menuSubProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		if aplicativo != nil {
			if disp := d2dMotorDe(hwnd, menuSub.w, menuSub.h); disp != nil {
				disp.iniciar()
				aplicativo.paintMenuWinD2D(&menuSub, false)
				disp.apresentar()
			} else {
				pValidateRect.Call(hwnd, 0)
			}

			return 0
		}

		pValidateRect.Call(hwnd, 0)

		return 0
	case wmSize:
		d2dSub.redimensionar(menuSub.w, menuSub.h)

		return 0
	case wmEraseBkgnd:
		return 1
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func menuSub2Proc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		if aplicativo != nil {
			if disp := d2dMotorDe(hwnd, menuSub2.w, menuSub2.h); disp != nil {
				disp.iniciar()
				aplicativo.paintMenuWinD2D(&menuSub2, false)
				disp.apresentar()
			} else {
				pValidateRect.Call(hwnd, 0)
			}

			return 0
		}

		pValidateRect.Call(hwnd, 0)

		return 0
	case wmSize:
		d2dSub2.redimensionar(menuSub2.w, menuSub2.h)

		return 0
	case wmEraseBkgnd:
		return 1
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func panelMenuProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		return pintarMenuPainelProc(aplicativo, hwnd)
	case wmSize:
		d2dPanel.redimensionar(panelMenu.w, panelMenu.h)

		return 0
	case wmEraseBkgnd:
		return 1
	case wmMouseWheel:
		aplicativo.rolarMenuPainelProc(wparam)

		return 0
	case wmMouseMove:
		aplicativo.moverMenuPainelProc(hwnd, lparam)

		return 0
	case wmLButtonUp:
		aplicativo.soltarMenuPainelProc(lparam)

		return 0
	case wmKeyDown:
		return teclaMenuPainelProc(aplicativo, wparam)
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

// pintarMenuPainelProc pinta a janela do menu do painel.
func pintarMenuPainelProc(aplicativo *app, hwnd uintptr) uintptr {
	if aplicativo != nil {
		if disp := d2dMotorDe(hwnd, panelMenu.w, panelMenu.h); disp != nil {
			disp.iniciar()
			aplicativo.paintMenuWinD2D(&panelMenu, false)
			disp.apresentar()
		} else {
			pValidateRect.Call(hwnd, 0)
		}

		return 0
	}

	pValidateRect.Call(hwnd, 0)

	return 0
}

// rolarMenuPainelProc rola o menu do painel.
func (a *app) rolarMenuPainelProc(wparam uintptr) {
	if a == nil {
		return
	}

	a.scrollMenu(&panelMenu, int32(int16(uint16(wparam>>deslocWordAltoMenu))))
}

// moverMenuPainelProc atualiza o hover do menu do painel.
func (a *app) moverMenuPainelProc(hwnd uintptr, lparam uintptr) {
	if a == nil {
		return
	}

	if idx := a.menuItemAtIn(&panelMenu, hiword(lparam)); idx != panelMenu.hover {
		panelMenu.hover = idx

		pInvalidateRect.Call(hwnd, 0, 0)
	}
}

// soltarMenuPainelProc executa o item clicado do menu do painel.
func (a *app) soltarMenuPainelProc(lparam uintptr) {
	if a == nil {
		return
	}

	if idx := a.menuItemAtIn(&panelMenu, hiword(lparam)); idx >= 0 && idx < len(panelMenu.items) {
		item := panelMenu.items[idx]

		id := item.id
		if item.kind == miIA {
			id = "ia:" + item.id
		}

		a.executarItemMenuPainel(id)
	} else {
		a.fecharMenuPainel()
	}
}

// teclaMenuPainelProc fecha o menu do painel no escape.
func teclaMenuPainelProc(aplicativo *app, wparam uintptr) uintptr {
	if aplicativo != nil && uint32(wparam) == vkEscape {
		aplicativo.fecharMenuPainel()
	}

	return 0
}
