//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Ajuda: lista de comandos do Marble.

package main

// Tela de ajuda do Marble Overdrive: todos os comandos em uma lista.
// Acessivel com F1 (ou o botao "?" ) e fechada com Escape.

//nolint:gochecknoglobals // Ajuda: estado de UI single-window.
var marbleAjudaAberta bool

type marbleAjudaItem struct {
	tecla string
	texto string
}

func marbleAjudaItens() []marbleAjudaItem {
	return []marbleAjudaItem{
		{"F1", "Mostrar esta ajuda"},
		{"F9", "Demo do Marble (board completo)"},
		{"W", "Nova janela / card vazio"},
		{"N", "Nova nota colorida"},
		{"Enter", "Read Mode (expandir card)"},
		{"Clique", "Abrir pagina no card (dive)"},
		{"Arrastar", "Mover card / criar card (no vazio)"},
		{"Duplo clique", "Abrir WebView no card"},
		{"Botao direito", "Menu de contexto"},
		{"T", "Tidy Up (alinhar cards)"},
		{"S", "Fold Trail (empilhar trilha)"},
		{"Tab", "Proximo na trilha"},
		{"Ctrl+Shift+Tab", "Anterior na trilha"},
		{"Ctrl+Z", "Desfazer"},
		{"Ctrl+Y", "Refazer"},
		{"F", "Fit to View"},
		{"Ctrl+K", "Paleta de comandos / busca"},
		{"Ctrl+1..9", "Trocar de Space"},
		{"Ctrl+Shift+B", "Alternar wallpaper / transparente"},
		{"Ctrl+Shift+O", "Mostrar slider de opacidade"},
		{"Delete", "Remover card selecionado"},
		{"Escape", "Fechar menus / ajuda"},
	}
}

func marbleAjudaRect(app *app) rectT {
	area := app.clientRect()
	largura := app.sc(560)
	altura := app.sc(560)
	esq := area.Left + (area.Width()-largura)/meioD2D
	topo := area.Top + (area.Height()-altura)/meioD2D
	return rectT{esq, topo, esq + largura, topo + altura}
}

func marblePintarAjuda(disp *d2dMotor, app *app) {
	if !marbleAjudaAberta {
		return
	}

	caixa := marbleAjudaRect(app)
	disp.redondo(caixa, app.sc(12), marbleMenuFundo, espBordaFoco)

	disp.textoUI("MARBLE OVERDRIVE — COMANDOS", rectT{
		caixa.Left + app.sc(20), caixa.Top + app.sc(18),
		caixa.Right - app.sc(20), caixa.Top + app.sc(50),
	}, espCorTitulo, false, false)

	itens := marbleAjudaItens()
	alturaItem := app.sc(22)
	topo := caixa.Top + app.sc(62)

	for indice, item := range itens {
		y := topo + int32(indice)*alturaItem
		if y+alturaItem > caixa.Bottom {
			break
		}

		cor := uint32(espBordaFoco)
		disp.textoUI(item.tecla, rectT{
			caixa.Left + app.sc(20), y,
			caixa.Left + app.sc(190), y + alturaItem,
		}, cor, true, false)

		disp.textoUI(item.texto, rectT{
			caixa.Left + app.sc(200), y,
			caixa.Right - app.sc(16), y + alturaItem,
		}, espCorDim, true, false)
	}
}

func marbleAlternarAjuda(app *app) {
	marbleAjudaAberta = !marbleAjudaAberta
	pInvalidateRect.Call(app.hwnd, 0, 1)
}
