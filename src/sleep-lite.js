'use strict';

// sleep-lite.ts
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
  if (window.__odSleepLite) {
    return;
  }
  window.__odSleepLite = true;
  const nSetTimeout = window.setTimeout.bind(window);
  const nClearTimeout = window.clearTimeout.bind(window);
  const nSetInterval = window.setInterval.bind(window);
  const nClearInterval = window.clearInterval.bind(window);
  const nRAF = window.requestAnimationFrame.bind(window);
  const nCAF = window.cancelAnimationFrame.bind(window);
  let ligado = false;
  let dormindo = false;
  const ociosoMs = 1000;
  let idleTimer = 0;
  let proximoId = 1;
  const registros = new Map;
  const rafMap = new Map;
  const rafNativos = new Map;
  let rafProximo = 1;
  let animPausadas = [];
  let pendentesIO = [];
  const IO = window.IntersectionObserver;
  if (IO) {
    try {
      const Nativo = IO;
      window.IntersectionObserver = function(cb, opts) {
        const guarda = function(entries, obs) {
          if (dormindo) {
            if (pendentesIO.length < 200) {
              pendentesIO.push({ cb, entries, obs });
            }
            return;
          }
          try {
            cb(entries, obs);
          } catch (e) {}
        };
        return new Nativo(guarda, opts);
      };
      window.IntersectionObserver.prototype = IO.prototype;
    } catch (e) {}
  }
  function drenarIO() {
    if (!pendentesIO.length) {
      return;
    }
    const fila = pendentesIO;
    pendentesIO = [];
    for (let i = 0;i < fila.length; i++) {
      try {
        fila[i].cb(fila[i].entries, fila[i].obs);
      } catch (e) {}
    }
  }
  let drenagemAgendada = false;
  function agendarDrenarIO() {
    if (drenagemAgendada) {
      return;
    }
    drenagemAgendada = true;
    nSetTimeout(function() {
      drenagemAgendada = false;
      drenarIO();
    }, 0);
  }
  window.setTimeout = function(fn, delay, ...rest) {
    if (typeof fn !== "function") {
      return nSetTimeout.apply(null, [fn, delay, ...rest]);
    }
    const args = rest;
    const id = proximoId++;
    const d = Math.max(0, Number(delay) || 0);
    const r = { id, tipo: "timeout", fn, args, delay: d, restante: d, nativo: 0, next: 0, ativo: true };
    registros.set(id, r);
    if (!dormindo) {
      agendarTimer(r);
    }
    return id;
  };
  window.clearTimeout = function(id) {
    if (id === undefined) {
      return;
    }
    const r = registros.get(id);
    if (!r) {
      return;
    }
    r.ativo = false;
    if (r.nativo) {
      nClearTimeout(r.nativo);
    }
    registros.delete(id);
  };
  window.setInterval = function(fn, delay, ...rest) {
    if (typeof fn !== "function") {
      return nSetInterval.apply(null, [fn, delay, ...rest]);
    }
    const args = rest;
    const id = proximoId++;
    const d = Math.max(0, Number(delay) || 0);
    const r = { id, tipo: "interval", fn, args, delay: d, restante: d, nativo: 0, next: 0, ativo: true };
    registros.set(id, r);
    if (!dormindo) {
      agendarTimer(r);
    }
    return id;
  };
  window.clearInterval = function(id) {
    if (id === undefined) {
      return;
    }
    const r = registros.get(id);
    if (!r) {
      return;
    }
    r.ativo = false;
    if (r.nativo) {
      nClearInterval(r.nativo);
    }
    registros.delete(id);
  };
  function agendarTimer(r) {
    if (!r || !r.ativo || r.nativo || dormindo) {
      return;
    }
    if (r.tipo === "interval") {
      r.nativo = nSetInterval(function() {
        if (!r || !r.ativo) {
          return;
        }
        try {
          r.fn.apply(window, r.args);
        } catch (e) {}
      }, r.delay);
      return;
    }
    const d = Math.max(0, r.restante || 0);
    r.next = Date.now() + d;
    r.nativo = nSetTimeout(function() {
      if (!r) {
        return;
      }
      r.nativo = 0;
      if (!r.ativo) {
        return;
      }
      registros.delete(r.id);
      try {
        r.fn.apply(window, r.args);
      } catch (e) {}
    }, d);
  }
  window.requestAnimationFrame = function(fn) {
    const id = rafProximo++;
    rafMap.set(id, fn);
    if (!dormindo) {
      agendarRaf(id);
    }
    return id;
  };
  window.cancelAnimationFrame = function(id) {
    rafMap.delete(id);
    const n = rafNativos.get(id);
    if (n) {
      nCAF(n);
      rafNativos.delete(id);
    }
  };
  function agendarRaf(id) {
    const fn = rafMap.get(id);
    if (!fn) {
      return;
    }
    const n = nRAF(function(ts) {
      rafNativos.delete(id);
      if (!rafMap.has(id)) {
        return;
      }
      rafMap.delete(id);
      try {
        fn(ts);
      } catch (e) {}
    });
    rafNativos.set(id, n);
  }
  function pausarTimers() {
    registros.forEach(function(r) {
      if (!r.nativo) {
        return;
      }
      if (r.tipo === "interval") {
        nClearInterval(r.nativo);
      } else {
        nClearTimeout(r.nativo);
        r.restante = Math.max(0, (r.next || Date.now()) - Date.now());
      }
      r.nativo = 0;
    });
    rafNativos.forEach(function(n) {
      nCAF(n);
    });
    rafNativos.clear();
  }
  function retomarTimers() {
    registros.forEach(function(r) {
      agendarTimer(r);
    });
    rafMap.forEach(function(_fn, id) {
      agendarRaf(id);
    });
  }
  function podeDormir() {
    if (!ligado || dormindo) {
      return false;
    }
    if (document.readyState !== "complete") {
      return false;
    }
    if (document.hidden) {
      return false;
    }
    try {
      const ms = document.querySelectorAll("video,audio");
      for (let i = 0;i < ms.length; i++) {
        const m = ms[i];
        if (!m.paused && !m.ended) {
          return false;
        }
      }
      const ae = document.activeElement;
      if (ae && (ae.tagName === "INPUT" || ae.tagName === "TEXTAREA" || ae.tagName === "SELECT" || ae.isContentEditable)) {
        return false;
      }
      if (document.querySelector('[role="dialog"][aria-modal="true"]')) {
        return false;
      }
    } catch (e) {
      return false;
    }
    return true;
  }
  function avisar(tipo) {
    try {
      if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
        window.chrome.webview.postMessage(JSON.stringify({ t: tipo }));
      }
    } catch (e) {}
  }
  function dormir() {
    if (!ligado || dormindo) {
      return;
    }
    if (!podeDormir()) {
      reiniciarOcioso(5000);
      return;
    }
    dormindo = true;
    try {
      const ms = document.querySelectorAll("video,audio");
      for (let i = 0;i < ms.length; i++) {
        const m = ms[i];
        if (!m.paused) {
          m.pause();
        }
      }
    } catch (e) {}
    try {
      if (document.getAnimations) {
        const anims = document.getAnimations();
        for (let j = 0;j < anims.length; j++) {
          if (anims[j].playState === "running") {
            anims[j].pause();
            animPausadas.push(anims[j]);
          }
        }
      }
    } catch (e) {}
    pausarTimers();
    avisar("sleep");
  }
  function acordar() {
    if (!ligado) {
      return;
    }
    if (dormindo) {
      dormindo = false;
      for (let i = 0;i < animPausadas.length; i++) {
        try {
          animPausadas[i].play();
        } catch (e) {}
      }
      animPausadas = [];
      retomarTimers();
      avisar("wake");
      agendarDrenarIO();
    }
    reiniciarOcioso(ociosoMs);
  }
  function reiniciarOcioso(ms) {
    if (!ligado) {
      return;
    }
    if (idleTimer) {
      nClearTimeout(idleTimer);
    }
    idleTimer = nSetTimeout(function() {
      idleTimer = 0;
      if (!ligado) {
        return;
      }
      dormir();
    }, ms || ociosoMs);
  }
  function naBarraDeRolagem(ev) {
    try {
      const me = ev;
      const x = Number(me.clientX);
      const y = Number(me.clientY);
      if (!isFinite(x) || !isFinite(y)) {
        return false;
      }
      const se = document.scrollingElement || document.documentElement;
      if (se) {
        const sw = window.innerWidth - se.clientWidth;
        if (sw > 0 && se.scrollHeight > se.clientHeight && x >= se.clientWidth) {
          return true;
        }
        const sh = window.innerHeight - se.clientHeight;
        if (sh > 0 && se.scrollWidth > se.clientWidth && y >= se.clientHeight) {
          return true;
        }
      }
      let el = me.target;
      while (el && el.nodeType === 1) {
        if (el !== se && el !== document.documentElement && el !== document.body) {
          const cs = window.getComputedStyle(el);
          const oy = String(cs.overflowY || "");
          const ox = String(cs.overflowX || "");
          const podeV = (oy === "auto" || oy === "scroll" || oy === "overlay") && el.scrollHeight > el.clientHeight + 1;
          const podeH = (ox === "auto" || ox === "scroll" || ox === "overlay") && el.scrollWidth > el.clientWidth + 1;
          if (podeV || podeH) {
            const r = el.getBoundingClientRect();
            if (x >= r.left && x < r.right && y >= r.top && y < r.bottom) {
              const direita = r.left + (el.clientLeft || 0) + el.clientWidth;
              const base = r.top + (el.clientTop || 0) + el.clientHeight;
              if (podeV && x >= direita - 1) {
                return true;
              }
              if (podeH && y >= base - 1) {
                return true;
              }
            }
          }
        }
        el = el.parentElement;
      }
    } catch (e) {}
    return false;
  }
  function acordarComEvento(ev) {
    if (ev && (ev.type === "pointerdown" || ev.type === "mousedown")) {
      const me = ev;
      if (me.button !== 0) {
        return;
      }
      if (naBarraDeRolagem(ev)) {
        return;
      }
    }
    acordar();
  }
  const eventos = ["pointerdown", "mousedown", "touchstart", "keydown"];
  for (let i = 0;i < eventos.length; i++) {
    document.addEventListener(eventos[i], acordarComEvento, { capture: true, passive: true });
  }
  function bloquearScroll(ev) {
    if (!dormindo) {
      return;
    }
    try {
      ev.stopImmediatePropagation();
    } catch (e) {}
  }
  try {
    window.addEventListener("scroll", bloquearScroll, { capture: true, passive: true });
    document.addEventListener("scroll", bloquearScroll, { capture: true, passive: true });
    window.addEventListener("scrollend", bloquearScroll, { capture: true, passive: true });
    document.addEventListener("scrollend", bloquearScroll, { capture: true, passive: true });
    window.addEventListener("wheel", bloquearScroll, { capture: true, passive: true });
  } catch (e) {}
  window.__odSleepOn = function() {
    if (ligado) {
      reiniciarOcioso();
      return "on";
    }
    ligado = true;
    reiniciarOcioso();
    return "on";
  };
  window.__odSleepOff = function() {
    if (dormindo) {
      dormindo = false;
      for (let i = 0;i < animPausadas.length; i++) {
        try {
          animPausadas[i].play();
        } catch (e) {}
      }
      animPausadas = [];
      retomarTimers();
    }
    ligado = false;
    if (idleTimer) {
      nClearTimeout(idleTimer);
      idleTimer = 0;
    }
    avisar("wake");
    agendarDrenarIO();
    return "off";
  };
})();
