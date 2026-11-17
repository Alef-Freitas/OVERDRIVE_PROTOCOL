//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Read Mode: geometria do Marble.

package main

// Read Mode do Marble: expande o card focado para occupar a janela inteira.

func marbleAlternarReadMode(app *app) {
	if app == nil || espCanvas.foco < 0 || espCanvas.foco >= len(espCanvas.cartoes) {
		return
	}

	indice := espCanvas.foco
	card := &espCanvas.cartoes[indice]

	if espCanvas.readMode {
		card.rect = espCanvas.rectAntesRead
		espCanvas.readMode = false
	} else {
		espCanvas.rectAntesRead = card.rect
		area := app.clientRect()
		card.rect = rectT{
			area.Left, area.Top,
			area.Right, area.Bottom,
		}
		espCanvas.readMode = true
	}

	if espCanvas.readMode {
		espAbrirCard(app, indice)
	}
	if espCanvas.aberto == indice {
		espPosicionarWebView(app)
	}
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
