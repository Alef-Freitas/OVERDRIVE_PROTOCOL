//go:build windows

package main

import (
	"sync"
	"testing"
)

// mutexTestesContas serializa os testes que tocam o login global.
//
//nolint:gochecknoglobals // Teste paralelo exige mutex unico (race real).
var mutexTestesContas sync.Mutex

func TestDominioDeURL(t *testing.T) {
	t.Parallel()

	casos := []struct {
		entrada string
		quero   string
	}{
		{"https://Chat.DeepSeek.com/a", "chat.deepseek.com"},
		{"http://exemplo.com:8080/x", "exemplo.com"},
		{"", ""},
		{"::invalida::", ""},
	}

	for _, c := range casos {
		if got := dominioDeURL(c.entrada); got != c.quero {
			t.Errorf("dominio(%q) = %q, quero %q", c.entrada, got, c.quero)
		}
	}
}

func TestEsquemaPaywallValido(t *testing.T) {
	t.Parallel()

	if !esquemaPaywallValido("https") || !esquemaPaywallValido("http") {
		t.Error("http(s) deviam passar")
	}

	for _, esquema := range []string{"", "ftp", "file", "HTTPS", "chrome"} {
		if esquemaPaywallValido(esquema) {
			t.Errorf("esquema %q devia reprovar", esquema)
		}
	}
}

func TestHostProibidoPaywall(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"localhost", "::1", "127.0.0.1", "127.9.9.9"} {
		if !hostProibidoPaywall(host) {
			t.Errorf("host %q devia bloquear", host)
		}
	}

	for _, host := range []string{"", "exemplo.com", "1270.0.0.1", "localhost.evil.com"} {
		if hostProibidoPaywall(host) {
			t.Errorf("host %q devia passar", host)
		}
	}
}

func TestMenuLinhaDesligada(t *testing.T) {
	t.Parallel()

	item := itemExtUI{Base: "plug", Nome: "Plug"}

	linha := menuLinhaDesligada(item)
	if linha.id != "extligar:plug" || linha.label != "Plug" {
		t.Errorf("linha desligada errada: %+v", linha)
	}

	if linha.kind == miHeader || linha.kind == miSep {
		t.Error("linha clicavel nao devia ser cabecalho nem separador")
	}
}

func TestEstadoDoCliqueYouTube(t *testing.T) {
	t.Parallel()

	if got := estadoDoCliqueYouTube(false, false); got != ytAbrirCheio {
		t.Errorf("fechado devia abrir, deu %v", got)
	}

	if got := estadoDoCliqueYouTube(true, false); got != ytOcultar {
		t.Errorf("visivel devia ocultar, deu %v", got)
	}

	if got := estadoDoCliqueYouTube(true, true); got != ytMostrar {
		t.Errorf("oculto devia mostrar, deu %v", got)
	}
}

func TestPrenderIndiceConta(t *testing.T) {
	t.Parallel()
	mutexTestesContas.Lock()

	defer mutexTestesContas.Unlock()

	anterior := loginAtual
	defer func() { loginAtual = anterior }()

	loginAtual.Contas = []contaLogin{{Apelido: "a"}, {Apelido: "b"}}

	if got := prenderIndiceConta(1); got != 1 {
		t.Errorf("indice valido devia passar, deu %d", got)
	}

	if got := prenderIndiceConta(-1); got != 0 {
		t.Errorf("negativo devia prender em 0, deu %d", got)
	}

	if got := prenderIndiceConta(99); got != 0 {
		t.Errorf("estouro devia prender em 0, deu %d", got)
	}

	loginAtual.Contas = nil

	if got := prenderIndiceConta(0); got != 0 {
		t.Errorf("sem contas devia dar 0, deu %d", got)
	}
}

func TestProximaConta(t *testing.T) {
	t.Parallel()
	mutexTestesContas.Lock()

	defer mutexTestesContas.Unlock()

	anterior := loginAtual
	defer func() { loginAtual = anterior }()

	loginAtual.Contas = nil
	loginAtual.Atual = 0

	if got := proximaConta(); got != 0 {
		t.Errorf("sem contas devia dar 0, deu %d", got)
	}

	loginAtual.Contas = []contaLogin{{Apelido: "a"}, {Apelido: "b"}, {Apelido: "c"}}
	loginAtual.Atual = 0

	if got := proximaConta(); got != 1 {
		t.Errorf("0 devia ir para 1, deu %d", got)
	}

	loginAtual.Atual = 2

	if got := proximaConta(); got != 0 {
		t.Errorf("ultima devia girar para 0, deu %d", got)
	}

	loginAtual.Atual = 99

	if got := proximaConta(); got != 1 {
		t.Errorf("atual invalido devia reancorar e ir para 1, deu %d", got)
	}
}

func TestContaAtiva(t *testing.T) {
	t.Parallel()
	mutexTestesContas.Lock()

	defer mutexTestesContas.Unlock()

	anterior := loginAtual
	defer func() { loginAtual = anterior }()

	loginAtual.Contas = nil
	loginAtual.Email = ""
	loginAtual.Senha = ""

	if _, temConta := contaAtiva(); temConta {
		t.Error("sem nada nao devia ter conta")
	}

	loginAtual.Email = "eu@exemplo.com"

	conta, temConta := contaAtiva()
	if !temConta || conta.Email != "eu@exemplo.com" {
		t.Errorf("email avulso devia valer, deu %+v %v", conta, temConta)
	}

	loginAtual.Contas = []contaLogin{{Apelido: "a", Email: "a@x.com"}}
	loginAtual.Atual = 5

	conta, temConta = contaAtiva()
	if !temConta || conta.Apelido != "a" {
		t.Errorf("atual invalido devia cair na primeira, deu %+v %v", conta, temConta)
	}
}
