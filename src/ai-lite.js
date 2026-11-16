'use strict';

// ai-lite.ts
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
  const host = String(location.hostname || "").toLowerCase();
  const path = String(location.pathname || "");
  const search = String(location.search || "");
  const isChatGPT = host === "chatgpt.com" || host === "chat.openai.com";
  const isAIMode = (host === "www.google.com" || host === "google.com") && (/^\/ai(\/|$)/.test(path) || /^\/search/.test(path) && /(?:[?&](?:udm=50|aep=11)(?:&|=|$))/.test(search));
  const isGemini = host === "gemini.google.com";
  const isDeepSeek = host === "chat.deepseek.com" || host === "www.deepseek.com" || host === "deepseek.com";
  if (isChatGPT || !(isAIMode || isGemini || isDeepSeek)) {
    return;
  }
  if (window.__odAILite) {
    return;
  }
  const prefix = "overdrive.ai-lite.";
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
  const buffer = Math.max(800, Math.min(4000, parseInt(String(cfg("buffer", 2000)), 10) || 2000));
  const unloadImages = cfg("unloadImages", true) !== false;
  const noAnim = cfg("noAnim", true) !== false;
  const noWillChange = cfg("willChange", true) !== false;
  const maxTurns = Math.max(20, Math.min(200, parseInt(String(cfg("maxTurns", 40)), 10) || 40));
  const pruneTurns = cfg("prune", true) !== false;
  const placeholder = "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=";
  const images = new WeakMap;
  const held = new Set;
  const seen = new WeakSet;
  const queued = new Set;
  let queuedTimer = 0;
  let podarInterval = 0;
  let liteStyle = null;
  let stopped = false;
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
  function podarConversa() {
    if (stopped || !pruneTurns || !isGemini) {
      return;
    }
    try {
      if (document.querySelector('[class*="virtualized"]')) {
        return;
      }
      const lista = document.querySelectorAll("model-response,user-query");
      if (lista.length <= maxTurns + 5) {
        return;
      }
      const excesso = lista.length - maxTurns;
      let removidos = 0;
      for (let i = 0;i < lista.length && removidos < excesso; i++) {
        const el = lista[i];
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
  function styleText() {
    const parts = [
      "html{scroll-behavior:auto!important}",
      '[class*="backdrop-blur"],[class*="backdrop-filter"],[style*="backdrop-filter"]{backdrop-filter:none!important;-webkit-backdrop-filter:none!important}'
    ];
    if (noAnim) {
      parts.push("*{animation-duration:.001s!important;animation-iteration-count:1!important;transition-duration:0s!important}");
    }
    if (noWillChange) {
      parts.push("*{will-change:auto!important}");
    }
    if (isAIMode) {
      parts.push('[role="main"] ul,[role="main"] ol,[role="main"] [role="listitem"],[role="main"] table,[role="main"] h1,[role="main"] h2,[role="main"] h3,main ul,main ol,main [role="listitem"],main table,main h1,main h2,main h3{content-visibility:auto!important;contain-intrinsic-size:auto 180px!important}');
    }
    return parts.join(`
`);
  }
  function installCSS() {
    if (liteStyle && liteStyle.isConnected) {
      return;
    }
    liteStyle = document.createElement("style");
    liteStyle.id = "od-ai-lite-style";
    liteStyle.textContent = styleText();
    (document.head || document.documentElement).appendChild(liteStyle);
  }
  function prepare(img) {
    if (!img || img.hasAttribute("data-od-ai-img")) {
      return;
    }
    img.setAttribute("data-od-ai-img", "1");
    try {
      if (img.getAttribute("loading") !== "lazy") {
        img.loading = "lazy";
      }
      if (img.getAttribute("decoding") !== "async") {
        img.decoding = "async";
      }
    } catch (e) {}
  }
  function worth(img) {
    const src = img.getAttribute("src") || "";
    const srcset = img.getAttribute("srcset") || "";
    if (!src && !srcset) {
      return false;
    }
    if (src.indexOf("data:") === 0 && src.length < 512 && !srcset) {
      return false;
    }
    if (src.indexOf("data:image/gif") === 0 && src.length < 128) {
      return false;
    }
    return true;
  }
  function unload(img) {
    if (!unloadImages || !img || images.has(img)) {
      return;
    }
    prepare(img);
    if (!worth(img)) {
      return;
    }
    const src = img.getAttribute("src");
    const srcset = img.getAttribute("srcset");
    const visibility = img.style.visibility;
    images.set(img, { src, srcset, visibility });
    held.add(img);
    try {
      img.removeAttribute("srcset");
      img.setAttribute("src", placeholder);
      img.style.visibility = "hidden";
    } catch (e) {}
  }
  function restore(img) {
    const data = images.get(img);
    if (!data) {
      return;
    }
    try {
      if (data.srcset) {
        img.setAttribute("srcset", data.srcset);
      }
      if (data.src) {
        img.setAttribute("src", data.src);
      } else {
        img.removeAttribute("src");
      }
      img.style.visibility = data.visibility || "";
    } catch (e) {}
    images.delete(img);
    held.delete(img);
  }
  function forget(root) {
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
  const io = new IntersectionObserver(function(entries) {
    for (let i = 0;i < entries.length; i++) {
      const img = entries[i].target;
      if (entries[i].isIntersecting) {
        restore(img);
      } else {
        unload(img);
      }
    }
  }, { rootMargin: buffer + "px 0px" });
  function observe(img) {
    if (stopped || !img || seen.has(img)) {
      return;
    }
    seen.add(img);
    prepare(img);
    io.observe(img);
  }
  function scan(root) {
    if (!root || root.nodeType !== 1 || !root.isConnected) {
      return;
    }
    const el = root;
    if (el.tagName === "IMG") {
      observe(el);
      return;
    }
    const list = el.querySelectorAll ? el.querySelectorAll("img") : null;
    if (list) {
      for (let i = 0;i < list.length; i++) {
        observe(list[i]);
      }
    }
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
      queued.forEach(scan);
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
        forget(m.removedNodes[r]);
      }
    }
  });
  function start() {
    installCSS();
    scan(document.body);
    mo.observe(document.body, { childList: true, subtree: true });
    if (isGemini && pruneTurns && !podarInterval) {
      podarInterval = window.setInterval(function() {
        if (stopped || document.hidden) {
          return;
        }
        podarConversa();
      }, 3000);
    }
  }
  function disable() {
    stopped = true;
    try {
      io.disconnect();
    } catch (e) {}
    try {
      mo.disconnect();
    } catch (e) {}
    if (podarInterval) {
      window.clearInterval(podarInterval);
      podarInterval = 0;
    }
    held.forEach(function(img) {
      if (img.isConnected) {
        restore(img);
      }
    });
    held.clear();
    if (liteStyle) {
      liteStyle.remove();
      liteStyle = null;
    }
    window.__odAILite = null;
  }
  window.__odAILiteStats = function() {
    let lazy = 0;
    let decAsync = 0;
    for (let i = 0;i < document.images.length; i++) {
      if (document.images[i].getAttribute("loading") === "lazy") {
        lazy++;
      }
      if (document.images[i].getAttribute("decoding") === "async") {
        decAsync++;
      }
    }
    const mem = performance.memory;
    return {
      enabled: !stopped,
      host,
      images: document.images.length,
      lazy,
      async: decAsync,
      held: held.size,
      nodes: document.getElementsByTagName("*").length,
      buffer,
      heap: mem && mem.usedJSHeapSize || -1
    };
  };
  window.__odAILite = {
    stats: window.__odAILiteStats,
    rescan: function() {
      scan(document.body);
    },
    disable
  };
  if (document.body) {
    start();
  } else {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  }
})();
