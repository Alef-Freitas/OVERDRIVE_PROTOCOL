//go:build windows

package main

import (
	"unsafe"
)

const (
	srccopyGDI  = 0xCC0020
	dibCoresGDI = 0
)

// contarPixelsAcesos conta pixels com algum canal RGB ligado.
func contarPixelsAcesos(pix []byte) int {
	acesos := 0

	for i := 0; i+3 < len(pix); i += bytesPorPixelCaptura {
		if pix[i] != 0 || pix[i+1] != 0 || pix[i+2] != 0 {
			acesos++
		}
	}

	return acesos
}

// fotografarTela tenta DXGI e cai para GDI se vier preto.
func fotografarTela() ([]byte, int32, int32, int32, int32, error) {
	pix, ox, oy, w, h, err := dxgiFotografarTela()
	if err == nil && contarPixelsAcesos(pix) > 0 {
		return pix, ox, oy, w, h, nil
	}

	return fotografarTelaGDI()
}

// fotografarTelaGDI captura a tela virtual via BitBlt (sempre funciona
// no desktop interativo; cobre drivers onde o DXGI devolve preto).
func fotografarTelaGDI() ([]byte, int32, int32, int32, int32, error) {
	origemX, origemY, largura, altura, telaOk := dxgiMedirTela()
	if !telaOk {
		return nil, 0, 0, 0, 0, erroTexto("tela indisponivel")
	}

	if pixelsCapturaExcedem(largura, altura) {
		return nil, 0, 0, 0, 0, erroTexto("tela grande demais")
	}

	hdcTela, _, _ := pGetDC.Call(0)
	if hdcTela == 0 {
		return nil, 0, 0, 0, 0, erroTexto("sem contexto da tela")
	}
	defer pReleaseDC.Call(0, hdcTela)

	hdcMem, _, _ := pCreateCompatibleDC.Call(hdcTela)
	if hdcMem == 0 {
		return nil, 0, 0, 0, 0, erroTexto("sem contexto de copia")
	}
	defer pDeleteDC.Call(hdcMem)

	hbmp, _, _ := pCreateCompatibleBitmap.Call(hdcTela, uintptr(largura), uintptr(altura))
	if hbmp == 0 {
		return nil, 0, 0, 0, 0, erroTexto("sem bitmap da tela")
	}
	defer pDeleteObject.Call(hbmp)

	anterior, _, _ := pSelectObject.Call(hdcMem, hbmp)
	defer pSelectObject.Call(hdcMem, anterior)

	r, _, _ := pBitBlt.Call(hdcMem, 0, 0, uintptr(largura), uintptr(altura),
		hdcTela, uintptr(origemX), uintptr(origemY), srccopyGDI)
	if r == 0 {
		return nil, 0, 0, 0, 0, erroTexto("falha ao copiar a tela")
	}

	var info bitmapInfo

	info.bmiHeader.biSize = uint32(unsafe.Sizeof(bitmapInfoHeader{}))
	info.bmiHeader.biWidth = largura
	info.bmiHeader.biHeight = -altura
	info.bmiHeader.biPlanes = 1
	info.bmiHeader.biBitCount = 32
	info.bmiHeader.biCompression = biRGB
	info.bmiHeader.biSizeImage = uint32(int(largura) * int(altura) * bytesPorPixelCaptura)

	buf := make([]byte, int(largura)*int(altura)*bytesPorPixelCaptura)

	linhas, _, _ := pGetDIBits.Call(hdcMem, hbmp, 0, uintptr(altura),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&info)), dibCoresGDI)
	if int32(linhas) != altura {
		return nil, 0, 0, 0, 0, erroTexto("falha ao ler a tela")
	}

	return buf, origemX, origemY, largura, altura, nil
}
