//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Sidebar: espacamento e cores do Marble intencionais.

package main

// Sidebar do Marble: traffic lights, nome do space, apps fixados e trilhas
// com favicon. Desenhada em Direct2D, sobre o board espacial.

const (
	marbleTLVermelho   = 0xFF575FFF
	marbleTLAmarelo    = 0xFF2EBCFE
	marbleTLVerde      = 0xFF40C828
	marbleSidebarFundo = 0xE81A1B1E
	marbleLinhaSel     = 0x302EC4B6
)

func marbleTrilhasTotais() []boardGroup {
	boardSincronizar()
	return boardFamilias(&boardAtivo)
}

func marblePintarTrafficLights(disp *d2dMotor, app *app, painel rectT) {
	raio := app.sc(11)
	esp := app.sc(22)
	topo := painel.Top + app.sc(20)
	esq := painel.Left + app.sc(20)

	cores := [...]uint32{marbleTLVermelho, marbleTLAmarelo, marbleTLVerde}
	for indice, cor := range cores {
		centroX := esq + int32(indice)*esp
		area := rectT{
			centroX - raio, topo - raio,
			centroX + raio, topo + raio,
		}
		disp.elipse(area, cor)
	}
}

func marblePintarCabecalho(disp *d2dMotor, app *app, painel rectT) int32 {
	marblePintarTrafficLights(disp, app, painel)

	topo := painel.Top + app.sc(52)
	rotulo := appTitle
	if rotulo == "" {
		rotulo = "Marble Overdrive"
	}
	disp.textoUI(rotulo, rectT{
		painel.Left + app.sc(20), topo,
		painel.Right - app.sc(12), topo + app.sc(34),
	}, espCorTitulo, false, false)

	return topo + app.sc(46)
}

func marbleCorFavicon(titulo string) uint32 {
	paleta := [...]uint32{
		0xFF3A6FE0, 0xFF3AA0E0, 0xFF3AC86B, 0xFFE0A93A,
		0xFFE06A3A, 0xFFC83AE0, 0xFF6A3AE0,
	}
	if titulo == "" {
		return paleta[0]
	}

	soma := 0
	for _, letra := range titulo {
		soma += int(letra)
	}

	return paleta[soma%len(paleta)]
}

func marblePintarLinhaTrilha(
	disp *d2dMotor, app *app, painel rectT, grupo boardGroup, topo int32, selecionada bool,
) {
	altura := app.sc(marbleLinhaAlt)
	linha := rectT{painel.Left, topo, painel.Right, topo + altura}

	if selecionada {
		disp.ret(linha, marbleLinhaSel)
	}

	noAtual := boardNo(&boardAtivo, grupo.Raiz)
	rotulo := "Untitled"
	if noAtual != nil && noAtual.Title != "" {
		rotulo = noAtual.Title
	}

	tamFav := app.sc(16)
	favY := topo + (altura-tamFav)/meioD2D
	fav := rectT{
		painel.Left + app.sc(20), favY,
		painel.Left + app.sc(20) + tamFav, favY + tamFav,
	}
	disp.redondo(fav, app.sc(4), marbleCorFavicon(rotulo), 0)

	corTexto := uint32(espCorDim)
	if selecionada {
		corTexto = espCorTitulo
	}
	disp.textoUI(rotulo, rectT{
		fav.Right + app.sc(10), topo,
		linha.Right - app.sc(24), linha.Bottom,
	}, corTexto, true, false)

	if len(grupo.Membros) > 1 {
		disp.textoUI("›", rectT{
			linha.Right - app.sc(22), topo,
			linha.Right - app.sc(6), linha.Bottom,
		}, espCorFaint, false, true)
	}
}

func marblePintarListaTrilhas(disp *d2dMotor, app *app, painel rectT, topo int32) int32 {
	grupos := marbleTrilhasTotais()
	posicao := topo

	for indice, grupo := range grupos {
		marblePintarLinhaTrilha(disp, app, painel, grupo, posicao, indice == marbleUI.linhaFoco)
		posicao += app.sc(marbleLinhaAlt)

		for _, membro := range grupo.Membros[1:] {
			noMembro := boardNo(&boardAtivo, membro)
			if noMembro == nil {
				continue
			}
			sublinha := rectT{painel.Left, posicao, painel.Right, posicao + app.sc(marbleLinhaAlt)}
			disp.textoUI(noMembro.Title, rectT{
				sublinha.Left + app.sc(marbleLinhaIndent*2+20), sublinha.Top,
				sublinha.Right - app.sc(8), sublinha.Bottom,
			}, espCorFaint, true, false)
			posicao += app.sc(marbleLinhaAlt)
		}
	}

	return posicao
}

func marblePintarAppsFixados(disp *d2dMotor, app *app, painel rectT, topo int32) int32 {
	rotulos := [...]string{"W", "P", "K"}
	cores := [...]uint32{0xFFE03A3A, 0xFF3AA0E0, 0xFF6A3AE0}
	tam := app.sc(34)
	esp := app.sc(42)
	esq := painel.Left + app.sc(20)

	for indice, rotulo := range rotulos {
		area := rectT{esq + int32(indice)*esp, topo, esq + int32(indice)*esp + tam, topo + tam}
		disp.redondo(area, app.sc(8), cores[indice], 0)
		disp.textoUI(rotulo, area, 0xFFFFFFFF, true, true)
	}

	return topo + tam + app.sc(18)
}

func marblePintarSidebar(disp *d2dMotor, app *app) {
	if !marbleUI.sidebarAberta {
		return
	}

	painel := marbleSidebarRect(app)
	disp.ret(painel, marbleSidebarFundo)
	disp.contorno(painel, espBordaCard)

	topo := marblePintarCabecalho(disp, app, painel)
	topo = marblePintarAppsFixados(disp, app, painel, topo)

	if topo >= painel.Bottom {
		return
	}

	marblePintarListaTrilhas(disp, app, painel, topo)
}
