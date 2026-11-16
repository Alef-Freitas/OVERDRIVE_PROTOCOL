package main

import (
	"fmt"
	"net/url"
	"strings"
)

type motorBusca struct {
	chave string
	nome  string
	badge string
	url   string
}

func motorPorChave(chave string) motorBusca {
	for _, m := range motoresBusca {
		if m.chave == chave {
			return m
		}
	}

	return motoresBusca[0]
}

func (a *app) motorAtual() motorBusca {
	if a == nil {
		return motoresBusca[0]
	}

	return motorPorChave(a.st.MotorBusca)
}

func (a *app) resolverInput(texto string) string {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return ""
	}

	lower := strings.ToLower(texto)

	esquemasSuportados := []string{
		"http:", "https:", "file:", "about:", "chrome-extension:", "overdrive:",
		"view-source:", "data:", "blob:", "mailto:", "tel:",
	}
	for _, p := range esquemasSuportados {
		if strings.HasPrefix(lower, p) {
			return texto
		}
	}

	if !strings.ContainsAny(texto, " \t\r\n") && (strings.Contains(texto, ".") || strings.HasPrefix(lower, hostLocal)) {
		return "https://" + texto
	}

	return a.motorAtual().url + url.QueryEscape(texto)
}

func (a *app) abrirMenuBuscador() {
	if menu.open && a.menuOrigem == idBuscador {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	atual := a.motorAtual()
	itens := []menuItem{{kind: miHeader, label: txtHdrBuscador}}

	for _, motor := range motoresBusca {
		hint := ""
		if motor.chave == atual.chave {
			hint = txtHintAtual
		}

		itens = append(itens, menuItem{kind: miItem, label: motor.nome, hint: hint, id: "buscador:" + motor.chave})
	}

	a.menuOrigem = idBuscador
	menu.items = itens
	a.menuMetricsIn(&menu, true)

	if menu.hwnd == 0 && !a.createMenuWindow() {
		a.dizerf(txtBuscadorFalhou)

		return
	}

	a.abrirMenuAncorado(a.engineBtn.rect)
}

func (a *app) escolherMotor(chave string) {
	motor := motorPorChave(chave)
	if motor.chave == "" {
		return
	}

	a.st.MotorBusca = motor.chave
	a.persist()
	a.status = fmt.Sprintf(txtFmtPesquisarCom, motor.nome)
	a.invalidateBar()
	a.focusAddress()
}
