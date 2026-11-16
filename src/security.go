package main

import (
	"fmt"
	"net/url"
	"strings"
)

func hostDeURL(endereco string) string {
	endereco = strings.TrimSpace(endereco)
	if endereco == "" {
		return ""
	}

	analisado, err := url.Parse(endereco)
	if err != nil {
		return ""
	}

	return strings.ToLower(analisado.Hostname())
}

func (a *app) urlDaPagina() string {
	if a == nil || a.left == nil {
		return ""
	}

	if u := a.urlDoLeitor(); u != "" {
		return u
	}

	if u := a.urlDaAba(); u != "" {
		return u
	}

	return a.urlDoWebView()
}

// urlDoLeitor devolve o URL do modo leitura quando ativo.
func (a *app) urlDoLeitor() string {
	if a.left.readerAtivo && a.left.readerURL != "" {
		return a.left.readerURL
	}

	return ""
}

// urlDaAba devolve a ultima URL da aba quando nao e interna.
func (a *app) urlDaAba() string {
	if a.left.ultimaURL != "" && !strings.HasPrefix(a.left.ultimaURL, "about:") {
		return a.left.ultimaURL
	}

	return ""
}

// urlDoWebView devolve a URL atual do WebView quando nao e interna.
func (a *app) urlDoWebView() string {
	if a.left.ready && a.left.wv != nil {
		if u := a.left.wv.Source(); u != "" && !strings.HasPrefix(u, "about:") {
			return u
		}
	}

	return ""
}

func (a *app) iconeSeguranca() string {
	urlPagina := a.urlDaPagina()

	host := hostDeURL(urlPagina)
	if host == "" {
		return idGlobe
	}

	if strings.HasPrefix(strings.ToLower(urlPagina), "https://") {
		return idLock
	}

	return idUnlock
}

func (a *app) dicaSeguranca() string {
	urlPagina := a.urlDaPagina()

	host := hostDeURL(urlPagina)
	if host == "" {
		return txtDicaPagInterna
	}

	if strings.HasPrefix(strings.ToLower(urlPagina), "https://") {
		return fmt.Sprintf(txtFmtDicaSegura, host)
	}

	return fmt.Sprintf(txtFmtDicaNaoSegura, host)
}

func (a *app) abrirMenuSeguranca() {
	if menu.open && a.menuOrigem == "seguranca" {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	urlPagina := a.urlDaPagina()
	host := hostDeURL(urlPagina)
	seguro := strings.HasPrefix(strings.ToLower(urlPagina), "https://")

	itens := a.cabecalhoSeguranca(host, seguro)
	itens = append(itens, a.itensPermissaoSeguranca(host)...)

	itens = append(itens, menuItem{kind: miSep})
	if urlPagina != "" {
		itens = append(itens, menuItem{kind: miItem, label: txtCopiarEndereco, id: "seg:copiar"})
	}

	if host != "" {
		itens = append(itens, menuItem{kind: miItem, label: txtLimparCookies, id: "seg:cookies"})
	}

	a.menuOrigem = "seguranca"
	menu.items = itens

	a.abrirJanelaSeguranca()
}

// cabecalhoSeguranca monta o cabecalho do menu conforme o esquema.
func (a *app) cabecalhoSeguranca(host string, seguro bool) []menuItem {
	switch {
	case host == "":
		return []menuItem{{kind: miHeader, label: txtHdrPagInterna}}
	case seguro:
		return []menuItem{
			{kind: miHeader, label: txtHdrSegura},
			{kind: miHeader, label: host},
		}
	default:
		return []menuItem{
			{kind: miHeader, label: txtHdrNaoSegura},
			{kind: miHeader, label: host},
			{kind: miItem, label: txtTentarHTTPS, hint: txtHintRecarregar, id: "seg:https"},
		}
	}
}

// itensPermissaoSeguranca lista as permissoes do host com dicas de estado.
func (a *app) itensPermissaoSeguranca(host string) []menuItem {
	if host == "" {
		return nil
	}

	itens := []menuItem{
		{kind: miSep},
		{kind: miHeader, label: txtHdrPermissoes},
	}

	for _, permissao := range permissoesSite {
		hint := txtHintBloqueado
		if a.permissaoPermitidaHost(host, permissao.kind) {
			hint = txtHintPermitido
		}

		itens = append(itens, menuItem{
			kind:  miItem,
			label: permissao.nome,
			hint:  hint,
			id:    fmt.Sprintf("perm:%d|%s", permissao.kind, host),
		})
	}

	return itens
}

// abrirJanelaSeguranca cria a janela do menu e ancora no botao de seguranca.
func (a *app) abrirJanelaSeguranca() {
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		a.dizerf(txtFalhaMenuSeg)

		return
	}

	a.abrirMenuAncorado(a.securityBtn.rect)
}

func (a *app) permissaoPermitidaHost(host string, kind uint32) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if a == nil || host == "" {
		return false
	}

	a.permissoesMu.RLock()
	regrasHost := a.permissoes[host]
	a.permissoesMu.RUnlock()

	if regrasHost == nil {
		return false
	}

	return regrasHost[kind]
}

func (a *app) permissaoPermitida(uri string, kind uint32) bool {
	return a.permissaoPermitidaHost(hostDeURL(uri), kind)
}

func (a *app) alternarPermissao(host string, kind uint32) {
	const limiteHostsPermissao = 256

	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return
	}

	a.permissoesMu.Lock()
	defer a.permissoesMu.Unlock()

	if a.permissoes == nil {
		a.permissoes = map[string]map[uint32]bool{}
	}

	if _, ok := a.permissoes[host]; !ok && len(a.permissoes) >= limiteHostsPermissao {
		a.podarPermissoes()
	}

	regrasHost := a.permissoes[host]
	if regrasHost == nil {
		regrasHost = map[uint32]bool{}
		a.permissoes[host] = regrasHost
	}

	regrasHost[kind] = !regrasHost[kind]
	nome := nomePermissao(kind)

	if regrasHost[kind] {
		a.dizerOkf(txtFmtPermLiberada, nome, host)
	} else {
		a.dizerf(txtFmtPermBloqueada, nome, host)
	}

	a.invalidateBar()
}

// podarPermissoes remove os hosts mais antigos quando o mapa enche.
func (a *app) podarPermissoes() {
	const podaPermissoesQtde = 16

	num := 0

	for h := range a.permissoes {
		delete(a.permissoes, h)

		num++
		if num >= podaPermissoesQtde {
			break
		}
	}
}

// nomePermissao devolve o rotulo da permissao pelo tipo.
func nomePermissao(kind uint32) string {
	for _, permissao := range permissoesSite {
		if permissao.kind == kind {
			return permissao.nome
		}
	}

	return "permissão"
}

func (a *app) aplicarPermissaoMenu(id string) {
	corpo := strings.TrimPrefix(id, "perm:")
	if i := strings.Index(corpo, "|"); i > 0 {
		var kind uint32
		fmt.Sscanf(corpo[:i], "%d", &kind)
		a.alternarPermissao(corpo[i+1:], kind)
	}
}

func (a *app) executarAcaoSeguranca(acao string) {
	switch acao {
	case esquemaHTTPS:
		urlPagina := a.urlDaPagina()

		analisado, err := url.Parse(urlPagina)
		if err != nil || !strings.EqualFold(analisado.Scheme, esquemaHTTP) {
			a.dizerf(txtSegNaoHTTP)

			return
		}

		analisado.Scheme = esquemaHTTPS
		a.navigateLeft(analisado.String())
	case sufAbaCopiar:
		urlPagina := a.urlDaPagina()
		if urlPagina == "" {
			a.dizerf(txtSegSemEnd)

			return
		}

		if copiarTexto(urlPagina) {
			a.dizerf(txtSegEndCopiado)
		} else {
			a.dizerf(txtSegNaoCopiou)
		}
	case idCookies:
		a.limparCookiesDoDominio()
	}
}

func (a *app) acaoMenuContexto(id, valor string) bool {
	valor = strings.TrimSpace(valor)

	switch id {
	case "link:newtab", "img:newtab":
		if valor != "" {
			a.novaAba(valor, true)
			a.status = txtSegAbertoGuia
			a.invalidateBar()
		}
	case "link:copy", "img:copy", "sel:copy":
		if valor != "" && copiarTexto(valor) {
			a.dizerf(txtSegCopiado)
		}
	case "sel:search":
		if valor != "" {
			a.navigateLeft(valor)
		}
	}

	return true
}
