//go:build windows

package main

import (
	"unsafe"

	rtwin "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d/common"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/directwrite"
	dxgicommon "github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/dxgi/common"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/gdi"
)

type motorD2D struct {
	alvo         *direct2d.ID2D1DCRenderTarget
	fabricaTexto *directwrite.IDWriteFactory
	formato      *directwrite.IDWriteTextFormat
	formatoTit   *directwrite.IDWriteTextFormat
	pinceis      map[uint32]*direct2d.ID2D1Brush
	hdcMem       uintptr
	hdcTela      uintptr
	hbitmap      uintptr
	bitmapAntigo uintptr
	largura      int32
	altura       int32
}

func corD2D(cor uint32) common.D2D1_COLOR_F {
	alfa := float32(1)
	if canalAlfa := (cor >> deslocAlfa) & mascaraCanal; canalAlfa != 0 {
		alfa = float32(canalAlfa) / alfaOpaco
	}

	return common.D2D1_COLOR_F{
		R: float32(cor&mascaraCanal) / alfaOpaco,
		G: float32((cor>>deslocVerde)&mascaraCanal) / alfaOpaco,
		B: float32((cor>>deslocAzul)&mascaraCanal) / alfaOpaco,
		A: alfa,
	}
}

func areaD2D(area rectT) common.D2D_RECT_F {
	return common.D2D_RECT_F{
		Left:   float32(area.Left),
		Top:    float32(area.Top),
		Right:  float32(area.Right),
		Bottom: float32(area.Bottom),
	}
}

func (m *motorD2D) criarDIB(apiLocal *api, largura, altura int32) bool {
	m.hdcTela, _, _ = apiLocal.pCreateCompatibleDC.Call(0)
	if m.hdcTela == 0 {
		return false
	}

	m.hdcMem, _, _ = apiLocal.pCreateCompatibleDC.Call(m.hdcTela)
	if m.hdcMem == 0 {
		return false
	}

	var cabecalhoVazio cabecalhoBitmap

	info := infoBitmap{Header: cabecalhoBitmap{
		Size:          uint32(unsafe.Sizeof(cabecalhoVazio)),
		Width:         largura,
		Height:        altura,
		Planes:        planoUnico,
		BitCount:      bitsPorPixel,
		Compression:   0,
		SizeImage:     0,
		XPelsPerMeter: 0,
		YPelsPerMeter: 0,
		ClrUsed:       0,
		ClrImportant:  0,
	}}

	var bits unsafe.Pointer

	m.hbitmap, _, _ = apiLocal.pCreateDIBSection.Call(
		m.hdcMem,
		uintptr(unsafe.Pointer(&info)),
		dibRgbColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if m.hbitmap == 0 {
		return false
	}

	m.bitmapAntigo, _, _ = apiLocal.pSelectObject.Call(m.hdcMem, m.hbitmap)

	return true
}

func (m *motorD2D) criarAlvo(largura, altura int32) bool {
	var desconhecido *rtwin.IUnknown
	if err := direct2d.D2D1CreateFactory(
		direct2d.D2D1_FACTORY_TYPE_SINGLE_THREADED,
		&direct2d.IID_ID2D1Factory,
		nil,
		&desconhecido,
	); err != nil || desconhecido == nil {
		return false
	}

	fabrica := (*direct2d.ID2D1Factory)(unsafe.Pointer(desconhecido))

	props := direct2d.D2D1_RENDER_TARGET_PROPERTIES{
		Type: direct2d.D2D1_RENDER_TARGET_TYPE_DEFAULT,
		PixelFormat: common.D2D1_PIXEL_FORMAT{
			Format:    dxgicommon.DXGI_FORMAT_B8G8R8A8_UNORM,
			AlphaMode: common.D2D1_ALPHA_MODE_PREMULTIPLIED,
		},
		DpiX:     0,
		DpiY:     0,
		Usage:    direct2d.D2D1_RENDER_TARGET_USAGE_GDI_COMPATIBLE,
		MinLevel: direct2d.D2D1_FEATURE_LEVEL_DEFAULT,
	}

	var alvo *direct2d.ID2D1DCRenderTarget
	if err := fabrica.CreateDCRenderTarget(&props, &alvo); err != nil || alvo == nil {
		fabrica.Release()

		return false
	}

	area := foundation.RECT{Left: 0, Top: 0, Right: largura, Bottom: altura}
	if err := alvo.BindDC(gdi.HDC(m.hdcMem), &area); err != nil {
		alvo.Release()
		fabrica.Release()

		return false
	}

	fabrica.Release()

	m.alvo = alvo

	return true
}

func (m *motorD2D) criar(apiLocal *api, handle uintptr, largura, altura int32) bool {
	_ = handle
	m.largura = largura
	m.altura = altura

	if !m.criarDIB(apiLocal, largura, altura) {
		return false
	}

	return m.criarAlvo(largura, altura)
}

func (m *motorD2D) pincel(cor uint32) *direct2d.ID2D1Brush {
	if m.alvo == nil {
		return nil
	}

	if m.pinceis == nil {
		m.pinceis = map[uint32]*direct2d.ID2D1Brush{}
	}

	if pincel, ok := m.pinceis[cor]; ok {
		return pincel
	}

	var solido *direct2d.ID2D1SolidColorBrush

	corF := corD2D(cor)
	if err := m.alvo.CreateSolidColorBrush(&corF, nil, &solido); err != nil || solido == nil {
		return nil
	}

	pincel := &solido.ID2D1Brush
	m.pinceis[cor] = pincel

	return pincel
}

func (m *motorD2D) garantirFabricaTexto() bool {
	if m.fabricaTexto != nil {
		return true
	}

	var desconhecido *rtwin.IUnknown
	if err := directwrite.DWriteCreateFactory(
		directwrite.DWRITE_FACTORY_TYPE_SHARED,
		&directwrite.IID_IDWriteFactory,
		&desconhecido,
	); err != nil || desconhecido == nil {
		return false
	}

	m.fabricaTexto = (*directwrite.IDWriteFactory)(unsafe.Pointer(desconhecido))

	return true
}

func (m *motorD2D) criarFormato(titulo bool) *directwrite.IDWriteTextFormat {
	if !m.garantirFabricaTexto() {
		return nil
	}

	tam := float32(fonteCorpo)
	face := "Segoe UI"

	if titulo {
		tam = fonteTitulo
		face = "Segoe UI Semibold"
	}

	var formato *directwrite.IDWriteTextFormat
	if err := m.fabricaTexto.CreateTextFormat(
		face, nil,
		directwrite.DWRITE_FONT_WEIGHT_NORMAL,
		directwrite.DWRITE_FONT_STYLE_NORMAL,
		directwrite.DWRITE_FONT_STRETCH_NORMAL,
		tam, "pt-BR", &formato,
	); err != nil || formato == nil {
		return nil
	}

	_ = formato.SetTextAlignment(directwrite.DWRITE_TEXT_ALIGNMENT_LEADING)
	_ = formato.SetParagraphAlignment(directwrite.DWRITE_PARAGRAPH_ALIGNMENT_CENTER)

	return formato
}

func (m *motorD2D) formatoTexto(titulo bool) *directwrite.IDWriteTextFormat {
	if titulo {
		if m.formatoTit == nil {
			m.formatoTit = m.criarFormato(true)
		}

		return m.formatoTit
	}

	if m.formato == nil {
		m.formato = m.criarFormato(false)
	}

	return m.formato
}

func (m *motorD2D) texto(texto string, area rectT, cor uint32, titulo bool) {
	formato := m.formatoTexto(titulo)

	pincel := m.pincel(cor)
	if formato == nil || pincel == nil || texto == "" {
		return
	}

	areaF := areaD2D(area)
	quantos := len([]rune(texto))
	m.alvo.DrawText(
		texto, uint32(quantos), formato, &areaF, pincel,
		direct2d.D2D1_DRAW_TEXT_OPTIONS_NONE,
		directwrite.DWRITE_MEASURING_MODE_NATURAL,
	)
}

func (m *motorD2D) retangulo(area rectT, cor uint32) {
	pincel := m.pincel(cor)
	if pincel == nil {
		return
	}

	areaF := areaD2D(area)
	m.alvo.FillRectangle(&areaF, pincel)
}

func (m *motorD2D) cartao(area rectT, raio float32, preenchimento, borda uint32) {
	redondo := direct2d.D2D1_ROUNDED_RECT{
		Rect: areaD2D(area), RadiusX: raio, RadiusY: raio,
	}

	if pincel := m.pincel(preenchimento); pincel != nil {
		m.alvo.FillRoundedRectangle(&redondo, pincel)
	}

	if pincel := m.pincel(borda); pincel != nil {
		m.alvo.DrawRoundedRectangle(&redondo, pincel, espessuraBorda, nil)
	}
}

func (m *motorD2D) linha(inicio, fim pontoT, cor uint32, espessura float32) {
	pincel := m.pincel(cor)
	if pincel == nil {
		return
	}

	p1 := common.D2D_POINT_2F{X: float32(inicio.X), Y: float32(inicio.Y)}
	p2 := common.D2D_POINT_2F{X: float32(fim.X), Y: float32(fim.Y)}
	m.alvo.DrawLine(p1, p2, pincel, espessura, nil)
}

func (m *motorD2D) iniciar() {
	m.alvo.BeginDraw()

	transparente := common.D2D1_COLOR_F{R: 0, G: 0, B: 0, A: 0}
	m.alvo.Clear(&transparente)
}

func (m *motorD2D) apresentar(apiLocal *api, handle uintptr) {
	var inicio, fim uint64
	if err := m.alvo.EndDraw(&inicio, &fim); err != nil {
		return
	}

	tamanho := tamanhoT{CX: m.largura, CY: m.altura}
	origem := pontoT{X: 0, Y: 0}
	blend := blendT{
		BlendOp:             acSrcOver,
		BlendFlags:          0,
		SourceConstantAlpha: alfaOpaco,
		AlphaFormat:         acSrcAlpha,
	}

	apiLocal.pUpdateLayeredWindow.Call(
		handle,
		0,
		0,
		uintptr(unsafe.Pointer(&tamanho)),
		m.hdcMem,
		uintptr(unsafe.Pointer(&origem)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlpha,
	)
}

func (m *motorD2D) libertarTexto() {
	if m.formato != nil {
		m.formato.Release()
		m.formato = nil
	}

	if m.formatoTit != nil {
		m.formatoTit.Release()
		m.formatoTit = nil
	}

	if m.fabricaTexto != nil {
		m.fabricaTexto.Release()
		m.fabricaTexto = nil
	}
}

func (m *motorD2D) libertarPinceis() {
	for cor, pincel := range m.pinceis {
		if pincel != nil {
			pincel.Release()
		}

		delete(m.pinceis, cor)
	}
}

func (m *motorD2D) libertar(apiLocal *api) {
	m.libertarTexto()
	m.libertarPinceis()

	if m.alvo != nil {
		m.alvo.Release()
		m.alvo = nil
	}

	if m.bitmapAntigo != 0 {
		apiLocal.pSelectObject.Call(m.hdcMem, m.bitmapAntigo)
		m.bitmapAntigo = 0
	}

	if m.hbitmap != 0 {
		apiLocal.pDeleteObject.Call(m.hbitmap)
		m.hbitmap = 0
	}

	if m.hdcMem != 0 {
		apiLocal.pDeleteDC.Call(m.hdcMem)
		m.hdcMem = 0
	}

	if m.hdcTela != 0 {
		apiLocal.pDeleteDC.Call(m.hdcTela)
		m.hdcTela = 0
	}
}
