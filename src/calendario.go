//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Constantes do calendario (dias da semana, anos bissextos e algoritmo da Pascoa).
const (
	calDiasSemana       = 7
	calSemanasGrade     = 6
	calAnoBaseSeculo    = 100
	calQuartoSeculo     = 4
	calCicloAnos        = 19
	calAjusteSeculo     = 25
	calFatorPascoa      = 11
	calFatorSeculo      = 8
	calFatorLua         = 22
	calDivisorPascoa    = 451
	calDiasMes          = 31
	calAjusteMes        = 114
	calDiasCarnaval     = -47
	calDiasSextaSanta   = -2
	calDiasCorpus       = 60
	calLarguraPadrao    = 320
	calAlturaPadrao     = 340
	calTipoNacional     = "NACIONAL"
	calTipoVip          = "VIP"
	calTipoAniversario  = "ANIVERSÁRIO"
	calTipoFacultativo  = "FACULTATIVO"
	calCorFeriado       = 0x00FACC15
	calCorAniversario   = 0x00FF69B4
	calCorVip           = 0x00C084FC
	calCorFacultativo   = 0x00FB923C
	calEspessuraMoldura = 2
	calUm               = 1
	calDois             = 2
	calTres             = 3
	calDiaSete          = 7
	calDiaOito          = 8
	calDiaDoze          = 12
	calDiaTreze         = 13
	calQuinze           = 15
	calDiaDezessete     = 17
	calDiaDezenove      = 19
	calDiaVinte         = 20
	calDiaVinteUm       = 21
	calDiaVinteCinco    = 25
	calTrinta           = 30
	calTrintaDois       = 32
)

// calendarioEvento e um feriado ou aniversario do dia.
type calendarioEvento struct {
	dia  int
	mes  time.Month
	nome string
	tipo string
	aniv bool
}

// aniversariosCalendario devolve os aniversarios fixos, como no projeto de origem (hardcode).
func aniversariosCalendario() []calendarioEvento {
	return []calendarioEvento{
		{dia: calDiaDezessete, mes: time.January, nome: "Analdina", tipo: calTipoAniversario, aniv: true},
		{dia: calTrinta, mes: time.May, nome: "Paula", tipo: calTipoAniversario, aniv: true},
		{dia: calDiaDezenove, mes: time.December, nome: "Alef", tipo: calTipoVip, aniv: true},
		{dia: calDiaTreze, mes: time.May, nome: "Manuela", tipo: calTipoVip, aniv: true},
		{dia: calDiaVinteUm, mes: time.April, nome: "Aya", tipo: calTipoVip, aniv: true},
	}
}

// feriadosFixosTabela devolve os feriados de data fixa (nacionais e regionais).
func feriadosFixosTabela() []calendarioEvento {
	return []calendarioEvento{
		{dia: calUm, mes: time.January, nome: "Ano Novo", tipo: calTipoNacional, aniv: false},
		{dia: calDiaVinteUm, mes: time.April, nome: "Tiradentes", tipo: calTipoNacional, aniv: false},
		{dia: calUm, mes: time.May, nome: "Dia do Trabalho", tipo: calTipoNacional, aniv: false},
		{dia: calDiaSete, mes: time.September, nome: "Independência do Brasil", tipo: calTipoNacional, aniv: false},
		{dia: calDiaDoze, mes: time.October, nome: "Nossa Senhora Aparecida", tipo: calTipoNacional, aniv: false},
		{dia: calDois, mes: time.November, nome: "Finados", tipo: calTipoNacional, aniv: false},
		{dia: calQuinze, mes: time.November, nome: "Proclamação da República", tipo: calTipoNacional, aniv: false},
		{dia: calDiaVinte, mes: time.November, nome: "Consciência Negra", tipo: calTipoNacional, aniv: false},
		{dia: calDiaVinteCinco, mes: time.December, nome: "Natal", tipo: calTipoNacional, aniv: false},
		{dia: calQuinze, mes: time.August, nome: "Assunção de Nossa Senhora", tipo: "BH", aniv: false},
		{dia: calDiaOito, mes: time.December, nome: "Imaculada Conceição", tipo: "BH/CE", aniv: false},
		{dia: calDiaVinteCinco, mes: time.March, nome: "Data Magna do Ceará", tipo: "CE", aniv: false},
		{dia: calDiaDezenove, mes: time.March, nome: "São José", tipo: "CE", aniv: false},
	}
}

// pascoa calcula a data da Pascoa pelo algoritmo de Meeus/Jones/Butcher,
// equivalente ao algoritmo de Gauss usado no projeto de origem.
func pascoa(ano int) time.Time {
	ciclo := ano % calCicloAnos
	seculo := ano / calAnoBaseSeculo
	anoSeculo := ano % calAnoBaseSeculo
	quartoSeculo := seculo / calQuartoSeculo
	restoSeculo := seculo % calQuartoSeculo
	ajusteSeculo := (seculo + calFatorSeculo) / calAjusteSeculo
	ajusteLua := (seculo - ajusteSeculo + calUm) / calTres
	epacta := (calCicloAnos*ciclo + seculo - quartoSeculo - ajusteLua + calQuinze) % calTrinta
	quartoAno := anoSeculo / calQuartoSeculo
	restoAno := anoSeculo % calQuartoSeculo
	diaLua := (calTrintaDois + calDois*restoSeculo + calDois*quartoAno - epacta - restoAno) % calDiasSemana
	ajusteFinal := (ciclo + calFatorPascoa*epacta + calFatorLua*diaLua) / calDivisorPascoa
	mes := (epacta + diaLua - calDiasSemana*ajusteFinal + calAjusteMes) / calDiasMes
	dia := ((epacta + diaLua - calDiasSemana*ajusteFinal + calAjusteMes) % calDiasMes) + 1

	//nolint:gosmopolitan // Datas locais do calendario do usuario.
	return time.Date(ano, time.Month(mes), dia, 0, 0, 0, 0, time.Local)
}

// feriadosMoveis devolve Carnaval, Sexta-Feira Santa e Corpus Christi.
func feriadosMoveis(ano int) []calendarioEvento {
	pascoaData := pascoa(ano)
	mover := func(dias int, nome, tipo string) calendarioEvento {
		d := pascoaData.AddDate(0, 0, dias)

		return calendarioEvento{dia: d.Day(), mes: d.Month(), nome: nome, tipo: tipo, aniv: false}
	}

	return []calendarioEvento{
		mover(calDiasCarnaval, "Carnaval", calTipoFacultativo),
		mover(calDiasSextaSanta, "Sexta-Feira Santa", calTipoNacional),
		mover(calDiasCorpus, "Corpus Christi", "BH/CE"),
	}
}

// eventosCalendario monta todos os eventos do ano, agrupados por mes/dia.
func eventosCalendario(ano int) map[[2]int][]calendarioEvento {
	resultado := make(map[[2]int][]calendarioEvento)

	adicionar := func(e calendarioEvento) {
		chave := [2]int{int(e.mes), e.dia}
		resultado[chave] = append(resultado[chave], e)
	}

	for _, e := range feriadosFixosTabela() {
		adicionar(e)
	}

	for _, e := range feriadosMoveis(ano) {
		adicionar(e)
	}

	for _, e := range aniversariosCalendario() {
		adicionar(e)
	}

	return resultado
}

// estado do popup do calendario.
type calendarioWin struct {
	hwnd      uintptr
	visivel   bool
	ano       int
	mes       int
	eventos   map[[2]int][]calendarioEvento
	hoverDia  int
	btnAntes  rectT
	btnDepois rectT
	btnFechar rectT
	diasRect  [calDiasSemana * calSemanasGrade]rectT
	diasNum   [calDiasSemana * calSemanasGrade]int
}

//nolint:gochecknoglobals // Popup unico do calendario: estado de UI single-window.
var calendarioUI calendarioWin

func abrirCalendario(app *app, ainda time.Time) {
	if app.hwnd == 0 || calendarioProcPtr == 0 {
		return
	}

	if calendarioUI.hwnd == 0 && !criarJanelaCalendario() {
		return
	}

	calendarioUI.ano = ainda.Year()
	calendarioUI.mes = int(ainda.Month())
	calendarioUI.eventos = eventosCalendario(ainda.Year())
	calendarioUI.hoverDia = 0

	posX, posY := app.posicaoCalendario(calLarguraPadrao, calAlturaPadrao)
	pSetWindowPos.Call(calendarioUI.hwnd, 0, uintptr(posX), uintptr(posY),
		uintptr(calLarguraPadrao), uintptr(calAlturaPadrao), swpNoZOrder|swpNoActivate|swpShowWindow)
	pShowWindow.Call(calendarioUI.hwnd, swShowNoActivate)
	pInvalidateRect.Call(calendarioUI.hwnd, 0, 0)
	pUpdateWindow.Call(calendarioUI.hwnd)
	calendarioUI.visivel = true
}

func fecharCalendario() {
	if calendarioUI.hwnd != 0 {
		pShowWindow.Call(calendarioUI.hwnd, swHide)
	}

	calendarioUI.visivel = false
	calendarioUI.hoverDia = 0

	d2dCalendario.soltar()
}

func criarJanelaCalendario() bool {
	nomeClasse := utf16("OverdriveCalendarioWnd")
	hInst := getModuleHandle()
	classe := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   calendarioProcPtr,
		hInstance:     hInst,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: nomeClasse,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classe)))

	h, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExNoActivate|wsExToolWindow,
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(utf16(""))),
		wsPopup,
		0, 0, calLarguraPadrao, calAlturaPadrao,
		0, 0, hInst, 0,
	)
	if h == 0 {
		return false
	}

	pSetWindowTheme.Call(h, uintptr(unsafe.Pointer(utf16("DarkMode_Explorer"))), 0)
	calendarioUI.hwnd = h

	return true
}

func (a *app) posicaoCalendario(largura, altura int32) (int32, int32) {
	origem := a.winClock
	if origem.Width() <= 0 {
		area := a.clientRect()
		origem = rectT{area.Right - a.sc(200), 0, area.Right, a.tabH}
	}

	ponto := struct{ x, y int32 }{origem.Left, origem.Bottom}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	x := ponto.x
	if area.Right > 0 && x+largura > area.Right-a.sc(4) {
		x = area.Right - largura - a.sc(4)
	}

	y := ponto.y + a.sc(4)
	if area.Bottom > 0 && y+altura > area.Bottom-a.sc(4) {
		y = area.Bottom - altura - a.sc(4)
	}

	return x, y
}

func calendarioProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		return calendarioPintar(hwnd, aplicativo)
	case wmSize:
		calendarioRedimensionar(hwnd)

		return 0
	case wmEraseBkgnd:
		return 1
	case wmLButtonDown:
		x, y := loword(lparam), hiword(lparam)
		tratarCliqueCalendario(x, y)

		return 0
	case wmMouseMove:
		calendarioMouseMove(hwnd, loword(lparam), hiword(lparam))

		return 0
	case wmNcHitTest:
		return htClient
	case wmKeyDown:
		if aplicativo != nil && uint32(wparam) == vkEscape {
			fecharCalendario()
		}

		return 0
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func calendarioPintar(hwnd uintptr, app *app) uintptr {
	if app == nil {
		pValidateRect.Call(hwnd, 0)

		return 0
	}

	var cliente rectT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cliente)))

	if d2dCalendario.rt == nil {
		d2dCalendario.criar(hwnd, cliente.Right, cliente.Bottom)
	}

	if d2dCalendario.rt == nil {
		pValidateRect.Call(hwnd, 0)

		return 0
	}

	d2dCalendario.iniciar()
	app.pintarCalendarioD2D(&d2dCalendario, cliente)
	d2dCalendario.apresentar()

	return 0
}

func calendarioRedimensionar(hwnd uintptr) {
	var cliente rectT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cliente)))
	d2dCalendario.redimensionar(cliente.Right, cliente.Bottom)
}

func calendarioMouseMove(hwnd uintptr, x, y int32) {
	dia := diaCalendarioEm(x, y)

	if dia != calendarioUI.hoverDia {
		calendarioUI.hoverDia = dia

		pInvalidateRect.Call(hwnd, 0, 0)
	}
}

// diaCalendarioEm devolve o numero do dia sob o cursor, ou 0 se for celula vazia.
func diaCalendarioEm(x, y int32) int {
	for indice, r := range calendarioUI.diasRect {
		if r.Width() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom {
			return calendarioUI.diasNum[indice]
		}
	}

	return 0
}

func tratarCliqueCalendario(x, y int32) {
	switch {
	case dentroRect(calendarioUI.btnFechar, x, y):
		fecharCalendario()
	case dentroRect(calendarioUI.btnAntes, x, y):
		mudarMesCalendario(-1)
	case dentroRect(calendarioUI.btnDepois, x, y):
		mudarMesCalendario(1)
	default:
		selecionarDiaCalendario(x, y)
	}
}

func selecionarDiaCalendario(x, y int32) {
	dia := diaCalendarioEm(x, y)
	if dia == 0 {
		return
	}

	calendarioUI.hoverDia = dia
	pInvalidateRect.Call(calendarioUI.hwnd, 0, 0)
}

func mudarMesCalendario(delta int) {
	mes := time.Month(calendarioUI.mes + delta)
	ano := calendarioUI.ano

	if mes < time.January {
		mes = time.December
		ano--
	} else if mes > time.December {
		mes = time.January
		ano++
	}

	calendarioUI.mes = int(mes)
	calendarioUI.ano = ano
	calendarioUI.eventos = eventosCalendario(ano)
	calendarioUI.hoverDia = 0
	pInvalidateRect.Call(calendarioUI.hwnd, 0, 0)
}

func (a *app) pintarCalendarioD2D(disp *d2dMotor, area rectT) {
	fundo := rectT{0, 0, area.Right, area.Bottom}
	disp.retB(fundo, a.brSurface)
	disp.contorno(fundo, corBordaForte)
	a.pintarCabecalhoCalendario(disp, area)
	a.pintarGradeCalendario(disp, area)
}

func (a *app) pintarCabecalhoCalendario(disp *d2dMotor, area rectT) {
	cabecalho := rectT{0, 0, area.Right, a.sc(40)}
	disp.ret(cabecalho, a.brChrome)

	titulo := fmt.Sprintf("%s %d", mesesRelogio[calendarioUI.mes-1], calendarioUI.ano)
	areaTitulo := rectT{a.sc(58), 0, area.Right - a.sc(58), cabecalho.Bottom}
	disp.textoUI(titulo, areaTitulo, corTexto, false, true)

	calendarioUI.btnAntes = rectT{a.sc(8), a.sc(10), a.sc(30), a.sc(32)}
	calendarioUI.btnDepois = rectT{area.Right - a.sc(30), a.sc(10), area.Right - a.sc(8), a.sc(32)}
	calendarioUI.btnFechar = rectT{
		area.Right - a.sc(30), area.Bottom - a.sc(30),
		area.Right - a.sc(8), area.Bottom - a.sc(8),
	}
	disp.textoUI("<", calendarioUI.btnAntes, corCiano, false, true)
	disp.textoUI(">", calendarioUI.btnDepois, corCiano, false, true)
	disp.textoUI("×", calendarioUI.btnFechar, corTextoFaint, false, true)

	inicio := cabecalho.Bottom + a.sc(8)
	semana := []string{"D", "S", "T", "Q", "Q", "S", "S"}
	larguraCol := (area.Right - a.sc(16)) / calDiasSemana

	for indice, nome := range semana {
		x := a.sc(8) + int32(indice)*larguraCol
		areaNome := rectT{x, inicio, x + larguraCol, inicio + a.sc(20)}
		disp.textoUI(nome, areaNome, corTextoFaint, true, true)
	}
}

func (a *app) pintarGradeCalendario(disp *d2dMotor, area rectT) {
	//nolint:gosmopolitan // Datas locais do calendario do usuario.
	primeiro := time.Date(calendarioUI.ano, time.Month(calendarioUI.mes), 1, 0, 0, 0, 0, time.Local)
	offset := int(primeiro.Weekday())
	//nolint:gosmopolitan // Datas locais do calendario do usuario.
	ultimo := time.Date(calendarioUI.ano, time.Month(calendarioUI.mes)+1, 0, 0, 0, 0, 0, time.Local)
	diasNoMes := ultimo.Day()

	topoGrade := a.sc(72)
	alturaLinha := a.sc(34)
	larguraCol := (area.Right - a.sc(16)) / calDiasSemana

	for indice := range calendarioUI.diasRect {
		calendarioUI.diasRect[indice] = rectT{}
		calendarioUI.diasNum[indice] = 0
	}

	agora := time.Now()

	for indice := 0; indice < offset+diasNoMes && indice < calDiasSemana*calSemanasGrade; indice++ {
		linha := indice / calDiasSemana
		coluna := indice % calDiasSemana
		x := a.sc(8) + int32(coluna)*larguraCol
		y := topoGrade + int32(linha)*alturaLinha
		celula := rectT{x, y, x + larguraCol, y + alturaLinha}
		calendarioUI.diasRect[indice] = celula

		dia := indice - offset + 1
		if dia < 1 || dia > diasNoMes {
			continue
		}

		calendarioUI.diasNum[indice] = dia

		chave := [2]int{calendarioUI.mes, dia}
		eventos := calendarioUI.eventos[chave]
		cor := a.corDiaCalendario(disp, celula, agora, dia)

		if len(eventos) > 0 {
			a.pintarMolduraEvento(disp, celula, corEventoCalendario(eventos))
		}

		areaNumero := rectT{celula.Left, celula.Top, celula.Right, celula.Bottom - a.sc(6)}
		disp.textoUI(strconv.Itoa(dia), areaNumero, cor, false, true)
	}

	rodape := rectT{a.sc(8), area.Bottom - a.sc(22), area.Right - a.sc(40), area.Bottom}
	disp.textoUI("passe o mouse sobre o contorno para ver o evento", rodape, corTextoFaint, true, false)

	a.pintarTooltipCalendario(disp, area)
}

// corEventoCalendario escolhe a cor do contorno conforme o tipo do evento.
func corEventoCalendario(eventos []calendarioEvento) uint32 {
	cor := uint32(0)

	for _, evento := range eventos {
		switch {
		case evento.tipo == calTipoVip:
			return calCorVip
		case evento.aniv:
			cor = calCorAniversario
		case evento.tipo == calTipoFacultativo && cor == 0:
			cor = calCorFacultativo
		case cor == 0:
			cor = calCorFeriado
		}
	}

	return cor
}

// pintarMolduraEvento desenha o contorno colorido do dia com evento.
func (a *app) pintarMolduraEvento(disp *d2dMotor, celula rectT, cor uint32) {
	margem := a.sc(3)
	moldura := rectT{
		celula.Left + margem, celula.Top + margem,
		celula.Right - margem, celula.Bottom - margem,
	}

	for range calEspessuraMoldura {
		disp.contorno(moldura, cor)
		moldura.Left++
		moldura.Top++
		moldura.Right--
		moldura.Bottom--
	}
}

func (a *app) corDiaCalendario(disp *d2dMotor, celula rectT, agora time.Time, dia int) uint32 {
	hoje := agora.Year() == calendarioUI.ano && int(agora.Month()) == calendarioUI.mes && agora.Day() == dia
	if hoje {
		disp.redondo(celula, a.sc(5), a.brActive, corCiano)

		return corCiano
	}

	if calendarioUI.hoverDia == dia {
		disp.redondo(celula, a.sc(5), a.brHover, corBorda)
	}

	return corTexto
}

// pintarTooltipCalendario mostra os eventos do dia sob o cursor.
func (a *app) pintarTooltipCalendario(disp *d2dMotor, area rectT) {
	dia := calendarioUI.hoverDia
	if dia <= 0 {
		return
	}

	eventos := calendarioUI.eventos[[2]int{calendarioUI.mes, dia}]
	if len(eventos) == 0 {
		return
	}

	var texto strings.Builder
	fmt.Fprintf(&texto, "%d de %s", dia, mesesRelogio[calendarioUI.mes-1])

	for _, evento := range eventos {
		texto.WriteString("\n")
		texto.WriteString(evento.nome)

		if evento.tipo != "" {
			texto.WriteString(" (")
			texto.WriteString(evento.tipo)
			texto.WriteString(")")
		}
	}

	alturaLinha := a.sc(16)
	altura := a.sc(12) + int32(1+len(eventos))*alturaLinha
	topo := area.Bottom - a.sc(26) - altura
	painel := rectT{a.sc(12), topo, area.Right - a.sc(12), topo + altura}

	disp.retB(painel, a.brTipBg)
	disp.contorno(painel, corBordaForte)

	interno := rectT{painel.Left + a.sc(8), painel.Top + a.sc(4), painel.Right - a.sc(8), painel.Bottom - a.sc(4)}
	disp.textoUI(texto.String(), interno, corTexto, true, false)
}
