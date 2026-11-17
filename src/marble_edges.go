//go:build windows

//nolint:wsl_v5,mnd // Geometria: espacamento e literais de interpolacao intencionais.

package main

// Conexoes de trilha do Marble: curvas cubicas de Bezier com docks retos nas
// bordas dos cards (app_shell::geom::edge_route + straight_docks). Desenhadas
// em Direct2D como segmentos de linha, sem cgo.

const (
	marbleSegmentosCurva  = 8
	marbleControle        = 0.42
	marbleEspessuraTrilha = 2.0
	marbleAlfaTrilha      = 0x70
)

func marblePontoBezierXY(
	iniX, iniY, ctlAX, ctlAY, ctlBX, ctlBY, fimX, fimY, proporcao float32,
) (float32, float32) {
	inverso := 1 - proporcao
	pesoIni := inverso * inverso * inverso
	pesoA := 3 * inverso * inverso * proporcao
	pesoB := 3 * inverso * proporcao * proporcao
	pesoFim := proporcao * proporcao * proporcao
	pontoX := pesoIni*iniX + pesoA*ctlAX + pesoB*ctlBX + pesoFim*fimX
	pontoY := pesoIni*iniY + pesoA*ctlAY + pesoB*ctlBY + pesoFim*fimY

	return pontoX, pontoY
}

func marblePintarTrilhas(disp *d2dMotor) {
	for _, aresta := range boardAtivo.Edges {
		noDe := boardNo(&boardAtivo, aresta.From)
		noPara := boardNo(&boardAtivo, aresta.To)
		if noDe == nil || noPara == nil {
			continue
		}

		de := boardParaRect(noDe.Rect)
		para := boardParaRect(noPara.Rect)

		iniX := float32(de.Right)
		iniY := float32(de.Top+de.Bottom) / meioD2D
		fimX := float32(para.Left)
		fimY := float32(para.Top+para.Bottom) / meioD2D

		distancia := fimX - iniX
		if distancia < 0 {
			distancia = -distancia
		}
		alcance := distancia * marbleControle

		ctlAX := iniX + alcance
		ctlAY := iniY
		ctlBX := fimX - alcance
		ctlBY := fimY

		anteriorX, anteriorY := iniX, iniY
		for passo := 1; passo <= marbleSegmentosCurva; passo++ {
			proporcao := float32(passo) / float32(marbleSegmentosCurva)
			pontoX, pontoY := marblePontoBezierXY(
				iniX, iniY, ctlAX, ctlAY, ctlBX, ctlBY, fimX, fimY, proporcao)
			disp.linhaCor(anteriorX, anteriorY, pontoX, pontoY, marbleEspessuraTrilha, espBordaFoco, marbleAlfaTrilha)
			anteriorX, anteriorY = pontoX, pontoY
		}
	}
}
