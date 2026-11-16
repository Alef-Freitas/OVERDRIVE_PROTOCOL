//go:build windows

package main

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// rastreadoresQuery lista parametros de rastreamento removidos na navegacao.
// So marketing/atribuicao: nunca remove parametros funcionais (busca,
// paginacao, oauth, sessao). Tudo em minusculo para comparar sem caixa.
var rastreadoresQuery = []string{ //nolint:gochecknoglobals // Lista fixa de leitura.
	"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
	"utm_id", "utm_source_platform", "utm_creative_format", "utm_marketing_tactic",
	"fbclid", "fb_action_ids", "fb_action_types", "fb_source",
	"gclid", "gbraid", "wbraid",
	"msclkid",
	"mc_cid", "mc_eid",
	"igshid",
	"dclid",
	"vero_conv", "vero_id",
	"yclid", "ymclid",
	"twclid",
	"li_fat_id",
	"igsh",
	"srsltid",
	"ctoken",
}

// provedorJSON espelha um provedor do banco ClearURLs.
type provedorJSON struct {
	Padrao   string   `json:"urlPattern"`
	Regras   []string `json:"rules"`
	Excecoes []string `json:"exceptions"`
	Crus     []string `json:"rawRules"`
}

// bancoClearURLs espelha o arquivo de regras ClearURLs.
type bancoClearURLs struct {
	Provedores map[string]provedorJSON `json:"providers"`
}

// provedorLimpeza guarda um provedor com as regex compiladas.
type provedorLimpeza struct {
	padrao   *regexp.Regexp
	regras   []*regexp.Regexp
	excecoes []*regexp.Regexp
	crus     []*regexp.Regexp
}

//nolint:gochecknoglobals // Banco de regras lido uma vez: estado de processo.
var (
	clearURLsUma        sync.Once
	clearURLsProvedores []provedorLimpeza
	clearURLsGlobais    []*regexp.Regexp
	clearURLsCrus       []*regexp.Regexp
)

// LimparTrackingURL remove parametros de rastreamento da URL, mantendo o
// resto intacto. Entrada invalida ou sem rastreador volta inalterada.
func LimparTrackingURL(endereco string) string {
	if endereco == "" {
		return endereco
	}

	saida := removerTrackingExato(endereco)

	return aplicarRegrasClearURLs(saida)
}

// removerTrackingExato tira os parametros conhecidos por nome exato.
func removerTrackingExato(endereco string) string {
	parsed, err := url.Parse(endereco)
	if err != nil || parsed.RawQuery == "" {
		return endereco
	}

	valores, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(valores) == 0 {
		return endereco
	}

	limpou := false

	for _, chave := range rastreadoresQuery {
		for nome := range valores {
			if strings.ToLower(nome) == chave {
				delete(valores, nome)

				limpou = true
			}
		}
	}

	if !limpou {
		return endereco
	}

	parsed.RawQuery = valores.Encode()

	return parsed.String()
}

// compilarRegraClearURLs ancora a regra como o ClearURLs faz (^regra$,
// sem caixa).
func compilarRegraClearURLs(padrao string) (*regexp.Regexp, bool) {
	if padrao == "" {
		return nil, false
	}

	expressao, err := regexp.Compile("(?i)^(?:" + padrao + ")$")
	if err != nil {
		return nil, false
	}

	return expressao, true
}

// compilarAlvoClearURLs compila padrao de URL ou regra crua (sem ancora).
func compilarAlvoClearURLs(padrao string) (*regexp.Regexp, bool) {
	if padrao == "" {
		return nil, false
	}

	expressao, err := regexp.Compile("(?i)" + padrao)
	if err != nil {
		return nil, false
	}

	return expressao, true
}

// carregarRegrasClearURLs le o banco embutido uma unica vez.
func carregarRegrasClearURLs() {
	clearURLsUma.Do(func() {
		var banco bancoClearURLs
		if err := json.Unmarshal([]byte(clearURLsJSON), &banco); err != nil {
			return
		}

		for nome, provedor := range banco.Provedores {
			regras := compilarListaRegras(provedor.Regras)
			crus := compilarListaAlvos(provedor.Crus)

			if nome == "globalRules" {
				clearURLsGlobais = append(clearURLsGlobais, regras...)
				clearURLsCrus = append(clearURLsCrus, crus...)

				continue
			}

			padrao, ok := compilarAlvoClearURLs(provedor.Padrao)
			if !ok {
				continue
			}

			clearURLsProvedores = append(clearURLsProvedores, provedorLimpeza{
				padrao:   padrao,
				regras:   regras,
				excecoes: compilarListaAlvos(provedor.Excecoes),
				crus:     crus,
			})
		}
	})
}

// compilarListaRegras compila regras ancoradas, pulando as invalidas.
func compilarListaRegras(padroes []string) []*regexp.Regexp {
	saida := make([]*regexp.Regexp, 0, len(padroes))

	for _, padrao := range padroes {
		if expressao, ok := compilarRegraClearURLs(padrao); ok {
			saida = append(saida, expressao)
		}
	}

	return saida
}

// compilarListaAlvos compila padroes sem ancora, pulando os invalidos.
func compilarListaAlvos(padroes []string) []*regexp.Regexp {
	saida := make([]*regexp.Regexp, 0, len(padroes))

	for _, padrao := range padroes {
		if expressao, ok := compilarAlvoClearURLs(padrao); ok {
			saida = append(saida, expressao)
		}
	}

	return saida
}

// aplicarRegrasClearURLs remove parametros e trechos pelas regras do banco.
func aplicarRegrasClearURLs(endereco string) string {
	if endereco == "" {
		return endereco
	}

	carregarRegrasClearURLs()

	saida := endereco

	for _, provedor := range clearURLsProvedores {
		if !provedor.padrao.MatchString(saida) || provedorExcecao(provedor, saida) {
			continue
		}

		saida = limparParametros(saida, provedor.regras)

		for _, crua := range provedor.crus {
			saida = crua.ReplaceAllString(saida, "")
		}
	}

	saida = limparParametros(saida, clearURLsGlobais)

	for _, crua := range clearURLsCrus {
		saida = crua.ReplaceAllString(saida, "")
	}

	return saida
}

// provedorExcecao diz se a URL cai em excecao do provedor.
func provedorExcecao(provedor provedorLimpeza, endereco string) bool {
	for _, excecao := range provedor.excecoes {
		if excecao.MatchString(endereco) {
			return true
		}
	}

	return false
}

// limparParametros tira query e fragmento que casem com as regras.
func limparParametros(endereco string, regras []*regexp.Regexp) string {
	if len(regras) == 0 {
		return endereco
	}

	parsed, err := url.Parse(endereco)
	if err != nil {
		return endereco
	}

	valores, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(valores) == 0 {
		valores = nil
	}

	for nome := range valores {
		if casaRegra(nome, regras) {
			delete(valores, nome)
		}
	}

	if valores == nil {
		parsed.RawQuery = ""
	} else {
		parsed.RawQuery = valores.Encode()
	}

	parsed.Fragment = limparFragmento(parsed.Fragment, regras)

	return parsed.String()
}

// casaRegra diz se o nome do parametro casa com alguma regra.
func casaRegra(nome string, regras []*regexp.Regexp) bool {
	for _, regra := range regras {
		if regra.MatchString(nome) {
			return true
		}
	}

	return false
}

// limparFragmento limpa pares chave=valor depois do #, mantendo ancora pura.
func limparFragmento(fragmento string, regras []*regexp.Regexp) string {
	if fragmento == "" || !strings.Contains(fragmento, "=") {
		return fragmento
	}

	pedacos := strings.Split(fragmento, "&")
	mantidos := make([]string, 0, len(pedacos))

	for _, pedaco := range pedacos {
		nome := pedaco
		if antes, _, achou := strings.Cut(pedaco, "="); achou {
			nome = antes
		}

		if !casaRegra(nome, regras) {
			mantidos = append(mantidos, pedaco)
		}
	}

	return strings.Join(mantidos, "&")
}
