//go:build windows

package main

import "testing"

const (
	testMenuX        = 800
	testMenuY        = 200
	testSubOffX      = 190
	testSubOffY      = 12
	testLarguraMenu  = 200
	testLarguraSub   = 180
	testLarguraSub2  = 140
	testMargem       = 4
	testSobreposicao = 6
	testAreaDireita  = 1920
)

// TestOrigemTelaNivel garante que submenus sao ancorados na janela pai certa,
// sem somar duas vezes a origem do menu raiz.
func TestOrigemTelaNivel(t *testing.T) {
	t.Parallel()

	menu.dx, menu.dy = testMenuX, testMenuY
	menuSub.dx, menuSub.dy = testSubOffX, testSubOffY

	raiz := nivelMenu{anca: &menu} //nolint:exhaustruct_v5 // Apenas a ancora importa.
	if x, y := origemTelaNivel(raiz); x != testMenuX || y != testMenuY {
		t.Errorf("raiz: (%d,%d), esperado (%d,%d)", x, y, testMenuX, testMenuY)
	}

	sub1 := nivelMenu{anca: &menuSub} //nolint:exhaustruct_v5 // Apenas a ancora importa.
	if x, y := origemTelaNivel(sub1); x != testMenuX+testSubOffX || y != testMenuY+testSubOffY {
		t.Errorf("sub1: (%d,%d), esperado (%d,%d)",
			x, y, testMenuX+testSubOffX, testMenuY+testSubOffY)
	}
}

// TestPosXNivelMenu garante que o nivel 1 abre a direita e o nivel 2 a esquerda.
func TestPosXNivelMenu(t *testing.T) {
	t.Parallel()

	menu.w = testLarguraMenu
	menuSub.w = testLarguraSub
	menuSub2.w = testLarguraSub2

	nivel1 := nivelMenu{anca: &menu, janela: &menuSub} //nolint:exhaustruct_v5 // So a posicao importa.

	const esperado1 int32 = testMenuX + testLarguraMenu - testSobreposicao
	if x := posXNivelMenu(nivel1, testMenuX, testMargem, testSobreposicao, 0, testAreaDireita); x != esperado1 {
		t.Errorf("nivel 1: x=%d, esperado %d", x, esperado1)
	}

	nivel2 := nivelMenu{anca: &menuSub, janela: &menuSub2} //nolint:exhaustruct_v5 // So a posicao importa.

	const esperado2 int32 = testMenuX - testLarguraSub2 + testSobreposicao
	if x := posXNivelMenu(nivel2, testMenuX, testMargem, testSobreposicao, 0, testAreaDireita); x != esperado2 {
		t.Errorf("nivel 2: x=%d, esperado %d (a esquerda do pai)", x, esperado2)
	}
}
