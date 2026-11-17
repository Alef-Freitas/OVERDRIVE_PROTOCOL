//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // WebPages: panes extras do demo.

package main

// WebPages: um WebView por card da trilha, mostrando conteúdo real dentro de
// cada card (como no frame 04 do vídeo do Marble).

//nolint:gochecknoglobals // Panes extras do demo: estado de UI single-window.
var marbleWebPanes []*pane

func marbleFecharWebPanes() {
	for _, p := range marbleWebPanes {
		if p != nil && p.wv != nil {
			p.wv.Close()
		}
	}
	marbleWebPanes = nil
}

func marbleCriarWebPane(app *app, indice int, endereco string, area rectT) {
	if app == nil || app.env == nil {
		return
	}

	p := &pane{zoom: app.st.Zoom}
	p.rect = Rect(area)
	p.ready = false

	marbleWebPanes = append(marbleWebPanes, p)

	erro := app.criarWebViewEm(app.env, p, false, nil, nil,
		func(w *WebView) {
			p.ready = true
			w.Navigate(endereco)
			marblePosicionarWebPane(app, indice, area)
		})
	_ = erro
}

func marblePosicionarWebPane(app *app, indice int, area rectT) {
	if indice < 0 || indice >= len(marbleWebPanes) {
		return
	}
	p := marbleWebPanes[indice]
	if p == nil || p.wv == nil || !p.ready {
		return
	}

	nova := Rect(area)
	if p.boundsApplied != nova {
		p.wv.SetBounds(nova)
		p.boundsApplied = nova
	}
	setPaneVisible(p, true)
}

