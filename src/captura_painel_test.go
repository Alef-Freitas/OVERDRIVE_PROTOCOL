//go:build windows

package main

import (
	"sync"
	"testing"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
)

// mutexTestesPrint serializa os testes do recorte (global recorte).
//
//nolint:gochecknoglobals // Teste paralelo exige mutex unico (race real).
var mutexTestesPrint sync.Mutex

func guardarRecorte() recorteEstado {
	return recorte
}

func restaurarRecorte(estado recorteEstado) {
	recorte = estado
}

func TestSegundosTimerPrint(t *testing.T) {
	t.Parallel()

	casos := []struct {
		id    string
		quero int
	}{
		{"print1", 1},
		{"print2", 2},
		{"print3", 3},
		{"print4", 4},
		{"print5", 5},
		{"print0", -1},
		{"print6", -1},
		{"print", -1},
		{"print10", -1},
		{"", -1},
		{"xrint1", -1},
		{"printa", -1},
	}

	for _, c := range casos {
		if got := segundosTimerPrint(c.id); got != c.quero {
			t.Errorf("timer(%q) = %d, quero %d", c.id, got, c.quero)
		}
	}
}

func TestPixelsCapturaExcedem(t *testing.T) {
	t.Parallel()

	if !pixelsCapturaExcedem(0, 1080) {
		t.Error("largura zero devia exceder")
	}

	if !pixelsCapturaExcedem(1920, 0) {
		t.Error("altura zero devia exceder")
	}

	if !pixelsCapturaExcedem(-1, 1080) {
		t.Error("negativo devia exceder")
	}

	if pixelsCapturaExcedem(1920, 1080) {
		t.Error("full HD devia caber")
	}

	if pixelsCapturaExcedem(7680, 4320) {
		t.Error("8K limite devia caber")
	}

	if !pixelsCapturaExcedem(7681, 4320) {
		t.Error("acima de 8K devia exceder")
	}

	if !pixelsCapturaExcedem(30000, 30000) {
		t.Error("absurdo devia exceder")
	}
}

func TestRecorteClamp(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.w, recorte.h = 800, 600

	casos := []struct {
		x, y, qx, qy int32
	}{
		{100, 100, 100, 100},
		{-5, 100, 0, 100},
		{100, -5, 100, 0},
		{900, 100, 800, 100},
		{100, 900, 100, 600},
		{-10, -10, 0, 0},
		{5000, 5000, 800, 600},
	}

	for _, c := range casos {
		if gx, gy := recorteClamp(c.x, c.y); gx != c.qx || gy != c.qy {
			t.Errorf("clamp(%d,%d) = (%d,%d), quero (%d,%d)", c.x, c.y, gx, gy, c.qx, c.qy)
		}
	}
}

func TestRetanguloDePontos(t *testing.T) {
	t.Parallel()

	ret := retanguloDePontos(10, 20, 30, 40)
	if ret.Left != 10 || ret.Top != 20 || ret.Right != 30 || ret.Bottom != 40 {
		t.Errorf("ordem normal quebrou: %+v", ret)
	}

	invertido := retanguloDePontos(30, 40, 10, 20)
	if invertido != ret {
		t.Errorf("arrasto invertido devia normalizar: %+v != %+v", invertido, ret)
	}

	ponto := retanguloDePontos(5, 5, 5, 5)
	if ponto.Width() != 0 || ponto.Height() != 0 {
		t.Errorf("ponto devia ter tamanho zero: %+v", ponto)
	}
}

func TestRecorteUnir(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	unido := recorteUnir(rectT{0, 0, 10, 10}, rectT{5, 5, 20, 20})
	if unido.Left != 0 || unido.Top != 0 || unido.Right != 20 || unido.Bottom != 20 {
		t.Errorf("uniao errada: %+v", unido)
	}

	vazio := recorteUnir(rectT{1, 1, 2, 2}, rectT{})
	if vazio.Left != 1 || vazio.Right != 2 {
		t.Errorf("novo vazio devia manter antigo: %+v", vazio)
	}

	englobado := recorteUnir(rectT{0, 0, 100, 100}, rectT{10, 10, 20, 20})
	if englobado.Left != 0 || englobado.Right != 100 {
		t.Errorf("englobado devia manter externo: %+v", englobado)
	}
}

func TestRecorteExpandirPrendeTela(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.w, recorte.h = 800, 600

	expandida := recorteExpandir(rectT{100, 100, 200, 200})
	if expandida.Left > 100 || expandida.Top > 100 || expandida.Right < 200 || expandida.Bottom < 200 {
		t.Errorf("expansao devia crescer: %+v", expandida)
	}

	borda := recorteExpandir(rectT{-100, -100, 900, 700})
	if borda.Left < 0 || borda.Top < 0 || borda.Right > 800 || borda.Bottom > 600 {
		t.Errorf("expansao devia prender na tela: %+v", borda)
	}
}

func TestRecorteBotaoEm(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.botoes = [4]rectT{
		{0, 0, 100, 30},
		{108, 0, 208, 30},
		{216, 0, 316, 30},
		{324, 0, 424, 30},
	}

	if got := recorteBotaoEm(50, 15); got != 0 {
		t.Errorf("botao 0 devia acertar, deu %d", got)
	}

	if got := recorteBotaoEm(100, 15); got != -1 {
		t.Errorf("borda direita e exclusiva, deu %d", got)
	}

	if got := recorteBotaoEm(150, 15); got != 1 {
		t.Errorf("botao 1 devia acertar, deu %d", got)
	}

	if got := recorteBotaoEm(500, 15); got != -1 {
		t.Errorf("fora devia dar -1, deu %d", got)
	}

	if got := recorteBotaoEm(50, 30); got != -1 {
		t.Errorf("embaixo e exclusivo, deu %d", got)
	}
}

func TestRecortePixelsRejeita(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.w, recorte.h = 10, 10
	recorte.origPix = make([]byte, 10*10*bytesPorPixelCaptura)
	recorte.sel = rectT{}

	if _, _, _, err := recortePixels(); err == nil {
		t.Error("selecao vazia devia falhar")
	}

	recorte.sel = rectT{0, 0, 20000, 20000}

	if _, _, _, err := recortePixels(); err == nil {
		t.Error("selecao absurda devia falhar")
	}

	recorte.sel = rectT{0, 0, 4, 4}
	recorte.origPix = []byte{1, 2, 3}

	if _, _, _, err := recortePixels(); err == nil {
		t.Error("pix curto devia falhar")
	}
}

func TestRecortePixelsCopia(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.w, recorte.h = 10, 10
	recorte.origPix = make([]byte, 10*10*bytesPorPixelCaptura)

	for i := range recorte.origPix {
		recorte.origPix[i] = byte(i % 251)
	}

	recorte.sel = rectT{2, 3, 6, 5}

	buf, larg, alt, err := recortePixels()
	if err != nil {
		t.Fatalf("recorte valido falhou: %v", err)
	}

	if larg != 4 || alt != 2 {
		t.Errorf("tamanho errado: %dx%d", larg, alt)
	}

	if len(buf) != 4*2*bytesPorPixelCaptura {
		t.Errorf("buf errado: %d bytes", len(buf))
	}

	esperado := recorte.origPix[(3*10+2)*bytesPorPixelCaptura : (3*10+2)*bytesPorPixelCaptura+bytesPorPixelCaptura]

	for i := range bytesPorPixelCaptura {
		if buf[i] != esperado[i] {
			t.Errorf("pixel inicial divergiu no byte %d", i)
		}
	}
}

func TestBgraParaNRGBA(t *testing.T) {
	t.Parallel()

	img := bgraParaNRGBA([]byte{10, 20, 30, 40}, 1, 1)
	if img.NRGBAAt(0, 0).R != 30 || img.NRGBAAt(0, 0).G != 20 || img.NRGBAAt(0, 0).B != 10 {
		t.Errorf("canais BGR trocados: %v", img.NRGBAAt(0, 0))
	}

	if img.NRGBAAt(0, 0).A != 255 {
		t.Error("alfa devia ser opaco")
	}

	curto := bgraParaNRGBA([]byte{1, 2}, 4, 4)
	if curto.Bounds().Dx() != 4 || curto.Bounds().Dy() != 4 {
		t.Error("curto devia voltar imagem vazia do tamanho pedido sem panico")
	}

	vazio := bgraParaNRGBA(nil, 0, 0)
	if vazio.Bounds().Dx() != 0 {
		t.Error("zero devia voltar vazio")
	}
}

func TestDxgiGeometriaRotacao(t *testing.T) {
	t.Parallel()

	for _, r := range []int{1, 2, 3, 4} {
		if !dxgiRotacaoValida(r) {
			t.Errorf("rotacao %d devia ser valida", r)
		}
	}

	for _, r := range []int{0, 5, -1, 99} {
		if dxgiRotacaoValida(r) {
			t.Errorf("rotacao %d devia ser invalida", r)
		}
	}
}

func TestDxgiDimensaoCombinada(t *testing.T) {
	t.Parallel()

	if !dxgiDimensaoCombinada(1, 800, 600, 1920, 1080) {
		t.Error("identidade sempre combina")
	}

	if !dxgiDimensaoCombinada(3, 800, 600, 800, 600) {
		t.Error("180 com mesma dimensao devia combinar")
	}

	if dxgiDimensaoCombinada(3, 800, 600, 600, 800) {
		t.Error("180 trocado nao devia combinar")
	}

	if !dxgiDimensaoCombinada(2, 600, 800, 800, 600) {
		t.Error("90 troca largura/altura")
	}

	if !dxgiDimensaoCombinada(4, 600, 800, 800, 600) {
		t.Error("270 troca largura/altura")
	}

	if dxgiDimensaoCombinada(2, 800, 600, 800, 600) {
		t.Error("90 sem troca nao devia combinar")
	}
}

func TestDxgiOrigemPixel(t *testing.T) {
	t.Parallel()

	if x, y := dxgiOrigemPixel(1, 3, 5, 100, 200); x != 3 || y != 5 {
		t.Errorf("identidade devia passar direto: %d,%d", x, y)
	}

	if x, y := dxgiOrigemPixel(3, 0, 0, 10, 20); x != 9 || y != 19 {
		t.Errorf("180 no canto devia inverter: %d,%d", x, y)
	}

	if x, y := dxgiOrigemPixel(2, 0, 0, 10, 20); x != 19 || y != 0 {
		t.Errorf("90 no canto errado: %d,%d", x, y)
	}

	if x, y := dxgiOrigemPixel(4, 0, 0, 10, 20); x != 0 || y != 9 {
		t.Errorf("270 no canto errado: %d,%d", x, y)
	}
}

func TestDxgiGravarPixelLimites(t *testing.T) {
	t.Parallel()

	pix := make([]byte, 4*4*bytesPorPixelDxgi)
	src := []byte{11, 22, 33, 44, 55, 66, 77, 88}

	dxgiGravarPixel(pix, src, 4, 0, 0, 0, 0, 0, 0, 8)

	if pix[0] != 11 || pix[1] != 22 || pix[2] != 33 || pix[3] != 0xFF {
		t.Errorf("pixel valido nao gravou: %v", pix[:4])
	}

	antes := append([]byte(nil), pix...)

	dxgiGravarPixel(pix, src, 4, 0, 0, 0, 0, -1, 0, 8)
	dxgiGravarPixel(pix, src, 4, 0, 0, -5, -5, 0, 0, 8)
	dxgiGravarPixel(pix, src, 4, 0, 0, 0, 0, 99, 99, 8)
	dxgiGravarPixel(pix, nil, 4, 0, 0, 1, 1, 0, 0, 8)

	for i := range pix {
		if pix[i] != antes[i] {
			t.Errorf("escrita fora do limite alterou o byte %d", i)

			break
		}
	}
}

func TestDxgiUnirPixelsLimites(t *testing.T) {
	t.Parallel()

	pix := make([]byte, 4*4*bytesPorPixelDxgi)
	src := make([]byte, 4*4*bytesPorPixelDxgi)

	for i := range src {
		src[i] = byte(i % 251)
	}

	dst := foundation.RECT{Left: 0, Top: 0, Right: -5, Bottom: 4}
	dxgiUnirPixels(pix, src, 0, 0, 4, dst, 4, 4, 1, 16)

	for _, b := range pix {
		if b != 0 {
			t.Error("dimensao negativa nao devia escrever nada")

			break
		}
	}

	dstOk := foundation.RECT{Left: 0, Top: 0, Right: 2, Bottom: 2}
	dxgiUnirPixels(pix, src, 0, 0, 4, dstOk, 2, 2, 1, 8)

	acendeu := false

	for _, b := range pix {
		if b != 0 {
			acendeu = true

			break
		}
	}

	if !acendeu {
		t.Error("regiao valida devia gravar pixels")
	}
}

func TestEncerrarRecorteIdempotente(t *testing.T) {
	t.Parallel()
	mutexTestesPrint.Lock()

	defer mutexTestesPrint.Unlock()

	anterior := guardarRecorte()

	defer restaurarRecorte(anterior)

	recorte.ativo = false
	recorte.arrastando = false

	encerrarRecorte()
	encerrarRecorte()

	recorte.ativo = true
	recorte.arrastando = true
	recorte.origPix = make([]byte, 16)
	recorte.hwnd = 0

	encerrarRecorte()

	if recorte.ativo || recorte.arrastando || recorte.origPix != nil {
		t.Error("encerrar devia limpar tudo mesmo sem janela")
	}

	encerrarRecorte()
}
