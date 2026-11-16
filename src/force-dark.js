// force-dark: modo escuro via Dark Reader (tema dinamico, vendido em
// darkreader.js). Ele analisa o CSS da pagina e gera substituicoes
// estaticas: sem invert cego (que deixava foto e logo em negativo) e sem
// filtro no html (que repintava a pagina inteira a cada frame e estourava
// a CPU). Antes de ligar, mede a luminosidade do fundo: se a pagina ja e
// escura, pula fora.
(function () {
  if (window.__odForceDark) { return; }
  window.__odForceDark = true;
  window.__odForceDarkLigado = false;

  function motor() {
    try {
      return window.DarkReader || null;
    } catch (e) { return null; }
  }

  function lum(cor) {
    var m = /rgba?\(([^)]+)\)/.exec(cor || '');
    if (!m) { return 1; }
    var p = m[1].split(',');
    var r = +p[0], g = +p[1], b = +p[2];
    var a = p.length > 3 ? +p[3] : 1;
    var l = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
    return l * a + (1 - a) * 1;
  }

  function paginaClara() {
    try {
      var b = document.body, h = document.documentElement;
      var lb = b ? lum(getComputedStyle(b).backgroundColor) : 1;
      if (lb < 0.5) { return false; }
      var lh = h ? lum(getComputedStyle(h).backgroundColor) : 1;
      return lh >= 0.5;
    } catch (e) { return true; }
  }

  function aplicar() {
    if (!window.__odForceDarkLigado) { return 'off'; }
    var dr = motor();
    if (!dr) { return 'sem-motor'; }
    try {
      if (typeof dr.isEnabled === 'function' && dr.isEnabled()) { return 'ok'; }
    } catch (e) { /* verifica de novo abaixo */ }
    if (!paginaClara()) { return 'pulada'; }
    try {
      dr.enable({});
    } catch (e) { return 'erro'; }
    return 'ok';
  }

  window.__odForceDarkOn = function () {
    window.__odForceDarkLigado = true;
    return aplicar();
  };

  window.__odForceDarkOff = function () {
    window.__odForceDarkLigado = false;
    var dr = motor();
    if (dr) {
      try { dr.disable(); } catch (e) { /* sem motor */ }
    }
    return 'ok';
  };
})();
