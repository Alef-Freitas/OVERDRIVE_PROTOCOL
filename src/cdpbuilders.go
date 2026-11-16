package main

import (
	"encoding/json"
)

// cdpCookie e um cookie do CDP (Network.getCookies); resposta compartilhada
// pelos leitores de cookies (painel esquerdo e troca de conta).
type cdpCookie struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// cdpCookiesResp decodifica a resposta de Network.getCookies.
type cdpCookiesResp struct {
	Cookies []cdpCookie `json:"cookies"`
}

// cdpMonta serializa o payload ou devolve erro (nunca JSON quebrado).
func cdpMonta(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err //nolint:wrapcheck // Serializacao em memoria: erro autoexplicativo.
	}

	return string(b), nil
}

// cdpInsertText monta Input.insertText (texto arbitrário do usuário).
func cdpInsertText(texto string) (string, error) {
	return cdpMonta(struct {
		Text string `json:"text"`
	}{Text: texto})
}

// botaoEsqMouse e o botao esquerdo no protocolo CDP (Input.dispatchMouseEvent).
const botaoEsqMouse = "left"

// cdpMouseClique monta Input.dispatchMouseEvent pressed/released.
func cdpMouseClique(posX, posY float64, tipo string) (string, error) {
	if tipo != "mousePressed" && tipo != "mouseReleased" {
		return "", erroTexto("tipo de mouse invalido: " + tipo)
	}

	return cdpMonta(struct {
		X          float64 `json:"x"`
		Y          float64 `json:"y"`
		Button     string  `json:"button"`
		ClickCount int     `json:"clickCount"`
		Type       string  `json:"type"`
	}{X: posX, Y: posY, Button: botaoEsqMouse, ClickCount: 1, Type: tipo})
}

// cdpTeclaEnter monta Input.dispatchKeyEvent do Enter (down/up).
func cdpTeclaEnter(tipo string) (string, error) {
	//exhaustruct:ignore // Evento CDP parcial: campos preenchidos conforme o tipo de tecla abaixo.
	carga := struct {
		Type                  string `json:"type"`
		Key                   string `json:"key,omitempty"`
		Code                  string `json:"code,omitempty"`
		WindowsVirtualKeyCode int    `json:"windowsVirtualKeyCode,omitempty"`
		NativeVirtualKeyCode  int    `json:"nativeVirtualKeyCode,omitempty"`
		Text                  string `json:"text,omitempty"`
		UnmodifiedText        string `json:"unmodifiedText,omitempty"`
	}{Type: tipo}
	if tipo == "keyDown" {
		carga.Key, carga.Code = "Enter", "Enter"
		carga.WindowsVirtualKeyCode, carga.NativeVirtualKeyCode = 13, 13
		carga.Text, carga.UnmodifiedText = "\r", "\r"
	} else if tipo != "keyUp" {
		return "", erroTexto("tipo de tecla invalido: " + tipo)
	}

	return cdpMonta(carga)
}

// cdpScriptDesligado monta Emulation.setScriptExecutionDisabled.
func cdpScriptDesligado(desligado bool) (string, error) {
	return cdpMonta(struct {
		Value bool `json:"value"`
	}{Value: desligado})
}

// alvoValido: targetId vem do protocolo (hex/UUID); sem aspas ou controle.
func alvoValido(id string) bool {
	if id == "" {
		return false
	}

	for i := range len(id) {
		if c := id[i]; c < 0x20 || c == '"' || c == '\\' {
			return false
		}
	}

	return true
}

// cdpFecharAlvo monta Target.closeTarget.
func cdpFecharAlvo(id string) (string, error) {
	if !alvoValido(id) {
		return "", erroTextoF("targetId invalido: %q", id)
	}

	return cdpMonta(struct {
		TargetID string `json:"targetId"`
	}{TargetID: id})
}
