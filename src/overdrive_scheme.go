//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

//odlint:somente-leitura tabela estatica de MIME.
//nolint:gochecknoglobals // Tabela MIME estatica: somente leitura.
var tipoPorExtOD = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
}

const paginaNaoEncontradaOD = `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Overdrive</title>
<style>html{background:#0b0b0b}body{margin:0;min-height:100vh;display:flex;align-items:center;` +
	`justify-content:center;background:#0b0b0b;color:#f2f2f2;` +
	`font:400 14px/1.5 "Segoe UI",system-ui,sans-serif}p{color:#b0b0b0}</style>
</head><body><div><h2 style="margin:0 0 8px">Página interna não encontrada</h2><p>` +
	`Verifique o endereço overdrive:// digitado.</p></div></body></html>`

func extrairRelativoInternoOD(uri string) (string, bool) {
	rest := strings.TrimPrefix(uri, "overdrive://")
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}

	if !strings.HasPrefix(rest, "paginas/") {
		return "", false
	}

	rel := filepath.FromSlash(strings.TrimPrefix(rest, "paginas/"))
	if rel == "" || rel == "." {
		return "", false
	}

	return rel, true
}

func validarCaminhoInternoOD(rel string) (string, bool) {
	limpo := filepath.Clean(rel)
	if limpo != rel || strings.HasPrefix(limpo, "..") || filepath.IsAbs(limpo) {
		return "", false
	}

	return limpo, true
}

func confinarPaginaInternaOD(limpo string) (string, bool) {
	base, err := filepath.Abs(filepath.Join(exeDir, "lib", "paginas"))
	if err != nil {
		return "", false
	}

	abs, err := filepath.Abs(filepath.Join(base, limpo))
	if err != nil || abs != base && !strings.HasPrefix(abs, base+string(filepath.Separator)) {
		return "", false
	}

	return abs, true
}

func responderPaginaInterna(uri string) ([]byte, int, string, string) {
	const httpStatusOK = 200

	rel, valido := extrairRelativoInternoOD(uri)
	if !valido {
		return pagina404OD()
	}

	limpo, valido := validarCaminhoInternoOD(rel)
	if !valido {
		return pagina404OD()
	}

	tipoConteudo, valido := tipoPorExtOD[strings.ToLower(filepath.Ext(limpo))]
	if !valido {
		return pagina404OD()
	}

	abs, valido := confinarPaginaInternaOD(limpo)
	if !valido {
		return pagina404OD()
	}
	// GOD: contencao verificada acima (prefixo base); sem traversal por desenho.
	b, err := os.ReadFile(abs) // #nosec G304
	if err != nil {
		return pagina404OD()
	}

	return b, httpStatusOK, "OK", "Content-Type: " + tipoConteudo + "\r\n"
}

func pagina404OD() ([]byte, int, string, string) {
	const httpStatusNaoEncontrado = 404

	return []byte(paginaNaoEncontradaOD), httpStatusNaoEncontrado, "Not Found",
		"Content-Type: text/html; charset=utf-8\r\n"
}
