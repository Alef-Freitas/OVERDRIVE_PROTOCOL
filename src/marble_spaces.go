//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Spaces: estado dos boards.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Spaces do Marble: cada space e um board inteiro. Troca com Ctrl+1..9.

const marbleNumSpaces = 9

//nolint:gochecknoglobals // Spaces: estado de UI single-window.
var (
	marbleEspacos     [marbleNumSpaces]marbleBoardSalvo
	marbleEspacoAtual int
	marbleEspacosSujo [marbleNumSpaces]bool
)

func marbleCaminhoSpace(indice int) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, fmt.Sprintf("marble_space_%d.json", indice+1))
}

func marbleSalvarSpace(indice int) {
	caminho := marbleCaminhoSpace(indice)
	if caminho == "" {
		return
	}

	salvo := marbleBoardSalvo{
		Camera:   boardAtivo.Camera,
		Arestas:  boardFloresta(&boardAtivo),
		CorAtual: marbleProximaCor,
	}
	for _, card := range espCanvas.cartoes {
		salvo.Cartoes = append(salvo.Cartoes, marbleCardSalvo{
			ID: card.id, Left: card.rect.Left, Top: card.rect.Top,
			Right: card.rect.Right, Bottom: card.rect.Bottom,
			Titulo: card.titulo, URL: card.url, EhNota: card.ehNota,
			CorNota: card.corNota, Texto: card.textoNota, TemSticky: card.temSticky,
		})
	}

	dados, erro := json.Marshal(salvo)
	if erro != nil {
		return
	}
	_ = os.WriteFile(caminho, dados, 0o600)
	marbleEspacos[indice] = salvo
	marbleEspacosSujo[indice] = false
}

func marbleCarregarSpace(indice int) bool {
	caminho := marbleCaminhoSpace(indice)
	if caminho == "" {
		return false
	}

	dados, erro := os.ReadFile(caminho) // #nosec G304 -- caminho interno.
	if erro != nil {
		return false
	}

	var salvo marbleBoardSalvo
	if json.Unmarshal(dados, &salvo) != nil {
		return false
	}

	espCanvas.cartoes = nil
	for _, item := range salvo.Cartoes {
		espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
			id: item.ID, rect: rectT{item.Left, item.Top, item.Right, item.Bottom},
			titulo: item.Titulo, url: item.URL, cr: espBordaCard,
			ehNota: item.EhNota, corNota: item.CorNota,
			textoNota: item.Texto, temSticky: item.TemSticky,
		})
	}

	espCanvas.foco = -1
	espCanvas.aberto = -1
	espCanvas.editando = -1
	boardSincronizar()
	boardAtivo.Edges = salvo.Arestas
	boardAtivo.Camera = salvo.Camera
	marbleProximaCor = salvo.CorAtual
	marbleEspacoAtual = indice

	return true
}

func marbleTrocarSpace(app *app, indice int) {
	if indice < 0 || indice >= marbleNumSpaces {
		return
	}

	marbleSalvarSpace(marbleEspacoAtual)
	if !marbleCarregarSpace(indice) {
		espCanvas.cartoes = nil
		boardIniciar()
		marbleEspacoAtual = indice
	}
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
