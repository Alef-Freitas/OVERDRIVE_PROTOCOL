//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Demo: reproduz o estado dos frames do video.

package main

// Modo demo: monta o board com o texto exato dos frames do vídeo oficial do
// Marble, para comparacao visual 1:1. Ativado com F9.

func marbleDemoAdicionarNota(texto string, rect rectT, cor uint32) {
	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		id:        0,
		rect:      rect,
		titulo:    texto,
		corNota:   cor,
		ehNota:    true,
		textoNota: texto,
	})
}

func marbleDemoAdicionarCard(titulo, endereco string, rect rectT) {
	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		id:     0,
		rect:   rect,
		titulo: titulo,
		url:    endereco,
		cr:     espBordaCard,
	})
}

// marbleModoDemo reproduz o board completo do vídeo: nota inicial, trilha de
// cards do Japão, notas coloridas e stickies, com o texto exato dos frames.
func marbleModoDemo(app *app) {
	espCanvas.cartoes = nil
	espCanvas.foco = -1
	espCanvas.aberto = -1
	espCanvas.editando = -1
	marbleProximaCor = 0

	// Nota inicial (frames 1-4).
	marbleDemoAdicionarNota(
		"Japan. 1 week. No idea where to start.",
		rectT{340, 120, 900, 340},
		marbleCoresNota[0],
	)

	// Nota de planejamento (frames 33-42).
	marbleDemoAdicionarNota(
		"Kyoto: 3 days. Done.",
		rectT{940, 140, 1180, 300},
		marbleCoresNota[1],
	)

	// Nota de tickets (frames 55-63).
	marbleDemoAdicionarNota(
		"Tickets teamLab Ghibli 10:00",
		rectT{300, 560, 560, 700},
		marbleCoresNota[2],
	)

	// Trilha de cards do Japão (frames 8-18) — 4 cards lado a lado.
	titulos := [...]string{
		"Kyoto - Google Maps",
		"Ginkakuji (Silver Pavilion)",
		"Kiyomizudera Temple",
		"Fushimi Inari Shrine",
	}
	baseX := int32(300)
	for indice, titulo := range titulos {
		x := baseX + int32(indice)*230
		marbleDemoAdicionarCard(titulo, "", rectT{x, 370, x + 215, 520})
	}

	boardSincronizar()

	// Conecta os cards da trilha com os IDs reais atribuidos pelo boardSincronizar.
	indicePrimeiroCard := len(espCanvas.cartoes) - len(titulos)
	for indice := 0; indice < len(titulos)-1; indice++ {
		idPai := espCanvas.cartoes[indicePrimeiroCard+indice].id
		idFilho := espCanvas.cartoes[indicePrimeiroCard+indice+1].id
		boardConectar(&boardAtivo, idPai, idFilho, boardEdgeLink)
	}

	marbleUI.sidebarAberta = false
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
