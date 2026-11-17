//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Persistencia: formato simples do board.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Persistencia do board: o Marble guarda um arquivo por space. Aqui usamos
// JSON simples com os cartoes, arestas, camera e notas.

type marbleCardSalvo struct {
	ID        boardID
	Left      int32
	Top       int32
	Right     int32
	Bottom    int32
	Titulo    string
	URL       string
	EhNota    bool
	CorNota   uint32
	Texto     string
	TemSticky bool
}

type marbleBoardSalvo struct {
	Cartoes  []marbleCardSalvo
	Arestas  []boardEdge
	Camera   boardCamera
	CorAtual int
}

func marbleCaminhoBoard() string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "marble_board.json")
}

func marbleSalvarBoard() {
	caminho := marbleCaminhoBoard()
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
			ID:        card.id,
			Left:      card.rect.Left,
			Top:       card.rect.Top,
			Right:     card.rect.Right,
			Bottom:    card.rect.Bottom,
			Titulo:    card.titulo,
			URL:       card.url,
			EhNota:    card.ehNota,
			CorNota:   card.corNota,
			Texto:     card.textoNota,
			TemSticky: card.temSticky,
		})
	}

	dados, erro := json.Marshal(salvo)
	if erro != nil {
		return
	}
	_ = os.WriteFile(caminho, dados, 0o600)
}

func marbleCarregarBoard() bool {
	caminho := marbleCaminhoBoard()
	if caminho == "" {
		return false
	}

	dados, erro := os.ReadFile(caminho) // #nosec G304 -- caminho interno do app.
	if erro != nil {
		return false
	}

	var salvo marbleBoardSalvo
	if erro := json.Unmarshal(dados, &salvo); erro != nil {
		return false
	}

	espCanvas.cartoes = nil
	for _, item := range salvo.Cartoes {
		espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
			id:        item.ID,
			rect:      rectT{item.Left, item.Top, item.Right, item.Bottom},
			titulo:    item.Titulo,
			url:       item.URL,
			cr:        espBordaCard,
			ehNota:    item.EhNota,
			corNota:   item.CorNota,
			textoNota: item.Texto,
			temSticky: item.TemSticky,
		})
	}

	espCanvas.foco = -1
	espCanvas.aberto = -1
	espCanvas.editando = -1
	boardSincronizar()
	boardAtivo.Edges = salvo.Arestas
	boardAtivo.Camera = salvo.Camera
	marbleProximaCor = salvo.CorAtual

	return len(espCanvas.cartoes) > 0
}
