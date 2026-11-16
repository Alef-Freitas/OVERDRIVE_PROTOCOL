//go:build windows

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseExtensaoValida(t *testing.T) {
	t.Parallel()

	casos := []struct {
		entrada string
		base    string
		ok      bool
	}{
		{"uBO", idUbo, true},
		{"UBO", idUbo, true},
		{"extensions/ubo", idUbo, true},
		{`extensions\ubo`, idUbo, true},
		{"", "", false},
		{".", "", false},
		{"..", "", false},
		{"../ubo", "ubo", true},
		{"extensions/../ubo", "ubo", true},
	}

	for _, c := range casos {
		base, ok := baseExtensaoValida(c.entrada)
		if ok != c.ok || base != c.base {
			t.Errorf("base(%q) = (%q,%v), quero (%q,%v)", c.entrada, base, ok, c.base, c.ok)
		}
	}
}

func TestBaseListaExtensao(t *testing.T) {
	t.Parallel()

	if got := baseListaExtensao(`extensions\uBO`); got != idUbo {
		t.Errorf("lista devia normalizar, deu %q", got)
	}

	if got := baseListaExtensao("extensions/UBO"); got != idUbo {
		t.Errorf("barra devia normalizar, deu %q", got)
	}

	if got := baseListaExtensao(".."); got != "" {
		t.Errorf("traversal devia dar vazio, deu %q", got)
	}
}

func TestLimparVersaoExt(t *testing.T) {
	t.Parallel()

	if got := limparVersaoExt("  v1.76.2 "); got != "1.76.2" {
		t.Errorf("limpeza errada: %q", got)
	}

	if got := limparVersaoExt("v"); got != "" {
		t.Errorf("v sozinho devia zerar, deu %q", got)
	}
}

func TestCompararVersoesPreLancamento(t *testing.T) {
	t.Parallel()

	const versaoBetaExt = "1.0-beta"

	casos := []struct {
		nova, atual string
		quero       int
	}{
		{"1.0", versaoBetaExt, 1},
		{versaoBetaExt, "1.0", -1},
		{versaoBetaExt, "1.0-alpha", 1},
		{"2.0", "10.0", -1},
		{"", "", 0},
	}

	for _, c := range casos {
		if got := compararVersoesExt(c.nova, c.atual); got != c.quero {
			t.Errorf("comparar(%q,%q) = %d, quero %d", c.nova, c.atual, got, c.quero)
		}
	}
}

func TestFonteConhecidaExtCase(t *testing.T) {
	t.Parallel()

	repo, ativo, ok := fonteConhecidaExt("ubo")
	if !ok || repo == "" || ativo == "" {
		t.Errorf("ubo devia ter fonte: %q %q %v", repo, ativo, ok)
	}

	if _, _, ok := fonteConhecidaExt("outra"); ok {
		t.Error("desconhecida nao devia mapear")
	}

	if _, _, ok := fonteConhecidaExt(" UBO "); ok != true {
		t.Error("espaco e case deviam normalizar")
	}
}

func TestEscolherAtivoUpdateExtFallbacks(t *testing.T) {
	t.Parallel()

	const (
		linkFirefoxExt  = "http://f"
		linkChromiumExt = "http://c"
		linkPrimeiroExt = "http://p"
		linkUnicoExt    = "http://a"
	)

	ativos := []ativoReleaseExt{
		{Nome: "plug.firefox.zip", Link: linkFirefoxExt},
		{Nome: "plug.chromium.zip", Link: linkChromiumExt},
		{Nome: "primeiro.zip", Link: linkPrimeiroExt},
	}

	nome, url := escolherAtivoUpdateExt(ativos, "plug.chromium.zip")
	if nome != "plug.chromium.zip" || url != linkChromiumExt {
		t.Errorf("exato devia ganhar: %q %q", nome, url)
	}

	nome, url = escolherAtivoUpdateExt(ativos, "inexistente.zip")
	if url != linkChromiumExt {
		t.Errorf("chromium devia ser fallback, deu %q %q", nome, url)
	}

	nome, url = escolherAtivoUpdateExt([]ativoReleaseExt{{Nome: "a.zip", Link: linkUnicoExt}}, "x.zip")
	if url != linkUnicoExt {
		t.Errorf("zip unico devia servir, deu %q %q", nome, url)
	}

	if nome, url := escolherAtivoUpdateExt([]ativoReleaseExt{{Nome: "a.tar.gz", Link: ""}}, ""); nome != "" || url != "" {
		t.Error("sem zip devia dar vazio")
	}

	if nome, url := escolherAtivoUpdateExt(nil, ""); nome != "" || url != "" {
		t.Error("vazio devia dar vazio")
	}
}

func TestSegmentoRaizUpdateExt(t *testing.T) {
	t.Parallel()

	criar := func(nome string) *zip.File {
		f := new(zip.File)
		f.Name = nome

		return f
	}

	if seg, ok := segmentoRaizUpdateExt(criar(nomeManifestoUpdateExt)); ok || seg != "" {
		t.Error("raiz devia ser ignorada sem pasta unica")
	}

	if seg, ok := segmentoRaizUpdateExt(criar("uBO/" + nomeManifestoUpdateExt)); !ok || seg != "uBO" {
		t.Errorf("pasta unica devia passar, deu %q %v", seg, ok)
	}

	if _, ok := segmentoRaizUpdateExt(criar(".git/config")); ok {
		t.Error("oculta devia reprovar")
	}

	if seg, ok := segmentoRaizUpdateExt(criar("a/b")); !ok || seg != "a" {
		t.Errorf("entrada unica com pasta devia passar, deu %q %v", seg, ok)
	}
}

func montarZipTeste(t *testing.T, arquivos map[string]string) string {
	t.Helper()

	pacote := filepath.Join(t.TempDir(), "pacote.zip")

	saida, err := os.Create(pacote) // #nosec G304
	if err != nil {
		t.Fatal(err)
	}

	escritor := zip.NewWriter(saida)

	for nome, conteudo := range arquivos {
		parte, err := escritor.Create(nome)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := parte.Write([]byte(conteudo)); err != nil {
			t.Fatal(err)
		}
	}

	if err := escritor.Close(); err != nil {
		t.Fatal(err)
	}

	if err := saida.Close(); err != nil {
		t.Fatal(err)
	}

	return pacote
}

func TestRaizPacoteUpdateExt(t *testing.T) {
	t.Parallel()

	raiz := montarZipTeste(t, map[string]string{nomeManifestoUpdateExt: `{"version":"1"}`})

	leitor, err := zip.OpenReader(raiz)
	if err != nil || leitor == nil {
		t.Fatalf("abrir pacote: %v", err)
	}

	defer func() { _ = leitor.Close() }()

	if got := raizPacoteUpdateExt(leitor); got != "" {
		t.Errorf("raiz devia dar vazio, deu %q", got)
	}

	embrulhado := montarZipTeste(t, map[string]string{"uBO/" + nomeManifestoUpdateExt: `{"version":"1"}`})

	leitor2, err := zip.OpenReader(embrulhado)
	if err != nil || leitor2 == nil {
		t.Fatalf("abrir embrulho: %v", err)
	}

	defer func() { _ = leitor2.Close() }()

	if got := raizPacoteUpdateExt(leitor2); got != "uBO/" {
		t.Errorf("embrulho unico devia dar prefixo, deu %q", got)
	}

	misto := montarZipTeste(t, map[string]string{"a/" + nomeManifestoUpdateExt: "{}", "b/x.js": "{}"})

	leitor3, err := zip.OpenReader(misto)
	if err != nil || leitor3 == nil {
		t.Fatalf("abrir misto: %v", err)
	}

	defer func() { _ = leitor3.Close() }()

	if got := raizPacoteUpdateExt(leitor3); got != "" {
		t.Errorf("duas pastas deviam anular, deu %q", got)
	}
}

func TestValidarPacoteUpdateExt(t *testing.T) {
	t.Parallel()

	bom := montarZipTeste(t, map[string]string{nomeManifestoUpdateExt: `{"version":"1.77"}`})
	if err := validarPacoteUpdateExt(bom, "1.77"); err != nil {
		t.Errorf("pacote bom reprovou: %v", err)
	}

	if err := validarPacoteUpdateExt(bom, "1.78"); err == nil {
		t.Error("versao trocada devia reprovar")
	}

	semManifesto := montarZipTeste(t, map[string]string{"x.js": "oi"})
	if err := validarPacoteUpdateExt(semManifesto, "1.77"); err == nil {
		t.Error("sem manifesto devia reprovar")
	}

	quebrado := montarZipTeste(t, map[string]string{nomeManifestoUpdateExt: "{"})
	if err := validarPacoteUpdateExt(quebrado, "1.77"); err == nil {
		t.Error("json quebrado devia reprovar")
	}

	embrulhado := montarZipTeste(t, map[string]string{"uBO/" + nomeManifestoUpdateExt: `{"version":"2.0"}`})
	if err := validarPacoteUpdateExt(embrulhado, "2.0"); err != nil {
		t.Errorf("embrulho unico devia passar: %v", err)
	}
}

func TestExtrairPacoteUpdateExt(t *testing.T) {
	t.Parallel()

	pacote := montarZipTeste(t, map[string]string{
		nomeManifestoUpdateExt: `{"version":"3.0"}`,
		"sub/x.js":             "conteudo",
	})
	destino := filepath.Join(t.TempDir(), "dest")

	if err := extrairPacoteUpdateExt(pacote, destino); err != nil {
		t.Fatalf("extracao falhou: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destino, nomeManifestoUpdateExt)); err != nil {
		t.Error("manifesto devia existir no destino")
	}

	if _, err := os.Stat(filepath.Join(destino, "sub", "x.js")); err != nil {
		t.Error("subarquivo devia existir no destino")
	}

	embrulhado := montarZipTeste(t, map[string]string{"uBO/" + nomeManifestoUpdateExt: `{"version":"3.0"}`})
	destino2 := filepath.Join(t.TempDir(), "dest2")

	if err := extrairPacoteUpdateExt(embrulhado, destino2); err != nil {
		t.Fatalf("embrulho falhou: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destino2, nomeManifestoUpdateExt)); err != nil {
		t.Error("prefixo unico devia ser removido")
	}
}

func TestExtrairPacoteZipSlip(t *testing.T) {
	t.Parallel()

	pacote := montarZipTeste(t, map[string]string{"../fora.txt": "mal"})
	if err := extrairPacoteUpdateExt(pacote, filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Error("zip-slip devia reprovar")
	}
}

func TestNomePacoteUpdateExt(t *testing.T) {
	t.Parallel()

	if got := nomePacoteUpdateExt("uBO"); got != "update-ubo.zip" {
		t.Errorf("nome devia ser por base, deu %q", got)
	}

	if got := nomePacoteUpdateExt(".."); got == "" || !strings.HasSuffix(got, ".zip") {
		t.Errorf("invalido devia cair no generico, deu %q", got)
	}

	if nomePacoteUpdateExt("uBO") == nomePacoteUpdateExt("outra") {
		t.Error("bases diferentes deviam dar arquivos diferentes (sem corrida)")
	}
}

func TestNumeroInicialVersao(t *testing.T) {
	t.Parallel()

	numero, resto := numeroInicialVersao("76beta")
	if numero != 76 || resto != "beta" {
		t.Errorf("prefixo errado: %d %q", numero, resto)
	}

	numero, resto = numeroInicialVersao("abc")
	if numero != 0 || resto != "abc" {
		t.Errorf("sem digito devia zerar: %d %q", numero, resto)
	}
}

func TestLerJSONPequenoExt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pequeno := filepath.Join(dir, "update.json")

	if err := os.WriteFile(pequeno, []byte(`{"repo":"a/b"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := lerJSONPequenoExt(pequeno); !ok {
		t.Error("json pequeno devia passar")
	}

	grande := filepath.Join(dir, "grande.json")
	f, _ := os.Create(grande) // #nosec G304
	_ = f.Truncate(300 << 10)
	_ = f.Close()

	if _, ok := lerJSONPequenoExt(grande); ok {
		t.Error("json gigante devia reprovar")
	}

	if _, ok := lerJSONPequenoExt(filepath.Join(dir, "falta.json")); ok {
		t.Error("ausente devia reprovar")
	}
}
