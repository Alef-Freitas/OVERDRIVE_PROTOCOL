'use strict';

// dom-lite.ts
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
  if (window.top !== window) {
    return;
  }
  if (!/^https?:$/.test(location.protocol)) {
    return;
  }
  const host = String(location.hostname || "").toLowerCase();
  if (host === "127.0.0.1" || host === "localhost" || host === "::1") {
    return;
  }
  if (window.__odDomLite) {
    return;
  }
  window.__odDomLite = true;
  let sombras = [];
  function raizes() {
    const vivas = [];
    for (let i = 0;i < sombras.length; i++) {
      const r = sombras[i];
      if (!r.host || r.host.isConnected) {
        vivas.push(r);
      }
    }
    if (vivas.length !== sombras.length) {
      sombras = vivas;
    }
    return [document].concat(vivas);
  }
  let pruneFeed = false;
  try {
    const pfc = localStorage.getItem("overdrive.dom-lite.pruneFeed");
    if (pfc !== null) {
      pruneFeed = JSON.parse(pfc) !== false;
    }
  } catch (e) {}
  try {
    const intervalOriginal = window.setInterval;
    if (intervalOriginal && !intervalOriginal.__od) {
      const intervalNovo = (...args) => {
        const copia = args.slice();
        if (document.hidden && (typeof copia[1] !== "number" || copia[1] < 1000)) {
          copia[1] = 1000;
        }
        return intervalOriginal.apply(window, copia);
      };
      intervalNovo.__od = true;
      window.setInterval = intervalNovo;
    }
  } catch (e) {}
  (() => {
    const nativeRAF = window.requestAnimationFrame;
    if (!nativeRAF || window.__odRaf30) {
      return;
    }
    window.__odRaf30 = true;
    const raf = nativeRAF;
    let fila = new Map;
    let proximo = 1;
    let agendado = false;
    let ultimo = 0;
    function agendar() {
      if (agendado) {
        return;
      }
      agendado = true;
      raf.call(window, rodar);
    }
    function rodar(ts) {
      agendado = false;
      if (ts - ultimo < 30) {
        agendar();
        return;
      }
      ultimo = ts;
      const lote = fila;
      fila = new Map;
      lote.forEach((cb) => {
        try {
          cb(ts);
        } catch (e) {}
      });
      if (fila.size) {
        agendar();
      }
    }
    window.requestAnimationFrame = function(cb) {
      const id = proximo++;
      fila.set(id, cb);
      agendar();
      return id;
    };
    window.cancelAnimationFrame = function(id) {
      fila.delete(id);
    };
  })();
  let IA = /(^|\.)(chatgpt\.com|chat\.openai\.com|chat\.deepseek\.com|deepseek\.com|gemini\.google\.com)$/.test(host);
  const caminho = String(location.pathname || "");
  const busca = String(location.search || "");
  if ((host === "www.google.com" || host === "google.com") && (/^\/ai(\/|$)/.test(caminho) || /^\/search/.test(caminho) && /(?:[?&](?:udm=50|aep=11)(?:&|=|$))/.test(busca))) {
    IA = true;
  }
  const EMBED = /(youtube(-nocookie)?\.com\/embed|player\.vimeo\.com|facebook\.com\/plugins|platform\.twitter\.com|instagram\.com\/embed|open\.spotify\.com\/embed|w\.soundcloud\.com\/player|google\.[^/]+\/maps\/embed|tiktok\.com\/embed|embed\.reddit|player\.twitch\.tv|rumble\.com\/embed|odysee\.com\/\$\/embed)/i;
  const MIDIA_EMBED = /(youtube(-nocookie)?\.com\/embed|youtu\.be\/|player\.vimeo\.com|open\.spotify\.com\/embed|w\.soundcloud\.com\/player|player\.twitch\.tv|rumble\.com\/embed|odysee\.com\/\$\/embed|tiktok\.com\/embed)/i;
  if (!IA) {
    try {
      const st = document.createElement("style");
      st.id = "od-dom-lite-css";
      st.textContent = 'article,[role="article"]{content-visibility:auto!important;contain-intrinsic-size:auto 500px!important}' + "html{scroll-behavior:auto!important}" + "*{backdrop-filter:none!important;-webkit-backdrop-filter:none!important}" + '[style*="filter"]{filter:none!important}';
      (document.head || document.documentElement).appendChild(st);
    } catch (e) {}
  }
  function visivel(el, margem) {
    try {
      const r = el.getBoundingClientRect();
      return r.bottom > -margem && r.top < window.innerHeight + margem && r.right > -margem && r.left < window.innerWidth + margem;
    } catch (e) {
      return true;
    }
  }
  function scrollavel(el) {
    let cur = el;
    while (cur && cur !== document.body) {
      try {
        const cs = window.getComputedStyle(cur);
        if (/(auto|scroll)/.test(cs.overflowY) && cur.scrollHeight > cur.clientHeight + 20) {
          return cur;
        }
      } catch (e) {}
      cur = cur.parentElement;
    }
    return document.scrollingElement || document.documentElement;
  }
  function podarFeeds() {
    if (!pruneFeed) {
      return;
    }
    const feeds = document.querySelectorAll('[role="feed"],shreddit-feed');
    for (let i = 0;i < feeds.length; i++) {
      const feed = feeds[i];
      if (feed.children.length <= 80) {
        continue;
      }
      if (feed.querySelector && feed.querySelector('[class*="virtualized"]')) {
        continue;
      }
      const excesso = feed.children.length - 60;
      let removidos = 0;
      for (let j = 0;j < feed.children.length && removidos < 6 && removidos < excesso; j++) {
        const child = feed.children[j];
        const r = child.getBoundingClientRect();
        if (r.bottom >= 0) {
          break;
        }
        const sc = scrollavel(child);
        const antes = sc ? sc.scrollHeight : 0;
        const topo = sc ? sc.scrollTop : 0;
        try {
          feed.removeChild(child);
        } catch (e) {
          break;
        }
        if (sc) {
          sc.scrollTop = Math.max(0, topo - (antes - sc.scrollHeight));
        }
        removidos++;
        j--;
      }
    }
  }
  const willGuardado = new WeakMap;
  const containGuardado = new WeakMap;
  function ajustarCamadas() {
    const alvos = [];
    try {
      const anims = document.getAnimations ? document.getAnimations() : [];
      for (let i = 0;i < anims.length; i++) {
        const eff = anims[i].effect;
        const t = eff?.target ?? null;
        if (t && t.nodeType === 1) {
          alvos.push(t);
        }
      }
    } catch (e) {}
    try {
      const inline = document.querySelectorAll('[style*="will-change"],[style*="contain"]');
      for (let j = 0;j < inline.length && j < 150; j++) {
        alvos.push(inline[j]);
      }
    } catch (e) {}
    for (let k = 0;k < alvos.length; k++) {
      const el = alvos[k];
      const html = el;
      if (!el || el.nodeType !== 1 || el === document.body || el === document.documentElement || !html.style) {
        continue;
      }
      if (!visivel(el, 200)) {
        try {
          const cs = window.getComputedStyle(el);
          const cvis = cs.contentVisibility;
          if (cvis && cvis !== "visible") {
            continue;
          }
          if (cs.willChange && cs.willChange !== "auto") {
            if (!willGuardado.has(el)) {
              willGuardado.set(el, html.style.willChange || "");
            }
            html.style.willChange = "auto";
          }
          if (html.style.contain) {
            if (!containGuardado.has(el)) {
              containGuardado.set(el, html.style.contain);
            }
            html.style.contain = "none";
          }
        } catch (e) {}
      } else {
        if (willGuardado.has(el)) {
          const wv = willGuardado.get(el);
          if (wv) {
            html.style.willChange = wv;
          } else {
            html.style.removeProperty("will-change");
          }
          willGuardado.delete(el);
        }
        if (containGuardado.has(el)) {
          html.style.contain = containGuardado.get(el);
          containGuardado.delete(el);
        }
      }
    }
  }
  const midiaPausada = new WeakSet;
  const animPausada = new Set;
  const videoFora = new WeakMap;
  const videoGuardado = new WeakMap;
  const videoPreload = new WeakMap;
  const videoDescarregado = new WeakSet;
  const canvasFora = new WeakMap;
  const canvasGuardado = new WeakMap;
  const canvasEncolhido = new WeakSet;
  const canvasGL = new WeakMap;
  function ajustarCanvas() {
    const lista = [];
    const rs = raizes();
    for (let ri = 0;ri < rs.length; ri++) {
      try {
        const found = rs[ri].querySelectorAll("canvas");
        for (let fi = 0;fi < found.length; fi++) {
          lista.push(found[fi]);
        }
      } catch (e) {}
    }
    const limite = lista.length > 80 ? 80 : lista.length;
    for (let i = 0;i < limite; i++) {
      const c = lista[i];
      if (!visivel(c, 200)) {
        if (!canvasFora.has(c)) {
          canvasFora.set(c, Date.now());
        }
        if (!canvasEncolhido.has(c) && Date.now() - (canvasFora.get(c) || 0) > 45000) {
          const r = c.getBoundingClientRect();
          if (r.width < 120 || r.height < 80) {
            continue;
          }
          if ((c.width || 0) * (c.height || 0) < 40000) {
            continue;
          }
          try {
            canvasGuardado.set(c, { w: c.width, h: c.height, sw: c.style.width, sh: c.style.height });
            canvasEncolhido.add(c);
            c.width = 0;
            c.height = 0;
            if (!c.style.width) {
              c.style.width = Math.round(r.width) + "px";
            }
            if (!c.style.height) {
              c.style.height = Math.round(r.height) + "px";
            }
            try {
              const gl = c.getContext("webgl2") || c.getContext("webgl");
              if (gl) {
                const ext = gl.getExtension("WEBGL_lose_context");
                if (ext) {
                  canvasGL.set(c, ext);
                  ext.loseContext();
                }
              }
            } catch (e) {}
          } catch (e) {}
        }
      } else {
        canvasFora.delete(c);
        if (canvasEncolhido.has(c)) {
          const g = canvasGuardado.get(c);
          if (g) {
            try {
              c.width = g.w;
              c.height = g.h;
              c.style.width = g.sw;
              c.style.height = g.sh;
            } catch (e) {}
            canvasGuardado.delete(c);
          }
          const ext = canvasGL.get(c);
          if (ext) {
            try {
              ext.restoreContext();
            } catch (e) {}
            canvasGL.delete(c);
          }
          canvasEncolhido.delete(c);
        }
      }
    }
  }
  function ajustarMidia() {
    const ms = [];
    const rs = raizes();
    for (let ri = 0;ri < rs.length; ri++) {
      try {
        const found = rs[ri].querySelectorAll("video,audio");
        for (let fi = 0;fi < found.length; fi++) {
          ms.push(found[fi]);
        }
      } catch (e) {}
    }
    for (let i = 0;i < ms.length; i++) {
      const m = ms[i];
      const vis = visivel(m, 300);
      if (!vis) {
        if (!m.paused) {
          try {
            m.pause();
            midiaPausada.add(m);
          } catch (e) {}
        }
        if (m.tagName === "VIDEO") {
          if (!videoFora.has(m)) {
            videoFora.set(m, Date.now());
          }
          if (!videoPreload.has(m)) {
            videoPreload.set(m, m.getAttribute("preload"));
          }
          if (m.getAttribute("preload") !== "none") {
            try {
              m.setAttribute("preload", "none");
            } catch (e) {}
          }
          const desde = videoFora.get(m) || 0;
          if (!videoDescarregado.has(m) && m.paused && Date.now() - desde > 60000 && m.getAttribute("src") && !m.querySelector("source")) {
            try {
              videoGuardado.set(m, {
                src: m.getAttribute("src"),
                preload: videoPreload.get(m),
                time: m.currentTime || 0,
                ours: midiaPausada.has(m)
              });
              videoDescarregado.add(m);
              m.removeAttribute("src");
              m.load();
              midiaPausada.delete(m);
            } catch (e) {}
          }
        }
      } else {
        videoFora.delete(m);
        if (m.tagName === "VIDEO" && videoDescarregado.has(m)) {
          const g = videoGuardado.get(m);
          if (g) {
            try {
              if (g.src) {
                m.setAttribute("src", g.src);
              }
              if (g.preload !== null && g.preload !== undefined) {
                m.setAttribute("preload", g.preload);
              } else {
                m.removeAttribute("preload");
              }
              ((video, tempo, tocar) => {
                video.addEventListener("loadedmetadata", () => {
                  try {
                    if (tempo > 0 && isFinite(tempo)) {
                      video.currentTime = tempo;
                    }
                  } catch (e) {}
                  if (tocar) {
                    try {
                      const pr = video.play();
                      if (pr && pr.catch) {
                        pr.catch(() => {});
                      }
                    } catch (e) {}
                  }
                }, { once: true });
              })(m, g.time, g.ours);
              m.load();
            } catch (e) {}
            videoGuardado.delete(m);
            videoDescarregado.delete(m);
          }
        } else if (midiaPausada.has(m)) {
          midiaPausada.delete(m);
          try {
            const p = m.play();
            if (p && p.catch) {
              p.catch(() => {});
            }
          } catch (e) {}
        }
      }
    }
    const tocando = [];
    for (let vi = 0;vi < ms.length; vi++) {
      const vv = ms[vi];
      if (vv.tagName === "VIDEO" && !vv.paused && !vv.ended && vv.readyState > 2) {
        tocando.push(vv);
      }
    }
    if (tocando.length > 1) {
      let melhor = tocando[0];
      let areaMax = -1;
      for (let wi = 0;wi < tocando.length; wi++) {
        const rr = tocando[wi].getBoundingClientRect();
        const area = Math.max(0, rr.width) * Math.max(0, rr.height);
        if (area > areaMax) {
          areaMax = area;
          melhor = tocando[wi];
        }
      }
      for (let pi = 0;pi < tocando.length; pi++) {
        if (tocando[pi] !== melhor) {
          try {
            tocando[pi].pause();
          } catch (e) {}
        }
      }
    }
    if (document.getAnimations) {
      if (animPausada.size) {
        try {
          animPausada.forEach((a) => {
            const alvoAnim = a.effect?.target ?? null;
            if (!alvoAnim || !alvoAnim.isConnected) {
              animPausada.delete(a);
            }
          });
        } catch (e) {}
      }
      const anims = document.getAnimations();
      for (let j = 0;j < anims.length; j++) {
        const a = anims[j];
        const alvo = a.effect?.target ?? null;
        if (!alvo || alvo === document.body || alvo === document.documentElement) {
          continue;
        }
        if (!visivel(alvo, 200)) {
          if (a.playState === "running") {
            try {
              a.pause();
              animPausada.add(a);
            } catch (e) {}
          }
        } else if (animPausada.has(a)) {
          animPausada.delete(a);
          try {
            a.play();
          } catch (e) {}
        }
      }
    }
    ajustarCanvas();
    ajustarCamadas();
    podarFeeds();
  }
  let timerAjuste = 0;
  function agendarAjuste() {
    if (timerAjuste) {
      return;
    }
    timerAjuste = window.setTimeout(() => {
      timerAjuste = 0;
      ajustarMidia();
    }, 300);
  }
  window.addEventListener("scroll", agendarAjuste, { passive: true, capture: true });
  window.addEventListener("resize", agendarAjuste, { passive: true });
  window.setInterval(() => {
    if (!document.hidden) {
      agendarAjuste();
    }
  }, 4000);
  const iframeVisto = new WeakSet;
  const iframeIO = new IntersectionObserver((entries) => {
    for (let i = 0;i < entries.length; i++) {
      const e = entries[i];
      if (!e.isIntersecting) {
        trocarIframe(e.target);
      }
    }
  }, { rootMargin: "400px 0px" });
  function elegivelIframe(f) {
    if (!f || f.tagName !== "IFRAME" || f.hasAttribute("data-od-if-off")) {
      return false;
    }
    const src = f.getAttribute("src") || "";
    if (!EMBED.test(src)) {
      return false;
    }
    if (MIDIA_EMBED.test(src)) {
      return false;
    }
    const allow = String(f.getAttribute("allow") || "");
    if (/autoplay|encrypted-media|picture-in-picture/i.test(allow)) {
      return false;
    }
    if (/(captcha|recaptcha|hcaptcha|turnstile|checkout|payment)/i.test(src)) {
      return false;
    }
    const r = f.getBoundingClientRect();
    return r.width >= 120 && r.height >= 80;
  }
  function trocarIframe(f) {
    if (!elegivelIframe(f)) {
      return;
    }
    const frame = f;
    const r = frame.getBoundingClientRect();
    const attrs = {};
    const nomes = [
      "src",
      "width",
      "height",
      "title",
      "allow",
      "allowfullscreen",
      "frameborder",
      "loading",
      "referrerpolicy",
      "class",
      "style"
    ];
    for (let i = 0;i < nomes.length; i++) {
      if (frame.hasAttribute(nomes[i])) {
        attrs[nomes[i]] = frame.getAttribute(nomes[i]);
      }
    }
    const ph = document.createElement("div");
    ph.setAttribute("data-od-if-ph", "1");
    ph.textContent = "▶";
    ph.title = "Clique para carregar o conteudo";
    ph.style.cssText = "display:flex;align-items:center;justify-content:center;box-sizing:border-box;" + "width:" + Math.max(60, Math.round(r.width)) + "px;height:" + Math.max(60, Math.round(r.height)) + "px;" + "background:#0b1118;color:#9ecbff;border:1px solid #24384f;border-radius:6px;" + "font:16px/1 system-ui,sans-serif;cursor:pointer";
    if (frame.parentNode) {
      frame.parentNode.insertBefore(ph, frame);
      frame.parentNode.removeChild(frame);
    }
    iframeIO.unobserve(frame);
    iframeVisto.delete(frame);
    ph.addEventListener("click", () => {
      const novo = document.createElement("iframe");
      for (const k in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, k)) {
          novo.setAttribute(k, attrs[k]);
        }
      }
      if (ph.parentNode) {
        ph.parentNode.replaceChild(novo, ph);
      }
      window.setTimeout(() => {
        if (novo.parentNode && elegivelIframe(novo)) {
          iframeVisto.add(novo);
          iframeIO.observe(novo);
        }
      }, 1500);
    }, { once: true });
  }
  function observarIframes(root) {
    if (!root) {
      return;
    }
    const nt = root.nodeType;
    let list;
    if (nt === 9) {
      list = root.querySelectorAll("iframe");
    } else if (nt === 1) {
      const el = root;
      list = el.tagName === "IFRAME" ? [el] : el.querySelectorAll("iframe");
    } else {
      return;
    }
    for (let i = 0;i < list.length; i++) {
      const f = list[i];
      if (!f.hasAttribute("loading")) {
        f.setAttribute("loading", "lazy");
      }
      if (iframeVisto.has(f)) {
        continue;
      }
      iframeVisto.add(f);
      iframeIO.observe(f);
    }
  }
  function temFramework(el) {
    try {
      for (let no = el, nivel = 0;no && no.nodeType === 1 && nivel < 6; no = no.parentElement, nivel++) {
        const chaves = Object.keys(no);
        for (let i = 0;i < chaves.length; i++) {
          const k = chaves[i];
          if (k.indexOf("__react") === 0 || k.indexOf("__vue") === 0 || k.indexOf("__ng") === 0 || k.indexOf("__svelte") === 0) {
            return true;
          }
        }
      }
    } catch (e) {}
    return false;
  }
  function limparModais() {
    if (!document.body) {
      return;
    }
    const sel = '[role="dialog"],dialog,[class*="modal"],[class*="popup"],[class*="overlay"],[class*="cookie-banner"]';
    const lista = document.querySelectorAll(sel);
    let removidos = 0;
    for (let i = 0;i < lista.length && removidos < 20; i++) {
      const el = lista[i];
      const html = el;
      if (!el.parentNode || el === document.activeElement) {
        continue;
      }
      if (el.tagName === "DIALOG" && el.hasAttribute("open")) {
        continue;
      }
      let escondido = el.getAttribute("aria-hidden") === "true" || Boolean(html.hidden);
      if (!escondido) {
        try {
          const cs = window.getComputedStyle(el);
          const rr = el.getBoundingClientRect();
          escondido = cs.display === "none" || cs.visibility === "hidden" || Number(cs.opacity) === 0 || rr.width === 0 || rr.height === 0;
        } catch (e) {
          escondido = false;
        }
      }
      if (!escondido) {
        continue;
      }
      if (el.querySelector("input,textarea,select,button,a,video,audio,iframe,canvas")) {
        continue;
      }
      if (el.contains(document.activeElement)) {
        continue;
      }
      if (temFramework(el)) {
        continue;
      }
      try {
        el.parentNode.removeChild(el);
        removidos++;
      } catch (e) {}
    }
  }
  let timerLimpeza = 0;
  function agendarLimpeza() {
    if (timerLimpeza) {
      return;
    }
    timerLimpeza = window.setTimeout(() => {
      timerLimpeza = 0;
      limparModais();
    }, 15000);
  }
  const pendentes = [];
  let timer = 0;
  const mo = new MutationObserver((muts) => {
    for (let i = 0;i < muts.length; i++) {
      const ns = muts[i].addedNodes;
      for (let j = 0;j < ns.length; j++) {
        pendentes.push(ns[j]);
      }
    }
    if (timer) {
      return;
    }
    timer = window.setTimeout(() => {
      timer = 0;
      const lote = pendentes.splice(0, pendentes.length);
      for (let k = 0;k < lote.length; k++) {
        const n = lote[k];
        if (!n || n.nodeType !== 1) {
          continue;
        }
        observarIframes(n);
        varrerSombras(n);
      }
      agendarLimpeza();
    }, 400);
  });
  if (document.documentElement) {
    mo.observe(document.documentElement, { childList: true, subtree: true });
  }
  function registrarSombra(root) {
    if (!root || !root.querySelectorAll) {
      return;
    }
    for (let i = 0;i < sombras.length; i++) {
      if (sombras[i] === root) {
        return;
      }
    }
    sombras.push(root);
    observarIframes(root);
    try {
      mo.observe(root, { childList: true, subtree: true });
    } catch (e) {}
    varrerSombras(root);
  }
  function varrerSombras(root) {
    if (!root) {
      return;
    }
    const q = root;
    if (!q || !q.querySelectorAll) {
      return;
    }
    try {
      if (q.shadowRoot) {
        registrarSombra(q.shadowRoot);
      }
      const els = q.querySelectorAll("*");
      for (let i = 0;i < els.length; i++) {
        if (els[i].shadowRoot) {
          registrarSombra(els[i].shadowRoot);
        }
      }
    } catch (e) {}
  }
  try {
    const attachOriginal = Element.prototype.attachShadow;
    if (attachOriginal) {
      const attachNovo = function(init) {
        const raiz = attachOriginal.call(this, init);
        try {
          registrarSombra(raiz);
        } catch (e) {}
        return raiz;
      };
      attachNovo.__od = true;
      Element.prototype.attachShadow = attachNovo;
    }
  } catch (e) {}
  function iniciar() {
    observarIframes(document);
    varrerSombras(document);
    agendarAjuste();
    agendarLimpeza();
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", iniciar, { once: true });
  } else {
    iniciar();
  }
})();
