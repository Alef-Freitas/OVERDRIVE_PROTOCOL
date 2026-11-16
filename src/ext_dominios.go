//go:build windows

package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unsafe"
)

type regraDomExt struct {
	Modo string   `json:"modo,omitempty"`
	Pats []string `json:"pats,omitempty"`
}

//nolint:gochecknoglobals // Cache de regex de dominios + mutex: estado de processo.
var (
	cachePadraoMu sync.RWMutex
	cachePadrao   = map[string]*regexp.Regexp{}
)

func rePadraoDominio(pat string) *regexp.Regexp {
	cachePadraoMu.RLock()

	if re, ok := cachePadrao[pat]; ok {
		cachePadraoMu.RUnlock()

		return re
	}

	cachePadraoMu.RUnlock()

	var montador strings.Builder
	montador.WriteString("(?i)^")

	for _, r := range pat {
		if r == '*' {
			montador.WriteString(".*")
		} else {
			montador.WriteString(regexp.QuoteMeta(string(r)))
		}
	}

	montador.WriteString("$")

	expressao, err := regexp.Compile(montador.String())
	if err != nil {
		return nil
	}

	cachePadraoMu.Lock()

	const limiteCachePadraoDominio = 500

	if len(cachePadrao) >= limiteCachePadraoDominio {
		cachePadrao = map[string]*regexp.Regexp{}
	}

	cachePadrao[pat] = expressao
	cachePadraoMu.Unlock()

	return expressao
}

func casaPadraoDominio(pat, dom string) bool {
	pat = strings.ToLower(strings.TrimSpace(pat))

	dom = strings.ToLower(strings.TrimSpace(dom))
	if pat == "" || dom == "" {
		return false
	}

	if !strings.Contains(pat, "*") {
		return pat == dom
	}

	re := rePadraoDominio(pat)
	if re == nil {
		return false
	}

	return re.MatchString(dom)
}

func (regra regraDomExt) permiteModoSo(dom string) bool {
	if len(regra.Pats) == 0 {
		return true
	}

	for _, p := range regra.Pats {
		if casaPadraoDominio(p, dom) {
			return true
		}
	}

	return false
}

func (regra regraDomExt) permiteModoNao(dom string) bool {
	for _, p := range regra.Pats {
		if casaPadraoDominio(p, dom) {
			return false
		}
	}

	return true
}

func (regra regraDomExt) permite(dom string, ligada bool) bool {
	if !ligada {
		return false
	}

	if dom == "" {
		return true
	}

	switch regra.Modo {
	case "so":
		return regra.permiteModoSo(dom)
	case modoNao:
		return regra.permiteModoNao(dom)
	default:
		return true
	}
}

func domCaractereValidoRegra(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '*'
}

func domValidoRegra(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}

	for _, r := range s {
		if !domCaractereValidoRegra(r) {
			return false
		}
	}

	return true
}

func (a *app) resumoDominios(base string) string {
	regra, ok := a.st.ExtDominios[base]
	if !ok || (regra.Modo != "so" && regra.Modo != modoNao) {
		return txtDomSempre
	}

	const maxPadroesResumoDominio = 2

	lista := strings.Join(regra.Pats, ", ")
	if len(regra.Pats) > maxPadroesResumoDominio {
		lista = strings.Join(regra.Pats[:maxPadroesResumoDominio], ", ") + ", …"
	}

	if regra.Modo == "so" {
		return fmt.Sprintf(txtFmtDomSoEm, lista)
	}

	return fmt.Sprintf(txtFmtDomExcetoEm, lista)
}

func dominioExtrairPedido(id string) (string, string, string, bool) {
	const posicaoDominioPedido = 3

	partes := strings.Split(id, ":")
	if len(partes) < posicaoDominioPedido {
		return "", "", "", false
	}

	acao := partes[1]
	base := strings.ToLower(strings.TrimSpace(partes[2]))

	dom := ""
	if len(partes) > posicaoDominioPedido {
		dom = strings.ToLower(strings.TrimSpace(partes[posicaoDominioPedido]))
	}

	return acao, base, dom, true
}

func (a *app) dominioBaseLiberada(base string) bool {
	if a == nil || base == "" || a.pastaRealExtensao(base) == "" {
		return false
	}

	return !ehExtensaoSistema(base)
}

func (a *app) dominioAplicarModo(base, acao, dom string) bool {
	if a.st.ExtDominios == nil {
		a.st.ExtDominios = map[string]regraDomExt{}
	}

	if acao == "off" {
		delete(a.st.ExtDominios, base)

		return true
	}

	if !domValidoRegra(dom) {
		a.dizerErrof(txtDomPatRuim)

		return false
	}

	modo := "so"
	if acao == modoNao || acao == modoNaow {
		modo = modoNao
	}

	pats := []string{dom}
	if acao == "sow" || acao == modoNaow {
		pats = append(pats, "*."+dom)
	}

	a.st.ExtDominios[base] = regraDomExt{Modo: modo, Pats: pats}

	return true
}

func (a *app) acaoDominioExt(id string) {
	if a == nil {
		return
	}

	acao, base, dom, ok := dominioExtrairPedido(id)
	if !ok {
		return
	}

	if !a.dominioBaseLiberada(base) {
		if base == "" || a.pastaRealExtensao(base) == "" {
			a.dizerErrof(txtDomBaseRuim)
		} else {
			a.dizerErrof(txtDomNaoSistema)
		}

		return
	}

	switch acao {
	case "off", "so", "sow", modoNao, modoNaow:
		if !a.dominioAplicarModo(base, acao, dom) {
			return
		}
	default:
		return
	}

	a.persist()
	a.dizerOkf(txtFmtDomSalva, base, a.resumoDominios(base))
	a.aplicarRegrasDominio()
}

func (a *app) dominioPronto() bool {
	return a != nil && !a.quitting && a.left != nil && a.left.ready && a.left.wv != nil
}

func (a *app) dominioAtual() string {
	dom := dominioDeURL(strings.TrimSpace(a.left.ultimaURL))
	if dom == "" {
		dom = dominioDeURL(a.left.wv.Source())
	}

	return dom
}

func (a *app) dominioAplicarSemRegras() {
	if a.extDominioEstado == nil {
		a.extDominioEstado = map[string]bool{}
	}

	for base := range a.extIDs {
		if ehExtensaoSistema(base) || !a.extensaoLigada(base) {
			continue
		}

		if atual, ok := a.extDominioEstado[base]; ok && atual {
			continue
		}

		a.aplicarEstadoDominio(base, true)
	}
}

func (a *app) dominioAplicarUma(base string, regra regraDomExt, dom string) {
	if ehExtensaoSistema(base) {
		return
	}

	if !a.extensaoLigada(base) {
		a.extDominioEstado[base] = false

		return
	}

	quer := regra.permite(dom, true)
	if atual, ok := a.extDominioEstado[base]; ok && atual == quer {
		return
	}

	if a.extIDs == nil || a.extIDs[base] == "" {
		return
	}

	a.aplicarEstadoDominio(base, quer)
}

func (a *app) aplicarRegrasDominio() {
	if !a.dominioPronto() {
		return
	}
	// Sem regras = extensao ligada em todo lugar. Sem esse caminho, apagar
	// a ultima regra deixava a extensao desligada para sempre.
	if len(a.st.ExtDominios) == 0 {
		a.dominioAplicarSemRegras()

		return
	}

	dom := a.dominioAtual()

	if a.extDominioEstado == nil {
		a.extDominioEstado = map[string]bool{}
	}

	for base, regra := range a.st.ExtDominios {
		a.dominioAplicarUma(base, regra, dom)
	}
}

func dominioAcharExtensao(list *ICoreWebView2BrowserExtensionList, id string) *ICoreWebView2BrowserExtension {
	var total uint32
	list.vtbl.GetCount.Call(uintptr(unsafe.Pointer(list)), uintptr(unsafe.Pointer(&total)))

	for i := range total {
		var ext *ICoreWebView2BrowserExtension

		hr, _, _ := list.vtbl.GetValueAtIndex.Call(
			uintptr(unsafe.Pointer(list)),
			uintptr(i),
			uintptr(unsafe.Pointer(&ext)),
		)
		if Failed(hr) || ext == nil {
			continue
		}

		var idp *uint16
		ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))

		if takeString(idp) == id {
			return ext
		}

		ext.Release()
	}

	return nil
}

func (a *app) dominioConfirmarEstado(base string, quer bool) {
	if a.extDominioEstado == nil {
		a.extDominioEstado = map[string]bool{}
	}

	a.extDominioEstado[base] = quer
	if !quer && a.extFloatOn && a.pastaDoFloat() == base {
		a.fecharPainelFlutuante()
	}
}

func (a *app) aplicarEstadoDominio(base string, quer bool) {
	if a.left == nil || !a.left.ready || a.left.wv == nil {
		return
	}

	id := a.extIDs[base]
	if id == "" {
		return
	}

	a.left.wv.GetBrowserExtensions(func(list *ICoreWebView2BrowserExtensionList, err error) {
		if err != nil || list == nil {
			return
		}
		defer list.Release()

		ext := dominioAcharExtensao(list, id)
		if ext == nil {
			return
		}

		ext.Enable(quer, func(err error) {
			ext.Release()

			if err != nil {
				return
			}

			a.depois(func() { a.dominioConfirmarEstado(base, quer) })
		})
	})
}
