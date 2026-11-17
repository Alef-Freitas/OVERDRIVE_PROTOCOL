//go:build windows

//nolint:wsl_v5,varnamelen,exhaustruct_v5 // Wallpaper: espacamento e nomes de pixel intencionais.

package main

import (
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d/common"
	dxgicommon "github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/dxgi/common"
)

//nolint:gochecknoglobals // Wallpaper do board: estado de UI single-window.
var (
	d2dWallpaper     *direct2d.ID2D1Bitmap
	marbleFundoFoto  = true
	marbleWallpaperT = "wallpaper.jpg"
)

func marbleWallpaperCaminho() string {
	if exeDir == "" {
		return ""
	}

	return filepath.Join(exeDir, marbleWallpaperT)
}

// marbleCarregarWallpaper decodifica o JPG em Go puro e cria o bitmap D2D.
func marbleCarregarWallpaper(disp *d2dMotor) {
	if disp == nil || disp.rt == nil || d2dWallpaper != nil {
		return
	}

	caminho := marbleWallpaperCaminho()
	if caminho == "" {
		return
	}

	arquivo, erro := os.Open(caminho) // #nosec G304 -- caminho resolvido a partir do diretorio do exe.
	if erro != nil {
		return
	}
	defer arquivo.Close()

	src, _, erro := image.Decode(arquivo)
	if erro != nil {
		return
	}

	limites := src.Bounds()
	largura := int32(limites.Dx())
	altura := int32(limites.Dy())
	if largura <= 0 || altura <= 0 {
		return
	}

	pixels := make([]byte, int(largura)*int(altura)*bytesPorPixelD2D)
	indice := 0
	for y := limites.Min.Y; y < limites.Max.Y; y++ {
		for x := limites.Min.X; x < limites.Max.X; x++ {
			cor, _ := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			pixels[indice] = cor.B
			pixels[indice+1] = cor.G
			pixels[indice+2] = cor.R
			pixels[indice+3] = cor.A
			indice += bytesPorPixelD2D
		}
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
	if erro := disp.rt.CreateBitmap(
		tamanho, unsafe.Pointer(&pixels[0]), uint32(largura)*bytesPorPixelD2D, &props, &bitmap,
	); erro != nil || bitmap == nil {
		return
	}
	d2dWallpaper = bitmap
}

func marblePintarWallpaper(disp *d2dMotor, app *app) {
	if !marbleFundoFoto || disp == nil || disp.rt == nil {
		return
	}

	marbleCarregarWallpaper(disp)
	if d2dWallpaper == nil {
		return
	}

	area := app.clientRect()
	destino := d2dRet(area)
	disp.rt.DrawBitmap(d2dWallpaper, &destino, 1,
		direct2d.D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, nil)
}

func marbleAlternarFundo(app *app) {
	marbleFundoFoto = !marbleFundoFoto
	espAplicarAlpha(app)
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
