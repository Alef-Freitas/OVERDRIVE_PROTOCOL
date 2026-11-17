//go:build windows

//nolint:wsl_v5,varnamelen // Camadas de UI: controle de visibilidade dos panes.

package main

// Os panes WebView2 sao janelas filhas que ficam por cima da area de cliente.
// Quando um overlay de UI (menu, paleta, ajuda) abre, os panes devem ser
// escondidos para nao capturar cliques nem cobrir o overlay desenhado em D2D.

func marbleOverlayAtivo() bool {
	return marbleUI.menuAberto || marbleUI.paletaAberta || marbleAjudaAberta
}

//nolint:gochecknoglobals // Estado anterior das camadas para detectar mudanca.
var marbleOverlayAtivoAntes bool

func marbleSincronizarCamadas(app *app) {
	if app == nil {
		return
	}

	esconder := marbleOverlayAtivo()

	if esconder {
		// Esconde o pane principal e os panes extras do demo.
		espOcultarWebView(app)
		for _, p := range marbleWebPanes {
			if p != nil && p.wv != nil && p.ready {
				setPaneVisible(p, false)
			}
		}
	} else if marbleOverlayAtivoAntes {
		// Overlay acabou de fechar: restaura os panes do demo.
		for indice, p := range marbleWebPanes {
			if p != nil && p.wv != nil && p.ready && indice < len(espCanvas.cartoes) {
				marblePosicionarWebPane(app, indice, espCanvas.cartoes[indice].rect)
			}
		}
	}

	marbleOverlayAtivoAntes = esconder
	pInvalidateRect.Call(app.hwnd, 0, 1)
}

// marbleVerificarCamadas detecta mudancas de overlay no paint (rede de seguranca).
func marbleVerificarCamadas(app *app) {
	if marbleOverlayAtivo() != marbleOverlayAtivoAntes {
		marbleSincronizarCamadas(app)
	}
}
