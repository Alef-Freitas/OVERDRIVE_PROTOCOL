//go:build windows

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAlvoSenha(t *testing.T) {
	t.Parallel()

	alvo, ok := alvoSenha("Exemplo.com")
	if !ok || alvo != "Overdrive/senha/exemplo.com" {
		t.Errorf("alvo errado: %q %v", alvo, ok)
	}

	for _, ruim := range []string{"", ".", "..", "a/b", `a\b`, strings.Repeat("x", 300)} {
		if _, ok := alvoSenha(ruim); ok {
			t.Errorf("dominio %q devia reprovar", ruim)
		}
	}
}

func TestDominioAlvoSenha(t *testing.T) {
	t.Parallel()

	dominio, ok := dominioAlvoSenha("Overdrive/senha/site-teste.com")
	if !ok || dominio != "site-teste.com" {
		t.Errorf("parse errado: %q %v", dominio, ok)
	}

	if _, ok := dominioAlvoSenha("Overdrive/outro/x"); ok {
		t.Error("prefixo trocado devia reprovar")
	}

	if _, ok := dominioAlvoSenha("Overdrive/senha/.."); ok {
		t.Error("traversal devia reprovar")
	}
}

func TestSenhaValida(t *testing.T) {
	t.Parallel()

	if !senhaValida("eu", "segredo") {
		t.Error("par normal devia passar")
	}

	if !senhaValida("", "segredo") {
		t.Error("sem usuario devia passar (PIN)")
	}

	if senhaValida("eu", "") {
		t.Error("sem senha devia reprovar")
	}

	if senhaValida(strings.Repeat("u", 300), "x") {
		t.Error("usuario gigante devia reprovar")
	}

	if senhaValida("eu", strings.Repeat("p", 2000)) {
		t.Error("senha gigante devia reprovar")
	}
}

func TestBlobSenhaIdaVolta(t *testing.T) {
	t.Parallel()

	blob := blobSenha("eu@site-teste.com", "s3gr3d0!")
	if blob == "" {
		t.Fatal("blob vazio")
	}

	cred, ok := abrirBlobSenha(blob)
	if !ok || cred.Usuario != "eu@site-teste.com" || cred.Segredo != "s3gr3d0!" {
		t.Errorf("volta errada: %+v %v", cred, ok)
	}

	if _, ok := abrirBlobSenha("lixo"); ok {
		t.Error("lixo devia reprovar")
	}

	if _, ok := abrirBlobSenha(blobSenha("u", "")); ok {
		t.Error("sem segredo devia reprovar")
	}
}

func TestFormatoPerguntaSenha(t *testing.T) {
	t.Parallel()

	comUsuario := formatoPerguntaSenha("salvar senha de %s?", "x.com", "eu")
	if !strings.Contains(comUsuario, "x.com") || !strings.Contains(comUsuario, "eu") {
		t.Errorf("pergunta incompleta: %q", comUsuario)
	}

	semUsuario := formatoPerguntaSenha("salvar senha de %s?", "x.com", "")
	if !strings.Contains(semUsuario, "x.com") {
		t.Errorf("pergunta incompleta: %q", semUsuario)
	}
}

func TestDominioIdSenha(t *testing.T) {
	t.Parallel()

	dominio, ok := dominioIDSenha("senhas:apagar:site-teste.com", "senhas:apagar:")
	if !ok || dominio != "site-teste.com" {
		t.Errorf("parse errado: %q %v", dominio, ok)
	}

	if _, ok := dominioIDSenha("senhas:apagar:../x", "senhas:apagar:"); ok {
		t.Error("traversal devia reprovar")
	}

	if _, ok := dominioIDSenha("outro:site-teste.com", "senhas:apagar:"); ok {
		t.Error("prefixo trocado devia reprovar")
	}
}

func TestScriptPreencherSenhaEscapa(t *testing.T) {
	t.Parallel()

	script := scriptPreencherSenha("eu\",x", "p\na")
	if strings.Contains(script, "eu\",x") || strings.Contains(script, "p\na") {
		t.Error("segredo cru vazou no JS")
	}

	if !strings.Contains(script, "querySelector") || !strings.Contains(script, "dispatchEvent") {
		t.Error("script sem preenchimento")
	}
}

func TestCofreLeituraAusente(t *testing.T) {
	t.Parallel()

	if _, achou := credLer("Overdrive/senha/inexistente-teste-xyz"); achou {
		t.Error("ausente devia dar false")
	}

	if _, achou := credLer(""); achou {
		t.Error("vazio devia dar false")
	}

	if lista := credListar("Overdrive/senha-inexistente-xyz/"); len(lista) != 0 {
		t.Errorf("filtro vazio devia dar nil, deu %v", lista)
	}

	if lista := credListar(""); lista != nil {
		t.Error("prefixo vazio devia dar nil")
	}

	if err := credApagar(""); err == nil {
		t.Error("apagar vazio devia falhar")
	}
}

func TestMsgSenhasAcoes(t *testing.T) {
	t.Parallel()

	var pedido msgSenhas
	if err := json.Unmarshal([]byte(`{"acao":"salvar","usuario":"eu","senha":"s"}`), &pedido); err != nil {
		t.Fatal(err)
	}

	if pedido.Acao != "salvar" || pedido.Usuario != "eu" || pedido.Senha != "s" {
		t.Errorf("parse errado: %+v", pedido)
	}
}
