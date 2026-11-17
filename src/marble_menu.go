//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Menu: itens e cores do Marble intencionais.

package main

// Menu de contexto do Marble (frame ~64 do video): New Window, Read Mode,
// Tidy Up, Fold Trail, Switch to Space, New Space, Settings.

const (
	marbleMenuLargura = 220
	marbleMenuAltura  = 282
	marbleMenuItemAlt = 32
	marbleMenuRaio    = 10
	marbleMenuFundo   = 0xF01E2024
)

var marbleItensMenu = [...]string{
	"New Window", "Read Mode", "Add Sticky", "Tidy Up",
	"Fold Trail", "Switch to Space", "New Space", "Settings",
}

func marbleMenuRect(app *app) rectT {
	largura := app.sc(marbleMenuLargura)
	altura := app.sc(marbleMenuAltura)
	area := app.clientRect()
	x := marbleUI.menuX
	y := marbleUI.menuY
	if x+largura > area.Right {
		x = area.Right - largura
	}
	if y+altura > area.Bottom {
		y = area.Bottom - altura
	}
	return rectT{x, y, x + largura, y + altura}
}

func marbleAbrirMenuContexto(app *app, x, y int32) {
	marbleUI.menuAberto = true
	marbleUI.menuX = x
	marbleUI.menuY = y
	marbleSincronizarCamadas(app)
}

func marbleExecutarItemMenu(app *app, indice int) {
	switch indice {
	case 0:
		marbleNovaCard(app)
	case 1:
		marbleAlternarReadMode(app)
	case 2:
		marbleAdicionarSticky()
	case 3:
		boardTidy()
	case 4:
		boardDobrar()
	case 5:
		marbleTrocarSpace(app, (marbleEspacoAtual+1)%marbleNumSpaces)
	case 6:
		marbleTrocarSpace(app, (marbleEspacoAtual+1)%marbleNumSpaces)
	case 7:
		marbleAlternarAjuda(app)
	}
	marbleUI.menuAberto = false
	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func marblePintarMenu(disp *d2dMotor, app *app) {
	if !marbleUI.menuAberto {
		return
	}

	caixa := marbleMenuRect(app)
	disp.redondo(caixa, app.sc(marbleMenuRaio), marbleMenuFundo, espBordaCard)

	topo := caixa.Top + app.sc(8)
	for indice, item := range marbleItensMenu {
		area := rectT{
			caixa.Left + app.sc(6), topo + int32(indice)*app.sc(marbleMenuItemAlt),
			caixa.Right - app.sc(6), topo + (int32(indice)+1)*app.sc(marbleMenuItemAlt),
		}
		cor := uint32(espCorDim)
		if indice == marbleUI.linhaFoco {
			disp.redondo(area, app.sc(6), espCorSelecao, 0)
			cor = espCorTitulo
		}
		disp.textoUI(item, rectT{
			area.Left + app.sc(12), area.Top,
			area.Right - app.sc(8), area.Bottom,
		}, cor, false, false)
	}
}

func marbleCliqueMenu(app *app, x, y int32) bool {
	if !marbleUI.menuAberto {
		return false
	}

	caixa := marbleMenuRect(app)
	if !espDentro(caixa, x, y) {
		marbleUI.menuAberto = false
		pInvalidateRect.Call(app.hwnd, 0, 1)

		return false
	}

	topo := caixa.Top + app.sc(8)
	for indice := range marbleItensMenu {
		area := rectT{
			caixa.Left + app.sc(6), topo + int32(indice)*app.sc(marbleMenuItemAlt),
			caixa.Right - app.sc(6), topo + (int32(indice)+1)*app.sc(marbleMenuItemAlt),
		}
		if espDentro(area, x, y) {
			marbleExecutarItemMenu(app, indice)

			return true
		}
	}

	return true
}
