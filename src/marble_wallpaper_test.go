//go:build windows

package main

import (
	"image"
	_ "image/jpeg"
	"os"
	"testing"
)

func TestMarbleWallpaperDecodifica(t *testing.T) {
	t.Parallel()

	caminho := "C:\\Users\\DEVICE\\Desktop\\OVERDRIVE PROTOCOL\\browser\\wallpaper.jpg"
	arquivo, erro := os.Open(caminho)
	if erro != nil {
		t.Fatalf("abrir: %v", erro)
	}
	defer arquivo.Close()

	src, formato, erro := image.Decode(arquivo)
	if erro != nil {
		t.Fatalf("decodificar: %v", erro)
	}

	limites := src.Bounds()
	if limites.Dx() <= 0 || limites.Dy() <= 0 {
		t.Fatalf("dimensoes invalidas: %+v", limites)
	}
	t.Logf("formato=%s %dx%d", formato, limites.Dx(), limites.Dy())
}
