'use strict';

// scroll-lite.ts
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
  if (window.__odScrollLite) {
    return;
  }
  window.__odScrollLite = true;
  let ultimo = 0;
  let pendente = 0;
  function enviar() {
    pendente = 0;
    ultimo = Date.now();
    try {
      if (window.chrome && window.chrome.webview && window.chrome.webview.postMessage) {
        window.chrome.webview.postMessage(JSON.stringify({ t: "scroll" }));
      }
    } catch (e) {}
  }
  function aoRolar() {
    const agora = Date.now();
    if (agora - ultimo < 30) {
      return;
    }
    if (agora - ultimo < 150) {
      if (!pendente) {
        pendente = window.setTimeout(enviar, 150 - (agora - ultimo));
      }
      return;
    }
    if (pendente) {
      window.clearTimeout(pendente);
      pendente = 0;
    }
    enviar();
  }
  window.addEventListener("scroll", aoRolar, { capture: true, passive: true });
  document.addEventListener("scroll", aoRolar, { capture: true, passive: true });
})();
