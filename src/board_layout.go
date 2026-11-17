//go:build windows

//nolint:wsl_v5,exhaustruct_v5 // Layout: espacamento e literais de geometria intencionais.

package main

import (
	"math"
	"slices"
	"time"
)

// Constantes de layout aproximadas dos simbolos tools::tidy do Marble.
// A formula exata nao esta no binario; estes valores reproduzem o comportamento
// observavel: familias de trilha em colunas, cards empilhados em cada familia.
const (
	boardTidyColunas  = 4
	boardTidyLargura  = 320.0
	boardTidyGapX     = 28.0
	boardTidyGapY     = 28.0
	boardTidyEsquerda = 80.0
	boardTidyTopo     = 80.0
	boardZoomMin      = 0.25
	boardZoomMax      = 3.0
	boardZoomPasso    = 1.12
)

type boardTarget struct {
	ID   boardID
	Rect boardRect
}

type boardGroup struct {
	Raiz    boardID
	Membros []boardID
}

func boardFamilias(workspace *boardWorkspace) []boardGroup {
	raizes := map[boardID]bool{}

	for _, noAtual := range workspace.Nodes {
		if noAtual.Deleted {
			continue
		}
		raizes[boardRaiz(workspace, noAtual.ID)] = true
	}

	var grupos []boardGroup
	for raiz := range raizes {
		grupo := boardGroup{Raiz: raiz, Membros: nil}
		boardCaminhar(workspace, raiz, func(identificador boardID) {
			noAtual := boardNo(workspace, identificador)
			if noAtual != nil && !noAtual.Deleted {
				grupo.Membros = append(grupo.Membros, identificador)
			}
		})
		grupos = append(grupos, grupo)
	}

	slices.SortFunc(grupos, func(primeiro, segundo boardGroup) int {
		return int(primeiro.Raiz) - int(segundo.Raiz)
	})

	return grupos
}

func boardTidyGrid(workspace *boardWorkspace, grupos []boardGroup) []boardTarget {
	var alvos []boardTarget
	coluna := 0
	posX := boardTidyEsquerda
	posY := boardTidyTopo
	alturaColuna := 0.0

	for _, grupo := range grupos {
		if coluna >= boardTidyColunas {
			coluna = 0
			posX = boardTidyEsquerda
			posY += alturaColuna + boardTidyGapY
			alturaColuna = 0
		}

		posicaoAtual := posY
		for _, identificador := range grupo.Membros {
			noAtual := boardNo(workspace, identificador)
			if noAtual == nil {
				continue
			}
			alvos = append(alvos, boardTarget{ID: identificador, Rect: boardRect{
				X: posX, Y: posicaoAtual, W: noAtual.Rect.W, H: noAtual.Rect.H,
			}})
			posicaoAtual += noAtual.Rect.H + boardTidyGapY
		}

		alturaColuna = math.Max(alturaColuna, posicaoAtual-posY)
		posX += boardTidyLargura + boardTidyGapX
		coluna++
	}

	return alvos
}

func boardAplicarTidy(workspace *boardWorkspace) bool {
	grupos := boardFamilias(workspace)
	if len(grupos) == 0 {
		return false
	}

	alvos := boardTidyGrid(workspace, grupos)
	mudou := false
	agora := time.Now().UnixMilli()

	for _, alvo := range alvos {
		noAtual := boardNo(workspace, alvo.ID)
		if noAtual == nil {
			continue
		}

		if noAtual.Rect.X != alvo.Rect.X || noAtual.Rect.Y != alvo.Rect.Y {
			noAtual.Rect.X = alvo.Rect.X
			noAtual.Rect.Y = alvo.Rect.Y
			mudou = true
		}
		noAtual.UpdatedAt = agora
	}

	return mudou
}

func boardTrilhaComum(workspace *boardWorkspace, selecionados []boardID) (boardID, bool) {
	if len(selecionados) == 0 {
		return 0, false
	}

	raiz := boardRaiz(workspace, selecionados[0])
	for _, identificador := range selecionados[1:] {
		if boardRaiz(workspace, identificador) != raiz {
			return 0, false
		}
	}

	return raiz, true
}

func boardFold(workspace *boardWorkspace, selecionados []boardID) (boardStack, bool) {
	raiz, ok := boardTrilhaComum(workspace, selecionados)
	if !ok {
		return boardStack{Members: nil, Front: 0, OutlineOpen: false, Collapsed: false}, false
	}

	var membros []boardID
	boardCaminhar(workspace, raiz, func(identificador boardID) {
		membros = append(membros, identificador)
	})

	pilha := boardStack{
		Members:     membros,
		Front:       raiz,
		OutlineOpen: false,
		Collapsed:   true,
	}
	workspace.Stacks = append(workspace.Stacks, pilha)

	for _, identificador := range membros {
		if noAtual := boardNo(workspace, identificador); noAtual != nil {
			noAtual.Collapsed = true
		}
	}

	return pilha, true
}

func boardUnfold(workspace *boardWorkspace, identificador boardID) bool {
	indice := -1

	for posicao, pilha := range workspace.Stacks {
		if pilha.Front == identificador {
			indice = posicao

			break
		}
	}
	if indice < 0 {
		return false
	}

	for _, membro := range workspace.Stacks[indice].Members {
		if noAtual := boardNo(workspace, membro); noAtual != nil {
			noAtual.Collapsed = false
		}
	}
	workspace.Stacks = slices.Delete(workspace.Stacks, indice, indice+1)

	return true
}

func boardLimitarZoom(zoom float64) float64 {
	if zoom < boardZoomMin {
		return boardZoomMin
	}
	if zoom > boardZoomMax {
		return boardZoomMax
	}

	return zoom
}

// boardZoomSobre e o equivalente a zoom_steps_about: zoom ancorado num ponto.
func boardZoomSobre(camera *boardCamera, ancora boardPoint, fator float64) {
	novoZoom := boardLimitarZoom(camera.Zoom * fator)
	if novoZoom == camera.Zoom {
		return
	}

	proporcao := camera.Zoom / novoZoom
	camera.Center.X = ancora.X + (camera.Center.X-ancora.X)*proporcao
	camera.Center.Y = ancora.Y + (camera.Center.Y-ancora.Y)*proporcao
	camera.Zoom = novoZoom
}

// boardAjustarTodos e o equivalente a fit_all_target.
func boardAjustarTodos(workspace *boardWorkspace) boardCamera {
	if len(workspace.Nodes) == 0 {
		return boardCamera{Center: boardPoint{X: 0, Y: 0}, Zoom: 1}
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)

	for _, noAtual := range workspace.Nodes {
		if noAtual.Deleted {
			continue
		}
		minX = math.Min(minX, noAtual.Rect.X)
		minY = math.Min(minY, noAtual.Rect.Y)
		maxX = math.Max(maxX, noAtual.Rect.X+noAtual.Rect.W)
		maxY = math.Max(maxY, noAtual.Rect.Y+noAtual.Rect.H)
	}

	return boardCamera{
		Center: boardPoint{X: (minX + maxX) / meioD2D, Y: (minY + maxY) / meioD2D},
		Zoom:   1,
	}
}
