//go:build windows

//nolint:wsl_v5,exhaustruct_v5,prealloc // Ponte: espacamento, literais e alocacao sob demanda intencionais.

package main

// Ponte entre o canvas espacial (espCanvas) e o modelo do board (boardAtivo).
// O canvas continua sendo a fonte de verdade para renderizacao e WebView; o
// modelo guarda trilhas, pilhas e historico derivados dos cartoes.

//nolint:gochecknoglobals // Espelho do canvas: estado de UI single-window.
var (
	boardAtivo boardWorkspace
	boardHist  boardHistorico
)

func boardIniciar() {
	boardAtivo = boardNovoWorkspace()
	boardHist = boardHistorico{passos: nil, futuro: nil}
}

func boardDaRect(area rectT) boardRect {
	return boardRect{
		X: float64(area.Left),
		Y: float64(area.Top),
		W: float64(area.Width()),
		H: float64(area.Height()),
	}
}

func boardParaRect(area boardRect) rectT {
	return rectT{
		Left:   int32(area.X),
		Top:    int32(area.Y),
		Right:  int32(area.X + area.W),
		Bottom: int32(area.Y + area.H),
	}
}

func boardSincronizar() {
	novo := boardNovoWorkspace()
	novo.NextID = boardAtivo.NextID

	for indice, card := range espCanvas.cartoes {
		identificador := card.id
		if identificador == 0 {
			identificador = novo.NextID
			novo.NextID++
			espCanvas.cartoes[indice].id = identificador
		}

		novo.Nodes = append(novo.Nodes, boardNode{
			ID:          identificador,
			Kind:        boardKindPage,
			Rect:        boardDaRect(card.rect),
			Z:           float64(indice),
			URL:         card.url,
			Title:       card.titulo,
			Tint:        "",
			CreatedAt:   int64(indice),
			UpdatedAt:   int64(indice),
			FromSID:     "",
			ToSID:       "",
			PageZoom:    1,
			Scale:       1,
			Sticky:      false,
			Collapsed:   false,
			Deleted:     false,
			Closed:      false,
			Body:        "",
			BodyFormat:  "",
			TrailName:   "",
			TrailIcon:   "",
			TrailColor:  "",
			OutlineRank: indice,
		})
	}

	for _, aresta := range boardAtivo.Edges {
		if boardNo(&novo, aresta.From) != nil && boardNo(&novo, aresta.To) != nil {
			novo.Edges = append(novo.Edges, aresta)
		}
	}

	novo.Stacks = boardAtivo.Stacks
	novo.Camera = boardAtivo.Camera
	novo.Focused = boardAtivo.Focused
	boardAtivo = novo
}

func boardCapturarRects() ([]boardID, []boardRect) {
	var ids []boardID
	var retangulos []boardRect

	for _, noAtual := range boardAtivo.Nodes {
		ids = append(ids, noAtual.ID)
		retangulos = append(retangulos, noAtual.Rect)
	}

	return ids, retangulos
}

func boardLerRects(ids []boardID) []boardRect {
	var retangulos []boardRect

	for _, identificador := range ids {
		if noAtual := boardNo(&boardAtivo, identificador); noAtual != nil {
			retangulos = append(retangulos, noAtual.Rect)
		} else {
			retangulos = append(retangulos, boardRect{})
		}
	}

	return retangulos
}

func boardAplicarAosCartoes() {
	for indice := range espCanvas.cartoes {
		noAtual := boardNo(&boardAtivo, espCanvas.cartoes[indice].id)
		if noAtual == nil {
			continue
		}
		espCanvas.cartoes[indice].rect = boardParaRect(noAtual.Rect)
	}
}

func boardIndiceDo(identificador boardID) int {
	for indice, card := range espCanvas.cartoes {
		if card.id == identificador {
			return indice
		}
	}

	return -1
}

func boardRegistrarMover(ids []boardID, antes, depois []boardRect, agrupar bool) {
	executar, reverter := boardComandoMover(ids, antes, depois)
	boardHist.registrar(executar, reverter, agrupar)
}

func boardRegistrarMovimentoFoco() {
	boardSincronizar()
	if espCanvas.foco < 0 || espCanvas.foco >= len(espCanvas.cartoes) {
		return
	}

	identificador := espCanvas.cartoes[espCanvas.foco].id
	noAtual := boardNo(&boardAtivo, identificador)
	if noAtual == nil {
		return
	}

	antes := []boardRect{boardDaRect(espCanvas.cardOrigem)}
	depois := []boardRect{noAtual.Rect}
	boardRegistrarMover([]boardID{identificador}, antes, depois, false)
}

func boardTidy() bool {
	boardSincronizar()
	ids, antes := boardCapturarRects()
	if !boardAplicarTidy(&boardAtivo) {
		return false
	}

	depois := boardLerRects(ids)
	executar, reverter := boardComandoTidy(ids, antes, depois)
	boardHist.registrar(executar, reverter, false)

	// Anima do rect anterior (antes) ate o novo (depois).
	for indice, retanguloAntes := range antes {
		if indice < len(depois) && indice < len(espCanvas.cartoes) {
			marbleAnimarPara(indice, boardParaRect(retanguloAntes), boardParaRect(depois[indice]))
		}
	}

	return true
}

func boardNavegar(app *app, anterior bool) bool {
	boardSincronizar()
	if espCanvas.foco < 0 || espCanvas.foco >= len(espCanvas.cartoes) {
		return false
	}

	idAtual := espCanvas.cartoes[espCanvas.foco].id
	var destino boardID
	if anterior {
		destino = boardAnteriorNaTrilha(&boardAtivo, idAtual)
	} else {
		destino = boardProximoNaTrilha(&boardAtivo, idAtual)
	}

	indice := boardIndiceDo(destino)
	if indice < 0 {
		return false
	}

	espCanvas.foco = indice
	espAbrirCard(app, indice)
	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

func boardDobrar() bool {
	boardSincronizar()
	if espCanvas.foco < 0 || espCanvas.foco >= len(espCanvas.cartoes) {
		return false
	}

	identificador := espCanvas.cartoes[espCanvas.foco].id
	pilha, ok := boardFold(&boardAtivo, []boardID{identificador})
	if !ok {
		return false
	}

	boardHist.registrar(
		boardComando{
			Kind: boardCmdDobrar, IDs: nil, Rects: nil, Nos: nil,
			Pilhas: []boardStack{pilha},
		},
		boardComando{
			Kind: boardCmdDesdobrar, IDs: []boardID{pilha.Front}, Rects: nil,
			Nos: nil, Pilhas: nil,
		},
		false,
	)

	return true
}

func boardDesfazer(app *app) bool {
	boardSincronizar()
	if !boardHist.desfazer(&boardAtivo) {
		return false
	}

	boardAplicarAosCartoes()
	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

func boardRefazer(app *app) bool {
	boardSincronizar()
	if !boardHist.refazer(&boardAtivo) {
		return false
	}

	boardAplicarAosCartoes()
	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

func boardAjustarView(app *app) bool {
	boardSincronizar()
	if len(boardAtivo.Nodes) == 0 {
		return false
	}

	boardAtivo.Camera = boardAjustarTodos(&boardAtivo)
	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

func boardConectarFoco(anterior boardID, atual boardID) bool {
	boardSincronizar()
	if anterior == 0 || atual == 0 {
		return false
	}
	if !boardConectar(&boardAtivo, anterior, atual, boardEdgeSpawn) {
		return false
	}

	return true
}
