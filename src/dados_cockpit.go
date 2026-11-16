//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Limites do cockpit de dados: a grade rolavel aguenta, mas a Cortina
// corta a cauda para nao montar milhares de zonas num so layout.
const (
	maxCookiesCockpit = 500
	maxOrigensCockpit = 40
)

// cockpitCookie e um cookie do perfil com os campos de gestao.
type cockpitCookie struct {
	Nome    string
	Dominio string
	Caminho string
	ValorTm int
	Expira  float64
	SoHTTP  bool
	Seguro  bool
	Mesmo   string
}

// cdpCookieCheio decodifica o objeto cheio de Network.getAllCookies.
type cdpCookieCheio struct {
	Nome     string  `json:"name"`
	Valor    string  `json:"value"`
	Dominio  string  `json:"domain"`
	Caminho  string  `json:"path"`
	Expira   float64 `json:"expires"`
	SoHTTP   bool    `json:"httpOnly"`
	Seguro   bool    `json:"secure"`
	MesmoSit string  `json:"sameSite"`
}

// cdpCookiesCheiosResp decodifica a resposta de Network.getAllCookies.
type cdpCookiesCheiosResp struct {
	Cookies []cdpCookieCheio `json:"cookies"`
}

// paneProntoCockpit diz se o painel pode receber comandos CDP.
func paneProntoCockpit(p *pane) bool {
	return p != nil && p.ready && p.wv != nil
}

// candidatosCockpit lista os paineis na ordem de preferencia do cockpit.
func candidatosCockpit(aplicativo *app) []*pane {
	candidatos := []*pane{aplicativo.left}
	for _, t := range aplicativo.tabs {
		if t != nil {
			candidatos = append(candidatos, t.pane)
		}
	}

	return append(candidatos, aplicativo.right, aplicativo.imgPanel)
}

// paneParaCockpit escolhe um painel pronto para operar os dados do perfil.
func paneParaCockpit(a *app) *pane {
	if a == nil {
		return nil
	}

	for _, p := range candidatosCockpit(a) {
		if paneProntoCockpit(p) {
			return p
		}
	}

	return nil
}

// carregarCookiesNoPane lista todos os cookies do perfil via CDP.
func carregarCookiesNoPane(a *app, p *pane) ([]cockpitCookie, error) {
	if p == nil {
		return nil, erroTexto(txtCockpitSemPainel)
	}

	cru, err := a.chamarCDP(p, "Network.getAllCookies", "{}")
	if err != nil {
		return nil, err
	}

	var resposta cdpCookiesCheiosResp
	if err := json.Unmarshal([]byte(cru), &resposta); err != nil {
		return nil, erroTextoF("cookies invalidos: %w", err)
	}

	saida := make([]cockpitCookie, 0, len(resposta.Cookies))
	for _, cookie := range resposta.Cookies {
		saida = append(saida, cockpitCookie{
			Nome:    cookie.Nome,
			Dominio: cookie.Dominio,
			Caminho: cookie.Caminho,
			ValorTm: len(cookie.Valor),
			Expira:  cookie.Expira,
			SoHTTP:  cookie.SoHTTP,
			Seguro:  cookie.Seguro,
			Mesmo:   cookie.MesmoSit,
		})
	}

	ordenarCookiesCockpit(saida)

	return saida, nil
}

// ordenarCookiesCockpit ordena por dominio e nome para a grade estavel.
func ordenarCookiesCockpit(cookies []cockpitCookie) {
	sort.Slice(cookies, func(i, j int) bool {
		if cookies[i].Dominio != cookies[j].Dominio {
			return cookies[i].Dominio < cookies[j].Dominio
		}

		return cookies[i].Nome < cookies[j].Nome
	})
}

// dominiosCockpit lista os dominios distintos em ordem.
func dominiosCockpit(cookies []cockpitCookie) []string {
	vistos := map[string]bool{}

	var dominios []string

	for _, cookie := range cookies {
		if cookie.Dominio == "" || vistos[cookie.Dominio] {
			continue
		}

		vistos[cookie.Dominio] = true
		dominios = append(dominios, cookie.Dominio)
	}

	sort.Strings(dominios)

	return dominios
}

// filtrarCookiesCockpit filtra por dominio ou nome; vazio passa tudo.
func filtrarCookiesCockpit(cookies []cockpitCookie, filtro string) []cockpitCookie {
	filtro = strings.ToLower(strings.TrimSpace(filtro))
	if filtro == "" {
		return cookies
	}

	saida := make([]cockpitCookie, 0, len(cookies))

	for _, c := range cookies {
		if strings.Contains(strings.ToLower(c.Dominio), filtro) ||
			strings.Contains(strings.ToLower(c.Nome), filtro) {
			saida = append(saida, c)
		}
	}

	return saida
}

// tamanhoCookieCockpit soma nome e valor para a coluna de tamanho.
func tamanhoCookieCockpit(c cockpitCookie) int64 {
	return int64(len(c.Nome) + c.ValorTm)
}

// tamanhoCookiesCockpit soma todos para o resumo.
func tamanhoCookiesCockpit(cookies []cockpitCookie) int64 {
	var total int64

	for _, c := range cookies {
		total += tamanhoCookieCockpit(c)
	}

	return total
}

// expiraCookieCockpit devolve sessao ou a data de expiracao.
func expiraCookieCockpit(c cockpitCookie) string {
	if c.Expira <= 0 {
		return txtCockpitSessao
	}

	return time.Unix(int64(c.Expira), 0).Format("02/01/2006")
}

// origemDeDominioCockpit normaliza o dominio para origem https.
func origemDeDominioCockpit(dominio string) string {
	dominio = strings.ToLower(strings.TrimSpace(dominio))
	dominio = strings.TrimPrefix(dominio, ".")

	return esquemaHTTPS + "://" + dominio
}

// paramsApagarCookieCockpit monta o Network.deleteCookies por nome+dominio.
func paramsApagarCookieCockpit(nome, dominio, caminho string) string {
	if caminho != "" {
		return fmt.Sprintf(`{"name":%q,"domain":%q,"path":%q}`, nome, dominio, caminho)
	}

	return fmt.Sprintf(`{"name":%q,"domain":%q}`, nome, dominio)
}

// apagarCookieNoPane apaga um cookie pelo nome e dominio.
func apagarCookieNoPane(a *app, p *pane, nome, dominio, caminho string) error {
	if p == nil {
		return erroTexto(txtCockpitSemPainel)
	}

	_, err := a.chamarCDP(p, "Network.deleteCookies", paramsApagarCookieCockpit(nome, dominio, caminho))

	return err
}

// apagarCookiesDominioNoPane apaga os cookies do dominio; devolve apagados.
func apagarCookiesDominioNoPane(aplicativo *app, p *pane, cookies []cockpitCookie, dominio string) int {
	apagados := 0

	for _, c := range cookies {
		if !strings.EqualFold(c.Dominio, dominio) {
			continue
		}

		if apagarCookieNoPane(aplicativo, p, c.Nome, c.Dominio, c.Caminho) == nil {
			apagados++
		}
	}

	return apagados
}

// limparCacheNoPane limpa o cache do navegador via CDP.
func limparCacheNoPane(a *app, p *pane) error {
	if p == nil {
		return erroTexto(txtCockpitSemPainel)
	}

	_, err := a.chamarCDP(p, "Network.clearBrowserCache", "{}")

	return err
}

// limparOrigemNoPane limpa cookies, storage e cache de um dominio via CDP.
func limparOrigemNoPane(aplicativo *app, p *pane, dominio string) error {
	if p == nil {
		return erroTexto(txtCockpitSemPainel)
	}

	tipos := "cookies,local_storage,indexeddb,websql,cache_storage,service_workers"

	var ultimo error

	for _, esquema := range []string{esquemaHTTPS, esquemaHTTP} {
		origem := esquema + "://" + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(dominio)), ".")

		params, err := cdpMonta(map[string]string{"origin": origem, "storageTypes": tipos})
		if err != nil {
			ultimo = err

			continue
		}

		_, err = aplicativo.chamarCDP(p, "Storage.clearDataForOrigin", params)
		if err == nil {
			return nil
		}

		ultimo = err
	}

	return ultimo
}

// cfgRecarregarCookies recarrega a grade em segundo plano; a UI nao trava.
func (a *app) cfgRecarregarCookies() {
	if cfgUI.cookiesLendo {
		return
	}

	p := paneParaCockpit(a)
	if p == nil {
		cfgUI.cookiesErro = txtCockpitSemPainel

		a.cfgLayout()
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return
	}

	cfgUI.cookiesLendo = true
	cfgUI.cookiesErro = ""

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

	go func() {
		cookies, err := carregarCookiesNoPane(a, p)
		a.depois(func() {
			cfgUI.cookiesLendo = false

			if err != nil {
				cfgUI.cookiesErro = err.Error()
			} else {
				cfgUI.cookiesCockpit = cookies
				cfgUI.cookiesErro = ""
			}

			if cfgUI.ativo && cfgUI.secao == cfgSecDados {
				a.cfgLayout()
				pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
			}
		})
	}()
}

// cfgApagarTodosCookies apaga os cookies listados em segundo plano.
func (a *app) cfgApagarTodosCookies() {
	p := paneParaCockpit(a)
	if p == nil {
		cfgUI.cookiesErro = txtCockpitSemPainel

		a.cfgLayout()
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return
	}

	lista := append([]cockpitCookie(nil), cfgUI.cookiesCockpit...)

	a.dizerProgressoIndetf(txtCockpitApagando)
	a.invalidateBar()

	go func() {
		for _, c := range lista {
			_ = apagarCookieNoPane(a, p, c.Nome, c.Dominio, c.Caminho)
		}

		a.depois(func() {
			a.dizerOkf(txtDadosApagados)
			a.invalidateBar()
			a.cfgRecarregarCookies()
		})
	}()
}

// cfgLimparCacheFundo limpa o cache em segundo plano.
func (a *app) cfgLimparCacheFundo() {
	p := paneParaCockpit(a)
	if p == nil {
		a.dizerErrof(txtCockpitSemPainel)
		a.invalidateBar()

		return
	}

	a.dizerProgressoIndetf(txtCockpitLimpando)
	a.invalidateBar()

	go func() {
		err := limparCacheNoPane(a, p)
		a.depois(func() {
			if err != nil {
				a.dizerErrof("%s", err.Error())
			} else {
				a.dizerOkf(txtDadosApagados)
			}

			a.invalidateBar()
		})
	}()
}

// cfgApagarTudoFundo apaga cookies e cache em segundo plano.
func (a *app) cfgApagarTudoFundo() {
	alvos := a.panesParaTudoCockpit()
	if len(alvos) == 0 {
		a.dizerErrof(txtCockpitSemPainel)
		a.invalidateBar()

		return
	}

	a.dizerProgressoIndetf(txtCockpitApagando)
	a.invalidateBar()

	go func() {
		for _, p := range alvos {
			_, _ = a.chamarCDP(p, "Network.clearBrowserCache", "{}")
		}

		a.depois(func() {
			if p := paneParaCockpit(a); p != nil && p.wv != nil {
				p.wv.ClearBrowsingDataAll(nil)
			}

			a.limparAposCookies()
			a.cfgRecarregarCookies()
		})
	}()
}

// panesParaTudoCockpit coleta os paineis prontos para a limpeza geral.
func (a *app) panesParaTudoCockpit() []*pane {
	vistos := map[*pane]bool{}

	var alvos []*pane

	for _, p := range candidatosCockpit(a) {
		if !paneProntoCockpit(p) || vistos[p] {
			continue
		}

		vistos[p] = true
		alvos = append(alvos, p)
	}

	return alvos
}
