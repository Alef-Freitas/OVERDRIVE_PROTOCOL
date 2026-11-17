//go:build windows

//nolint:wsl_v5,goconst // Testes de UI: espacamento e constantes repetidas sao intencionais.

package main

import "testing"

func TestMarbleNomeSite(t *testing.T) {
	t.Parallel()

	casos := map[string]string{
		"https://www.youtube.com/watch?v=abc": "youtube.com",
		"http://chat.deepseek.com/":           "deepseek.com",
		"deepseek.com":                        "deepseek.com",
		"https://sub.dominio.com.br/caminho":  "com.br",
	}

	for entrada, esperado := range casos {
		if obtido := marbleNomeSite(entrada); obtido != esperado {
			t.Errorf("marbleNomeSite(%q) = %q, esperado %q", entrada, obtido, esperado)
		}
	}
}
