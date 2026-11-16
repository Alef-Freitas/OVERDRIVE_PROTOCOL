//go:build windows

package main

import (
	"testing"
)

func TestCofreProvaViva(t *testing.T) {
	t.Parallel()

	const alvo = prefixoAlvoSenha + "prova-teste-xyz"

	_ = credApagar(alvo)

	blob := blobSenha("eu", "s3gr3d0!")
	if err := credEscrever(alvo, blob); err != nil {
		t.Fatalf("escrever falhou: %v", err)
	}

	lido, ok := credLer(alvo)
	if !ok || lido != blob {
		t.Fatalf("leitura errada: %q %v", lido, ok)
	}

	achou := false

	for _, candidato := range credListar(prefixoAlvoSenha) {
		if candidato == alvo {
			achou = true
		}
	}

	if !achou {
		t.Fatal("prova nao listada")
	}

	if err := credApagar(alvo); err != nil {
		t.Fatalf("apagar falhou: %v", err)
	}

	if _, ok := credLer(alvo); ok {
		t.Fatal("apagada ainda legivel")
	}
}
