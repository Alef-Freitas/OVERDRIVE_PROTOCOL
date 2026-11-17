//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Notas: cores do Marble e nomes curtos intencionais.

package main

import "unsafe"

// Notas e stickies do Marble: card colorido com texto, criado com a tecla N.
// Cores extraidas do video oficial (frames 33-63).

var marbleCoresNota = [...]uint32{
	0xFFEBE7E6, // branco / paper (#E6E7EB, medido do frame)
	0xFF3C5CE8, // laranja / ember
	0xFF8AE6FD, // amarelo / amber
	0xFF50AF4C, // verde / moss
	0xFFFA8BA7, // roxo / violet
	0xFFB18FF4, // rosa / pink
}

//nolint:gochecknoglobals // Cor atual da proxima nota: estado de UI single-window.
var marbleProximaCor int

func marbleCriarNota(app *app) {
	if app == nil {
		return
	}

	area := app.clientRect()
	largura := app.sc(260)
	altura := app.sc(200)
	esq := area.Left + (area.Width()-largura)/meioD2D
	topo := area.Top + (area.Height()-altura)/meioD2D

	cor := marbleCoresNota[marbleProximaCor%len(marbleCoresNota)]
	marbleProximaCor++

	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		id:        0,
		rect:      rectT{esq, topo, esq + largura, topo + altura},
		titulo:    "",
		url:       "",
		cr:        cor,
		ehNota:    true,
		corNota:   cor,
		textoNota: "",
	})

	espCanvas.foco = len(espCanvas.cartoes) - 1
	marbleEditarNota(app, espCanvas.foco)
}

func marbleEditarNota(app *app, indice int) {
	if app == nil || app.urlEdit == 0 || indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	card := espCanvas.cartoes[indice]
	pSetWindowPos.Call(app.urlEdit, 0,
		uintptr(card.rect.Left+app.sc(12)), uintptr(card.rect.Top+app.sc(12)),
		uintptr(card.rect.Width()-app.sc(24)), uintptr(app.sc(40)),
		swpNoZOrder|swpNoActivate|swpShowWindow)

	pSetWindowTextW.Call(app.urlEdit, uintptr(unsafe.Pointer(utf16(card.textoNota))))
	app.invalidaEditCache()
	pShowWindow.Call(app.urlEdit, swShow)
	pSetFocus.Call(app.urlEdit)

	espCanvas.editando = indice
	espEditAtivo = true
}

func marbleAplicarTextoNota(app *app, texto string) {
	indice := espCanvas.editando
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}

	espCanvas.cartoes[indice].textoNota = texto
	espCanvas.cartoes[indice].titulo = texto
	espOcultarEdit(app)
	espCanvas.editando = -1
	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func marblePintarNota(disp *d2dMotor, indice int, app *app) {
	card := espCanvas.cartoes[indice]
	disp.redondo(card.rect, espCardRaio, card.corNota, 0)

	if card.textoNota == "" || indice == espCanvas.editando {
		return
	}

	recuo := app.sc(20)
	topoTexto := app.sc(20)
	area := rectT{
		card.rect.Left + recuo, card.rect.Top + topoTexto,
		card.rect.Right - app.sc(16), card.rect.Top + topoTexto + app.sc(48),
	}
	disp.texto(card.textoNota, area, espCorGround, "Segoe UI", app.sc(22), 0, 0, 1)
}

func marbleAdicionarSticky() {
	indice := espCanvas.foco
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}
	if espCanvas.cartoes[indice].ehNota {
		return
	}
	espCanvas.cartoes[indice].temSticky = true
}

func marblePintarNotas(disp *d2dMotor, app *app) {
	for indice, card := range espCanvas.cartoes {
		if card.ehNota {
			marblePintarNota(disp, indice, app)
		}
	}
}
