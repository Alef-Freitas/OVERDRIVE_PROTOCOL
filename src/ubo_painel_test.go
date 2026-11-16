//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	versaoUboTeste      = "1.76.2"
	nomeBloqueadorTeste = "Bloqueador"
)

func TestEhDuplicataUbo(t *testing.T) {
	t.Parallel()

	if !ehDuplicataUbo("id-antigo", "id-novo", "uBlock Origin") {
		t.Error("id trocado com mesmo nome devia ser duplicata")
	}

	if ehDuplicataUbo("id-novo", "id-novo", "uBlock Origin") {
		t.Error("mesmo id nao e duplicata")
	}

	if ehDuplicataUbo("", "id-novo", "uBlock Origin") {
		t.Error("id vazio nao e duplicata")
	}

	if ehDuplicataUbo("id-antigo", "id-novo", "Outra Coisa") {
		t.Error("outro nome nao e duplicata do ubo")
	}
}

func TestPopupDoManifesto(t *testing.T) {
	t.Parallel()

	ler := func(js string) extManifest {
		var manifesto extManifest
		if err := json.Unmarshal([]byte(js), &manifesto); err != nil {
			t.Fatalf("json de teste invalido: %v", err)
		}

		return manifesto
	}

	comAction := ler(`{"action":{"default_popup":"popup-fenix.html"}}`)
	if got := popupDoManifesto(comAction); got != "popup-fenix.html" {
		t.Errorf("action devia ganhar, deu %q", got)
	}

	comLegado := ler(`{"browser_action":{"default_popup":"velho.html"}}`)
	if got := popupDoManifesto(comLegado); got != "velho.html" {
		t.Errorf("browser_action devia servir, deu %q", got)
	}

	ambos := ler(`{"action":{"default_popup":"novo.html"},"browser_action":{"default_popup":"velho.html"}}`)
	if got := popupDoManifesto(ambos); got != "novo.html" {
		t.Errorf("action tem prioridade, deu %q", got)
	}

	if got := popupDoManifesto(ler(`{}`)); got != "" {
		t.Errorf("sem popup devia dar vazio, deu %q", got)
	}
}

func TestPopupDoManifestoExtensao(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if got := popupDoManifestoExtensao(dir); got != "" {
		t.Errorf("sem manifesto devia dar vazio, deu %q", got)
	}

	conteudo := `{"manifest_version":3,"name":"x","version":"1","action":{"default_popup":"painel.html"}}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := popupDoManifestoExtensao(dir); got != "painel.html" {
		t.Errorf("popup do disco devia ler, deu %q", got)
	}
}

func TestDirContidoExe(t *testing.T) {
	t.Parallel()

	if _, ok := dirContidoExe("extensions\\" + idUbo); !ok {
		t.Error("pasta interna devia passar")
	}

	if _, ok := dirContidoExe("extensions\\..\\" + idUbo); !ok {
		t.Error("caminho normalizado dentro devia passar")
	}

	if _, ok := dirContidoExe(`..\fora`); ok {
		t.Error("traversal devia reprovar")
	}

	if _, ok := dirContidoExe(`..\..\windows`); ok {
		t.Error("fuga da raiz devia reprovar")
	}
}

func TestNormalizarDestinoPopup(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{}

	if _, _, _, ok := aplicativo.normalizarDestinoPopup("", "p.html", "x"); ok {
		t.Error("id vazio devia reprovar")
	}

	id, caminho, nome, valido := aplicativo.normalizarDestinoPopup("id1", "", "")
	if !valido || id != "id1" || caminho != paginaPopup || nome == "" {
		t.Errorf("vazios deviam ganhar padrao, deu %q %q %q %v", id, caminho, nome, valido)
	}

	id, caminho, nome, valido = aplicativo.normalizarDestinoPopup("id1", "p.html", nomeBloqueadorTeste)
	if !valido || caminho != "p.html" || nome != nomeBloqueadorTeste {
		t.Errorf("cheio devia passar direto, deu %q %q %q %v", id, caminho, nome, valido)
	}
}

func TestEhPopupUbo(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{
		extIDs:    map[string]string{idUbo: "ID123"},
		extLoaded: map[string]bool{idUbo: true},
	}

	if !aplicativo.ehPopupUbo("ID123", "popup-fenix.html") {
		t.Error("popup oficial devia reconhecer")
	}

	if !aplicativo.ehPopupUbo("ID123", "/popup-fenix.html?x=1#ancora") {
		t.Error("query e ancora deviam ser ignoradas")
	}

	if !aplicativo.ehPopupUbo("ID123", "POPUP-FENIX.HTML") {
		t.Error("case devia ignorar")
	}

	if aplicativo.ehPopupUbo("OUTRO", "popup-fenix.html") {
		t.Error("id trocado nao e popup do ubo")
	}

	if aplicativo.ehPopupUbo("ID123", "dashboard.html") {
		t.Error("outra pagina nao e o popup")
	}

	var semApp *app
	if semApp.ehPopupUbo("ID123", "popup-fenix.html") {
		t.Error("app nil nao devia reconhecer")
	}
}

func TestDicaLinhaExtUbo(t *testing.T) {
	t.Parallel()

	desligada := itemExtUI{Nome: nomeBloqueadorTeste, Ligada: false}
	if got := dicaLinhaExt(desligada); got != "" {
		t.Errorf("desligada nao tem dica, deu %q", got)
	}

	carregando := itemExtUI{Nome: nomeBloqueadorTeste, Ligada: true, Ativa: false}
	if got := dicaLinhaExt(carregando); got == "" {
		t.Error("carregando devia avisar")
	}

	semPainel := itemExtUI{Nome: nomeBloqueadorTeste, Ligada: true, Ativa: true, TemPopup: false}
	if got := dicaLinhaExt(semPainel); got == "" {
		t.Error("sem painel devia avisar")
	}

	pronta := itemExtUI{Nome: nomeBloqueadorTeste, Ligada: true, Ativa: true, TemPopup: true}
	if got := dicaLinhaExt(pronta); got != "" {
		t.Errorf("pronta nao tem dica, deu %q", got)
	}
}

func TestPastaChuteExtensaoTravas(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{}

	if _, ok := aplicativo.pastaChuteExtensao(""); ok {
		t.Error("vazio devia reprovar")
	}

	if _, ok := aplicativo.pastaChuteExtensao("."); ok {
		t.Error("ponto devia reprovar")
	}

	if _, ok := aplicativo.pastaChuteExtensao(".."); ok {
		t.Error("traversal devia reprovar")
	}
}

func TestPainelDaExtensaoTravas(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{
		extIDs:    map[string]string{},
		extLoaded: map[string]bool{},
	}

	if _, _, _, err := aplicativo.painelDaExtensao(""); err == nil {
		t.Error("vazio devia reprovar")
	}

	if _, _, _, err := aplicativo.painelDaExtensao(".."); err == nil {
		t.Error("traversal devia reprovar")
	}

	if _, _, _, err := aplicativo.painelDaExtensao("inexistente"); err == nil {
		t.Error("nao carregada devia reprovar")
	}
}

func TestBaseUboEstados(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	carregado := &app{extLoaded: map[string]bool{idUbo: true}}
	if got := carregado.baseUbo(); got != idUbo {
		t.Errorf("carregado devia voltar ubo, deu %q", got)
	}

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	desligado := &app{st: state{}, extLoaded: map[string]bool{}}
	if got := desligado.baseUbo(); got != "" {
		t.Errorf("sem nada devia dar vazio, deu %q", got)
	}
}

func TestUboFallbackChromiumVersionado(t *testing.T) {
	t.Parallel()

	ativos := []ativoReleaseExt{
		{Nome: "uBlock0_1.76.2.firefox.signed.xpi", Link: "http://f"},
		{Nome: "uBlock0_1.76.2.chromium.zip", Link: "http://c"},
	}

	nome, url := escolherAtivoUpdateExt(ativos, "uBlock0.chromium.zip")
	if url != "http://c" || !strings.Contains(nome, "chromium") {
		t.Errorf("nome versionado devia cair no fallback chromium, deu %q %q", nome, url)
	}

	if compararVersoesExt(versaoUboTeste, "1.76.0") != 1 {
		t.Error("remoto 1.76.2 devia ser maior que o instalado 1.76.0")
	}
}

func TestNomeExibicaoExtensao(t *testing.T) {
	t.Parallel()

	m := extManifest{Name: "uBlock Origin"} //exhaustruct:ignore // Manifesto de teste: so o nome importa.
	if got := nomeExibicaoExtensao(m, ""); got != "uBlock Origin" {
		t.Errorf("nome cru devia passar, deu %q", got)
	}

	vazio := extManifest{} //exhaustruct:ignore // Manifesto de teste: vazio intencional.
	if got := nomeExibicaoExtensao(vazio, ""); got != "" {
		t.Errorf("sem nome devia dar vazio, deu %q", got)
	}
}

func TestGuardarUpdateExtCase(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os campos do cenario importam.
	aplicativo := &app{}
	aplicativo.guardarUpdateExt(strings.ToUpper(idUbo), versaoUboTeste, "http://u", "u.zip")

	if got := aplicativo.novaVersaoUpdateExt("ubo"); got != versaoUboTeste {
		t.Errorf("case devia normalizar, deu %q", got)
	}

	if got := aplicativo.novaVersaoUpdateExt(" " + strings.ToUpper(idUbo) + " "); got != versaoUboTeste {
		t.Errorf("espaco devia normalizar, deu %q", got)
	}

	var semApp *app
	if got := semApp.novaVersaoUpdateExt("ubo"); got != "" {
		t.Errorf("app nil devia dar vazio, deu %q", got)
	}
}

func TestFenixPresenteNoDiscoUbo(t *testing.T) {
	t.Parallel()

	var dir string

	orig := ""

	if pastas, err := os.ReadDir(extDir); err == nil {
		for _, d := range pastas {
			if d.IsDir() && strings.EqualFold(d.Name(), idUbo) {
				orig = d.Name()
			}
		}
	}

	if orig == "" {
		t.Skip("bloqueador fora do kit de teste")
	}

	dir = filepath.Join(extDir, orig)

	dados, err := os.ReadFile(filepath.Join(dir, "manifest.json")) // #nosec G304
	if err != nil {
		t.Fatalf("manifesto do ubo devia existir: %v", err)
	}

	if !strings.Contains(string(dados), "popup-fenix.html") {
		t.Error("manifesto instalado devia referenciar o popup-fenix")
	}
}
