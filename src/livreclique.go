//go:build windows

package main

const (
	jsLivreOn = `(function(){try{return window.__odLivreOn?window.__odLivreOn():'sem-script';` +
		`}catch(e){return 'erro';}})()`
	jsLivreOff = `(function(){try{return window.__odLivreOff?window.__odLivreOff():'sem-script';` +
		`}catch(e){return 'erro';}})()`
)

// toggleLivreClique liga/desliga o botao direito e o copiar/colar livres.
func (a *app) toggleLivreClique() {
	a.alternarModoPagina(&a.st.LivreClique, a.aplicarLivreClique,
		txtLivreLigadoSt, txtLivreDesligadoSt)
}

// ativarLivreCliqueNoPane libera o documento atual; dorme nos demais.
func (a *app) ativarLivreCliqueNoPane(p *pane) {
	if !a.st.LivreClique {
		return
	}

	avaliarNoPane(p, jsLivreOn)
}

// desativarLivreCliqueNoPane devolve as travas do documento atual.
func (a *app) desativarLivreCliqueNoPane(p *pane) {
	avaliarNoPane(p, jsLivreOff)
}

// aplicarLivreClique liga na aba atual ou desliga em todas.
func (a *app) aplicarLivreClique() {
	if a.st.LivreClique {
		a.ativarLivreCliqueNoPane(a.left)

		return
	}

	a.desligarModoPaginaEmTodos(a.desativarLivreCliqueNoPane)
}

// sincronizarLivreClique aplica o modo ao painel da aba ativada.
func (a *app) sincronizarLivreClique(p *pane) {
	sincronizarModoPagina(p, a.st.LivreClique, a.ativarLivreCliqueNoPane, a.desativarLivreCliqueNoPane)
}
