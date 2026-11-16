//go:build windows

package main

import (
	"strings"
	"testing"
)

// TestToggleForceDark garante o liga/desliga sem painel nem janela.
func TestToggleForceDark(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so o interruptor importa.
	aplicativo := &app{}

	aplicativo.toggleForceDark()

	if !aplicativo.st.ForceDark {
		t.Fatal("primeiro toque devia ligar")
	}

	aplicativo.toggleForceDark()

	if aplicativo.st.ForceDark {
		t.Error("segundo toque devia desligar")
	}

	// Sem pane pronto nao faz nada nem da panico.
	aplicativo.st.ForceDark = true
	aplicativo.aplicarForceDark()
	aplicativo.sincronizarForceDark(nil)
	aplicativo.desativarForceDarkNoPane(nil)
	aplicativo.ativarForceDarkNoPane(nil)
}

// TestScrollbarFixSoBarra trava o escopo do script de rolagem: ele pinta
// so a barra. Fundo forcado quebra sites claros e engana o detector do
// modo escuro (fundo preto = "pagina ja escura", inversao nunca liga).
func TestScrollbarFixSoBarra(t *testing.T) {
	t.Parallel()

	if strings.Contains(scrollbarFixJS, "background-color") {
		t.Error("script de rolagem nao devia forcar fundo nas paginas")
	}

	if strings.Contains(scrollbarFixJS, "color-scheme") {
		t.Error("script de rolagem nao devia forcar color-scheme nas paginas")
	}

	if !strings.Contains(scrollbarFixJS, "webkit-scrollbar") {
		t.Error("script devia estilizar a barra de rolagem")
	}
}

// TestForceDarkMotorDinamico trava o anti-negativo: o modo escuro usa o
// motor dinamico (Dark Reader vendido) em vez de invert cego, que deixava
// foto e logo em negativo e estourava a CPU com filtro no html.
func TestForceDarkMotorDinamico(t *testing.T) {
	t.Parallel()

	if !strings.Contains(forceDarkJS, "DarkReader") {
		t.Error("script devia usar o motor dinamico")
	}

	if !strings.Contains(forceDarkJS, "dr.enable") {
		t.Error("ligar devia ativar o motor")
	}

	if !strings.Contains(forceDarkJS, "dr.disable") {
		t.Error("desligar devia desativar o motor")
	}

	if strings.Contains(forceDarkJS, "invert(1)") {
		t.Error("script nao devia inverter a pagina no cego")
	}

	if strings.Contains(forceDarkJS, "color-scheme:") {
		t.Error("script nao devia forcar color-scheme junto com o motor")
	}
}

// TestMotorDarkReaderVendido garante o bundle do motor presente.
func TestMotorDarkReaderVendido(t *testing.T) {
	t.Parallel()

	if len(darkReaderJS) < 100000 {
		t.Errorf("bundle devia ter mais de 100KB, tem %d bytes", len(darkReaderJS))
	}

	if !strings.Contains(darkReaderJS, "isEnabled") {
		t.Error("bundle devia expor a API do motor")
	}
}
