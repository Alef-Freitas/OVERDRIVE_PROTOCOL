// livre-clique: botao direito e copiar/colar livres no documento atual.
//
// Tecnica: listeners em fase de captura registrados no document_start
// (antes dos scripts da pagina) que chamam stopImmediatePropagation sem
// preventDefault. O padrao (menu, copia, selecao) continua liberado, mas os
// bloqueios da pagina nunca executam — e como nunca executam, a pagina nao
// tem como observar diferenca: nenhum flag dela e setado, nenhum
// preventDefault dela e chamado. O JS da pagina nao enumera listeners
// adicionados por addEventListener.
(function () {
  if (window.__odLivreInit) { return; }
  window.__odLivreInit = true;
  window.__odLivreLigado = false;

  var ATIVOS = ['contextmenu', 'copy', 'cut', 'paste', 'selectstart'];
  var ESTILO_ID = '__odLivreEstilo';
  var obs = null;

  function neutralizar(e) {
    try { e.stopImmediatePropagation(); } catch (err) { /* sem padrao propio */ }
  }

  function garantirEstilo() {
    var st = document.getElementById(ESTILO_ID);
    if (!st) {
      st = document.createElement('style');
      st.id = ESTILO_ID;
      st.textContent = 'html body *{user-select:text!important;-webkit-user-select:text!important}';
      (document.head || document.documentElement).appendChild(st);
    }
  }

  function limparPropriedades() {
    var props = ['oncontextmenu', 'oncopy', 'oncut', 'onpaste', 'onselectstart'];
    for (var d = 0; d < props.length; d++) {
      try { if (window[props[d]]) { window[props[d]] = null; } } catch (e1) {}
      try { if (document[props[d]]) { document[props[d]] = null; } } catch (e2) {}
      try { if (document.body && document.body[props[d]]) { document.body[props[d]] = null; } } catch (e3) {}
    }
  }

  window.__odLivreOn = function () {
    if (window.__odLivreLigado) { garantirEstilo(); return 'ok'; }
    window.__odLivreLigado = true;
    for (var i = 0; i < ATIVOS.length; i++) {
      window.addEventListener(ATIVOS[i], neutralizar, true);
      document.addEventListener(ATIVOS[i], neutralizar, true);
    }
    limparPropriedades();
    garantirEstilo();
    try {
      if (window.MutationObserver && !obs) {
        obs = new MutationObserver(function () {
          if (window.__odLivreLigado) { garantirEstilo(); }
        });
        obs.observe(document.documentElement, { childList: true, subtree: true });
      }
    } catch (e4) { /* sem observador */ }
    return 'ok';
  };

  window.__odLivreOff = function () {
    window.__odLivreLigado = false;
    for (var j = 0; j < ATIVOS.length; j++) {
      try { window.removeEventListener(ATIVOS[j], neutralizar, true); } catch (e5) {}
      try { document.removeEventListener(ATIVOS[j], neutralizar, true); } catch (e6) {}
    }
    try {
      if (obs) { obs.disconnect(); obs = null; }
      var st2 = document.getElementById(ESTILO_ID);
      if (st2 && st2.parentNode) { st2.parentNode.removeChild(st2); }
    } catch (e7) { /* sem limpeza */ }
    return 'ok';
  };
})();
