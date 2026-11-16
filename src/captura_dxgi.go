//go:build windows

package main

import (
	"unsafe"

	"github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct3d"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct3d11"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/dxgi"
	dxgicommon "github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/dxgi/common"
	systemcom "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
)

func dxgiMedirTela() (int32, int32, int32, int32, bool) {
	var origemX, origemY, largura, altura int32

	vxr, _, _ := pGetSystemMetrics.Call(smXVirtualScreen)
	vyr, _, _ := pGetSystemMetrics.Call(smYVirtualScreen)
	wr, _, _ := pGetSystemMetrics.Call(smCxVirtualScreen)
	hr, _, _ := pGetSystemMetrics.Call(smCyVirtualScreen)
	origemX, origemY = int32(vxr), int32(vyr)

	largura, altura = int32(wr), int32(hr)
	if largura <= 0 || altura <= 0 {
		return 0, 0, 0, 0, false
	}

	return origemX, origemY, largura, altura, true
}

func dxgiCriarFabrica() (*dxgi.IDXGIFactory1, bool) {
	var unk *win32.IUnknown
	if e := dxgi.CreateDXGIFactory1(&dxgi.IID_IDXGIFactory1, &unk); e != nil || unk == nil {
		return nil, false
	}

	factory := (*dxgi.IDXGIFactory1)(unsafe.Pointer(unk))

	return factory, true
}

const (
	bytesPorPixelDxgi     = 4
	esperaFrameDxgiMs     = 1000
	limiteAdaptadoresDxgi = 4
	alfaOpacoDxgi         = 0xFF
	rotacaoIdentidadeDxgi = 1
	rotacao90Dxgi         = 2
	rotacao180Dxgi        = 3
	rotacao270Dxgi        = 4
)

func dxgiVarrerAdaptadores(factory *dxgi.IDXGIFactory1, pix []byte, origemX, origemY, largura int32) int {
	vivos := 0

	for idxAdaptador := uint32(0); ; idxAdaptador++ {
		var adaptador *dxgi.IDXGIAdapter1
		if e := factory.EnumAdapters1(idxAdaptador, &adaptador); e != nil || adaptador == nil {
			break
		}

		vivos += dxgiFotografarAdaptador(adaptador, pix, origemX, origemY, largura)
		adaptador.Release()

		if vivos > 0 && idxAdaptador > limiteAdaptadoresDxgi {
			break
		}
	}

	return vivos
}

func dxgiFotografarTela() ([]byte, int32, int32, int32, int32, error) {
	origemX, origemY, largura, altura, telaOk := dxgiMedirTela()
	if !telaOk {
		return nil, 0, 0, 0, 0, erroTexto("tela indisponivel")
	}

	if pixelsCapturaExcedem(largura, altura) {
		return nil, 0, 0, 0, 0, erroTexto("tela grande demais")
	}

	pix := make([]byte, int(largura)*int(altura)*bytesPorPixelDxgi)

	factory, fabricaOk := dxgiCriarFabrica()
	if !fabricaOk {
		return nil, 0, 0, 0, 0, erroTexto("sem DXGI")
	}
	defer factory.Release()

	if dxgiVarrerAdaptadores(factory, pix, origemX, origemY, largura) == 0 {
		return nil, 0, 0, 0, 0, erroTexto("duplicacao indisponivel")
	}

	return pix, origemX, origemY, largura, altura, nil
}

func dxgiFotografarAdaptador(adaptador *dxgi.IDXGIAdapter1, pix []byte, origemX, origemY, largura int32) int {
	var (
		disp *direct3d11.ID3D11Device
		ctx  *direct3d11.ID3D11DeviceContext
	)
	// Adaptador explicito exige UNKNOWN; HARDWARE aqui da E_INVALIDARG em
	// drivers estritos (ex. Intel UHD) e a duplicacao nunca abre.
	if e := direct3d11.D3D11CreateDevice(
		&adaptador.IDXGIAdapter, direct3d.D3D_DRIVER_TYPE_UNKNOWN, 0, 0, nil,
		direct3d11.D3D11_SDK_VERSION, &disp, nil, &ctx,
	); e != nil || disp == nil || ctx == nil {
		if disp != nil {
			disp.Release()
		}

		return 0
	}
	defer disp.Release()
	defer ctx.Release()

	devDesconhecido := (*systemcom.IUnknown)(unsafe.Pointer(disp))

	vivos := 0

	for idxSaida := uint32(0); ; idxSaida++ {
		var saida *dxgi.IDXGIOutput
		if e := adaptador.EnumOutputs(idxSaida, &saida); e != nil || saida == nil {
			break
		}

		if dxgiFotografarSaida(saida, devDesconhecido, disp, ctx, pix, origemX, origemY, largura) {
			vivos++
		}

		saida.Release()
	}

	return vivos
}

func dxgiRotacaoValida(rotacao int) bool {
	switch rotacao {
	case rotacaoIdentidadeDxgi, rotacao90Dxgi, rotacao180Dxgi, rotacao270Dxgi:
		return true
	default:
		return false
	}
}

func dxgiGeometriaSaida(desc dxgi.DXGI_OUTPUT_DESC) (int32, int32, bool) {
	larg := desc.DesktopCoordinates.Right - desc.DesktopCoordinates.Left
	alt := desc.DesktopCoordinates.Bottom - desc.DesktopCoordinates.Top

	if larg <= 0 || alt <= 0 {
		return 0, 0, false
	}

	return larg, alt, true
}

func dxgiAbrirDuplicacao(saida *dxgi.IDXGIOutput, dev *systemcom.IUnknown) (*dxgi.IDXGIOutputDuplication, bool) {
	saida1ptr := queryInterface(unsafe.Pointer(saida), (*GUID)(unsafe.Pointer(&dxgi.IID_IDXGIOutput1)))
	if saida1ptr == nil {
		return nil, false
	}

	saida1 := (*dxgi.IDXGIOutput1)(saida1ptr)

	var dup *dxgi.IDXGIOutputDuplication
	if e := saida1.DuplicateOutput(dev, &dup); e != nil || dup == nil {
		return nil, false
	}

	return dup, true
}

func dxgiFotografarSaida(saida *dxgi.IDXGIOutput, dev *systemcom.IUnknown, disp *direct3d11.ID3D11Device,
	ctx *direct3d11.ID3D11DeviceContext, pix []byte, origemX, origemY, larguraVirt int32,
) bool {
	desc, e := saida.GetDesc()
	if e != nil || desc.AttachedToDesktop == 0 {
		return false
	}

	if !dxgiRotacaoValida(int(desc.Rotation)) {
		return false
	}

	if _, _, geometriaOk := dxgiGeometriaSaida(desc); !geometriaOk {
		return false
	}

	dup, duplicou := dxgiAbrirDuplicacao(saida, dev)
	if !duplicou {
		return false
	}

	copiou := dxgiCopiarFrame(
		dup, disp, ctx, pix, origemX, origemY, larguraVirt, desc.DesktopCoordinates, int(desc.Rotation))
	dup.Release()

	return copiou
}

func dxgiAdquirirTextura(dup *dxgi.IDXGIOutputDuplication) (*direct3d11.ID3D11Texture2D, func(), bool) {
	var (
		info    dxgi.DXGI_OUTDUPL_FRAME_INFO
		recurso *dxgi.IDXGIResource
	)
	if e := dup.AcquireNextFrame(esperaFrameDxgiMs, &info, &recurso); e != nil || recurso == nil {
		return nil, nil, false
	}

	texPtr := queryInterface(unsafe.Pointer(recurso), (*GUID)(unsafe.Pointer(&direct3d11.IID_ID3D11Texture2D)))
	recurso.Release()

	if texPtr == nil {
		dup.ReleaseFrame()

		return nil, nil, false
	}

	tex := (*direct3d11.ID3D11Texture2D)(texPtr)
	soltar := func() {
		tex.Release()
		dup.ReleaseFrame()
	}

	return tex, soltar, true
}

func dxgiCriarStaging(
	disp *direct3d11.ID3D11Device, descricao direct3d11.D3D11_TEXTURE2D_DESC,
) (*direct3d11.ID3D11Texture2D, bool) {
	sdesc := direct3d11.D3D11_TEXTURE2D_DESC{
		Width:          descricao.Width,
		Height:         descricao.Height,
		MipLevels:      1,
		ArraySize:      1,
		Format:         dxgicommon.DXGI_FORMAT_B8G8R8A8_UNORM,
		SampleDesc:     descricao.SampleDesc,
		Usage:          direct3d11.D3D11_USAGE_STAGING,
		BindFlags:      0,
		CPUAccessFlags: uint32(direct3d11.D3D11_CPU_ACCESS_READ),
		MiscFlags:      0,
	}

	var staging *direct3d11.ID3D11Texture2D
	if e := disp.CreateTexture2D(&sdesc, nil, &staging); e != nil || staging == nil {
		return nil, false
	}

	return staging, true
}

func dxgiMapearLeitura(ctx *direct3d11.ID3D11DeviceContext, staging *direct3d11.ID3D11Texture2D,
	descricao direct3d11.D3D11_TEXTURE2D_DESC,
) ([]byte, int, func(), bool) {
	resStag := (*direct3d11.ID3D11Resource)(unsafe.Pointer(staging))

	var mapeado direct3d11.D3D11_MAPPED_SUBRESOURCE
	if e := ctx.Map(resStag, 0, direct3d11.D3D11_MAP_READ, 0, &mapeado); e != nil || mapeado.PData == nil {
		return nil, 0, nil, false
	}

	passo := int(mapeado.RowPitch)

	return unsafe.Slice((*byte)(mapeado.PData), passo*int(descricao.Height)), passo, func() { ctx.Unmap(resStag, 0) }, true
}

func dxgiDimensaoCombinada(rotacao, largDestino, altDestino, largFrame, altFrame int) bool {
	if rotacao == rotacaoIdentidadeDxgi {
		return true
	}
	// Rotacao DXGI: 1=identidade, 2=90, 3=180, 4=270. O frame da
	// duplicacao vem sem rotacao; o destino esta rotacionado.
	if rotacao == rotacao180Dxgi {
		return largDestino == largFrame && altDestino == altFrame
	}

	return largDestino == altFrame && altDestino == largFrame
}

func dxgiOrigemPixel(rotacao, destX, destY, largFrame, altFrame int) (int, int) {
	switch rotacao {
	case rotacao180Dxgi:
		return largFrame - 1 - destX, altFrame - 1 - destY
	case rotacao90Dxgi:
		return altFrame - 1 - destY, destX
	case rotacao270Dxgi:
		return destY, largFrame - 1 - destX
	default:
		return destX, destY
	}
}

func dxgiGravarPixel(pix, src []byte, larguraVirt int32, origemX, origemY, destX, destY, fonteX, fonteY, passo int) {
	if fonteX < 0 || fonteY < 0 {
		return
	}

	posX := origemX + destX
	posY := origemY + destY

	if posX < 0 || posY < 0 {
		return
	}

	indiceSrc := fonteY*passo + fonteX*bytesPorPixelDxgi
	indiceDst := (posY*int(larguraVirt) + posX) * bytesPorPixelDxgi

	if indiceSrc < 0 || indiceDst < 0 || indiceSrc+3 >= len(src) || indiceDst+3 >= len(pix) {
		return
	}

	pix[indiceDst], pix[indiceDst+1], pix[indiceDst+2], pix[indiceDst+3] =
		src[indiceSrc], src[indiceSrc+1], src[indiceSrc+2], alfaOpacoDxgi
}

func dxgiUnirPixels(
	pix, src []byte, origemX, origemY, larguraVirt int32, dst foundation.RECT, largFrame, altFrame, rotacao int,
	passo int,
) {
	largDestino := int(dst.Right - dst.Left)
	altDestino := int(dst.Bottom - dst.Top)
	origemDstX := int(dst.Left - origemX)
	origemDstY := int(dst.Top - origemY)

	if largDestino <= 0 || altDestino <= 0 || largFrame <= 0 || altFrame <= 0 {
		return
	}

	for linha := range altDestino {
		for coluna := range largDestino {
			fonteX, fonteY := dxgiOrigemPixel(rotacao, coluna, linha, largFrame, altFrame)
			if fonteX >= largFrame || fonteY >= altFrame {
				continue
			}

			dxgiGravarPixel(pix, src, larguraVirt, origemDstX, origemDstY, coluna, linha, fonteX, fonteY, passo)
		}
	}
}

func dxgiCopiarFrame(dup *dxgi.IDXGIOutputDuplication, disp *direct3d11.ID3D11Device,
	ctx *direct3d11.ID3D11DeviceContext, pix []byte, origemX, origemY, larguraVirt int32, dst foundation.RECT,
	rotacao int,
) bool {
	tex, soltar, texturaOk := dxgiAdquirirTextura(dup)
	if !texturaOk {
		return false
	}
	defer soltar()

	var descricao direct3d11.D3D11_TEXTURE2D_DESC
	tex.GetDesc(&descricao)

	if descricao.Width == 0 || descricao.Height == 0 {
		return false
	}

	staging, criou := dxgiCriarStaging(disp, descricao)
	if !criou {
		return false
	}
	defer staging.Release()

	resTex := (*direct3d11.ID3D11Resource)(unsafe.Pointer(tex))
	resStag := (*direct3d11.ID3D11Resource)(unsafe.Pointer(staging))

	ctx.CopyResource(resStag, resTex)

	src, passo, desmapear, leituraOk := dxgiMapearLeitura(ctx, staging, descricao)
	if !leituraOk {
		return false
	}
	defer desmapear()

	largDestino := int(dst.Right - dst.Left)
	altDestino := int(dst.Bottom - dst.Top)
	largFrame, altFrame := int(descricao.Width), int(descricao.Height)

	if !dxgiDimensaoCombinada(rotacao, largDestino, altDestino, largFrame, altFrame) {
		return false
	}

	dxgiUnirPixels(pix, src, origemX, origemY, larguraVirt, dst, largFrame, altFrame, rotacao, passo)

	return true
}
