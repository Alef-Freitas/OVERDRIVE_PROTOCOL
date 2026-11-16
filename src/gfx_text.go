//go:build windows

package main

import (
	"image"
	"image/color"
	"os"
)

const (
	alfaOpacoIcone       = 255
	limiarAlfaFundo      = 200
	toleranciaCorIcone   = 42
	meioIcone            = 2
	bytesPorPixelIcone   = 4
	luminanciaFundoIcone = 0.35
)

//odlint:somente-leitura // Tabela estatica id->glifo da barra de navegacao.
//nolint:gochecknoglobals // Tabela estatica id->glifo da barra de navegacao.
var glifosNav = map[string]string{
	idHome:        "⌂",
	idVoltar:      "←",
	idAvancar:     "→",
	"go":          "↵",
	idRecarregar:  "↻",
	idParar:       "✕",
	idZoom:        "⌕",
	idExpand:      "⛶",
	idTrash:       "⌫",
	"tr":          "⊕",
	idYoutube:     "▶",
	idCookies:     "●",
	idNojs:        "⊘",
	idVPN:         "⚿",
	idSleep:       "☾",
	idPaywall:     "▣",
	idReader:      "≡",
	idTranslate:   "A",
	idSend:        "➤",
	idPin:         "◆",
	idAudio:       "♪",
	idMute:        "×",
	idLock:        "■",
	idUnlock:      "□",
	idGlobe:       "○",
	idFerramentas: "⚒",
	idImprimir:    "◉",
	idDownloads:   "↓",
	idLivreClique: "✎",
}

func glifoNav(id string) string {
	if g, ok := glifosNav[id]; ok {
		return g
	}

	if id == idPanels || id == idClipboard || id == idCopiarLeitura {
		return "▤"
	}

	return ""
}

func lumOf(cor color.NRGBA) float64 {
	return (0.2126*float64(cor.R) + 0.7152*float64(cor.G) + 0.0722*float64(cor.B)) / alfaOpacoIcone
}

func closeTo(corA, corB color.NRGBA, tolerancia int) bool {
	distancia := func(posX, posY uint8) int {
		if posX > posY {
			return int(posX - posY)
		}

		return int(posY - posX)
	}

	return distancia(corA.R, corB.R) <= tolerancia && distancia(corA.G, corB.G) <= tolerancia &&
		distancia(corA.B, corB.B) <= tolerancia
}

type cropIcone struct {
	src     image.Image
	crop    srcRect
	stripBg bool
	bg      color.NRGBA
}

func decodificarIcone(path string) (cropIcone, error) {
	var recorte cropIcone
	// GOD: chamadores passam caminhos contidos (icone validado em iconeArquivoExt).
	arquivo, err := os.Open(path) // #nosec G304
	if err != nil {
		return recorte, erroTextoF("abrir icone: %w", err)
	}
	defer arquivo.Close()

	src, _, err := image.Decode(arquivo)
	if err != nil {
		return recorte, erroTextoF("decodificar icone: %w", err)
	}

	limites := src.Bounds()
	if limites.Dx() <= 0 || limites.Dy() <= 0 {
		return recorte, erroTextoF("imagem vazia: %s", path)
	}

	recorte.src = src

	fundo, ok := color.NRGBAModel.Convert(src.At(limites.Min.X, limites.Min.Y)).(color.NRGBA)
	if !ok {
		return recorte, erroTextoF("imagem sem fundo legivel: %s", path)
	}

	recorte.bg = fundo
	recorte.stripBg = recorte.bg.A > limiarAlfaFundo && lumOf(recorte.bg) < luminanciaFundoIcone

	recorte.crop = medirCropIcone(src, recorte.bg, recorte.stripBg, limites)

	if recorte.crop.x1 < recorte.crop.x0 || recorte.crop.y1 < recorte.crop.y0 {
		recorte.crop = srcRect{limites.Min.X, limites.Min.Y, limites.Max.X - 1, limites.Max.Y - 1}
	}

	return recorte, nil
}

// medirCropIcone mede a caixa dos pixels visiveis fora do fundo.
func medirCropIcone(src image.Image, fundo color.NRGBA, stripBg bool, limites image.Rectangle) srcRect {
	crop := srcRect{x0: limites.Max.X, y0: limites.Max.Y, x1: limites.Min.X, y1: limites.Min.Y}
	for posY := limites.Min.Y; posY < limites.Max.Y; posY++ {
		for posX := limites.Min.X; posX < limites.Max.X; posX++ {
			if !pixelVisivelIcone(src, fundo, stripBg, posX, posY) {
				continue
			}

			alargarCropIcone(&crop, posX, posY)
		}
	}

	return crop
}

// pixelVisivelIcone diz se o pixel participa do conteudo do icone.
func pixelVisivelIcone(src image.Image, fundo color.NRGBA, stripBg bool, x, y int) bool {
	pixel, ok := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
	if !ok {
		return false
	}

	if pixel.A < 8 || (stripBg && closeTo(pixel, fundo, toleranciaCorIcone)) {
		return false
	}

	return true
}

// alargarCropIcone expande a caixa para incluir o ponto.
func alargarCropIcone(crop *srcRect, posX, posY int) {
	if posX < crop.x0 {
		crop.x0 = posX
	}

	if posX > crop.x1 {
		crop.x1 = posX
	}

	if posY < crop.y0 {
		crop.y0 = posY
	}

	if posY > crop.y1 {
		crop.y1 = posY
	}
}

func enquadrarIcone(crop srcRect, size int32) (int32, int32, int32, int32) {
	largura := crop.x1 - crop.x0 + 1
	altura := crop.y1 - crop.y0 + 1

	dstW, dstH := size, size
	if largura > altura {
		dstH = size * int32(altura) / int32(largura)
	} else if altura > largura {
		dstW = size * int32(largura) / int32(altura)
	}

	if dstW < 1 {
		dstW = 1
	}

	if dstH < 1 {
		dstH = 1
	}

	offX := (size - dstW) / meioIcone
	offY := (size - dstH) / meioIcone

	return dstW, dstH, offX, offY
}

func fillIconPixels(dst []byte, src image.Image, crop srcRect, stripBg bool, fundo color.NRGBA,
	size, dstW, dstH, offX, offY int32,
) {
	largura := int32(crop.x1 - crop.x0 + 1)

	altura := int32(crop.y1 - crop.y0 + 1)

	for dy := range dstH {
		for dx := range dstW {
			indice := ((dy+offY)*size + (dx + offX)) * bytesPorPixelIcone
			somaR, somaG, somaB, somaA, total := mediarJanelaIcone(
				src, crop, stripBg, fundo, largura, altura, dstW, dstH, dx, dy,
			)

			if total == 0 {
				total = 1
			}

			dst[indice+0] = byte(somaB / total)
			dst[indice+1] = byte(somaG / total)
			dst[indice+2] = byte(somaR / total)
			dst[indice+3] = byte(somaA / total)
		}
	}
}

// mediarJanelaIcone acumula os canais da janela fonte do pixel destino.
func mediarJanelaIcone(src image.Image, crop srcRect, stripBg bool, fundo color.NRGBA,
	largura, altura, dstW, dstH, dx, dy int32,
) (uint32, uint32, uint32, uint32, uint32) {
	var somaR, somaG, somaB, somaA, total uint32

	x0, x1 := faixaIcone(crop.x0, int(largura), int(dstW), int(dx))
	y0, y1 := faixaIcone(crop.y0, int(altura), int(dstH), int(dy))

	for posY := y0; posY < y1; posY++ {
		for posX := x0; posX < x1; posX++ {
			pixel, ok := color.NRGBAModel.Convert(src.At(posX, posY)).(color.NRGBA)
			if !ok {
				continue
			}

			if stripBg && closeTo(pixel, fundo, toleranciaCorIcone) {
				total++

				continue
			}

			alfa := uint32(pixel.A)
			somaR += uint32(pixel.R) * alfa / alfaOpacoIcone
			somaG += uint32(pixel.G) * alfa / alfaOpacoIcone
			somaB += uint32(pixel.B) * alfa / alfaOpacoIcone
			somaA += alfa
			total++
		}
	}

	return somaR, somaG, somaB, somaA, total
}

// faixaIcone converte o indice destino na faixa fonte correspondente.
func faixaIcone(base, total, destino, i int) (int, int) {
	ini := base + total*i/destino

	fim := base + total*(i+1)/destino
	if fim <= ini {
		fim = ini + 1
	}

	return ini, fim
}
