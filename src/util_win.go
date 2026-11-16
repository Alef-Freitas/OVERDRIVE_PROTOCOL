//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func dominioDeURL(u string) string {
	pu, err := url.Parse(u)
	if err != nil {
		return ""
	}

	return strings.ToLower(pu.Hostname())
}

func formataMilhar(n int) string {
	const grupoMilharDigitos = 3

	texto := strconv.Itoa(n)
	if len(texto) <= grupoMilharDigitos {
		return texto
	}

	var montador strings.Builder

	ini := len(texto) % grupoMilharDigitos
	if ini > 0 {
		montador.WriteString(texto[:ini])
	}

	for i := ini; i < len(texto); i += grupoMilharDigitos {
		if montador.Len() > 0 {
			montador.WriteByte('.')
		}

		montador.WriteString(texto[i : i+grupoMilharDigitos])
	}

	return montador.String()
}

func jsonDeString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}

	return string(b)
}

func desembrulharEvalJS(resposta string, destino any) error {
	texto := strings.TrimSpace(resposta)
	if texto == "" {
		return erroTexto("o EvalJS nao devolveu nada")
	}

	if len(texto) >= 2 && texto[0] == '"' && texto[len(texto)-1] == '"' {
		var txt string
		if err := json.Unmarshal([]byte(texto), &txt); err == nil {
			texto = txt
		}
	}

	if err := json.Unmarshal([]byte(texto), destino); err != nil {
		return erroTextoF("resposta do EvalJS em formato inesperado: %w", err)
	}

	return nil
}

func (a *app) chamarCDP(p *pane, metodo, params string) (string, error) {
	const timeoutRespostaCDPSegundos = 10

	if params == "" {
		params = "{}"
	}

	saida := make(chan string, 1)
	falha := make(chan error, 1)

	pronto := make(chan struct{})
	defer close(pronto)

	a.depois(func() {
		a.solicitarCDP(p, metodo, params, saida, falha, pronto)
	})

	select {
	case saidaTexto := <-saida:
		return saidaTexto, nil
	case e := <-falha:
		return "", e
	case <-time.After(timeoutRespostaCDPSegundos * time.Second):
		return "", erroTextoF("o navegador nao respondeu a %s", metodo)
	}
}

// solicitarCDP pede o metodo no painel e entrega o resultado nos canais.
func (a *app) solicitarCDP(p *pane, metodo, params string, saida chan string, falha chan error, pronto chan struct{}) {
	if p == nil || !p.ready || p.wv == nil {
		select {
		case falha <- erroTexto("sem pagina alvo"):
		case <-pronto:
		}

		return
	}

	p.wv.CallDevToolsResult(metodo, params, func(err error, res string) {
		a.entregarCDP(err, res, saida, falha, pronto)
	})
}

// entregarCDP repassa o retorno do DevTools salvo cancelamento.
func (a *app) entregarCDP(err error, res string, saida chan string, falha chan error, pronto chan struct{}) {
	select {
	case <-pronto:
		return
	default:
	}

	if err != nil {
		select {
		case falha <- err:
		case <-pronto:
		}

		return
	}

	select {
	case saida <- res:
	case <-pronto:
	}
}

func (a *app) clicarCDP(p *pane, x, y float64) {
	if pressed, err := cdpMouseClique(x, y, "mousePressed"); err == nil {
		p.wv.CallDevTools("Input.dispatchMouseEvent", pressed)
	}

	if released, err := cdpMouseClique(x, y, "mouseReleased"); err == nil {
		p.wv.CallDevTools("Input.dispatchMouseEvent", released)
	}
}

func origemDe(endereco string) string {
	endereco = strings.TrimSpace(endereco)
	if endereco == "" {
		return ""
	}

	analisado, err := url.Parse(endereco)
	if err != nil || analisado.Scheme == "" || analisado.Host == "" {
		return ""
	}

	if analisado.Scheme != esquemaHTTP && analisado.Scheme != esquemaHTTPS {
		return ""
	}

	return analisado.Scheme + "://" + analisado.Host
}

func urlUtilizavel(endereco string) bool {
	endereco = strings.TrimSpace(endereco)

	return endereco != "" && !strings.HasPrefix(endereco, "about:") && !strings.HasPrefix(endereco, "data:")
}

func (a *app) depois(tarefa func()) {
	if tarefa == nil || a == nil || a.hwnd == 0 {
		return
	}

	a.tarefasMu.Lock()
	a.tarefasFila = append(a.tarefasFila, tarefa)
	a.tarefasMu.Unlock()
	pPostMessageW.Call(a.hwnd, wmAppTarefa, 0, 0)
}

func (a *app) drainTarefas() {
	a.tarefasMu.Lock()
	fila := a.tarefasFila
	a.tarefasFila = nil
	a.tarefasMu.Unlock()

	for _, tarefa := range fila {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Sem log a falha vira silencio: deixa rastro no
					// debugger sem derrubar a janela.
					if pOutputDebugString != nil {
						s := fmt.Sprintf("overdrive: panico em tarefa: %v", r)
						pOutputDebugString.Call(uintptr(unsafe.Pointer(utf16(s))))
					}
				}
			}()

			tarefa()
		}()
	}
}

func semBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
}

func (a *app) dizerf(format string, args ...any) {
	a.avisar(fmt.Sprintf(format, args...), avisoDuracaoMs, false, 0, false)
}

func (a *app) dizerOkf(format string, args ...any) {
	a.avisar(fmt.Sprintf(format, args...), avisoDuracaoOk, false, 0, false)
}

func (a *app) dizerErrof(format string, args ...any) {
	a.avisar(fmt.Sprintf(format, args...), avisoDuracaoErro, false, 0, false)
}

func (a *app) dizerProgressof(pct int, format string, args ...any) {
	a.avisar(fmt.Sprintf(format, args...), avisoProgressoMs, true, pct, false)
}

func (a *app) dizerProgressoIndetf(format string, args ...any) {
	a.avisar(fmt.Sprintf(format, args...), avisoProgressoMs, true, 0, true)
}

func (a *app) avisar(texto string, duracao int, temBarra bool, pct int, indet bool) {
	a.status = texto
	a.mostrarAviso2(texto, duracao, temBarra, pct, indet)

	if a.hwnd != 0 {
		if duracao <= 0 {
			duracao = avisoDuracaoMs
		}

		pSetTimer.Call(a.hwnd, timerStatus, uintptr(duracao), 0)
	}
}

func absI32(v int32) int32 {
	if v < 0 {
		return -v
	}

	return v
}

func loadCursor(id int) uintptr {
	h, _, _ := pLoadCursorW.Call(0, uintptr(id))

	return h
}

func (r rectT) Width() int32 { return r.Right - r.Left }

func (r rectT) Height() int32 { return r.Bottom - r.Top }

func utf16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}

	return p
}

func loword(v uintptr) int32 { return int32(int16(uint16(v & mascaraPalavraWin))) }

func hiword(v uintptr) int32 {
	return int32(int16(uint16((v >> deslocamentoPalavraAltaWin) & mascaraPalavraWin)))
}

const (
	mascaraPalavraWin          = 0xFFFF
	deslocamentoPalavraAltaWin = 16
)

func makeLParam(x, y int32) uintptr {
	return uintptr(uint32(uint16(x))) | uintptr(uint32(uint16(y)))<<16
}

func getKeyDown(vk int) bool {
	r, _, _ := pGetKeyState.Call(uintptr(vk))

	return int16(r) < 0
}

func getModuleHandle() uintptr {
	h, _, _ := pGetModuleHandleW.Call(0)

	return h
}

func loadIconFile(path string) uintptr {
	h, _, _ := pLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(path))),
		imageIcon, 0, 0,
		lrLoadFromFile|lrDefaultSize,
	)

	return h
}

func dpiForWindow(hwnd uintptr) int {
	const dpiPadraoWindows = 96

	r, _, _ := pGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return dpiPadraoWindows
	}

	return int(r)
}

func caminho(v any, chaves ...any) any {
	atual := v

	for _, chave := range chaves {
		switch item := chave.(type) {
		case string:
			m, ok := atual.(map[string]any)
			if !ok {
				return nil
			}

			atual = m[item]
		case int:
			l, ok := atual.([]any)
			if !ok || item < 0 || item >= len(l) {
				return nil
			}

			atual = l[item]
		default:
			return nil
		}

		if atual == nil {
			return nil
		}
	}

	return atual
}

func textoDe(v any) string {
	switch conteudo := v.(type) {
	case string:
		return conteudo
	case map[string]any:
		if s, ok := conteudo["simpleText"].(string); ok {
			return s
		}

		if s, ok := conteudo["content"].(string); ok {
			return s
		}

		if runs, ok := conteudo["runs"].([]any); ok {
			var montador strings.Builder

			for _, trecho := range runs {
				if s, ok := caminho(trecho, "text").(string); ok {
					montador.WriteString(s)
				}
			}

			return montador.String()
		}

		if s := textoDe(conteudo["text"]); s != "" {
			return s
		}
	}

	return ""
}

func acharTodos(v any, chave string, out *[]map[string]any, prof int) {
	if prof > 24 || len(*out) > 400 {
		return
	}

	switch nodo := v.(type) {
	case map[string]any:
		if obj, ok := nodo[chave].(map[string]any); ok {
			*out = append(*out, obj)
		}

		for _, f := range nodo {
			acharTodos(f, chave, out, prof+1)
		}
	case []any:
		for _, f := range nodo {
			acharTodos(f, chave, out, prof+1)
		}
	}
}

func (r Rect) Width() int32 { return r.Right - r.Left }

func (r Rect) Height() int32 { return r.Bottom - r.Top }

func portaDebugValida(texto string) bool {
	if texto == "" || len(texto) > 5 {
		return false
	}

	for _, r := range texto {
		if r < '0' || r > '9' {
			return false
		}
	}

	const baseDecimalPortaDebug = 10

	n := 0
	for _, r := range texto {
		n = n*baseDecimalPortaDebug + int(r-'0')
	}

	return n >= 1 && n <= 65535
}
