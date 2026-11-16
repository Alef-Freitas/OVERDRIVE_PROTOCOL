//go:build windows

package main

// avaliarNoPane roda um script no documento atual, se houver um pronto.
func avaliarNoPane(p *pane, js string) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	p.wv.EvalJS(js, func(error, string) {})
}

// alternarModoPagina troca o estado, persiste, aplica e anuncia na barra.
func (a *app) alternarModoPagina(ligado *bool, aplicar func(), ligadoSt, desligadoSt string) {
	*ligado = !*ligado

	a.persist()
	aplicar()

	if *ligado {
		a.status = ligadoSt
	} else {
		a.status = desligadoSt
	}

	a.layout()
	a.invalidateBar()
}

// desligarModoPaginaEmTodos devolve cada painel visto ao modo normal.
func (a *app) desligarModoPaginaEmTodos(desligar func(*pane)) {
	vistos := map[*pane]bool{}

	for _, t := range a.tabs {
		if t != nil && t.pane != nil && !vistos[t.pane] {
			vistos[t.pane] = true
			desligar(t.pane)
		}
	}

	if a.left != nil && !vistos[a.left] {
		desligar(a.left)
	}
}

// sincronizarModoPagina aplica o modo ao painel da aba ativada.
func sincronizarModoPagina(p *pane, ligado bool, ativar, desativar func(*pane)) {
	if p == nil || !p.ready {
		return
	}

	if ligado {
		ativar(p)
	} else {
		desativar(p)
	}
}
