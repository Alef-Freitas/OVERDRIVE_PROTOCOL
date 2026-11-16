//go:build windows

package main

import (
	"strings"
	"sync"
	"testing"
)

// mutexTestesClip serializa os testes do painel: o clipboard usa
// globais de UI e o portao exige t.Parallel em todo teste.
//
//nolint:gochecknoglobals // Singleton de teste: sem ele os paralelos disputam clipHist/clipUI (data race real).
var mutexTestesClip sync.Mutex

const (
	textoRepetidoClip = "repetido"
	textoMesmoClip    = "mesmo"
)

// limparEstadoClipTeste zera o historico e o painel para testes isolados.
func limparEstadoClipTeste() {
	var zerado clipPainel

	clipHist = nil
	clipUI = zerado
}

func TestClipAdicionarLimiteItens(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	for i := range clipMaxItens + 10 {
		clipAdicionar(clipItem{tipo: clipTexto, texto: "texto-" + strings.Repeat("x", i)})
	}

	if len(clipHist) != clipMaxItens {
		t.Fatalf("quero %d itens, deu %d", clipMaxItens, len(clipHist))
	}

	if !strings.HasPrefix(clipHist[0].texto, "texto-") {
		t.Error("mais novo devia estar no topo")
	}

	for _, item := range clipHist {
		if item.texto == "texto-" {
			t.Error("mais antigo devia ter saido")
		}
	}
}

func TestClipAdicionarDedup(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipAdicionar(clipItem{tipo: clipTexto, texto: textoRepetidoClip})
	clipAdicionar(clipItem{tipo: clipTexto, texto: "outro"})
	clipAdicionar(clipItem{tipo: clipTexto, texto: textoRepetidoClip})

	if len(clipHist) != 2 {
		t.Fatalf("duplicado devia juntar, deu %d", len(clipHist))
	}

	if clipHist[0].texto != textoRepetidoClip {
		t.Error("duplicado devia voltar ao topo")
	}
}

func TestClipAdicionarDedupMantemFixo(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipAdicionar(clipItem{tipo: clipTexto, texto: "fixado"})

	clipHist[0].fixo = true

	clipAdicionar(clipItem{tipo: clipTexto, texto: "outro"})
	clipAdicionar(clipItem{tipo: clipTexto, texto: "fixado"})

	if len(clipHist) != 2 {
		t.Fatalf("duplicado devia juntar, deu %d", len(clipHist))
	}

	if !clipHist[0].fixo {
		t.Error("fixo devia sobreviver a duplicata")
	}
}

func TestClipAdicionarLimiteTamanho(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipAdicionar(clipItem{tipo: clipTexto, texto: strings.Repeat("g", clipMaxTextoItem+1)})

	if len(clipHist) != 0 {
		t.Error("texto acima do limite devia ser ignorado")
	}

	clipAdicionar(clipItem{tipo: clipImagem, dib: make([]byte, clipMaxBytesImagem+1)})

	if len(clipHist) != 0 {
		t.Error("imagem acima do limite devia ser ignorada")
	}

	clipAdicionar(clipItem{tipo: clipTexto, texto: strings.Repeat("o", clipMaxTextoItem)})

	if len(clipHist) != 1 {
		t.Error("texto no limite devia entrar")
	}
}

func TestClipPodarFixoSobrevive(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipAdicionar(clipItem{tipo: clipTexto, texto: "antigo fixo"})

	clipHist[0].fixo = true

	for i := range clipMaxItens {
		clipAdicionar(clipItem{tipo: clipTexto, texto: strings.Repeat("n", i+1)})
	}

	achou := false

	for _, item := range clipHist {
		if item.texto == "antigo fixo" {
			achou = true
		}
	}

	if !achou {
		t.Error("fixo nao devia ser podado")
	}

	if len(clipHist) != clipMaxItens {
		t.Errorf("quero %d itens, deu %d", clipMaxItens, len(clipHist))
	}
}

func TestClipPodarImagens(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	for i := range clipMaxImagens + 1 {
		clipAdicionar(clipItem{tipo: clipImagem, dib: []byte{byte(i), 1, 2, 3}})
	}

	if got := len(clipHist); got != clipMaxImagens {
		t.Errorf("quero %d imagens, deu %d", clipMaxImagens, got)
	}

	if clipHist[0].dib[0] != byte(clipMaxImagens) {
		t.Error("mais nova devia estar no topo")
	}
}

func TestClipPodarBytesTexto(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	for range 5 {
		clipAdicionar(clipItem{tipo: clipTexto, texto: strings.Repeat("b", clipMaxBytesTexto/4)})
	}

	if len(clipHist) == 0 || len(clipHist) > 4 {
		t.Errorf("bytes acima do teto deviam podar, deu %d", len(clipHist))
	}
}

func TestClipOrdenarFixoPrimeiro(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipAdicionar(clipItem{tipo: clipTexto, texto: "primeiro"})
	clipAdicionar(clipItem{tipo: clipTexto, texto: "segundo"})

	clipHist[1].fixo = true

	clipOrdenar()

	if !clipHist[0].fixo || clipHist[0].texto != "primeiro" {
		t.Error("fixo devia ir para o topo")
	}
}

func TestClipSomarUso(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipHist = []clipItem{
		{tipo: clipTexto, texto: "abc"},
		{tipo: clipImagem, dib: []byte{1, 2}},
	}

	imgs, bytesImg, bytesTxt := clipSomarUso()

	if imgs != 1 || bytesImg != 2 || bytesTxt != 3 {
		t.Errorf("soma errada: %d %d %d", imgs, bytesImg, bytesTxt)
	}

	if clipPrecisaPodar(0, 0, 0) {
		t.Error("vazio nao precisa podar")
	}

	if !clipPrecisaPodar(clipMaxImagens+1, 0, 0) {
		t.Error("acima do teto de imagens precisa podar")
	}
}

func TestClipEnquadrarRolagem(t *testing.T) {
	t.Parallel()

	casos := []struct {
		rolagem, total, visivel, quero int32
	}{
		{0, 100, 500, 0},
		{-10, 100, 500, 0},
		{1000, 2000, 500, 1000},
		{100, 2000, 500, 100},
		{0, 0, 0, 0},
		{5, 10, 10, 0},
		{7, 10, 0, 7},
	}

	for _, c := range casos {
		if got := clipEnquadrarRolagem(c.rolagem, c.total, c.visivel); got != c.quero {
			t.Errorf("rolagem(%d,%d,%d) = %d, quero %d", c.rolagem, c.total, c.visivel, got, c.quero)
		}
	}
}

func TestClipDentroBotao(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	area := rectT{Left: 10, Top: 10, Right: 110, Bottom: 40}
	if clipDentroBotao(50, 20, area) {
		t.Error("historico vazio nunca acerta")
	}

	clipHist = []clipItem{{tipo: clipTexto, texto: "x"}}

	if !clipDentroBotao(10, 10, area) {
		t.Error("borda inicial devia acertar")
	}

	if !clipDentroBotao(109, 39, area) {
		t.Error("borda final devia acertar")
	}

	if clipDentroBotao(110, 20, area) {
		t.Error("fora da direita nao devia acertar")
	}

	if clipDentroBotao(50, 40, area) {
		t.Error("fora de baixo nao devia acertar")
	}
}

func TestClipLinhaEm(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	if clipLinhaEm(20) != -1 {
		t.Error("sem linhas devia dar -1")
	}

	var vazio rectT

	clipUI.rows = []clipLinha{
		{idx: 0, r: rectT{Left: 0, Top: 10, Right: 100, Bottom: 76}, check: vazio, pin: vazio, del: vazio},
		{idx: 1, r: rectT{Left: 0, Top: 82, Right: 100, Bottom: 148}, check: vazio, pin: vazio, del: vazio},
	}

	if got := clipLinhaEm(9); got != -1 {
		t.Errorf("acima devia dar -1, deu %d", got)
	}

	if got := clipLinhaEm(10); got != 0 {
		t.Errorf("inicio devia dar 0, deu %d", got)
	}

	if got := clipLinhaEm(75); got != 0 {
		t.Errorf("meio devia dar 0, deu %d", got)
	}

	if got := clipLinhaEm(76); got != -1 {
		t.Error("fim e exclusivo")
	}

	if got := clipLinhaEm(79); got != -1 {
		t.Error("vao entre linhas devia dar -1")
	}

	if got := clipLinhaEm(82); got != 1 {
		t.Errorf("segunda linha devia dar 1, deu %d", got)
	}
}

func TestClipApararMarcados(t *testing.T) {
	t.Parallel()
	mutexTestesClip.Lock()

	defer mutexTestesClip.Unlock()

	limparEstadoClipTeste()

	clipHist = []clipItem{{tipo: clipTexto, texto: "vivo"}}
	clipHist[0].chave = clipHash(clipHist[0])
	clipUI.marcados = map[uint64]bool{clipHist[0].chave: true, 999: true}

	clipApararMarcados()

	if len(clipUI.marcados) != 1 || !clipUI.marcados[clipHist[0].chave] {
		t.Error("chave morta devia sair e viva ficar")
	}
}

func TestClipTextoParaIA(t *testing.T) {
	t.Parallel()

	if got := clipTextoParaIA(clipItem{tipo: clipTexto, texto: "oi"}); got != "oi" {
		t.Errorf("texto passa cru, deu %q", got)
	}

	got := clipTextoParaIA(clipItem{tipo: clipArquivos, texto: "  a\n"})
	if got != "Arquivos:\na" {
		t.Errorf("arquivo ganha prefixo e corta, deu %q", got)
	}
}

func TestPreviaTextoClip(t *testing.T) {
	t.Parallel()

	var semApp *app

	if got := semApp.previaTextoClip(clipItem{tipo: clipTexto, texto: "cru"}); got != "cru" {
		t.Errorf("texto passa cru, deu %q", got)
	}

	unico := semApp.previaTextoClip(clipItem{tipo: clipArquivos, texto: `C:\pasta\nome.png`})
	if unico != "nome.png" {
		t.Errorf("arquivo unico mostra o nome, deu %q", unico)
	}

	muitos := semApp.previaTextoClip(clipItem{tipo: clipArquivos, texto: "C:\\pasta\\a.png\nD:\\outra\\b.png"})
	if !strings.Contains(muitos, "+1 arquivo") {
		t.Errorf("varios mostram contador, deu %q", muitos)
	}
}

func TestClipHashConsistencia(t *testing.T) {
	t.Parallel()

	base := clipItem{tipo: clipTexto, texto: textoMesmoClip}
	outro := clipItem{tipo: clipArquivos, texto: textoMesmoClip}

	if clipHash(base) != clipHash(clipItem{tipo: clipTexto, texto: textoMesmoClip}) {
		t.Error("mesmo conteudo devia dar mesma chave")
	}

	if clipHash(base) == clipHash(outro) {
		t.Error("tipo diferente devia dar chave diferente")
	}
}
