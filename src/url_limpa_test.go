//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLimparTrackingURL(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome  string
		entra string
		quer  string
	}{
		{"vazia", "", ""},
		{"sem query", "https://exemplo.com/pag", "https://exemplo.com/pag"},
		{"utm", "https://exemplo.com/?utm_source=google&q=overdrive", "https://exemplo.com/?q=overdrive"},
		{"fbclid", "https://exemplo.com/?fbclid=abc&id=1", "https://exemplo.com/?id=1"},
		{"gclid", "https://exemplo.com/?gclid=abc", "https://exemplo.com/"},
		{"maiuscula", "https://exemplo.com/?UTM_Medium=email&q=1", "https://exemplo.com/?q=1"},
		{"funcional fica", "https://exemplo.com/busca?q=teste&page=2", "https://exemplo.com/busca?page=2&q=teste"},
		{"oauth fica", "https://exemplo.com/cb?code=abc&state=xyz", "https://exemplo.com/cb?code=abc&state=xyz"},
		{"invalida", ":// quebrado", ":// quebrado"},
	}

	for _, caso := range casos {
		if got := LimparTrackingURL(caso.entra); got != caso.quer {
			t.Errorf("%s: entrou %q, saiu %q, quero %q", caso.nome, caso.entra, got, caso.quer)
		}
	}

	if got := LimparTrackingURL("https://exemplo.com/?utm_source=a&utm_source=b&q=1"); !strings.Contains(got, "q=1") ||
		strings.Contains(got, "utm_source") {
		t.Errorf("utm repetido devia sumir e manter q, deu %q", got)
	}
}

func TestLimparTrackingClearURLs(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome  string
		entra string
		quer  string
	}{
		{"amazon ref", "https://www.amazon.com/dp/B0TEST?ref_=test&psc=1", "https://www.amazon.com/dp/B0TEST?psc=1"},
		{"fragmento utm", "https://exemplo.com/pag#utm_source=x&secao=2", "https://exemplo.com/pag#secao=2"},
		{"ancora pura fica", "https://exemplo.com/pag#secao", "https://exemplo.com/pag#secao"},
	}

	for _, caso := range casos {
		if got := LimparTrackingURL(caso.entra); got != caso.quer {
			t.Errorf("%s: entrou %q, saiu %q, quero %q", caso.nome, caso.entra, got, caso.quer)
		}
	}
}

func TestBancoClearURLsCarrega(t *testing.T) {
	t.Parallel()

	carregarRegrasClearURLs()

	if len(clearURLsProvedores) < 100 {
		t.Errorf("banco devia ter mais de 100 provedores, tem %d", len(clearURLsProvedores))
	}

	if len(clearURLsGlobais) == 0 {
		t.Error("banco devia ter regras globais")
	}
}

func TestSalvarCompletoSemPaginaNaoTrava(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: sem pagina.
	aplicativo := &app{}
	aplicativo.salvarCompleto()

	var vazio *WebView
	if vazio.MostrarSalvarComo() {
		t.Error("webview nil nao devia abrir salvar")
	}

	if (&WebView{}).MostrarSalvarComo() {
		t.Error("sem interface _25 nao devia abrir salvar")
	}
}

func TestLeituraEmbutidaPresente(t *testing.T) {
	t.Parallel()

	if len(readabilityEmbutida) < 10000 {
		t.Errorf("readability embutida devia ter mais de 10KB, tem %d bytes", len(readabilityEmbutida))
	}

	if len(readerableEmbutida) < 1000 {
		t.Errorf("readerable embutida devia ter mais de 1KB, tem %d bytes", len(readerableEmbutida))
	}

	if !strings.Contains(readabilityEmbutida, "Readability") {
		t.Error("readability embutida devia conter o motor")
	}

	if !strings.Contains(readerableEmbutida, "isProbablyReaderable") {
		t.Error("readerable embutida devia conter o detector")
	}
}

func TestMotorArquivoUnicoVendido(t *testing.T) {
	t.Parallel()

	if len(singleFileJS) < 500000 {
		t.Errorf("bundle devia ter mais de 500KB, tem %d bytes", len(singleFileJS))
	}

	if !strings.Contains(singleFileJS, "getPageData") {
		t.Error("bundle devia expor getPageData")
	}

	if got := invocacaoArquivoUnico(); !strings.Contains(got, "singlefile.getPageData") {
		t.Error("invocacao devia chamar o motor")
	}
}

func TestInjecaoSingleFileOK(t *testing.T) {
	t.Parallel()

	for _, caso := range []struct {
		res  string
		ok   bool
		erro string
	}{
		{`"{\"ok\":true}"`, true, ""},
		{`{"ok":true}`, true, ""},
		{`"{\"ok\":false,\"err\":\"sem global\"}"`, false, "sem global"},
		{`lixo`, false, "lixo"},
	} {
		if ok, motivo := injecaoSingleFileOK(caso.res); ok != caso.ok || motivo != caso.erro {
			t.Errorf("injecao %q: ok=%v motivo=%q, quero %v %q", caso.res, ok, motivo, caso.ok, caso.erro)
		}
	}
}

func TestSalvarUnicoSemPaginaNaoTrava(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: sem pagina.
	aplicativo := &app{}
	aplicativo.salvarArquivoUnico()
	aplicativo.salvarArquivoUnicoMotor("", "", nil)
	aplicativo.salvarArquivoUnicoMotor("", "", erroTexto("x"))
	aplicativo.salvarArquivoUnicoMotor("", `{"ok":false,"err":"sem global"}`, nil)
	aplicativo.salvarArquivoUnicoCaiu("boom")

	if got := expressaoInjecaoSingleFile(); !strings.Contains(got, "globalThis.singlefile") ||
		!strings.Contains(got, "sem global") {
		t.Error("injecao devia diagnosticar global ausente")
	}

	aplicativo.salvarArquivoUnicoResposta("", erroTexto("x"), "")
	aplicativo.salvarArquivoUnicoResposta("", nil, `{"ok":false}`)
	aplicativo.salvarArquivoUnicoResposta("", nil, `{"ok":true,"html":""}`)
	aplicativo.salvarArquivoUnicoResposta("", nil, `{"ok":false,"err":"boom"}`)
	aplicativo.salvarArquivoUnicoMagra("", "x")
	aplicativo.salvarArquivoUnicoRespostaMagra("", erroTexto("x"), "", "")
	aplicativo.salvarArquivoUnicoRespostaMagra("", nil, `{"ok":false,"err":"y"}`, "")
	aplicativo.salvarArquivoUnicoRespostaMagra("", nil, `{"ok":true,"html":"<p>oi</p>"}`, "")

	if got := invocacaoArquivoUnicoMagra(); !strings.Contains(got, "maxResourceSize") {
		t.Error("versao magra devia ter teto de recurso")
	}

	casoCDP := `{"result":{"value":{"ok":true,"html":"<p>oi</p>"}}}`
	if motivo, html := respostaArquivoUnicoCDP(nil, casoCDP); motivo != "" || html != "<p>oi</p>" {
		t.Errorf("resposta CDP boa devia voltar html, deu %q %q", motivo, html)
	}

	if motivo, html := respostaArquivoUnicoCDP(erroTexto("x"), ""); motivo != "execucao" || html != "" {
		t.Errorf("erro devia voltar execucao, deu %q %q", motivo, html)
	}

	if codigo := cdpString("a\"b\n"); codigo != `"a\"b\n"` {
		t.Errorf("cdpString devia escapar, deu %q", codigo)
	}

	//exhaustruct:ignore // App de teste: sem pagina nem janela.
	quieto := &app{}
	quieto.salvarArquivoUnicoMagra("", "x")
	quieto.avisarFalhaSalvar("boom")
	quieto.salvarArquivoUnicoRespostaMagra("", erroTexto("x"), "", "")
}

func TestResumirErroSalvar(t *testing.T) {
	t.Parallel()

	if got := resumirErroSalvar(""); got != "falha desconhecida" {
		t.Errorf("vazio devia dar padrao, deu %q", got)
	}

	if got := resumirErroSalvar("boom\nlinha2"); got != "boom" {
		t.Errorf("devia cortar na quebra, deu %q", got)
	}

	longo := strings.Repeat("x", 200)
	if got := resumirErroSalvar(longo); len(got) != 123 || !strings.HasSuffix(got, "...") {
		t.Errorf("devia truncar com reticencias, deu %q", got)
	}
}
