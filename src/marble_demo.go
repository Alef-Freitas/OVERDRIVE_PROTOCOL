//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Demo: reproduz o estado dos frames do video.

package main

// Modo demo: monta o board com o texto exato dos frames do vídeo oficial do
// Marble, para comparacao visual 1:1. Ativado com F9.

const (
	marbleDemoNotaX = 340.0
	marbleDemoNotaY = 180.0
	marbleDemoNotaW = 560.0
	marbleDemoNotaH = 360.0
)

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

// marbleModoDemo reproduz o frame 3 (nota "Japan. 1 week...") e o frame 6
// (card "Kyoto Travel Guide"), com o texto exato do vídeo.
func marbleModoDemo(app *app) {
	espCanvas.cartoes = nil
	espCanvas.foco = -1
	espCanvas.aberto = -1
	espCanvas.editando = -1
	marbleProximaCor = 0

	x, y := marbleDemoNotaX, marbleDemoNotaY
	marbleDemoAdicionarNota(
		"Japan. 1 week. No idea where to start.",
		rectT{int32(x), int32(y), int32(x + marbleDemoNotaW), int32(y + marbleDemoNotaH)},
		marbleCoresNota[0],
	)

	boardSincronizar()
	marbleUI.sidebarAberta = false
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
