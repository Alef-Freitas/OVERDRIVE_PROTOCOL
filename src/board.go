//go:build windows

//nolint:wsl_v5 // Modelo do board: espacamento segue o estilo do projeto.

package main

import (
	"slices"
	"time"
)

// Modelo do board derivado de storage::workspace e scene::* do Marble.
// Go puro: sem cgo, sem WebView e sem qualquer modulo de IA/Explain.

type boardID uint64

type boardPoint struct {
	X float64
	Y float64
}

type boardRect struct {
	X float64
	Y float64
	W float64
	H float64
}

type boardKind uint8

const (
	boardKindPage boardKind = iota
	boardKindNote
	boardKindSticky
	boardKindFile
	boardKindImage
)

type boardEdgeKind uint8

const (
	boardEdgeLink boardEdgeKind = iota
	boardEdgeSpawn
	boardEdgeNote
)

type boardNode struct {
	ID          boardID
	Kind        boardKind
	Rect        boardRect
	Z           float64
	URL         string
	Title       string
	Tint        string
	CreatedAt   int64
	UpdatedAt   int64
	FromSID     string
	ToSID       string
	PageZoom    float64
	Scale       float64
	Sticky      bool
	Collapsed   bool
	Deleted     bool
	Closed      bool
	Body        string
	BodyFormat  string
	TrailName   string
	TrailIcon   string
	TrailColor  string
	OutlineRank int
}

type boardEdge struct {
	From      boardID
	To        boardID
	Kind      boardEdgeKind
	CreatedAt int64
	FromSID   string
	ToSID     string
}

type boardStack struct {
	Members     []boardID
	Front       boardID
	OutlineOpen bool
	Collapsed   bool
}

type boardCamera struct {
	Center boardPoint
	Zoom   float64
}

type boardWorkspace struct {
	Camera  boardCamera
	Nodes   []boardNode
	Edges   []boardEdge
	Stacks  []boardStack
	Focused []boardID
	NextID  boardID
}

func boardNovoWorkspace() boardWorkspace {
	return boardWorkspace{
		Camera: boardCamera{
			Center: boardPoint{X: 0, Y: 0},
			Zoom:   1,
		},
		Nodes:   nil,
		Edges:   nil,
		Stacks:  nil,
		Focused: nil,
		NextID:  1,
	}
}

func boardAdicionarNo(workspace *boardWorkspace, kind boardKind, rect boardRect) boardID {
	identificador := workspace.NextID
	workspace.NextID++

	agora := time.Now().UnixMilli()
	workspace.Nodes = append(workspace.Nodes, boardNode{
		ID:          identificador,
		Kind:        kind,
		Rect:        rect,
		Z:           float64(len(workspace.Nodes)),
		URL:         "",
		Title:       "",
		Tint:        "",
		CreatedAt:   agora,
		UpdatedAt:   agora,
		FromSID:     "",
		ToSID:       "",
		PageZoom:    1,
		Scale:       1,
		Sticky:      kind == boardKindSticky,
		Collapsed:   false,
		Deleted:     false,
		Closed:      false,
		Body:        "",
		BodyFormat:  "",
		TrailName:   "",
		TrailIcon:   "",
		TrailColor:  "",
		OutlineRank: len(workspace.Nodes),
	})

	return identificador
}

func boardNo(workspace *boardWorkspace, identificador boardID) *boardNode {
	for indice := range workspace.Nodes {
		if workspace.Nodes[indice].ID == identificador {
			return &workspace.Nodes[indice]
		}
	}

	return nil
}

func boardConectar(workspace *boardWorkspace, de, para boardID, kind boardEdgeKind) bool {
	if boardNo(workspace, de) == nil || boardNo(workspace, para) == nil {
		return false
	}

	workspace.Edges = append(workspace.Edges, boardEdge{
		From:      de,
		To:        para,
		Kind:      kind,
		CreatedAt: time.Now().UnixMilli(),
		FromSID:   "",
		ToSID:     "",
	})

	return true
}

func boardRemoverNo(workspace *boardWorkspace, identificador boardID) {
	workspace.Nodes = slices.DeleteFunc(workspace.Nodes, func(no boardNode) bool {
		return no.ID == identificador
	})
	workspace.Edges = slices.DeleteFunc(workspace.Edges, func(aresta boardEdge) bool {
		return aresta.From == identificador || aresta.To == identificador
	})
	workspace.Focused = slices.DeleteFunc(workspace.Focused, func(focado boardID) bool {
		return focado == identificador
	})
}

// boardPai devolve o vertice de origem de uma aresta que termina em identificador.
func boardPai(workspace *boardWorkspace, identificador boardID) (boardID, bool) {
	for _, aresta := range workspace.Edges {
		if aresta.To == identificador {
			return aresta.From, true
		}
	}

	return 0, false
}

func boardFilhos(workspace *boardWorkspace, identificador boardID) []boardID {
	var filhos []boardID

	for _, aresta := range workspace.Edges {
		if aresta.From == identificador {
			filhos = append(filhos, aresta.To)
		}
	}

	slices.SortFunc(filhos, func(primeiro, segundo boardID) int {
		noPrimeiro := boardNo(workspace, primeiro)
		noSegundo := boardNo(workspace, segundo)
		if noPrimeiro == nil || noSegundo == nil {
			return 0
		}

		if noPrimeiro.CreatedAt != noSegundo.CreatedAt {
			if noPrimeiro.CreatedAt < noSegundo.CreatedAt {
				return -1
			}

			return 1
		}

		return noPrimeiro.OutlineRank - noSegundo.OutlineRank
	})

	return filhos
}

func boardRaiz(workspace *boardWorkspace, identificador boardID) boardID {
	visitados := map[boardID]bool{}
	atual := identificador

	for {
		if visitados[atual] {
			return atual
		}
		visitados[atual] = true

		pai, existe := boardPai(workspace, atual)
		if !existe {
			return atual
		}
		atual = pai
	}
}

func boardAncestrais(workspace *boardWorkspace, identificador boardID) []boardID {
	var cadeia []boardID
	atual := identificador

	for {
		pai, existe := boardPai(workspace, atual)
		if !existe {
			break
		}
		cadeia = append(cadeia, pai)
		atual = pai
	}

	return cadeia
}

func boardFloresta(workspace *boardWorkspace) []boardEdge {
	return slices.Clone(workspace.Edges)
}

func boardCaminhar(workspace *boardWorkspace, raiz boardID, visitar func(boardID)) {
	visitados := map[boardID]bool{}
	boardCaminharVisitados(workspace, raiz, visitados, visitar)
}

func boardCaminharVisitados(
	workspace *boardWorkspace, identificador boardID, visitados map[boardID]bool, visitar func(boardID),
) {
	if visitados[identificador] {
		return
	}
	visitados[identificador] = true

	visitar(identificador)
	for _, filho := range boardFilhos(workspace, identificador) {
		boardCaminharVisitados(workspace, filho, visitados, visitar)
	}
}

// boardProximoNaTrilha equivale a next_on_trail (Tab).
func boardProximoNaTrilha(workspace *boardWorkspace, identificador boardID) boardID {
	filhos := boardFilhos(workspace, identificador)
	if len(filhos) > 0 {
		return filhos[0]
	}

	atual := identificador
	for {
		pai, existe := boardPai(workspace, atual)
		if !existe {
			raiz := boardRaiz(workspace, identificador)
			primeiros := boardFilhos(workspace, raiz)
			if len(primeiros) > 0 {
				return primeiros[0]
			}

			return raiz
		}

		irmaos := boardFilhos(workspace, pai)
		for indice, irmao := range irmaos {
			if irmao == atual && indice+1 < len(irmaos) {
				return irmaos[indice+1]
			}
		}
		atual = pai
	}
}

// boardAnteriorNaTrilha equivale a previous_on_trail (Ctrl+Shift+Tab).
func boardAnteriorNaTrilha(workspace *boardWorkspace, identificador boardID) boardID {
	pai, existe := boardPai(workspace, identificador)
	if existe {
		return pai
	}

	ultimo := identificador
	boardCaminhar(workspace, identificador, func(no boardID) {
		ultimo = no
	})

	return ultimo
}
