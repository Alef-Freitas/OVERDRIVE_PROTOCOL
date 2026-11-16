'use strict';

// chatgpt-lite.ts
(() => {
  try {
    if (window.__odLiteSkip) {
      return;
    }
    if (window.top === window && sessionStorage.getItem("__od_lite_off__")) {
      sessionStorage.removeItem("__od_lite_off__");
      window.__odLiteSkip = true;
      return;
    }
  } catch (e) {}
  const host = String(location.hostname || "");
  if (host !== "chatgpt.com" && host !== "chat.openai.com") {
    return;
  }
  if (window.__odChatLite) {
    return;
  }
  const prefix = "overdrive.chatgpt-lite.";
  function cfg(key, fallback) {
    try {
      const value = localStorage.getItem(prefix + key);
      return value === null ? fallback : JSON.parse(value);
    } catch (e) {
      return fallback;
    }
  }
  if (cfg("enabled", true) === false) {
    return;
  }
  const noTel = cfg("noTelemetry", true) !== false;
  let telBloq = 0;
  const telAmostra = [];
  const telAmostraMax = 40;
  function amostraRede(u) {
    try {
      const s = String(u || "");
      if (s.indexOf("http") !== 0 && s.indexOf("//") !== 0) {
        return;
      }
      const a = document.createElement("a");
      a.href = s;
      const h = a.hostname || "";
      if (!h) {
        return;
      }
      const partes = String(a.pathname || "").split("/").filter(function(x) {
        return !!x;
      }).slice(0, 2);
      const chave = h + "/" + partes.join("/");
      if (telAmostra.indexOf(chave) < 0 && telAmostra.length < telAmostraMax) {
        telAmostra.push(chave);
      }
    } catch (e) {}
  }
  const telRE = /(unauth-mweb\/events\/|backend-anon\/beacons\/|\/events\/(business|statsc|performance|page-view)|statsc\/flush|ab\.chatgpt\.com\/v1\/sdk_exception|\/ces\/v1|bzr\.openai\.com\/v1\/obi)/;
  function ehTel(u) {
    try {
      return telRE.test(String(u || ""));
    } catch (e) {
      return false;
    }
  }
  function urlDeFetch(x) {
    try {
      if (typeof x === "string") {
        return x;
      }
      const r = x;
      const u = r && r.url;
      return typeof u === "string" ? u : "";
    } catch (e) {
      return "";
    }
  }
  if (noTel) {
    try {
      if (window.fetch) {
        const fetchOrig = window.fetch;
        window.fetch = function(...args) {
          amostraRede(urlDeFetch(args[0]));
          if (ehTel(urlDeFetch(args[0]))) {
            telBloq++;
            return Promise.resolve(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
          }
          return fetchOrig.apply(window, args);
        };
      }
    } catch (e) {}
    try {
      const XHR = window.XMLHttpRequest;
      const XHRP = XHR ? XHR.prototype : undefined;
      if (XHRP && XHRP.open) {
        const openOrig = XHRP.open;
        XHRP.open = function(metodo, url, ...rest) {
          try {
            amostraRede(url);
            if (ehTel(url)) {
              this.__odTelBlock = true;
            }
          } catch (e2) {}
          openOrig.apply(this, [metodo, url, ...rest]);
        };
        const sendOrig = XHRP.send;
        XHRP.send = function(...args) {
          if (this.__odTelBlock) {
            telBloq++;
            return;
          }
          sendOrig.apply(this, args);
        };
      }
    } catch (e) {}
    try {
      if (navigator.sendBeacon) {
        const beaconOrig = navigator.sendBeacon.bind(navigator);
        navigator.sendBeacon = function(url, ...rest) {
          amostraRede(url);
          if (ehTel(url)) {
            telBloq++;
            return true;
          }
          return beaconOrig.apply(navigator, [url, ...rest]);
        };
      }
    } catch (e) {}
  }
  const telTagRE = /(beacon|telemetry|statsc|pixel\.gif|collect\?|analytics\/)/;
  function podaTelNo(no) {
    try {
      if (!no || no.nodeType !== 1) {
        return;
      }
      const el = no;
      const tag = (el.tagName || "").toUpperCase();
      if (tag !== "IMG" && tag !== "IFRAME") {
        return;
      }
      const srcEl = el;
      const src = String(srcEl.currentSrc || srcEl.src || "");
      if (src) {
        amostraRede(src);
      }
      if (src && telTagRE.test(src)) {
        if (el.parentNode) {
          el.parentNode.removeChild(no);
          telBloq++;
        }
      }
    } catch (e) {}
  }
  function varreduraTel(raiz) {
    try {
      const base = raiz || document;
      const lista = base.querySelectorAll("img,iframe");
      for (let i = 0;i < lista.length; i++) {
        podaTelNo(lista[i]);
      }
    } catch (e) {}
  }
  const SEL = '[data-testid^="conversation-turn-"], [data-turn-id], [class*="virtualized-turn"]';
  const buffer = Math.max(800, Math.min(8000, parseInt(String(cfg("buffer", 2500)), 10) || 2500));
  const unloadImages = cfg("unloadImages", true) !== false;
  const noAnim = cfg("noAnim", true) !== false;
  const noSpell = cfg("spellcheck", true) !== false;
  const hideStream = cfg("hideStream", true) !== false;
  const maxTurns = Math.max(20, Math.min(200, parseInt(String(cfg("maxTurns", 40)), 10) || 40));
  const pruneTurns = cfg("prune", true) !== false;
  const placeholder = "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=";
  const turns = new Set;
  const seen = new WeakSet;
  const BLOCK_SEL = '[class*="prose"] p,[class*="prose"] pre,[class*="prose"] ul,[class*="prose"] ol,[class*="prose"] blockquote';
  let blockSeen = new WeakSet;
  const images = new WeakMap;
  const held = new Set;
  const queued = new Set;
  let queuedTimer = 0;
  let podarInterval = 0;
  let liteStyle = null;
  let stopped = false;
  let gerando = null;
  let ativo = false;
  let ativoEnviado = null;
  let ultimoAtivo = 0;
  let gerTimer = 0;
  let gerInterval = 0;
  let atividadeLigada = false;
  const ric = window.requestIdleCallback;
  function scrollavel(el) {
    let cur = el;
    while (cur && cur !== document.body) {
      try {
        const cs = getComputedStyle(cur);
        if (/(auto|scroll)/.test(cs.overflowY) && cur.scrollHeight > cur.clientHeight + 20) {
          return cur;
        }
      } catch (e) {}
      cur = cur.parentElement;
    }
    return document.scrollingElement || document.documentElement;
  }
  function podarTurns() {
    if (stopped || !pruneTurns || gerando) {
      return;
    }
    try {
      if (document.querySelector('[class*="virtualized-turn"]')) {
        return;
      }
      const lista = document.querySelectorAll(SEL);
      if (lista.length <= maxTurns + 5) {
        return;
      }
      const excesso = lista.length - maxTurns;
      let removidos = 0;
      for (let i = 0;i < lista.length && removidos < excesso; i++) {
        const el = lista[i];
        if (el.contains(document.activeElement)) {
          continue;
        }
        const r = el.getBoundingClientRect();
        if (r.bottom >= 0) {
          break;
        }
        const sc = scrollavel(el);
        const antes = sc ? sc.scrollHeight : 0;
        const topo = sc ? sc.scrollTop : 0;
        if (el.parentNode) {
          el.parentNode.removeChild(el);
        }
        if (sc) {
          sc.scrollTop = Math.max(0, topo - (antes - sc.scrollHeight));
        }
        removidos++;
      }
    } catch (e) {}
  }
  function limparStreaming() {
    try {
      const marcados = document.querySelectorAll(".od-streaming-turn");
      for (let i = 0;i < marcados.length; i++) {
        marcados[i].classList.remove("od-streaming-turn");
      }
    } catch (e) {}
  }
  function marcarStreaming() {
    if (!hideStream) {
      return;
    }
    try {
      let atual = null;
      const st = document.querySelector('.result-streaming, [class*="result-streaming"]');
      if (st) {
        atual = st.closest('[data-message-author-role="assistant"]') || st;
      }
      if (!atual) {
        const turnsQ = document.querySelectorAll('[data-testid^="conversation-turn-"], [data-turn-id]');
        for (let i = turnsQ.length - 1;i >= 0; i--) {
          const a = turnsQ[i].querySelector('[data-message-author-role="assistant"]');
          if (a) {
            atual = a;
            break;
          }
        }
      }
      if (!atual) {
        const todos = document.querySelectorAll('[data-message-author-role="assistant"]');
        if (todos.length) {
          atual = todos[todos.length - 1];
        }
      }
      if (!atual) {
        return;
      }
      if (!atual.classList.contains("od-streaming-turn")) {
        atual.classList.add("od-streaming-turn");
      }
    } catch (e) {}
  }
  function avisarEstado() {
    let stop = null;
    try {
      stop = document.querySelector('[data-testid="stop-button"], [data-testid*="stop" i], #composer-stop-button, button[aria-label*="Stop" i], button[aria-label*="Parar" i], button[aria-label*="Interromper" i], .result-streaming, [class*="result-streaming"]');
    } catch (e) {}
    const streaming = !!stop;
    try {
      document.documentElement.classList.toggle("od-streaming", streaming);
    } catch (e) {}
    if (streaming) {
      marcarStreaming();
    } else {
      limparStreaming();
    }
    if (streaming === gerando && ativo === ativoEnviado) {
      return;
    }
    gerando = streaming;
    ativoEnviado = ativo;
    try {
      if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
        window.chrome.webview.postMessage(JSON.stringify({ t: "odchat", streaming, active: ativo }));
      }
    } catch (e) {}
  }
  function agendarEstado() {
    if (gerTimer) {
      return;
    }
    gerTimer = window.setTimeout(function() {
      gerTimer = 0;
      avisarEstado();
    }, 250);
  }
  function marcarAtivo() {
    ultimoAtivo = Date.now();
    if (!ativo) {
      ativo = true;
      agendarEstado();
    }
  }
  function ligarAtividade() {
    if (atividadeLigada) {
      return;
    }
    atividadeLigada = true;
    const eventos = ["keydown", "mousedown", "pointerdown", "touchstart", "input", "focusin", "wheel"];
    for (let i = 0;i < eventos.length; i++) {
      document.addEventListener(eventos[i], marcarAtivo, { capture: true, passive: true });
    }
    document.addEventListener("scroll", marcarAtivo, { capture: true, passive: true });
  }
  function styleText() {
    const parts = [
      "html{scroll-behavior:auto!important}",
      ".od-cv{content-visibility:auto!important;contain-intrinsic-size:auto 420px!important}",
      '[class*="LoadingBlock"]{will-change:auto!important}',
      '[class*="backdrop-blur"],[class*="backdrop-filter"],[style*="backdrop-filter"]{backdrop-filter:none!important;-webkit-backdrop-filter:none!important}'
    ];
    if (hideStream) {
      parts.push("html.od-streaming .od-streaming-turn{content-visibility:hidden!important;contain-intrinsic-size:auto 320px!important}");
    }
    if (noAnim) {
      parts.push("*{animation-duration:.001s!important;animation-iteration-count:1!important;transition-duration:0s!important}");
    }
    return parts.join(`
`);
  }
  function installCSS() {
    if (liteStyle && liteStyle.isConnected) {
      return;
    }
    liteStyle = document.createElement("style");
    liteStyle.id = "od-chatgpt-lite-style";
    liteStyle.textContent = styleText();
    (document.head || document.documentElement).appendChild(liteStyle);
  }
  function prepararImagem(img) {
    if (!img || img.hasAttribute("data-od-lite-img")) {
      return;
    }
    img.setAttribute("data-od-lite-img", "1");
    try {
      if (img.getAttribute("loading") !== "lazy") {
        img.loading = "lazy";
      }
      if (img.getAttribute("decoding") !== "async") {
        img.decoding = "async";
      }
    } catch (e) {}
  }
  function prepararImagensEm(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    const el = root;
    if (el.tagName === "IMG") {
      prepararImagem(el);
      return;
    }
    const list = el.querySelectorAll ? el.querySelectorAll("img") : null;
    if (list) {
      for (let i = 0;i < list.length; i++) {
        prepararImagem(list[i]);
      }
    }
  }
  function desligarSpell(root) {
    if (!noSpell) {
      return;
    }
    try {
      document.documentElement.setAttribute("spellcheck", "false");
    } catch (e) {}
    const lista = [];
    const rel = root && root.nodeType === 1 ? root : null;
    if (rel && rel.matches && rel.matches('[contenteditable="true"]')) {
      lista.push(rel);
    }
    if (rel && rel.querySelectorAll) {
      const achados = rel.querySelectorAll('[contenteditable="true"]');
      for (let i = 0;i < achados.length; i++) {
        lista.push(achados[i]);
      }
    }
    if (!root) {
      const ed = document.querySelector('[contenteditable="true"]');
      if (ed) {
        lista.push(ed);
      }
    }
    for (let j = 0;j < lista.length; j++) {
      try {
        lista[j].setAttribute("spellcheck", "false");
        lista[j].spellcheck = false;
      } catch (e) {}
    }
  }
  function tirarImagens(sec) {
    if (!unloadImages) {
      return;
    }
    const list = sec.querySelectorAll("img");
    for (let i = 0;i < list.length; i++) {
      const img = list[i];
      prepararImagem(img);
      if (images.has(img)) {
        continue;
      }
      const src = img.getAttribute("src");
      if (!src || src.indexOf("data:") === 0) {
        continue;
      }
      const srcset = img.getAttribute("srcset");
      images.set(img, { src, srcset });
      held.add(img);
      try {
        img.removeAttribute("srcset");
        img.setAttribute("src", placeholder);
        img.style.visibility = "hidden";
      } catch (e) {}
    }
  }
  function voltarImagens(sec) {
    const list = sec.querySelectorAll("img");
    for (let i = 0;i < list.length; i++) {
      const img = list[i];
      const data = images.get(img);
      if (!data) {
        continue;
      }
      try {
        if (data.srcset) {
          img.setAttribute("srcset", data.srcset);
        }
        img.setAttribute("src", data.src);
        img.style.visibility = "";
      } catch (e) {}
      images.delete(img);
      held.delete(img);
    }
  }
  function esquecerImagens(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    const el = root;
    const list = el.tagName === "IMG" ? [el] : el.querySelectorAll("img");
    for (let i = 0;i < list.length; i++) {
      images.delete(list[i]);
      held.delete(list[i]);
    }
  }
  const blockIO = new IntersectionObserver(function(entries) {
    for (let i = 0;i < entries.length; i++) {
      if (!entries[i].isIntersecting) {
        continue;
      }
      const el = entries[i].target;
      try {
        const h = Math.ceil(el.getBoundingClientRect().height);
        if (h > 0) {
          el.style.setProperty("contain-intrinsic-size", "auto " + h + "px", "important");
        }
        el.classList.add("od-cv");
        blockSeen.add(el);
      } catch (e) {}
      blockIO.unobserve(el);
    }
  }, { rootMargin: "200px 0px" });
  function observarBlocos(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    const el = root;
    if (el.matches && el.matches(BLOCK_SEL) && !blockSeen.has(el)) {
      blockIO.observe(el);
    }
    const list = el.querySelectorAll ? el.querySelectorAll(BLOCK_SEL) : null;
    if (list) {
      for (let i = 0;i < list.length; i++) {
        if (!blockSeen.has(list[i])) {
          blockIO.observe(list[i]);
        }
      }
    }
  }
  const io = new IntersectionObserver(function(entries) {
    for (let i = 0;i < entries.length; i++) {
      const sec = entries[i].target;
      if (entries[i].isIntersecting) {
        voltarImagens(sec);
      } else {
        tirarImagens(sec);
      }
    }
  }, { rootMargin: buffer + "px 0px" });
  function observar(sec) {
    if (stopped || !sec || seen.has(sec)) {
      return;
    }
    seen.add(sec);
    turns.add(sec);
    io.observe(sec);
    const imgs = sec.querySelectorAll("img");
    for (let i = 0;i < imgs.length; i++) {
      prepararImagem(imgs[i]);
    }
  }
  function escanear(root) {
    if (!root || root.nodeType !== 1 || !root.isConnected) {
      return;
    }
    const el = root;
    podaTelNo(el);
    const tel = el.querySelectorAll ? el.querySelectorAll("img,iframe") : null;
    if (tel) {
      for (let ti = 0;ti < tel.length; ti++) {
        podaTelNo(tel[ti]);
      }
    }
    prepararImagensEm(el);
    if (el.matches && el.matches('[contenteditable="true"]')) {
      desligarSpell(el);
    }
    if (el.matches && el.matches(SEL)) {
      observar(el);
    }
    const list = el.querySelectorAll ? el.querySelectorAll(SEL) : null;
    if (list) {
      for (let i = 0;i < list.length; i++) {
        observar(list[i]);
      }
    }
    observarBlocos(el);
  }
  function agendar(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    queued.add(root);
    if (queuedTimer) {
      return;
    }
    const run = function() {
      queuedTimer = 0;
      queued.forEach(escanear);
      queued.clear();
    };
    if (typeof ric === "function") {
      queuedTimer = ric(run, { timeout: 400 });
    } else {
      queuedTimer = window.setTimeout(run, 120);
    }
  }
  const mo = new MutationObserver(function(mutations) {
    for (let i = 0;i < mutations.length; i++) {
      const m = mutations[i];
      for (let a = 0;a < m.addedNodes.length; a++) {
        agendar(m.addedNodes[a]);
      }
      for (let r = 0;r < m.removedNodes.length; r++) {
        const rem = m.removedNodes[r];
        if (!rem || rem.nodeType !== 1) {
          continue;
        }
        const rel = rem;
        esquecerImagens(rel);
        if (rel.matches && rel.matches(SEL)) {
          io.unobserve(rel);
          turns.delete(rel);
        }
        if (rel.matches && rel.matches(BLOCK_SEL)) {
          blockIO.unobserve(rel);
        }
        if (rel.querySelectorAll) {
          const gone = rel.querySelectorAll(SEL);
          for (let g = 0;g < gone.length; g++) {
            io.unobserve(gone[g]);
            turns.delete(gone[g]);
          }
          const goneBlocks = rel.querySelectorAll(BLOCK_SEL);
          for (let b = 0;b < goneBlocks.length; b++) {
            blockIO.unobserve(goneBlocks[b]);
          }
        }
      }
    }
    agendarEstado();
  });
  function start() {
    installCSS();
    desligarSpell(null);
    varreduraTel(document);
    escanear(document.body);
    ligarAtividade();
    ultimoAtivo = Date.now();
    avisarEstado();
    const imgs = document.images;
    for (let i = 0;i < imgs.length; i++) {
      prepararImagem(imgs[i]);
    }
    mo.observe(document.body, { childList: true, subtree: true });
    if (!gerInterval) {
      gerInterval = window.setInterval(function() {
        if (stopped || document.hidden) {
          return;
        }
        if (ativo && Date.now() - ultimoAtivo > 8000) {
          ativo = false;
        }
        avisarEstado();
      }, 1500);
    }
    if (pruneTurns && !podarInterval) {
      podarInterval = window.setInterval(function() {
        if (stopped || document.hidden) {
          return;
        }
        podarTurns();
      }, 3000);
    }
    if (typeof ric === "function") {
      ric(function() {
        desligarSpell(null);
      }, { timeout: 3000 });
    } else {
      window.setTimeout(function() {
        desligarSpell(null);
      }, 1500);
    }
  }
  function disable() {
    stopped = true;
    try {
      document.documentElement.classList.remove("od-streaming");
    } catch (e) {}
    try {
      io.disconnect();
    } catch (e) {}
    try {
      blockIO.disconnect();
    } catch (e) {}
    try {
      mo.disconnect();
    } catch (e) {}
    if (gerInterval) {
      window.clearInterval(gerInterval);
      gerInterval = 0;
    }
    if (podarInterval) {
      window.clearInterval(podarInterval);
      podarInterval = 0;
    }
    const marcados = document.querySelectorAll(".od-cv");
    for (let m = 0;m < marcados.length; m++) {
      const mEl = marcados[m];
      try {
        mEl.classList.remove("od-cv");
        mEl.style.removeProperty("contain-intrinsic-size");
      } catch (e) {}
    }
    blockSeen = new WeakSet;
    held.forEach(function(img) {
      const data = images.get(img);
      if (data) {
        try {
          if (data.srcset) {
            img.setAttribute("srcset", data.srcset);
          }
          img.setAttribute("src", data.src);
          img.style.visibility = "";
        } catch (e) {}
        images.delete(img);
      }
    });
    held.clear();
    turns.clear();
    if (liteStyle) {
      liteStyle.remove();
      liteStyle = null;
    }
    window.__odChatLite = null;
  }
  function contarCulling() {
    const r = { cvTurns: 0, cvProse: 0, proseChildren: 0, cvBlocks: 0, proseBlocks: 0 };
    const ts = document.querySelectorAll(SEL);
    for (let i = 0;i < ts.length; i++) {
      try {
        if (getComputedStyle(ts[i]).contentVisibility === "auto") {
          r.cvTurns++;
        }
      } catch (e) {}
    }
    const ds = document.querySelectorAll('[class*="prose"] > *');
    for (let j = 0;j < ds.length; j++) {
      r.proseChildren++;
      try {
        if (getComputedStyle(ds[j]).contentVisibility === "auto") {
          r.cvProse++;
        }
      } catch (e) {}
    }
    const bs = document.querySelectorAll(BLOCK_SEL);
    r.proseBlocks = bs.length;
    r.cvBlocks = document.querySelectorAll(".od-cv").length;
    return r;
  }
  window.__odChatLiteStats = function() {
    const mem = performance.memory;
    const heap = mem && mem.usedJSHeapSize || -1;
    const cv = contarCulling();
    const imgs = document.images;
    let lazy = 0;
    let decAsync = 0;
    for (let i = 0;i < imgs.length; i++) {
      if (imgs[i].getAttribute("loading") === "lazy") {
        lazy++;
      }
      if (imgs[i].getAttribute("decoding") === "async") {
        decAsync++;
      }
    }
    return {
      enabled: !stopped,
      turns: document.querySelectorAll(SEL).length,
      observed: turns.size,
      imagesHeld: held.size,
      imagesLazy: lazy,
      imagesAsync: decAsync,
      cvTurns: cv.cvTurns,
      cvProse: cv.cvProse,
      proseChildren: cv.proseChildren,
      cvBlocks: cv.cvBlocks,
      proseBlocks: cv.proseBlocks,
      nodes: document.getElementsByTagName("*").length,
      images: document.images.length,
      heap,
      telBloq,
      telAmostra: telAmostra.slice(),
      buffer
    };
  };
  window.__odChatLite = {
    stats: window.__odChatLiteStats,
    rescan: function() {
      escanear(document.body);
    },
    disable
  };
  if (document.body) {
    start();
  } else {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  }
})();
