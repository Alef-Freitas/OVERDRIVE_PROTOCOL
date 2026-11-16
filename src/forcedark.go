//go:build windows

package main

const (
	jsForceDarkOn = `(function(){try{return window.__odForceDarkOn?window.__odForceDarkOn():'sem-script';` +
		`}catch(e){return 'erro';}})()`
	jsForceDarkOff = `(function(){try{return window.__odForceDarkOff?window.__odForceDarkOff():'sem-script';` +
		`}catch(e){return 'erro';}})()`
)

// toggleForceDark liga/desliga o modo escuro forcado nas paginas.
func (a *app) toggleForceDark() {
	a.alternarModoPagina(&a.st.ForceDark, a.aplicarForceDark,
		txtForceDarkLigadoSt, txtForceDarkDesligadoSt)
}

// ativarForceDarkNoPane escurece o documento atual.
func (a *app) ativarForceDarkNoPane(p *pane) {
	if !a.st.ForceDark {
		return
	}

	avaliarNoPane(p, jsForceDarkOn)
}

// desativarForceDarkNoPane devolve o documento atual ao normal.
func (a *app) desativarForceDarkNoPane(p *pane) {
	avaliarNoPane(p, jsForceDarkOff)
}

// aplicarForceDark liga na aba atual ou desliga em todas.
func (a *app) aplicarForceDark() {
	if a.st.ForceDark {
		a.ativarForceDarkNoPane(a.left)

		return
	}

	a.desligarModoPaginaEmTodos(a.desativarForceDarkNoPane)
}

// sincronizarForceDark aplica o modo ao painel da aba ativada.
func (a *app) sincronizarForceDark(p *pane) {
	sincronizarModoPagina(p, a.st.ForceDark, a.ativarForceDarkNoPane, a.desativarForceDarkNoPane)
}
