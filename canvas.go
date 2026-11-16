//go:build windows

package main

import "slices"

const (
	cartaoLargura    = 240
	cartaoAltura     = 160
	cartaoRaio       = 12
	zoomMinimo       = 0.25
	zoomMaximo       = 3.0
	zoomPasso        = 1.12
	trilhaEspessura  = 2
	bordaCartao      = 2
	linhaTituloAlt   = 30
	linhaURLAlt      = 20
	corFundoCartao   = 0xD0182028
	corBordaCartao   = 0xFF3A4A5A
	corBordaFoco     = 0xFF00D0E0
	corTrilha        = 0x90406070
	corTitulo        = 0xFFE8F0F4
	corURL           = 0xFF8FA4B0
	corBotaoSair     = 0xC02A3A48
	corCartaoUm      = 0xFF00D0E0
	corCartaoDois    = 0xFFB05CFF
	corCartaoTres    = 0xFFFFB050
	corCartaoQuatro  = 0xFF50E0A0
	posicaoXDois     = 320
	posicaoXQuatro   = 660
	posicaoYMenos    = -60
	posicaoYDois     = 140
	posicaoYQuatro   = 40
	indiceUm         = 1
	indiceDois       = 2
	indiceTres       = 3
	zoomRaioReduzido = 0.6
	zoomTextoVisivel = 0.45
	raioReduzido     = 4
	margemTexto      = 8
)

type cartao struct {
	x      float64
	y      float64
	titulo string
	url    string
	cor    uint32
}

type trilha struct {
	origem  int
	destino int
}

type canvas struct {
	offsetX    float64
	offsetY    float64
	zoom       float64
	cartoes    []cartao
	trilhas    []trilha
	foco       int
	arrastando bool
	ultX       int32
	ultY       int32
	movido     bool
}

func novoCanvas() canvas {
	return canvas{
		offsetX: 0,
		offsetY: 0,
		zoom:    1,
		cartoes: []cartao{
			{x: 0, y: 0, titulo: "Bem-vindo", url: "sobre:overdrive.protocol", cor: corCartaoUm},
			{x: posicaoXDois, y: posicaoYMenos, titulo: "Pesquisa", url: "sobre:canvas.infinito", cor: corCartaoDois},
			{x: posicaoXDois, y: posicaoYDois, titulo: "Notas", url: "sobre:espaco.1", cor: corCartaoTres},
			{x: posicaoXQuatro, y: posicaoYQuatro, titulo: "Conexões", url: "sobre:trilhas", cor: corCartaoQuatro},
		},
		trilhas: []trilha{
			{origem: 0, destino: indiceUm},
			{origem: 0, destino: indiceDois},
			{origem: indiceUm, destino: indiceTres},
			{origem: indiceDois, destino: indiceTres},
		},
		foco:       -1,
		arrastando: false,
		ultX:       0,
		ultY:       0,
		movido:     false,
	}
}

func (c *canvas) mundoParaTela(x, y float64, centroX, centroY int32) pontoT {
	return pontoT{
		X: int32((x-c.offsetX)*c.zoom + float64(centroX)),
		Y: int32((y-c.offsetY)*c.zoom + float64(centroY)),
	}
}

func (c *canvas) cartaoEm(x, y int32, centroX, centroY int32) int {
	largura := int32(float64(cartaoLargura) * c.zoom)
	altura := int32(float64(cartaoAltura) * c.zoom)

	for indice := range slices.Backward(c.cartoes) {
		p := c.mundoParaTela(c.cartoes[indice].x, c.cartoes[indice].y, centroX, centroY)
		if x >= p.X && x < p.X+largura && y >= p.Y && y < p.Y+altura {
			return indice
		}
	}

	return -1
}

func (c *canvas) mover(dx, dy int32) {
	c.offsetX -= float64(dx) / c.zoom
	c.offsetY -= float64(dy) / c.zoom
}

func (c *canvas) aproximar(x, y int32, centroX, centroY int32, fator float64) {
	mundoX := float64(x-centroX)/c.zoom + c.offsetX
	mundoY := float64(y-centroY)/c.zoom + c.offsetY

	novo := c.zoom * fator
	if novo < zoomMinimo {
		novo = zoomMinimo
	}

	if novo > zoomMaximo {
		novo = zoomMaximo
	}

	c.zoom = novo
	c.offsetX = mundoX - float64(x-centroX)/c.zoom
	c.offsetY = mundoY - float64(y-centroY)/c.zoom
}

func (c *canvas) pintarTrilhas(motor *motorD2D, centroX, centroY int32) {
	for _, ligacao := range c.trilhas {
		if ligacao.origem < 0 || ligacao.destino < 0 ||
			ligacao.origem >= len(c.cartoes) || ligacao.destino >= len(c.cartoes) {
			continue
		}

		origem := c.cartoes[ligacao.origem]
		destino := c.cartoes[ligacao.destino]

		pontoOrigem := c.mundoParaTela(
			origem.x+cartaoLargura/2, origem.y+cartaoAltura/2, centroX, centroY,
		)
		pontoDestino := c.mundoParaTela(
			destino.x+cartaoLargura/2, destino.y+cartaoAltura/2, centroX, centroY,
		)
		motor.linha(pontoOrigem, pontoDestino, corTrilha, trilhaEspessura)
	}
}

func (c *canvas) areaCartao(indice int, centroX, centroY int32) (rectT, int32, int32) {
	atual := c.cartoes[indice]
	p := c.mundoParaTela(atual.x, atual.y, centroX, centroY)
	largura := int32(float64(cartaoLargura) * c.zoom)
	altura := int32(float64(cartaoAltura) * c.zoom)

	return rectT{p.X, p.Y, p.X + largura, p.Y + altura}, largura, altura
}

func (c *canvas) pintarCartao(motor *motorD2D, indice int, centroX, centroY int32) {
	area, _, _ := c.areaCartao(indice, centroX, centroY)
	atual := c.cartoes[indice]

	corBorda := uint32(corBordaCartao)
	if indice == c.foco {
		corBorda = corBordaFoco
	}

	raio := float32(cartaoRaio)
	if c.zoom < zoomRaioReduzido {
		raio = raioReduzido
	}

	motor.cartao(area, raio, corFundoCartao, corBorda)

	faixa := rectT{
		area.Left, area.Top, area.Right,
		area.Top + int32(float64(linhaTituloAlt)*c.zoom),
	}
	motor.retangulo(faixa, atual.cor)

	if c.zoom < zoomTextoVisivel {
		return
	}

	topoTexto := faixa.Bottom
	tituloArea := rectT{
		area.Left + margemTexto, topoTexto,
		area.Right - margemTexto, topoTexto + int32(float64(linhaURLAlt)*c.zoom),
	}
	urlArea := rectT{
		area.Left + margemTexto, tituloArea.Bottom,
		area.Right - margemTexto, tituloArea.Bottom + int32(float64(linhaURLAlt)*c.zoom),
	}
	motor.texto(atual.titulo, tituloArea, corTitulo, true)
	motor.texto(atual.url, urlArea, corURL, false)
}

func (c *canvas) pintar(motor *motorD2D, centroX, centroY int32, botaoSair rectT) {
	c.pintarTrilhas(motor, centroX, centroY)

	for indice := range c.cartoes {
		c.pintarCartao(motor, indice, centroX, centroY)
	}

	motor.cartao(botaoSair, cartaoRaio, corBotaoSair, corBordaCartao)
	motor.texto("SAIR", botaoSair, corTitulo, false)
}

func (c *canvas) clique(x, y int32, centroX, centroY int32, botaoSair rectT) bool {
	if x >= botaoSair.Left && x < botaoSair.Right && y >= botaoSair.Top && y < botaoSair.Bottom {
		return true
	}

	c.foco = c.cartaoEm(x, y, centroX, centroY)

	return false
}
