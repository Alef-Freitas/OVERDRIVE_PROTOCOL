package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const maxLeitor = 900_000

const (
	esperaSondagemLeitura  = 3000 * time.Millisecond
	esperaWatchdogLeitura  = 30 * time.Second
	esperaConfirmarLeitura = 3 * time.Second
	limiteTextoLeitura     = 30000
	esperaTravaLeitura     = 30 * time.Second
	cpmPadraoLeitura       = 987
	variacaoPadraoLeitura  = 118
	tamanhoMaxMetaLeitura  = 32
)

// leitorGen invalida o watchdog do modo leitura: cada tentativa consome uma
// geracao e qualquer resposta (ou nova tentativa) invalida a anterior.
//
//nolint:gochecknoglobals // Geracao do watchdog do modo leitura: invalida tentativas antigas.
var leitorGen uint64

const leituraJS = `(function(){
  try {
    if (typeof Readability !== 'function') { return {ok:false, err:'Readability nao carregou'}; }
    // Como o Firefox (Readerable): checagem barata antes do parse pesado.
    // Sem o gate, o botão fica sempre visível e só falha no fim.
    if (typeof isProbablyReaderable === 'function') {
      var legivel = false;
      try { legivel = isProbablyReaderable(document); } catch (e) { legivel = true; }
      if (!legivel) { return {ok:false, err:'pagina sem artigo legivel'}; }
    }
    var art = null;
    try {
      art = new Readability(document.cloneNode(true), {charThreshold: 150}).parse();
    } catch (e) {
      return {ok:false, err:String(e && e.message ? e.message : e)};
    }
    if (!art || !art.content) { return {ok:false, err:'nao achei o artigo'}; }

    var div = document.createElement('div');
    div.innerHTML = art.content;

    function urlDeSrcset(ss) {
      // "foto-p.jpg 300w, foto-g.jpg 1200w" → URL do maior descritor.
      var melhor = '', melhorV = -1;
      var partes = String(ss || '').split(',');
      for (var s = 0; s < partes.length; s++) {
        var t = String(partes[s] || '').trim().split(/\s+/);
        if (!t[0]) { continue; }
        var v = -1;
        for (var d = 1; d < t.length; d++) {
          var m = /^([0-9]+(?:\.[0-9]+)?)(w|x)$/.exec(t[d]);
          if (m) { v = parseFloat(m[1]) * (m[2] === 'x' ? 100000 : 1); break; }
        }
        if (v < 0) { v = s; }
        if (v >= melhorV) { melhorV = v; melhor = t[0]; }
      }
      return melhor;
    }

    // O Readability derruba o <img> sem src dentro de <picture>: reconstrói
    // a partir do <source> antes do laço principal.
    var fotos = div.querySelectorAll('picture');
    for (var pi = 0; pi < fotos.length; pi++) {
      var pic = fotos[pi];
      var temImg = pic.querySelector('img');
      var srcImg = temImg ? String(temImg.getAttribute('src') || temImg.getAttribute('data-src') || '') : '';
      if (temImg && /^https?:/i.test(srcImg)) { continue; }
      var ssPic = '';
      var fontes = pic.querySelectorAll('source[srcset]');
      for (var f = 0; f < fontes.length; f++) {
        var s2 = String(fontes[f].getAttribute('srcset') || '');
        if (s2) { ssPic = s2; }
      }
      var uPic = urlDeSrcset(ssPic);
      if (!uPic) { continue; }
      if (!temImg) {
        temImg = document.createElement('img');
        pic.appendChild(temImg);
      }
      temImg.setAttribute('src', uPic);
    }

    var imgs = div.querySelectorAll('img');
    var totalImgs = 0;
    for (var im = 0; im < imgs.length; im++) {
      var velha = imgs[im];
      var src = String(velha.currentSrc || velha.src || velha.getAttribute('src') ||
        velha.getAttribute('data-od-src') || velha.getAttribute('data-src') ||
        velha.getAttribute('data-lazy-src') || velha.getAttribute('data-original') || '');
      if (/^data:/i.test(src)) {
        src = String(velha.getAttribute('data-od-src') || velha.getAttribute('data-src') ||
          velha.getAttribute('data-lazy-src') || velha.getAttribute('data-original') || '');
      }
      if (!/^https?:/i.test(src)) {
        // <img> só com srcset: colhe a maior URL.
        var ss = String(velha.getAttribute('srcset') || velha.getAttribute('data-srcset') ||
          velha.getAttribute('data-lazy-srcset') || velha.getAttribute('data-original-srcset') || '');
        var u2 = urlDeSrcset(ss);
        if (u2) { src = u2; }
      }
      try { src = new URL(src, location.href).href; } catch (e) { src = ''; }
      var w = parseInt(velha.getAttribute('width') || '0', 10) || velha.naturalWidth || 0;
      var h = parseInt(velha.getAttribute('height') || '0', 10) || velha.naturalHeight || 0;
      if (!/^https?:/i.test(src) || totalImgs >= 50 || (w > 0 && w < 100) || (h > 0 && h < 80)) {
        if (velha.parentNode) { velha.parentNode.removeChild(velha); }
        continue;
      }
      var nova = document.createElement('img');
      nova.setAttribute('src', src);
      nova.setAttribute('loading', 'lazy');
      nova.setAttribute('decoding', 'async');
      var alt = String(velha.getAttribute('alt') || '').trim();
      if (alt) { nova.setAttribute('alt', alt.slice(0, 200)); }
      if (w > 0) { nova.setAttribute('width', String(w)); }
      if (h > 0) { nova.setAttribute('height', String(h)); }
      if (velha.parentNode) { velha.parentNode.replaceChild(nova, velha); totalImgs++; }
    }

    var lixo = div.querySelectorAll('script,style,noscript,iframe,object,embed,form,input,button,' +
      'textarea,select,video,audio,canvas,svg,source,track,link,meta,base');
    for (var i = 0; i < lixo.length; i++) {
      if (lixo[i].parentNode) { lixo[i].parentNode.removeChild(lixo[i]); }
    }

    var todos = div.querySelectorAll('*');
    for (var j = 0; j < todos.length; j++) {
      var el = todos[j];
      var attrs = el.attributes;
      for (var k = attrs.length - 1; k >= 0; k--) {
        var nome = String(attrs[k].name || '').toLowerCase();
        var manter = false;
        if ((el.tagName === 'TD' || el.tagName === 'TH') &&
            (nome === 'colspan' || nome === 'rowspan' || nome === 'scope')) {
          manter = true;
        }
        if (el.tagName === 'IMG' &&
            (nome === 'src' || nome === 'alt' || nome === 'loading' || nome === 'decoding' ||
             nome === 'width' || nome === 'height')) {
          manter = true;
        }
        if (!manter) { el.removeAttribute(attrs[k].name); }
      }
    }

    var vazios = div.querySelectorAll('figure,picture,div,section');
    for (var v = vazios.length - 1; v >= 0; v--) {
      var q = vazios[v];
      if (!q.textContent.trim() && !q.querySelector('img,hr,table,ul,ol,blockquote,pre')) {
        if (q.parentNode) { q.parentNode.removeChild(q); }
      }
    }

    var html = div.innerHTML.trim();
    if (!html) { return {ok:false, err:'artigo vazio'}; }
    if (html.length > LIMITE_CHARS) { return {ok:false, err:'artigo grande demais para o modo leitura'}; }
    // O Firefox só usa o publishedTime do parse (meta/JSON-LD), que falta
    // em muitos portais; a página em si costuma ter <time datetime>.
    var publicado = String(art.publishedTime || '');
    if (!publicado) {
      try {
        var t = document.querySelector('time[datetime]');
        if (t) {
          var dt = String(t.getAttribute('datetime') || '').trim();
          if (/^\d{4}-\d{2}-\d{2}/.test(dt)) { publicado = dt; }
        }
      } catch (e) {}
    }
    return {
      ok:true,
      titulo:String(art.title || document.title || ''),
      site:String(art.siteName || location.hostname || ''),
      byline:String(art.byline || ''),
      url:String(location.href || ''),
      html:html,
      chars:(art.textContent || '').length,
      resumo:String(art.excerpt || '').slice(0, 2000),
      idioma:String(art.lang || ''),
      direcao:(art.dir === 'rtl' ? 'rtl' : ''),
      publicadoEm:publicado
    };
  } catch (e) {
    return {ok:false, err:String(e && e.message ? e.message : e)};
  }
})()`

type leituraResultado struct {
	OK          bool   `json:"ok"`
	Erro        string `json:"err"`
	Titulo      string `json:"titulo"`
	Site        string `json:"site"`
	Byline      string `json:"byline"`
	URL         string `json:"url"`
	HTML        string `json:"html"`
	Chars       int    `json:"chars"`
	Resumo      string `json:"resumo"`
	Idioma      string `json:"idioma"`
	Direcao     string `json:"direcao"`
	PublicadoEm string `json:"publicadoEm"`
}

func limparLeitor(p *pane) {
	if p == nil {
		return
	}

	p.readerAtivo = false
	p.readerCarregando = false
	p.readerPronto = false
	p.readerURL = ""
}

// esquemaLeituraValido diz se a URL pode entrar no modo leitura.
func esquemaLeituraValido(uri string) (*url.URL, bool) {
	enderecoParse, err := url.Parse(uri)
	if err != nil || enderecoParse.Scheme != esquemaHTTP && enderecoParse.Scheme != esquemaHTTPS {
		return nil, false
	}

	return enderecoParse, true
}

// reagendarSondagemLeitura re-checa apos renderizar, com trava pela URL.
func (a *app) reagendarSondagemLeitura(p *pane, uri string) {
	time.AfterFunc(esperaSondagemLeitura, func() {
		a.depois(func() {
			if a.quitting || p != a.left || !p.ready || p.wv == nil || p.readerAtivo {
				return
			}

			atual := strings.TrimSpace(p.ultimaURL)
			if atual == "" {
				atual = strings.TrimSpace(p.wv.Source())
			}

			if atual != uri {
				return
			}

			a.sondarLeitura(p, uri)
		})
	})
}

// avaliarLeituraUsavel decide se o livro aparece na barra de endereço. Como
// o Firefox, mostra só quando a página parece artigo (gate barato + 1
// re-checagem após renderizar, com trava pela URL atual).
func (a *app) avaliarLeituraUsavel(p *pane, uri string) {
	if a == nil || p == nil || p != a.left {
		return
	}

	uri = strings.TrimSpace(uri)
	if p.readerAtivo {
		a.marcarLeituraUsavel(uri, false)

		return
	}

	if _, ok := esquemaLeituraValido(uri); !ok {
		a.marcarLeituraUsavel(uri, false)

		return
	}

	a.sondarLeitura(p, uri)
	a.reagendarSondagemLeitura(p, uri)
}

// interpretarSondaLeitura extrai o booleano do JS de legibilidade.
func interpretarSondaLeitura(err error, res string) bool {
	if err != nil {
		return false
	}

	var b bool
	if desembrulharEvalJS(res, &b) != nil {
		return false
	}

	return b
}

// urlAtualPane devolve a URL atual com queda para o WebView.
func urlAtualPane(p *pane) string {
	atual := strings.TrimSpace(p.ultimaURL)
	if p.wv != nil && atual == "" {
		atual = strings.TrimSpace(p.wv.Source())
	}

	return atual
}

// aplicarSondaLeitura marca o resultado com trava pela URL sondada.
func (a *app) aplicarSondaLeitura(p *pane, uri string, legivel bool) {
	if a.quitting || p != a.left || p.readerAtivo {
		return
	}

	if urlAtualPane(p) != uri {
		return
	}

	a.marcarLeituraUsavel(uri, legivel)
}

func (a *app) sondarLeitura(p *pane, uri string) {
	if p == nil || p.wv == nil {
		return
	}

	lib := lerReaderable()
	if lib == "" {
		return
	}

	p.wv.EvalJS(lib+"\n(typeof isProbablyReaderable==='function'&&isProbablyReaderable(document))",
		func(err error, res string) {
			ok := interpretarSondaLeitura(err, res)

			a.depois(func() { a.aplicarSondaLeitura(p, uri, ok) })
		})
}

func (a *app) marcarLeituraUsavel(uri string, usavel bool) {
	a.readerCheckURL = uri
	if a.readerUsavel == usavel {
		return
	}

	a.readerUsavel = usavel
	a.layout()
	a.invalidateBar()
}

// painelLeituraDisponivel valida painel, tube e estado do leitor.
func (a *app) painelLeituraDisponivel(p *pane) bool {
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtLeituraSemPag)

		return false
	}

	if a.ytAberto && !a.ytOculto {
		a.dizerf(txtLeituraFecheTube)

		return false
	}

	if p.readerAtivo {
		a.dizerf(txtLeituraAtivo)

		return false
	}

	return true
}

// urlLeituraAtual valida a URL atual para o modo leitura.
func (a *app) urlLeituraAtual(p *pane) (string, *url.URL, bool) {
	atual := strings.TrimSpace(p.wv.Source())
	enderecoParse, err := url.Parse(atual)

	if err != nil || enderecoParse.Host == "" ||
		(enderecoParse.Scheme != esquemaHTTP && enderecoParse.Scheme != esquemaHTTPS) {
		a.dizerf(txtLeituraHTTP)

		return "", nil, false
	}

	if p.readerCarregando {
		a.dizerf(txtLeituraPrep)

		return "", nil, false
	}

	return atual, enderecoParse, true
}

// paneLeituraPronto valida o painel e devolve a URL atual.
func (a *app) paneLeituraPronto() (*pane, string, *url.URL, bool) {
	p := a.left
	if !a.painelLeituraDisponivel(p) {
		return nil, "", nil, false
	}

	atual, enderecoParse, valido := a.urlLeituraAtual(p)
	if !valido {
		return nil, "", nil, false
	}

	return p, atual, enderecoParse, true
}

// armarWatchdogLeitura invalida a tentativa se o renderer nao responder.
func (a *app) armarWatchdogLeitura(p *pane, gen uint64) {
	// Watchdog: se o renderer morrer ou a pagina navegar no meio do EvalJS,
	// nenhuma resposta chega e a flag ficava presa para sempre.
	time.AfterFunc(esperaWatchdogLeitura, func() {
		a.depois(func() {
			if atomic.LoadUint64(&leitorGen) != gen {
				return
			}

			if p != a.left || !p.readerCarregando || p.readerPronto || p.readerAtivo {
				return
			}

			p.readerCarregando = false

			a.dizerErrof("%s", txtLeituraTempo)
			a.invalidateBar()
		})
	})
}

// motivoLeituraExtraido interpreta o retorno do JS de leitura.
func motivoLeituraExtraido(res string) (leituraResultado, string, bool) {
	var resultado leituraResultado
	if e := desembrulharEvalJS(res, &resultado); e != nil || !resultado.OK {
		motivo := resultado.Erro
		if e != nil {
			motivo = e.Error()
		}

		if motivo == "" {
			motivo = txtLeituraSemArtigo
		}

		return resultado, motivo, false
	}

	return resultado, "", true
}

// alvoLeituraValido resolve a URL final do artigo.
func alvoLeituraValido(resultado leituraResultado, atual string, enderecoParse *url.URL) (string, *url.URL) {
	alvo := strings.TrimSpace(resultado.URL)

	puAlvo, errAlvo := url.Parse(alvo)
	if errAlvo != nil || puAlvo.Host == "" || (puAlvo.Scheme != esquemaHTTP && puAlvo.Scheme != esquemaHTTPS) {
		return atual, enderecoParse
	}

	return alvo, puAlvo
}

// publicarPaginaLeitura troca o painel pelo artigo extraido.
func (a *app) publicarPaginaLeitura(
	p *pane, atual string, _ *url.URL, resultado leituraResultado, alvo string, puAlvo *url.URL,
) {
	pagina := montarPaginaLeitura(resultado.Titulo, resultado.Site, resultado.Byline, alvo, resultado.HTML, leituraExtra{
		Resumo: resultado.Resumo, Idioma: resultado.Idioma, Direcao: resultado.Direcao,
		PublicadoEm: resultado.PublicadoEm, Chars: resultado.Chars,
	})

	a.depois(func() {
		if p != a.left || !p.ready || p.wv == nil {
			p.readerCarregando = false

			return
		}

		if strings.TrimSpace(p.wv.Source()) != atual {
			p.readerCarregando = false

			return
		}

		p.readerAtivo = true
		p.readerCarregando = true
		p.readerPronto = false
		p.readerURL = alvo
		p.ultimaURL = alvo
		a.setAddress(alvo)
		a.readerUsavel = false

		p.wv.NavigateToString(pagina)

		a.status = fmt.Sprintf(txtFmtLeituraMotivo, primeiroNaoVazio(resultado.Titulo, puAlvo.Hostname()))
		a.loading = false
		a.invalidateBar()
		a.layout()
		time.AfterFunc(esperaConfirmarLeitura, func() {
			a.depois(func() {
				if p == a.left && p.readerAtivo {
					p.readerCarregando = false
					p.readerPronto = true
				}
			})
		})
	})
}

// tratarRetornoLeitura interpreta o EvalJS do modo leitura.
func (a *app) tratarRetornoLeitura(p *pane, atual string, enderecoParse *url.URL, err error, res string) {
	atomic.AddUint64(&leitorGen, 1)

	if err != nil {
		a.depois(func() {
			p.readerCarregando = false

			a.dizerErrof("%s", txtLeituraFalhou)
			a.invalidateBar()
		})

		return
	}

	resultado, motivo, valido := motivoLeituraExtraido(res)
	if !valido {
		a.depois(func() {
			p.readerCarregando = false

			a.dizerErrof(txtFmtLeituraMotivo, motivo)
			a.invalidateBar()
		})

		return
	}

	alvo, puAlvo := alvoLeituraValido(resultado, atual, enderecoParse)

	if len(resultado.HTML) == 0 || utf8.RuneCountInString(resultado.HTML) > maxLeitor {
		a.depois(func() {
			p.readerCarregando = false

			a.dizerf(txtLeituraGrande)
			a.invalidateBar()
		})

		return
	}

	a.publicarPaginaLeitura(p, atual, enderecoParse, resultado, alvo, puAlvo)
}

func (a *app) abrirModoLeitura() {
	p, atual, enderecoParse, valido := a.paneLeituraPronto()
	if !valido {
		return
	}

	lib := lerReadability()
	if lib == "" {
		a.dizerf(txtLeituraSemLib)

		return
	}

	p.readerCarregando = true
	p.readerPronto = false

	a.dizerf(txtLeituraGerando)
	a.invalidateBar()

	gen := atomic.AddUint64(&leitorGen, 1)
	a.armarWatchdogLeitura(p, gen)

	script := strings.ReplaceAll(leituraJS, "LIMITE_CHARS", strconv.Itoa(maxLeitor))
	p.wv.EvalJS(lib+"\n"+script, func(err error, res string) {
		a.tratarRetornoLeitura(p, atual, enderecoParse, err, res)
	})
}

func (a *app) sairDoLeitor(p *pane) {
	if p == nil || !p.readerAtivo {
		return
	}

	endereco := p.readerURL
	limparLeitor(p)
	a.invalidateBar()
	a.layout()

	if endereco == "" || p.wv == nil || !p.ready {
		return
	}

	a.status = txtLeituraRecarga
	a.loading = true
	a.invalidateBar()
	p.wv.Navigate(endereco)
}

func (a *app) tratarMsgLeitor(p *pane, msg string) bool {
	if p == nil || !p.readerAtivo || !strings.Contains(msg, `"t":"reader"`) {
		return false
	}

	var mensagem struct {
		Acao   string `json:"acao"`
		Titulo string `json:"titulo"`
		URL    string `json:"url"`
		Texto  string `json:"texto"`
	}
	if err := json.Unmarshal([]byte(msg), &mensagem); err != nil {
		return true
	}

	switch mensagem.Acao {
	case idSair:
		a.sairDoLeitor(p)
	case "resumir":
		a.resumirLeituraNoChat(p, mensagem.Titulo, mensagem.URL, mensagem.Texto)
	}

	return true
}

// textoLeituraParaIA compõe o mesmo conteúdo que vai para a IA:
// pedido + título + URL + texto (teto de 30 mil caracteres).
func (a *app) textoLeituraParaIA(p *pane, titulo, url, texto string) string {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return ""
	}

	if utf8.RuneCountInString(texto) > limiteTextoLeitura {
		texto = string([]rune(texto)[:limiteTextoLeitura])
	}

	titulo = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(titulo), txtLeituraSufTitulo))

	var pagina strings.Builder
	pagina.WriteString(txtLeituraPecaResumo + ":\n")

	if titulo != "" {
		pagina.WriteString("TÍTULO: " + titulo + "\n")
	}

	if strings.TrimSpace(url) == "" && p != nil {
		url = strings.TrimSpace(p.readerURL)
	}

	if strings.TrimSpace(url) != "" {
		pagina.WriteString("URL: " + strings.TrimSpace(url) + "\n")
	}

	pagina.WriteString("\n" + texto)

	return pagina.String()
}

// resumirLeituraNoChat manda o texto do modo leitura para a IA da direita
// (a que estiver ativa), pedindo resumo. Com o painel fechado, enfileira e
// abre, como o resto do chat.
func (a *app) resumirLeituraNoChat(p *pane, titulo, url, texto string) {
	corpo := a.textoLeituraParaIA(p, titulo, url, texto)
	if corpo == "" {
		a.dizerf(txtNadaMandar)
		a.invalidateBar()

		return
	}

	n := utf8.RuneCountInString(corpo)
	a.enviarPaginaNoChat(corpo, n, n, false)
}

// copiarLeitura joga na área de transferência, em UMA entrada só, o mesmo
// conteúdo que iria para a IA. De lá, o botão de enviar do histórico manda
// para a IA quando o usuário quiser.
func (a *app) copiarLeitura(p *pane, titulo, url, texto string) {
	corpo := a.textoLeituraParaIA(p, titulo, url, texto)
	if corpo == "" {
		a.dizerf(txtNadaMandar)
		a.invalidateBar()

		return
	}

	if err := clipboardGravarTexto(corpo); err != nil {
		a.dizerf("%s", txtClipPrefix+err.Error())
		a.invalidateBar()

		return
	}

	clipAdicionar(clipItem{tipo: clipTexto, texto: corpo})
	a.dizerOkf(txtLeituraCopiada)
	a.invalidateBar()
}

// copiarArtigoJS extrai o mesmo texto que iria para a IA (artigo via
// Readability, com queda para o texto visível), sem entrar no modo leitura.
const copiarArtigoJS = `(function(){
  try {
    var titulo = String(document.title || '');
    var href = String(location.href || '');
    var corpo = '';
    if (typeof Readability === 'function') {
      try {
        var art = new Readability(document.cloneNode(true), {charThreshold: 150}).parse();
        if (art) {
          if (art.title) { titulo = String(art.title); }
          corpo = String(art.textContent || '');
        }
      } catch (e) {}
    }
    if (!corpo || !corpo.replace(/\s+/g, '')) {
      try { corpo = String(document.body ? document.body.innerText : ''); } catch (e) { corpo = ''; }
    }
    corpo = String(corpo || '').replace(/[ \t]+/g, ' ').replace(/\n{3,}/g, '\n\n').trim();
    if (!corpo.replace(/\s+/g, '')) { return {ok:false, err:'sem texto na pagina'}; }
    if (corpo.length > 30000) { corpo = corpo.slice(0, 30000); }
    return {ok:true, titulo:titulo, url:href, texto:corpo};
  } catch (e) { return {ok:false, err:String(e && e.message || e)}; }
})()`

// copiarArtigoDaBarra é o botão ao lado do livro na barra de URL: extrai o
// artigo da página atual e grava UMA entrada no histórico, sem abrir o leitor.
func (a *app) copiarArtigoDaBarra() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtLeituraSemPag)

		return
	}

	if a.ytAberto && !a.ytOculto {
		a.dizerf(txtLeituraFecheTube)

		return
	}

	if a.leituraAtiva {
		a.dizerf(txtJaLendo)

		return
	}

	a.leituraAtiva = true
	gen := a.leituraGen + 1
	a.leituraGen = gen
	a.dizerf(txtLendoPagina)
	a.invalidateBar()
	a.agendarTravaLeitura(gen)

	carga := copiarArtigoJS

	if !p.readerAtivo {
		if lib := lerReadability(); lib != "" {
			carga = lib + "\n" + copiarArtigoJS
		}
	}

	p.wv.EvalJS(carga, func(err error, res string) {
		a.tratarArtigoCopiado(p, gen, err, res)
	})
}

// agendarTravaLeitura desiste da leitura se o JS nao responder em 30s.
func (a *app) agendarTravaLeitura(gen uint64) {
	time.AfterFunc(esperaTravaLeitura, func() {
		a.depois(func() {
			if gen == a.leituraGen && a.leituraAtiva {
				a.leituraAtiva = false
				a.dizerf(txtLeituraTravou)
				a.invalidateBar()
			}
		})
	})
}

// tratarArtigoCopiado interpreta o retorno do JS e copia ou explica a falha.
func (a *app) tratarArtigoCopiado(p *pane, gen uint64, err error, res string) {
	if err != nil {
		a.depois(func() {
			a.falharLeitura(gen, txtLeituraFalhou)
		})

		return
	}

	var resposta struct {
		OK     bool   `json:"ok"`
		Erro   string `json:"err"`
		Titulo string `json:"titulo"`
		URL    string `json:"url"`
		Texto  string `json:"texto"`
	}
	if e := desembrulharEvalJS(res, &resposta); e != nil || !resposta.OK {
		motivo := resposta.Erro
		if e != nil {
			motivo = e.Error()
		}

		if motivo == "" {
			motivo = txtLeituraSemArtigo
		}

		a.depois(func() {
			if gen != a.leituraGen {
				return
			}

			a.leituraAtiva = false
			a.dizerErrof(txtFmtLeituraMotivo, motivo)
			a.invalidateBar()
		})

		return
	}

	titulo, paginaURL, texto := resposta.Titulo, resposta.URL, resposta.Texto

	a.depois(func() {
		if gen != a.leituraGen {
			return
		}

		a.leituraAtiva = false
		a.copiarLeitura(p, titulo, paginaURL, texto)
	})
}

// falharLeitura encerra a leitura da geracao se ainda for a atual.
func (a *app) falharLeitura(gen uint64, texto string) {
	if gen != a.leituraGen {
		return
	}

	a.leituraAtiva = false
	a.dizerErrof("%s", texto)
	a.invalidateBar()
}

// tabelaLeituraChars traz caracteres/minuto e variação por idioma, como o
// Firefox (ReaderMode._getReadingSpeedForLanguage). "en" é o padrão.
func tabelaLeituraChars() map[string][2]int {
	return map[string][2]int{
		"ar": {612, 88}, "de": {920, 86}, "es": {1025, 127},
		"fi": {1078, 121}, "fr": {998, 126}, "he": {833, 130},
		"it": {950, 140}, "ja": {357, 56}, "nl": {978, 143},
		"pl": {916, 126}, "pt": {913, 145}, "ru": {986, 175},
		"sl": {885, 145}, "sv": {917, 156}, "tr": {1054, 156},
		"zh": {255, 29},
	}
}

// velLeituraChars traz caracteres/minuto e variação por idioma, como o
// Firefox (ReaderMode._getReadingSpeedForLanguage). "en" é o padrão.
func velLeituraChars(lang string) (int, int) {
	if v, ok := tabelaLeituraChars()[lang]; ok {
		return v[0], v[1]
	}

	return cpmPadraoLeitura, variacaoPadraoLeitura
}

// baseIdioma reduz idiomaPtBR a "pt" para a tabela de leitura.
func baseIdioma(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}

	if len(lang) < 2 || len(lang) > 3 {
		return ""
	}

	for _, r := range lang {
		if r < 'a' || r > 'z' {
			return ""
		}
	}

	return lang
}

// etiquetaIdiomaValida diz se um segmento do lang e bem formado.
func etiquetaIdiomaValida(p string) bool {
	if len(p) < 2 || len(p) > 8 {
		return false
	}

	for _, r := range p {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}

	return true
}

// partesIdiomaValidas parte o lang em 1-2 etiquetas validas.
func partesIdiomaValidas(lang string) ([]string, bool) {
	partes := strings.FieldsFunc(lang, func(r rune) bool { return r == '-' || r == '_' })
	if len(partes) == 0 || len(partes) > 2 {
		return nil, false
	}

	for _, p := range partes {
		if !etiquetaIdiomaValida(p) {
			return nil, false
		}
	}

	return partes, true
}

// idiomaPagina valida o lang do artigo para o <html>; cai para pt-BR.
func idiomaPagina(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return idiomaPtBR
	}

	if _, ok := partesIdiomaValidas(lang); !ok {
		return idiomaPtBR
	}

	return lang
}

// tempoLeitura estima "N min de leitura" pelos caracteres, como o Firefox
// (readingTimeMinsSlow/Fast). Vazio quando não há o que estimar.
func tempoLeitura(chars int, lang string) string {
	if chars <= 0 {
		return ""
	}

	cpm, variacao := velLeituraChars(baseIdioma(lang))

	lento, rapido := chars/(cpm-variacao), chars/(cpm+variacao)
	if chars%(cpm-variacao) != 0 {
		lento++
	}

	if chars%(cpm+variacao) != 0 {
		rapido++
	}

	if rapido < 1 {
		rapido = 1
	}

	if lento < 1 {
		lento = 1
	}

	if lento == rapido {
		return fmt.Sprintf(txtFmtLeituraMin, rapido)
	}

	return fmt.Sprintf(txtFmtLeituraMinMin, rapido, lento)
}

// dataLeitura formata o publishedTime do artigo (ISO ou curto) em dd/mm/aaaa;
// fora disso devolve o texto cru aparado, já que é metadado da página.
func dataLeitura(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	for _, layout := range []string{
		time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", time.RFC1123Z,
		time.RFC1123, time.RFC822Z, time.RFC822,
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("02/01/2006")
		}
	}

	if len(raw) > tamanhoMaxMetaLeitura {
		raw = raw[:tamanhoMaxMetaLeitura]
	}

	return raw
}

// leituraExtra carrega os metadados do parse do Firefox que a página de
// leitura exibe (resumo, idioma, direção, data e base do tempo de leitura).
type leituraExtra struct {
	Resumo, Idioma, Direcao, PublicadoEm string
	Chars                                int
}

// juntarMetaLeitura soma um fragmento ao meta com separador.
func juntarMetaLeitura(meta, pedaco string) string {
	if pedaco == "" {
		return meta
	}

	if meta != "" {
		meta += txtSeparador
	}

	return meta + html.EscapeString(pedaco)
}

// metaLeitura compoe a linha site byline data tempo.
func metaLeitura(site, byline string, extra leituraExtra) string {
	meta := html.EscapeString(site)
	if byline != "" {
		if meta != "" {
			meta += txtSeparador
		}

		meta += html.EscapeString(byline)
	}

	meta = juntarMetaLeitura(meta, dataLeitura(extra.PublicadoEm))
	meta = juntarMetaLeitura(meta, tempoLeitura(extra.Chars, extra.Idioma))

	return meta
}

// resumoLeitura aparar o resumo, sem repetir o titulo.
func resumoLeitura(titulo string, extra leituraExtra) string {
	resumo := strings.TrimSpace(extra.Resumo)
	if resumo == titulo {
		return ""
	}

	return resumo
}

// cabecalhoLeitura escreve o topo da pagina de leitura.
func cabecalhoLeitura(pagina *strings.Builder, titulo string, extra leituraExtra, dirAttr string) {
	pagina.WriteString(`<!doctype html>
<html lang="`)
	pagina.WriteString(html.EscapeString(idiomaPagina(extra.Idioma)))
	pagina.WriteString(`"`)
	pagina.WriteString(dirAttr)
	pagina.WriteString(`>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="dark">
<title>`)
	pagina.WriteString(html.EscapeString(titulo))
	pagina.WriteString(txtLeituraSufTitulo + `</title>`)
}

// estiloLeitura devolve o CSS da pagina de leitura.
func estiloLeitura() string {
	return `:root{color-scheme:dark}
*{box-sizing:border-box}
html{background:#101010}
body{margin:0;background:#101010;color:#dedede;font:19px/1.7 Georgia,"Iowan Old Style",` +
		`"Palatino Linotype","Segoe UI",serif}
.barra{position:sticky;top:0;z-index:5;display:flex;align-items:center;gap:12px;padding:10px 18px;
  background:rgba(16,16,16,.96);border-bottom:1px solid #2a2a2a}
.barra button{height:34px;padding:0 13px;border:1px solid #343434;border-radius:17px;background:#1b1b1b;
  color:#e8e8e8;font:13px/1 system-ui,"Segoe UI",sans-serif;cursor:pointer}
.barra button:hover{background:#262626}
.dominio{flex:1;min-width:0;text-align:center;color:#8f8f8f;font:12px/1.2 system-ui,"Segoe UI",sans-serif;
  overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
main{max-width:760px;margin:0 auto;padding:34px 24px 90px}
h1{font-size:clamp(28px,5vw,42px);line-height:1.18;letter-spacing:-.02em;margin:0 0 10px;color:#fff}
.meta{color:#8f8f8f;font:13px/1.4 system-ui,"Segoe UI",sans-serif;margin:0 0 28px}
.resumo{color:#b5b5b5;font-size:18px;line-height:1.55;margin:0 0 8px}
article{font-size:19px;line-height:1.72}
article h1,article h2,article h3,article h4{color:#fff;line-height:1.25;margin:1.5em 0 .55em;letter-spacing:-.01em}
article h2{font-size:1.45em}article h3{font-size:1.2em}
article p{margin:0 0 1.1em}
article a{color:inherit;text-decoration:none;cursor:text}
article blockquote{margin:1.2em 0;padding:0 0 0 1em;border-left:3px solid #3a3a3a;color:#c9c9c9}
article pre,article code{font:14px/1.55 Consolas,"Cascadia Mono",monospace;background:#191919;border-radius:6px}
article pre{padding:12px;overflow:auto}
article code{padding:2px 5px}
article table{border-collapse:collapse;width:100%;margin:1.2em 0;font-size:15px}
article td,article th{border:1px solid #333;padding:6px 8px;vertical-align:top}
article ul,article ol{padding-left:1.45em;margin:0 0 1.1em}
article li{margin:.25em 0}
article hr{border:0;border-top:1px solid #2f2f2f;margin:1.8em 0}
article figure{margin:1.5em 0}
article img{display:block;max-width:100%;height:auto;margin:1.2em auto;border-radius:6px}
article figcaption{color:#8f8f8f;font:12px/1.45 system-ui,"Segoe UI",sans-serif;text-align:center;margin-top:6px}
article video,article iframe,article svg{display:none!important}
::selection{background:#67e8f9;color:#0b0b0b}
@media(max-width:600px){body{font-size:18px}main{padding:24px 16px 70px}.barra{padding:9px 10px;gap:8px}}`
}

// roteiroLeitura devolve o JS da pagina de leitura.
func roteiroLeitura() string {
	return `  var saindo = false;
  function sair(){
    if (saindo) { return; }
    saindo = true;
    try { window.chrome.webview.postMessage(JSON.stringify({t:'reader',acao:'sair'})); } catch (e) {}
    setTimeout(function(){
      try { if (original) { location.href = original; } } catch (e) {}
    }, 120);
  }
  var b = document.getElementById('sair');
  if (b) { b.addEventListener('click', sair); }
  var r = document.getElementById('resumir');
  if (r) { r.addEventListener('click', function(){
    var txt = '';
    try {
      var art = document.querySelector('article');
      txt = String((art ? art.innerText : document.body.innerText) || '');
      if (txt.length > 30000) { txt = txt.slice(0, 30000); }
    } catch (e) {}
    try { window.chrome.webview.postMessage(JSON.stringify({t:'reader',acao:'resumir',titulo:document.title,
      url:location.href,texto:txt})); } catch (e) {}
  }); }
  var t = document.getElementById('topo');
  if (t) { t.addEventListener('click', function(){ window.scrollTo({top:0,behavior:'smooth'}); }); }
  document.addEventListener('keydown', function(e){
    if (e.key === 'Escape') { e.preventDefault(); sair(); }
  });
})();
</script>
</body>
</html>`
}

// corpoLeitura escreve barra, artigo e roteiro da pagina.
func corpoLeitura(pagina *strings.Builder, titulo, site, resumo, meta, conteudo string, originalJS []byte) {
	pagina.WriteString(`<style>`)
	pagina.WriteString(estiloLeitura())
	pagina.WriteString(`</style>
</head>
<body>
<div class="barra">
  <button id=idSair type="button">` + txtLeituraVoltar + `</button>
  <button id="resumir" type="button">` + txtLeituraResumo + `</button>
  <span class="dominio">`)
	pagina.WriteString(html.EscapeString(site))
	pagina.WriteString(`</span>
  <button id="topo" type="button">` + txtLeituraTopo + `</button>
</div>
<main>
  <h1>`)
	pagina.WriteString(html.EscapeString(titulo))
	pagina.WriteString(`</h1>`)

	if resumo != "" {
		pagina.WriteString(`<p class="resumo">` + html.EscapeString(resumo) + `</p>`)
	}

	if meta != "" {
		pagina.WriteString(`<p class="meta">` + meta + `</p>`)
	}

	pagina.WriteString(`<article>`)
	pagina.WriteString(conteudo)
	pagina.WriteString(`</article>
</main>
<script>
(function(){
  var original = `)
	pagina.Write(originalJS)
	pagina.WriteString(";\n")
	pagina.WriteString(roteiroLeitura())
}

func montarPaginaLeitura(titulo, site, byline, original, conteudo string, extra leituraExtra) string {
	titulo = strings.TrimSpace(titulo)
	if titulo == "" {
		titulo = txtLeituraTitulo
	}

	site = strings.TrimSpace(site)
	byline = strings.TrimSpace(byline)

	resumo := resumoLeitura(titulo, extra)

	//nolint:errchkjson // Marshal de string e infalivel; sem erro possivel.
	originalJS, _ := json.Marshal(strings.TrimSpace(original))

	meta := metaLeitura(site, byline, extra)

	dirAttr := ""
	if strings.TrimSpace(extra.Direcao) == "rtl" {
		dirAttr = ` dir="rtl"`
	}

	var b strings.Builder
	cabecalhoLeitura(&b, titulo, extra, dirAttr)
	corpoLeitura(&b, titulo, site, resumo, meta, conteudo, originalJS)

	return b.String()
}
