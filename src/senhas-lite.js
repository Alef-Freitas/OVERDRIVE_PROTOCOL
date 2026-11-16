// senhas-lite.ts — captura login salvo e avisa campo de senha.
// Roda em toda pagina via AddInitScript; nativo filtra internas.
(function () {
  "use strict";

  var MAX_USUARIO = 256;
  var MAX_SENHA = 1024;
  var JA_AVIsOU = "__odSenhasTemCampo";

  function podePostar() {
    try {
      return !!(window.chrome && window.chrome.webview && window.chrome.webview.postMessage);
    } catch (e) {
      return false;
    }
  }

  function postar(obj) {
    try {
      window.chrome.webview.postMessage(JSON.stringify(obj));
    } catch (e) { /* sem pagina util: ignora */ }
  }

  function corta(s, max) {
    s = String(s || "");
    if (s.length > max) {
      s = s.slice(0, max);
    }

    return s;
  }

  function visivel(c) {
    try {
      return !!c && !c.disabled && c.offsetParent !== null;
    } catch (e) {
      return false;
    }
  }

  function campoSenha(raiz) {
    try {
      var campos = (raiz || document).querySelectorAll('input[type="password"]');

      for (var i = 0; i < campos.length; i++) {
        if (visivel(campos[i])) {
          return campos[i];
        }
      }
    } catch (e) { /* seletor indisponivel */ }

    return null;
  }

  function campoUsuario(raiz, senha) {
    try {
      var escopo = (senha && senha.form) || raiz || document;
      var candidatos = escopo.querySelectorAll(
        'input[type="email"],input[type="text"],input:not([type])'
      );

      for (var i = 0; i < candidatos.length; i++) {
        var c = candidatos[i];
        if (c && !c.disabled && c !== senha && c.offsetParent !== null && c.value) {
          return c;
        }
      }
    } catch (e) { /* seletor indisponivel */ }

    return null;
  }

  function avisarTemCampo() {
    if (!podePostar()) {
      return;
    }

    try {
      if (document[JA_AVIsOU]) {
        return;
      }

      if (campoSenha(document)) {
        document[JA_AVIsOU] = true;
        postar({ t: "senhas", acao: "temcampo" });
      }
    } catch (e) { /* ignora */ }
  }

  function enviarAtual(origem) {
    if (!podePostar()) {
      return;
    }

    var senha = campoSenha(document);
    if (!senha || !senha.value) {
      return;
    }

    var usuario = campoUsuario(document, senha);
    var textoUsuario = usuario ? usuario.value : "";

    postar({
      t: "senhas",
      acao: "salvar",
      origem: origem || "",
      usuario: corta(textoUsuario, MAX_USUARIO),
      senha: corta(senha.value, MAX_SENHA)
    });
  }

  function aoEnviar(evento) {
    var alvo = null;
    try {
      alvo = evento && evento.target;
    } catch (e) {
      return;
    }

    var raiz = document;
    if (alvo && alvo.tagName === "FORM") {
      raiz = alvo;
    } else if (alvo && alvo.form) {
      raiz = alvo.form;
    }

    var senha = campoSenha(raiz) || campoSenha(document);
    if (!senha || !senha.value) {
      return;
    }

    var usuario = campoUsuario(raiz, senha);
    enviarAtual("submit");
  }

  function aoClicar(evento) {
    // Qualquer clique com senha preenchida vale: botoes de login
    // costumam ser divs, nao submit (ex.: Google). O nativo filtra
    // repetidos e respeita o "nao" da sessao.
    try {
      if (!evento) {
        return;
      }
    } catch (e) {
      return;
    }

    var senha = campoSenha(document);
    if (!senha || !senha.value) {
      return;
    }

    enviarAtual("clique");
  }

  function aoSair() {
    var senha = campoSenha(document);
    if (!senha || !senha.value) {
      return;
    }

    enviarAtual("saida");
  }

  function aoTeclar(evento) {
    try {
      if (!evento || evento.key !== "Enter") {
        return;
      }

      var alvo = evento.target;
      if (!alvo || alvo.type !== "password") {
        return;
      }

      if (!alvo.value) {
        return;
      }

      enviarAtual("enter");
    } catch (e) { /* ignora */ }
  }

  function observarCamposTardios() {
    try {
      if (typeof MutationObserver !== "function") {
        return;
      }

      var pendente = false;
      var observador = new MutationObserver(function () {
        if (pendente) {
          return;
        }

        pendente = true;
        setTimeout(function () {
          pendente = false;
          avisarTemCampo();
        }, 800);
      });

      observador.observe(document.documentElement, { childList: true, subtree: true });
    } catch (e) { /* pagina restrita: ignora */ }
  }

  try {
    document.addEventListener("submit", aoEnviar, true);
    document.addEventListener("click", aoClicar, true);
    document.addEventListener("keydown", aoTeclar, true);
    window.addEventListener("pagehide", aoSair, true);

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", function () {
        avisarTemCampo();
        observarCamposTardios();
      }, { once: true });
    } else {
      avisarTemCampo();
      observarCamposTardios();
    }
  } catch (e) { /* pagina restrita: ignora */ }
})();
