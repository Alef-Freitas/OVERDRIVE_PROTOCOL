//go:build windows

package main

import (
	"strings"
	"testing"
)

// msgImagemTeste monta um anexo completo para o teste.
func msgImagemTeste(nome, tipo, dados string) msgImagens {
	return msgImagens{
		T: "", N: "", Acao: "", I: 0, Nome: nome, Tipo: tipo, Dados: dados,
		Msg: "", Src: "", OK: 0, Falha: 0, Via: "", Total: 0, CB: 0,
	}
}

// TestGuardarImagemSemLimite garante que o envio pra IA nao rejeita pelo
// tamanho: data URL grande entra, prefixo invalido continua fora.
func TestGuardarImagemSemLimite(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so o anexo importa.
	aplicativo := &app{}
	grande := "data:image/jpeg;base64," + strings.Repeat("A", 7<<20)
	aplicativo.guardarDadoImagem(msgImagemTeste("g.jpg", "image/jpeg", grande))

	if len(aplicativo.imgLidas) != 1 {
		t.Fatalf("anexo grande devia entrar, deu %d", len(aplicativo.imgLidas))
	}

	//exhaustruct:ignore // App de teste: so o anexo importa.
	outro := &app{}
	outro.guardarDadoImagem(msgImagemTeste("x.bin", "x", strings.Repeat("B", 10)))

	if len(outro.imgLidas) != 0 {
		t.Error("sem prefixo data:image/ devia continuar fora")
	}
}
