//go:build windows

package main

import (
	"testing"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/directwrite"
)

func TestFonteReticenciasLinhaUnica(t *testing.T) {
	t.Parallel()

	var disp d2dMotor

	formato := disp.fonte("Segoe UI", 16, 0, 0, 1)
	if formato == nil {
		t.Skip("sem DirectWrite no ambiente de teste")
	}

	var corte directwrite.DWRITE_TRIMMING

	var sinal *directwrite.IDWriteInlineObject
	if err := formato.GetTrimming(&corte, &sinal); err != nil {
		t.Fatalf("GetTrimming falhou: %v", err)
	}

	if sinal != nil {
		sinal.Release()
	}

	if corte.Granularity != directwrite.DWRITE_TRIMMING_GRANULARITY_CHARACTER {
		t.Errorf("linha unica devia cortar por caractere, veio %v", corte.Granularity)
	}
}

func TestFonteSemReticenciasMultiLinha(t *testing.T) {
	t.Parallel()

	var disp d2dMotor

	formato := disp.fonte("Segoe UI", 16, 0, 0, 0)
	if formato == nil {
		t.Skip("sem DirectWrite no ambiente de teste")
	}

	var corte directwrite.DWRITE_TRIMMING

	var sinal *directwrite.IDWriteInlineObject
	if err := formato.GetTrimming(&corte, &sinal); err != nil {
		t.Fatalf("GetTrimming falhou: %v", err)
	}

	if sinal != nil {
		sinal.Release()
	}

	if corte.Granularity != directwrite.DWRITE_TRIMMING_GRANULARITY_NONE {
		t.Errorf("multilinha devia ficar sem corte, veio %v", corte.Granularity)
	}
}

func TestTextoColoridoNaoTravaSemRT(t *testing.T) {
	t.Parallel()

	var disp d2dMotor

	disp.texto("", rectT{0, 0, 10, 10}, corTexto, "Segoe UI", 16, 0, 1, 1)
	disp.texto("oi", rectT{}, corTexto, "Segoe UI", 16, 0, 1, 1)
	disp.texto("oi", rectT{0, 0, 10, 10}, corTexto, "Segoe UI", 16, 0, 1, 1)
	disp.textoFonte("titulo longo", rectT{0, 0, 10, 10}, corTexto, 0, dtEndEllipsis)
}
