//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Overlay de atalhos do Marble.

package main

// Overlays de atalho do Marble: quando uma tecla e pressionada, aparece no
// centro um selo "NOME DO COMANDO" com a letra da tecla (ex.: "NOTE  N").

import "time"

const marbleOverlayMs = 900

//nolint:gochecknoglobals // Overlay: estado de UI single-window.
var (
	marbleOverlayTexto  string
	marbleOverlayTecla  string
	marbleOverlayInicio time.Time
)

func marbleMostrarOverlay(nome, tecla string) {
	marbleOverlayTexto = nome
	marbleOverlayTecla = tecla
	marbleOverlayInicio = time.Now()
}

func marblePintarOverlay(disp *d2dMotor, app *app) {
	if marbleOverlayTexto == "" {
		return
	}

	decorrido := time.Since(marbleOverlayInicio).Milliseconds()
	if decorrido > marbleOverlayMs {
		marbleOverlayTexto = ""

		return
	}

	// Fade out nos ultimos 300ms.
	alfa := float32(1)
	if decorrido > marbleOverlayMs-300 {
		alfa = float32(marbleOverlayMs-decorrido) / 300
	}
	_ = alfa

	area := app.clientRect()
	centroX := area.Left + area.Width()/meioD2D
	centroY := area.Top + area.Height() - app.sc(70)

	disp.textoUI(marbleOverlayTexto, rectT{
		centroX - app.sc(120), centroY - app.sc(20),
		centroX + app.sc(120), centroY,
	}, espCorFaint, true, true)

	caixa := rectT{
		centroX - app.sc(16), centroY,
		centroX + app.sc(16), centroY + app.sc(32),
	}
	disp.redondo(caixa, app.sc(7), espCorGround|0xFF000000, espBordaCard)
	disp.textoUI(marbleOverlayTecla, caixa, espCorTitulo, false, true)
}
