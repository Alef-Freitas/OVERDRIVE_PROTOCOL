//go:build windows

package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed assets/home-bg.png
var homeBgPNG []byte

//go:embed chatgpt-lite.js
var chatGPTLiteJS string

//go:embed img-lite.js
var imgLiteJS string

//go:embed dom-lite.js
var domLiteJS string

//go:embed sleep-lite.js
var sleepLiteJS string

//go:embed force-dark.js
var forceDarkJS string

//go:embed darkreader.js
var darkReaderJS string

//go:embed clearurls.json
var clearURLsJSON string

// singlefile.js: bundle UMD (global singlefile) gerado com bun a partir de
// single-file 1.2.4 + single-file-core 1.13.0, por Gildas Lormeau,
// licenca AGPL-3.0-or-later, fonte: https://github.com/gildas-lormeau/SingleFile
//
//go:embed singlefile.js
var singleFileJS string

//go:embed readability.js
var readabilityEmbutida string

//go:embed readerable.js
var readerableEmbutida string

//go:embed livre-clique.js
var livreCliqueJS string

//go:embed scroll-lite.js
var scrollLiteJS string

//go:embed senhas-lite.js
var senhasLiteJS string

//go:embed ai-lite.js
var aiLiteJS string

//go:embed traducao-lite.js
var traducaoLiteJS string

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var (
	wndProcPtr  uintptr
	editProcPtr uintptr
)

//nolint:gochecknoglobals // Contador atomico de limpeza pesada em voo.
var memoriaPesadaRodando int32

const (
	tempoClienteYt          = 25 * time.Second
	iconeErroWin            = 0x10
	codigoSaidaFatal        = 3
	esperaPrivadosInicial   = 12 * time.Second
	esperaPrivadosPeriodo   = 60 * time.Second
	esperaLimpezaLeveInicio = 3 * time.Second
	esperaLimpezaPesadaIni  = 12 * time.Second
	maxGuiasFechadas        = 10
)

//nolint:tagliatelle // Espelho do manifesto Chrome: chaves com underline sao o contrato da API de extensoes.
type extManifest struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	HomepageURL     string            `json:"homepage_url"`
	DefaultLocale   string            `json:"default_locale"`
	Icons           map[string]string `json:"icons"`
	Permissions     []string          `json:"permissions"`
	HostPermissions []string          `json:"host_permissions"`
	ContentScripts  []struct {
		Matches []string `json:"matches"`
	} `json:"content_scripts"`
	OptionsUI *struct {
		Page      string `json:"page"`
		OpenInTab bool   `json:"open_in_tab"`
	} `json:"options_ui"`
	OptionsPage string `json:"options_page"`
	Action      *struct {
		DefaultPopup string `json:"default_popup"`
	} `json:"action"`
	BrowserAction *struct {
		DefaultPopup string `json:"default_popup"`
	} `json:"browser_action"`
}

const idleSuspenderMs = 30000

const (
	avisoClasse      = "OverdriveAvisoWnd"
	avisoDuracaoMs   = 12000
	avisoDuracaoOk   = 8000
	avisoDuracaoErro = 20000
	avisoProgressoMs = 15000
	avisoAnimMs      = 150
	avisoMaxLargura  = 0.40
	avisoMaxLinhas   = 3
)

type avisoJanela struct {
	hwnd      uintptr
	texto     string
	visto     string
	quando    time.Time
	linhas    []string
	duracao   int
	visivel   bool
	temBarra  bool
	pct       int
	indet     bool
	altura    int32
	offsetY   int32
	animDe    int32
	animAte   int32
	animIni   time.Time
	animando  bool
	saindo    bool
	fixo      bool
	copiar    string
	btnCopiar rectT
	btnFechar rectT
}

//nolint:gochecknoglobals // Janela de aviso unica do app.
var aviso avisoJanela

//nolint:gochecknoglobals // Ponto de entrada Win32 registrado uma vez no registrarCallbacks.
var avisoProcPtr uintptr

const (
	linkChatMesmaURLJanela = 6 * time.Second
	linkChatMaxJanela      = time.Minute
	linkChatMaxAbas        = 6

	lembreteAguaCada = 90 * time.Minute
)

type configIAJSON struct {
	Key                string   `json:"key"`
	Name               string   `json:"name"`
	URL                string   `json:"url"`
	Hosts              []string `json:"hosts"`
	FieldSelector      string   `json:"fieldSelector"`
	SendButtonSelector string   `json:"sendButtonSelector"`
	Scripts            []string `json:"scripts"`
}

type overdriveConfigJSON struct {
	BrowserName    string            `json:"browserName"`
	Providers      []configIAJSON    `json:"providers"`
	InjectedScript map[string]string `json:"injectedScripts"`
}

//nolint:gochecknoglobals // Estado global de provedores IA/sites: lido uma vez, imutavel depois.
var (
	sitesIA         []siteIA
	pageScripts     = map[string]string{}
	navHelperScript string
	idiomaJS        string
	aplicarJS       string
	anexarImagensJS string
	linksChatJS     string
	chaveValida     = regexp.MustCompile(`^[a-z0-9_-]{1,48}$`)
)

const avisoPaginaJS = `(function(){
  var ID = 'gb-aviso';
  var b = document.getElementById(ID);
  if (!b) {
    if (!document.body) { return 'sem body'; }
    b = document.createElement('div');
    b.id = ID;
    b.style.cssText = 'position:fixed;z-index:2147483645;pointer-events:none;' +
      'top:10px;right:10px;background:rgba(28,28,28,.96);color:#b0b0b0;' +
      'border:1px solid #3a3a3a;border-radius:7px;' +
      'font:12px/1.7 system-ui,sans-serif;padding:2px 8px;white-space:nowrap';
    document.body.appendChild(b);
  }
  b.textContent = 'TEXTO_AQUI';
  b.style.display = 'block';
  if (window.__gbAvisoTimer) { clearTimeout(window.__gbAvisoTimer); }
  window.__gbAvisoTimer = setTimeout(function () { b.style.display = 'none'; }, 6000);
  return 'ok';
})()`

//exhaustruct:ignore // Envio preparado: construtor parcial por desenho.
type envioPreparado struct {
	Botao   bool
	BX      float64
	BY      float64
	Chars   int
	Erro    string
	Ocupado bool
}

//exhaustruct:ignore // Provedor de IA: construtor parcial por desenho.
type siteIA struct {
	chave string
	nome  string
	url   string
	hosts []string

	campo  string
	enviar string

	scripts []string
}

type srcRect struct{ x0, y0, x1, y1 int }

const (
	imgMaxLista  = 120
	imgMaxAnexar = 8

	imgMaxPixels = 2400
)

type imagemPagina struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
	W   int    `json:"w"`
	H   int    `json:"h"`
}

type imagemAnexo struct {
	Nome  string `json:"nome"`
	Tipo  string `json:"tipo"`
	Dados string `json:"dataUrl"`
}

const imagensHomeURL = "https://www.google.com/search?tbm=isch&q="

const imgSearchInitJS = `(function(){
  if (window.__odImgSearch) { return; }
  window.__odImgSearch = true;
  var timer = 0;
  var ultimo = '';
  function post(q) {
    q = String(q || '');
    if (q === ultimo) { return; }
    ultimo = q;
    try { window.chrome.webview.postMessage(JSON.stringify({ t: 'imgq', q: q })); } catch (e) {}
  }
  function atual() {
    var els = document.querySelectorAll('textarea, [contenteditable="true"], input[type="text"]');
    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      var v = (el.value !== undefined) ? el.value : (el.innerText || '');
      if (v && v.length > 1) { return v; }
    }
    return '';
  }
  document.addEventListener('input', function () {
    if (timer) { clearTimeout(timer); }
    timer = setTimeout(function () { post(atual()); }, 400);
  }, true);
})();`

const imagensDaPaginaJS = `(function(){
  try {
    var vistos = {}, lista = [];
    var ims = document.querySelectorAll('img');
    for (var i = 0; i < ims.length; i++) {
      var im = ims[i];
      var src = im.currentSrc || im.src || im.getAttribute('data-src') || '';
      if (!src || src.indexOf('data:image/svg') === 0) { continue; }
      var r = im.getBoundingClientRect();
      var w = Math.max(im.naturalWidth || 0, Math.round(r.width) || 0);
      var h = Math.max(im.naturalHeight || 0, Math.round(r.height) || 0);
      var chave = src.split('#')[0];
      if (vistos[chave]) { continue; }
      vistos[chave] = 1;
      lista.push({
        src: src,
        alt: (im.alt || im.title || '').slice(0, 90),
        w: w, h: h,
        lazy: (!im.complete || im.naturalWidth === 0)
      });
      if (lista.length >= LIMITE_LISTA) { break; }
    }
    lista.sort(function (a, b) { return (b.w * b.h) - (a.w * a.h); });
    return JSON.stringify(lista);
  } catch (e) { return '[]'; }
})()`

const seletorImagensJS = `(function(){
  var LISTA = LISTA_AQUI;
  var NONCE = 'NONCE_AQUI';
  var anterior = document.getElementById('gb-seletor-img');
  if (anterior) { anterior.parentNode.removeChild(anterior); }
  if (!LISTA.length) {
    window.chrome.webview.postMessage(JSON.stringify({t:'gbimg', n:NONCE, acao:'vazio'}));
    return 'vazio';
  }
  var raiz = document.createElement('div');
  raiz.id = 'gb-seletor-img';
  raiz.setAttribute('style', 'position:fixed!important;left:0;top:0;right:0;bottom:0;z-index:2147483647!important;' +
    'background:rgba(0,0,0,.92)!important;color:#f2f2f2!important;font:13px/1.45 system-ui,' +
    'Segoe UI,sans-serif!important;' +
    'display:flex!important;flex-direction:column!important;padding:14px!important;box-sizing:border-box!important;');
  var cab = document.createElement('div');
  cab.setAttribute('style', 'display:flex!important;align-items:center!important;gap:10px!important;' +
    'margin-bottom:10px!important;flex-wrap:wrap!important;');
  cab.innerHTML = '<b style="font-size:14px!important;color:#fff!important">Imagens da página</b>' +
    '<span id="gb-img-conta" style="color:#8a8a8a!important"></span>' +
    '<span style="flex:1"></span>' +
    '<button id="gb-img-todas" style="background:#232323!important;color:#f2f2f2!important;' +
    'border:1px solid #3a3a3a!important;border-radius:7px!important;padding:5px 11px!important;' +
    'font:inherit!important;cursor:pointer!important">Selecionar todas</button>' +
    '<button id="gb-img-nenhuma" style="background:#232323!important;color:#f2f2f2!important;' +
    'border:1px solid #3a3a3a!important;border-radius:7px!important;padding:5px 11px!important;' +
    'font:inherit!important;cursor:pointer!important">Limpar</button>' +
    '<button id="gb-img-fechar" style="background:#232323!important;color:#f2f2f2!important;' +
    'border:1px solid #3a3a3a!important;border-radius:7px!important;padding:5px 11px!important;' +
    'font:inherit!important;cursor:pointer!important">Fechar</button>';
  raiz.appendChild(cab);
  var grade = document.createElement('div');
  grade.setAttribute('style', 'flex:1!important;overflow:auto!important;display:grid!important;' +
    'grid-template-columns:repeat(auto-fill,minmax(140px,1fr))!important;gap:10px!important;' +
    'align-content:start!important;');
  raiz.appendChild(grade);
  var pe = document.createElement('div');
  pe.setAttribute('style', 'display:flex!important;align-items:center!important;gap:10px!important;' +
    'margin-top:10px!important;flex-wrap:wrap!important;');
  pe.innerHTML = '<span id="gb-img-aviso" style="color:#8a8a8a!important"></span><span style="flex:1"></span>' +
    '<button id="gb-img-anexar" style="background:#67e8f9!important;color:#0b0b0b!important;' +
    'border:1px solid #67e8f9!important;border-radius:7px!important;padding:6px 14px!important;' +
    'font:inherit!important;cursor:pointer!important">Anexar ao chat</button>';
  raiz.appendChild(pe);
  document.documentElement.appendChild(raiz);

  var marcadas = {}, celDe = {};
  function atualiza() {
    var n = Object.keys(marcadas).length;
    document.getElementById('gb-img-conta').textContent = n + ' de ' + LISTA.length +
      ' selecionada' + (n === 1 ? '' : 's');
    var b = document.getElementById('gb-img-anexar');
    b.disabled = n === 0 || n > LIMITE_ANEXAR;
    b.style.opacity = b.disabled ? '.5' : '1';
    document.getElementById('gb-img-aviso').textContent = n > LIMITE_ANEXAR
      ? ('escolha no maximo ' + LIMITE_ANEXAR + ' imagens por vez') : '';
  }
  LISTA.forEach(function (it, i) {
    var cel = document.createElement('label');
    cel.setAttribute('style', 'position:relative!important;display:block!important;cursor:pointer!important;' +
      'background:#1c1c1c!important;border:1px solid #3a3a3a!important;border-radius:10px!important;' +
      'padding:8px!important;box-sizing:border-box!important');
    var img = document.createElement('img');
    img.src = it.src;
    img.loading = 'lazy';
    img.setAttribute('style', 'width:100%!important;height:110px!important;object-fit:contain!important;' +
      'background:#000!important;border-radius:6px!important;display:block!important');
    var caixa = document.createElement('input');
    caixa.type = 'checkbox';
    caixa.setAttribute('style', 'position:absolute!important;left:12px!important;top:12px!important;' +
      'width:18px!important;height:18px!important;accent-color:#67e8f9!important;cursor:pointer!important');
    caixa.addEventListener('change', function () {
      if (caixa.checked) { marcadas[i] = it; celDe[i] = cel; } else { delete marcadas[i]; delete celDe[i]; }
      cel.style.borderColor = caixa.checked ? '#67e8f9' : '#3a3a3a';
      atualiza();
    });
    var info = document.createElement('div');
    info.setAttribute('style', 'color:#8a8a8a!important;font-size:11px!important;margin-top:6px!important;' +
      'word-break:break-all!important');
    info.textContent = it.w + 'x' + it.h + (it.alt ? ' - ' + it.alt : '');
    var estado = document.createElement('div');
    estado.className = 'gb-img-estado';
    estado.setAttribute('style', 'font-size:11px!important;margin-top:4px!important;min-height:14px!important');
    cel.appendChild(img); cel.appendChild(caixa); cel.appendChild(info); cel.appendChild(estado);
    grade.appendChild(cel);
  });
  function fechar() {
    if (raiz.parentNode) { raiz.parentNode.removeChild(raiz); }
    document.removeEventListener('keydown', tecla, true);
  }
  function tecla(ev) {
    if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); fechar(); }
  }
  document.addEventListener('keydown', tecla, true);
  document.getElementById('gb-img-fechar').addEventListener('click', fechar);
  document.getElementById('gb-img-nenhuma').addEventListener('click', function () {
    grade.querySelectorAll('input[type=checkbox]').forEach(function (c) {
      if (c.checked) { c.checked = false; c.dispatchEvent(new Event('change')); } });
  });
  document.getElementById('gb-img-todas').addEventListener('click', function () {
    var cs = grade.querySelectorAll('input[type=checkbox]');
    for (var i = 0; i < cs.length && i < LIMITE_ANEXAR; i++) {
      if (!cs[i].checked) { cs[i].checked = true; cs[i].dispatchEvent(new Event('change')); } }
  });
  document.getElementById('gb-img-anexar').addEventListener('click', function () {
    var chaves = Object.keys(marcadas);
    if (!chaves.length) { return; }
    var sel = [], cels = [];
    chaves.forEach(function (k) { sel.push(marcadas[k]); cels.push(celDe[k]); });
    var b = document.getElementById('gb-img-anexar');
    var aviso = document.getElementById('gb-img-aviso');
    b.disabled = true; b.textContent = 'Lendo imagens...';
    aviso.style.color = '#8a8a8a';
    aviso.textContent = 'lendo ' + sel.length + ' imagem(ns) da página...';
    function estado(idx, cor, txt) {
      var cel = cels[idx];
      if (!cel) { return; }
      cel.style.borderColor = cor;
      var s = cel.querySelector('.gb-img-estado');
      if (s) { s.textContent = txt; s.style.color = cor; }
    }
    window.__gbLerImagens(sel, NONCE, function (ok, falhas) {
      b.textContent = 'Anexar de novo';
      b.disabled = false;
      b.style.opacity = '1';
      aviso.style.color = ok ? '#9ee6a0' : '#ff8080';
      aviso.textContent = ok
        ? ((ok === 1 ? '1 imagem enviada' : ok + ' imagens enviadas') + ' ao campo de anexo do chat' +
          (falhas ? ' - ' + falhas + ' falharam' : '') + '. Pode fechar esta janela (Esc).')
        : ('não consegui ler nenhuma destas ' + falhas + ' imagens: este site bloqueia a leitura. Tente outras.');
    }, function (idx, msg) {
      estado(idx, '#ff8080', 'falhou: ' + msg);
    }, function (idx) {
      estado(idx, '#9ee6a0', 'lida e enviada');
    });
  });
  atualiza();
  return 'ok';
})()`

const leitorImagensJS = `(function(){
  function paraJPEG(fonte, largura, altura) {
    var MAX = LIMITE_PIXELS;
    var w = largura || fonte.naturalWidth || fonte.width;
    var h = altura || fonte.naturalHeight || fonte.height;
    if (!w || !h) { throw new Error('imagem sem tamanho'); }
    var esc = Math.min(1, MAX / Math.max(w, h));
    var cw = Math.max(1, Math.round(w * esc)), ch = Math.max(1, Math.round(h * esc));
    var cv = document.createElement('canvas');
    cv.width = cw; cv.height = ch;
    var cx = cv.getContext('2d');
    cx.fillStyle = '#000';
    cx.fillRect(0, 0, cw, ch);
    cx.drawImage(fonte, 0, 0, cw, ch);
    return { dados: cv.toDataURL('image/jpeg', 0.9), tipo: 'image/jpeg' };
  }
  function porImagemComCors(alvo) {
    return new Promise(function (res, rej) {
      var img = new Image();
      img.crossOrigin = 'anonymous';
      img.onload = function () {
        try { res(paraJPEG(img)); } catch (e) { rej(e); }
      };
      img.onerror = function () { rej(new Error('a imagem não carregou')); };
      img.src = alvo.src;
    });
  }
  function porFetch(alvo) {
    return fetch(alvo.src, { credentials: 'include' })
      .then(function (r) {
        if (!r.ok) { throw new Error('HTTP ' + r.status); }
        return r.blob();
      })
      .then(function (b) {
        if (window.createImageBitmap) {
          return createImageBitmap(b).then(function (bmp) {
            try { return paraJPEG(bmp, bmp.width, bmp.height); } finally { try { bmp.close(); } catch (e) {} }
          });
        }
        return new Promise(function (res, rej) {
          var u = URL.createObjectURL(b);
          var img = new Image();
          img.onload = function () {
            try { res(paraJPEG(img)); } catch (e) { rej(e); } finally { URL.revokeObjectURL(u); }
          };
          img.onerror = function () { URL.revokeObjectURL(u); rej(new Error('não decodifiquei o arquivo')); };
          img.src = u;
        });
      });
  }
  function porElemento(alvo) {
    return new Promise(function (res, rej) {
      var lista = document.querySelectorAll('img'), el = null;
      for (var i = 0; i < lista.length; i++) {
        if (lista[i].src === alvo.src || lista[i].currentSrc === alvo.src) { el = lista[i]; break; }
      }
      if (!el || !el.naturalWidth) { rej(new Error('imagem não encontrada na página')); return; }
      try { res(paraJPEG(el)); } catch (e) { rej(e); }
    });
  }
  window.__gbLerImagens = function (alvos, nonce, aoTerminar, aoFalhar, aoLer) {
    function envia(extra) {
      var base = { t: 'gbimg', n: nonce };
      for (var k in extra) { base[k] = extra[k]; }
      try { window.chrome.webview.postMessage(JSON.stringify(base)); } catch (e) {}
    }
    var i = 0, ok = 0, falhas = 0;
    function proxima() {
      if (i >= alvos.length) {
        envia({ acao: 'fim', ok: ok, falhas: falhas });
        if (aoTerminar) { aoTerminar(ok, falhas); }
        return;
      }
      var idx = i, alvo = alvos[i];
      var nome = 'imagem-' + (idx + 1) + '.jpg';
      porImagemComCors(alvo)
        .catch(function () { return porFetch(alvo); })
        .catch(function () { return porElemento(alvo); })
        .then(function (r) {
          ok++;
          envia({ acao: 'dados', i: idx, nome: nome, tipo: 'image/jpeg', dataUrl: r.dados });
          if (aoLer) { aoLer(idx, nome); }
        })
        .catch(function (e) {
          falhas++;
          var msg = String(e && e.message ? e.message : e);
          envia({ acao: 'erro', i: idx, src: alvo.src, msg: msg });
          if (aoFalhar) { aoFalhar(idx, msg); }
        })
        .then(function () { i++; proxima(); });
    }
    proxima();
    return 'lendo';
  };
  return 'ok';
})()`

const maxLeitura = 40000

//nolint:gochecknoglobals // Cache de parser de leitura (sync.Once + fonte): estado de processo.
var readabilityUma sync.Once

//nolint:gochecknoglobals // Cache de parser de leitura (sync.Once + fonte): estado de processo.
var readabilityFonte string

//nolint:gochecknoglobals // Cache de parser de leitura (sync.Once + fonte): estado de processo.
var readerableUma sync.Once

//nolint:gochecknoglobals // Cache de parser de leitura (sync.Once + fonte): estado de processo.
var readerableFonte string

const extrairJS = `(function(){
  var BQ = String.fromCharCode(96), CERCA = BQ + BQ + BQ;
  var LIMITE = LIMITE_CHARS;
  var MODO = 'MODO_AQUI';

  function txt(el) { return String(el.textContent || '').replace(/\s+/g, ' ').trim(); }

  function textoVisivel() {
    var t = String(document.body ? document.body.innerText : '');
    return t.replace(/[ \t]+/g, ' ').replace(/\n{3,}/g, '\n\n').trim();
  }

  function doArtigo(html) {
    var div = document.createElement('div');
    div.innerHTML = html;
    var partes = [];
    function anda(no) {
      var filhos = no.children;
      for (var i = 0; i < filhos.length; i++) {
        var el = filhos[i], tag = el.tagName;
        if (tag === 'SCRIPT' || tag === 'STYLE' || tag === 'NOSCRIPT') { continue; }
        var t = '';
        if (/^H[1-6]$/.test(tag)) {
          t = '\n' + '######'.slice(0, parseInt(tag.charAt(1), 10)) + ' ' + txt(el) + '\n';
        } else if (tag === 'P') {
          t = txt(el) + '\n';
        } else if (tag === 'LI') {
          t = '- ' + txt(el);
        } else if (tag === 'BLOCKQUOTE') {
          t = '> ' + txt(el) + '\n';
        } else if (tag === 'PRE') {
          t = CERCA + '\n' + String(el.textContent || '').replace(/\s+$/, '') + '\n' + CERCA + '\n';
        } else if (tag === 'FIGCAPTION') {
          t = '(' + txt(el) + ')';
        } else if (tag === 'HR') {
          t = '---';
        } else if (tag === 'TD' || tag === 'TH') {
          t = txt(el) + ' |';
        } else if (el.children && el.children.length) {
          anda(el);
          continue;
        } else {
          t = txt(el);
          if (!t) { continue; }
        }
        if (t.replace(/\s+/g, '')) { partes.push(t); }
      }
    }
    anda(div);
    return partes.join('\n').replace(/\n{3,}/g, '\n\n').trim();
  }

  var art = null, erro = '';
  if (MODO === 'artigo' && typeof Readability === 'function') {
    try {
      art = new Readability(document.cloneNode(true), {charThreshold: 150}).parse();
    } catch (e) { erro = String(e && e.message || e); }
  } else if (MODO === 'artigo') {
    erro = 'Readability não carregou';
  }

  var visivel = textoVisivel();
  var corpo = '', titulo = String(document.title || ''), metodo = '';
  var charsArtigo = 0;

  if (art && art.content) {
    corpo = doArtigo(art.content);
    charsArtigo = corpo.length;
    titulo = String(art.title || titulo);
    metodo = 'artigo (Readability)';
  }

  if (MODO === 'tudo') {
    corpo = visivel;
    metodo = 'página inteira (texto visível)';
  } else if (corpo.length < 200 || corpo.length < visivel.length * 0.5) {
    if (visivel.length > corpo.length) {
      corpo = visivel;
      metodo = 'texto visível (o artigo do Readability tinha ' + charsArtigo +
        ' de ' + visivel.length + ' caracteres)';
    }
  }

  if (!corpo.replace(/\s+/g, '')) {
    return {ok:false, err: erro || 'não achei texto na página'};
  }
  var cab = 'TÍTULO DA PÁGINA:\n' + (titulo || '(sem título)') + '\n\n' +
            'URL DA PÁGINA:\n' + location.href + '\n\n' +
            'CONTEÚDO DA PÁGINA (' + (MODO === 'tudo' ? 'texto visível' : 'artigo') + '):\n';
  var fim = '\n\n[FIM DO CONTEÚDO DA PÁGINA]';
  var texto = cab + corpo + fim;
  var truncado = false;
  if (texto.length > LIMITE) {
    var sobra = LIMITE - cab.length - fim.length - 16;
    if (sobra < 200) { sobra = LIMITE - cab.length - 16; }
    texto = cab + corpo.slice(0, sobra) + '\n...[truncado]' + fim;
    truncado = true;
  }
  return {ok:true, titulo:titulo, metodo:metodo, chars:texto.length, truncado:truncado,
          charsArtigo:charsArtigo, charsVisivel:visivel.length, texto:texto};
})()`

type dadosLogin struct {
	Email string `json:"email"`
	Senha string `json:"senha"`

	Atual  int          `json:"atual"`
	Contas []contaLogin `json:"contas"`
	Auto   bool         `json:"auto"`
}

//nolint:gochecknoglobals // Sessao de login ativa: estado global do app single-window.
var loginAtual dadosLogin

//exhaustruct:ignore // Conta de login: construtor parcial por desenho.
type contaLogin struct {
	Apelido string `json:"apelido"`
	Email   string `json:"email"`
	Senha   string `json:"senha"`
}

const (
	appVersion = "1.0"
)

//nolint:gochecknoglobals // Identidade do app: constantes de processo.
var (
	appTitle = "Overdrive"
	urlChat  = "https://chat.deepseek.com/"
)

//nolint:gochecknoglobals // Diretorios resolvidos uma vez no main: estado de processo.
var (
	exeDir     string
	dataDir    string
	runtimeDir string
	extDir     string
)

//nolint:gochecknoglobals // Flags de inicializacao: lidas uma vez, imutaveis depois.
var profileOverride string

//nolint:gochecknoglobals // Flags de inicializacao: lidas uma vez, imutaveis depois.
var perfilPrivado bool

//exhaustruct:ignore // Estado global: construtor parcial por desenho.
type state struct {
	WinX             int32   `json:"winx"`
	WinY             int32   `json:"winy"`
	WinW             int32   `json:"winw"`
	WinH             int32   `json:"winh"`
	Split            float64 `json:"split"`
	ImgSplit         float64 `json:"imgsplit"`
	LeftPanelVisible bool    `json:"leftpanelvisible"`
	LeftPanelSplit   float64 `json:"leftpanelsplit"`
	RightVisible     bool    `json:"rightvisible"`
	ImgVisible       bool    `json:"imgvisible"`
	Zoom             float64 `json:"zoom"`
	ZoomPainelIA     float64 `json:"zoompainelia"`
	RightURL         string  `json:"righturl"`
	RightSite        string  `json:"rightsite"`

	Idioma           string                 `json:"idioma"`
	ModoTraducao     string                 `json:"modotraducao"`
	MotorBusca       string                 `json:"motorbusca"`
	SemServiceWorker bool                   `json:"semserviceworker"`
	SemGPU           bool                   `json:"semgpu"`
	EconomiaMax      bool                   `json:"economiamax"`
	LimpezaAcao      bool                   `json:"limpezaacao"`
	LimparClip       bool                   `json:"limparclip"`
	SleepLite        bool                   `json:"sleeplite"`
	WebviewUnica     bool                   `json:"webviewunica"`
	LivreClique      bool                   `json:"livreclique"`
	SenhasAuto       bool                   `json:"senhasauto"`
	ForceDark        bool                   `json:"forcedark"`
	ExtraArgs        string                 `json:"extraargs"`
	Extensions       []string               `json:"extensions"`
	ExtFixadas       []string               `json:"extfixadas"`
	ExtFixarNovas    bool                   `json:"extfixarnovas"`
	ExtDominios      map[string]regraDomExt `json:"extdominios"`
	GuiasFechadas    []tabState             `json:"guiasfechadas"`
	Maximized        bool                   `json:"maximized"`
	StateVersion     int                    `json:"stateversion"`
}

//nolint:gochecknoglobals // Ultimo estado serializado: escrita unica no shutdown, leitura unica no boot.
var stateSaved []byte

//exhaustruct:ignore // Aba/painel: construtor parcial por desenho (campos opcionais).
type pane struct {
	wv            *WebView
	rect          Rect
	boundsApplied Rect
	boundsSet     bool
	ready         bool
	mobile        bool
	zoom          float64
	zoomLogged    float64
	busca         bool
	titulo        string

	visivel    bool
	visivelSet bool

	queriaURL  string
	tentativas int

	ultimaURL string

	emulFixa bool

	cssW, cssH int32

	frio           bool
	frioManual     bool
	frioPendente   bool
	nojsPendente   bool
	nojsAtivo      bool
	nojsCarregando bool
	sonoDormindo   bool
	ociosoDesde    time.Time
	frioDesde      time.Time

	readerAtivo      bool
	readerCarregando bool
	readerPronto     bool
	readerURL        string

	descarregado    bool
	urlDescarga     string
	scrollDescarga  float64
	scrollRestaurar float64

	tocandoAudio bool
	mudo         bool

	ultimaLimpezaSono time.Time
}

//exhaustruct:ignore // Botao de barra: construtor parcial por desenho.
type tbutton struct {
	id    string
	label string
	rect  rectT
	on    bool
}

type layoutKey struct {
	w, h, tb int32
	split    float64
	rightVis bool
	l, r     Rect
	popup    bool
}

type app struct {
	hwnd uintptr
	env  *Environment

	left                     *pane
	right                    *pane
	imgPanel                 *pane
	leftAux                  *pane
	leftAuxSplitX            int32
	leftAuxBtn               rectT
	leftAuxFullBtn           rectT
	painelCheio              bool
	dragLeftAuxSplit         bool
	popup                    *pane
	popupOn                  bool
	popupGen                 uint64
	popupAltura              int32
	popupVigiaGen            uint64
	urlEdit                  uintptr
	urlEditing               bool
	oldEditProc              uintptr
	editCache                string
	editCacheOK              bool
	dpi                      int
	tbHeight                 int32
	fontUI                   uintptr
	fontDPI                  int
	iconFace                 string
	tipHwnd                  uintptr
	titulo                   string
	loading                  bool
	traduzido                bool
	tradRetentou             bool
	traducaoOrigemURL        string
	readerUsavel             bool
	readerCheckURL           string
	ultimaAgua               time.Time
	recorteTimerGen          uint64
	chatVazioTentou          bool
	imgNonce                 string
	imgLidas                 []imagemAnexo
	imgErros                 int
	chatTravando             bool
	chatGerando              bool
	chatAtivo                bool
	chatManual               bool
	chatUsado                bool
	chatFrio                 bool
	chatFrioPendente         bool
	chatFrioGen              uint64
	descarteGen              uint64
	leituraGen               uint64
	envioGen                 uint64
	envioAtivo               bool
	ultimaLeitura            time.Time
	ultimaLeituraModo        string
	ultimoReload             time.Time
	ultimaRecuperacaoAba     time.Time
	leituraAtiva             bool
	imgInteragido            time.Time
	ultimaLimpezaScroll      time.Time
	scrollLimpezaTimer       *time.Timer
	tipText                  string
	tipAnchor                rectT
	tipPending               bool
	tipShown                 bool
	fontSm                   uintptr
	buttons                  []tbutton
	extBtn                   tbutton
	extPinBtns               []tbutton
	uboBtn                   tbutton
	downBtn                  tbutton
	menuBtn                  tbutton
	toolsBtn                 tbutton
	printBtn                 tbutton
	clipBtn                  tbutton
	trBtn                    tbutton
	nojsBtn                  tbutton
	sleepBtn                 tbutton
	vpnBtn                   tbutton
	cookiesBtn               tbutton
	openLeftBtn              tbutton
	paywallBtn               tbutton
	readerBtn                tbutton
	readerCopyBtn            tbutton
	engineBtn                tbutton
	securityBtn              tbutton
	privadoBadge             rectT
	splitBtn                 rectT
	chatHomeBtn              rectT
	panelMenuBtn             rectT
	panelTranslateBtn        rectT
	panelSendBtn             rectT
	panelFullBtn             rectT
	panelReloadBtn           rectT
	imgToggleBtn             rectT
	soIA                     bool
	hover                    string
	dragSplit                bool
	dragImgSplit             bool
	imgSplitY                int32
	splitXAntes              int32
	splitX                   int32
	rail                     int32
	brBg                     uint32
	brChrome                 uint32
	brSurface                uint32
	brHover                  uint32
	brActive                 uint32
	brSep                    uint32
	brBorder                 uint32
	brTipBg                  uint32
	brTipBorder              uint32
	brTabOn                  uint32
	brAccent                 uint32
	winMin, winMax, winClose rectT
	winClock                 rectT
	relogioTxt               string
	relogioMin               int64
	winHover                 int
	brFechar                 uint32
	brIcon                   uint32
	brIconHover              uint32

	brEdit            uintptr
	tabs              []*tab
	guiasFechadas     []tabState
	ativa             int
	contextAba        int
	menuOrigem        string
	subAtrasoIdx      int
	subAtrasoQuando   time.Time
	paywallAlvo       string
	janelaAtiva       bool
	tudoSuspenso      bool
	ytAberto          bool
	ytOculto          bool
	ytPane            *pane
	ytBtn             tbutton
	linkChatVistos    map[string]time.Time
	linkChatConta     int
	linkChatJanela    time.Time
	chatPendente      func()
	anexosPendentes1  []imagemAnexo
	escolhaURL        string
	escolhaDestino    *pane
	tabH              int32
	tabRects          []rectT
	tabCloseRects     []rectT
	tabAudioRects     []rectT
	tabNewRect        rectT
	tabOverflowRect   rectT
	tabOverflowN      int
	tabLimitePintura  int32
	tabHover          int
	tabHoverClose     int
	tabHoverAudio     int
	tabDrag           int
	tabDragStartX     int32
	tabDragAtivo      bool
	tabDrop           int
	permissoes        map[string]map[uint32]bool
	permissoesMu      sync.RWMutex
	downloads         []*downloadAtivo
	downloadOps       map[uintptr]*downloadAtivo
	downloadOpsMu     sync.Mutex
	downloadSeq       int
	downloadRegCarreg bool
	downloadsReparado bool
	st                state
	focus             string
	status            string
	extReady          bool
	extLog            string

	popupPath        string
	extIDs           map[string]string
	extPopups        map[string]string
	extLoaded        map[string]bool
	extUpdate        map[string]estadoUpdateExt
	extDominioEstado map[string]bool
	senhasNegadas    map[string]bool
	extIniciado      bool
	extInstalando    bool

	popupExtID   string
	popupExtNome string

	extFloat        *pane
	extFloatOn      bool
	extFloatHWND    uintptr
	extFloatGen     uint64
	extFloatVigia   uint64
	extFloatID      string
	extFloatNome    string
	extFloatAltura  int32
	extFloatMedH    int
	extFloatMedW    int
	extFloatMedN    int
	extFloatMedido  bool
	extFloatEstavel int
	extFloatFixo    bool
	extLimpezaGen   int
	ctxHandlers     []uintptr
	addrFrame       rectT
	editRect        rectT
	trackingMouse   bool
	clientCached    rectT
	clientValid     bool
	lastLayout      layoutKey
	lastLayoutSet   bool
	quitting        bool
	persistTimer    *time.Timer
	persistPendente bool
	limpezaTimer    *time.Timer
	limpezaPendente bool
	limpezaUltima   time.Time
	ultimoBounds    time.Time
	loadPhase       int32
	loadStart       time.Time
	tickOcioso      uint64
	tickPesado      uint64

	vpn         *vpnManager
	tarefasMu   sync.Mutex
	tarefasFila []func()
}

//nolint:gochecknoglobals // Instancia unica do app: atribuida antes da primeira mensagem da janela.
var theApp *app

const (
	corFundo       = 0x000B0B0B
	corSuperficie  = 0x001C1C1C
	corSuperficie2 = 0x00232323
	corSuperficie3 = 0x002A2A2A
	corBorda       = 0x002F2F2F
	corBordaForte  = 0x003A3A3A
	corTexto       = 0x00F2F2F2
	corTextoFraco  = 0x00B0B0B0
	corTextoFaint  = 0x008A8A8A
	corAccent      = 0x00F9E867
	corAccentSoft  = 0x00FCF3A5
	corSuccess     = 0x0099D334
	corWarning     = 0x0024BFFB
	corDanger      = 0x007171F8

	corCianoClaro = corTexto
	corCiano      = corTextoFraco
	corCianoForte = corAccent
)

//nolint:gochecknoglobals // Callback fixo do EnumWindows: registrado uma vez.
var enumRaiseCb = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	aplicativo := theApp
	if aplicativo == nil {
		return 0
	}

	var area rectT
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&area)))
	ponto := struct{ x, y int32 }{area.Left, area.Top}
	pScreenToClient.Call(aplicativo.hwnd, uintptr(unsafe.Pointer(&ponto)))

	match := func(p *pane) bool {
		if p == nil || !p.ready {
			return false
		}

		if p == aplicativo.popup && (!aplicativo.popupOn || p.rect.Width() <= 0) {
			return false
		}

		if (p == aplicativo.right) && (!aplicativo.st.RightVisible || p.rect.Width() <= 0) {
			return false
		}

		return absI32(ponto.x-p.rect.Left) <= 3 && absI32(ponto.y-p.rect.Top) <= 3
	}
	if match(aplicativo.left) || match(aplicativo.right) || match(aplicativo.popup) {
		pSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoActivate)
	}

	return 1
})

// scrollbarFixJS cuida so da barra de rolagem escura. Nao pode forcar
// fundo nem color-scheme: fundo preto engana o detector do modo escuro
// (paginaClara) e quebra o texto dos sites claros.
const scrollbarFixJS = `(function(){
  if(window.__odScrollbarFix) return;
  try{
    if(window.top!==window) return;
    var proto=location.protocol;
    if(proto!=='https:'&&proto!=='http:') return;
    var h=location.hostname||'';
    if(h==='127.0.0.1'||h==='localhost'||h==='::1') return;
    window.__odScrollbarFix=true;
    var css='*{scrollbar-width:auto!important;scrollbar-color:#5a5a5a #141414!important}'+
      '::-webkit-scrollbar{width:14px!important;height:14px!important;background:#141414!important}'+
      '::-webkit-scrollbar-track{background:#141414!important}'+
      '::-webkit-scrollbar-thumb{background:#5a5a5a!important;border-radius:7px!important;'+
      'border:3px solid #141414!important;min-height:40px!important}'+
      '::-webkit-scrollbar-thumb:hover{background:#7a7a7a!important}'+
      '::-webkit-scrollbar-corner{background:#141414!important}';
    function add(){
      try{
        var s=document.getElementById('od-scrollbar-fix');
        if(!s){
          s=document.createElement('style');
          s.id='od-scrollbar-fix';
          (document.head||document.documentElement).appendChild(s);
        }
        s.textContent=css;
      }catch(e){}
    }
    if(document.head) add(); else document.addEventListener('DOMContentLoaded',add,{once:true});
  }catch(e){}
})();`

const semAutoplayJS = `(function(){
  if(window.__odSemAutoplay) return;
  window.__odSemAutoplay=true;
  var liberado=false;
  function liberar(){liberado=true;}
  // Só gesto de verdade libera (clique/toque/tecla). Scroll liberava tudo:
  // era só rolar a página para os vídeos começarem sozinhos.
  ['pointerdown','keydown'].forEach(function(e){
    window.addEventListener(e,liberar,{capture:true,once:true,passive:true});
  });
  function ehMidia(v){return v&&(v.tagName==='VIDEO'||v.tagName==='AUDIO');}
  window.addEventListener('play',function(ev){
    var v=ev.target;
    if(!ehMidia(v)) return;
    if(liberado) return;
    // Só pausa: mutar aqui deixava o vídeo mudo quando o usuário dava play.
    try{v.pause();}catch(e){}
  },true);
  var vio=null;
  if(window.IntersectionObserver){
    vio=new IntersectionObserver(function(es){
      for(var i=0;i<es.length;i++){
        var v=es[i].target;
        if(!es[i].isIntersecting&&ehMidia(v)&&!v.paused){try{v.pause();}catch(e){}}
      }
    },{threshold:0});
  }
  function prepararVideo(v){
    if(!ehMidia(v)||v.__odVid) return;
    v.__odVid=true;
    try{if(!v.hasAttribute('preload'))v.preload='none';}catch(e){}
    if(vio) vio.observe(v);
  }
  function observarMidia(root){
    if(!root||root.nodeType!==1) return;
    var list=root.tagName==='VIDEO'||root.tagName==='AUDIO'?[root]:root.querySelectorAll('video,audio');
    for(var i=0;i<list.length;i++) prepararVideo(list[i]);
  }
  var mo=new MutationObserver(function(ms){
    for(var i=0;i<ms.length;i++) for(var j=0;j<ms[i].addedNodes.length;j++) observarMidia(ms[i].addedNodes[j]);
  });
  function start(){
    observarMidia(document.body);
    try{mo.observe(document.body,{childList:true,subtree:true});}catch(e){}
  }
  if(document.body) start(); else document.addEventListener('DOMContentLoaded',start,{once:true});
})();`

const cookieRejectJS = `(function(){
  if (window.__odCookieReject) return;
  window.__odCookieReject = true;
  var clicados = new WeakSet();
  var rejeita = /(reject|decline|deny|refuse|disagree|do not agree|don't agree|without accepting|` +
	`only necessary|necessary only|essential only|use necessary|strictly necessary|no thanks|not now|` +
	`rejeitar|recusar|discordar|n\u00e3o concordo|nao concordo|n\u00e3o aceito|nao aceito|apenas necess|` +
	`somente necess|continuar sem aceitar|dispensar|rechazar|solo necesarias|s\u00f3lo necesarias|` +
	`continuar sin aceptar|refuser|rejeter|uniquement n\u00e9cessaires|continuer sans accepter|` +
	`ablehnen|verweigern|nur notwendige|nur erforderliche|rifiuta|nega|solo necessari|` +
	`continua senza accettare|weigeren|alleen noodzakelijke)/i;
  var aceita = /(accept all|allow all|agree to all|accept cookies|aceitar todos|aceitar cookies|` +
	`concordo com todos|permitir todos|aceptar todo|tout accepter|alle akzeptieren|accetta tutto|got it|entendi)/i;
  function visivel(e){
    try{
      var r=e.getBoundingClientRect(), s=getComputedStyle(e);
      if(r.width<=0||r.height<=0||s.visibility==='hidden'||s.display==='none'||s.opacity==='0'||e.disabled)
        return false;
      return true;
    }catch(x){return false;}
  }
  function texto(e){
    var t='';
    try{t=(e.innerText||e.textContent||e.value||e.getAttribute('aria-label')||e.getAttribute('title')||'')
      .toString();}catch(x){}
    return t.trim().replace(/\s+/g,' ');
  }
  function esconderBanner(e){
    try{
      var alvo=null, cur=e;
      for(var i=0;i<10&&cur&&cur!==document.body&&cur!==document.documentElement;i++){
        var s=getComputedStyle(cur);
        var idc=((cur.id||'')+' '+(typeof cur.className==='string'?cur.className:'')).toLowerCase();
        var z=parseInt(s.zIndex,10);
        if(/cookie|consent|privacy|gdpr|lgpd|privacidade|banner|dialog|modal|popup/.test(idc)||
           cur.getAttribute('role')==='dialog'||cur.getAttribute('aria-modal')==='true'||
           s.position==='fixed'||s.position==='sticky'||(!isNaN(z)&&z>=1000)){
          alvo=cur;
        }
        cur=cur.parentElement;
      }
      if(!alvo) return false;
      var r=alvo.getBoundingClientRect();
      if(r.width<40||r.height<20) return false;
      if(r.width>=innerWidth*0.98&&r.height>=innerHeight*0.9) return false;
      alvo.style.setProperty('display','none','important');
      alvo.style.setProperty('visibility','hidden','important');
      return true;
    }catch(x){return false;}
  }
  function clicar(e){
    if(!e||clicados.has(e)||!visivel(e)) return false;
    var t=texto(e);
    if(!t||t.length>120) return false;
    if(aceita.test(t)&&!rejeita.test(t)) return false;
    if(!rejeita.test(t)) return false;
    clicados.add(e);
    try{e.click();}catch(x){return false;}
    setTimeout(function(){
      try{if(visivel(e)) esconderBanner(e);}catch(x){}
    },1200);
    return true;
  }
  function sombras(host){
    var out=[], fila=[host], n=0;
    while(fila.length&&n<40){
      var r=fila.shift(); n++;
      if(!r) continue;
      out.push(r);
      var all;
      try{all=r.querySelectorAll('*');}catch(x){continue;}
      for(var i=0;i<all.length&&i<12000;i++){ if(all[i].shadowRoot) fila.push(all[i].shadowRoot); }
    }
    return out;
  }
  function candidatos(){
    var sel=['#usercentrics-cmp-ui','#usercentrics-root','#usercentrics-button',
      '#onetrust-banner-sdk','#onetrust-pc-sdk','#onetrust-consent-sdk',
      '#CybotCookiebotDialog','#didomi-notice','#didomi-host','.didomi-popup-container',
      '#truste-consent-track','#truste-consent-content','.truste_box_overlay',
      '.osano-cm-window','#qc-cmp2-container','#qc-cmp2-ui','.fc-consent-root',
      '#sp_message_container_0','[id^="sp_message_container"]','.klaro','#klaro',
      '.cmplz-cookiebanner','#cmplz-cookiebanner-container','#BorlabsCookieBox','.borlabs-cookie',
      '.iubenda-cs-container','#iubenda-cs-banner','.cky-consent-container','#cookie-law-info-bar',
      '#termly-code-snippet-support','.termly-styles-root','#hs-eu-cookie-confirmation',
      '#shopify-pc__banner','#wcpConsentBannerCtrl','#sp-cc','#cookieConsent','#cookie-banner',
      '#cookieBanner','#consent-banner','#consentBanner','[class*="cookie-banner" i]',
      '[class*="cookie-consent" i]','[class*="consent-banner" i]','[class*="consent-modal" i]'];
    var out=[], vistos=new Set();
    function add(el){ if(!el||vistos.has(el))return; vistos.add(el); out.push(el); }
    for(var i=0;i<sel.length;i++){
      var list; try{list=document.querySelectorAll(sel[i]);}catch(x){continue;}
      for(var j=0;j<list.length&&j<40;j++) add(list[j]);
    }
    var base=out.slice(0);
    for(var k=0;k<base.length;k++){
      var rs=sombras(base[k]);
      for(var m=0;m<rs.length;m++) add(rs[m]);
    }
    return out;
  }
  var conhecidos=['#onetrust-reject-all-handler','.ot-pc-refuse-all-handler',
    '#CybotCookiebotDialogBodyButtonDecline','#CybotCookiebotDialogBodyLevelButtonLevelOptinDeclineAll',
    '[data-testid="uc-deny-all-button"]','[data-testid="uc-reject-all-button"]','[data-testid="uc-deny-button"]',
    '.didomi-continue-without-agreeing','#didomi-notice-disagree-button',
    '[data-testid="didomi-notice-disagree-button"]',
    '.truste-consent-required','.osano-cm-denyAll','.osano-cm-button--type_denyAll',
    '.cm-btn-decline','.cmplz-deny','#cmplz-deny','#BorlabsCookieBtnDeny','.borlabs-cookie-btn-deny',
    '.iubenda-cs-reject-btn','.cky-btn-reject','[data-cky-tag="reject-button"]',
    '#termly-decline','[data-tid="banner-decline"]','#hs-eu-decline-button',
    '#shopify-pc__banner__btn-decline','#W0wltc','#reject-button','#sp-cc-rejectall-link',
    '[data-testid="cookie-policy-manage-dialog-decline-button"]','[data-testid="reject-all"]',
    'button[title="Reject All" i]','button[title="Rejeitar tudo" i]',
    'button[aria-label*="reject all" i]','button[aria-label*="rejeitar tudo" i]','button[aria-label*="recusar tudo" i]',
    'button[aria-label*="decline" i]','button[aria-label*="recusar" i]','button[aria-label*="rejeitar" i]'];
  function clicarSombra(hostSel, btnSel){
    var hosts; try{hosts=document.querySelectorAll(hostSel);}catch(x){return false;}
    for(var i=0;i<hosts.length;i++){
      var sr=hosts[i].shadowRoot;
      if(!sr) continue;
      var bs; try{bs=sr.querySelectorAll(btnSel);}catch(x){continue;}
      for(var j=0;j<bs.length;j++){ if(clicar(bs[j])) return true; }
    }
    return false;
  }
  function tentar(){
    if(clicarSombra('#usercentrics-cmp-ui','#deny,.uc-deny-button,.deny')) return true;
    if(clicarSombra('#usercentrics-root','#deny,.uc-deny-button,.deny')) return true;
    for(var k=0;k<conhecidos.length;k++){
      var list; try{list=document.querySelectorAll(conhecidos[k]);}catch(x){continue;}
      for(var j=0;j<list.length;j++){ if(clicar(list[j])) return true; }
    }
    var cs=candidatos();
    for(var c=0;c<cs.length;c++){
      var els;
      try{els=cs[c].querySelectorAll('button,a,[role="button"],input[type="button"],'+
        'input[type="submit"]');}catch(x){continue;}
      for(var i=0;i<els.length&&i<400;i++){ if(clicar(els[i])) return true; }
    }
    return false;
  }
  var parar=false, n=0;
  function tick(){
    if(parar) return;
    try{if(tentar()){setTimeout(tick,350);}}catch(e){}
  }
  function agendar(){try{clearTimeout(window.__odCookieTimer);window.__odCookieTimer=setTimeout(tick,200);}catch(e){}}
  var iv=setInterval(function(){
    n++;
    if(n>600){clearInterval(iv);parar=true;return;}
    if(n<=120||n%5===0) tick();
  },1000);
  try{
    var obs=new MutationObserver(agendar);
    obs.observe(document.documentElement||document,{childList:true,subtree:true});
    setTimeout(function(){try{obs.disconnect();}catch(e){}},600000);
  }catch(e){}
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',tick,{once:true});else tick();
  window.addEventListener('load',tick,{once:true});
})();`

const mobileUA = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/155.0.0.0 Mobile Safari/537.36"

const desktopUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/155.0.0.0 Safari/537.36"

const chatMaxWidthLogical = 455

const (
	railLogico           = 30
	railFinoLogico       = 12
	splitMin             = 0.25
	splitMax             = 0.92
	leftPanelSplitPadrao = 0.22
	leftPanelSplitMin    = 0.14
	leftPanelSplitMax    = 0.42
	leftPanelMinLogical  = 170
	painelMinEsq         = 240
	painelMinDir         = 300
	imgSplitMin          = 0.15
	imgSplitMax          = 0.85
	imgSplitPadrao       = 0.33
	imgAlturaMin         = 120
	escalaMinima         = 0.4
	escalaMaxima         = 1.15
	botaoPainelLargura   = 26
	botaoPainelAltura    = 236
	botaoPainelLinha     = 28
	botaoPainelInset     = 4
	botaoImagem          = 20
	botaoImagemMargem    = 26
)

var rxVersion = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)\.(\d+)`)

const (
	miItem = iota
	miHeader
	miSep

	miZoom

	miSub

	miIA

	miCheck
	miRadio

	miExt
)

//exhaustruct:ignore // Item de menu: construtor parcial por desenho (campos opcionais com zero significativo).
type menuItem struct {
	kind      int
	label     string
	hint      string
	id        string
	sub       []menuItem
	iconeBase string
	check     bool

	ext string

	desc string
}

type menuWin struct {
	hwnd     uintptr
	items    []menuItem
	hover    int
	open     bool
	w, h     int32
	itemH    int32
	padY     int32
	headerH  int32
	sepY     int32
	contentH int32
	scrollY  int32

	zRow              rectT
	zIcon, zLabel     rectT
	zMinus, zPct      rectT
	zPlus, zSep, zFit rectT
	hoverZoom         int

	ignorarSoltura bool

	subPai  int
	sub2Pai int
	dx, dy  int32

	headerX bool

	hoverZona int
}

//nolint:gochecknoglobals // Menus unicos do app: estado de UI single-window.
var menu menuWin

//nolint:gochecknoglobals // Menus unicos do app: estado de UI single-window.
var menuSub menuWin

//nolint:gochecknoglobals // Menus unicos do app: estado de UI single-window.
var menuSub2 menuWin

//nolint:gochecknoglobals // Menus unicos do app: estado de UI single-window.
var panelMenu menuWin

const menuClassName = "OverdriveMenuWnd"

const menuSubClassName = "OverdriveMenuSubWnd"

const menuSub2ClassName = "OverdriveMenuSub2Wnd"

const panelMenuClassName = "OverdrivePanelMenuWnd"

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var menuProcPtr uintptr

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var menuSubProcPtr uintptr

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var menuSub2ProcPtr uintptr

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var calendarioProcPtr uintptr

//nolint:gochecknoglobals // Pontos de entrada Win32 registrados uma vez no registrarCallbacks.
var panelMenuProcPtr uintptr

type msgApp struct {
	T      string          `json:"t"`
	Pedido string          `json:"pedido"`
	Acao   string          `json:"acao"`
	Args   json.RawMessage `json:"args"`
}

const pedacoEnvio = 8000

//nolint:gochecknoglobals // Pagina inicial embutida: somente leitura.
var startPageHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="dark">
<title>Overdrive — Início</title>
<style>
:root{color-scheme:dark;--bg:#0b0b0b;--surface:#1c1c1c;--surface2:#232323;--border:#2f2f2f;
--border2:#3a3a3a;--text:#f2f2f2;--muted:#b0b0b0;--faint:#8a8a8a;--accent:#67e8f9;--accent2:#a5f3fc}
*{box-sizing:border-box}
html{min-height:100%;background:var(--bg);scrollbar-color:#3a3a3a #0b0b0b;scrollbar-width:thin}
body{margin:0;min-height:100vh;background:var(--bg);color:var(--text);
font:400 14px/1.55 "Segoe UI","Segoe UI Variable Text",system-ui,sans-serif}
body::before{content:"";position:fixed;inset:0;z-index:0;pointer-events:none;
  background-image:url("__ICON_BG__");background-repeat:no-repeat;
  background-position:center calc(50% + 40px);background-size:min(560px,72vmin);
  opacity:.055}
button{font:inherit}
a,button{-webkit-tap-highlight-color:transparent}
.page{width:100%;max-width:1180px;margin:0 auto;padding:30px 32px 24px;display:flex;min-height:100vh;
flex-direction:column;position:relative;z-index:1}
.topbar{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:46px}
.brand{display:flex;align-items:center;gap:11px;font-size:15px;font-weight:650;letter-spacing:-.25px}
.brand-mark{width:34px;height:34px;border-radius:10px;display:block}
.top-label{display:flex;align-items:center;gap:7px;color:var(--muted);font-size:12px}
.top-label::before{content:"";width:6px;height:6px;border-radius:50%;background:var(--accent)}
.hero{max-width:760px;margin:0 auto 40px;text-align:center}
.eyebrow{margin-bottom:12px;color:var(--accent2);font-size:11px;font-weight:650;letter-spacing:2.4px;
text-transform:uppercase}
h1{margin:0;font-size:clamp(28px,4.2vw,42px);font-weight:650;letter-spacing:-1.6px;line-height:1.14}
.subtitle{margin:12px 0 0;color:var(--muted);font-size:14px}
.tip{margin-top:14px;color:var(--faint);font-size:11px;line-height:2}
kbd{display:inline-block;padding:2px 6px;border:1px solid var(--border);border-bottom-color:var(--border2);
border-radius:5px;background:var(--surface);color:var(--muted);font:inherit;font-size:10px;font-weight:600}
.section{margin-top:30px}
.section-heading{display:flex;align-items:baseline;justify-content:space-between;gap:12px;
margin-bottom:12px;padding:0 2px}
h2{margin:0;font-size:13px;font-weight:650;letter-spacing:.1px}
.section-description{color:var(--faint);font-size:11px}
.shortcuts{display:grid;grid-template-columns:repeat(auto-fill,minmax(205px,1fr));gap:9px}
.shortcut{display:flex;align-items:center;gap:12px;min-width:0;min-height:72px;padding:12px 13px;
border:1px solid var(--border);border-radius:13px;background:var(--surface);color:var(--text);text-decoration:none}
.shortcut:hover{border-color:var(--accent);background:var(--surface2)}
.shortcut-icon{display:grid;flex:0 0 38px;width:38px;height:38px;place-items:center;
border:1px solid var(--border2);border-radius:11px;background:#151515;color:var(--muted);
font-family:"Segoe Fluent Icons","Segoe MDL2 Assets";font-size:16px;font-weight:400;letter-spacing:0}
.shortcut:hover .shortcut-icon{color:var(--accent2);border-color:var(--border2)}
.shortcut-text{min-width:0}
.shortcut-name{display:block;overflow:hidden;font-size:12.5px;font-weight:600;text-overflow:ellipsis;
white-space:nowrap}
.shortcut-detail{display:block;overflow:hidden;margin-top:2px;color:var(--faint);font-size:10.5px;
text-overflow:ellipsis;white-space:nowrap}
.bottom{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:auto;
padding-top:26px;color:var(--faint);font-size:10.5px}
.bottom strong{color:var(--muted);font-weight:600}
a:focus-visible,button:focus-visible{outline:2px solid var(--accent);outline-offset:3px}
@media(max-width:820px){.page{padding:24px 22px 20px}.topbar{margin-bottom:38px}
.shortcuts{grid-template-columns:repeat(auto-fill,minmax(180px,1fr))}}
@media(max-width:520px){.page{padding:20px 14px 16px}.topbar{margin-bottom:30px}.top-label{display:none}
.shortcuts{grid-template-columns:1fr 1fr;gap:7px}.shortcut{gap:9px;padding:9px;min-height:64px}
.shortcut-icon{flex-basis:32px;width:32px;height:32px;font-size:16px}.section-description{display:none}}
@media(max-width:380px){.shortcuts{grid-template-columns:1fr}}
@media(max-height:560px){.page{padding-top:18px}.topbar{margin-bottom:22px}.hero{margin-bottom:22px}
h1{font-size:30px}.subtitle{margin:8px 0 14px}.tip{margin-top:8px;line-height:1.8}.section{margin-top:18px}}
</style>
</head>
<body>
<div class="page">
<header class="topbar"><div class="brand"><img class="brand-mark" src="__ICON_BG__" alt="" aria-hidden="true">` +
	`<span>Overdrive</span></div><div class="top-label">Navegação leve e privada</div></header>
<main>
<section class="hero"><div class="eyebrow">Simples. Rápido. Seu.</div><h1>O que vamos explorar?</h1>` +
	`<p class="subtitle">Use a barra de endereço acima para pesquisar ou abrir um site.</p>
<div class="tip"><kbd>Ctrl</kbd> + <kbd>L</kbd> endereço <span aria-hidden="true">·</span> <kbd>Ctrl</kbd> + ` +
	`<kbd>T</kbd> nova aba ` +
	`<span aria-hidden="true">·</span> <kbd>Ctrl</kbd> + <kbd>Shift</kbd> + <kbd>H</kbd>` +
	` área de transferência</div></section>
<section class="section"><div class="section-heading"><h2>Inteligência artificial</h2>` +
	`<span class="section-description">Seus assistentes</span></div><div class="shortcuts">
<a class="shortcut" href="https://chat.deepseek.com/"><span class="shortcut-icon">&#xE99A;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">DeepSeek</span><span class="shortcut-detail">` +
	`Conversas e pesquisa</span></span></a>
<a class="shortcut" href="https://chatgpt.com/"><span class="shortcut-icon">&#xE8F2;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">ChatGPT</span><span class="shortcut-detail">` +
	`Assistente inteligente</span></span></a>
<a class="shortcut" href="https://gemini.google.com/"><span class="shortcut-icon">&#xE721;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">Gemini</span><span class="shortcut-detail">` +
	`IA do Google</span></span></a>
<a class="shortcut" href="https://chat.qwen.ai/"><span class="shortcut-icon">&#xE8F2;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">Qwen</span><span class="shortcut-detail">` +
	`Chat e pesquisa</span></span></a>
<a class="shortcut" href="https://www.kimi.com/"><span class="shortcut-icon">&#xE8F2;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">Kimi</span><span class="shortcut-detail">` +
	`Assistente Moonshot</span></span></a>
<a class="shortcut" href="https://chat.mistral.ai/"><span class="shortcut-icon">&#xE724;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">Mistral</span><span class="shortcut-detail">` +
	`Le Chat</span></span></a>
<a class="shortcut" href="https://duck.ai/"><span class="shortcut-icon">&#xE721;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">DuckDuckGo AI</span><span class="shortcut-detail">` +
	`Chat anônimo</span></span></a>
<a class="shortcut" href="https://www.google.com/ai"><span class="shortcut-icon">&#xE774;</span>` +
	`<span class="shortcut-text"><span class="shortcut-name">Google AI Mode</span><span class="shortcut-detail">` +
	`Pesquisa com IA</span></span></a>
</div></section>
</main>
<footer class="bottom"><span><strong>Overdrive</strong> · navegação sem distrações</span>` +
	`<span>Sem histórico · sem telemetria</span></footer>
</div>
</body>
</html>`

//nolint:gochecknoglobals // Servidor local unico + mutex: estado de processo single-window.
var (
	serverMu   sync.Mutex
	serverHTTP *http.Server
	serverPort int
)

//exhaustruct:ignore // Aba: construtor parcial por desenho (campos opcionais).
type tab struct {
	pane         *pane
	url          string
	titulo       string
	criando      bool
	criandoGen   int
	criandoDesde time.Time
	fixa         bool
	inativoDesde time.Time
}

//nolint:tagliatelle // Formato de sessao persistida em disco: renomear quebra sessoes salvas.
//exhaustruct:ignore // Estado persistido de aba: construtor parcial por desenho.
type tabState struct {
	URL    string `json:"URL"`
	Titulo string `json:"Titulo"`
	Indice int    `json:"Indice,omitempty"`
	Fixa   bool   `json:"Fixa,omitempty"`
}

const tipClassName = "OverdriveTipWnd"

//nolint:gochecknoglobals // Ponto de entrada Win32 registrado uma vez no registrarCallbacks.
var tipProcPtr uintptr

const (
	tradMaxTrechos = 2000
	tradMaxChars   = 150000
)

//nolint:gochecknoglobals // Traducao em voo: estado global do app single-window.
var traduzindo bool

//nolint:gochecknoglobals // DLLs e procs Win32 resolvidos uma vez: handles fixos do processo.
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	uxtheme  = windows.NewLazySystemDLL("uxtheme.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	psapi    = windows.NewLazySystemDLL("psapi.dll")

	pRegisterClassExW           = user32.NewProc("RegisterClassExW")
	pCreateWindowExW            = user32.NewProc("CreateWindowExW")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pDefWindowProcW             = user32.NewProc("DefWindowProcW")
	pDestroyWindow              = user32.NewProc("DestroyWindow")
	pPostQuitMessage            = user32.NewProc("PostQuitMessage")
	pGetMessageW                = user32.NewProc("GetMessageW")
	pPostMessageW               = user32.NewProc("PostMessageW")
	pTranslateMessage           = user32.NewProc("TranslateMessage")
	pDispatchMessageW           = user32.NewProc("DispatchMessageW")
	pSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW            = user32.NewProc("CallWindowProcW")
	pSetWindowPos               = user32.NewProc("SetWindowPos")
	pGetClientRect              = user32.NewProc("GetClientRect")
	pGetWindowRect              = user32.NewProc("GetWindowRect")
	pGetWindowPlacement         = user32.NewProc("GetWindowPlacement")
	pRegisterHotKey             = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	pAdjustWindowRectExForDpi   = user32.NewProc("AdjustWindowRectExForDpi")
	pInvalidateRect             = user32.NewProc("InvalidateRect")
	pValidateRect               = user32.NewProc("ValidateRect")
	pLoadCursorW                = user32.NewProc("LoadCursorW")
	pSetCursor                  = user32.NewProc("SetCursor")
	pShowWindow                 = user32.NewProc("ShowWindow")
	pUpdateWindow               = user32.NewProc("UpdateWindow")
	pSetFocus                   = user32.NewProc("SetFocus")
	pGetFocus                   = user32.NewProc("GetFocus")
	pSetCapture                 = user32.NewProc("SetCapture")
	pReleaseCapture             = user32.NewProc("ReleaseCapture")
	pSetWindowTextW             = user32.NewProc("SetWindowTextW")
	pGetWindowTextW             = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	pIsZoomed                   = user32.NewProc("IsZoomed")
	pIsChild                    = user32.NewProc("IsChild")
	pIsDialogMessageW           = user32.NewProc("IsDialogMessageW")
	pIsIconic                   = user32.NewProc("IsIconic")
	pIsWindowVisible            = user32.NewProc("IsWindowVisible")

	pTrackMouseEvent = user32.NewProc("TrackMouseEvent")

	pOpenClipboard                 = user32.NewProc("OpenClipboard")
	pEmptyClipboard                = user32.NewProc("EmptyClipboard")
	pSetClipboardData              = user32.NewProc("SetClipboardData")
	pGetClipboardData              = user32.NewProc("GetClipboardData")
	pCloseClipboard                = user32.NewProc("CloseClipboard")
	pGlobalAlloc                   = kernel32.NewProc("GlobalAlloc")
	pGlobalLock                    = kernel32.NewProc("GlobalLock")
	pGlobalUnlock                  = kernel32.NewProc("GlobalUnlock")
	pGlobalFree                    = kernel32.NewProc("GlobalFree")
	pGlobalSize                    = kernel32.NewProc("GlobalSize")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pGetKeyState                   = user32.NewProc("GetKeyState")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pScreenToClient                = user32.NewProc("ScreenToClient")
	pClientToScreen                = user32.NewProc("ClientToScreen")
	pGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	pSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	pMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	pMonitorFromPoint              = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pEnumChildWindows              = user32.NewProc("EnumChildWindows")
	pLoadImageW                    = user32.NewProc("LoadImageW")

	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pGetDIBits              = gdi32.NewProc("GetDIBits")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pSetBkColor             = gdi32.NewProc("SetBkColor")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetTimer               = user32.NewProc("SetTimer")
	pKillTimer              = user32.NewProc("KillTimer")

	pSetWindowTheme        = uxtheme.NewProc("SetWindowTheme")
	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	pShellExecuteW         = shell32.NewProc("ShellExecuteW")
	pSHGetKnownFolderPath  = shell32.NewProc("SHGetKnownFolderPath")
	pGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")

	pEmptyWorkingSet          = psapi.NewProc("EmptyWorkingSet")
	pCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	pOutputDebugString        = kernel32.NewProc("OutputDebugStringW")
	pProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	pProcess32NextW           = kernel32.NewProc("Process32NextW")
	pOpenProcess              = kernel32.NewProc("OpenProcess")
	pCloseHandle              = kernel32.NewProc("CloseHandle")
	pSetProcessWorkingSetSize = kernel32.NewProc("SetProcessWorkingSetSize")
	pSetProcessInformation    = kernel32.NewProc("SetProcessInformation")
	pGetCurrentProcessID      = kernel32.NewProc("GetCurrentProcessId")
)

const (
	wsOverlappedWindow = 0x00CF0000

	wsCaption     = 0x00C00000
	wsThickFrame  = 0x00040000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000
	wsSysMenu     = 0x00080000
	wsJanelaApp   = wsOverlappedWindow&^wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox |
		wsSysMenu | wsClipChildren
	wsClipChildren = 0x02000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsTabStop      = 0x00010000

	esAutoHScroll = 0x0080
	esLeft        = 0x0000

	swShow           = 5
	swShowNoActivate = 4
	swMaximize       = 3
	swHide           = 0
	swRestore        = 9
	swMinimize       = 6

	wsPopup         = 0x80000000
	wsExToolWindow  = 0x00000080
	wsExTopmost     = 0x00000008
	wsExNoActivate  = 0x08000000
	wsExTransparent = 0x00000020
	wsExLayered     = 0x00080000
	lwaAlpha        = 0x00000002
	alfaJanela      = 205
	hwndTopmost     = ^uintptr(0)
	swpShowWindow   = 0x0040

	waInactive = 0

	tmeLeave = 0x00000002

	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	timerTip       = 2
	timerRetryChat = 8
	timerStatus    = 10
	timerAviso     = 11
	timerOcioso    = 12
	timerChatFrio  = 13
	timerChatIdle  = 14
	timerHover     = 15
	timerSubDelay  = 16
	timerAvisoAnim = 17

	biRGB = 0

	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmSetFocus        = 0x0007
	wmKillFocus       = 0x0008
	wmPaint           = 0x000F
	wmClose           = 0x0010
	wmEraseBkgnd      = 0x0014
	wmSetCursor       = 0x0020
	wmGetMinMaxInfo   = 0x0024
	wmCommand         = 0x0111
	wmTimer           = 0x0113
	wmAppTarefa       = 0x8001
	wmKeyDown         = 0x0100
	wmHotKey          = 0x0312
	wmSysKeyDown      = 0x0104
	wmChar            = 0x0102
	wmCut             = 0x0300
	wmPaste           = 0x0302
	wmClear           = 0x0303
	wmUndo            = 0x0304
	wmLButtonDown     = 0x0201
	wmLButtonUp       = 0x0202
	wmLButtonDblClk   = 0x0203
	wmRButtonUp       = 0x0205
	wmMButtonUp       = 0x0208
	wmMouseMove       = 0x0200
	wmMouseLeave      = 0x02A3
	wmCtlColorEdit    = 0x0133
	wmCtlColorStatic  = 0x0138
	wmDpiChanged      = 0x02E0
	wmNcLButtonDown   = 0x00A1
	wmNcLButtonDblClk = 0x00A3
	wmNcHitTest       = 0x0084
	wmActivate        = 0x0006

	htCaption = 2
	htClient  = 1

	wmNcCalcSize = 0x0083

	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	winNenhum   = 0
	winMinimiza = 1
	winMaximiza = 2
	winFecha    = 3

	gwlpWndProc  = ^uintptr(3)
	gwlpUserData = ^uintptr(20)

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpHideWindow = 0x0080
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001

	idcArrow  = 32512
	idcCross  = 32515
	idcSizeWE = 32644
	idcSizeNS = 32645
	idcHand   = 32649

	dtLeft       = 0x00000000
	dtCenter     = 0x00000001
	dtRight      = 0x00000002
	dtVCenter    = 0x00000004
	dtSingleLine = 0x00000020
	dtCalcRect   = 0x00000400
	dtNoPrefix   = 0x00000800

	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

	imageIcon      = 1
	lrLoadFromFile = 0x0010
	lrDefaultSize  = 0x0040

	defaultCharset     = 1
	outTtPrecis        = 4
	clipDefaultPrecis  = 0
	cleartypeQuality   = 5
	antialiasedQuality = 4
	defaultPitch       = 34
	fwNormal           = 400

	transparent = 1
	opaque      = 2

	dwmwaUseImmersiveDarkMode = 20
	dwmwaWindowCorner         = 33
	dwmwaBorderColor          = 34
	dwmwaCaptionColor         = 35

	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

	spiGetWorkArea = 0x0030

	vkTab      = 0x09
	vkSpace    = 0x20
	vkReturn   = 0x0D
	vkEscape   = 0x1B
	vkPrior    = 0x21
	vkNext     = 0x22
	vkHome     = 0x23
	vkEnd      = 0x24
	vkUp       = 0x26
	vkDown     = 0x28
	vkLeft     = 0x25
	vkRight    = 0x27
	vkF5       = 0x74
	vkF6       = 0x75
	vkF11      = 0x7A
	vkF12      = 0x7B
	vkOEMPlus  = 0xBB
	vkOEMMinus = 0xBD
	vk0        = 0x30
	vkA        = 0x41
	vkB        = 0x42
	vkC        = 0x43
	vkG        = 0x47
	vkH        = 0x48
	vkL        = 0x4C
	vkP        = 0x50
	vkS        = 0x53
	vkF        = 0x46
	vkF3       = 0x72
	vkDelete   = 0x2E
	vkJ        = 0x4A
	vkT        = 0x54
	vkV        = 0x56
	vkW        = 0x57
	vkX        = 0x58
	vkY        = 0x59
	vkZ        = 0x5A
	vkN        = 0x4E
	vkR        = 0x52
	vkU        = 0x55
	vkAdd      = 0x6B
	vkSubtract = 0x6D
	vkControl  = 0x11
	vkShift    = 0x10
	vkMenu     = 0x12
)

//exhaustruct:ignore // Espelho da API Win32: campos nao usados ficam zero.
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msgT struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

//exhaustruct:ignore // Retangulo interno: construtor parcial por desenho.
type rectT struct {
	Left, Top, Right, Bottom int32
}

type monitorInfoT struct {
	cbSize    uint32
	rcMonitor rectT
	rcWork    rectT
	dwFlags   uint32
}

const monitorPadraoMaisProximo = 2

func areaTrabalhoJanela(hwnd uintptr) rectT {
	var infoMonitor monitorInfoT

	infoMonitor.cbSize = uint32(unsafe.Sizeof(infoMonitor))

	handle, _, _ := pMonitorFromWindow.Call(hwnd, monitorPadraoMaisProximo)
	if handle != 0 {
		if ret, _, _ := pGetMonitorInfoW.Call(handle, uintptr(unsafe.Pointer(&infoMonitor))); ret != 0 {
			return infoMonitor.rcWork
		}
	}

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	return area
}

func areaTrabalhoPonto(x, y int32) rectT {
	var infoMonitor monitorInfoT

	infoMonitor.cbSize = uint32(unsafe.Sizeof(infoMonitor))
	ponto := struct{ x, y int32 }{x, y}

	handle, _, _ := pMonitorFromPoint.Call(uintptr(unsafe.Pointer(&ponto)), monitorPadraoMaisProximo)
	if handle != 0 {
		if ret, _, _ := pGetMonitorInfoW.Call(handle, uintptr(unsafe.Pointer(&infoMonitor))); ret != 0 {
			return infoMonitor.rcWork
		}
	}

	return areaTrabalhoJanela(0)
}

type windowPlacementT struct {
	length           uint32
	flags            uint32
	showCmd          uint32
	ptMinPosition    struct{ x, y int32 }
	ptMaxPosition    struct{ x, y int32 }
	rcNormalPosition rectT
}

type minMaxInfo struct {
	ptReserved     struct{ x, y int32 }
	ptMaxSize      struct{ x, y int32 }
	ptMaxPosition  struct{ x, y int32 }
	ptMinTrackSize struct{ x, y int32 }
	ptMaxTrackSize struct{ x, y int32 }
}

type ncCalcSizeParams struct {
	rgrc  [3]rectT
	lppos uintptr
}

type processEntry32W struct {
	Size                uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

//exhaustruct:ignore // Espelho da API Win32 (BITMAPINFOHEADER): campos nao usados ficam zero.
type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

//exhaustruct:ignore // Espelho da API Win32 (BITMAPINFO): campos nao usados ficam zero.
type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
}

//exhaustruct:ignore // Espelho da API Win32 (TRACKMOUSEEVENT): hover usa padrao do sistema.
type trackMouseEventT struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
}

const ytBase = "https://www.youtube.com/youtubei/v1/"

const (
	ytWebUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/131.0.0.0 Safari/537.36"
)

//nolint:gochecknoglobals // Cliente YouTube dedicado: timeout fixo, somente leitura apos init.
//exhaustruct:ignore // http.Client: so timeout + redirect; transporte/Jar padrao do Go.
var ytHTTP = &http.Client{Timeout: tempoClienteYt, CheckRedirect: redirectPublico}

//exhaustruct:ignore // Item do YouTube: construtor parcial por desenho.
type ytItem struct {
	ID          string `json:"id"`
	Titulo      string `json:"titulo"`
	Canal       string `json:"canal,omitempty"`
	CanalID     string `json:"canalId,omitempty"`
	CanalAvatar string `json:"canalAvatar,omitempty"`
	Views       string `json:"views,omitempty"`
	Publicado   string `json:"publicado,omitempty"`
	Duracao     string `json:"duracao,omitempty"`
	Descricao   string `json:"descricao,omitempty"`
	AoVivo      bool   `json:"aoVivo,omitempty"`
}

//exhaustruct:ignore // Video do YouTube: construtor parcial por desenho.
type ytVideo struct {
	ID           string      `json:"id"`
	Titulo       string      `json:"titulo"`
	Canal        string      `json:"canal"`
	CanalID      string      `json:"canalId"`
	CanalAvatar  string      `json:"canalAvatar,omitempty"`
	Inscritos    string      `json:"inscritos"`
	Views        string      `json:"views"`
	Data         string      `json:"data"`
	Likes        string      `json:"likes"`
	Deslikes     string      `json:"deslikes"`
	Descricao    string      `json:"descricao"`
	Relacionados []ytItem    `json:"relacionados"`
	Patrocinio   []ytTrecho  `json:"patrocinio"`
	Legendas     []ytLegenda `json:"legendas,omitempty"`
	Duracao      float64     `json:"duracao,omitempty"`
	Erro         string      `json:"erro,omitempty"`
}

type ytTrecho struct {
	Inicio    float64 `json:"inicio"`
	Fim       float64 `json:"fim"`
	Categoria string  `json:"categoria"`
}

//exhaustruct:ignore // Comentario do YouTube: construtor parcial por desenho.
type ytComentario struct {
	Autor     string `json:"autor"`
	Texto     string `json:"texto"`
	Quando    string `json:"quando"`
	Likes     string `json:"likes"`
	Respostas int    `json:"respostas"`
	Token     string `json:"token,omitempty"`
	Fixo      bool   `json:"fixo,omitempty"`
	Coracao   bool   `json:"coracao,omitempty"`
	Iniciais  string `json:"-"`
	Cor       int    `json:"-"`
	VideoID   string `json:"-"`
}

var tagComentario = regexp.MustCompile(`(?s)</?[a-zA-Z][^>]*>`)

//nolint:gochecknoglobals // Espelhos Piped: lista fixa, somente leitura.
var pipedInstancias = []string{
	"https://api.piped.private.coffee",
	"https://pipedapi.ducks.party",
}

type estadoYouTube int

const (
	ytAbrirCheio estadoYouTube = iota
	ytOcultar
	ytMostrar
)

const (
	SOK          uintptr = 0
	ENoInterface uintptr = 0x80004002
	EInvalidArg  uintptr = 0x80070057
	EFail        uintptr = 0x80004005
)

//nolint:gochecknoglobals // DLLs COM resolvidas uma vez: handles fixos do processo.
var (
	ole32              = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoTaskMemFree  = ole32.NewProc("CoTaskMemFree")
	procCoTaskMemAlloc = ole32.NewProc("CoTaskMemAlloc")
	shlwapi            = windows.NewLazySystemDLL("shlwapi.dll")
	pSHCreateMemStream = shlwapi.NewProc("SHCreateMemStream")
)

const coinitApartmentThreaded = 2

// refsOffset desloca do inicio do objeto COM (vtbl em zero) ate o refCounted.
// Cada handler COM declara vtbl primeiro e refCounted depois, senao o loader
// le lixo como vtbl e faulta no primeiro AddRef.
const refsOffset = unsafe.Sizeof(uintptr(0))

//nolint:gochecknoglobals // Gancho de panico instalavel por teste: nil no app real.
var PanicHook func(string)

//nolint:gochecknoglobals // Gancho de trace instalavel por teste: nil no app real.
var TraceHook func(string)

//exhaustruct:ignore // Retangulo: construtor parcial por desenho.
type Rect struct {
	Left, Top, Right, Bottom int32
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var envCompletedVtbl = ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(envCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var controllerCompletedVtbl = ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(controllerCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var findStartVtbl = ICoreWebView2FindStartCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(findStartInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var addExtCompletedVtbl = ICoreWebView2ProfileAddBrowserExtensionCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(addExtCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var getExtCompletedVtbl = ICoreWebView2ProfileGetBrowserExtensionsCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(getExtCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var removeExtCompletedVtbl = ICoreWebView2BrowserExtensionRemoveCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(removeExtCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var enableExtCompletedVtbl = ICoreWebView2BrowserExtensionEnableCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(enableExtCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var cdpCompletedVtbl = ICoreWebView2CallDevToolsProtocolMethodCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(cdpCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var srcChangedVtbl = ICoreWebView2SourceChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(srcChangedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var navStartingVtbl = ICoreWebView2NavigationStartingEventHandlerVtbl{
	unkTriple(),
	NewComProc(navStartingInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var navCompletedVtbl = ICoreWebView2NavigationCompletedEventHandlerVtbl{
	unkTriple(),
	NewComProc(navCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var titleChangedVtbl = ICoreWebView2DocumentTitleChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(titleChangedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var newWindowVtbl = ICoreWebView2NewWindowRequestedEventHandlerVtbl{
	unkTriple(),
	NewComProc(newWindowInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var webMessageVtbl = ICoreWebView2WebMessageReceivedEventHandlerVtbl{
	unkTriple(),
	NewComProc(webMessageInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var accelKeyVtbl = ICoreWebView2AcceleratorKeyPressedEventHandlerVtbl{
	unkTriple(),
	NewComProc(accelKeyInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var clearBrowsingDataVtbl = ICoreWebView2ClearBrowsingDataCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(clearBrowsingDataInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var webResourceRequestedVtbl = ICoreWebView2WebResourceRequestedEventHandlerVtbl{
	unkTriple(),
	NewComProc(webResourceRequestedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var contextMenuRequestedVtbl = ICoreWebView2ContextMenuRequestedEventHandlerVtbl{
	unkTriple(),
	NewComProc(contextMenuRequestedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var customItemSelectedVtbl = ICoreWebView2CustomItemSelectedEventHandlerVtbl{
	unkTriple(),
	NewComProc(customItemSelectedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var downloadStartingVtbl = ICoreWebView2DownloadStartingEventHandlerVtbl{
	unkTriple(),
	NewComProc(downloadStartingInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var downloadBytesVtbl = ICoreWebView2BytesReceivedChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(downloadBytesInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var downloadStateVtbl = ICoreWebView2StateChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(downloadStateInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var isMutedChangedVtbl = ICoreWebView2IsMutedChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(isMutedChangedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var isPlayingAudioChangedVtbl = ICoreWebView2IsDocumentPlayingAudioChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(isPlayingAudioChangedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var permissionRequestedVtbl = ICoreWebView2PermissionRequestedEventHandlerVtbl{
	unkTriple(),
	NewComProc(permissionRequestedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var processFailedVtbl = ICoreWebView2ProcessFailedEventHandlerVtbl{
	unkTriple(),
	NewComProc(processFailedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var gotFocusVtbl = ICoreWebView2FocusChangedEventHandlerVtbl{
	unkTriple(),
	NewComProc(gotFocusInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var evalCompletedVtbl = ICoreWebView2ExecuteScriptCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(evalCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var trySuspendVtbl = ICoreWebView2TrySuspendCompletedHandlerVtbl{
	unkTriple(),
	NewComProc(trySuspendCompletedInvoke),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var saveAsUICVtbl = ICoreWebView2ShowSaveAsUICompletedHandlerVtbl{
	unkTriple(),
	NewComProc(saveAsUICompletedInvoke),
}

//nolint:gochecknoglobals // Fixacao de callbacks COM + mutex: estado de processo.
var (
	pinnedMu sync.Mutex
	pinned   = map[uintptr]any{}
)

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var optBaseVtbl = ICoreWebView2EnvironmentOptionsVtbl{
	optUnkTriple(),
	NewComProc(optGetArgs), NewComProc(optPutArgs),
	NewComProc(optGetLang), NewComProc(optPutLang),
	NewComProc(optGetTarget), NewComProc(optPutTarget),
	NewComProc(optGetSSO), NewComProc(optPutSSO),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var opt6Vtbl = ICoreWebView2EnvironmentOptions6Vtbl{
	optUnkTriple(),
	NewComProc(opt6GetExt), NewComProc(opt6PutExt),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var schemeRegVtbl = ICoreWebView2CustomSchemeRegistrationVtbl{
	unkTriple(),
	NewComProc(schemeGetName),
	NewComProc(schemeGetSecure), NewComProc(schemePutSecure),
	NewComProc(schemeGetOrigins), NewComProc(schemeSetOrigins),
	NewComProc(schemeGetAuthority), NewComProc(schemePutAuthority),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var opt4Vtbl = ICoreWebView2EnvironmentOptions4Vtbl{
	optUnkTriple(),
	NewComProc(opt4GetSchemes), NewComProc(opt4SetSchemes),
}

//nolint:gochecknoglobals // Tabela COM fixa: constante de processo.
var opt5Vtbl = ICoreWebView2EnvironmentOptions5Vtbl{
	optUnkTriple(),
	NewComProc(opt5GetTracking), NewComProc(opt5PutTracking),
}

//exhaustruct:ignore // Token de evento: valor fixado no registro logo apos; zero intermediario nao escapa.
type eventToken struct{ value int64 }

const (
	MoveFocusProgrammatic = 0

	ColorSchemeDefault = 0
	ColorSchemeDark    = 2

	BoundsModeRawPixels = 0
)

type physicalKeyStatus struct {
	RepeatCount   uint32
	ScanCode      uint32
	IsExtendedKey int32
	IsMenuKeyDown int32
	WasKeyDown    int32
	IsKeyReleased int32
}

//nolint:gochecknoglobals // DLLs Win32 resolvidas uma vez: handles fixos do processo.
var (
	user32wv2            = windows.NewLazySystemDLL("user32.dll")
	procGetAsyncKeyState = user32wv2.NewProc("GetAsyncKeyState")
)

// registrarCallbacks fixa os pontos de entrada Win32 antes de qualquer janela
// existir. Ficava em init(); virou chamada explicita no topo do main.
func registrarCallbacks() {
	wndProcPtr = windows.NewCallback(wndProc)
	editProcPtr = windows.NewCallback(editProc)
	avisoProcPtr = windows.NewCallback(avisoProc)
	menuProcPtr = windows.NewCallback(menuProc)
	menuSubProcPtr = windows.NewCallback(menuSubProc)
	menuSub2ProcPtr = windows.NewCallback(menuSub2Proc)
	calendarioProcPtr = windows.NewCallback(calendarioProc)
	panelMenuProcPtr = windows.NewCallback(panelMenuProc)
	recorteProcPtr = windows.NewCallback(recorteProc)
	clipProcPtr = windows.NewCallback(clipProc)
	cfgProcPtr = windows.NewCallback(cfgProc)
	tipProcPtr = windows.NewCallback(tipProc)

	registrarProcFloat()
}

func montarPaginaInicial() {
	dados := "data:image/png;base64," + base64.StdEncoding.EncodeToString(homeBgPNG)
	startPageHTML = strings.ReplaceAll(startPageHTML, "__ICON_BG__", dados)
}

func main() {
	runtime.LockOSThread()

	registrarCallbacks()
	montarPaginaInicial()
	espIniciar()

	defer func() {
		if r := recover(); r != nil {
			msg := utf16(fmt.Sprintf(txtFmtErroFatal, fmt.Sprint(r)))
			pMessageBoxW.Call(0, uintptr(unsafe.Pointer(msg)),
				uintptr(unsafe.Pointer(utf16(appTitle))), iconeErroWin)
			os.Exit(codigoSaidaFatal)
		}
	}()

	exe, _ := os.Executable()
	exeDir = filepath.Dir(exe)

	prepararDiretoriosOverdrive()

	if !carregarArranqueOverdrive() {
		return
	}

	perfilar := perfilarCPUOverdrive()
	defer perfilar()

	limparPrivadosAntigos()
	time.AfterFunc(esperaPrivadosInicial, limparPrivadosAntigos)
	time.AfterFunc(esperaPrivadosPeriodo, limparPrivadosAntigos)

	_ = os.RemoveAll(filepath.Join(dataDir, "sandbox"))

	pSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)

	_ = InitCOM()

	interpretarArgumentos()

	//exhaustruct:ignore // Instancia unica do app: 4 campos iniciais, resto zero intencional (frio).
	aplicativo := &app{
		focus: idLeft, status: "iniciando...",
		permissoes: map[string]map[uint32]bool{}, downloadOps: map[uintptr]*downloadAtivo{},
	}
	theApp = aplicativo

	aplicativo.vpn = newVPNManager()
	defer func() {
		if aplicativo.vpn != nil {
			aplicativo.vpn.Close()
		}
	}()

	aplicativo.st = loadState()

	espFecharPaineisInicio(aplicativo)

	aplicativo.restaurarGuiasFechadas()

	aplicativo.carregarRegistroDownloads()
	aplicativo.abrirJanelaPrincipal()
}

// abrirJanelaPrincipal cria os panes, a janela e o ambiente WebView.
func (a *app) abrirJanelaPrincipal() {
	a.left = &pane{zoom: a.st.Zoom}

	a.right = &pane{zoom: a.st.Zoom, mobile: true}
	if err := a.createWindow(); err != nil {
		return
	}

	if _, err := os.Stat(statePath()); os.IsNotExist(err) {
		a.dizerf(txtBoasVindas)
	}

	a.startEnvironment()
	a.agendarLimpezaMemoria(false, esperaLimpezaLeveInicio)
	a.agendarLimpezaMemoria(true, esperaLimpezaPesadaIni)

	bombearMensagens()
}

// prepararDiretoriosOverdrive resolve as pastas do executavel.
func prepararDiretoriosOverdrive() {
	dataDir = filepath.Join(exeDir, "data")
	runtimeDir = filepath.Join(exeDir, "runtime")
	extDir = filepath.Join(exeDir, "extensions")
	_ = os.MkdirAll(dataDir, 0o700)
}

// carregarArranqueOverdrive carrega config e login; false = sair.
func carregarArranqueOverdrive() bool {
	configCaminho := filepath.Join(exeDir, "config", "overdrive.json")
	if err := carregarConfigOverdrive(configCaminho); err != nil {
		texto := utf16(fmt.Sprintf(txtFmtFalhaBoot, err.Error()))
		titulo := utf16(txtTituloConfig)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(texto)), uintptr(unsafe.Pointer(titulo)), iconeErroWin)

		return false
	}

	carregarLogin()

	return true
}

// perfilarCPUOverdrive liga o perfil PGO quando pedido via ambiente.
func perfilarCPUOverdrive() func() {
	nada := func() {}
	// PGO: com OVERDRIVE_CPU_PROFILE=<dir>, grava cpu-<pid>.pprof da sessao.
	// Renomeie o melhor para src/default.pprof e compile com -pgo=auto.
	if dirPerfil := strings.TrimSpace(os.Getenv("OVERDRIVE_CPU_PROFILE")); dirPerfil != "" {
		// GOD: perfil PGO opt-in via env do operador (documentado); diretorio do proprio operador.
		pprofCaminho := filepath.Join(dirPerfil, fmt.Sprintf("cpu-%d.pprof", os.Getpid()))
		if arquivo, err := os.Create(pprofCaminho); err == nil { // #nosec G703, G304
			_ = pprof.StartCPUProfile(arquivo)

			return func() {
				pprof.StopCPUProfile()

				_ = arquivo.Close()
			}
		}
	}

	return nada
}

// interpretarArgumentos aplica --perfil e a bandeira de privado.
func interpretarArgumentos() {
	args := os.Args[1:]
	for indice := 0; indice < len(args); indice++ {
		if args[indice] == "--perfil" && indice+1 < len(args) {
			profileOverride = "profile-" + args[indice+1]
			indice++
		}

		if args[indice] == flagPrivado {
			profileOverride = "privado-" + strconv.Itoa(os.Getpid()) + "-" + time.Now().Format("20060102-150405")
			perfilPrivado = true
		}
	}
}

// restaurarGuiasFechadas recupera ate 10 guias da sessao anterior.
func (a *app) restaurarGuiasFechadas() {
	for _, guia := range a.st.GuiasFechadas {
		if strings.TrimSpace(guia.URL) == "" {
			continue
		}

		if len(a.guiasFechadas) >= maxGuiasFechadas {
			break
		}

		a.guiasFechadas = append(a.guiasFechadas, guia)
	}
}

// bombearMensagens executa o loop de mensagens do Windows.
func bombearMensagens() {
	var msg msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}

		if repassarDialogoConfig(msg) {
			continue
		}

		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// repassarDialogoConfig entrega a mensagem ao dialogo de config; true = consumida.
func repassarDialogoConfig(msg msgT) bool {
	if cfgUI.ativo && cfgUI.hwnd != 0 {
		if ch, _, _ := pIsChild.Call(cfgUI.hwnd, msg.hwnd); ch != 0 || msg.hwnd == cfgUI.hwnd {
			if dlg, _, _ := pIsDialogMessageW.Call(cfgUI.hwnd, uintptr(unsafe.Pointer(&msg))); dlg != 0 {
				return true
			}
		}
	}

	return false
}
