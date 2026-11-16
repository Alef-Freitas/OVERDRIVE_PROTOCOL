package main

import "time"

const (
	// A limpeza espera o scroll parar para nao competir com o carregamento
	// da pagina durante a rolagem.
	scrollLimpezaEspera = 1200 * time.Millisecond
	// Mesmo com scroll continuo, nao vale a pena coletar lixo mais de uma vez
	// a cada intervalo: o ganho e pequeno e o custo aparece como engasgo.
	scrollLimpezaIntervalo = 30 * time.Second
)

// agendarLimpezaScroll e chamado sempre que qualquer painel rola. A limpeza e
// global: coleta o lixo de todos os WebViews e, depois, esvazia o working set
// de todos os processos descendentes do Overdrive (inclusive os renderers do
// WebView2). Assim, rolar uma pagina tambem devolve a memoria que ficou presa
// nos paineis ocultos.
// scrollPodeLimpar diz se a limpeza por scroll pode ser agendada agora.
func (a *app) scrollPodeLimpar(p *pane) bool {
	if a == nil || !a.st.EconomiaMax || a.tudoSuspenso || a.quitting {
		return false
	}

	return p != nil && p.ready && p.wv != nil
}

// dispararLimpezaScroll agenda a coleta global apos a rolagem parar.
func (a *app) dispararLimpezaScroll() {
	var timer *time.Timer

	timer = time.AfterFunc(scrollLimpezaEspera, func() {
		a.depois(func() {
			if a.scrollLimpezaTimer != timer {
				return
			}

			a.scrollLimpezaTimer = nil
			if !a.ultimaLimpezaScroll.IsZero() && time.Since(a.ultimaLimpezaScroll) < scrollLimpezaIntervalo {
				return
			}

			a.ultimaLimpezaScroll = time.Now()
			a.limpezaPesadaMemoria()
		})
	})
	a.scrollLimpezaTimer = timer
}

func (a *app) agendarLimpezaScroll(p *pane) {
	if !a.scrollPodeLimpar(p) {
		return
	}

	if a.scrollLimpezaTimer != nil {
		a.scrollLimpezaTimer.Stop()
	}

	a.dispararLimpezaScroll()
}
