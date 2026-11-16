'use strict';

// img-lite.ts
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
  const caminho = String(location.pathname || "");
  const busca = String(location.search || "");
  if (window.top !== window) {
    return;
  }
  if (!/^https?:$/.test(location.protocol)) {
    return;
  }
  if (host === "127.0.0.1" || host === "localhost" || host === "::1") {
    return;
  }
  const googleAI = (host === "www.google.com" || host === "google.com") && (/^\/ai(\/|$)/.test(caminho) || /^\/search/.test(caminho) && /(?:[?&](?:udm=50|aep=11)(?:&|=|$))/.test(busca));
  if (host === "chatgpt.com" || host === "chat.openai.com" || host === "gemini.google.com" || host === "chat.deepseek.com" || host === "www.deepseek.com" || host === "deepseek.com" || googleAI) {
    return;
  }
  if (window.__odImgLite) {
    return;
  }
  window.__odImgLite = true;
  const placeholder = "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=";
  const BG_SEL = '[style*="background"],[class*="hero"],[class*="banner"],[class*="cover"],[class*="background"],[class*="thumb"],[class*="image"],[class*="photo"]';
  const guardadas = new WeakMap;
  const vistas = new WeakSet;
  let sombras = [];
  function elegivel(img) {
    if (!img || img.tagName !== "IMG") {
      return false;
    }
    const src = img.getAttribute("src");
    if (!src || src.indexOf("data:") === 0) {
      return false;
    }
    if (img.hasAttribute("data-od-img")) {
      return false;
    }
    return true;
  }
  function tirar(img) {
    if (!elegivel(img) || guardadas.has(img)) {
      return;
    }
    const r = img.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) {
      return;
    }
    const d = {
      src: img.getAttribute("src"),
      srcset: img.getAttribute("srcset"),
      w: img.style.width,
      h: img.style.height,
      fontes: []
    };
    const srcOriginal = img.currentSrc || img.src || d.src || "";
    if (srcOriginal) {
      img.setAttribute("data-od-src", srcOriginal);
    }
    if (d.srcset) {
      img.setAttribute("data-od-srcset", d.srcset);
    }
    const pic = img.parentElement;
    if (pic && pic.tagName === "PICTURE") {
      const ss = pic.querySelectorAll("source");
      for (let si = 0;si < ss.length; si++) {
        const s = ss[si];
        if (!s.hasAttribute("srcset")) {
          continue;
        }
        d.fontes.push({ el: s, srcset: s.getAttribute("srcset"), sizes: s.getAttribute("sizes") });
        s.removeAttribute("srcset");
      }
    }
    guardadas.set(img, d);
    img.setAttribute("data-od-img", "1");
    img.removeAttribute("srcset");
    img.setAttribute("src", placeholder);
    if (!d.w) {
      img.style.width = r.width + "px";
    }
    if (!d.h) {
      img.style.height = r.height + "px";
    }
  }
  function voltar(img) {
    const d = guardadas.get(img);
    if (!d) {
      return;
    }
    if (d.srcset) {
      img.setAttribute("srcset", d.srcset);
    }
    img.setAttribute("src", d.src);
    img.style.width = d.w;
    img.style.height = d.h;
    if (d.fontes) {
      for (let i = 0;i < d.fontes.length; i++) {
        const f = d.fontes[i];
        if (f.srcset) {
          f.el.setAttribute("srcset", f.srcset);
        }
        if (f.sizes) {
          f.el.setAttribute("sizes", f.sizes);
        }
      }
    }
    img.removeAttribute("data-od-img");
    img.removeAttribute("data-od-src");
    img.removeAttribute("data-od-srcset");
    img.setAttribute("fetchpriority", "auto");
    guardadas.delete(img);
  }
  const io = new IntersectionObserver((entries) => {
    for (let i = 0;i < entries.length; i++) {
      const e = entries[i];
      const alvo = e.target;
      if (e.isIntersecting) {
        voltar(alvo);
      } else {
        tirar(alvo);
      }
    }
  }, { rootMargin: "1200px 0px" });
  const gifIO = new IntersectionObserver((entries) => {
    for (let i = 0;i < entries.length; i++) {
      const e = entries[i];
      const alvo = e.target;
      if (e.isIntersecting) {
        voltar(alvo);
      } else {
        tirar(alvo);
      }
    }
  }, { rootMargin: "50px 0px" });
  function observar(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    const el = root;
    const list = el.tagName === "IMG" ? [el] : el.querySelectorAll("img");
    for (let i = 0;i < list.length; i++) {
      const im = list[i];
      if (im.hasAttribute("data-od-img") && !guardadas.has(im)) {
        const original = im.getAttribute("data-od-src");
        if (original) {
          im.setAttribute("src", original);
        }
        im.removeAttribute("data-od-img");
        im.removeAttribute("data-od-src");
        im.removeAttribute("data-od-srcset");
      }
      if (vistas.has(im)) {
        continue;
      }
      vistas.add(im);
      if (im.getAttribute("loading") !== "lazy") {
        im.loading = "lazy";
      }
      if (im.getAttribute("decoding") !== "async") {
        im.decoding = "async";
      }
      if (!im.hasAttribute("fetchpriority")) {
        im.setAttribute("fetchpriority", "low");
      }
      const u = String(im.currentSrc || im.src || im.getAttribute("src") || "");
      if (/\.gif(\?|#|$)/i.test(u)) {
        gifIO.observe(im);
      } else {
        io.observe(im);
      }
    }
  }
  let timer = 0;
  const pendentes = [];
  const mo = new MutationObserver((mutations) => {
    for (let i = 0;i < mutations.length; i++) {
      const nodes = mutations[i].addedNodes;
      for (let j = 0;j < nodes.length; j++) {
        pendentes.push(nodes[j]);
      }
      const removidos = mutations[i].removedNodes;
      for (let r = 0;r < removidos.length; r++) {
        esquecerNo(removidos[r]);
      }
    }
    if (timer) {
      return;
    }
    timer = window.setTimeout(() => {
      timer = 0;
      const lote = pendentes.splice(0, pendentes.length);
      for (let k = 0;k < lote.length; k++) {
        observar(lote[k]);
        observarBg(lote[k]);
        varrerSombrasImg(lote[k]);
      }
    }, 250);
  });
  const bgVistas = new WeakSet;
  const bgGuardadas = new WeakMap;
  const bgIO = new IntersectionObserver((entries) => {
    for (let i = 0;i < entries.length; i++) {
      const el = entries[i].target;
      const html = el;
      if (entries[i].isIntersecting) {
        const g = bgGuardadas.get(el);
        if (g) {
          if (g.inline) {
            html.style.backgroundImage = g.inline;
          } else {
            html.style.removeProperty("background-image");
          }
          bgGuardadas.delete(el);
        }
        continue;
      }
      if (bgGuardadas.has(el)) {
        continue;
      }
      let cs = null;
      try {
        cs = window.getComputedStyle(el);
      } catch (e) {
        continue;
      }
      const bi = cs.backgroundImage || "";
      if (!bi || bi === "none" || bi.indexOf("url(") < 0) {
        bgIO.unobserve(el);
        continue;
      }
      const r = el.getBoundingClientRect();
      if (r.width < 120 || r.height < 80) {
        bgIO.unobserve(el);
        continue;
      }
      bgGuardadas.set(el, { inline: html.style.backgroundImage || "" });
      html.style.backgroundImage = "none";
    }
  }, { rootMargin: "1000px 0px" });
  function esquecerNo(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    try {
      const q = root;
      const imgs = q.tagName === "IMG" ? [q] : q.querySelectorAll("img");
      for (let i = 0;i < imgs.length; i++) {
        const im = imgs[i];
        try {
          io.unobserve(im);
          gifIO.unobserve(im);
        } catch (e) {}
        guardadas.delete(im);
        vistas.delete(im);
      }
      let bgs;
      try {
        bgs = q.matches && q.matches(BG_SEL) ? [q] : q.querySelectorAll(BG_SEL);
      } catch (e) {
        bgs = [];
      }
      for (let b = 0;b < bgs.length; b++) {
        const el = bgs[b];
        try {
          bgIO.unobserve(el);
        } catch (e) {}
        bgVistas.delete(el);
        bgGuardadas.delete(el);
      }
    } catch (e) {}
  }
  function observarBg(root) {
    if (!root || root.nodeType !== 1) {
      return;
    }
    const sel = BG_SEL;
    const q = root;
    let list;
    try {
      list = q.matches && q.matches(sel) ? [q] : Array.prototype.slice.call(q.querySelectorAll(sel));
    } catch (e) {
      return;
    }
    const limite = list.length > 400 ? 400 : list.length;
    for (let i = 0;i < limite; i++) {
      const el = list[i];
      if (bgVistas.has(el)) {
        continue;
      }
      bgVistas.add(el);
      bgIO.observe(el);
    }
  }
  function start() {
    observar(document.body);
    observarBg(document.body);
    varrerSombrasImg(document.body);
    try {
      if (document.body) {
        mo.observe(document.body, { childList: true, subtree: true });
      }
    } catch (e) {}
  }
  function registrarSombraImg(root) {
    if (!root || !root.querySelectorAll) {
      return;
    }
    for (let i = 0;i < sombras.length; i++) {
      if (sombras[i] === root) {
        return;
      }
    }
    sombras.push(root);
    observar(root);
    observarBg(root);
    try {
      mo.observe(root, { childList: true, subtree: true });
    } catch (e) {}
    varrerSombrasImg(root);
  }
  function varrerSombrasImg(root) {
    if (sombras.length) {
      const vivas = [];
      for (let s = 0;s < sombras.length; s++) {
        const sr = sombras[s];
        if (!sr.host || sr.host.isConnected) {
          vivas.push(sr);
        }
      }
      if (vivas.length !== sombras.length) {
        sombras = vivas;
      }
    }
    if (!root) {
      return;
    }
    const q = root;
    if (!q || !q.querySelectorAll) {
      return;
    }
    try {
      if (q.shadowRoot) {
        registrarSombraImg(q.shadowRoot);
      }
      const els = q.querySelectorAll("*");
      for (let i = 0;i < els.length; i++) {
        if (els[i].shadowRoot) {
          registrarSombraImg(els[i].shadowRoot);
        }
      }
    } catch (e) {}
  }
  try {
    const attachOriginalImg = Element.prototype.attachShadow;
    if (attachOriginalImg && !attachOriginalImg.__od) {
      const attachNovoImg = function(init) {
        const raiz = attachOriginalImg.call(this, init);
        try {
          registrarSombraImg(raiz);
        } catch (e) {}
        return raiz;
      };
      attachNovoImg.__od = true;
      Element.prototype.attachShadow = attachNovoImg;
    }
  } catch (e) {}
  if (document.body) {
    start();
  } else {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  }
})();
