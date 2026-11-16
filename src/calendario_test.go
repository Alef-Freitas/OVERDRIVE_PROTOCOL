//go:build windows

package main

import (
	"testing"
	"time"
)

// Constantes de teste para os dias/anos verificados (evita numeros magicos).
const (
	testAno2000      = 2000
	testAno2023      = 2023
	testAno2024      = 2024
	testAno2025      = 2025
	testAno2026      = 2026
	testDiaQuatro    = 4
	testDiaCinco     = 5
	testDiaNove      = 9
	testDiaDezoito   = 18
	testDiaVinteTres = 23
)

// TestPascoa verifica o algoritmo contra datas conhecidas da Pascoa.
func TestPascoa(t *testing.T) {
	t.Parallel()

	casos := []struct {
		ano int
		mes time.Month
		dia int
	}{
		{ano: testAno2000, mes: time.April, dia: testDiaVinteTres},
		{ano: testAno2023, mes: time.April, dia: testDiaNove},
		{ano: testAno2024, mes: time.March, dia: calDiasMes},
		{ano: testAno2025, mes: time.April, dia: calDiaVinte},
		{ano: testAno2026, mes: time.April, dia: testDiaCinco},
	}

	for _, caso := range casos {
		obtida := pascoa(caso.ano)
		if obtida.Year() != caso.ano || obtida.Month() != caso.mes || obtida.Day() != caso.dia {
			t.Errorf("pascoa(%d) = %s, esperado %d/%d/%d",
				caso.ano, obtida.Format("02/01/2006"), caso.dia, caso.mes, caso.ano)
		}
	}
}

// TestFeriadosMoveis verifica Carnaval, Sexta Santa e Corpus Christi de 2025.
func TestFeriadosMoveis(t *testing.T) {
	t.Parallel()

	moveis := feriadosMoveis(testAno2025)
	if len(moveis) != 3 {
		t.Fatalf("feriadosMoveis devolveu %d eventos, esperado 3", len(moveis))
	}

	esperados := []calendarioEvento{
		{dia: testDiaQuatro, mes: time.March, nome: "Carnaval", tipo: "FACULTATIVO", aniv: false},
		{dia: testDiaDezoito, mes: time.April, nome: "Sexta-Feira Santa", tipo: calTipoNacional, aniv: false},
		{dia: calDiaDezenove, mes: time.June, nome: "Corpus Christi", tipo: "BH/CE", aniv: false},
	}

	for indice, esperado := range esperados {
		obtido := moveis[indice]
		if obtido.dia != esperado.dia || obtido.mes != esperado.mes ||
			obtido.nome != esperado.nome || obtido.tipo != esperado.tipo || obtido.aniv {
			t.Errorf("movel[%d] = %+v, esperado %+v", indice, obtido, esperado)
		}
	}
}

// TestEventosCalendario verifica feriados fixos e aniversarios em dias conhecidos.
func TestEventosCalendario(t *testing.T) {
	t.Parallel()

	eventos := eventosCalendario(testAno2025)

	casos := []struct {
		mes  time.Month
		dia  int
		nome string
		aniv bool
	}{
		{mes: time.January, dia: calUm, nome: "Ano Novo", aniv: false},
		{mes: time.January, dia: calDiaDezessete, nome: "Analdina", aniv: true},
		{mes: time.April, dia: calDiaVinteUm, nome: "Aya", aniv: true},
		{mes: time.May, dia: calDiaTreze, nome: "Manuela", aniv: true},
		{mes: time.May, dia: calTrinta, nome: "Paula", aniv: true},
		{mes: time.December, dia: calDiaDezenove, nome: "Alef", aniv: true},
		{mes: time.December, dia: calDiaVinteCinco, nome: "Natal", aniv: false},
	}

	for _, caso := range casos {
		chave := [2]int{int(caso.mes), caso.dia}
		lista := eventos[chave]

		if len(lista) == 0 {
			t.Errorf("nenhum evento em %d/%d", caso.dia, caso.mes)

			continue
		}

		encontrado := false

		for _, evento := range lista {
			if evento.nome == caso.nome && evento.aniv == caso.aniv {
				encontrado = true

				break
			}
		}

		if !encontrado {
			t.Errorf("evento %q (aniv=%t) ausente em %d/%d: %+v", caso.nome, caso.aniv, caso.dia, caso.mes, lista)
		}
	}
}

// TestSobreControleRelogio garante que a area do relogio e cliente (clicavel),
// e nao caption (que viraria arrasto da janela).
func TestSobreControleRelogio(t *testing.T) {
	t.Parallel()

	aplicativo := &app{tabH: 30, winClock: rectT{100, 0, 200, 30}} //nolint:exhaustruct_v5 // Hit-test apenas.

	if !aplicativo.sobreControle(150, 10) {
		t.Error("sobreControle deve reconhecer a area do relogio")
	}

	if aplicativo.sobreControle(50, 10) {
		t.Error("sobreControle nao deve reconhecer ponto fora do relogio")
	}
}

// TestEventosCalendarioMoveisPorAno garante que os moveis acompanham o ano.
func TestEventosCalendarioMoveisPorAno(t *testing.T) {
	t.Parallel()

	eventos2024 := eventosCalendario(testAno2024)
	chave2024 := [2]int{int(time.February), calDiaTreze}

	if _, ok := eventos2024[chave2024]; !ok {
		t.Errorf("Carnaval de 2024 (13/02) ausente")
	}

	eventos2025 := eventosCalendario(testAno2025)
	chave2025 := [2]int{int(time.March), testDiaQuatro}

	if _, ok := eventos2025[chave2025]; !ok {
		t.Errorf("Carnaval de 2025 (04/03) ausente")
	}
}
