//go:build windows

package main

import (
	"archive/zip"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCasaPadraoDominio(t *testing.T) {
	t.Parallel()

	const hostTeste = "google.com"

	casos := []struct {
		pat, dom string
		ok       bool
	}{
		{hostTeste, hostTeste, true},
		{hostTeste, "mail.google.com", false},
		{"*.google.com", "mail.google.com", true},
		{"*.google.com", hostTeste, false},
		{"*", "qualquer.coisa", true},
		{"GOOGLE.COM", hostTeste, true},
		{"", hostTeste, false},
		{hostTeste, "", false},
		{"*ads*", "ads.example.com", true},
		{"*ads*", "example.com", false},
	}
	for _, c := range casos {
		if got := casaPadraoDominio(c.pat, c.dom); got != c.ok {
			t.Errorf("casaPadraoDominio(%q, %q) = %v, quero %v", c.pat, c.dom, got, c.ok)
		}
	}
	// Determinismo com cache: segunda chamada usa o padrao compilado.
	if !casaPadraoDominio("*.google.com", "mail.google.com") {
		t.Error("cache quebrou o match")
	}
}

func TestTradCortarCodificado(t *testing.T) {
	t.Parallel()

	const maxCod = 1500

	textos := []string{
		"O rato roeu a roupa do rei de Roma. E a rainha, com raiva, remendou tudo!",
		strings.Repeat("palavra ", 400),
		"日本語のテキスト。" + strings.Repeat("テスト ", 300),    //nolint:gosmopolitan // CJK exercita o corte.
		"emoji 🎉🎉🎉 " + strings.Repeat("festa ", 300), // Emoji exercita o corte.
		"a",
		"semespacos" + strings.Repeat("x", 3000),
	}
	for idx, txt := range textos {
		partes := tradCortarCodificado(txt)
		if len(partes) == 0 {
			t.Fatalf("texto %d: nenhuma parte", idx)
		}

		for _, p := range partes {
			if n := len(urlQueryEscape(p)); n > maxCod {
				t.Errorf("texto %d: parte com %d codificados > %d", idx, n, maxCod)
			}
		}
		// Nada perdido nem inventado: concatenar sem separador (os cortes
		// sao mid-word quando nao ha espaco) reconstrói o normalizado.
		// Nada perdido nem inventado: o splitter so come espacos de borda,
		// entao a sequencia de nao-espacos e identica (vale para cortes
		// no meio de palavra gigante e para cortes em espaco).
		antes := strings.Join(strings.Fields(txt), "")

		depois := strings.Join(strings.Fields(strings.Join(partes, " ")), "")
		if antes != depois {
			t.Errorf("texto %d: conteudo diverge", idx)
		}
	}
}

func TestUrlQueryEscape(t *testing.T) {
	t.Parallel()

	if urlQueryEscape("a b") != "a+b" {
		t.Error("espaco vira +")
	}

	if urlQueryEscape("azAZ09-_.~") != "azAZ09-_.~" {
		t.Error("nao reservados passam crus")
	}

	dec, err := url.QueryUnescape(urlQueryEscape("São Paulo & Cia"))
	if err != nil || dec != "São Paulo & Cia" {
		t.Errorf("roundtrip falhou: %q, %v", dec, err)
	}
}

func TestHostPreBloqueado(t *testing.T) {
	t.Parallel()

	if !hostPreBloqueado("https://ads.doubleclick.net/pagina") {
		t.Error("host da lista devia bloquear")
	}

	if !hostPreBloqueado("https://SUB.DOMAIN.Criteo.COM/x") {
		t.Error("maiusculas e subdominio deviam bloquear")
	}

	if !hostPreBloqueado("https://ads.yahoo.com:443/x") {
		t.Error("porta nao devia impedir o match")
	}

	if hostPreBloqueado("https://www.google.com/") {
		t.Error("host fora da lista nao bloqueia")
	}

	if hostPreBloqueado("noticia.txt") {
		t.Error("sem esquema nao bloqueia")
	}
}

func TestAlinharVersaoChrome(t *testing.T) {
	t.Parallel()

	salvaV, salvaC, salvaE := versaoChrome, uaChromeLoja, uaEdgeLoja
	defer func() { versaoChrome, uaChromeLoja, uaEdgeLoja = salvaV, salvaC, salvaE }()

	alinharVersaoChrome("140.0.3485.14")

	if versaoChrome != "140" {
		t.Errorf("versaoChrome = %q, quero 140", versaoChrome)
	}

	if !strings.Contains(uaEdgeLoja, "Edg/140.0.0.0") {
		t.Errorf("uaEdge desatualizada: %q", uaEdgeLoja)
	}

	alinharVersaoChrome("lixo")

	if versaoChrome != "140" {
		t.Error("versao invalida nao devia mudar nada")
	}
}

func TestPortaOrdemRede(t *testing.T) {
	t.Parallel()

	if portaOrdemRede(0x5000) != 80 {
		t.Errorf("0x5000 devia ser 80, deu %d", portaOrdemRede(0x5000))
	}

	if portaOrdemRede(0xBB01) != 443 {
		t.Errorf("0xBB01 devia ser 443, deu %d", portaOrdemRede(0xBB01))
	}
}

func TestRelayBidirecional(t *testing.T) {
	t.Parallel()

	pontaIda, pontaVolta := net.Pipe()
	extremIda, extremVolta := net.Pipe()

	var grupoEspera sync.WaitGroup

	grupoEspera.Go(func() { ; relay(pontaIda, extremIda) })

	prazos := time.Now().Add(10 * time.Second)
	_ = pontaVolta.SetDeadline(prazos)
	_ = extremVolta.SetDeadline(prazos)

	if _, err := pontaVolta.Write([]byte("ida")); err != nil {
		t.Fatalf("escrita ida: %v", err)
	}

	buf := make([]byte, 8)

	num, err := extremVolta.Read(buf)
	if err != nil || string(buf[:num]) != "ida" {
		t.Fatalf("leitura ida: %q, %v", buf[:num], err)
	}

	if _, err := extremVolta.Write([]byte("volta")); err != nil {
		t.Fatalf("escrita volta: %v", err)
	}

	num, err = pontaVolta.Read(buf)
	if err != nil || string(buf[:num]) != "volta" {
		t.Fatalf("leitura volta: %q, %v", buf[:num], err)
	}

	_ = pontaVolta.Close()
	_ = extremVolta.Close()

	grupoEspera.Wait()
}

func BenchmarkTradCortarCodificado(b *testing.B) {
	txt := "O rato roeu a roupa do rei de Roma. " + strings.Repeat("palavra ", 200)

	b.ResetTimer()

	for range b.N {
		_ = tradCortarCodificado(txt)
	}
}

func BenchmarkCasaPadraoDominio(b *testing.B) {
	for range b.N {
		_ = casaPadraoDominio("*.google.com", "mail.google.com")
	}
}

func BenchmarkHostPreBloqueado(b *testing.B) {
	for range b.N {
		_ = hostPreBloqueado("https://ads.doubleclick.net/pagina")
	}
}

func FuzzTradCortarCodificado(f *testing.F) {
	f.Add("O rato roeu a roupa do rei de Roma. E a rainha remendou!")
	f.Add("日本語テスト " + strings.Repeat("x ", 500)) //nolint:gosmopolitan // Seed de fuzz: CJK exercita o corte.
	f.Fuzz(func(t *testing.T, txt string) {
		const maxCod = 1500

		partes := tradCortarCodificado(txt)
		for _, p := range partes {
			if len(urlQueryEscape(p)) > maxCod {
				t.Fatalf("parte estoura o limite: %d", len(urlQueryEscape(p)))
			}
		}

		if strings.TrimSpace(txt) == "" && len(partes) != 0 {
			t.Fatal("texto vazio devia dar nil")
		}
	})
}

func FuzzCasaPadraoDominio(f *testing.F) {
	f.Add("*.google.com", "mail.google.com")
	f.Add("ads", "ads.example.com")
	f.Fuzz(func(t *testing.T, pat, dom string) {
		a := casaPadraoDominio(pat, dom)

		b := casaPadraoDominio(pat, dom)
		if a != b {
			t.Fatal("nao deterministico (cache?)")
		}
	})
}

func FuzzHostPreBloqueado(f *testing.F) {
	f.Add("https://ads.doubleclick.net/x")
	f.Add("https://www.google.com/")
	f.Fuzz(func(_ *testing.T, uri string) {
		_ = hostPreBloqueado(uri)
	})
}

// TestHandlersComecamComVtbl garante o layout COM: vtbl em zero e
// refCounted em refsOffset. Se inverter, o loader le lixo como vtbl e
// faulta no primeiro AddRef (foi o crash do boot no runtime 157).
func TestHandlersComecamComVtbl(t *testing.T) {
	t.Parallel()

	for _, tipo := range tiposHandlersCom() {
		checarHandlerCom(t, tipo)
	}
}

// tiposHandlersCom lista os handlers COM com vtbl propria.
func tiposHandlersCom() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[envCompletedHandler](),
		reflect.TypeFor[controllerCompletedHandler](),
		reflect.TypeFor[addExtCompletedHandler](),
		reflect.TypeFor[getExtCompletedHandler](),
		reflect.TypeFor[removeExtCompletedHandler](),
		reflect.TypeFor[enableExtCompletedHandler](),
		reflect.TypeFor[schemeReg](),
		reflect.TypeFor[cdpCompletedHandler](),
		reflect.TypeFor[srcChangedHandler](),
		reflect.TypeFor[navStartingHandler](),
		reflect.TypeFor[navCompletedHandler](),
		reflect.TypeFor[titleChangedHandler](),
		reflect.TypeFor[newWindowHandler](),
		reflect.TypeFor[webMessageHandler](),
		reflect.TypeFor[accelKeyHandler](),
		reflect.TypeFor[clearBrowsingDataHandler](),
		reflect.TypeFor[findStartHandler](),
		reflect.TypeFor[webResourceRequestedHandler](),
		reflect.TypeFor[contextMenuRequestedHandler](),
		reflect.TypeFor[customItemSelectedHandler](),
		reflect.TypeFor[downloadStartingHandler](),
		reflect.TypeFor[downloadBytesHandler](),
		reflect.TypeFor[downloadStateHandler](),
		reflect.TypeFor[isMutedChangedHandler](),
		reflect.TypeFor[isPlayingAudioChangedHandler](),
		reflect.TypeFor[permissionRequestedHandler](),
		reflect.TypeFor[processFailedHandler](),
		reflect.TypeFor[gotFocusHandler](),
		reflect.TypeFor[evalCompletedHandler](),
		reflect.TypeFor[trySuspendHandler](),
	}
}

// checarHandlerCom valida os deslocamentos de um handler COM.
func checarHandlerCom(t *testing.T, tipo reflect.Type) {
	t.Helper()

	campoVtbl, existe := tipo.FieldByName("vtbl")
	if !existe {
		t.Errorf("%s: sem campo vtbl", tipo.Name())

		return
	}

	if campoVtbl.Offset != 0 {
		t.Errorf("%s: vtbl em %d, quero zero", tipo.Name(), campoVtbl.Offset)
	}

	campoRefs, existe := tipo.FieldByName("refCounted")
	if !existe {
		t.Errorf("%s: sem campo refCounted", tipo.Name())

		return
	}

	if campoRefs.Offset != refsOffset {
		t.Errorf("%s: refCounted em %d, quero %d", tipo.Name(), campoRefs.Offset, refsOffset)
	}
}

func medirClipFalso(porSimbolo int32) medidaClip {
	return func(candidato string) int32 {
		return int32(len([]rune(candidato))) * porSimbolo
	}
}

func TestQuebrarPalavraLongaNaoVaza(t *testing.T) {
	t.Parallel()

	const (
		porSimbolo = 10
		larguraMax = 95
		maxLinhas  = 2
	)

	medir := medirClipFalso(porSimbolo)

	casos := []struct {
		nome  string
		texto string
	}{
		{"palavra unica gigante", strings.Repeat("x", 300)},
		{"url sem espaco", "https://" + strings.Repeat("a", 280) + "/fim"},
		{"misto com espacos", "olha " + strings.Repeat("z", 200) + " fim"},
		{"emoji continuo", strings.Repeat("🎉", 120)},
	}

	for _, caso := range casos {
		linhas := quebrarComMedida(caso.texto, larguraMax, maxLinhas, medir)
		if len(linhas) == 0 {
			t.Errorf("%s: nenhuma linha", caso.nome)

			continue
		}

		if len(linhas) > maxLinhas {
			t.Errorf("%s: %d linhas passam do maximo %d", caso.nome, len(linhas), maxLinhas)
		}

		for _, linha := range linhas {
			if medir(linha) > larguraMax {
				t.Errorf("%s: linha vaza (%d > %d)", caso.nome, medir(linha), larguraMax)
			}
		}
	}
}

func TestQuebrarMarcaCorteReticencias(t *testing.T) {
	t.Parallel()

	const (
		porSimbolo = 10
		larguraMax = 95
	)

	medir := medirClipFalso(porSimbolo)

	linhas := quebrarComMedida(strings.Repeat("x", 300), larguraMax, 2, medir)
	if len(linhas) != 2 {
		t.Fatalf("quero 2 linhas, deu %d", len(linhas))
	}

	ultima := linhas[len(linhas)-1]
	if !strings.HasSuffix(ultima, "…") {
		t.Error("corte sem reticencias na ultima linha")
	}

	if medir(ultima) > larguraMax {
		t.Errorf("ultima linha com reticencias vaza: %d > %d", medir(ultima), larguraMax)
	}

	curta := quebrarComMedida("olha mundo", larguraMax, 2, medir)
	if len(curta) == 0 {
		t.Fatal("texto curto sumiu")
	}

	if strings.HasSuffix(curta[len(curta)-1], "…") {
		t.Error("texto curto nao devia ganhar reticencias")
	}
}

func TestRetsClipNaoSobrepoemBotoes(t *testing.T) {
	t.Parallel()

	texto := rectT{Left: 14, Top: 13, Right: 200, Bottom: 53}
	check := rectT{Left: 210, Top: 20, Right: 236, Bottom: 46}
	pino := rectT{Left: 238, Top: 20, Right: 264, Bottom: 46}
	apagar := rectT{Left: 266, Top: 20, Right: 292, Bottom: 46}

	for _, botao := range []rectT{check, pino, apagar} {
		if retsSobrepoemClip(texto, botao) {
			t.Errorf("texto cobre botao %+v", botao)
		}
	}

	if !retsSobrepoemClip(texto, texto) {
		t.Error("iguais deviam sobrepor")
	}

	if retsSobrepoemClip(rectT{}, check) {
		t.Error("retangulo vazio nao sobrepoe")
	}

	if retsSobrepoemClip(texto, rectT{Left: 500, Top: 500, Right: 600, Bottom: 600}) {
		t.Error("retangulo longe nao sobrepoe")
	}
}

func FuzzQuebrarComMedida(f *testing.F) {
	f.Add("palavra " + strings.Repeat("x", 100))
	f.Add("https://" + strings.Repeat("a", 200))
	f.Fuzz(func(t *testing.T, texto string) {
		const (
			porSimbolo = 10
			larguraMax = 95
			maxLinhas  = 2
		)

		medir := medirClipFalso(porSimbolo)

		linhas := quebrarComMedida(texto, larguraMax, maxLinhas, medir)
		if len(linhas) > maxLinhas {
			t.Fatalf("passou do maximo: %d", len(linhas))
		}

		for _, linha := range linhas {
			if medir(linha) > larguraMax {
				t.Fatalf("linha vaza: %q", linha)
			}
		}
	})
}

func TestCompararVersoesExt(t *testing.T) {
	t.Parallel()

	const versaoBaseUpdateExt = "1.76.0"

	casos := []struct {
		nova, atual string
		quero       int
	}{
		{"1.77.0", versaoBaseUpdateExt, 1},
		{"v1.77.0", versaoBaseUpdateExt, 1},
		{versaoBaseUpdateExt, versaoBaseUpdateExt, 0},
		{"1.75.0", versaoBaseUpdateExt, -1},
		{"1.76.1", versaoBaseUpdateExt, 1},
		{"2.0", "1.99.9", 1},
		{"1.76", "1.76.0", -1},
	}

	for _, c := range casos {
		if got := compararVersoesExt(c.nova, c.atual); got != c.quero {
			t.Errorf("%s x %s = %d, quero %d", c.nova, c.atual, got, c.quero)
		}
	}
}

func TestEscolherAtivoUpdateExt(t *testing.T) {
	t.Parallel()

	const urlExemploUpdateExt = "http://exemplo/chromium"

	ativos := []ativoReleaseExt{
		{Nome: "uBlock0.firefox.xpi", Link: "http://exemplo/firefox"},
		{Nome: "uBlock0.chromium.zip", Link: urlExemploUpdateExt},
	}

	nome, url := escolherAtivoUpdateExt(ativos, "uBlock0.chromium.zip")
	if nome != "uBlock0.chromium.zip" || url != urlExemploUpdateExt {
		t.Errorf("exato nao achado: %q %q", nome, url)
	}

	nome, url = escolherAtivoUpdateExt(ativos, "")
	if url != urlExemploUpdateExt {
		t.Errorf("fallback nao pegou o zip: %q %q", nome, url)
	}

	nome, url = escolherAtivoUpdateExt(nil, "uBlock0.chromium.zip")
	if nome != "" || url != "" {
		t.Errorf("vazio devia dar vazio: %q %q", nome, url)
	}
}

func TestFonteConhecidaExt(t *testing.T) {
	t.Parallel()

	repo, ativo, ok := fonteConhecidaExt("uBO")
	if !ok || repo == "" || ativo == "" {
		t.Errorf("ubo devia ter fonte: %q %q %v", repo, ativo, ok)
	}

	if _, _, ok := fonteConhecidaExt("inexistente"); ok {
		t.Error("desconhecida nao devia ter fonte")
	}
}

func TestPacoteUpdateExtComPasta(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pacote := filepath.Join(dir, "pacote.zip")
	criarPacoteUpdateTeste(t, pacote)

	if err := validarPacoteUpdateExt(pacote, "9.9.9"); err != nil {
		t.Fatalf("valida: %v", err)
	}

	if err := validarPacoteUpdateExt(pacote, "1.0.0"); err == nil {
		t.Fatal("versao errada devia falhar")
	}

	destino := filepath.Join(dir, "ext")
	if err := extrairPacoteUpdateExt(pacote, destino); err != nil {
		t.Fatalf("extrai: %v", err)
	}

	for _, rel := range []string{"manifest.json", filepath.Join("js", "fundo.js")} {
		if _, err := os.Stat(filepath.Join(destino, rel)); err != nil {
			t.Errorf("sem %s extraido: %v", rel, err)
		}
	}
}

// criarPacoteUpdateTeste monta um zip com pasta raiz unica para o teste.
func criarPacoteUpdateTeste(t *testing.T, pacote string) {
	t.Helper()

	saida, err := os.Create(pacote) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}

	compactado := zip.NewWriter(saida)
	for nome, conteudo := range map[string]string{
		"uBlock0.chromium/manifest.json": `{"version":"9.9.9"}`,
		"uBlock0.chromium/js/fundo.js":   "x",
	} {
		fragmento, err := compactado.Create(nome)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.WriteString(fragmento, conteudo); err != nil {
			t.Fatal(err)
		}
	}

	if err := compactado.Close(); err != nil {
		t.Fatal(err)
	}

	if err := saida.Close(); err != nil {
		t.Fatal(err)
	}
}
