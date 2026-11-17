//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	rtwin "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d/common"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/directwrite"
	dxgicommon "github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/dxgi/common"
	"golang.org/x/sys/windows"
)

type d2dFonteKey struct {
	face       string
	px         int32
	ah, av, wr int32
}

type d2dPincelAlfaKey struct {
	cor  uint32
	alfa uint8
}

type d2dMotor struct {
	hwnd    uintptr
	rt      *direct2d.ID2D1HwndRenderTarget
	solidos map[uint32]*direct2d.ID2D1Brush
	alfas   map[d2dPincelAlfaKey]*direct2d.ID2D1Brush
	fontes  map[d2dFonteKey]*directwrite.IDWriteTextFormat
	bmps    map[string]*direct2d.ID2D1Bitmap
	bmpTam  int32
}

const (
	mascaraCanalD2D      = 0xFF
	deslocVerdeD2D       = 8
	deslocAzulD2D        = 16
	alfaOpacoD2D         = 255
	meioD2D              = 2
	baseHexD2D           = 16
	dpiPadraoD2D         = 96
	bytesPorPixelD2D     = 4
	tamFontePeqD2D       = 11
	tamFonteD2D          = 13
	ajusteMedidaD2D      = 0.5
	largMaxMedidaD2D     = 100000
	altMaxLayoutD2D      = 10000
	porcentoD2D          = 100
	alfaFundoRecorteD2D  = 0.59
	tamIconeD2D          = 16
	linhasPrevClipD2D    = 2
	alfaIconeExtD2D      = 255
	segsPorMinD2D        = 60
	limiarEtaMinD2D      = 45
	tamChipClipD2D       = 32
	alinhFimD2D          = 2
	zonaZoomPctD2D       = 2
	zonaZoomMaisD2D      = 3
	zonaZoomAjusteD2D    = 4
	blocosCargaD2D       = 3
	alinhamentoStrideD2D = 4
	baseDecimalD2D       = 10
)

//nolint:gochecknoglobals // Motor D2D unico + factory: estado de processo.
var (
	d2dMain d2dMotor
	d2dWF   *directwrite.IDWriteFactory
)

func d2dCorF(cor uint32) common.D2D1_COLOR_F {
	return common.D2D1_COLOR_F{
		R: float32(cor&mascaraCanalD2D) / alfaOpacoD2D,
		G: float32((cor>>deslocVerdeD2D)&mascaraCanalD2D) / alfaOpacoD2D,
		B: float32((cor>>deslocAzulD2D)&mascaraCanalD2D) / alfaOpacoD2D,
		A: 1,
	}
}

func d2dRet(area rectT) common.D2D_RECT_F {
	return common.D2D_RECT_F{
		Left: float32(area.Left), Top: float32(area.Top),
		Right: float32(area.Right), Bottom: float32(area.Bottom),
	}
}

func (d *d2dMotor) soltar() {
	for _, pincel := range d.solidos {
		if pincel != nil {
			pincel.Release()
		}
	}

	for _, pincel := range d.alfas {
		if pincel != nil {
			pincel.Release()
		}
	}

	for _, pincel := range d.bmps {
		if pincel != nil {
			pincel.Release()
		}
	}

	for _, fonte := range d.fontes {
		if fonte != nil {
			fonte.Release()
		}
	}

	if d.rt != nil {
		d.rt.Release()
		d.rt = nil
	}

	d.solidos = nil
	d.alfas = nil
	d.fontes = nil
	d.bmps = nil
	d.bmpTam = 0
}

func limparD2D() {
	d2dMain.soltar()
}

func (d *d2dMotor) criar(hwnd uintptr, largura, altura int32) {
	d.soltar()
	d.hwnd = hwnd

	var unk *rtwin.IUnknown
	if err := direct2d.D2D1CreateFactory(direct2d.D2D1_FACTORY_TYPE_SINGLE_THREADED,
		&direct2d.IID_ID2D1Factory, nil, &unk); err != nil || unk == nil {
		return
	}

	factory := (*direct2d.ID2D1Factory)(unsafe.Pointer(unk))

	var props direct2d.D2D1_RENDER_TARGET_PROPERTIES

	//exhaustruct:ignore // PresentOptions padrao do D2D: zero intencional.
	propsAlvo := direct2d.D2D1_HWND_RENDER_TARGET_PROPERTIES{
		Hwnd:      foundation.HWND(hwnd),
		PixelSize: common.D2D_SIZE_U{Width: uint32(largura), Height: uint32(altura)},
	}

	var target *direct2d.ID2D1HwndRenderTarget
	if err := factory.CreateHwndRenderTarget(&props, &propsAlvo, &target); err != nil || target == nil {
		factory.Release()

		return
	}

	factory.Release()

	d.rt = target
}

func (d *d2dMotor) pincel(cor uint32) *direct2d.ID2D1Brush {
	if d.rt == nil {
		return nil
	}

	if d.solidos == nil {
		d.solidos = map[uint32]*direct2d.ID2D1Brush{}
	}

	if pincel, ok := d.solidos[cor]; ok {
		return pincel
	}

	corF := d2dCorF(cor)

	var pincelSolido *direct2d.ID2D1SolidColorBrush
	if err := d.rt.CreateSolidColorBrush(&corF, nil, &pincelSolido); err != nil || pincelSolido == nil {
		return nil
	}

	pincelBase := &pincelSolido.ID2D1Brush
	d.solidos[cor] = pincelBase

	return pincelBase
}

// pincelAlfa devolve um pincel solido com alpha explicito (para glow e halo).
func (d *d2dMotor) pincelAlfa(cor uint32, alfa uint8) *direct2d.ID2D1Brush {
	if d.rt == nil {
		return nil
	}

	if d.alfas == nil {
		d.alfas = map[d2dPincelAlfaKey]*direct2d.ID2D1Brush{}
	}

	chave := d2dPincelAlfaKey{cor: cor, alfa: alfa}
	if pincel, ok := d.alfas[chave]; ok {
		return pincel
	}

	corF := d2dCorF(cor)
	corF.A = float32(alfa) / alfaOpacoD2D

	var pincelSolido *direct2d.ID2D1SolidColorBrush
	if err := d.rt.CreateSolidColorBrush(&corF, nil, &pincelSolido); err != nil || pincelSolido == nil {
		return nil
	}

	pincelBase := &pincelSolido.ID2D1Brush
	d.alfas[chave] = pincelBase

	return pincelBase
}

// elipseAlfa preenche uma elipse com alpha explicito.
func (d *d2dMotor) elipseAlfa(area rectT, cor uint32, alfa uint8) {
	if d.rt == nil || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	if pincel := d.pincelAlfa(cor, alfa); pincel != nil {
		elipse := direct2d.D2D1_ELLIPSE{
			Point:   common.D2D_POINT_2F{X: float32(area.Left+area.Right) / meioD2D, Y: float32(area.Top+area.Bottom) / meioD2D},
			RadiusX: float32(area.Width()) / meioD2D,
			RadiusY: float32(area.Height()) / meioD2D,
		}
		d.rt.FillEllipse(&elipse, pincel)
	}
}

func d2dGarantirWF() bool {
	if d2dWF != nil {
		return true
	}

	var unk *rtwin.IUnknown
	if err := directwrite.DWriteCreateFactory(directwrite.DWRITE_FACTORY_TYPE_SHARED,
		&directwrite.IID_IDWriteFactory, &unk); err != nil || unk == nil {
		return false
	}

	d2dWF = (*directwrite.IDWriteFactory)(unsafe.Pointer(unk))

	return true
}

func (d *d2dMotor) fonte(face string, tamPx int32, alinhH, alinhV, quebra int32) *directwrite.IDWriteTextFormat {
	if !d2dGarantirWF() {
		return nil
	}

	if d.fontes == nil {
		d.fontes = map[d2dFonteKey]*directwrite.IDWriteTextFormat{}
	}

	chave := d2dFonteKey{face: face, px: tamPx, ah: alinhH, av: alinhV, wr: quebra}
	if formato, ok := d.fontes[chave]; ok {
		return formato
	}

	var formatoTexto *directwrite.IDWriteTextFormat
	if err := d2dWF.CreateTextFormat(face, nil, directwrite.DWRITE_FONT_WEIGHT_NORMAL,
		directwrite.DWRITE_FONT_STYLE_NORMAL, directwrite.DWRITE_FONT_STRETCH_NORMAL,
		float32(tamPx), idiomaPtBR, &formatoTexto); err != nil || formatoTexto == nil {
		return nil
	}

	var alinhamento directwrite.DWRITE_TEXT_ALIGNMENT

	switch alinhH {
	case 1:
		alinhamento = directwrite.DWRITE_TEXT_ALIGNMENT_CENTER
	case alinhFimD2D:
		alinhamento = directwrite.DWRITE_TEXT_ALIGNMENT_TRAILING
	}

	var alinhVert directwrite.DWRITE_PARAGRAPH_ALIGNMENT
	if alinhV == 1 {
		alinhVert = directwrite.DWRITE_PARAGRAPH_ALIGNMENT_CENTER
	}

	var embrulho directwrite.DWRITE_WORD_WRAPPING
	if quebra == 1 {
		embrulho = directwrite.DWRITE_WORD_WRAPPING_NO_WRAP
	}

	_ = formatoTexto.SetTextAlignment(alinhamento)
	_ = formatoTexto.SetParagraphAlignment(alinhVert)
	_ = formatoTexto.SetWordWrapping(embrulho)
	d.aplicarReticencias(formatoTexto, quebra)
	d.fontes[chave] = formatoTexto

	return formatoTexto
}

// aplicarReticencias liga o "..." do DirectWrite nas linhas unicas.
func (d *d2dMotor) aplicarReticencias(formatoTexto *directwrite.IDWriteTextFormat, quebra int32) {
	if formatoTexto == nil || quebra != 1 || d2dWF == nil {
		return
	}

	var sinal *directwrite.IDWriteInlineObject
	if err := d2dWF.CreateEllipsisTrimmingSign(formatoTexto, &sinal); err != nil || sinal == nil {
		return
	}

	defer sinal.Release()

	corte := directwrite.DWRITE_TRIMMING{
		Granularity:    directwrite.DWRITE_TRIMMING_GRANULARITY_CHARACTER,
		Delimiter:      0,
		DelimiterCount: 0,
	}
	_ = formatoTexto.SetTrimming(&corte, sinal)
}

// desenharTexto emite o DrawText com formato e pincel ja resolvidos.
func (d *d2dMotor) desenharTexto(
	texto string, area rectT, formato *directwrite.IDWriteTextFormat, pincel *direct2d.ID2D1Brush,
) {
	if d.rt == nil || texto == "" || area.Width() <= 0 || area.Height() <= 0 || formato == nil || pincel == nil {
		return
	}

	total := 0
	for range texto {
		total++
	}

	areaF := d2dRet(area)
	d.rt.DrawText(texto, uint32(total), formato, &areaF, pincel,
		direct2d.D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|direct2d.D2D1_DRAW_TEXT_OPTIONS_CLIP,
		directwrite.DWRITE_MEASURING_MODE_NATURAL)
}

func (d *d2dMotor) texto(texto string, area rectT, cor uint32, face string, tamPx int32, alinhH, alinhV, quebra int32) {
	if d.rt == nil || texto == "" || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	formato := d.fonte(face, tamPx, alinhH, alinhV, quebra)
	if formato == nil {
		return
	}

	d.desenharTexto(texto, area, formato, d.pincel(cor))
}

// textoComHalo aplica o text-shadow de 3 camadas do html.float do Marble:
// 0 0 2px, 0 0 6px e 0 0 12px na cor --ground, antes do texto principal.
func (d *d2dMotor) textoComHalo(
	texto string, area rectT, cor, corHalo uint32, face string, tamPx int32, alinhH, alinhV, quebra int32,
) {
	if d.rt == nil || texto == "" || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	formato := d.fonte(face, tamPx, alinhH, alinhV, quebra)
	if formato == nil {
		return
	}

	for _, raio := range [...]int32{espHaloRaio1, espHaloRaio2, espHaloRaio3} {
		halo := rectT{area.Left - raio, area.Top - raio, area.Right + raio, area.Bottom + raio}
		d.desenharTexto(texto, halo, formato, d.pincelAlfa(corHalo, espHaloAlfa))
	}

	d.desenharTexto(texto, area, formato, d.pincel(cor))
}

func (d *d2dMotor) elipse(area rectT, cor uint32) {
	if d.rt == nil || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	if pincel := d.pincel(cor); pincel != nil {
		elipse := direct2d.D2D1_ELLIPSE{
			Point:   common.D2D_POINT_2F{X: float32(area.Left+area.Right) / meioD2D, Y: float32(area.Top+area.Bottom) / meioD2D},
			RadiusX: float32(area.Width()) / meioD2D,
			RadiusY: float32(area.Height()) / meioD2D,
		}
		d.rt.FillEllipse(&elipse, pincel)
	}
}

func (d *d2dMotor) linha(iniX, iniY, fimX, fimY float32, esp float32) {
	if d.rt == nil {
		return
	}

	if pincel := d.pincel(corAccentSoft); pincel != nil {
		d.rt.DrawLine(common.D2D_POINT_2F{X: iniX, Y: iniY}, common.D2D_POINT_2F{X: fimX, Y: fimY}, pincel, esp, nil)
	}
}

func (d *d2dMotor) linhaCor(iniX, iniY, fimX, fimY float32, esp float32, cor uint32, alfa uint8) {
	if d.rt == nil {
		return
	}

	if pincel := d.pincelAlfa(cor, alfa); pincel != nil {
		d.rt.DrawLine(common.D2D_POINT_2F{X: iniX, Y: iniY}, common.D2D_POINT_2F{X: fimX, Y: fimY}, pincel, esp, nil)
	}
}

func (d *d2dMotor) contorno(area rectT, cor uint32) {
	if area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	d.ret(rectT{area.Left, area.Top, area.Right, area.Top + 1}, cor)
	d.ret(rectT{area.Left, area.Bottom - 1, area.Right, area.Bottom}, cor)
	d.ret(rectT{area.Left, area.Top, area.Left + 1, area.Bottom}, cor)
	d.ret(rectT{area.Right - 1, area.Top, area.Right, area.Bottom}, cor)
}

func (d *d2dMotor) larguraTexto(texto string, fonte uintptr, _ int32) int32 {
	largura, _ := d.medirDWrite(texto, tamanhoFonte(fonte), false, 0)

	return largura
}

func (d *d2dMotor) alturaTextoQuebrado(texto string, fonte uintptr, larg int32) int32 {
	if larg <= 0 {
		return 0
	}

	_, altura := d.medirDWrite(texto, tamanhoFonte(fonte), true, larg)

	return altura
}

func tamanhoFonte(fonte uintptr) int32 {
	if theApp != nil && fonte == theApp.fontSm {
		return tamFontePeqD2D
	}

	return tamFonteD2D
}

func (d *d2dMotor) medirDWrite(texto string, sizeLogical int32, quebrar bool, maxW int32) (int32, int32) {
	tamPx, largMax, quebra, pronto := parametrosMedidaDWrite(texto, sizeLogical, quebrar, maxW)
	if !pronto {
		return 0, 0
	}

	layout, pronto := d.criarLayoutMedida(texto, tamPx, largMax, quebra)
	if !pronto {
		return 0, 0
	}
	defer layout.Release()

	var metricas directwrite.DWRITE_TEXT_METRICS
	if err := layout.GetMetrics(&metricas); err != nil {
		return 0, 0
	}

	return limitarMedidaDWrite(
		int32(metricas.WidthIncludingTrailingWhitespace+ajusteMedidaD2D),
		int32(metricas.Height+ajusteMedidaD2D))
}

// parametrosMedidaDWrite valida a entrada e converte para unidades do DWrite.
func parametrosMedidaDWrite(texto string, sizeLogical int32, quebrar bool, maxW int32) (int32, float32, int32, bool) {
	if theApp == nil || strings.TrimSpace(texto) == "" || !d2dGarantirWF() {
		return 0, 0, 0, false
	}

	tamPx := theApp.sc(sizeLogical)
	quebra := int32(1)
	largMax := float32(largMaxMedidaD2D)

	if quebrar {
		quebra = 0
		largMax = float32(maxW)
	}

	return tamPx, largMax, quebra, true
}

// criarLayoutMedida monta o TextLayout para medir a string.
func (d *d2dMotor) criarLayoutMedida(
	texto string, tamPx int32, largMax float32, quebra int32,
) (*directwrite.IDWriteTextLayout, bool) {
	formato := d.fonte("Segoe UI", tamPx, 0, 0, quebra)
	if formato == nil {
		return nil, false
	}

	textoUtf16, err := windows.UTF16FromString(texto)
	if err != nil || len(textoUtf16) == 0 {
		return nil, false
	}

	var layout *directwrite.IDWriteTextLayout
	if err := d2dWF.CreateTextLayout(
		texto, uint32(len(textoUtf16)-1), formato, largMax, altMaxLayoutD2D, &layout,
	); err != nil || layout == nil {
		return nil, false
	}

	return layout, true
}

// limitarMedidaDWrite zera dimensoes negativas da metrica.
func limitarMedidaDWrite(largura, altura int32) (int32, int32) {
	if largura < 0 {
		largura = 0
	}

	if altura < 0 {
		altura = 0
	}

	return largura, altura
}

func (d *d2dMotor) quebrar(texto string, fonte uintptr, maxW int32, maxLinhas int) []string {
	limpo := strings.Join(strings.Fields(texto), " ")
	if limpo == "" || maxW <= 0 || maxLinhas <= 0 {
		return nil
	}
	// Cache local: medir a mesma linha repetidas vezes (uma por palavra)
	// recriava um TextLayout a cada chamada.
	medidas := map[string]int32{}
	medir := func(linha string) int32 {
		if largura, ok := medidas[linha]; ok {
			return largura
		}

		largura := d.larguraTexto(linha, fonte, maxW)
		medidas[linha] = largura

		return largura
	}

	return quebrarComMedida(limpo, maxW, maxLinhas, medir)
}

// medidaClip mede a largura em pixels de um candidato a linha.
type medidaClip func(candidato string) int32

// quebrarComMedida divide o texto em linhas que cabem na largura maxima.
// Palavra sem espaco e fatiada no meio para nunca vazar do retangulo.
func quebrarComMedida(textoLimpo string, larguraMax int32, maxLinhas int, medirLargura medidaClip) []string {
	if textoLimpo == "" || larguraMax <= 0 || maxLinhas <= 0 || medirLargura == nil {
		return nil
	}

	var linhas []string

	for palavra := range strings.SplitSeq(textoLimpo, " ") {
		linhas = acomodarPalavraClip(linhas, palavra, larguraMax, medirLargura)
	}

	return limitarLinhasClip(linhas, maxLinhas, medirLargura, larguraMax)
}

// acomodarPalavraClip anexa a palavra na ultima linha ou abre linha nova.
// Palavra maior que a largura e fatiada antes de anexar.
func acomodarPalavraClip(destino []string, palavra string, larguraMax int32, medirLargura medidaClip) []string {
	if palavra == "" {
		return destino
	}

	if len(destino) == 0 {
		return append(destino, fatiarPalavraLonga(palavra, larguraMax, medirLargura)...)
	}

	tentativa := destino[len(destino)-1] + " " + palavra
	if medirLargura(tentativa) <= larguraMax {
		destino[len(destino)-1] = tentativa

		return destino
	}

	return append(destino, fatiarPalavraLonga(palavra, larguraMax, medirLargura)...)
}

// fatiarPalavraLonga corta palavra sem espaco em pedacos que cabem na largura.
// Cada pedaco e medido por runa para respeitar CJK e emoji.
func fatiarPalavraLonga(palavra string, larguraMax int32, medirLargura medidaClip) []string {
	if medirLargura(palavra) <= larguraMax {
		return []string{palavra}
	}

	var pedacos []string

	var atual strings.Builder

	descarregar := func() {
		if atual.Len() > 0 {
			pedacos = append(pedacos, atual.String())
			atual.Reset()
		}
	}

	for _, simbolo := range palavra {
		if atual.Len() > 0 && medirLargura(atual.String()+string(simbolo)) > larguraMax {
			descarregar()
		}

		atual.WriteRune(simbolo)
	}

	descarregar()

	if len(pedacos) == 0 {
		return []string{palavra}
	}

	return pedacos
}

// limitarLinhasClip corta no maximo de linhas e marca o corte com reticencias.
// A ultima linha e aparada por runa ate caber com o marcador.
func limitarLinhasClip(linhas []string, maxLinhas int, medirLargura medidaClip, larguraMax int32) []string {
	if len(linhas) <= maxLinhas {
		return linhas
	}

	cortadas := make([]string, maxLinhas)
	copy(cortadas, linhas)

	cortadas[maxLinhas-1] = ajustarReticenciasClip(cortadas[maxLinhas-1], medirLargura, larguraMax)

	return cortadas
}

// ajustarReticenciasClip apara a linha ate caber com o marcador de corte.
func ajustarReticenciasClip(linha string, medirLargura medidaClip, larguraMax int32) string {
	simbolos := []rune(linha)

	for len(simbolos) > 0 && medirLargura(string(simbolos)+"…") > larguraMax {
		simbolos = simbolos[:len(simbolos)-1]
	}

	return string(simbolos) + "…"
}

// retsSobrepoemClip informa se dois retangulos se interceptam.
// Usado em teste para garantir que texto nunca cobre botao.
func retsSobrepoemClip(primeiro, segundo rectT) bool {
	if primeiro.Right <= primeiro.Left || primeiro.Bottom <= primeiro.Top {
		return false
	}

	if segundo.Right <= segundo.Left || segundo.Bottom <= segundo.Top {
		return false
	}

	return primeiro.Left < segundo.Right && segundo.Left < primeiro.Right &&
		primeiro.Top < segundo.Bottom && segundo.Top < primeiro.Bottom
}

func (d *d2dMotor) ret(area rectT, cor uint32) {
	if d.rt == nil || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	if pincel := d.pincel(cor); pincel != nil {
		areaF := d2dRet(area)
		d.rt.FillRectangle(&areaF, pincel)
	}
}

func (d *d2dMotor) redondo(area rectT, raio int32, fill, borda uint32) {
	if d.rt == nil || area.Width() <= 0 || area.Height() <= 0 {
		return
	}

	cantos := direct2d.D2D1_ROUNDED_RECT{Rect: d2dRet(area), RadiusX: float32(raio), RadiusY: float32(raio)}

	if borda != 0 {
		if d.pintarRedondoComBorda(area, raio, fill, borda) {
			return
		}
	}

	if fundo := d.pincel(fill); fundo != nil {
		d.rt.FillRoundedRectangle(&cantos, fundo)
	}
}

// pintarRedondoComBorda preenche borda e miolo; true = pintou com borda.
func (d *d2dMotor) pintarRedondoComBorda(area rectT, raio int32, fill, borda uint32) bool {
	pincel := d.pincel(borda)
	if pincel == nil {
		return false
	}

	cantos := direct2d.D2D1_ROUNDED_RECT{Rect: d2dRet(area), RadiusX: float32(raio), RadiusY: float32(raio)}
	d.rt.FillRoundedRectangle(&cantos, pincel)

	miolo := area
	miolo.Left++
	miolo.Top++
	miolo.Right--

	miolo.Bottom--
	if miolo.Width() <= 0 || miolo.Height() <= 0 {
		return true
	}

	mioloArr := direct2d.D2D1_ROUNDED_RECT{Rect: d2dRet(miolo), RadiusX: float32(raio), RadiusY: float32(raio)}
	if fundo := d.pincel(fill); fundo != nil {
		d.rt.FillRoundedRectangle(&mioloArr, fundo)
	}

	return true
}

func (d *d2dMotor) icone(id string, area rectT, cor uint32, base int32) {
	cp := iconesFluent[id]
	if cp == 0 || theApp == nil || theApp.iconFace == "" {
		return
	}

	d.texto(string(rune(cp)), area, cor, theApp.iconFace, theApp.sc(base), 1, 1, 1)
}

func (d *d2dMotor) iniciar() {
	// fundo := d2dCorF(corFundo)
	fundo := common.D2D1_COLOR_F{R: 0, G: 0, B: 0, A: 0}

	d.rt.BeginDraw()
	d.rt.Clear(&fundo)
}

func (d *d2dMotor) apresentar() {
	var ini, fim uint64
	if err := d.rt.EndDraw(&ini, &fim); err != nil {
		d.soltar()

		return
	}

	pValidateRect.Call(d.hwnd, 0)
}

func (d *d2dMotor) redimensionar(largura, altura int32) {
	if d.rt == nil || largura <= 0 || altura <= 0 {
		return
	}

	tamanho := common.D2D_SIZE_U{Width: uint32(largura), Height: uint32(altura)}
	if err := d.rt.Resize(&tamanho); err != nil {
		d.soltar()
	}
}

func (d *d2dMotor) bmpExt(base string, caixa rectT) {
	if d.rt == nil || caixa.Width() <= 0 || caixa.Height() <= 0 || theApp == nil {
		return
	}

	d.reciclarBmpsExt()

	bitmap, ok := d.obterBitmapExt(base)
	if !ok {
		return
	}

	lado := theApp.sc(tamIconeD2D)
	posX := (caixa.Left + caixa.Right) / meioD2D
	posY := (caixa.Top + caixa.Bottom) / meioD2D
	destino := d2dRet(rectT{posX - lado/meioD2D, posY - lado/meioD2D, posX + lado/meioD2D, posY + lado/meioD2D})
	opacidade := float32(alfaIconeExtD2D) / alfaOpacoD2D
	d.rt.DrawBitmap(bitmap, &destino, opacidade, direct2d.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)
}

// reciclarBmpsExt limpa o cache quando o tamanho do icone muda.
func (d *d2dMotor) reciclarBmpsExt() {
	if tam := theApp.tamanhoIconeExt(); tam != d.bmpTam {
		for _, bitmap := range d.bmps {
			if bitmap != nil {
				bitmap.Release()
			}
		}

		d.bmps = nil
		d.bmpTam = tam
	}

	if d.bmps == nil {
		d.bmps = map[string]*direct2d.ID2D1Bitmap{}
	}
}

// obterBitmapExt devolve o bitmap em cache ou cria sob demanda.
func (d *d2dMotor) obterBitmapExt(base string) (*direct2d.ID2D1Bitmap, bool) {
	if bitmap, ok := d.bmps[base]; ok {
		return bitmap, true
	}

	bitmap, err := d.bmpBitmap(base)
	if err != nil || bitmap == nil {
		return nil, false
	}

	d.bmps[base] = bitmap

	return bitmap, true
}

type d2dPixels struct {
	p    unsafe.Pointer
	lado int32
	buf  []byte
}

func d2dPixelsIcone(base string) (d2dPixels, error) {
	var pixels d2dPixels

	tamanho := theApp.tamanhoIconeExt()

	orig := theApp.pastaRealExtensao(base)
	if orig == "" {
		return pixels, erroTexto("sem pasta")
	}

	dir := filepath.Join(extDir, orig)

	var manifesto extManifest
	// GOD: orig e pasta real no disco (pastaRealExtensao); nome fixo.
	if dados, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil { // #nosec G304
		_ = json.Unmarshal(dados, &manifesto)
	}

	abs := iconeArquivoExt(dir, manifesto)
	if abs == "" {
		return pixels, erroTexto("sem icone")
	}

	recorte, err := decodificarIcone(abs)
	if err != nil {
		return pixels, err
	}

	dstW, dstH, offX, offY := enquadrarIcone(recorte.crop, tamanho)
	pixels.buf = make([]byte, int(tamanho)*int(tamanho)*bytesPorPixelD2D)
	fillIconPixels(pixels.buf, recorte.src, recorte.crop, recorte.stripBg, recorte.bg, tamanho, dstW, dstH, offX, offY)
	pixels.p = unsafe.Pointer(&pixels.buf[0])
	pixels.lado = tamanho

	return pixels, nil
}

func (d *d2dMotor) retB(area rectT, cor uint32) {
	d.ret(area, cor)
}

func (d *d2dMotor) redondoB(area rectT, raio int32, fillB, borderB uint32) {
	d.redondo(area, raio, fillB, borderB)
}

func (d *d2dMotor) navIcone(id string, area rectT, cor uint32) {
	if theApp == nil {
		return
	}

	if iconesFluent[id] != 0 {
		d.icone(id, area, cor, tamIconeD2D)

		return
	}

	if glifo := glifoNav(id); glifo != "" {
		d.texto(glifo, area, cor, "Segoe UI", theApp.sc(tamIconeD2D), 1, 1, 1)
	}
}

func (d *d2dMotor) clip(area rectT) {
	if d.rt == nil {
		return
	}

	areaF := d2dRet(area)
	d.rt.PushAxisAlignedClip(&areaF, direct2d.D2D1_ANTIALIAS_MODE_PER_PRIMITIVE)
}

func (d *d2dMotor) desclip() {
	if d.rt == nil {
		return
	}

	d.rt.PopAxisAlignedClip()
}

func (d *d2dMotor) textoUI(texto string, area rectT, cor uint32, pequeno bool, centro bool) {
	if theApp == nil {
		return
	}

	tamPx := theApp.sc(tamFonteD2D)
	if pequeno {
		tamPx = theApp.sc(tamFontePeqD2D)
	}

	alinhH := int32(0)
	if centro {
		alinhH = 1
	}

	d.texto(texto, area, cor, "Segoe UI", tamPx, alinhH, 1, 1)
}

func (d *d2dMotor) textoUIR(texto string, area rectT, cor uint32, pequeno bool) {
	if theApp == nil {
		return
	}

	tamPx := theApp.sc(tamFonteD2D)
	if pequeno {
		tamPx = theApp.sc(tamFontePeqD2D)
	}

	d.texto(texto, area, cor, "Segoe UI", tamPx, alinhFimD2D, 1, 1)
}

func (d *d2dMotor) textoFonte(texto string, area rectT, cor uint32, fonte uintptr, flags uint32) {
	if theApp == nil {
		return
	}

	var tamPx int32

	pequeno := true

	if fonte == theApp.fontUI {
		tamPx = theApp.sc(tamFonteD2D)
		pequeno = false
	} else {
		tamPx = theApp.sc(tamFontePeqD2D)
	}

	alinhH := int32(0)
	if flags&dtCenter != 0 {
		alinhH = 1
	}

	if flags&dtRight != 0 {
		alinhH = alinhFimD2D
	}

	alinhV := int32(0)
	if flags&dtVCenter != 0 {
		alinhV = 1
	}

	quebra := int32(1)
	if flags&dtWordbreak != 0 {
		quebra = 0
	}

	if flags&dtEndEllipsis != 0 {
		d.texto(texto, area, cor, "Segoe UI", tamPx, alinhH, alinhV, 1)

		return
	}

	d.texto(texto, area, cor, "Segoe UI", tamPx, alinhH, alinhV, quebra)

	_ = pequeno
}

//nolint:gochecknoglobals // Motores de desenho auxiliares: estado de UI single-window.
var (
	d2dMenu, d2dSub, d2dSub2, d2dPanel d2dMotor
	d2dClip                            d2dMotor
	d2dClipImgs                        = map[uint64]*direct2d.ID2D1Bitmap{}
	d2dClipImgsMu                      sync.Mutex
	d2dRec                             d2dMotor
	d2dRecOrig                         *direct2d.ID2D1Bitmap
	d2dAviso, d2dTip, d2dCfg           d2dMotor
	d2dCalendario                      d2dMotor
)

func d2dBmpPixels(buf []byte, largura, altura int32) (*direct2d.ID2D1Bitmap, error) {
	if len(buf) < int(largura)*int(altura)*bytesPorPixelD2D || largura <= 0 || altura <= 0 {
		return nil, erroTexto("sem pixels")
	}

	props := direct2d.D2D1_BITMAP_PROPERTIES{
		PixelFormat: common.D2D1_PIXEL_FORMAT{
			Format:    dxgicommon.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: common.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX: dpiPadraoD2D,
		DpiY: dpiPadraoD2D,
	}
	tamanho := common.D2D_SIZE_U{Width: uint32(largura), Height: uint32(altura)}

	var bitmap *direct2d.ID2D1Bitmap
	if err := d2dRec.rt.CreateBitmap(
		tamanho, unsafe.Pointer(&buf[0]), uint32(largura)*bytesPorPixelD2D, &props, &bitmap,
	); err != nil || bitmap == nil {
		return nil, erroTexto("sem bitmap d2d")
	}

	return bitmap, nil
}

func d2dRecGarantir() bool {
	if d2dRecOrig != nil {
		return true
	}

	orig, err := d2dBmpPixels(recorte.origPix, recorte.w, recorte.h)
	if err != nil {
		return false
	}

	d2dRecOrig = orig

	return true
}

func (a *app) paintRecorteD2D() {
	disp := &d2dRec

	if !recorte.ativo || len(recorte.origPix) == 0 {
		return
	}

	if !d2dRecGarantir() {
		return
	}

	fundo := d2dRet(rectT{0, 0, recorte.w, recorte.h})

	disp.rt.DrawBitmap(d2dRecOrig, &fundo, alfaFundoRecorteD2D, direct2d.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)

	selecao := recorte.sel

	temSel := selecao.Width() >= meioD2D && selecao.Height() >= meioD2D
	if temSel {
		destino := d2dRet(selecao)
		origem := d2dRet(selecao)
		disp.rt.DrawBitmap(d2dRecOrig, &destino, 1, direct2d.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, &origem)
	}

	altura := a.sc(18)
	a.pintarDicaRecorteD2D(disp, altura)

	if !temSel {
		return
	}

	a.pintarSelecaoRecorteD2D(disp, selecao, altura)
}

// pintarDicaRecorteD2D mostra a dica de arraste enquanto nao escolhe.
func (a *app) pintarDicaRecorteD2D(disp *d2dMotor, altura int32) {
	if recorte.escolhendo {
		return
	}

	dica := txtRecorteArraste
	largura := disp.larguraTexto(dica, a.fontUI, recorte.w)
	x := (recorte.w - largura) / meioD2D

	y := a.sc(22)
	if x < a.sc(8) {
		x = a.sc(8)
	}

	fundo := rectT{x - a.sc(12), y - a.sc(6), x + largura + a.sc(12), y + altura + a.sc(6)}
	disp.redondoB(fundo, a.sc(9), a.brSurface, a.brBorder)
	disp.textoUI(dica, rectT{x, y, x + largura, y + altura}, corTextoFraco, false, false)
}

// pintarSelecaoRecorteD2D desenha moldura e etiqueta da selecao.
func (a *app) pintarSelecaoRecorteD2D(disp *d2dMotor, selecao rectT, altura int32) {
	disp.linha(
		float32(selecao.Left), float32(selecao.Top),
		float32(selecao.Right-1), float32(selecao.Top), meioD2D*float32(a.sc(1)))
	disp.linha(
		float32(selecao.Right-1), float32(selecao.Top),
		float32(selecao.Right-1), float32(selecao.Bottom), meioD2D*float32(a.sc(1)))
	disp.linha(
		float32(selecao.Right-1), float32(selecao.Bottom),
		float32(selecao.Left), float32(selecao.Bottom), meioD2D*float32(a.sc(1)))
	disp.linha(
		float32(selecao.Left), float32(selecao.Bottom),
		float32(selecao.Left), float32(selecao.Top), meioD2D*float32(a.sc(1)))

	posX, posY := a.posicaoEtiquetaRecorte(selecao, disp, altura)

	tamanho := fmt.Sprintf(txtFmtRecorteTam, selecao.Width(), selecao.Height())
	largTexto := disp.larguraTexto(tamanho, a.fontUI, recorte.w)

	etiqueta := rectT{posX, posY, posX + largTexto + a.sc(18), posY + altura + a.sc(8)}
	disp.redondo(etiqueta, a.sc(7), a.brChrome, a.brAccent)
	disp.textoUI(
		tamanho, rectT{posX + a.sc(9), posY + a.sc(4), posX + largTexto + a.sc(9), posY + altura + a.sc(4)},
		corTexto, false, false)

	if recorte.escolhendo {
		a.paintRecorteBarraD2D()
	}
}

// posicaoEtiquetaRecorte calcula onde fica a etiqueta de tamanho.
func (a *app) posicaoEtiquetaRecorte(selecao rectT, disp *d2dMotor, altura int32) (int32, int32) {
	tamanho := fmt.Sprintf(txtFmtRecorteTam, selecao.Width(), selecao.Height())
	largTexto := disp.larguraTexto(tamanho, a.fontUI, recorte.w)
	posX := selecao.Left

	posY := selecao.Top - altura - a.sc(8)
	if posY < 0 {
		posY = selecao.Bottom + a.sc(6)
	}

	if posX+largTexto+a.sc(20) > recorte.w {
		posX = recorte.w - largTexto - a.sc(20)
	}

	if posX < 0 {
		posX = 0
	}

	return posX, posY
}

func (a *app) paintRecorteBarraD2D() {
	disp := &d2dRec

	barra := recorte.barra
	if barra.Width() <= 0 {
		return
	}

	disp.redondoB(barra, a.sc(12), a.brSurface, a.brBorder)

	rotulos := [...]string{txtRecorteCopiar, txtRecorteOCR, txtRecorteSalvar, txtRecorteDescart}
	icones := [...]string{idCopy, "scan", idSave, "discard"}

	for indice, botao := range recorte.botoes {
		cor := uint32(corTexto)

		if indice == 0 {
			fundo := a.brAccent
			if recorte.hoverBtn == indice {
				fundo = a.brIconHover
			}

			disp.redondoB(botao, a.sc(8), fundo, 0)

			cor = 0x00000000
		} else {
			fundo := a.brChrome
			if recorte.hoverBtn == indice {
				fundo = a.brHover
			}

			disp.redondoB(botao, a.sc(8), fundo, a.brBorder)
		}

		iconeArea := rectT{botao.Left + a.sc(10), botao.Top + a.sc(9), botao.Left + a.sc(26), botao.Bottom - a.sc(9)}
		disp.navIconeAuto(icones[indice], iconeArea, cor)
		disp.textoUI(
			rotulos[indice], rectT{botao.Left + a.sc(30), botao.Top, botao.Right - a.sc(4), botao.Bottom},
			cor, false, false)
	}

	dica := txtRecorteTeclas
	disp.textoUI(
		dica, rectT{barra.Left + a.sc(12), barra.Bottom - a.sc(26), barra.Right - a.sc(12), barra.Bottom - a.sc(6)},
		corTextoFraco, true, true)
}

func (a *app) paintAvisoD2D() {
	disp := &d2dAviso

	var area rectT
	pGetClientRect.Call(aviso.hwnd, uintptr(unsafe.Pointer(&area)))

	if area.Right <= 0 || area.Bottom <= 0 {
		return
	}

	cartao := rectT{area.Left, area.Top + aviso.offsetY, area.Right, area.Bottom + aviso.offsetY}
	disp.redondo(cartao, a.sc(9), a.brTipBg, corBorda)

	posY := cartao.Top + (cartao.Height()-a.sc(16))/meioD2D
	if aviso.temBarra {
		posY = cartao.Top + a.sc(8) + (int32(len(aviso.linhas))*a.sc(19)-a.sc(16))/meioD2D
	}

	disp.icone(
		"info", rectT{cartao.Left + a.sc(13), posY, cartao.Left + a.sc(29), posY + a.sc(16)},
		corAccentSoft, tamIconeD2D)

	texto := rectT{cartao.Left + a.sc(38), cartao.Top, cartao.Right - a.sc(14), cartao.Bottom}
	if len(aviso.linhas) > 0 {
		a.pintarLinhasAvisoD2D(disp, texto)
	} else {
		disp.textoClip(aviso.texto, texto, corTexto, false)
	}

	if aviso.fixo {
		a.pintarBotoesAvisoD2D(disp, cartao)
	}
}

// pintarBotoesAvisoD2D desenha copiar e dispensar no aviso persistente.
func (a *app) pintarBotoesAvisoD2D(disp *d2dMotor, cartao rectT) {
	copiar := deslocarRect(aviso.btnCopiar, 0, cartao.Top)
	fechar := deslocarRect(aviso.btnFechar, 0, cartao.Top)

	disp.redondo(copiar, a.sc(5), a.brActive, corBorda)
	disp.textoUI(txtAvisoCopiar, copiar, corTexto, false, true)

	disp.redondo(fechar, a.sc(5), a.brHover, corBorda)
	disp.textoUI(txtAvisoDispensar, fechar, corTexto, false, true)
}

func deslocarRect(r rectT, dx, dy int32) rectT {
	return rectT{r.Left + dx, r.Top + dy, r.Right + dx, r.Bottom + dy}
}

// pintarLinhasAvisoD2D desenha as linhas do aviso e a barra opcional.
func (a *app) pintarLinhasAvisoD2D(disp *d2dMotor, texto rectT) {
	y := texto.Top + a.sc(8)
	for _, linha := range aviso.linhas {
		disp.textoUI(linha, rectT{texto.Left, y, texto.Right, y + a.sc(19)}, corTexto, false, false)
		y += a.sc(19)
	}

	if aviso.temBarra {
		a.pintarBarraAvisoD2D(disp, texto, y)
	}
}

// pintarBarraAvisoD2D desenha a barra de progresso do aviso.
func (a *app) pintarBarraAvisoD2D(disp *d2dMotor, texto rectT, y int32) {
	iniX, fimX := texto.Left, texto.Right
	posY := y + a.sc(3)

	faixa := rectT{iniX, posY, fimX, posY + a.sc(3)}
	if aviso.indet {
		disp.ret(faixa, corAccentSoft)

		return
	}

	disp.ret(faixa, a.brSep)

	largura := int32(int64(faixa.Width()) * int64(aviso.pct) / porcentoD2D)
	if largura > 0 {
		parcial := rectT{iniX, posY, iniX + largura, posY + a.sc(3)}
		disp.ret(parcial, corAccentSoft)
	}
}

func (a *app) paintTipD2D() {
	disp := &d2dTip

	if a == nil || a.tipHwnd == 0 {
		return
	}

	var area rectT
	pGetClientRect.Call(a.tipHwnd, uintptr(unsafe.Pointer(&area)))

	if area.Right <= 0 || area.Bottom <= 0 {
		return
	}

	disp.retB(area, a.brTipBg)
	disp.contorno(area, corBordaForte)
	interno := rectT{area.Left + a.sc(8), area.Top, area.Right - a.sc(8), area.Bottom}
	disp.textoUI(a.tipText, interno, corCianoClaro, true, false)
}

func itoaD2D(num int64) string {
	if num == 0 {
		return "0"
	}

	negativo := num < 0
	if negativo {
		num = -num
	}

	var digitos [20]byte

	indice := len(digitos)
	for num > 0 {
		indice--
		digitos[indice] = byte(num%baseDecimalD2D) + '0'
		num /= 10
	}

	if negativo {
		indice--
		digitos[indice] = '-'
	}

	return string(digitos[indice:])
}

func (a *app) paintCfgD2D() {
	disp := &d2dCfg
	recorte := rectT{0, 0, cfgUI.w, cfgUI.h}
	disp.retB(recorte, a.brSurface)
	pad := a.sc(16)
	disp.textoUI(txtCfgTitulo, rectT{pad, a.sc(12), cfgUI.navW, a.sc(40)}, corTexto, false, false)

	if cfgUI.hoverID == "x" {
		disp.elipse(cfgUI.fechar, corBordaForte)
	}

	disp.icone("close", cfgUI.fechar, corTextoFaint, tamIconeD2D)

	topo := cfgUI.headH + a.sc(8)
	a.pintarNavegacaoCfgD2D(disp, topo)

	sep := rectT{cfgUI.navW, cfgUI.headH, cfgUI.navW + 1, cfgUI.h}
	disp.retB(sep, a.brSep)

	a.pintarTextosCfgD2D(disp)
	a.pintarZonasCfgD2D()
	a.pintarDownsCfgD2D()
	a.pintarBarrasCfgD2D(disp)
	a.pintarRolagemCfgD2D(disp)
}

// alturaVisivelCfg devolve a altura util da area rolavel.
func (a *app) alturaVisivelCfg() int32 {
	return cfgUI.h - cfgUI.headH - a.sc(16)*dobroMargemCfg
}

// trilhoBarraCfg devolve a trilha no canto direito do conteudo.
func (a *app) trilhoBarraCfg() rectT {
	return rectT{cfgUI.w - a.sc(10), cfgUI.headH + a.sc(4), cfgUI.w - a.sc(4), cfgUI.h - a.sc(4)}
}

// polegadaBarraCfg devolve o polegar proporcional a rolagem.
func (a *app) polegadaBarraCfg(trilho rectT) rectT {
	vis := a.alturaVisivelCfg()

	total := cfgUI.maxScroll + vis
	if total <= 0 {
		total = 1
	}

	altura := trilho.Height() * vis / total
	if minimo := a.sc(24); altura < minimo {
		altura = minimo
	}

	if altura > trilho.Height() {
		altura = trilho.Height()
	}

	viagem := trilho.Height() - altura

	topo := trilho.Top
	if viagem > 0 && cfgUI.maxScroll > 0 {
		topo += viagem * cfgUI.scroll / cfgUI.maxScroll
	}

	return rectT{trilho.Left, topo, trilho.Right, topo + altura}
}

// pintarRolagemCfgD2D desenha a barra de rolagem da configuracao.
func (a *app) pintarRolagemCfgD2D(disp *d2dMotor) {
	if cfgUI.maxScroll <= 0 {
		return
	}

	trilho := a.trilhoBarraCfg()
	disp.retB(trilho, a.brSep)

	fundo := a.brHover
	if cfgUI.barHover || cfgUI.arrastaBarra {
		fundo = a.brAccent
	}

	disp.retB(a.polegadaBarraCfg(trilho), fundo)
}

// Zonas da barra de rolagem da configuracao.
const (
	zonaBarraFora    = 0
	zonaBarraTrilha  = 1
	zonaBarraPolegar = 2
)

// cfgZonaBarra classifica o ponto: fora, trilha ou polegar.
func (a *app) cfgZonaBarra(x, y int32) int {
	if cfgUI.maxScroll <= 0 {
		return zonaBarraFora
	}

	trilho := a.trilhoBarraCfg()
	if !dentroRect(trilho, x, y) {
		return zonaBarraFora
	}

	if dentroRect(a.polegadaBarraCfg(trilho), x, y) {
		return zonaBarraPolegar
	}

	return zonaBarraTrilha
}

// cfgRolagemPonto converte o arrasto em rolagem presa nos limites.
func cfgRolagemPonto(trilhoTopo, viagem, rolagemMax, y, pega int32) int32 {
	if viagem <= 0 || rolagemMax <= 0 {
		return 0
	}

	rol := (y - trilhoTopo - pega) * rolagemMax / viagem
	if rol < 0 {
		return 0
	}

	if rol > rolagemMax {
		return rolagemMax
	}

	return rol
}

// cfgRolarPara prende a rolagem, reposiciona os filhos e repinta.
func (a *app) cfgRolarPara(novo int32) {
	if novo < 0 {
		novo = 0
	}

	if novo > cfgUI.maxScroll {
		novo = cfgUI.maxScroll
	}

	cfgUI.scroll = novo
	a.cfgPosicionarFilhos(cfgUI.navW+a.sc(16), cfgConteudoLarg())
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

// cfgCliqueBarra trata pagina do trilho e inicio do arrasto.
func (a *app) cfgCliqueBarra(zona int, y int32) {
	trilho := a.trilhoBarraCfg()
	polegada := a.polegadaBarraCfg(trilho)

	if zona == zonaBarraPolegar {
		cfgUI.arrastaBarra = true
		cfgUI.pegaBarra = y - polegada.Top
		pSetCapture.Call(cfgUI.hwnd)

		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return
	}

	passo := a.alturaVisivelCfg()
	if passo <= 0 {
		passo = a.sc(56)
	}

	if y < polegada.Top {
		a.cfgRolarPara(cfgUI.scroll - passo)
	} else {
		a.cfgRolarPara(cfgUI.scroll + passo)
	}
}

// cfgArrastarBarra segue o mouse no arrasto do polegar.
func (a *app) cfgArrastarBarra(y int32) {
	trilho := a.trilhoBarraCfg()
	polegada := a.polegadaBarraCfg(trilho)
	viagem := trilho.Height() - polegada.Height()
	a.cfgRolarPara(cfgRolagemPonto(trilho.Top, viagem, cfgUI.maxScroll, y, cfgUI.pegaBarra))
}

// pintarNavegacaoCfgD2D desenha a lista de secoes da configuracao.
func (a *app) pintarNavegacaoCfgD2D(disp *d2dMotor, topo int32) {
	for indice, secao := range cfgSecoes() {
		area := rectT{
			a.sc(10), topo + int32(indice)*a.sc(40), cfgUI.navW - a.sc(10), topo + int32(indice)*a.sc(40) + a.sc(36),
		}
		a.pintarItemNavegacaoCfgD2D(disp, secao, indice, area)
	}
}

// pintarItemNavegacaoCfgD2D desenha uma secao da navegacao.
func (a *app) pintarItemNavegacaoCfgD2D(disp *d2dMotor, secao struct {
	titulo string
	icone  string
}, indice int, area rectT,
) {
	if cfgSecao(indice) == cfgUI.secao {
		disp.redondoB(area, a.sc(8), a.brActive, a.brBorder)
	} else if cfgUI.hoverNav == indice {
		disp.redondoB(area, a.sc(8), a.brHover, 0)
	}

	cor := uint32(corTextoFraco)
	if cfgSecao(indice) == cfgUI.secao || cfgUI.hoverNav == indice {
		cor = corTexto
	}

	disp.icone(secao.icone, rectT{
		area.Left + a.sc(10), area.Top + (area.Height()-a.sc(16))/meioD2D, area.Left + a.sc(26),
		area.Top + (area.Height()+a.sc(16))/meioD2D,
	}, cor, tamIconeD2D)
	disp.textoUI(secao.titulo, rectT{area.Left + a.sc(34), area.Top, area.Right - a.sc(6), area.Bottom}, cor, false, false)
}

// pintarTextosCfgD2D desenha os textos rolaveis da configuracao.
func (a *app) pintarTextosCfgD2D(disp *d2dMotor) {
	for _, texto := range cfgUI.txts {
		posY := texto.r.Top - cfgUI.scroll

		area := rectT{texto.r.Left, posY, texto.r.Right, posY + texto.r.Height()}
		if area.Bottom < cfgUI.headH || area.Top > cfgUI.h {
			continue
		}

		disp.textoFonte(texto.s, area, texto.cor, texto.fonte, texto.flags)
	}
}

// pintarZonasCfgD2D desenha as zonas clicaveis da configuracao.
func (a *app) pintarZonasCfgD2D() {
	for _, zona := range cfgUI.zonas {
		posY := zona.r.Top - cfgUI.scroll

		area := rectT{zona.r.Left, posY, zona.r.Right, posY + zona.r.Height()}
		if area.Bottom < cfgUI.headH || area.Top > cfgUI.h {
			continue
		}

		a.cfgZonaD2D(zona.id, zona.label, area, cfgUI.hoverID == zona.id)
	}
}

// pintarDownsCfgD2D desenha as linhas de download da configuracao.
func (a *app) pintarDownsCfgD2D() {
	for _, linha := range cfgUI.downs {
		a.cfgDownD2D(&linha)
	}
}

// pintarBarrasCfgD2D desenha as barras de progresso da configuracao.
func (a *app) pintarBarrasCfgD2D(disp *d2dMotor) {
	for _, barra := range cfgUI.bars {
		posY := barra.r.Top - cfgUI.scroll

		area := rectT{barra.r.Left, posY, barra.r.Right, posY + barra.r.Height()}
		if area.Bottom < cfgUI.headH || area.Top > cfgUI.h {
			continue
		}

		disp.redondoB(area, a.sc(3), a.brSep, 0)

		if barra.pct > 0 {
			largura := area.Width() * int32(barra.pct) / porcentoD2D
			if largura > area.Width() {
				largura = area.Width()
			}

			parcial := rectT{area.Left, area.Top, area.Left + largura, area.Bottom}
			disp.redondoB(parcial, a.sc(3), a.brAccent, 0)
		}
	}
}

func (a *app) cfgZonaD2D(id, label string, area rectT, hover bool) {
	disp := &d2dCfg

	switch {
	case id == "x":
		a.pintarZonaFecharD2D(disp, area, hover)
	case strings.HasPrefix(id, "ck:x:"):
		a.pintarZonaFecharD2D(disp, area, hover)
	case strings.HasPrefix(id, "chk:"):
		a.pintarZonaCheckD2D(disp, id, label, area, hover)
	case strings.HasPrefix(id, "step:"):
		a.pintarZonaPassoD2D(disp, label, area, hover)
	default:
		a.pintarZonaBotaoD2D(disp, id, label, area, hover)
	}
}

// pintarZonaFecharD2D desenha a zona do botao fechar.
func (a *app) pintarZonaFecharD2D(disp *d2dMotor, area rectT, hover bool) {
	if hover {
		disp.elipse(area, corBordaForte)
	}

	disp.icone("close", area, corTextoFaint, tamIconeD2D)
}

// pintarZonaCheckD2D desenha uma zona de caixa de marcacao.
func (a *app) pintarZonaCheckD2D(disp *d2dMotor, id, label string, area rectT, hover bool) {
	if hover {
		disp.redondoB(area, a.sc(6), a.brHover, 0)
	}

	iniX := area.Left + a.sc(2)
	posY := (area.Top + area.Bottom) / meioD2D
	caixa := rectT{iniX, posY - a.sc(8), iniX + a.sc(16), posY + a.sc(8)}
	disp.retB(caixa, a.brActive)

	if a.cfgCheckOn(id) {
		disp.icone("check", caixa, corAccentSoft, tamIconeD2D)
	}

	disp.textoUI(label, rectT{iniX + a.sc(22), area.Top, area.Right, area.Bottom}, corTexto, false, false)
}

// pintarZonaPassoD2D desenha uma zona de passo numerico.
func (a *app) pintarZonaPassoD2D(disp *d2dMotor, label string, area rectT, hover bool) {
	fundo := a.brActive
	if hover {
		fundo = a.brHover
	}

	disp.redondoB(area, a.sc(6), fundo, a.brBorder)
	disp.textoUI(label, area, corTexto, false, true)
}

// pintarZonaBotaoD2D desenha uma zona de botao generico.
func (a *app) pintarZonaBotaoD2D(disp *d2dMotor, id, label string, area rectT, hover bool) {
	fundo := a.brActive

	borda := a.brBorder
	if strings.HasPrefix(id, "btn:excluirDados") {
		fundo = a.brFechar
		borda = 0
	}

	if hover {
		fundo = a.brHover
	}

	disp.redondoB(area, a.sc(7), fundo, borda)

	cor := a.corTextoZonaBotao(id, hover)
	disp.textoUI(label, area, cor, true, true)
}

// corTextoZonaBotao escolhe a cor do rotulo do botao.
func (a *app) corTextoZonaBotao(id string, hover bool) uint32 {
	if !strings.HasPrefix(id, "down:") {
		return corTexto
	}

	if hover {
		return corTexto
	}

	return corTextoFraco
}

func (a *app) cfgDownD2D(linha *cfgDownRow) {
	disp := &d2dCfg
	descarga := linha.dl
	posY := linha.r.Top - cfgUI.scroll

	area := rectT{linha.r.Left, posY, linha.r.Right, posY + linha.r.Height()}
	if area.Bottom < cfgUI.headH || area.Top > cfgUI.h {
		return
	}

	disp.redondoB(area, a.sc(8), a.brChrome, 0)
	nomeY := area.Top + a.sc(6)
	estado, corEstado := cfgDownEstado(descarga)
	selo := " · " + estado

	disp.textoClip(
		descarga.Nome, rectT{area.Left + a.sc(10), nomeY, area.Right - a.sc(120), nomeY + a.sc(20)},
		corTexto, false)
	disp.textoUI(
		selo, rectT{area.Right - a.sc(116), nomeY, area.Right - a.sc(8), nomeY + a.sc(20)},
		corEstado, true, false)

	meta := cfgTamanho(descarga.Recebido)
	meta = anexarTotalDownD2D(descarga, meta)
	meta = anexarAndamentoDownD2D(descarga, meta)

	if quando := cfgQuando(descarga.Quando); quando != "" {
		meta += " · " + quando
	}

	disp.textoClip(
		meta, rectT{area.Left + a.sc(10), nomeY + a.sc(20), area.Right - a.sc(8), nomeY + a.sc(40)},
		corTextoFraco, true)
}

// anexarTotalDownD2D acrescenta total e percentual ao rodape.
func anexarTotalDownD2D(descarga downloadAtivo, meta string) string {
	if descarga.Total <= 0 {
		return meta
	}

	meta += " de " + cfgTamanho(descarga.Total)

	pct := int(descarga.Recebido * porcentoD2D / descarga.Total)

	return meta + " · " + itoaD2D(int64(pct)) + "%"
}

// anexarAndamentoDownD2D acrescenta velocidade e ETA ao rodape.
func anexarAndamentoDownD2D(descarga downloadAtivo, meta string) string {
	if descarga.Cancelado || descarga.Estado != 0 {
		return meta
	}

	if descarga.Velocidade > 0 {
		meta += " · " + cfgTamanho(descarga.Velocidade) + "/s"
	}

	if descarga.ETA >= 0 {
		meta = anexarETADownD2D(descarga.ETA, meta)
	}

	return meta
}

// anexarETADownD2D formata o tempo restante estimado.
func anexarETADownD2D(eta int64, meta string) string {
	if eta < limiarEtaMinD2D {
		return meta + " · menos de 1 min"
	}

	minutos := eta / segsPorMinD2D
	if minutos < segsPorMinD2D {
		return meta + " · faltam ~" + itoaD2D(minutos) + " min"
	}

	meta += " · faltam ~" + itoaD2D(minutos/segsPorMinD2D) + "h"
	if minutos%segsPorMinD2D != 0 {
		meta += itoaD2D(minutos%segsPorMinD2D) + "min"
	}

	return meta
}

func d2dMotorDe(hwnd uintptr, largura, altura int32) *d2dMotor {
	var disp *d2dMotor

	switch hwnd {
	case menu.hwnd:
		disp = &d2dMenu
	case menuSub.hwnd:
		disp = &d2dSub
	case menuSub2.hwnd:
		disp = &d2dSub2
	case panelMenu.hwnd:
		disp = &d2dPanel
	default:
		return nil
	}

	if disp.rt == nil {
		disp.criar(hwnd, largura, altura)
	}

	if disp.rt == nil {
		return nil
	}

	return disp
}

func (a *app) paintMenuWinD2D(janela *menuWin, principal bool) {
	disp := motorMenuWinD2D(janela)

	area := rectT{0, 0, janela.w, janela.h}
	disp.retB(area, a.brSurface)
	disp.contorno(area, corBordaForte)

	padX := a.sc(16)
	y := janela.padY - janela.scrollY
	sitePainel := a.siteAtualIA().chave

	for indice, item := range janela.items {
		y = a.pintarItemMenuD2D(disp, janela, item, indice, principal, padX, sitePainel, y)
	}

	a.pintarRolagemMenuD2D(disp, janela)
}

// motorMenuWinD2D escolhe o motor da janela do menu.
func motorMenuWinD2D(janela *menuWin) *d2dMotor {
	switch janela.hwnd {
	case menu.hwnd:
		return &d2dMenu
	case menuSub.hwnd:
		return &d2dSub
	case menuSub2.hwnd:
		return &d2dSub2
	default:
		return &d2dPanel
	}
}

// pintarItemMenuD2D desenha uma linha do menu e devolve o proximo y.
func (a *app) pintarItemMenuD2D(
	disp *d2dMotor, janela *menuWin, item menuItem, indice int, principal bool, padX int32, sitePainel string, y int32,
) int32 {
	switch item.kind {
	case miSep:
		return a.pintarSepMenuD2D(disp, janela, y, padX)
	case miHeader:
		return pintarCabecalhoMenuD2D(a, disp, janela, item, y, padX)
	}

	a.destacarItemMenuD2D(disp, janela, indice, principal, y)

	switch item.kind {
	case miZoom:
		return a.pintarZoomMenuD2D(janela, y)
	case miIA:
		return a.pintarIAMenuD2D(disp, janela, item, indice, padX, sitePainel, y)
	case miExt:
		return a.pintarExtMenuD2D(disp, janela, item, indice, principal, padX, y)
	}

	return a.pintarItemComumMenuD2D(disp, janela, item, indice, principal, padX, y)
}

// destacarItemMenuD2D marca o fundo do item em hover.
func (a *app) destacarItemMenuD2D(disp *d2dMotor, janela *menuWin, indice int, principal bool, y int32) {
	if indice == janela.hover || (principal && indice == menu.subPai) {
		disp.redondoB(rectT{a.sc(6), y + a.sc(1), janela.w - a.sc(6), y + janela.itemH - a.sc(1)}, a.sc(9), a.brHover, 0)
	}
}

// pintarSepMenuD2D desenha o separador do menu.
func (a *app) pintarSepMenuD2D(disp *d2dMotor, janela *menuWin, y, padX int32) int32 {
	sep := rectT{padX, y + a.sc(4), janela.w - padX, y + a.sc(4) + 1}
	disp.retB(sep, a.brSep)

	return y + janela.sepY
}

// pintarCabecalhoMenuD2D desenha o cabecalho do menu.
func pintarCabecalhoMenuD2D(a *app, disp *d2dMotor, janela *menuWin, item menuItem, y, padX int32) int32 {
	if janela.headerX {
		return pintarTextoCabecalhoMenuD2D(a, disp, janela, item, y, padX, corTexto, corTextoFraco)
	}

	return pintarTextoCabecalhoMenuD2D(a, disp, janela, item, y, padX, corTextoFraco, corTextoFaint)
}

// pintarTextoCabecalhoMenuD2D desenha rotulo e descricao do cabecalho.
func pintarTextoCabecalhoMenuD2D(
	aplicativo *app, disp *d2dMotor, janela *menuWin, item menuItem, y, padX int32, corRotulo, corDesc uint32,
) int32 {
	posY := y
	if item.desc != "" {
		posY += aplicativo.sc(8)
	}

	disp.textoUI(item.label, rectT{padX, posY, janela.w - padX, posY + janela.headerH}, corRotulo, false, false)

	y = posY + janela.headerH
	if item.desc != "" {
		linha := rectT{padX, y, janela.w - padX, y + aplicativo.sc(20)}
		disp.textoUI(item.desc, linha, corDesc, true, false)

		y += aplicativo.sc(20)
	}

	return y
}

// pintarZoomMenuD2D desenha a linha de zoom do menu.
func (a *app) pintarZoomMenuD2D(janela *menuWin, y int32) int32 {
	a.paintMenuZoomD2D()

	return y + janela.itemH
}

// pintarIAMenuD2D desenha a linha de IA com o radio do painel.
func (a *app) pintarIAMenuD2D(
	disp *d2dMotor, janela *menuWin, item menuItem, indice int, padX int32, sitePainel string, y int32,
) int32 {
	corIcone := uint32(corTextoFaint)
	if indice == janela.hover {
		corIcone = corAccentSoft
	}

	disp.icone(
		idRobot, rectT{padX, y + (janela.itemH-a.sc(16))/meioD2D, padX + a.sc(16), y + (janela.itemH+a.sc(16))/meioD2D},
		corIcone, tamIconeD2D)
	disp.textoClip(item.label, rectT{padX + a.sc(26), y, janela.w - a.sc(84), y + janela.itemH}, corTexto, false)

	painelAtivo := sitePainel == item.id

	painelRect := rectT{janela.w - a.sc(78), y + a.sc(3), janela.w - a.sc(8), y + janela.itemH - a.sc(3)}

	if painelAtivo {
		disp.redondoB(painelRect, a.sc(6), a.brActive, 0)
	}

	a.pintarRadioIAMenuD2D(disp, painelRect, painelAtivo)

	return y + janela.itemH
}

// pintarRadioIAMenuD2D desenha o radio do painel de IA.
func (a *app) pintarRadioIAMenuD2D(disp *d2dMotor, painelRect rectT, painelAtivo bool) {
	corPainel := uint32(corCiano)
	idRadio := idRadioOff

	if painelAtivo {
		corPainel = corAccentSoft
		idRadio = idRadioOn
	}

	diametro := painelRect.Height() - a.sc(4)
	if diametro < a.sc(12) {
		diametro = a.sc(12)
	}

	posX := (painelRect.Left + painelRect.Right) / meioD2D
	posY := (painelRect.Top + painelRect.Bottom) / meioD2D
	disp.icone(
		idRadio, rectT{posX - diametro/meioD2D, posY - diametro/meioD2D, posX + diametro/meioD2D, posY + diametro/meioD2D},
		corPainel, tamIconeD2D)
}

// pintarExtMenuD2D desenha a linha de extensao com fixar.
func (a *app) pintarExtMenuD2D(
	disp *d2dMotor, janela *menuWin, item menuItem, indice int, principal bool, padX, y int32,
) int32 {
	corLinha := uint32(corTextoFaint)
	if indice == janela.hover || (principal && indice == menu.subPai) {
		corLinha = corAccentSoft
	}

	a.pintarIconeExtMenuD2D(disp, janela, item, padX, y, corLinha)
	a.pintarRotuloExtMenuD2D(disp, janela, item, padX, y)
	a.pintarPinoExtMenuD2D(disp, janela, item, indice, y)

	return y + janela.itemH
}

// pintarIconeExtMenuD2D desenha o icone da linha de extensao.
func (a *app) pintarIconeExtMenuD2D(disp *d2dMotor, janela *menuWin, item menuItem, padX, y int32, corLinha uint32) {
	if a.temIconeExt(item.iconeBase) {
		lado := a.sc(16)
		posX := padX + a.sc(8)
		posY := y + janela.itemH/meioD2D
		disp.bmpExt(item.iconeBase, rectT{posX - lado/meioD2D, posY - lado/meioD2D, posX + lado/meioD2D, posY + lado/meioD2D})

		return
	}

	disp.icone(
		"puzzle", rectT{padX, y + (janela.itemH-a.sc(16))/meioD2D, padX + a.sc(16), y + (janela.itemH+a.sc(16))/meioD2D},
		corLinha, tamIconeD2D)
}

// pintarRotuloExtMenuD2D desenha rotulo e dica da extensao.
func (a *app) pintarRotuloExtMenuD2D(disp *d2dMotor, janela *menuWin, item menuItem, padX, y int32) {
	larguraHint := int32(0)
	if item.hint != "" {
		larguraHint = a.sc(88)
	}

	disp.textoClip(
		item.label, rectT{padX + a.sc(26), y, janela.w - padX - a.sc(34) - larguraHint, y + janela.itemH},
		corTexto, false)

	if item.hint != "" {
		disp.textoUIR(
			item.hint, rectT{janela.w - padX - a.sc(34) - a.sc(88), y, janela.w - padX - a.sc(34), y + janela.itemH},
			corTextoFaint, true)
	}
}

// pintarPinoExtMenuD2D desenha o fixador da linha de extensao.
func (a *app) pintarPinoExtMenuD2D(disp *d2dMotor, janela *menuWin, item menuItem, indice int, y int32) {
	pino := a.extRowZona(janela, y)

	corPino := uint32(corTextoFaint)
	if a.extFixada(item.ext) || indice == janela.hover {
		corPino = corAccentSoft
	}

	if janela.hoverZona == 1 && indice == janela.hover {
		disp.elipse(pino, corBordaForte)
	}

	disp.icone(idPin, a.centroZona(pino), corPino, tamIconeD2D)
}

// pintarItemComumMenuD2D desenha linha comum com icone e dica.
func (a *app) pintarItemComumMenuD2D(
	disp *d2dMotor, janela *menuWin, item menuItem, indice int, principal bool, padX, y int32,
) int32 {
	corIcone := uint32(corTextoFaint)
	if indice == janela.hover || (principal && indice == menu.subPai) {
		corIcone = corAccentSoft
	}

	iconeArea := rectT{padX, y + (janela.itemH-a.sc(16))/meioD2D, padX + a.sc(16), y + (janela.itemH+a.sc(16))/meioD2D}
	a.pintarIconeItemMenuD2D(disp, janela, item, iconeArea, y, corIcone)

	largura := a.sc(64)
	disp.textoClip(item.label, rectT{padX + a.sc(26), y, janela.w - padX - largura, y + janela.itemH}, corTexto, false)

	if item.kind == miSub {
		disp.icone(
			idChevronRight, rectT{janela.w - a.sc(24), y, janela.w - a.sc(8), y + janela.itemH},
			corTextoFaint, tamIconeD2D)
	} else if item.hint != "" {
		disp.textoUIR(item.hint, rectT{janela.w - a.sc(88), y, janela.w - padX, y + janela.itemH}, corTextoFaint, true)
	}

	return y + janela.itemH
}

// pintarIconeItemMenuD2D escolhe o icone da linha comum.
func (a *app) pintarIconeItemMenuD2D(
	disp *d2dMotor, janela *menuWin, item menuItem, iconeArea rectT, y int32, corIcone uint32,
) {
	switch {
	case item.kind == miCheck:
		if item.check {
			disp.icone("check", iconeArea, corIcone, tamIconeD2D)
		} else {
			disp.navIconeAuto(iconeDoItem(item), iconeArea, corIcone)
		}
	case item.kind == miRadio:
		idRadio := idRadioOff
		if item.check {
			idRadio = idRadioOn
		}

		disp.icone(idRadio, iconeArea, corIcone, tamIconeD2D)
	case item.iconeBase != "":
		a.pintarIconeBaseMenuD2D(disp, item, iconeArea, y, janela, corIcone)
	default:
		disp.navIconeAuto(iconeDoItem(item), iconeArea, corIcone)
	}
}

// pintarIconeBaseMenuD2D desenha o icone de extensao ou o quebra-cabeca.
func (a *app) pintarIconeBaseMenuD2D(
	disp *d2dMotor, item menuItem, iconeArea rectT, y int32, janela *menuWin, corIcone uint32,
) {
	padX := iconeArea.Left

	if a.temIconeExt(item.iconeBase) {
		lado := a.sc(16)
		posX := padX + a.sc(8)
		posY := y + janela.itemH/meioD2D
		disp.bmpExt(item.iconeBase, rectT{posX - lado/meioD2D, posY - lado/meioD2D, posX + lado/meioD2D, posY + lado/meioD2D})

		return
	}

	disp.icone("puzzle", iconeArea, corIcone, tamIconeD2D)
}

// pintarRolagemMenuD2D desenha a barra de rolagem do menu.
func (a *app) pintarRolagemMenuD2D(disp *d2dMotor, janela *menuWin) {
	if janela.contentH <= janela.h {
		return
	}

	trilho := rectT{janela.w - a.sc(3), a.sc(3), janela.w - a.sc(2), janela.h - a.sc(3)}
	disp.retB(trilho, a.brSep)

	polegada := int32(int64(janela.h) * int64(janela.h) / int64(janela.contentH))
	if minPolegada := a.sc(24); polegada < minPolegada {
		polegada = minPolegada
	}

	viagem := trilho.Height() - polegada
	rolagemMax := janela.contentH - janela.h

	posPolegada := trilho.Top
	if viagem > 0 && rolagemMax > 0 {
		posPolegada += int32(int64(viagem) * int64(janela.scrollY) / int64(rolagemMax))
	}

	polegadaArea := rectT{trilho.Left, posPolegada, trilho.Right, posPolegada + polegada}
	disp.retB(polegadaArea, a.brHover)
}

func (a *app) paintMenuZoomD2D() {
	disp := &d2dMenu
	corZona := func(tipo int) uint32 {
		if menu.hoverZoom == tipo {
			return corTexto
		}

		return corTextoFraco
	}
	alvo := rectT{}

	switch menu.hoverZoom {
	case 1:
		alvo = menu.zMinus
	case zonaZoomPctD2D:
		alvo = menu.zPct
	case zonaZoomMaisD2D:
		alvo = menu.zPlus
	case zonaZoomAjusteD2D:
		alvo = menu.zFit
	}

	if alvo.Width() > 0 {
		destaque := rectT{alvo.Left - a.sc(3), alvo.Top + a.sc(1), alvo.Right + a.sc(3), alvo.Bottom - a.sc(1)}
		disp.redondoB(destaque, a.sc(7), a.brHover, 0)
	}

	disp.navIcone(idZoom, menu.zIcon, corTextoFraco)
	disp.textoUI(txtZoomRotulo, menu.zLabel, corTexto, false, false)
	disp.icone("remove", menu.zMinus, corZona(1), tamIconeD2D)
	disp.icone("add", menu.zPlus, corZona(zonaZoomMaisD2D), tamIconeD2D)
	disp.textoUI(fmt.Sprintf(txtFmtZoomPct, a.st.Zoom*porcentoD2D), menu.zPct, corZona(zonaZoomPctD2D), false, true)
	disp.retB(menu.zSep, a.brSep)
	disp.navIcone(idExpand, menu.zFit, corZona(zonaZoomAjusteD2D))
}

func (d *d2dMotor) textoClip(texto string, area rectT, cor uint32, pequeno bool) {
	d.clip(area)
	d.textoUI(texto, area, cor, pequeno, false)
	d.desclip()
}

func (a *app) paintFaixaD2D() {
	disp := &d2dMain

	if a.tabH <= 0 {
		return
	}

	recorte := a.clientRect()
	fundo := rectT{0, 0, recorte.Right, a.tabH}
	disp.retB(fundo, a.brChrome)

	limitePintura := recorte.Right - a.sc(3*46) - a.winClock.Width() - a.sc(6)
	if a.tabOverflowRect.Width() > 0 {
		limitePintura = a.tabOverflowRect.Left - a.sc(2)
	}

	a.tabLimitePintura = limitePintura
	for indice, aba := range a.tabs {
		if indice >= len(a.tabRects) {
			break
		}

		if aba == nil {
			continue
		}

		area := a.tabRects[indice]
		if area.Width() <= 0 || area.Right > limitePintura {
			continue
		}

		a.pintarAbaFaixaD2D(disp, aba, area, indice, indice == a.ativa)
	}

	a.pintarDivisoresFaixaD2D(disp)
	a.pintarArrastoFaixaD2D(disp)
	a.pintarExtrasFaixaD2D(disp, recorte)
}

// pintarAbaFaixaD2D desenha uma aba na faixa de abas.
func (a *app) pintarAbaFaixaD2D(disp *d2dMotor, aba *tab, area rectT, indice int, ativo bool) {
	hover := a.tabHover == indice
	a.pintarFundoAbaD2D(disp, area, ativo, hover)

	cor := uint32(corCiano)
	if ativo || hover {
		cor = corTexto
	}

	titulo := a.tituloDaAba(aba)

	areaLeft, areaRight := a.bordasTituloAbaD2D(disp, aba, area, indice, cor)

	if areaRight < areaLeft {
		areaRight = areaLeft
	}

	a.pintarTituloAbaD2D(disp, aba, titulo, areaLeft, areaRight, area, cor)

	if ativo {
		marco := rectT{area.Left + a.sc(11), area.Bottom - a.sc(3), area.Right - a.sc(11), area.Bottom - a.sc(1)}
		disp.redondo(marco, a.sc(1), corAccentSoft, 0)
	}
}

// pintarFundoAbaD2D desenha o fundo da aba ativa ou em hover.
func (a *app) pintarFundoAbaD2D(disp *d2dMotor, area rectT, ativo, hover bool) {
	if ativo {
		disp.redondoB(area, a.sc(9), a.brActive, a.brBorder)
	} else if hover {
		disp.redondoB(area, a.sc(9), a.brHover, 0)
	}
}

// limitarDireitaTitulo prende a area do titulo no icone mais a esquerda:
// o fechar fica a direita do audio e nao pode reexpandir a area.
func limitarDireitaTitulo(areaRight, corte int32) int32 {
	if corte < areaRight {
		return corte
	}

	return areaRight
}

// bordasTituloAbaD2D desconta pino, audio e fechar da area do titulo.
func (a *app) bordasTituloAbaD2D(disp *d2dMotor, aba *tab, area rectT, indice int, _ uint32) (int32, int32) {
	areaLeft := area.Left + a.sc(11)
	areaRight := area.Right - a.sc(8)

	if aba.fixa {
		pino := rectT{area.Left + a.sc(6), area.Top + a.sc(2), area.Left + a.sc(22), area.Bottom - a.sc(2)}
		disp.navIcone("pinned", pino, corAccentSoft)
		areaLeft = pino.Right + a.sc(1)
	}

	if audio := a.tabAudioRects[indice]; audio.Width() > 0 {
		areaRight = limitarDireitaTitulo(areaRight, a.pintarAudioAbaD2D(disp, aba, audio, indice, areaRight))
	}

	if fechar := a.tabCloseRects[indice]; fechar.Width() > 0 && a.fecharAbaVisivel(indice, area) {
		areaRight = limitarDireitaTitulo(areaRight, a.pintarFecharAbaD2D(disp, fechar, indice, areaRight))
	}

	return areaLeft, areaRight
}

// pintarAudioAbaD2D desenha o indicador de audio da aba.
func (a *app) pintarAudioAbaD2D(disp *d2dMotor, aba *tab, audio rectT, indice int, _ int32) int32 {
	audioCor := uint32(corCiano)
	if a.tabHoverAudio == indice {
		audioCor = corTexto
	}

	id := idAudio
	if aba.pane != nil && aba.pane.mudo {
		id = idMute
	}

	disp.navIcone(id, audio, audioCor)

	return audio.Left - a.sc(2)
}

// fecharAbaVisivel diz se o X da aba aparece no estado atual.
func (a *app) fecharAbaVisivel(indice int, area rectT) bool {
	ativo := indice == a.ativa
	hover := a.tabHover == indice

	return a.tabHoverClose == indice ||
		(ativo && area.Width() >= a.sc(112)) || (hover && area.Width() >= a.sc(92))
}

// pintarFecharAbaD2D desenha o X da aba.
func (a *app) pintarFecharAbaD2D(disp *d2dMotor, fechar rectT, indice int, _ int32) int32 {
	corFecha := uint32(corCiano)

	if a.tabHoverClose == indice {
		disp.redondoB(fechar, a.sc(6), a.brHover, 0)

		corFecha = corTexto
	}

	disp.icone("close", fechar, corFecha, tamIconeD2D)

	return fechar.Left - a.sc(2)
}

// pintarTituloAbaD2D desenha globo ou titulo conforme o espaco.
func (a *app) pintarTituloAbaD2D(
	disp *d2dMotor, aba *tab, titulo string, areaLeft, areaRight int32, area rectT, cor uint32,
) {
	if areaRight-areaLeft < a.sc(64) {
		if !aba.fixa {
			disp.navIcone(idGlobe, rectT{areaLeft, area.Top + a.sc(4), areaRight, area.Bottom - a.sc(4)}, cor)
		}

		return
	}

	disp.textoClip(titulo, rectT{areaLeft, area.Top, areaRight, area.Bottom}, cor, false)
}

// pintarDivisoresFaixaD2D desenha os separadores entre abas.
func (a *app) pintarDivisoresFaixaD2D(disp *d2dMotor) {
	for indice := 0; indice+1 < len(a.tabs) && indice+1 < len(a.tabRects); indice++ {
		if indice == a.ativa || indice+1 == a.ativa || a.tabHover == indice || a.tabHover == indice+1 {
			continue
		}

		area := a.tabRects[indice]

		proxima := a.tabRects[indice+1]
		if area.Width() <= 0 || proxima.Width() <= 0 {
			continue
		}

		posX := proxima.Left - a.sc(1)
		divisor := rectT{posX, area.Top + a.sc(9), proxima.Left, area.Bottom - a.sc(9)}
		disp.retB(divisor, a.brBorder)
	}
}

// pintarArrastoFaixaD2D desenha o indicador de soltar a aba.
func (a *app) pintarArrastoFaixaD2D(disp *d2dMotor) {
	if a.tabDrag < 0 || !a.tabDragAtivo || a.tabDrop < 0 {
		return
	}

	posX := a.posicaoSoltaAba()
	if posX <= 0 {
		return
	}

	linha := rectT{posX - a.sc(1), a.sc(4), posX + a.sc(1), a.tabH - a.sc(4)}
	disp.retB(linha, a.brAccent)
}

// posicaoSoltaAba calcula o X do indicador de soltura.
func (a *app) posicaoSoltaAba() int32 {
	if a.tabDrop >= len(a.tabRects) {
		if len(a.tabRects) == 0 {
			return 0
		}

		return a.tabRects[len(a.tabRects)-1].Right
	}

	if a.tabDrop >= 0 {
		return a.tabRects[a.tabDrop].Left
	}

	return 0
}

// pintarExtrasFaixaD2D desenha nova aba, overflow, relogio e rodape.
func (a *app) pintarExtrasFaixaD2D(disp *d2dMotor, recorte rectT) {
	if botao := a.tabNewRect; botao.Width() > 0 && a.tabOverflowN == 0 {
		a.pintarNovaAbaFaixaD2D(disp, botao)
	}

	if overflow := a.tabOverflowRect; overflow.Width() > 0 {
		if a.hover == "tab:overflow" {
			disp.redondoB(overflow, a.sc(8), a.brHover, 0)
		}

		disp.icone("overflow", overflow, corCiano, tamIconeD2D)
	}

	if relogio := a.winClock; relogio.Width() > a.sc(20) && a.relogioTxt != "" {
		disp.textoUI(a.relogioTxt, relogio, uint32(corCiano), false, true)
	}

	a.paintBotoesJanelaD2D()
	faixa := rectT{0, a.tabH - 1, recorte.Right, a.tabH}
	disp.retB(faixa, a.brSep)
}

// pintarNovaAbaFaixaD2D desenha o botao de nova aba.
func (a *app) pintarNovaAbaFaixaD2D(disp *d2dMotor, area rectT) {
	if a.tabHover == -2 {
		disp.redondoB(area, a.sc(8), a.brHover, 0)
	}

	coluna := uint32(corCiano)
	if a.tabHover == -2 {
		coluna = corTexto
	}

	disp.icone(idPlus, area, coluna, tamIconeD2D)
}

func (a *app) paintBotoesJanelaD2D() {
	disp := &d2dMain

	maximizada := false
	if estado, _, _ := pIsZoomed.Call(a.hwnd); estado != 0 {
		maximizada = true
	}

	desenha := func(area rectT, qual int) {
		if area.Width() <= 0 {
			return
		}

		cantos := rectT{area.Left + a.sc(4), area.Top + a.sc(5), area.Right - a.sc(4), area.Bottom - a.sc(5)}
		if a.winHover == qual {
			fundo := a.brHover
			if qual == winFecha {
				fundo = a.brFechar
			}

			disp.redondoB(cantos, a.sc(7), fundo, 0)
		}

		corBotao := uint32(corCiano)
		if a.winHover == qual {
			corBotao = corTexto
		}

		id := "winmin"

		switch qual {
		case winMaximiza:
			id = "winmax"
			if maximizada {
				id = "winrestore"
			}
		case winFecha:
			id = "winclose"
		}

		disp.icone(id, area, corBotao, tamIconeD2D)
	}
	desenha(a.winMin, winMinimiza)
	desenha(a.winMax, winMaximiza)
	desenha(a.winClose, winFecha)
}

func (a *app) paintD2D() {
	disp := &d2dMain

	recorte := a.clientRect()
	// Modo Marble Overdrive: o canvas espacial sempre pinta. Os early-returns
	// do painel cheio/soIA do OVERDRIVE ficam desligados (codigo preservado abaixo).
	// if a.soIA {
	// 	a.drawSplitButtonD2D()
	// 	return
	// }
	// if a.painelCheioAtivo() {
	// 	a.paintEspacoAppsD2D(recorte)
	// 	a.drawLeftAuxButtonD2D()
	// 	return
	// }

	_ = recorte
	// a.paintFaixaD2D()
	// fundoBarra := rectT{0, a.tabH, recorte.Right, a.tabH + a.tbHeight}
	// disp.retB(fundoBarra, a.brChrome)

	// a.pintarFundoBotoesD2D(disp)
	// a.pintarIconesBotoesD2D(disp)
	// a.pintarQuadroEnderecoD2D(disp)
	// a.pintarBotaoUboD2D(disp)
	// a.pintarBotoesNoQuadroD2D(disp)
	// a.pintarBotoesLeituraD2D(disp)
	// a.pintarTextoEnderecoD2D(disp)
	// a.pintarSeloSegurancaD2D(disp)
	// a.pintarDivisoresRodapeD2D(disp, recorte)

	// a.paintEspacoAppsD2D(recorte)
	// a.drawLeftAuxButtonD2D()
	// a.drawSplitButtonD2D()
	// a.drawImgToggleD2D()

	espPintar(disp, a)
}

// pularBotaoBarraD2D diz se o botao sai da barra principal.
func (a *app) pularBotaoBarraD2D(botao tbutton) bool {
	if ocultoBarraD2D(botao) {
		return true
	}

	if a.uboBtn.rect.Width() > 0 && botao.id == a.uboBtn.id {
		return true
	}

	// Print, historico e ferramentas moram dentro do quadro do endereco e tem
	// pintura dedicada depois dele: o preenchimento do quadro cobriria o fundo
	// e o icone da passada generica.
	if botao.id == idFerramentas || botao.id == idImprimir || botao.id == idClipboard {
		return true
	}

	return botao.rect.Width() <= 0
}

// ocultoBarraD2D diz se o botao sai da barra principal.
func ocultoBarraD2D(botao tbutton) bool {
	return botao.id == idReader || botao.id == idPaywall || botao.id == idEngine || botao.id == idSeguranca ||
		botao.id == "tr" || botao.id == idNojs || botao.id == idSleep || botao.id == idVPN || botao.id == idCookies
}

// pintarFundoBotoesD2D desenha o fundo dos botoes da barra.
func (a *app) pintarFundoBotoesD2D(disp *d2dMotor) {
	for _, botao := range a.buttons {
		if a.pularBotaoBarraD2D(botao) {
			continue
		}

		cantos := rectT{
			botao.rect.Left + a.sc(1), botao.rect.Top + a.sc(1),
			botao.rect.Right - a.sc(1), botao.rect.Bottom - a.sc(1),
		}
		if a.hover == botao.id && !botao.on {
			disp.redondoB(cantos, a.sc(8), a.brHover, 0)
		} else if botao.on {
			disp.redondoB(cantos, a.sc(8), a.brActive, a.brBorder)
		}
	}
}

// pintarIconesBotoesD2D desenha os icones dos botoes da barra.
func (a *app) pintarIconesBotoesD2D(disp *d2dMotor) {
	for _, botao := range a.buttons {
		if a.pularBotaoBarraD2D(botao) {
			continue
		}

		a.pintarIconeBotaoBarraD2D(disp, botao)
	}
}

// pintarIconeBotaoBarraD2D desenha o icone de um botao da barra.
func (a *app) pintarIconeBotaoBarraD2D(disp *d2dMotor, botao tbutton) {
	ativo, coluna := a.corBotaoBarraD2D(botao)

	switch {
	case botao.id == idMenu:
		disp.icone("more", botao.rect, coluna, tamIconeD2D)
	case botao.id == idExtensoes:
		disp.icone("puzzle", botao.rect, coluna, tamIconeD2D)
	case strings.HasPrefix(botao.id, "ext:"):
		a.pintarBotaoExtBarraD2D(disp, botao, coluna)
	default:
		a.pintarIconeNativoBarraD2D(disp, botao, coluna, ativo)
	}
}

// corBotaoBarraD2D escolhe estado e cor do botao da barra.
func (a *app) corBotaoBarraD2D(botao tbutton) (bool, uint32) {
	ativo := botao.on
	if botao.id == idExtensoes {
		ativo = a.extBtn.on
	}

	coluna := uint32(corCiano)
	if ativo {
		coluna = corAccentSoft
	} else if a.hover == botao.id {
		coluna = corTexto
	}

	return ativo, coluna
}

// pintarBotaoExtBarraD2D desenha botao de extensao da barra.
func (a *app) pintarBotaoExtBarraD2D(disp *d2dMotor, botao tbutton, coluna uint32) {
	lado := a.sc(16)
	posX := (botao.rect.Left + botao.rect.Right) / meioD2D
	posY := (botao.rect.Top + botao.rect.Bottom) / meioD2D

	caixa := rectT{posX - lado/meioD2D, posY - lado/meioD2D, posX + lado/meioD2D, posY + lado/meioD2D}
	if a.temIconeExt(strings.TrimPrefix(botao.id, "ext:")) {
		disp.bmpExt(strings.TrimPrefix(botao.id, "ext:"), caixa)

		return
	}

	disp.icone("puzzle", botao.rect, coluna, tamIconeD2D)
}

// pintarIconeNativoBarraD2D desenha icone nativo ou rotulo do botao.
func (a *app) pintarIconeNativoBarraD2D(disp *d2dMotor, botao tbutton, coluna uint32, _ bool) {
	iconID := botao.id
	if botao.id == idImprimir {
		iconID = idCamera
	}

	if botao.id == idRecarregar && a.loading {
		iconID = idParar
	}

	if !disp.navIconeAuto(iconID, botao.rect, coluna) {
		disp.textoUI(botao.label, botao.rect, coluna, false, true)
	}
}

func (a *app) pintarQuadroEnderecoD2D(disp *d2dMotor) {
	if a.addrFrame.Width() <= meioD2D {
		return
	}

	quadro := a.addrFrame
	raio := a.sc(11)

	var anel uint32 = corBorda
	if a.urlEditing {
		anel = corAccentSoft
	}

	disp.redondo(quadro, raio, anel, 0)
	interno := rectT{quadro.Left + 1, quadro.Top + 1, quadro.Right - 1, quadro.Bottom - 1}
	disp.redondo(interno, raio, corSuperficie, 0)

	if a.loading {
		a.pintarCargaEnderecoD2D(disp, quadro)
	}

	if pct, total, pronto := a.progressoDownloads(); pronto && total > 0 {
		a.pintarProgressoEnderecoD2D(disp, quadro, pct)
	}
}

// pintarCargaEnderecoD2D desenha a barra de carregamento do endereco.
func (a *app) pintarCargaEnderecoD2D(disp *d2dMotor, quadro rectT) {
	largura := quadro.Right - quadro.Left - a.sc(12)
	if largura <= a.sc(40) {
		return
	}

	bloco := largura / blocosCargaD2D
	posX := quadro.Left + a.sc(6) + (a.loadPhase%blocosCargaD2D)*bloco
	barra := rectT{posX, quadro.Bottom - a.sc(3), posX + bloco, quadro.Bottom - a.sc(1)}
	disp.ret(barra, corAccentSoft)
}

// pintarProgressoEnderecoD2D desenha o progresso de download no endereco.
func (a *app) pintarProgressoEnderecoD2D(disp *d2dMotor, quadro rectT, pct int) {
	largura := quadro.Right - quadro.Left - a.sc(12)
	if largura <= a.sc(40) {
		return
	}

	faixa := rectT{quadro.Left + a.sc(6), quadro.Bottom - a.sc(6), quadro.Right - a.sc(6), quadro.Bottom - a.sc(4)}
	if pct < 0 {
		disp.ret(faixa, corTextoFraco)

		return
	}

	parcial := rectT{faixa.Left, faixa.Top, faixa.Left + int32(int64(faixa.Width())*int64(pct)/porcentoD2D), faixa.Bottom}
	disp.ret(parcial, corTextoFraco)
}

// pintarBotaoUboD2D desenha o botao da extensao fixada na barra.
func (a *app) pintarBotaoUboD2D(disp *d2dMotor) {
	if a.uboBtn.rect.Width() <= 0 {
		return
	}

	botao := a.uboBtn
	base := strings.TrimPrefix(botao.id, "ext:")
	ativo := a.extFloatOn && a.pastaDoFloat() == base

	cantos := rectT{
		botao.rect.Left + a.sc(1), botao.rect.Top + a.sc(1),
		botao.rect.Right - a.sc(1), botao.rect.Bottom - a.sc(1),
	}
	if ativo {
		disp.redondoB(cantos, a.sc(8), a.brActive, a.brBorder)
	} else if a.hover == botao.id {
		disp.redondoB(cantos, a.sc(8), a.brHover, 0)
	}

	coluna := uint32(corCiano)
	if ativo {
		coluna = corAccentSoft
	} else if a.hover == botao.id {
		coluna = corTexto
	}

	lado := a.sc(16)
	posX := (botao.rect.Left + botao.rect.Right) / meioD2D
	posY := (botao.rect.Top + botao.rect.Bottom) / meioD2D

	caixa := rectT{posX - lado/meioD2D, posY - lado/meioD2D, posX + lado/meioD2D, posY + lado/meioD2D}
	if a.temIconeExt(base) {
		disp.bmpExt(base, caixa)
	} else {
		disp.icone(idUbo, botao.rect, coluna, tamIconeD2D)
	}
}

// pintarBotoesNoQuadroD2D desenha print, historico e ferramentas dentro do
// quadro do endereco. Precisa vir depois do preenchimento do quadro, que cobre
// a passada generica (mesmo motivo do uBO ter pintura dedicada).
func (a *app) pintarBotoesNoQuadroD2D(disp *d2dMotor) {
	for _, botao := range []tbutton{a.printBtn, a.clipBtn, a.toolsBtn} {
		a.pintarBotaoNoQuadroD2D(disp, botao)
	}
}

// pintarBotaoNoQuadroD2D desenha fundo e icone de um botao da barra.
func (a *app) pintarBotaoNoQuadroD2D(disp *d2dMotor, botao tbutton) {
	if botao.rect.Width() <= 0 {
		return
	}

	ativo := botao.on

	coluna := uint32(corCiano)
	if ativo {
		coluna = corAccentSoft
	} else if a.hover == botao.id {
		coluna = corTexto
	}

	cantos := rectT{
		botao.rect.Left + a.sc(1), botao.rect.Top + a.sc(1),
		botao.rect.Right - a.sc(1), botao.rect.Bottom - a.sc(1),
	}
	if ativo {
		disp.redondoB(cantos, a.sc(8), a.brActive, a.brBorder)
	} else if a.hover == botao.id {
		disp.redondoB(cantos, a.sc(8), a.brHover, 0)
	}

	a.pintarIconeNativoBarraD2D(disp, botao, coluna, ativo)
}

// pintarBotoesLeituraD2D desenha os botoes de leitura e acoes da pagina.
func (a *app) pintarBotoesLeituraD2D(disp *d2dMotor) {
	for _, botao := range []tbutton{a.readerBtn, a.readerCopyBtn, a.paywallBtn, a.trBtn, a.cookiesBtn} {
		a.pintarUmBotaoLeituraD2D(disp, botao)
	}
}

// pintarUmBotaoLeituraD2D desenha um botao de leitura da pagina.
func (a *app) pintarUmBotaoLeituraD2D(disp *d2dMotor, botao tbutton) {
	if botao.rect.Width() <= 0 {
		return
	}

	ativo := a.ligadoBotaoLeituraD2D(botao)

	cantos := rectT{
		botao.rect.Left + a.sc(1), botao.rect.Top + a.sc(1),
		botao.rect.Right - a.sc(1), botao.rect.Bottom - a.sc(1),
	}
	if ativo {
		disp.redondoB(cantos, a.sc(8), a.brActive, a.brBorder)
	} else if a.hover == botao.id {
		disp.redondoB(cantos, a.sc(8), a.brHover, 0)
	}

	disp.icone(botao.id, botao.rect, a.corBotaoLeituraD2D(botao, ativo), tamIconeD2D)
}

// ligadoBotaoLeituraD2D diz se o botao de leitura esta ativo.
func (a *app) ligadoBotaoLeituraD2D(botao tbutton) bool {
	switch botao.id {
	case idReader:
		return a.left != nil && a.left.readerAtivo
	case "tr":
		return a.traduzido
	case idPaywall:
		return a.menuOrigem == idPaywall
	}

	return false
}

// corBotaoLeituraD2D escolhe a cor do botao de leitura.
func (a *app) corBotaoLeituraD2D(botao tbutton, ativo bool) uint32 {
	if botao.id == idReader && a.readerUsavel && (a.left == nil || !a.left.readerAtivo) {
		return corAccentSoft
	}

	if ativo {
		return corAccentSoft
	}

	if a.hover == botao.id {
		return corTexto
	}

	return corCiano
}

// pintarTextoEnderecoD2D desenha o texto do campo de endereco.
func (a *app) pintarTextoEnderecoD2D(disp *d2dMotor) {
	if a.urlEdit == 0 || a.urlEditing || a.addrFrame.Width() <= a.sc(20) {
		return
	}

	texto := strings.TrimSpace(a.editText())
	pad := a.sc(4)

	areaLeft := a.addrFrame.Left + a.sc(6)
	if a.editRect.Width() > 0 {
		areaLeft = a.editRect.Left + a.sc(8)
	}

	areaRight := a.addrFrame.Right - pad
	if a.editRect.Width() > 0 {
		areaRight = a.editRect.Right - a.sc(6)
	}

	area := rectT{areaLeft, a.addrFrame.Top, areaRight, a.addrFrame.Bottom}
	if texto == "" {
		disp.textoClip(txtPlaceholderAddr, area, corTextoFaint, false)
	} else {
		disp.textoClip(texto, area, corTexto, false)
	}
}

// pintarSeloSegurancaD2D desenha seguranca, motor e selo privado.
func (a *app) pintarSeloSegurancaD2D(disp *d2dMotor) {
	if a.securityBtn.rect.Width() > 0 {
		segColuna := uint32(corCiano)
		if a.hover == idSeguranca {
			segColuna = corTexto
		}

		disp.navIcone(a.iconeSeguranca(), a.securityBtn.rect, segColuna)
	}

	if a.engineBtn.rect.Width() > 0 {
		a.pintarBotaoMotorD2D(disp)
	}

	if a.privadoBadge.Width() > 0 {
		disp.redondoB(a.privadoBadge, a.sc(7), a.brActive, a.brBorder)
		disp.textoUI(txtBadgePrivado, a.privadoBadge, corAccentSoft, false, true)
	}
}

// pintarBotaoMotorD2D desenha o selo do motor de busca atual.
func (a *app) pintarBotaoMotorD2D(disp *d2dMotor) {
	engColuna := uint32(corCiano)
	if a.hover == idEngine {
		engColuna = corTexto
	}

	motor := a.motorAtual()
	emblema := a.engineBtn.rect
	emblema.Right -= a.sc(12)
	disp.textoUI(motor.badge, emblema, engColuna, false, true)

	posX := a.engineBtn.rect.Right - a.sc(10)
	posY := (a.engineBtn.rect.Top + a.engineBtn.rect.Bottom) / meioD2D
	disp.icone(
		"chevrondown", rectT{posX - a.sc(8), posY - a.sc(8), posX + a.sc(8), posY + a.sc(8)},
		engColuna, tamIconeD2D)
}

// pintarDivisoresRodapeD2D desenha as divisorias abaixo da barra.
func (a *app) pintarDivisoresRodapeD2D(disp *d2dMotor, recorte rectT) {
	faixa := rectT{0, a.tabH + a.tbHeight, recorte.Right, a.tabH + a.tbHeight + 1}
	disp.retB(faixa, a.brSep)

	if a.leftAuxSplitX > 0 {
		meia := a.rail / meioD2D
		if a.leftAux == nil || a.leftAux.rect.Width() <= 0 {
			meia = a.sc(railFinoLogico) / meioD2D
		}

		divisor := rectT{a.leftAuxSplitX - meia, a.tabH + a.tbHeight + 1, a.leftAuxSplitX + meia, recorte.Bottom}
		disp.retB(divisor, a.brBg)
		linha := rectT{a.leftAuxSplitX, a.tabH + a.tbHeight + 1, a.leftAuxSplitX + 1, recorte.Bottom}
		disp.retB(linha, a.brSep)
	}

	if a.st.RightVisible {
		a.pintarDivisorDireitoD2D(disp, recorte)
	} else {
		fino := a.sc(railFinoLogico)
		lateral := rectT{recorte.Right - fino, a.tabH + a.tbHeight + 1, recorte.Right, recorte.Bottom}
		disp.retB(lateral, a.brBg)

		centroX := recorte.Right - fino/meioD2D
		linha := rectT{centroX, a.tabH + a.tbHeight + 1, centroX + 1, recorte.Bottom}
		disp.retB(linha, a.brSep)
	}
}

// pintarDivisorDireitoD2D desenha as divisorias do painel direito.
func (a *app) pintarDivisorDireitoD2D(disp *d2dMotor, recorte rectT) {
	lateral := rectT{a.splitX - a.rail/meioD2D, a.tabH + a.tbHeight + 1, a.splitX + a.rail/meioD2D, recorte.Bottom}
	disp.retB(lateral, a.brBg)
	linha := rectT{a.splitX, a.tabH + a.tbHeight + 1, a.splitX + 1, recorte.Bottom}
	disp.retB(linha, a.brSep)

	if a.imgSplitY > 0 {
		barra := rectT{a.splitX + a.rail/meioD2D, a.imgSplitY - a.rail/meioD2D, recorte.Right, a.imgSplitY + a.rail/meioD2D}
		disp.retB(barra, a.brBg)
		traco := rectT{barra.Left, a.imgSplitY, recorte.Right, a.imgSplitY + 1}
		disp.retB(traco, a.brSep)
	}
}

func (d *d2dMotor) navIconeAuto(id string, area rectT, cor uint32) bool {
	if iconesFluent[id] != 0 {
		if theApp == nil {
			return false
		}

		d.icone(id, area, cor, tamIconeD2D)

		return true
	}

	if glifo := glifoNav(id); glifo != "" {
		if theApp == nil {
			return false
		}

		d.texto(glifo, area, cor, "Segoe UI", theApp.sc(tamIconeD2D), 1, 1, 1)

		return true
	}

	return false
}

func (a *app) drawSplitButtonD2D() {
	disp := &d2dMain

	botao := a.splitBtn
	if botao.Width() <= 0 {
		return
	}

	fundo := a.brSurface
	if a.hover == idSplit {
		fundo = a.brActive
	}

	disp.redondoB(botao, a.sc(9), fundo, 0)

	a.pintarBotoesSplitD2D(disp)

	posX := (botao.Left + botao.Right) / meioD2D
	recorte := a.clientRect()

	topo := a.tabH + a.tbHeight + 1
	if a.soIA {
		topo = 0
	}

	posY := topo + (recorte.Bottom-topo)/meioD2D

	id := idChevronLeft
	if a.st.RightVisible {
		id = idChevronRight
	}

	disp.icone(id, rectT{posX - a.sc(8), posY - a.sc(8), posX + a.sc(8), posY + a.sc(8)}, corTextoFraco, tamIconeD2D)
}

// pintarBotoesSplitD2D desenha os botoes internos da barra dividida.
func (a *app) pintarBotoesSplitD2D(disp *d2dMotor) {
	for _, botao := range []struct {
		id, icon string
		area     rectT
	}{
		{idPainelMenu, idPanels, a.panelMenuBtn},
		{idPainelTranslate, idTranslate, a.panelTranslateBtn},
		{idPainelSend, idSend, a.panelSendBtn},
		{idChatHome, idHome, a.chatHomeBtn},
		{idPainelReload, idRecarregar, a.panelReloadBtn},
		{idPainelFull, idExpand, a.panelFullBtn},
	} {
		a.pintarUmBotaoSplitD2D(disp, botao.id, botao.icon, botao.area)
	}
}

// pintarUmBotaoSplitD2D desenha um botao da barra dividida.
func (a *app) pintarUmBotaoSplitD2D(disp *d2dMotor, id, icon string, area rectT) {
	if area.Width() <= 0 {
		return
	}

	disp.navIcone(
		icon, rectT{area.Left + a.sc(1), area.Top + a.sc(4), area.Right - a.sc(1), area.Bottom - a.sc(4)},
		a.corBotaoSplitD2D(id))
}

// corBotaoSplitD2D escolhe a cor do botao da barra dividida.
func (a *app) corBotaoSplitD2D(id string) uint32 {
	if a.hover == id || (id == idPainelMenu && panelMenu.open) || (id == idPainelFull && a.soIA) {
		return corTexto
	}

	return corCiano
}

func (a *app) drawLeftAuxButtonD2D() {
	disp := &d2dMain

	botao := a.leftAuxBtn
	if botao.Width() <= 0 {
		return
	}

	fundo := a.brSurface
	if a.hover == idLeftAux || a.st.LeftPanelVisible {
		fundo = a.brActive
	}

	disp.redondoB(botao, a.sc(7), fundo, 0)
	posX := (botao.Left + botao.Right) / meioD2D
	posY := (botao.Top + botao.Bottom) / meioD2D

	id := idChevronRight
	if a.st.LeftPanelVisible {
		id = idChevronLeft
	}

	disp.icone(id, rectT{posX - a.sc(8), posY - a.sc(8), posX + a.sc(8), posY + a.sc(8)}, corTextoFraco, tamIconeD2D)
	a.drawLeftAuxSmallButtonD2D(a.leftAuxFullBtn, idLeftAuxFull, idExpand)
}

func (a *app) drawLeftAuxSmallButtonD2D(area rectT, hoverID, icon string) {
	disp := &d2dMain

	if area.Width() <= 0 {
		return
	}

	fundo := a.brSurface
	if a.hover == hoverID {
		fundo = a.brActive
	}

	disp.redondoB(area, a.sc(7), fundo, 0)

	coluna := uint32(corCiano)
	if a.hover == hoverID {
		coluna = corTexto
	}

	interno := rectT{area.Left + a.sc(3), area.Top + a.sc(3), area.Right - a.sc(3), area.Bottom - a.sc(3)}
	disp.navIcone(icon, interno, coluna)
}

func (a *app) drawImgToggleD2D() {
	disp := &d2dMain

	botao := a.imgToggleBtn
	if botao.Width() <= 0 {
		return
	}

	fundo := a.brSurface
	if a.hover == idImgToggle {
		fundo = a.brActive
	}

	disp.redondoB(botao, a.sc(7), fundo, 0)
	posX := (botao.Left + botao.Right) / meioD2D
	posY := (botao.Top + botao.Bottom) / meioD2D

	id := "chevrondown"
	if a.st.ImgVisible {
		id = "chevronup"
	}

	disp.icone(id, rectT{posX - a.sc(8), posY - a.sc(8), posX + a.sc(8), posY + a.sc(8)}, corTextoFraco, tamIconeD2D)
}

func (a *app) paintClipboardD2D() {
	disp := &d2dClip
	largura, altura := clipUI.w, clipUI.h
	fundo := rectT{0, 0, largura, altura}
	disp.retB(fundo, a.brBg)
	disp.contorno(fundo, corBordaForte)

	topo := clipUI.headerH + a.sc(4)

	base := clipUI.h - clipUI.footerH
	a.pintarCorpoClipboardD2D(disp, largura, altura, topo, base)

	rodape := rectT{0, altura - clipUI.footerH, largura, altura}
	disp.retB(rodape, a.brChrome)

	linhaRodape := rectT{0, altura - clipUI.footerH, largura, altura - clipUI.footerH + 1}
	disp.retB(linhaRodape, a.brSep)

	dica := txtClipDica
	disp.textoUI(
		dica, rectT{a.sc(14), altura - clipUI.footerH, largura - a.sc(14), altura - a.sc(4)},
		corTextoFaint, true, true)

	a.pintarCabecalhoClipboardD2D(disp, largura)
	a.pintarAcoesClipboardD2D(disp)

	sep := rectT{0, clipUI.headerH, largura, clipUI.headerH + 1}
	disp.retB(sep, a.brSep)
}

// pintarCorpoClipboardD2D desenha vazio ou linhas com rolagem.
func (a *app) pintarCorpoClipboardD2D(disp *d2dMotor, largura, _, topo, base int32) {
	if len(clipHist) == 0 {
		meio := (topo + base) / meioD2D
		disp.icone(
			idClipboard, rectT{largura/meioD2D - a.sc(16), meio - a.sc(74), largura/meioD2D + a.sc(16), meio - a.sc(42)},
			corTextoFaint, tamChipClipD2D)
		disp.textoUI(
			txtClipVazio, rectT{a.sc(18), meio - a.sc(30), largura - a.sc(18), meio - a.sc(6)},
			corTexto, false, true)
		disp.textoUI(
			txtClipComoUsar, rectT{a.sc(18), meio - a.sc(6), largura - a.sc(18), meio + a.sc(16)},
			corTextoFraco, true, true)
		disp.textoUI(
			txtClipMemoria, rectT{a.sc(18), meio + a.sc(18), largura - a.sc(18), meio + a.sc(40)},
			corTextoFaint, true, true)

		return
	}

	hoverIdx := -1
	if clipUI.hover >= 0 && clipUI.hover < len(clipUI.rows) {
		hoverIdx = clipUI.rows[clipUI.hover].idx
	}

	for _, linha := range clipUI.rows {
		a.paintClipLinhaD2D(linha, linha.idx == hoverIdx)
	}

	a.pintarRolagemClipboardD2D(disp, largura, topo, base)
}

// pintarRolagemClipboardD2D desenha a barra de rolagem da lista.
func (a *app) pintarRolagemClipboardD2D(disp *d2dMotor, largura, topo, base int32) {
	if clipUI.total <= base-topo {
		return
	}

	trilho := rectT{largura - a.sc(6), topo, largura - a.sc(4), base}
	disp.retB(trilho, a.brSep)

	prop := float32(base-topo) / float32(clipUI.total)

	polegada := int32(float32(base-topo) * prop)
	if polegada < a.sc(20) {
		polegada = a.sc(20)
	}

	faixa := clipUI.total - (base - topo)

	var posPolegada int32
	if faixa > 0 {
		posPolegada = topo + int32(float32(base-topo-polegada)*float32(clipUI.scroll)/float32(faixa))
	}

	polegadaArea := rectT{trilho.Left, posPolegada, trilho.Right, posPolegada + polegada}
	disp.retB(polegadaArea, a.brIcon)
}

// pintarCabecalhoClipboardD2D desenha titulo e contador do clipboard.
func (a *app) pintarCabecalhoClipboardD2D(disp *d2dMotor, largura int32) {
	cabecalho := rectT{0, 0, largura, clipUI.headerH + a.sc(4)}
	disp.retB(cabecalho, a.brChrome)

	titulo := txtClipTitulo

	disp.icone(idClipboard, rectT{a.sc(16), a.sc(18), a.sc(32), a.sc(34)}, corAccentSoft, tamIconeD2D)
	disp.textoUI(titulo, rectT{a.sc(38), a.sc(12), largura - a.sc(220), a.sc(42)}, corTexto, false, false)

	chip := clipUI.contar
	disp.redondoB(chip, a.sc(7), a.brSurface, a.brBorder)
	disp.textoUI(a.rotuloContadorClipboard(), chip, corTextoFraco, true, true)
}

// rotuloContadorClipboard escolhe o texto do contador de itens.
func (a *app) rotuloContadorClipboard() string {
	if len(clipHist) == 0 {
		return txtClipVazioChip
	}

	if len(clipUI.marcados) == len(clipHist) {
		return txtClipTudoMarcado
	}

	return quant(len(clipHist), "item", "itens")
}

// pintarAcoesClipboardD2D desenha os botoes enviar e limpar.
func (a *app) pintarAcoesClipboardD2D(disp *d2dMotor) {
	if len(clipHist) == 0 {
		return
	}

	enviar := clipUI.enviar

	fundoEnv := a.brSurface
	if clipUI.hoverEnv {
		fundoEnv = a.brHover
	}

	disp.redondoB(enviar, a.sc(7), fundoEnv, a.brBorder)
	disp.icone(idSend, enviar, corAccentSoft, tamIconeD2D)

	textoLimpar := txtClipLimpar
	if clipUI.confirmar {
		textoLimpar = txtClipConfirmar
	}

	limpar := clipUI.limpar

	fundo := a.brSurface
	if clipUI.hoverLim {
		fundo = a.brHover
	}

	disp.redondoB(limpar, a.sc(7), fundo, a.brBorder)
	disp.textoUI(textoLimpar, limpar, corTexto, true, true)
}

func (a *app) paintClipLinhaD2D(linha clipLinha, hover bool) {
	disp := &d2dClip
	item := clipHist[linha.idx]
	area := linha.r
	sel := clipUI.sel == linha.idx
	fundo := a.brSurface
	borda := uint32(0)

	if hover || sel {
		fundo = a.brHover
		borda = a.brBorder
	}

	disp.redondoB(area, a.sc(10), fundo, borda)

	if item.fixo {
		barra := rectT{area.Left + a.sc(1), area.Top + a.sc(10), area.Left + a.sc(3), area.Bottom - a.sc(10)}
		disp.redondoB(barra, a.sc(1), a.brAccent, 0)
	}

	disp.textoUIR(
		a.metaLinhaClip(item),
		rectT{area.Right - a.sc(210), area.Top + a.sc(8), linha.check.Left - a.sc(2), area.Top + a.sc(26)},
		corTextoFaint, true)

	if item.tipo == clipImagem {
		a.pintarImagemLinhaClip(disp, item, area, linha)
	} else {
		a.pintarTextoLinhaClip(disp, item, area, linha)
	}

	a.pintarMarcasLinhaClip(disp, item, linha)
}

// metaLinhaClip monta o rotulo tipo + horario da linha.
func (a *app) metaLinhaClip(item clipItem) string {
	return a.tipoLinhaClip(item) + "  ·  " + item.quando.Format("15:04")
}

// tipoLinhaClip escolhe o rotulo do tipo do item.
func (a *app) tipoLinhaClip(item clipItem) string {
	switch item.tipo {
	case clipTexto:
		return txtClipTipoTexto
	case clipArquivos:
		if len(strings.Split(item.texto, "\n")) > 1 {
			return txtClipTipoArqs
		}

		return txtClipTipoArq
	case clipImagem:
		return txtClipTipoImg
	}

	return txtClipTipoTexto
}

// pintarImagemLinhaClip desenha a miniatura de imagem da linha.
func (a *app) pintarImagemLinhaClip(disp *d2dMotor, item clipItem, area rectT, linha clipLinha) {
	bitmap := d2dClipImagem(item)
	posX := area.Left + a.sc(14)
	posY := area.Top + a.sc(28)
	maxW := a.sc(130)
	maxH := area.Height() - a.sc(40)

	largMini, altMini := a.tamanhoMiniaturaClip(item, maxW, maxH)

	destino := rectT{posX, posY + (maxH-altMini)/meioD2D, posX + largMini, posY + (maxH-altMini)/meioD2D + altMini}
	disp.contorno(destino, corBordaForte)

	if bitmap != nil {
		origem := d2dRet(destino)
		disp.rt.DrawBitmap(bitmap, &origem, 1, direct2d.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)
	}

	disp.textoUI(fmt.Sprintf("Imagem %d x %d", item.w, item.h),
		rectT{posX + a.sc(144), area.Top + a.sc(38), linha.check.Left - a.sc(4), area.Top + a.sc(60)}, corTexto, false, false)
}

// tamanhoMiniaturaClip ajusta a miniatura a caixa maxima.
func (a *app) tamanhoMiniaturaClip(item clipItem, maxW, maxH int32) (int32, int32) {
	largura, altura := maxW, maxH
	if larguraItem, alturaItem := item.w, item.h; larguraItem > 0 && alturaItem > 0 {
		if int64(larguraItem)*int64(maxH) > int64(alturaItem)*int64(maxW) {
			altura = int32(int64(alturaItem) * int64(maxW) / int64(larguraItem))
		} else {
			largura = int32(int64(larguraItem) * int64(maxH) / int64(alturaItem))
		}
	}

	if largura < a.sc(8) {
		largura = a.sc(8)
	}

	if altura < a.sc(8) {
		altura = a.sc(8)
	}

	return largura, altura
}

// larguraPreviaClip limita a previa para nao invadir o rotulo tipo+hora.
func (a *app) larguraPreviaClip(area rectT, checkLeft int32) int32 {
	fimTexto := checkLeft - a.sc(26)
	if inicioMeta := area.Right - a.sc(210); inicioMeta-a.sc(8) < fimTexto {
		fimTexto = inicioMeta - a.sc(8)
	}

	largura := fimTexto - (area.Left + a.sc(14))
	if largura < a.sc(40) {
		return a.sc(40)
	}

	return largura
}

// pintarTextoLinhaClip desenha a previa de texto ou arquivos da linha.
func (a *app) pintarTextoLinhaClip(disp *d2dMotor, item clipItem, area rectT, linha clipLinha) {
	resumo := a.previaTextoClip(item)

	largMax := a.larguraPreviaClip(area, linha.check.Left)
	fragmentos := disp.quebrar(resumo, a.fontUI, largMax, linhasPrevClipD2D)

	if len(fragmentos) == 0 {
		return
	}

	alturaTexto := a.sc(20) * int32(len(fragmentos))
	caixa := rectT{
		area.Left + a.sc(14), area.Top + a.sc(13),
		area.Left + a.sc(14) + largMax, area.Top + a.sc(13) + alturaTexto,
	}
	disp.clip(caixa)

	posY := area.Top + a.sc(13)

	for indice, fragmento := range fragmentos {
		cor := uint32(corTexto)
		if indice > 0 {
			cor = corTextoFraco
		}

		disp.textoUI(
			fragmento, rectT{area.Left + a.sc(14), posY, area.Left + a.sc(14) + largMax, posY + a.sc(20)},
			cor, false, false)
		posY += a.sc(20)
	}

	disp.desclip()
}

// previaTextoClip resume o texto ou lista de arquivos da linha.
func (a *app) previaTextoClip(item clipItem) string {
	if item.tipo != clipArquivos {
		return item.texto
	}

	partes := strings.Split(item.texto, "\n")

	nome := partes[0]
	if indice := strings.LastIndexAny(nome, "\\/"); indice >= 0 && indice+1 < len(nome) {
		nome = nome[indice+1:]
	}

	if len(partes) <= 1 {
		return nome
	}

	extra := "arquivos"
	if len(partes)-1 == 1 {
		extra = "arquivo"
	}

	return fmt.Sprintf(txtFmtClipPrev, nome, len(partes)-1, extra)
}

// pintarMarcasLinhaClip desenha fixar, excluir e marcar da linha.
func (a *app) pintarMarcasLinhaClip(disp *d2dMotor, item clipItem, linha clipLinha) {
	pinCor := uint32(corTextoFaint)
	pinID := idPin

	if item.fixo {
		pinCor = corAccentSoft
		pinID = "pinned"
	}

	disp.icone(pinID, linha.pin, pinCor, tamIconeD2D)
	disp.icone(idDelete, linha.del, corTextoFaint, tamIconeD2D)

	quadro := rectT{
		linha.check.Left + a.sc(4), linha.check.Top + a.sc(4),
		linha.check.Right - a.sc(4), linha.check.Bottom - a.sc(4),
	}
	if clipUI.marcados[item.chave] {
		disp.redondoB(quadro, a.sc(4), a.brActive, a.brBorder)
		disp.icone("check", quadro, corAccentSoft, tamIconeD2D)
	} else {
		disp.contorno(quadro, corTextoFaint)
	}
}

func d2dClipImagem(item clipItem) *direct2d.ID2D1Bitmap {
	if item.chave == 0 || item.dib == nil {
		return nil
	}

	d2dClipImgsMu.Lock()
	if bitmap, ok := d2dClipImgs[item.chave]; ok {
		d2dClipImgsMu.Unlock()

		return bitmap
	}
	d2dClipImgsMu.Unlock()

	largura, altura, desloc, pronto := clipDIBInfo(item.dib)
	if !pronto || largura <= 0 || altura <= 0 {
		return nil
	}

	passo := ((int(largura)*bytesPorPixelD2D + alinhamentoStrideD2D - 1) / alinhamentoStrideD2D) * alinhamentoStrideD2D
	plano := make([]byte, int(altura)*passo)
	copy(plano, item.dib[desloc:])

	props := direct2d.D2D1_BITMAP_PROPERTIES{
		PixelFormat: common.D2D1_PIXEL_FORMAT{
			Format:    dxgicommon.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: common.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX: dpiPadraoD2D,
		DpiY: dpiPadraoD2D,
	}
	tamanho := common.D2D_SIZE_U{Width: uint32(largura), Height: uint32(altura)}

	var bitmap *direct2d.ID2D1Bitmap
	if err := d2dClip.rt.CreateBitmap(tamanho, unsafe.Pointer(&plano[0]), uint32(passo), &props, &bitmap); err != nil ||
		bitmap == nil {
		return nil
	}

	d2dClipImgsMu.Lock()
	if atual, ok := d2dClipImgs[item.chave]; ok {
		d2dClipImgsMu.Unlock()
		bitmap.Release()

		return atual
	}

	d2dClipImgs[item.chave] = bitmap
	d2dClipImgsMu.Unlock()

	return bitmap
}

func (d *d2dMotor) bmpBitmap(base string) (*direct2d.ID2D1Bitmap, error) {
	pixels, err := d2dPixelsIcone(base)
	if err != nil {
		return nil, err
	}

	props := direct2d.D2D1_BITMAP_PROPERTIES{
		PixelFormat: common.D2D1_PIXEL_FORMAT{
			Format:    dxgicommon.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: common.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX: dpiPadraoD2D,
		DpiY: dpiPadraoD2D,
	}
	lado := pixels.lado
	tamanho := common.D2D_SIZE_U{Width: uint32(lado), Height: uint32(lado)}

	var bitmap *direct2d.ID2D1Bitmap
	if err := d.rt.CreateBitmap(
		tamanho, pixels.p, uint32(lado)*bytesPorPixelD2D, &props, &bitmap,
	); err != nil || bitmap == nil {
		return nil, erroTexto("sem bitmap")
	}

	return bitmap, nil
}
