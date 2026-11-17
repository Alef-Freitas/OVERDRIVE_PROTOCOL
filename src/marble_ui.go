//go:build windows

//nolint:wsl_v5,exhaustruct_v5,cyclop // UI: nomes curtos e literais origem layout intencionais.

package main

import (
	"net/url"
	"strings"
)

// Elementos visiveis do Marble desenhados em Direct2D: sidebar origem trilhas e
// paleta origem comandos. Go puro, sem cgo; a navegacao usa o modelo board.

const (
	marbleSidebarLargura = 248
	marbleCabecalhoAlt   = 40
	marbleLinhaAlt       = 30
	marbleLinhaIndent    = 18
	marblePaletaLargura  = 520
	marblePaletaAltura   = 360
	marblePaletaRaio     = 12
	marbleCampoAlt       = 40
	marbleItemAlt        = 34
	marbleBordaEspessa   = 2
)

type marbleUIEstado struct {
	sidebarAberta bool
	paletaAberta  bool
	menuAberto    bool
	menuX         int32
	menuY         int32
	busca         string
	linhaFoco     int
	sugestoes     []string
}

//nolint:gochecknoglobals // Estado da UI single-window.
var marbleUI marbleUIEstado

func marbleIniciar() {
	marbleUI = marbleUIEstado{
		sidebarAberta: true,
		paletaAberta:  false,
		menuAberto:    false,
		busca:         "",
		linhaFoco:     0,
	}
}

func marbleSidebarRect(app *app) rectT {
	area := app.clientRect()
	return rectT{
		area.Left, area.Top,
		area.Left + app.sc(marbleSidebarLargura), area.Bottom,
	}
}

func marblePaletaRect(app *app) rectT {
	area := app.clientRect()
	largura := app.sc(marblePaletaLargura)
	altura := app.sc(marblePaletaAltura)
	esq := area.Left + (area.Width()-largura)/meioD2D
	topo := area.Top + app.sc(80)
	return rectT{esq, topo, esq + largura, topo + altura}
}

func marbleComandos() []string {
	return []string{
		"Tidy Up", "Fold Trail", "Fit to View", "Next on Trail",
		"Previous on Trail", "Undo", "Redo", "New Card",
		"Toggle Trails Panel", "Toggle Transparency", "Show Trails",
	}
}

func marblePintarPaleta(disp *d2dMotor, app *app) {
	if !marbleUI.paletaAberta {
		return
	}

	caixa := marblePaletaRect(app)
	disp.redondo(caixa, app.sc(marblePaletaRaio), espFundoCard, espBordaFoco)

	campo := rectT{
		caixa.Left + app.sc(12), caixa.Top + app.sc(12),
		caixa.Right - app.sc(12), caixa.Top + app.sc(12) + app.sc(marbleCampoAlt),
	}
	disp.redondo(campo, app.sc(8), espFundoCard, espBordaCard)
	textoBusca := marbleUI.busca
	if textoBusca == "" {
		textoBusca = "Search, commands, and Import..."
	}
	disp.textoUI(textoBusca, rectT{
		campo.Left + app.sc(12), campo.Top,
		campo.Right - app.sc(12), campo.Bottom,
	}, espCorDim, false, false)

	comandos := marbleComandos()
	topo := campo.Bottom + app.sc(12)
	for indice, comando := range comandos {
		item := rectT{
			caixa.Left + app.sc(12), topo + int32(indice)*app.sc(marbleItemAlt),
			caixa.Right - app.sc(12), topo + (int32(indice)+1)*app.sc(marbleItemAlt),
		}
		cor := uint32(espCorDim)
		if indice == marbleUI.linhaFoco {
			disp.redondo(item, app.sc(6), espCorSelecao, 0)
			cor = espCorTitulo
		}
		disp.textoUI(comando, rectT{
			item.Left + app.sc(12), item.Top,
			item.Right - app.sc(8), item.Bottom,
		}, cor, false, false)
	}
}

// marbleNomeSite extrai o host legivel da URL, como o site_name do Marble.
func marbleNomeSite(endereco string) string {
	pendente := endereco
	if !strings.Contains(pendente, "://") {
		pendente = "https://" + pendente
	}

	parseada, erro := url.Parse(pendente)
	if erro != nil || parseada.Hostname() == "" {
		return endereco
	}

	host := strings.TrimPrefix(parseada.Hostname(), "www.")
	if partes := strings.Split(host, "."); len(partes) > 2 {
		return strings.Join(partes[len(partes)-2:], ".")
	}

	return host
}

func marbleAtualizarSugestoes(texto string) {
	marbleUI.sugestoes = nil
	if texto == "" {
		return
	}
	minúsculo := strings.ToLower(texto)
	for _, card := range espCanvas.cartoes {
		if card.titulo == "" || card.ehNota {
			continue
		}
		if strings.HasPrefix(strings.ToLower(card.titulo), minúsculo) {
			marbleUI.sugestoes = append(marbleUI.sugestoes, card.titulo)
		}
	}
}

func marblePintarSugestoes(disp *d2dMotor, app *app) {
	if espCanvas.editando < 0 || len(marbleUI.sugestoes) == 0 {
		return
	}

	area := espAreaURL(espCanvas.editando)
	if area.Width() <= 0 {
		return
	}

	alturaItem := app.sc(26)
	topo := area.Bottom + app.sc(4)
	caixa := rectT{
		area.Left, topo,
		area.Right, topo + alturaItem*int32(len(marbleUI.sugestoes)),
	}
	disp.redondo(caixa, app.sc(6), marbleMenuFundo, espBordaCard)

	for indice, sugestao := range marbleUI.sugestoes {
		linha := rectT{
			caixa.Left, topo + int32(indice)*alturaItem,
			caixa.Right, topo + (int32(indice)+1)*alturaItem,
		}
		disp.textoUI(sugestao, rectT{
			linha.Left + app.sc(10), linha.Top,
			linha.Right - app.sc(8), linha.Bottom,
		}, espCorDim, true, false)
	}
}

func marblePintar(disp *d2dMotor, app *app) {
	marblePintarSidebar(disp, app)
	marblePintarPaleta(disp, app)
	marblePintarMenu(disp, app)
	marblePintarSugestoes(disp, app)
	marblePintarOverlay(disp, app)
	marblePintarAjuda(disp, app)
}

// marbleComandosFiltrados devolve so os comandos que casam com a busca.
func marbleComandosFiltrados() []string {
	todos := marbleComandos()
	if marbleUI.busca == "" {
		return todos
	}

	var filtrados []string
	for _, comando := range todos {
		if strings.Contains(strings.ToLower(comando), strings.ToLower(marbleUI.busca)) {
			filtrados = append(filtrados, comando)
		}
	}

	return filtrados
}

func marbleExecutarComando(app *app, comando string) {
	switch comando {
	case "Tidy Up":
		boardTidy()
	case "Fold Trail":
		boardDobrar()
	case "Fit to View":
		boardAjustarView(app)
	case "Next on Trail":
		boardNavegar(app, false)
	case "Previous on Trail":
		boardNavegar(app, true)
	case "Undo":
		boardDesfazer(app)
	case "Redo":
		boardRefazer(app)
	case "New Card":
		marbleNovaCard(app)
	case "Toggle Trails Panel":
		marbleUI.sidebarAberta = !marbleUI.sidebarAberta
	case "Toggle Transparency":
		espCanvas.sliderVisivel = !espCanvas.sliderVisivel
	case "Show Trails":
		marbleUI.sidebarAberta = true
	}

	pInvalidateRect.Call(app.hwnd, 0, 1)
}

func marbleNovaCard(app *app) {
	area := app.clientRect()
	largura := app.sc(240)
	altura := app.sc(160)
	esq := area.Left + area.Width()/meioD2D - largura/meioD2D
	topo := area.Top + area.Height()/meioD2D - altura/meioD2D

	espAdicionar(rectT{esq, topo, esq + largura, topo + altura})
	marbleUI.paletaAberta = false
	espMostrarEdit(app, espCanvas.editando)
}

// marbleTecla processa o teclado da paleta. Devolve true quando consumiu a tecla.
func marbleTecla(app *app, tecla, bruta uint32) bool {
	ctrl := getKeyDown(vkControl)

	if tecla == teclaTeclaK && ctrl {
		marbleUI.paletaAberta = !marbleUI.paletaAberta
		marbleUI.busca = ""
		marbleUI.linhaFoco = 0
		pInvalidateRect.Call(app.hwnd, 0, 1)

		return true
	}

	if !marbleUI.paletaAberta {
		return false
	}

	filtrados := marbleComandosFiltrados()

	switch tecla {
	case vkEscape:
		marbleUI.paletaAberta = false
		marbleUI.busca = ""
	case vkUp:
		if marbleUI.linhaFoco > 0 {
			marbleUI.linhaFoco--
		}
	case vkDown:
		if marbleUI.linhaFoco < len(filtrados)-1 {
			marbleUI.linhaFoco++
		}
	case vkReturn:
		if marbleUI.linhaFoco >= 0 && marbleUI.linhaFoco < len(filtrados) {
			comando := filtrados[marbleUI.linhaFoco]
			marbleUI.paletaAberta = false
			marbleUI.busca = ""
			marbleExecutarComando(app, comando)
		}
	case vkBackspace:
		if len(marbleUI.busca) > 0 {
			marbleUI.busca = marbleUI.busca[:len(marbleUI.busca)-1]
			marbleUI.linhaFoco = 0
		}
	default:
		if !ctrl && bruta >= espPrimeiroCar && bruta <= 0x7E {
			marbleUI.busca += string(rune(bruta))
			marbleUI.linhaFoco = 0
		}
	}

	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}

// marbleClique processa cliques na sidebar e na paleta. Devolve true se consumiu.
func marbleClique(app *app, x, posicaoY int32) bool {
	if marbleCliqueMenu(app, x, posicaoY) {
		return true
	}

	if marbleUI.paletaAberta {
		caixa := marblePaletaRect(app)
		itemTopo := caixa.Top + app.sc(12) + app.sc(marbleCampoAlt) + app.sc(12)
		filtrados := marbleComandosFiltrados()

		for indice := range filtrados {
			item := rectT{
				caixa.Left + app.sc(12), itemTopo + int32(indice)*app.sc(marbleItemAlt),
				caixa.Right - app.sc(12), itemTopo + (int32(indice)+1)*app.sc(marbleItemAlt),
			}
			if espDentro(item, x, posicaoY) {
				comando := filtrados[indice]
				marbleUI.paletaAberta = false
				marbleUI.busca = ""
				marbleExecutarComando(app, comando)

				return true
			}
		}
		if espDentro(caixa, x, posicaoY) {
			return true
		}

		marbleUI.paletaAberta = false
		pInvalidateRect.Call(app.hwnd, 0, 1)
	}

	if marbleUI.sidebarAberta && espDentro(marbleSidebarRect(app), x, posicaoY) {
		area := marbleSidebarRect(app)
		topo := area.Top + app.sc(marbleCabecalhoAlt)
		grupos := boardFamilias(&boardAtivo)
		for indice, grupo := range grupos {
			linha := rectT{
				area.Left, topo + int32(indice)*app.sc(marbleLinhaAlt),
				area.Right, topo + (int32(indice)+1)*app.sc(marbleLinhaAlt),
			}
			if espDentro(linha, x, posicaoY) {
				marbleUI.linhaFoco = indice
				noAtual := boardNo(&boardAtivo, grupo.Raiz)
				if noAtual != nil {
					espCanvas.foco = boardIndiceDo(noAtual.ID)
					pInvalidateRect.Call(app.hwnd, 0, 1)
				}

				return true
			}
		}

		return true
	}

	return false
}
