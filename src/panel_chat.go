//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Constantes locais do painel de chat (esperas, limites e fatores sem outro dono).
const (
	esperaLimpezaAposPainel    = 1500 * time.Millisecond
	esperaProgressoEnvio       = 40 * time.Millisecond
	minimoPartesProgressivo    = 2
	limiteOcupacaoChat         = 4
	esperaConferePreenchimento = 350 * time.Millisecond
	esperaMaximaConferencia    = 9 * time.Second
	divisorAtrasoChars         = 40
	tentativasReenvioEnter     = 2
	esperaLeituraChat          = 30 * time.Second
	esperaAvisoPaginaGrande    = 6 * time.Second
	esperaLimpezaSoIA          = 600 * time.Millisecond
	fatorPorcentagemZoom       = 100
	ajusteArredondaZoom        = 0.5
)

// retomarPainelChatPronto retoma o chat ja criado; true se tratou.
func (a *app) retomarPainelChatPronto() bool {
	if !a.right.ready {
		return false
	}

	a.retomarChatGPTFrio()
	a.right.wv.Resume()

	return true
}

// criarPainelChatNovo cria o WebView do painel de IA.
func (a *app) criarPainelChatNovo() {
	a.chatUsado = false
	a.chatAtivo = false
	a.chatGerando = false

	a.chatFrioPendente = false
	if err := a.criarWebView(a.right, true,
		func() bool { return a.st.RightVisible },
		nil,
		func(w *WebView) {
			url := a.st.RightURL
			if !urlUtilizavel(url) || !a.siteAtualIA().aceitaURL(url) {
				url = a.siteAtualIA().url
			}

			a.right.queriaURL = url
			w.Navigate(url)
			a.loadExtensions()
			a.depois(func() { a.pararServicosDeFundo() })
		}); err != nil {
		a.dizerErrof(txtFalhaCriarAba)
	}
}

func (a *app) abrirPainelChat() {
	if a.env == nil || a.hwnd == 0 {
		return
	}

	if a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil && a.st.ImgVisible {
		a.imgPanel.wv.Resume()
	}

	if a.right == nil {
		a.right = &pane{zoom: a.st.Zoom, mobile: true}
	}

	if a.retomarPainelChatPronto() {
		return
	}

	a.criarPainelChatNovo()
}

func (a *app) fecharPainelChat() {
	a.soIA = false
	a.chatFrioGen++
	a.chatFrioPendente = false
	a.chatUsado = false
	a.chatAtivo = false
	a.chatGerando = false
	a.chatManual = false

	a.envioGen++
	a.leituraGen++
	a.leituraAtiva = false
	a.envioAtivo = false
	a.chatPendente = nil

	a.anexosPendentes1 = nil
	if a.right != nil {
		a.right.frio = false
		a.right.frioManual = false
	}

	if a.chatFrio {
		a.chatFrio = false
		pKillTimer.Call(a.hwnd, timerChatFrio)
	}

	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.Resume()
	}

	a.fecharPainelImagens()

	if a.right == nil {
		return
	}

	a.destruirPane(a.right)
	a.agendarLimpezaMemoria(true, esperaLimpezaAposPainel)
}

// voltarChatParaIA devolve o painel para a IA e abre a URL fora.
func (a *app) voltarChatParaIA(p *pane, url string, site siteIA, jaAberta int) {
	if p.ready && p.wv != nil {
		p.queriaURL = site.url
		p.wv.Navigate(site.url)
	}

	if jaAberta >= 0 {
		a.ativarAba(jaAberta)
	} else {
		a.novaAba(url, true)
	}

	a.status = fmt.Sprintf(txtFmtPainelEsq, site.nome)
	a.invalidateBar()
}

// recolherChatParaIA decide se a URL externa abre em aba e volta o painel.
func (a *app) recolherChatParaIA(p *pane, url string, abrir bool, jaAberta int) {
	site := a.siteAtualIA()
	if !abrir && jaAberta < 0 {
		a.depois(func() {
			if p.ready && p.wv != nil {
				p.queriaURL = site.url
				p.wv.Navigate(site.url)
			}
		})

		return
	}

	if p == a.right {
		a.chatTravando = true
	}

	a.depois(func() { a.voltarChatParaIA(p, url, site, jaAberta) })
}

func (a *app) travarChatNaURL(p *pane, url string) {
	site := a.siteAtualIA()
	if site.aceitaURL(url) || strings.HasPrefix(url, "about:") || url == "" {
		if p == a.right {
			a.chatTravando = false
		}

		return
	}

	if p == a.right && a.chatTravando {
		return
	}

	abrir, jaAberta := a.podeAbrirAbaDoChat(url)
	a.recolherChatParaIA(p, url, abrir, jaAberta)
}

func (a *app) acaoDeIA(id string) bool {
	if after, ok := strings.CutPrefix(id, "ia:"); ok {
		a.trocarSiteIA(after)

		return true
	}

	return false
}

const jsEnvioVisivel = `function visivel(e) {
    if (!e) { return false; }
    var r = e.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) { return false; }
    var cs = getComputedStyle(e);
    return cs.visibility !== 'hidden' && cs.display !== 'none' && cs.opacity !== '0';
  }`

const jsEnvioCampo = `function achaCampo() {
    var t = [];
    if (selCampo) { t.push(selCampo); }
    t.push('textarea', 'div[role="textbox"]', '[contenteditable="true"]', 'input[type="text"]');
    for (var i = 0; i < t.length; i++) {
      var lista = [];
      try { lista = [].slice.call(document.querySelectorAll(t[i])); } catch (e) { lista = []; }
      for (var j = 0; j < lista.length; j++) {
        if (visivel(lista[j]) && !lista[j].disabled &&
          !lista[j].readOnly && ehCampoTexto(lista[j])) { return lista[j]; }
      }
    }
    return null;
  }`

const jsEnvioBotao = `function achaBotao() {
    if (selEnviar) {
      var exatos = [];
      try { exatos = [].slice.call(document.querySelectorAll(selEnviar)); } catch (e) { exatos = []; }
      for (var e = 0; e < exatos.length; e++) {
        if (visivel(exatos[e]) && !exatos[e].disabled) {
          var c = coordenadasSeguras(exatos[e]);
          if (c) { return c; }
        }
      }
    }
    var botoes = [];
    try { botoes = [].slice.call(document.querySelectorAll('button,[role="button"]')); } catch (e) { botoes = []; }
    var melhor = null, nota = -1;
    for (var i = 0; i < botoes.length; i++) {
      var b = botoes[i];
      if (!visivel(b) || b.disabled) { continue; }
      var t = String(b.getAttribute('aria-label') || '') + ' ' + String(b.getAttribute('title') || '') + ' ' +
              String(b.getAttribute('data-testid') || '') + ' ' + String(b.id || '') + ' ' + String(b.className || '');
      if (!/send|enviar|submit/i.test(t)) { continue; }
      var r = b.getBoundingClientRect();
      var n = r.bottom * 1000 + r.right;
      if (n > nota && coordenadasSeguras(b)) { nota = n; melhor = b; }
    }
    if (!melhor) { return null; }
    return coordenadasSeguras(melhor);
  }`

func prepararEnvioJS(s siteIA) string {
	//nolint:errchkjson // Marshal de string e infalivel; sem erro possivel.
	campo, _ := json.Marshal(s.campo)
	//nolint:errchkjson // Marshal de string e infalivel; sem erro possivel.
	enviar, _ := json.Marshal(s.enviar)

	return fmt.Sprintf(`(function(){
  var selCampo = %s;
  var selEnviar = %s;
  `+jsEnvioVisivel+`
  `+jsEnvioCampo+`
  // Só elementos de texto: clicar num input errado (ex.: file) abre
  // filepicker em vez de focar o chat.
  function ehCampoTexto(e) {
    if (!e) { return false; }
    var tag = String(e.tagName || '').toUpperCase();
    if (tag === 'TEXTAREA') { return true; }
    if (tag === 'INPUT') {
      var tp = String(e.type || 'text').toLowerCase();
      return tp === 'text' || tp === 'search' || tp === 'email' || tp === 'url';
    }
    if (e.isContentEditable) { return true; }
    var role = String(e.getAttribute && e.getAttribute('role') || '').toLowerCase();
    return role === 'textbox' || role === 'searchbox';
  }
  function coordenadasSeguras(b) {
    var rc = b.getBoundingClientRect();
    var x = rc.left + rc.width / 2, y = rc.top + rc.height / 2;
    // O layout mexe entre localizar e clicar: se outro elemento cobre o
    // ponto (ex.: anexar sobre o enviar), não clica no escuro.
    var topo = null;
    try { topo = document.elementFromPoint(x, y); } catch (e) { topo = null; }
    if (topo) {
      var cur = topo, dentro = false;
      for (var i = 0; i < 4 && cur; i++) {
        if (cur === b) { dentro = true; break; }
        cur = cur.parentElement;
      }
      if (!dentro) { return null; }
    }
    return {x: x, y: y};
  }
  `+jsEnvioBotao+`
  var el = achaCampo();
  if (!el) { return {ok:false, err:'campo de mensagem não encontrado'}; }
  try { el.focus(); el.click(); } catch (e) {}
  var b = achaBotao();
  return {ok:true, botao: !!b, bx: b ? b.x : 0, by: b ? b.y : 0};
})()`, string(campo), string(enviar))
}

func conferirEnvioJS(s siteIA, tamanho int) string {
	//nolint:errchkjson // Marshal de string e infalivel; sem erro possivel.
	campo, _ := json.Marshal(s.campo)

	return fmt.Sprintf(`(function(){
  var selCampo = %s, esperado = %d;
  function visivel(e) {
    if (!e) { return false; }
    var r = e.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) { return false; }
    var cs = getComputedStyle(e);
    return cs.visibility !== 'hidden' && cs.display !== 'none';
  }
  var t = [];
  if (selCampo) { t.push(selCampo); }
  t.push('textarea', 'div[role="textbox"]', '[contenteditable="true"]', 'input[type="text"]');
  for (var i = 0; i < t.length; i++) {
    var lista = [];
    try { lista = [].slice.call(document.querySelectorAll(t[i])); } catch (e) { lista = []; }
    for (var j = 0; j < lista.length; j++) {
      var el = lista[j];
      if (!visivel(el)) { continue; }
      var v = String(el.value !== undefined ? el.value : el.innerText || '').trim();
      return {ok:true, restou: v.length > esperado * 0.6};
    }
  }
  return {ok:false};
})()`, string(campo), tamanho)
}

func (a *app) novoEnvio() uint64 {
	a.envioGen++

	return a.envioGen
}

func (a *app) inserirTextoProgressivo(p *pane, partes []string, gen uint64, rotulo string, indice int, aoFim func()) {
	if gen != a.envioGen {
		return
	}

	if p == nil || p.wv == nil || !p.ready {
		a.envioAtivo = false
		a.status = txtEnvioCancelado
		a.invalidateBar()

		return
	}

	if indice >= len(partes) {
		a.envioAtivo = false

		if aoFim != nil {
			aoFim()
		}

		return
	}

	if params, err := cdpInsertText(partes[indice]); err == nil {
		p.wv.CallDevTools("Input.insertText", params)
	}

	if n := len(partes); n > 1 {
		a.dizerProgressof((indice+1)*100/n, "%s %d/%d...", rotulo, indice+1, n)
	}

	time.AfterFunc(esperaProgressoEnvio, func() {
		a.depois(func() { a.inserirTextoProgressivo(p, partes, gen, rotulo, indice+1, aoFim) })
	})
}

// interpretarPreparoChat extrai o preparo do JS de envio.
func interpretarPreparoChat(err error, res string) (envioPreparado, string, bool) {
	var prep struct {
		OK    bool    `json:"ok"`
		Erro  string  `json:"err"`
		Botao bool    `json:"botao"`
		BX    float64 `json:"bx"`
		BY    float64 `json:"by"`
	}
	if err != nil || desembrulharEvalJS(res, &prep) != nil || !prep.OK {
		motivo := prep.Erro
		if err != nil {
			motivo = err.Error()
		}

		if motivo == "" {
			motivo = "campo de mensagem não encontrado"
		}

		return envioPreparado{}, motivo, false
	}

	return envioPreparado{Botao: prep.Botao, BX: prep.BX, BY: prep.BY}, "", true
}

// medirOcupacaoChat conta os chars ja presentes no campo.
func medirOcupacaoChat(err error, res string) int {
	if err != nil {
		return 0
	}

	var resposta struct {
		OK    bool `json:"ok"`
		Chars int  `json:"chars"`
	}
	if desembrulharEvalJS(res, &resposta) != nil || !resposta.OK {
		return 0
	}

	return resposta.Chars
}

// despejarTextoCurtoChat insere de uma vez textos de parte unica.
func despejarTextoCurtoChat(p *pane, partes []string) {
	for _, parte := range partes {
		if params, err := cdpInsertText(parte); err == nil {
			p.wv.CallDevTools("Input.insertText", params)
		}
	}
}

// finalizarPreenchimentoChat confere o texto e devolve o preparo.
func (a *app) finalizarPreenchimentoChat(p *pane, s siteIA, prep envioPreparado, aoPronto func(envioPreparado)) {
	a.conferirPreenchimento(p, s, 0, func(chars int) {
		a.envioAtivo = false

		r := envioPreparado{Botao: prep.Botao, BX: prep.BX, BY: prep.BY, Chars: chars}
		if chars == 0 {
			r.Erro = "o chat não aceitou o texto"
		}

		aoPronto(r)
	})
}

// despejarTextoNoChat escolhe entre insercao direta ou progressiva.
func (a *app) despejarTextoNoChat(p *pane, site siteIA, texto string,
	prep envioPreparado, aoPronto func(envioPreparado),
) {
	conferir := func() {
		if p.wv == nil || !p.ready {
			a.envioAtivo = false

			aoPronto(envioPreparado{Erro: txtPainelIaIndisponivel})

			return
		}

		a.finalizarPreenchimentoChat(p, site, prep, aoPronto)
	}

	partes := pedacos(texto, pedacoEnvio)
	if len(partes) < minimoPartesProgressivo {
		despejarTextoCurtoChat(p, partes)
		conferir()

		return
	}

	a.inserirTextoProgressivo(p, partes, a.novoEnvio(), "preenchendo o chat", 0, conferir)
}

// ocuparCampoChat mede ocupacao e despeja o texto quando livre.
func (a *app) ocuparCampoChat(p *pane, site siteIA, texto string,
	prep envioPreparado, antes int, aoPronto func(envioPreparado),
) {
	if p.wv == nil || !p.ready {
		aoPronto(envioPreparado{Erro: txtPainelIaIndisponivel})

		return
	}

	if antes > limiteOcupacaoChat {
		a.dizerf(txtFmtChatOcupado, formataMilhar(antes))
		a.invalidateBar()
		aoPronto(envioPreparado{Ocupado: true, Chars: antes})

		return
	}

	a.envioAtivo = true
	a.despejarTextoNoChat(p, site, texto, prep, aoPronto)
}

// sondarOcupacaoChat mede o campo antes de preencher.
func (a *app) sondarOcupacaoChat(p *pane, site siteIA, texto string,
	prep envioPreparado, aoPronto func(envioPreparado),
) {
	p.wv.EvalJS(conferirPreenchimentoJS(site), func(err error, res string) {
		antes := medirOcupacaoChat(err, res)

		a.depois(func() { a.ocuparCampoChat(p, site, texto, prep, antes, aoPronto) })
	})
}

func (a *app) preencherNoChat(p *pane, site siteIA, texto string, aoPronto func(envioPreparado)) {
	if aoPronto == nil {
		return
	}

	pronto := func(r envioPreparado) {
		a.depois(func() { aoPronto(r) })
	}
	if p == nil || !p.ready || p.wv == nil {
		pronto(envioPreparado{Erro: txtPainelIaIndisponivel})

		return
	}

	p.wv.EvalJS(prepararEnvioJS(site), func(err error, res string) {
		prep, motivo, ok := interpretarPreparoChat(err, res)
		if !ok {
			pronto(envioPreparado{Erro: motivo})

			return
		}

		a.depois(func() {
			if p.wv == nil || !p.ready {
				aoPronto(envioPreparado{Erro: txtPainelIaIndisponivel})

				return
			}

			a.sondarOcupacaoChat(p, site, texto, prep, aoPronto)
		})
	})
}

// extrairCharsPreenchimento conta os chars do retorno do JS.
func extrairCharsPreenchimento(err error, res string) int {
	if err != nil {
		return 0
	}

	var resposta struct {
		OK    bool `json:"ok"`
		Chars int  `json:"chars"`
	}
	if desembrulharEvalJS(res, &resposta) != nil || !resposta.OK {
		return 0
	}

	return resposta.Chars
}

// tratarRetornoPreenchimento decide entre repetir ou concluir.
func (a *app) tratarRetornoPreenchimento(p *pane, site siteIA, tentativa int, aoPronto func(int), chars int) {
	if p == nil || p.wv == nil || !p.ready {
		aoPronto(0)

		return
	}

	if chars == 0 && tentativa < 8 {
		a.conferirPreenchimento(p, site, tentativa+1, aoPronto)

		return
	}

	aoPronto(chars)
}

func (a *app) conferirPreenchimento(p *pane, site siteIA, tentativa int, aoPronto func(int)) {
	time.AfterFunc(esperaConferePreenchimento, func() {
		a.depois(func() {
			if p == nil || p.wv == nil || !p.ready {
				aoPronto(0)

				return
			}

			p.wv.EvalJS(conferirPreenchimentoJS(site), func(err error, res string) {
				chars := extrairCharsPreenchimento(err, res)

				a.tratarRetornoPreenchimento(p, site, tentativa, aoPronto, chars)
			})
		})
	})
}

func conferirPreenchimentoJS(site siteIA) string {
	//nolint:errchkjson // Marshal de string e infalivel; sem erro possivel.
	campo, _ := json.Marshal(site.campo)

	return fmt.Sprintf(`(function(){
  var selCampo = %s;
  function visivel(e) {
    if (!e) { return false; }
    var r = e.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) { return false; }
    var cs = getComputedStyle(e);
    return cs.visibility !== 'hidden' && cs.display !== 'none' && cs.opacity !== '0';
  }
  var t = [];
  if (selCampo) { t.push(selCampo); }
  t.push('textarea', 'div[role="textbox"]', '[contenteditable="true"]', 'input[type="text"]');
  for (var i = 0; i < t.length; i++) {
    var lista = [];
    try { lista = [].slice.call(document.querySelectorAll(t[i])); } catch (e) { lista = []; }
    for (var j = 0; j < lista.length; j++) {
      if (!visivel(lista[j])) { continue; }
      var v = String(lista[j].value !== undefined ? lista[j].value : lista[j].innerText || '');
      return {ok:true, chars:v.length};
    }
  }
  return {ok:false, chars:0};
})()`, string(campo))
}

func delayConferencia(n int) time.Duration {
	d := min(1500*time.Millisecond+time.Duration(n/divisorAtrasoChars)*time.Millisecond, esperaMaximaConferencia)

	return d
}

func (a *app) conferirEnvio(p *pane, site siteIA, tamanho, tentativa int) {
	time.AfterFunc(delayConferencia(tamanho), func() {
		a.depois(func() {
			a.verificarEnvio(p, site, tamanho, tentativa)
		})
	})
}

// verificarEnvio confere via JS se o texto foi consumido; reenvia ou avisa.
func (a *app) verificarEnvio(p *pane, site siteIA, tamanho, tentativa int) {
	if p.wv == nil || !p.ready {
		return
	}

	p.wv.EvalJS(conferirEnvioJS(site, tamanho), func(err error, res string) {
		if err != nil {
			return
		}

		var resposta struct {
			OK     bool `json:"ok"`
			Restou bool `json:"restou"`
		}
		if desembrulharEvalJS(res, &resposta) != nil || !resposta.OK {
			return
		}

		if resposta.Restou {
			a.tratarEnvioIncompleto(p, site, tamanho, tentativa)
		}
	})
}

// tratarEnvioIncompleto reenvia Enter ate 2 vezes ou avisa que nao enviou.
func (a *app) tratarEnvioIncompleto(p *pane, site siteIA, tamanho, tentativa int) {
	if tentativa < tentativasReenvioEnter {
		a.depois(func() {
			if p.wv == nil || !p.ready {
				return
			}

			a.teclarEnterCDP(p)
			a.conferirEnvio(p, site, tamanho, tentativa+1)
		})

		return
	}

	a.depois(func() {
		a.dizerf(txtFmtNaoEnviou, site.nome)
	})
}

func (a *app) criarFindOptionsEm(env *Environment) *ICoreWebView2FindOptions {
	if env == nil || env.ptr == nil {
		return nil
	}

	p := queryInterface(unsafe.Pointer(env.ptr), IIDICoreWebView2Environment15)
	if p == nil {
		return nil
	}

	env15 := (*ICoreWebView2Environment15)(p)
	defer env15.vtbl.Release.Call(uintptr(unsafe.Pointer(env15)))

	var opt *ICoreWebView2FindOptions

	hr, _, _ := env15.vtbl.CreateFindOptions.Call(uintptr(unsafe.Pointer(env15)), uintptr(unsafe.Pointer(&opt)))
	if Failed(hr) || opt == nil {
		return nil
	}

	opt.vtbl.PutSuppressDefaultFindDialog.Call(uintptr(unsafe.Pointer(opt)), 0)

	return opt
}

func (a *app) abrirBuscaNativa() {
	p := a.focused()
	if p == nil || !p.ready || p.wv == nil {
		p = a.left
	}

	if p == nil || !p.ready || p.wv == nil {
		return
	}

	env := a.env

	opt := a.criarFindOptionsEm(env)
	if opt == nil {
		return
	}

	ok := p.wv.StartFind(opt)
	opt.vtbl.Release.Call(uintptr(unsafe.Pointer(opt)))

	if ok {
		a.status = txtBuscaNativa
	} else {
		a.status = txtBuscaFalhou
	}

	a.invalidateBar()
}

func (a *app) abrirExclusaoChatGPT() {
	if a.right == nil || !a.right.ready || a.right.wv == nil {
		a.dizerf(txtChatGPTNaoPronto)

		return
	}

	if a.siteAtualIA().chave != chaveChatgpt {
		a.dizerf(txtNaoEChatGPT)

		return
	}

	a.retomarChatGPTFrio()
	a.right.wv.Navigate("https://chatgpt.com/#settings/DataControls")
	a.status = txtChatGPTDados
	a.invalidateBar()
}

func (a *app) reavaliarChatGPT() {
	if a.chatFrio && a.chatManual {
		return
	}

	if a.chatGerando || a.chatAtivo {
		a.chatFrioGen++
		a.chatFrioPendente = false
		a.retomarChatGPTFrio()

		return
	}

	a.agendarCongelamentoChatGPT()
}

func (a *app) pontoNoPainelChat(x, y int32) bool {
	if a.right == nil {
		return false
	}

	r := a.right.rect

	return r.Width() > 0 && r.Height() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) cursorNoPainelChat() bool {
	var ponto struct{ x, y int32 }
	if r, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&ponto))); r == 0 {
		return false
	}

	pScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	return a.pontoNoPainelChat(ponto.x, ponto.y)
}

func sitePorChave(chave string) siteIA {
	for _, s := range sitesIA {
		if s.chave == chave {
			return s
		}
	}

	for _, s := range sitesIA {
		if s.chave == chaveDeepseek {
			return s
		}
	}

	if len(sitesIA) > 0 {
		return sitesIA[0]
	}

	return siteIA{}
}

func (a *app) siteAtualIA() siteIA {
	return sitePorChave(a.st.RightSite)
}

func (a *app) nomeIAAtual() string {
	return a.siteAtualIA().nome
}

func (s siteIA) aceitaURL(u string) bool {
	if u == "" || strings.HasPrefix(u, "about:") || strings.HasPrefix(u, "data:") {
		return true
	}

	baixo := strings.ToLower(u)
	for _, h := range s.hosts {
		if strings.Contains(baixo, h) {
			return true
		}
	}

	return false
}

func (a *app) trocarSiteIA(chave string) {
	site := sitePorChave(chave)
	if site.chave == "" {
		return
	}

	if chave == a.st.RightSite {
		return
	}

	a.st.RightSite = site.chave
	a.st.RightURL = ""
	a.persist()

	if a.right == nil || !a.right.ready {
		a.st.RightVisible = true
		a.abrirPainelChat()
		a.atualizarMenuPainel()
		a.layout()

		return
	}

	if a.right != nil && a.right.ready && a.right.wv != nil {
		a.navegarPainelIA(a.right, site)
	}

	a.atualizarMenuPainel()
}

func (a *app) navegarPainelIA(p *pane, site siteIA) {
	if p == nil || !p.ready || p.wv == nil || site.chave == "" {
		return
	}

	if site.chave == chaveDeepseek && loginAutoLigado() {
		p.wv.AddInitScript(loginScript())
	}

	if p == a.right {
		a.chatTravando, a.chatVazioTentou = false, false
		a.chatFrioGen++
		a.chatFrioPendente = false
		a.chatUsado = false
		a.chatAtivo = false
		a.chatGerando = false
		a.chatManual = false
		a.right.frio = false

		a.right.frioManual = false
		if a.chatFrio {
			a.retomarChatGPTFrio()
		}
	}

	p.tentativas = 0
	p.queriaURL = site.url
	p.frio = false
	p.frioManual = false
	p.ociosoDesde = time.Time{}

	p.descarregado = false
	p.urlDescarga = ""
	p.scrollDescarga = 0

	p.scrollRestaurar = 0
	if p.wv != nil {
		p.wv.SetFrozen(false)
		adicionarInitScripts(p.wv, scriptsDaIA(site))
		p.wv.Navigate(site.url)
	}

	a.status = fmt.Sprintf(txtFmtPainelNome, site.nome)
	a.invalidateBar()
}

func (a *app) anexarImagensNoChat(itens []imagemAnexo) {
	if len(itens) == 0 {
		return
	}

	if a.right == nil || !a.right.ready {
		a.anexosPendentes1 = append([]imagemAnexo(nil), itens...)
		a.st.RightVisible = true
		a.abrirPainelChat()
		a.layout()
	}

	if a.right != nil && a.right.ready && a.right.wv != nil {
		a.anexarImagensEmPainel(a.right, itens)
		a.status = txtImgsAnexadas
	}

	a.invalidateBar()
}

func lerReadability() string {
	readabilityUma.Do(func() {
		var construtor strings.Builder

		if dados, err := os.ReadFile(filepath.Join(exeDir, "lib", "Readability.js")); err == nil {
			construtor.Write(dados)
		} else {
			construtor.WriteString(readabilityEmbutida)
		}

		if dados, err := os.ReadFile(filepath.Join(exeDir, "lib", "Readerable.js")); err == nil {
			construtor.WriteString("\n")
			construtor.Write(dados)
		} else {
			construtor.WriteString("\n")
			construtor.WriteString(readerableEmbutida)
		}

		readabilityFonte = construtor.String()
	})

	return readabilityFonte
}

func lerReaderable() string {
	readerableUma.Do(func() {
		if dados, err := os.ReadFile(filepath.Join(exeDir, "lib", "Readerable.js")); err == nil {
			readerableFonte = string(dados)
		} else {
			readerableFonte = readerableEmbutida
		}
	})

	return readerableFonte
}

func (a *app) lerPaginaParaChat(modo string) {
	a.lerPaginaParaChatCom(modo, "", false)
}

func (a *app) traduzirNoChat() {
	a.lerPaginaParaChatCom(modoTudo, instrucaoTraduzir, true)
}

// normalizarModoLeituraChat reduz o modo para tudo ou artigo.
func normalizarModoLeituraChat(modo string) string {
	if modo != modoTudo {
		return "artigo"
	}

	return modo
}

// bibliotecaLeituraChat carrega a Readability quando precisa do artigo.
func bibliotecaLeituraChat(modo string) (string, bool) {
	if modo == modoTudo {
		return "", true
	}

	lib := lerReadability()
	if lib == "" {
		return "", false
	}

	return lib, true
}

// travarLeituraChat impede duplo disparo em sequencia.
func (a *app) travarLeituraChat(modo string) (uint64, bool) {
	agora := time.Now()
	if modo == a.ultimaLeituraModo && !a.ultimaLeitura.IsZero() && agora.Sub(a.ultimaLeitura) < 2*time.Second {
		a.dizerf(txtJaLendo)

		return 0, false
	}

	a.ultimaLeitura, a.ultimaLeituraModo = agora, modo
	a.leituraGen++

	if a.leituraAtiva {
		a.dizerf(txtJaLendo)

		return 0, false
	}

	if a.envioAtivo {
		a.dizerf(txtJaEnviando)

		return 0, false
	}

	return a.leituraGen, true
}

// armarTravasLeituraChat agenda os avisos de demora e trava.
func (a *app) armarTravasLeituraChat(gen uint64) {
	time.AfterFunc(esperaLeituraChat, func() {
		a.depois(func() {
			if gen == a.leituraGen && a.leituraAtiva {
				a.leituraAtiva = false
				a.status = txtLeituraTravou
				a.invalidateBar()
			}
		})
	})

	time.AfterFunc(esperaAvisoPaginaGrande, func() {
		a.depois(func() {
			if gen == a.leituraGen && a.status == txtLendoPagina {
				a.status = txtPagGrande
				a.invalidateBar()
			}
		})
	})
}

type textoLidoChat struct {
	OK           bool   `json:"ok"`
	Erro         string `json:"err"`
	Titulo       string `json:"titulo"`
	Metodo       string `json:"metodo"`
	Chars        int    `json:"chars"`
	Truncado     bool   `json:"truncado"`
	CharsArtigo  int    `json:"charsArtigo"`
	CharsVisivel int    `json:"charsVisivel"`
	Texto        string `json:"texto"`
}

// falharLeituraChat encerra a leitura com erro de JS.
func (a *app) falharLeituraChat(gen uint64) {
	a.depois(func() {
		if gen != a.leituraGen {
			return
		}

		a.leituraAtiva = false
		a.status = txtNaoLiPagina
		a.invalidateBar()
	})
}

// falharTextoLidoChat encerra a leitura com erro do parse.
func (a *app) falharTextoLidoChat(gen uint64, motivo string) {
	a.depois(func() {
		if gen != a.leituraGen {
			return
		}

		a.leituraAtiva = false
		a.status = fmt.Sprintf(txtFmtSemTexto, motivo)
		a.invalidateBar()
	})
}

// entregarTextoLidoChat publica o texto lido no chat.
func (a *app) entregarTextoLidoChat(p *pane, gen uint64, modo, prefixo string, enviar bool, textoLido textoLidoChat) {
	a.depois(func() {
		if gen != a.leituraGen {
			return
		}

		a.leituraAtiva = false
		a.avisarNaPagina(p, textoLido.Chars, modo)
		a.status = fmt.Sprintf(txtFmtTextoExtraido, formataMilhar(textoLido.Chars))
		a.invalidateBar()

		if enviar {
			a.enviarPaginaNoChat(prefixo+textoLido.Texto, textoLido.Chars, textoLido.CharsVisivel, textoLido.Truncado)

			return
		}

		a.entregarNoChat(prefixo+textoLido.Texto, textoLido.Chars, textoLido.CharsVisivel, textoLido.Truncado)

		if modo == modoTudo {
			a.abrirSeletorImagens()
		}
	})
}

// tratarTextoLidoChat interpreta o retorno do JS de extracao.
func (a *app) tratarTextoLidoChat(p *pane, gen uint64, modo, prefixo string, enviar bool, err error, res string) {
	if err != nil {
		a.falharLeituraChat(gen)

		return
	}

	var textoLido textoLidoChat
	if e := json.Unmarshal([]byte(res), &textoLido); e != nil || !textoLido.OK {
		a.falharTextoLidoChat(gen, textoLido.Erro)

		return
	}

	a.entregarTextoLidoChat(p, gen, modo, prefixo, enviar, textoLido)
}

func (a *app) lerPaginaParaChatCom(modo, prefixo string, enviar bool) {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.status = txtPagIndisp
		a.invalidateBar()

		return
	}

	modo = normalizarModoLeituraChat(modo)

	lib, ok := bibliotecaLeituraChat(modo)
	if !ok {
		a.status = txtFaltaReadability
		a.invalidateBar()

		return
	}

	gen, travou := a.travarLeituraChat(modo)
	if !travou {
		return
	}

	a.leituraAtiva = true
	a.status = txtLendoPagina
	a.invalidateBar()
	a.armarTravasLeituraChat(gen)

	script := strings.ReplaceAll(extrairJS, "LIMITE_CHARS", strconv.Itoa(maxLeitura))
	script = strings.ReplaceAll(script, "MODO_AQUI", modo)

	carga := script
	if modo != modoTudo {
		carga = lib + "\n" + script
	}

	p.wv.EvalJS(carga, func(err error, res string) {
		a.tratarTextoLidoChat(p, gen, modo, prefixo, enviar, err, res)
	})
}

func (a *app) entregarNoChat(texto string, chars, charsPagina int, truncado bool) {
	if strings.TrimSpace(texto) == "" {
		a.dizerf(txtNadaMandar)
		a.invalidateBar()

		return
	}

	p := a.right
	if p == nil || !p.ready {
		a.chatPendente = func() { a.entregarNoChat(texto, chars, charsPagina, truncado) }
		a.st.RightVisible = true
		a.abrirPainelChat()
		a.layout()
		a.status = txtAbrindoChat
		a.invalidateBar()

		return
	}

	aviso := func(n int) string {
		if truncado {
			return fmt.Sprintf(txtFmtTextoLimite, n, charsPagina)
		}

		return fmt.Sprintf(txtFmtTextoEnter, n)
	}

	a.preencherNoChat(p, a.siteAtualIA(), texto, func(resultado envioPreparado) {
		if resultado.Ocupado {
			return
		}

		if resultado.Erro != "" || resultado.Chars == 0 {
			a.copiarComoPlanoB(texto, resultado.Erro)

			return
		}

		a.status = aviso(resultado.Chars)
		a.invalidateBar()
	})
}

// garantirChatParaEnvio abre o chat quando ainda nao esta pronto.
func (a *app) garantirChatParaEnvio(texto string, chars, charsPagina int, truncado bool) *pane {
	p := a.right
	if p != nil && p.ready {
		return p
	}

	a.chatPendente = func() { a.enviarPaginaNoChat(texto, chars, charsPagina, truncado) }
	a.st.RightVisible = true
	a.abrirPainelChat()
	a.layout()
	a.status = txtAbrindoChatEnvio
	a.invalidateBar()

	return nil
}

// extrairCliqueEnvio relocaliza o botao de envio antes do clique.
func extrairCliqueEnvio(err error, res string) (float64, float64, bool) {
	var prep struct {
		OK    bool    `json:"ok"`
		Botao bool    `json:"botao"`
		BX    float64 `json:"bx"`
		BY    float64 `json:"by"`
	}

	if err == nil {
		if desembrulharEvalJS(res, &prep) == nil && prep.OK && prep.Botao {
			return prep.BX, prep.BY, true
		}
	}

	return 0, 0, false
}

// clicarEnviarNoChat clica no botao ou tecla Enter como reserva.
func (a *app) clicarEnviarNoChat(p *pane, site siteIA, texto string) {
	final := func() {
		a.dizerOkf(txtTextoEnviado)
		a.invalidateBar()
		a.conferirEnvio(p, site, len(texto), 0)
	}

	p.wv.EvalJS(prepararEnvioJS(site), func(err error, res string) {
		posX, posY, achouBotao := extrairCliqueEnvio(err, res)

		a.depois(func() {
			if p.wv == nil || !p.ready {
				return
			}

			if achouBotao {
				a.clicarCDP(p, posX, posY)
			} else {
				a.teclarEnterCDP(p)
			}

			final()
		})
	})
}

// dispararEnvioNoChat envia com Enter ou clique apos preencher.
func (a *app) dispararEnvioNoChat(p *pane, site siteIA, texto string, resultado envioPreparado) {
	if p.wv == nil || !p.ready {
		return
	}

	if resultado.Erro != "" || resultado.Chars == 0 {
		a.copiarComoPlanoB(texto, resultado.Erro)

		return
	}

	if !resultado.Botao {
		a.teclarEnterCDP(p)
		a.dizerOkf(txtTextoEnviado)
		a.invalidateBar()
		a.conferirEnvio(p, site, len(texto), 0)

		return
	}

	a.clicarEnviarNoChat(p, site, texto)
}

func (a *app) enviarPaginaNoChat(texto string, chars, charsPagina int, truncado bool) {
	if strings.TrimSpace(texto) == "" {
		a.dizerf(txtNadaMandar)
		a.invalidateBar()

		return
	}

	p := a.garantirChatParaEnvio(texto, chars, charsPagina, truncado)
	if p == nil {
		return
	}

	site := a.siteAtualIA()
	a.preencherNoChat(p, site, texto, func(r envioPreparado) {
		if r.Ocupado {
			return
		}

		a.dispararEnvioNoChat(p, site, texto, r)
	})
}

func (a *app) copiarComoPlanoB(texto, motivo string) {
	switch {
	case copiarTexto(texto):
		a.dizerOkf(txtTextoColado)
	case motivo != "":
		a.dizerErrof(txtFmtNaoEntregue, motivo)
	default:
		a.dizerErrof(txtNaoEntregue)
	}

	a.invalidateBar()
}

func (a *app) rectDoChat() Rect {
	if a.right == nil {
		return Rect{}
	}

	return a.right.rect
}

func (a *app) toggleChat() {
	defer a.limpezaAposAcao("alternar-chat")

	a.st.RightVisible = !a.st.RightVisible
	if a.st.RightVisible {
		a.abrirPainelChat()
	} else {
		a.fecharPainelChat()
		a.status = txtIAFechado
		a.invalidateBar()
	}

	a.layout()
	a.persist()

	if a.st.RightVisible && (a.right == nil || !a.right.ready) {
		a.status = fmt.Sprintf(txtFmtAbrindoIA, a.nomeIAAtual())
		a.invalidateBar()
	}
}

// suspenderTudoParaSoIA esconde abas e imagens no modo so IA.
func (a *app) suspenderTudoParaSoIA() {
	a.painelCheio = false
	a.fecharPopup()
	a.hideTip()

	if a.urlEdit != 0 {
		pShowWindow.Call(a.urlEdit, swHide)
		a.urlEditing = false
	}

	for _, t := range a.tabs {
		suspenderPane(t.pane)
	}

	for _, p := range []*pane{a.imgPanel} {
		suspenderPane(p)
	}

	a.status = txtPainelFull
	a.agendarLimpezaMemoria(true, esperaLimpezaSoIA)
}

// retomarPainelPrincipalSoIA reexibe a aba principal ao sair do so IA.
func (a *app) retomarPainelPrincipalSoIA() {
	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.resumirPane(a.left)
	} else if a.ativa >= 0 && a.ativa < len(a.tabs) {
		a.ativarAba(a.ativa)
	}
}

// retomarPaineisSoIA reabre chat e imagens ao sair do so IA.
func (a *app) retomarPaineisSoIA() {
	if a.st.RightVisible && (a.right == nil || !a.right.ready) {
		a.abrirPainelChat()
	}

	if a.st.RightVisible && a.st.ImgVisible && a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil {
		a.resumirPane(a.imgPanel)
	}
}

func (a *app) toggleSoIA() {
	if !a.st.RightVisible || a.right == nil {
		return
	}
	defer a.limpezaAposAcao("alternar-soia")

	a.soIA = !a.soIA
	if a.soIA {
		a.suspenderTudoParaSoIA()
	} else {
		a.retomarPainelPrincipalSoIA()
		a.retomarPaineisSoIA()

		a.retomarTodosFrios()
		a.status = txtPainelNormal
	}

	a.layout()
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) voltarPainelIAInicial() {
	if !a.st.RightVisible || a.right == nil || !a.right.ready || a.right.wv == nil {
		a.dizerf(txtIAAbrindo)

		return
	}

	site := a.siteAtualIA()
	if site.url == "" {
		return
	}

	a.chatTravando = false
	a.chatVazioTentou = false
	a.right.tentativas = 0
	a.right.queriaURL = site.url
	a.right.wv.Navigate(site.url)
	a.navegarImagens(imagensHomeURL, true)
	a.status = fmt.Sprintf(txtFmtPainelReinic, site.nome)
	a.invalidateBar()
}

func (a *app) setZoom(fator float64) {
	if fator < escalaMinima {
		fator = escalaMinima
	}

	if fator > escalaMaxima {
		fator = escalaMaxima
	}

	a.st.Zoom = float64(int(fator*fatorPorcentagemZoom+ajusteArredondaZoom)) / fatorPorcentagemZoom
	if a.left != nil {
		a.left.zoom = a.st.Zoom
	}

	a.reaplicarZoomNativo()
	a.status = fmt.Sprintf(txtFmtZoom, a.st.Zoom*fatorPorcentagemZoom)
	a.persist()
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

// aplicarZoomNativo aplica o zoom do painel no controlador: o Chromium
// mapeia a entrada sozinho, sem o deslocamento do zoom via CSS no root.
func (a *app) aplicarZoomNativo(p *pane, fator float64) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if fator < escalaMinima || fator > escalaMaxima {
		fator = 1
	}

	p.wv.SetZoomFactor(fator)
}

func (a *app) reaplicarZoomNativo() {
	for _, p := range []*pane{a.left, a.right, a.popup} {
		if p == nil || !p.ready {
			continue
		}

		a.aplicarZoomNativo(p, a.paneZoom(p))
	}
}

func (a *app) escalaRapidaChat() {
	a.escalaRapidaChatPane(a.right)
}

func (a *app) escalaRapidaChatPane(p *pane) {
	if p == nil || !p.ready || p.cssW <= 0 {
		return
	}

	w := p.rect.Width()
	if w <= 0 {
		return
	}

	z := float64(w) / float64(p.cssW)
	p.wv.SetZoomFactor(z)
}

func (a *app) aplicarTamanhoChat(p *pane) {
	if p == nil || !p.ready {
		return
	}

	largura := p.rect.Width()

	h := p.rect.Height()
	if largura <= 0 || h <= 0 {
		return
	}

	mudou := aplicarViewport(p, largura, h, 1, true)
	if !mudou {
		return
	}

	if d := 1.0; d != p.zoomLogged {
		p.zoomLogged = d
	}

	a.aplicarZoomNativo(p, a.paneZoom(p))
}

func (a *app) paneUsaDeepSeek(p *pane) bool {
	if p == nil {
		return false
	}

	if p == a.right {
		return a.siteAtualIA().chave == chaveDeepseek
	}

	return false
}

func (a *app) paneUsaChatGPT(p *pane) bool {
	if p == nil {
		return false
	}

	if p == a.right {
		return a.siteAtualIA().chave == chaveChatgpt
	}

	return false
}

func (a *app) paneUsaIA(p *pane) bool {
	return p != nil && p == a.right
}

// paginaBrancaVazia diz se o painel de IA caiu numa pagina em branco.
func paginaBrancaVazia(p *pane) bool {
	u := p.wv.Source()

	return strings.Contains(u, "deepseek.com/a/chat/")
}

// contarNosPaginaBranca interpreta a contagem de nos do JS.
func contarNosPaginaBranca(err error, res string) (int, bool) {
	if err != nil {
		return 0, false
	}

	n, e := strconv.Atoi(strings.Trim(strings.TrimSpace(res), `"`))
	if e != nil || n <= 0 || n >= 40 {
		return 0, false
	}

	return n, true
}

// recarregarChatVazio volta o chat para a URL inicial.
func (a *app) recarregarChatVazio(p *pane) {
	if a.chatVazioTentou || !p.ready || p.wv == nil {
		return
	}

	a.chatVazioTentou = true
	a.st.RightURL = ""
	p.queriaURL = urlChat
	p.wv.Navigate(urlChat)

	a.status = txtChatVazioDeep
	a.invalidateBar()
}

func (a *app) chatVazioCuidaDaPaginaEmBranco(p *pane, ok bool) {
	if !ok || p == nil || !p.ready || p != a.right {
		return
	}

	if a.chatVazioTentou || !paginaBrancaVazia(p) {
		return
	}

	p.wv.EvalJS(`(function(){ try { return document.querySelectorAll('*').length; } catch (e) { return -1; } })()`,
		func(err error, res string) {
			if _, vazio := contarNosPaginaBranca(err, res); !vazio {
				return
			}

			a.depois(func() { a.recarregarChatVazio(p) })
		})
}

func pedacos(texto string, tamanho int) []string {
	if texto == "" {
		return nil
	}

	if tamanho < 1 {
		tamanho = 1
	}

	runas := []rune(texto)
	if len(runas) <= tamanho {
		return []string{texto}
	}

	saida := make([]string, 0, len(runas)/tamanho+1)
	for i := 0; i < len(runas); i += tamanho {
		fim := min(i+tamanho, len(runas))

		saida = append(saida, string(runas[i:fim]))
	}

	return saida
}
