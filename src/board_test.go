//go:build windows

//nolint:wsl_v5,exhaustruct_v5 // Testes: literais parciais sao intencionais.

package main

import (
	"math"
	"testing"
)

func TestBoardTrilhaFloresta(t *testing.T) {
	t.Parallel()

	ws := boardNovoWorkspace()
	raiz := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 0, Y: 0, W: 100, H: 80})
	filhoA := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 120, Y: 0, W: 100, H: 80})
	filhoB := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 240, Y: 0, W: 100, H: 80})
	neto := boardAdicionarNo(&ws, boardKindNote, boardRect{X: 120, Y: 100, W: 100, H: 80})

	boardConectar(&ws, raiz, filhoA, boardEdgeLink)
	boardConectar(&ws, raiz, filhoB, boardEdgeLink)
	boardConectar(&ws, filhoA, neto, boardEdgeSpawn)

	filhos := boardFilhos(&ws, raiz)
	if len(filhos) != 2 || filhos[0] != filhoA || filhos[1] != filhoB {
		t.Fatalf("filhos da raiz = %v", filhos)
	}

	if obtida := boardRaiz(&ws, neto); obtida != raiz {
		t.Fatalf("raiz do neto = %d, esperado %d", obtida, raiz)
	}

	ancestrais := boardAncestrais(&ws, neto)
	if len(ancestrais) != 2 || ancestrais[0] != filhoA || ancestrais[1] != raiz {
		t.Fatalf("ancestrais = %v", ancestrais)
	}

	if floresta := boardFloresta(&ws); len(floresta) != 3 {
		t.Fatalf("floresta = %d arestas", len(floresta))
	}

	var percorridos []boardID
	boardCaminhar(&ws, raiz, func(id boardID) {
		percorridos = append(percorridos, id)
	})
	if len(percorridos) != 4 {
		t.Fatalf("caminhada = %v", percorridos)
	}
}

func TestBoardNavegacaoTrilha(t *testing.T) {
	t.Parallel()

	ws := boardNovoWorkspace()
	raiz := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	filhoA := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	filhoB := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	boardConectar(&ws, raiz, filhoA, boardEdgeLink)
	boardConectar(&ws, filhoA, filhoB, boardEdgeLink)

	if obtido := boardProximoNaTrilha(&ws, raiz); obtido != filhoA {
		t.Fatalf("proximo da raiz = %d, esperado %d", obtido, filhoA)
	}
	if obtido := boardProximoNaTrilha(&ws, filhoA); obtido != filhoB {
		t.Fatalf("proximo do filho A = %d, esperado %d", obtido, filhoB)
	}
	if obtido := boardAnteriorNaTrilha(&ws, filhoB); obtido != filhoA {
		t.Fatalf("anterior do filho B = %d, esperado %d", obtido, filhoA)
	}
	if obtido := boardAnteriorNaTrilha(&ws, raiz); obtido != filhoB {
		t.Fatalf("anterior da raiz = %d, esperado %d (wrap)", obtido, filhoB)
	}
}

func TestBoardTidy(t *testing.T) {
	t.Parallel()

	ws := boardNovoWorkspace()
	primeiro := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 500, Y: 500, W: 120, H: 90})
	segundo := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 900, Y: 700, W: 120, H: 90})
	boardConectar(&ws, primeiro, segundo, boardEdgeLink)

	if !boardAplicarTidy(&ws) {
		t.Fatal("tidy deveria mover os cards")
	}

	noPrimeiro := boardNo(&ws, primeiro)
	noSegundo := boardNo(&ws, segundo)
	if noPrimeiro == nil || noSegundo == nil {
		t.Fatal("nos desapareceram")
	}
	if noPrimeiro.Rect.X != boardTidyEsquerda || noPrimeiro.Rect.Y != boardTidyTopo {
		t.Fatalf("raise = %+v", noPrimeiro.Rect)
	}
	if noSegundo.Rect.Y <= noPrimeiro.Rect.Y {
		t.Fatalf("descendente nao ficou abaixo: %+v", noSegundo.Rect)
	}
	if boardAplicarTidy(&ws) {
		t.Fatal("segundo tidy nao deveria mudar nada")
	}
}

func TestBoardFold(t *testing.T) {
	t.Parallel()

	ws := boardNovoWorkspace()
	raiz := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	filho := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	outra := boardAdicionarNo(&ws, boardKindPage, boardRect{})
	boardConectar(&ws, raiz, filho, boardEdgeLink)

	pilha, ok := boardFold(&ws, []boardID{raiz})
	if !ok || len(pilha.Members) != 2 || !pilha.Collapsed {
		t.Fatalf("fold = %+v, ok=%v", pilha, ok)
	}
	if no := boardNo(&ws, filho); no == nil || !no.Collapsed {
		t.Fatal("filho nao ficou colapsado")
	}
	if _, ok := boardFold(&ws, []boardID{raiz, outra}); ok {
		t.Fatal("fold de trilhas diferentes deveria falhar")
	}
	if !boardUnfold(&ws, pilha.Front) {
		t.Fatal("unfold falhou")
	}
	if no := boardNo(&ws, filho); no == nil || no.Collapsed {
		t.Fatal("filho nao foi expandido")
	}
}

func TestBoardUndoRedo(t *testing.T) {
	t.Parallel()

	ws := boardNovoWorkspace()
	id := boardAdicionarNo(&ws, boardKindPage, boardRect{X: 10, Y: 10, W: 100, H: 80})
	antes := []boardRect{{X: 10, Y: 10, W: 100, H: 80}}
	depois := []boardRect{{X: 200, Y: 300, W: 100, H: 80}}
	executar, reverter := boardComandoMover([]boardID{id}, antes, depois)

	var hist boardHistorico
	hist.registrar(executar, reverter, false)
	boardAplicarComando(&ws, executar)

	if obtido := boardNo(&ws, id); obtido == nil || obtido.Rect.X != 200 {
		t.Fatal("comando nao aplicou")
	}
	if !hist.desfazer(&ws) {
		t.Fatal("desfazer falhou")
	}
	if obtido := boardNo(&ws, id); obtido == nil || obtido.Rect.X != 10 {
		t.Fatal("desfazer nao restaurou")
	}
	if !hist.refazer(&ws) {
		t.Fatal("refazer falhou")
	}
	if obtido := boardNo(&ws, id); obtido == nil || obtido.Rect.X != 200 {
		t.Fatal("refazer nao reaplicou")
	}
}

func TestBoardCoalesce(t *testing.T) {
	t.Parallel()

	var hist boardHistorico
	primeiro, reverso1 := boardComandoMover([]boardID{1}, nil, nil)
	segundo, reverso2 := boardComandoMover([]boardID{1}, nil, nil)

	hist.registrar(primeiro, reverso1, true)
	hist.registrar(segundo, reverso2, true)

	if tamanho := hist.tamanho(); tamanho != 1 {
		t.Fatalf("coalesce deveria agrupar em 1 passo, tem %d", tamanho)
	}
}

func TestBoardCamera(t *testing.T) {
	t.Parallel()

	if obtido := boardLimitarZoom(99); obtido != boardZoomMax {
		t.Fatalf("zoom max = %v", obtido)
	}
	if obtido := boardLimitarZoom(0.01); obtido != boardZoomMin {
		t.Fatalf("zoom min = %v", obtido)
	}

	camera := boardCamera{Center: boardPoint{X: 100, Y: 100}, Zoom: 1}
	boardZoomSobre(&camera, boardPoint{X: 100, Y: 100}, boardZoomPasso)
	if math.Abs(camera.Zoom-boardZoomPasso) > 1e-9 {
		t.Fatalf("zoom = %v", camera.Zoom)
	}
	if camera.Center.X != 100 || camera.Center.Y != 100 {
		t.Fatalf("ancora moveu: %+v", camera.Center)
	}

	ws := boardNovoWorkspace()
	boardAdicionarNo(&ws, boardKindPage, boardRect{X: 0, Y: 0, W: 100, H: 100})
	boardAdicionarNo(&ws, boardKindPage, boardRect{X: 200, Y: 200, W: 100, H: 100})
	ajustada := boardAjustarTodos(&ws)
	if ajustada.Center.X != 150 || ajustada.Center.Y != 150 {
		t.Fatalf("centro ajustado = %+v", ajustada.Center)
	}
}
