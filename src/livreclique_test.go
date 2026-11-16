//go:build windows

package main

import (
	"testing"
)

// TestToggleLivreClique garante o liga/desliga sem painel nem janela.
func TestToggleLivreClique(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so o interruptor importa.
	aplicativo := &app{}

	aplicativo.toggleLivreClique()

	if !aplicativo.st.LivreClique {
		t.Fatal("primeiro toque devia ligar")
	}

	aplicativo.toggleLivreClique()

	if aplicativo.st.LivreClique {
		t.Error("segundo toque devia desligar")
	}

	// Sem pane pronto nao faz nada nem da panico.
	aplicativo.st.LivreClique = true
	aplicativo.aplicarLivreClique()
	aplicativo.sincronizarLivreClique(nil)
	aplicativo.desativarLivreCliqueNoPane(nil)
	aplicativo.ativarLivreCliqueNoPane(nil)
}
