//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Animacoes: easing do Marble.

package main

// Animacoes suaves de posicao dos cards (Tidy, spawn), como o Marble.
// Interpolacao linear com easing ease-out cubico.

import (
	"time"
)

const (
	marbleAnimMs  = 260
	marbleAnimFps = 30
)

type destinoAnim struct {
	indice      int
	inicio      rectT
	fim         rectT
	inicioTempo time.Time
}

//nolint:gochecknoglobals // Animacao: estado de UI single-window.
var marbleAnimacoes []destinoAnim

func easeOutCubico(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)
}

func marbleAnimarPara(indice int, origem, destino rectT) {
	if indice < 0 || indice >= len(espCanvas.cartoes) {
		return
	}
	marbleAnimacoes = append(marbleAnimacoes, destinoAnim{
		indice: indice, inicio: origem, fim: destino, inicioTempo: time.Now(),
	})
}

// marbleAtualizarAnimacoes aplica as animacoes em curso e devolve true se
// ainda ha animacao rodando (para o chamador agendar o proximo frame).
func marbleAtualizarAnimacoes() bool {
	if len(marbleAnimacoes) == 0 {
		return false
	}

	agora := time.Now()
	restam := marbleAnimacoes[:0]

	for _, anim := range marbleAnimacoes {
		if anim.indice < 0 || anim.indice >= len(espCanvas.cartoes) {
			continue
		}

		decorrido := agora.Sub(anim.inicioTempo).Milliseconds()
		if decorrido >= marbleAnimMs {
			espCanvas.cartoes[anim.indice].rect = anim.fim

			continue
		}

		t := float64(decorrido) / float64(marbleAnimMs)
		fator := easeOutCubico(t)
		card := &espCanvas.cartoes[anim.indice]
		card.rect = rectT{
			lerp(anim.inicio.Left, anim.fim.Left, fator),
			lerp(anim.inicio.Top, anim.fim.Top, fator),
			lerp(anim.inicio.Right, anim.fim.Right, fator),
			lerp(anim.inicio.Bottom, anim.fim.Bottom, fator),
		}
		restam = append(restam, anim)
	}

	marbleAnimacoes = restam

	return len(marbleAnimacoes) > 0
}

func lerp(inicio, fim int32, fator float64) int32 {
	return inicio + int32(float64(fim-inicio)*fator)
}
