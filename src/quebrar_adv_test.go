//go:build windows

package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestQuebrarTextosAdversos estressa o fatiamento com muros sem espaco.
// Toda linha tem que caber na largura e o corte usa reticencias.
func TestQuebrarTextosAdversos(t *testing.T) {
	t.Parallel()

	const larguraMax = 326

	const maxLinhas = 2

	const (
		textoCjkMuro = "日本語テスト漢字" //nolint:gosmopolitan // CJK exercita o corte.
		ideogramaHan = "漢"        //nolint:gosmopolitan // CJK exercita o corte.
	)

	medir := func(candidato string) int32 {
		var total int32

		for _, simbolo := range candidato {
			if simbolo > 0x2FFF {
				total += 14
			} else {
				total += 7
			}
		}

		return total
	}

	casos := map[string]string{
		"palavra 20k":  strings.Repeat("x", 20000),
		"url gigante":  "https://" + strings.Repeat("a", 5000) + ".com/" + strings.Repeat("b", 5000),
		"base64":       strings.Repeat("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eHk=", 400),
		"cjk muro":     strings.Repeat(textoCjkMuro, 2000),
		"emoji muro":   strings.Repeat("🎉🔥🚀", 2000),
		"misto":        "olha " + strings.Repeat("z", 3000) + " fim " + strings.Repeat(ideogramaHan, 1000),
		"runa gigante": strings.Repeat("a", 9999) + " ",
	}

	for nome, texto := range casos {
		limpo := strings.Join(strings.Fields(texto), " ")
		linhas := quebrarComMedida(limpo, larguraMax, maxLinhas, medir)

		if len(linhas) == 0 {
			t.Errorf("%s: nenhuma linha", nome)

			continue
		}

		if len(linhas) > maxLinhas {
			t.Errorf("%s: %d linhas", nome, len(linhas))
		}

		for i, linha := range linhas {
			if w := medir(linha); w > larguraMax {
				t.Errorf("%s linha %d: %dpx > %dpx (runas=%d)", nome, i, w, larguraMax, utf8.RuneCountInString(linha))
			}
		}

		if ultima := linhas[len(linhas)-1]; !strings.HasSuffix(ultima, "…") {
			t.Errorf("%s: corte sem reticencias", nome)
		}
	}
}
