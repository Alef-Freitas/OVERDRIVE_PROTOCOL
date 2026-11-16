//go:build windows

package main

import (
	"testing"
	"unicode"
	"unicode/utf8"
)

// rotuloPrimeiroRune diz se o rotulo comeca bem: primeira letra maiuscula;
// digito ou simbolo passam (setas, atalhos etc). Vazio passa.
func rotuloComecaBem(rotulo string) bool {
	letra, _ := utf8.DecodeRuneInString(rotulo)
	if letra == utf8.RuneError {
		return true
	}

	if unicode.IsLetter(letra) {
		return unicode.IsUpper(letra)
	}

	return true
}

// coletarRotulos junta os rotulos clicaveis e de cabecalho, descendo submenus.
func coletarRotulos(itens []menuItem, destino *[]string) {
	for _, item := range itens {
		switch item.kind {
		case miItem, miCheck, miRadio, miSub, miHeader:
			*destino = append(*destino, item.label)
		}

		coletarRotulos(item.sub, destino)
	}
}

// TestRotulosMaiuscula trava a regra: todo rotulo de menu comeca com
// maiuscula (mensagens de status continuam minusculas por estilo).
func TestRotulosMaiuscula(t *testing.T) {
	t.Parallel()

	//exhaustruct:ignore // App de teste: so os rotulos importam.
	aplicativo := &app{}

	rotulos := []string{
		txtSenhasAuto, txtSenhasPreencher, txtSenhasCopiarUsuario,
		txtSenhasCopiarSenha, txtSenhasApagar, txtSenhasVazio, txtSenhasVer,
		txtNenhumaConta, txtHdrBuscador, txtExtNenhuma,
		txtDomSecao, txtDomSempreItem, txtAbrirPastaDados, txtAbaNovaEsquerda,
	}
	coletarRotulos(aplicativo.buildMenuFerramentas(), &rotulos)
	coletarRotulos(subMenuPrincipal(), &rotulos)

	// No paywall os servicos sao dominios (dados, minusculos por natureza);
	// aqui valem so os cabecalhos autorais.
	for _, item := range itensMenuPaywall() {
		if item.kind == miHeader {
			rotulos = append(rotulos, item.label)
		}
	}

	for _, rotulo := range rotulos {
		if !rotuloComecaBem(rotulo) {
			t.Errorf("rotulo minusculo: %q", rotulo)
		}
	}
}
