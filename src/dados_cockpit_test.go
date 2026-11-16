//go:build windows

package main

import (
	"strings"
	"testing"
)

const dominioCockpitTeste = ".a.com"

func cockpitAmostra() []cockpitCookie {
	return []cockpitCookie{
		{
			Nome: "sid", Dominio: "b.com", Caminho: "/", ValorTm: 10,
			Expira: 1893456000, SoHTTP: false, Seguro: false, Mesmo: "",
		},
		{
			Nome: "a", Dominio: dominioCockpitTeste, Caminho: "/", ValorTm: 4,
			Expira: 0, SoHTTP: false, Seguro: false, Mesmo: "",
		},
		{
			Nome: "z", Dominio: "a.com", Caminho: "/", ValorTm: 6,
			Expira: 0, SoHTTP: true, Seguro: false, Mesmo: "",
		},
	}
}

// TestCockpitOrdemDominios garante grade estavel por dominio e nome.
func TestCockpitOrdemDominios(t *testing.T) {
	t.Parallel()

	cookies := cockpitAmostra()
	ordenarCookiesCockpit(cookies)

	if cookies[0].Dominio != dominioCockpitTeste || cookies[1].Dominio != "a.com" || cookies[2].Dominio != "b.com" {
		t.Errorf("ordem errada: %+v", cookies)
	}

	dominios := dominiosCockpit(cookies)
	if len(dominios) != 3 || dominios[0] != dominioCockpitTeste {
		t.Errorf("dominios errados: %q", dominios)
	}
}

// TestCockpitFiltro garante busca por dominio ou nome.
func TestCockpitFiltro(t *testing.T) {
	t.Parallel()

	cookies := cockpitAmostra()

	if got := filtrarCookiesCockpit(cookies, ""); len(got) != 3 {
		t.Errorf("filtro vazio devia passar tudo, deu %d", len(got))
	}

	if got := filtrarCookiesCockpit(cookies, "b.c"); len(got) != 1 || got[0].Nome != "sid" {
		t.Errorf("filtro b.c errado: %+v", got)
	}

	if got := filtrarCookiesCockpit(cookies, "SID"); len(got) != 1 {
		t.Errorf("filtro devia ignorar caixa, deu %+v", got)
	}
}

// TestCockpitTamanhoExpira garante tamanho e rotulo de sessao.
func TestCockpitTamanhoExpira(t *testing.T) {
	t.Parallel()

	cookies := cockpitAmostra()

	if got := tamanhoCookiesCockpit(cookies); got != int64(len("sid")+10+len("a")+4+len("z")+6) {
		t.Errorf("tamanho errado: %d", got)
	}

	if got := expiraCookieCockpit(cookies[1]); got != txtCockpitSessao {
		t.Errorf("sem expiracao devia ser sessao, deu %q", got)
	}

	if got := expiraCookieCockpit(cookies[0]); !strings.Contains(got, "/") {
		t.Errorf("com expiracao devia ser data, deu %q", got)
	}
}

// TestCockpitOrigemParams garante origem normalizada e JSON valido.
func TestCockpitOrigemParams(t *testing.T) {
	t.Parallel()

	if got := origemDeDominioCockpit(".Exemplo.COM "); got != "https://exemplo.com" {
		t.Errorf("origem errada: %q", got)
	}

	if got := cockpitDominioDeOrigem("https://x.com"); got != "x.com" {
		t.Errorf("dominio errado: %q", got)
	}

	params := paramsApagarCookieCockpit("n", "d.com", "/p")
	if !strings.Contains(params, `"name":"n"`) || !strings.Contains(params, `"path":"/p"`) {
		t.Errorf("params sem path errados: %s", params)
	}

	params = paramsApagarCookieCockpit("n", "d.com", "")
	if strings.Contains(params, "path") {
		t.Errorf("params sem path nao deviam ter path: %s", params)
	}
}
