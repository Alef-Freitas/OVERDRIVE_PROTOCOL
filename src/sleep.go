package main

import "time"

func (a *app) toggleSonoLite() {
	a.st.SleepLite = !a.st.SleepLite
	a.persist()
	a.aplicarSonoLite()

	if a.st.SleepLite {
		a.status = txtSonoLigadoSt
	} else {
		a.status = txtSonoDesligadoSt
	}

	a.layout()
	a.invalidateBar()
}

func (a *app) ativarSonoLiteNoPane(p *pane) {
	if !a.st.SleepLite || p == nil || !p.ready || p.wv == nil {
		return
	}

	jsSonoOn := `(function(){try{return window.__odSleepOn?window.__odSleepOn():'sem-script';}catch(e){return 'erro';}})()`
	p.wv.EvalJS(jsSonoOn,
		func(error, string) {})
}

func (a *app) desativarSonoLiteNoPane(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	jsSonoOff := `(function(){try{return window.__odSleepOff?window.__odSleepOff():'sem-script';` +
		`}catch(e){return 'erro';}})()`
	p.wv.EvalJS(jsSonoOff,
		func(error, string) {})
}

func (a *app) aplicarSonoLite() {
	if a.st.SleepLite {
		a.ativarSonoLiteNoPane(a.left)

		return
	}

	vistos := map[*pane]bool{}
	for _, t := range a.tabs {
		if t != nil && t.pane != nil && !vistos[t.pane] {
			vistos[t.pane] = true
			a.desativarSonoLiteNoPane(t.pane)
		}
	}

	if a.left != nil && !vistos[a.left] {
		a.desativarSonoLiteNoPane(a.left)
	}
}

func (a *app) limparMemoriaSono(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if time.Since(p.ultimaLimpezaSono) < time.Minute {
		return
	}

	p.ultimaLimpezaSono = time.Now()
	p.wv.CallDevTools("HeapProfiler.collectGarbage", "{}")
	p.wv.CallDevTools("Memory.simulatePressureNotification", `{"level":"critical"}`)

	const atrasoLimpezaSono = 900 * time.Millisecond
	time.AfterFunc(atrasoLimpezaSono, func() { limparWorkingSet() })
}
