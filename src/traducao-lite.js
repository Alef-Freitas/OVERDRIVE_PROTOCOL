'use strict';

// traducao-lite.ts
(() => {
  if (window.__odTrad) {
    return;
  }
  const PULAR = { SCRIPT: 1, STYLE: 1, NOSCRIPT: 1, PRE: 1, CODE: 1, KBD: 1, SAMP: 1, TEXTAREA: 1, SVG: 1, CANVAS: 1, IFRAME: 1, SELECT: 1, OPTION: 1 };
  let mapa = window.__odTradMapa || {};
  const originais = new WeakMap;
  const attrsOrig = new WeakMap;
  const ativos = new WeakSet;
  let observador = null;
  let observadorAte = 0;
  let novos = [];
  let novosSet = Object.create(null);
  let pendente = 0;
  let alvoAtual = "";
  let aplicando = false;
  function limpo(s) {
    return String(s == null ? "" : s).replace(/\s+/g, " ").trim();
  }
  function util(t) {
    if (!t || t.length < 2) {
      return false;
    }
    if (!/[A-Za-zÀ-ɏЀ-ӿͰ-Ͽ؀-ۿぁ-ヿ一-鿿]/.test(t)) {
      return false;
    }
    if (/^https?:\/\//i.test(t) || /^[\w.+-]+@[\w.-]+$/.test(t)) {
      return false;
    }
    if (/^[\d\s.,%$€R$-]+$/.test(t)) {
      return false;
    }
    return true;
  }
  function ignorado(el) {
    if (!el) {
      return true;
    }
    if (PULAR[el.tagName]) {
      return true;
    }
    try {
      if (el.getAttribute && el.getAttribute("translate") === "no") {
        return true;
      }
      if (el.closest && el.closest('[translate="no"],.notranslate')) {
        return true;
      }
    } catch (e) {}
    return false;
  }
  function visivel(el) {
    try {
      if (!el || typeof el.getBoundingClientRect !== "function") {
        return true;
      }
      const r = el.getBoundingClientRect();
      if (r.width < 2 && r.height < 2) {
        return false;
      }
    } catch (e) {}
    return true;
  }
  function coletar(limite, limiteChars) {
    const lim = limite || 2000;
    const limChars = limiteChars || 120000;
    const vistos = Object.create(null);
    const itens = [];
    let total = 0;
    try {
      const todos = document.body ? document.body.querySelectorAll("*") : [];
      for (let i = 0;i < todos.length && itens.length < lim; i++) {
        const el = todos[i];
        if (ignorado(el) || !visivel(el)) {
          continue;
        }
        for (let k = 0;k < el.childNodes.length; k++) {
          const no = el.childNodes[k];
          if (no.nodeType !== 3) {
            continue;
          }
          const t = limpo(no.nodeValue);
          if (!util(t)) {
            continue;
          }
          if (mapa[t] !== undefined) {
            continue;
          }
          if (vistos[t]) {
            continue;
          }
          if (itens.length >= lim || total + t.length > limChars) {
            break;
          }
          vistos[t] = 1;
          itens.push(t);
          total += t.length;
        }
        if (el.getAttribute) {
          const ph = limpo(el.getAttribute("placeholder"));
          if (util(ph) && mapa[ph] === undefined && !vistos[ph]) {
            vistos[ph] = 1;
            itens.push(ph);
            total += ph.length;
          }
          const ti = limpo(el.getAttribute("title"));
          if (util(ti) && mapa[ti] === undefined && !vistos[ti]) {
            vistos[ti] = 1;
            itens.push(ti);
            total += ti.length;
          }
        }
      }
    } catch (e) {
      return JSON.stringify({ ok: false, err: String(e) });
    }
    return JSON.stringify({ ok: true, itens, lang: String(document.documentElement.lang || ""), total });
  }
  function aplicarAtributos(el, alvo) {
    try {
      const ph = limpo(el.getAttribute("placeholder"));
      if (ph && mapa[ph] !== undefined) {
        const o = attrsOrig.get(el) || {};
        if (o.placeholder === undefined) {
          o.placeholder = el.getAttribute("placeholder");
        }
        attrsOrig.set(el, o);
        if (el.getAttribute("placeholder") !== mapa[ph]) {
          el.setAttribute("placeholder", mapa[ph]);
        }
      }
      const ti = limpo(el.getAttribute("title"));
      if (ti && mapa[ti] !== undefined) {
        const o2 = attrsOrig.get(el) || {};
        if (o2.title === undefined) {
          o2.title = el.getAttribute("title");
        }
        attrsOrig.set(el, o2);
        if (el.getAttribute("title") !== mapa[ti]) {
          el.setAttribute("title", mapa[ti]);
        }
      }
    } catch (e) {}
  }
  function aplicarNo(no) {
    const bruto = no.nodeValue;
    const t = limpo(bruto);
    if (!t || mapa[t] === undefined) {
      return 0;
    }
    const trad = mapa[t];
    if (!trad || bruto === trad) {
      return 0;
    }
    const b = bruto;
    if (!originais.has(no)) {
      originais.set(no, b);
    }
    const iniM = b.match(/^\s*/);
    const fimM = b.match(/\s*$/);
    const ini = iniM ? iniM[0] : "";
    const fim = fimM ? fimM[0] : "";
    const novoValor = ini + trad + fim;
    if (no.nodeValue !== novoValor) {
      no.nodeValue = novoValor;
      return 1;
    }
    return 0;
  }
  function olhar(raiz) {
    let n = 0;
    try {
      const alvo = raiz && raiz.nodeType === 1 ? raiz : raiz && raiz.parentElement || document.body;
      if (!alvo) {
        return 0;
      }
      if (alvo.nodeType === 3) {
        return aplicarNo(alvo);
      }
      if (ignorado(alvo)) {
        return 0;
      }
      if (alvo.nodeType === 1 && alvo.childNodes) {
        for (let i = 0;i < alvo.childNodes.length; i++) {
          const c = alvo.childNodes[i];
          if (c.nodeType === 3) {
            n += aplicarNo(c);
          }
        }
        const ael = alvo;
        aplicarAtributos(ael, alvoAtual);
      }
      const alvoEl = alvo;
      const lista = alvoEl.querySelectorAll("*");
      for (let j = 0;j < lista.length; j++) {
        const el = lista[j];
        if (ignorado(el)) {
          continue;
        }
        if (el.childNodes) {
          for (let k = 0;k < el.childNodes.length; k++) {
            const no = el.childNodes[k];
            if (no.nodeType === 3) {
              n += aplicarNo(no);
            }
          }
        }
        aplicarAtributos(el, alvoAtual);
      }
    } catch (e) {}
    return n;
  }
  function anotarNovos(no) {
    try {
      if (!no || no.nodeType !== 3) {
        return;
      }
      const t = limpo(no.nodeValue);
      if (!util(t) || mapa[t] !== undefined || novosSet[t]) {
        return;
      }
      const pai = no.parentElement;
      if (!pai || ignorado(pai)) {
        return;
      }
      novosSet[t] = 1;
      novos.push(t);
      if (novos.length > 400) {
        novos.length = 400;
      }
    } catch (e) {}
  }
  function ligarObservador() {
    try {
      if (observador) {
        observadorAte = Date.now() + 120000;
        return;
      }
      observador = new MutationObserver(function(muts) {
        if (Date.now() > observadorAte) {
          desligarObservador();
          return;
        }
        if (pendente) {
          return;
        }
        pendente = window.setTimeout(function() {
          pendente = 0;
          try {
            for (let i = 0;i < muts.length && i < 200; i++) {
              const m = muts[i];
              if (m.type === "characterData") {
                aplicarNo(m.target);
                continue;
              }
              for (let a = 0;a < m.addedNodes.length; a++) {
                const nd = m.addedNodes[a];
                if (nd.nodeType === 3) {
                  anotarNovos(nd);
                }
                if (nd.nodeType === 1) {
                  olhar(nd);
                  const ndel = nd;
                  const txts = ndel.querySelectorAll("*");
                  for (let t2 = 0;t2 < txts.length; t2++) {
                    for (let c2 = 0;c2 < txts[t2].childNodes.length; c2++) {
                      if (txts[t2].childNodes[c2].nodeType === 3) {
                        anotarNovos(txts[t2].childNodes[c2]);
                      }
                    }
                  }
                }
              }
            }
          } catch (e) {}
        }, 180);
      });
      observador.observe(document.documentElement || document, { childList: true, subtree: true, characterData: true });
      observadorAte = Date.now() + 120000;
    } catch (e) {}
  }
  function desligarObservador() {
    try {
      if (observador) {
        observador.disconnect();
        observador = null;
      }
    } catch (e) {}
    observadorAte = 0;
  }
  window.__odColetar = coletar;
  window.__odAplicar = function(mapaJSON, alvo) {
    try {
      if (aplicando) {
        return JSON.stringify({ ok: false, err: "ocupado" });
      }
      aplicando = true;
      const novo = typeof mapaJSON === "string" ? JSON.parse(mapaJSON) : mapaJSON || {};
      alvoAtual = alvo || alvoAtual;
      for (const k in novo) {
        if (Object.prototype.hasOwnProperty.call(novo, k)) {
          const v = novo[k];
          if (v && String(v).trim() !== "") {
            mapa[k] = String(v).trim();
          }
        }
      }
      const n = olhar(document.body);
      const restantes = [];
      for (let i = 0;i < novos.length; i++) {
        if (mapa[novos[i]] === undefined) {
          restantes.push(novos[i]);
        }
      }
      novos = restantes;
      novosSet = Object.create(null);
      for (let j = 0;j < novos.length; j++) {
        novosSet[novos[j]] = 1;
      }
      if (alvoAtual) {
        try {
          document.documentElement.setAttribute("lang", alvoAtual);
        } catch (e) {}
      }
      window.__odTradAtivo = true;
      ligarObservador();
      aplicando = false;
      return JSON.stringify({ ok: true, n, total: Object.keys(mapa).length });
    } catch (e) {
      aplicando = false;
      return JSON.stringify({ ok: false, err: String(e) });
    }
  };
  window.__odNovos = function() {
    const saida = novos.slice(0, 200);
    return JSON.stringify({ ok: true, itens: saida });
  };
  window.__odDesfazer = function() {
    try {
      desligarObservador();
      let n = 0;
      const todos = document.body ? document.body.querySelectorAll("*") : [];
      for (let i = 0;i < todos.length; i++) {
        const el = todos[i];
        const o = attrsOrig.get(el);
        if (o) {
          if (o.placeholder !== undefined) {
            el.setAttribute("placeholder", o.placeholder);
          }
          if (o.title !== undefined) {
            el.setAttribute("title", o.title);
          }
          attrsOrig.delete(el);
        }
        for (let k = 0;k < el.childNodes.length; k++) {
          const no = el.childNodes[k];
          if (no.nodeType !== 3) {
            continue;
          }
          if (originais.has(no)) {
            const orig = originais.get(no);
            if (orig !== undefined) {
              no.nodeValue = orig;
            }
            originais.delete(no);
            n++;
          }
        }
      }
      mapa = {};
      window.__odTradMapa = mapa;
      window.__odTradAtivo = false;
      novos = [];
      novosSet = Object.create(null);
      return JSON.stringify({ ok: true, n });
    } catch (e) {
      return JSON.stringify({ ok: false, err: String(e) });
    }
  };
  window.__odTrad = {
    ativo: function() {
      return window.__odTradAtivo === true;
    },
    mapa: function() {
      return mapa;
    }
  };
  window.__odTradAtivo = false;
  window.__odTradMapa = mapa;
})();
