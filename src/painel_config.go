//go:build windows

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfgClasse = "OverdriveCfgWnd"
	cfgTimer  = 17

	wsBorder  = 0x00800000
	wsTabstop = 0x00010000
	wsGroup   = 0x00020000
	wsVScroll = 0x00200000

	esMultiline   = 0x0004
	esAutovscroll = 0x0040
	esAutohscroll = 0x0080
	esWantreturn  = 0x1000

	cbsDropdownlist = 0x0003

	wmSetfont = 0x0030

	cbAddstring = 0x0143
	cbSetcursel = 0x014E
	cbGetcursel = 0x0147

	emSetcuebanner = 0x1501

	bnClicked    = 0
	cbnSelchange = 1
	enChange     = 0x0300

	dtEndEllipsis = 0x00008000
	dtWordbreak   = 0x00000010

	cfgCtlFiltro = 1001
	cfgCtlIdioma = 1002
	cfgCtlVPN    = 1003
)

// Constantes locais do painel de configuracao (fatores e limites sem outro dono).
const (
	margemTextoControle     = 2
	bytesPorKB              = 1024
	bytesPorMB              = 1048576
	bytesPorGB              = 1073741824
	meioArredondamento      = 0.5
	tamanhoInicialJanelaCfg = 100
	tamanhoInicialFilhoCfg  = 10
	divisorMetadeCfg        = 2
	dobroMargemCfg          = 2
	intervaloAssinaturaCfg  = 500
	estadoDownConcluido     = 2
	totalPartesAcaoCfg      = 3
	fatorPorcentagemCfg     = 100
	zoomMinimoTexto         = 40
	zoomMaximoTexto         = 160
	zoomMinimoPainelIA      = 85
	zoomMaximoPainelIA      = 115
	deltaRodaMouse          = 120
	mascaraWordBaixa        = 0xFFFF
	deslocamentoWordAlta    = 16
	decimaisTamanhoGB       = 2
)

// downAss (campo de cfgWin) guarda a ultima assinatura da lista de
// downloads: o timer de 500 ms so reconstrói o layout quando algo mudou.
func (a *app) assDownloads() string {
	if a == nil {
		return ""
	}

	var construtor strings.Builder
	construtor.WriteString(strconv.Itoa(len(a.downloads)))

	for _, download := range a.downloads {
		if download == nil {
			continue
		}

		construtor.WriteString(";")
		construtor.WriteString(strconv.Itoa(download.ID))
		construtor.WriteString(",")
		construtor.WriteString(strconv.FormatInt(download.Recebido, 10))
		construtor.WriteString(",")
		construtor.WriteString(strconv.FormatInt(int64(download.Estado), 10))

		if download.Cancelado {
			construtor.WriteString(",x")
		}
	}

	return construtor.String()
}

type cfgSecao int

const (
	cfgSecDown cfgSecao = iota
	cfgSecTrad
	cfgSecCfg
	cfgSecDados
	cfgSecVPN
)

type cfgZona struct {
	id    string
	label string
	r     rectT
}

type cfgTxt struct {
	s     string
	r     rectT
	cor   uint32
	fonte uintptr
	flags uint32
}

type cfgBar struct {
	r   rectT
	pct int
}

type cfgDownRow struct {
	dl         downloadAtivo
	r          rectT
	rBar       rectT
	mostrarBar bool
}

type cfgWin struct {
	hwnd      uintptr
	ativo     bool
	secao     cfgSecao
	w, h      int32
	navW      int32
	headH     int32
	scroll    int32
	maxScroll int32

	hoverNav int
	hoverID  string

	filtro     string
	tradOK     bool
	idiomaPend string

	cookiesCockpit []cockpitCookie
	cookiesErro    string
	cookiesLendo   bool

	editFiltro  uintptr
	editVPN     uintptr
	comboIdioma uintptr
	filhosOK    bool

	zonas []cfgZona
	txts  []cfgTxt
	bars  []cfgBar
	downs []cfgDownRow

	arrastaBarra bool
	pegaBarra    int32
	barHover     bool

	downAss string

	fechar rectT
}

//nolint:gochecknoglobals // Janela de configuracao + proc: estado de UI single-window.
var (
	cfgUI      cfgWin
	cfgProcPtr uintptr
)

func cfgSecoes() []struct {
	titulo string
	icone  string
} {
	return []struct {
		titulo string
		icone  string
	}{
		{txtCfgSecDown, idDownload},
		{txtCfgSecTrad, idDictionary},
		{txtCfgSecCfg, idSettings},
		{txtCfgSecDados, idDelete},
		{txtCfgSecVPN, idVPN},
	}
}

func cfgSeccaoDe(nome string) cfgSecao {
	switch strings.ToLower(strings.TrimSpace(nome)) {
	case "traducao":
		return cfgSecTrad
	case "config":
		return cfgSecCfg
	case "dados":
		return cfgSecDados
	case idVPN:
		return cfgSecVPN
	default:
		return cfgSecDown
	}
}

func cfgTextoControle(h uintptr) string {
	if h == 0 {
		return ""
	}

	n, _, _ := pGetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+margemTextoControle)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))

	return windows.UTF16ToString(buf)
}

func cfgTamanho(tamanho int64) string {
	if tamanho < bytesPorKB {
		return strconv.FormatInt(tamanho, 10) + " B"
	}

	if tamanho < bytesPorKB*bytesPorKB {
		return strings.Replace(trimFloat(float64(tamanho)/bytesPorKB, 1), ".", ",", 1) + " KB"
	}

	if tamanho < bytesPorKB*bytesPorKB*bytesPorKB {
		return strings.Replace(trimFloat(float64(tamanho)/bytesPorMB, 1), ".", ",", 1) + " MB"
	}

	return strings.Replace(trimFloat(float64(tamanho)/bytesPorGB, 1), ".", ",", 1) + " GB"
}

func trimFloat(valor float64, casas int) string {
	mult := 1.0
	for range casas {
		mult *= 10
	}

	v := float64(int(valor*mult+meioArredondamento)) / mult
	s := strconv.FormatFloat(v, 'f', casas, 64)

	return s
}

func cfgQuando(iso string) string {
	if strings.TrimSpace(iso) == "" {
		return ""
	}

	data, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}

	agora := time.Now()
	if data.Year() == agora.Year() && data.YearDay() == agora.YearDay() {
		return data.Format("15:04")
	}

	return data.Format("02/01 15:04")
}

func cfgCriarJanela() bool {
	if cfgUI.hwnd != 0 {
		return true
	}

	app := theApp
	if app == nil {
		return false
	}

	if cfgProcPtr == 0 {
		return false
	}

	hInst := getModuleHandle()
	classe := utf16(cfgClasse)
	classeJanela := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   cfgProcPtr,
		hInstance:     hInst,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: classe,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classeJanela)))

	h, _, _ := pCreateWindowExW.Call(
		uintptr(wsExToolWindow),
		uintptr(unsafe.Pointer(classe)),
		uintptr(unsafe.Pointer(utf16(txtCfgTitulo))),
		wsPopup,
		0, 0, tamanhoInicialJanelaCfg, tamanhoInicialJanelaCfg, app.hwnd, 0, hInst, 0,
	)
	if h == 0 {
		return false
	}

	cfgUI.hwnd = h
	aplicarCantosDWM(h)
	app.cfgCriarFilhos()

	return true
}

func (a *app) cfgCriarFilhos() {
	if cfgUI.filhosOK || cfgUI.hwnd == 0 {
		return
	}

	hInst := getModuleHandle()
	mkEdit := func(multiline bool, idControle int) uintptr {
		estilo := uintptr(wsChild | wsVisible | wsBorder | wsTabstop | wsGroup | esLeft | esAutohscroll)
		if multiline {
			estilo = uintptr(wsChild | wsVisible | wsBorder | wsTabstop | wsGroup | esLeft | esMultiline |
				esAutovscroll | esWantreturn | wsVScroll)
		}

		h, _, _ := pCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(utf16("EDIT"))),
			0, estilo,
			0, 0, tamanhoInicialFilhoCfg, tamanhoInicialFilhoCfg, cfgUI.hwnd, uintptr(idControle), hInst, 0,
		)

		return h
	}
	cfgUI.editFiltro = mkEdit(false, cfgCtlFiltro)
	cfgUI.editVPN = mkEdit(true, cfgCtlVPN)
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("COMBOBOX"))),
		0, uintptr(wsChild|wsVisible|wsBorder|wsTabstop|wsGroup|cbsDropdownlist|wsVScroll),
		0, 0, tamanhoInicialFilhoCfg, tamanhoInicialFilhoCfg, cfgUI.hwnd, uintptr(cfgCtlIdioma), hInst, 0,
	)

	cfgUI.comboIdioma = h
	for _, filho := range []uintptr{cfgUI.editFiltro, cfgUI.editVPN, cfgUI.comboIdioma} {
		if filho == 0 {
			continue
		}

		pSendMessageW.Call(filho, wmSetfont, a.fontUI, 1)
		pSetWindowTheme.Call(filho, uintptr(unsafe.Pointer(utf16("DarkMode_Explorer"))), 0)
	}

	for _, it := range idiomasOferecidos {
		pSendMessageW.Call(cfgUI.comboIdioma, cbAddstring, 0, uintptr(unsafe.Pointer(utf16(it.nome))))
	}

	pSendMessageW.Call(cfgUI.editFiltro, emSetcuebanner, 0, uintptr(unsafe.Pointer(utf16(txtCfgFiltro))))
	cfgUI.filhosOK = true
}

func (a *app) abrirCfg(secao string) {
	if !cfgCriarJanela() {
		return
	}

	cfgUI.secao = cfgSeccaoDe(secao)
	cfgUI.w = a.sc(780)
	cfgUI.h = a.sc(600)
	cfgUI.navW = a.sc(210)
	cfgUI.headH = a.sc(52)

	var area rectT
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0)

	if area.Height() > 0 && cfgUI.h > area.Height()-a.sc(40) {
		cfgUI.h = area.Height() - a.sc(40)
	}

	if area.Width() > 0 && cfgUI.w > area.Width()-a.sc(40) {
		cfgUI.w = area.Width() - a.sc(40)
	}

	x := area.Left + (area.Width()-cfgUI.w)/divisorMetadeCfg
	y := area.Top + (area.Height()-cfgUI.h)/divisorMetadeCfg
	cfgUI.scroll = 0
	cfgUI.hoverNav = -1
	cfgUI.hoverID = ""
	cfgUI.arrastaBarra = false
	cfgUI.pegaBarra = 0
	cfgUI.barHover = false
	cfgUI.tradOK = false
	cfgUI.idiomaPend = a.st.Idioma
	a.cfgSincronizarFilhos()
	a.cfgLayout()

	cfgUI.ativo = true
	pSetWindowPos.Call(cfgUI.hwnd, 0, uintptr(uint32(x)), uintptr(uint32(y)),
		uintptr(cfgUI.w), uintptr(cfgUI.h), swpShowWindow)
	pShowWindow.Call(cfgUI.hwnd, swShow)
	pSetTimer.Call(cfgUI.hwnd, cfgTimer, intervaloAssinaturaCfg, 0)
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

func (a *app) fecharCfg() {
	if !cfgUI.ativo {
		return
	}

	cfgUI.ativo = false
	cfgUI.arrastaBarra = false
	cfgUI.pegaBarra = 0
	pKillTimer.Call(cfgUI.hwnd, cfgTimer)
	pShowWindow.Call(cfgUI.hwnd, swHide)
	d2dCfg.soltar()

	if a.hwnd != 0 {
		pSetForegroundWindow.Call(a.hwnd)
	}
}

func (a *app) cfgSincronizarFilhos() {
	if !cfgUI.filhosOK {
		return
	}

	idx := 0

	for i, it := range idiomasOferecidos {
		if it.codigo == a.st.Idioma {
			idx = i

			break
		}
	}

	pSendMessageW.Call(cfgUI.comboIdioma, cbSetcursel, uintptr(idx), 0)

	if a.vpn != nil {
		pSetWindowTextW.Call(cfgUI.editVPN, uintptr(unsafe.Pointer(utf16(a.vpn.textoConfig()))))
	}

	pSetWindowTextW.Call(cfgUI.editFiltro, uintptr(unsafe.Pointer(utf16(cfgUI.filtro))))
}

func cfgConteudoLarg() int32 {
	a := theApp

	return cfgUI.w - cfgUI.navW - a.sc(16)*2
}

func (a *app) cfgAddZona(id, label string, r rectT) {
	cfgUI.zonas = append(cfgUI.zonas, cfgZona{id: id, label: label, r: r})
}

func (a *app) cfgAddTxt(s string, r rectT, cor uint32, fonte uintptr, flags uint32) {
	cfgUI.txts = append(cfgUI.txts, cfgTxt{s: s, r: r, cor: cor, fonte: fonte, flags: flags})
}

func (a *app) cfgBotaoMedido(id, label string, x, y int32) rectT {
	larg := d2dMain.larguraTexto(label, a.fontSm, 1) + a.sc(24)
	if larg < a.sc(72) {
		larg = a.sc(72)
	}

	r := rectT{x, y, x + larg, y + a.sc(30)}
	a.cfgAddZona(id, label, r)

	return r
}

// tituloSecaoCfg devolve o titulo da secao aberta.
func tituloSecaoCfg() string {
	switch cfgUI.secao {
	case cfgSecDown:
		return txtCfgSecDown
	case cfgSecTrad:
		return txtCfgSecTrad
	case cfgSecCfg:
		return txtCfgSecCfg
	case cfgSecDados:
		return txtCfgSecDados
	case cfgSecVPN:
		return txtCfgSecVPN
	}

	return ""
}

// desenharSecaoCfg despacha o layout da secao aberta.
func (a *app) desenharSecaoCfg(posX, larg, y int32) int32 {
	switch cfgUI.secao {
	case cfgSecDown:
		return a.cfgLayoutDown(posX, larg, y)
	case cfgSecTrad:
		return a.cfgLayoutTrad(posX, larg, y)
	case cfgSecCfg:
		return a.cfgLayoutCfg(posX, larg, y)
	case cfgSecDados:
		return a.cfgLayoutDados(posX, larg, y)
	case cfgSecVPN:
		return a.cfgLayoutVPN(posX, larg, y)
	}

	return y
}

func (a *app) cfgLayout() {
	cfgUI.zonas = cfgUI.zonas[:0]
	cfgUI.txts = cfgUI.txts[:0]
	cfgUI.bars = cfgUI.bars[:0]
	cfgUI.downs = cfgUI.downs[:0]
	pad := a.sc(16)
	cfgUI.fechar = rectT{cfgUI.w - pad - a.sc(28), a.sc(12), cfgUI.w - pad, a.sc(40)}
	posX := cfgUI.navW + pad
	larg := cfgConteudoLarg()
	y := cfgUI.headH + pad - cfgUI.scroll
	tit := tituloSecaoCfg()

	a.cfgAddTxt(tit, rectT{posX, y, posX + larg, y + a.sc(30)}, corTexto, a.fontUI, dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(34)

	y = a.desenharSecaoCfg(posX, larg, y)

	total := y + cfgUI.scroll - (cfgUI.headH + pad)
	vis := cfgUI.h - cfgUI.headH - pad*dobroMargemCfg

	cfgUI.maxScroll = max(total-vis, 0)

	if cfgUI.scroll > cfgUI.maxScroll {
		cfgUI.scroll = cfgUI.maxScroll
	}

	a.cfgPosicionarFilhos(posX, larg)
}

func (a *app) cfgNota(texto string, posX, larg, y int32) int32 {
	if strings.TrimSpace(texto) == "" {
		return y
	}

	alt := d2dMain.alturaTextoQuebrado(texto, a.fontSm, larg)
	if alt < a.sc(18) {
		alt = a.sc(18)
	}

	a.cfgAddTxt(texto, rectT{posX, y, posX + larg, y + alt}, corTextoFaint, a.fontSm, dtLeft|dtWordbreak|dtNoPrefix)

	return y + alt + a.sc(10)
}

func (a *app) cfgCheck(id, label string, _ bool, posX, larg, y int32) int32 {
	r := rectT{posX, y, posX + larg, y + a.sc(28)}
	a.cfgAddZona(id, label, r)

	return y + a.sc(28) + a.sc(4)
}

func (a *app) cfgStep(id, label, valor string, posX, larg, y int32) int32 {
	a.cfgAddTxt(label, rectT{posX, y, posX + larg - a.sc(150), y + a.sc(28)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix|dtVCenter)
	bx := posX + larg - a.sc(150)
	a.cfgAddZona(id+":-", "−", rectT{bx, y, bx + a.sc(34), y + a.sc(28)})
	a.cfgAddTxt(valor, rectT{bx + a.sc(34), y, bx + a.sc(116), y + a.sc(28)}, corTexto, a.fontUI,
		dtCenter|dtSingleLine|dtNoPrefix|dtVCenter)
	a.cfgAddZona(id+":+", "+", rectT{bx + a.sc(116), y, bx + a.sc(150), y + a.sc(28)})

	return y + a.sc(28) + a.sc(8)
}

// filtrarDownloadsCfg filtra e ordena os downloads pelo filtro.
func filtrarDownloadsCfg(a *app) []downloadAtivo {
	lista := a.downloadsAtivos()
	filtro := strings.ToLower(strings.TrimSpace(cfgUI.filtro))

	var itens []downloadAtivo

	for _, d := range lista {
		if filtro == "" || strings.Contains(strings.ToLower(d.Nome+" "+d.URI), filtro) {
			itens = append(itens, d)
		}
	}

	sort.Slice(itens, func(i, j int) bool { return itens[i].ID > itens[j].ID })

	return itens
}

// avisarListaVaziaDownCfg desenha o vazio da lista de downloads.
func (a *app) avisarListaVaziaDownCfg(posX, larg, posY int32) int32 {
	msg := txtCfgDownVazio
	if strings.TrimSpace(cfgUI.filtro) != "" {
		msg = txtCfgDownSemFiltro
	}

	a.cfgAddTxt(
		msg, rectT{posX, posY, posX + larg, posY + a.sc(24)}, corTextoFaint, a.fontSm, dtLeft|dtSingleLine|dtNoPrefix,
	)

	return posY + a.sc(24)
}

// registrarBarraDownCfg usa sc() para a barra sem quebrar o layout.
func (a *app) registrarBarraDownCfg(posX, larg, ye, alt int32, d downloadAtivo) rectT {
	bar := rectT{posX + a.sc(2), ye + alt - a.sc(14), posX + larg - a.sc(2), ye + alt - a.sc(8)}

	cfgUI.bars = append(cfgUI.bars, cfgBar{r: bar, pct: 0})
	if d.Total > 0 {
		cfgUI.bars[len(cfgUI.bars)-1].pct = int(d.Recebido * 100 / d.Total)
	}

	return bar
}

// botoesDownAtivo desenha os botoes do download em andamento.
func botoesDownAtivo(d downloadAtivo, por func(id, rotulo string)) {
	if strings.TrimSpace(d.URI) != "" {
		por("down:copy", txtDownCopiarURL)
	}

	por("down:pause", txtDownPausar)
	por("down:cancel", txtDownCancelar)
}

// botoesDownPausado desenha os botoes do download pausado.
func botoesDownPausado(por func(id, rotulo string)) {
	por("down:resume", txtDownRetomar)
	por("down:cancel", txtDownCancelar)
	por("down:remove", txtDownRemover)
}

// botoesDownConcluido desenha os botoes do download concluido.
func botoesDownConcluido(d downloadAtivo, por func(id, rotulo string)) {
	if strings.TrimSpace(d.Caminho) != "" {
		por("down:open", txtDownAbrir)
		por("down:reveal", txtDownRevelar)
	}

	por("down:remove", txtDownRemover)
}

// botoesAcaoDownCfg desenha os botoes conforme o estado do download.
func (a *app) botoesAcaoDownCfg(download downloadAtivo, _, _ int32, por func(id, rotulo string)) {
	switch {
	case downloadAtivoEmAndamento(download):
		botoesDownAtivo(download, por)
	case downloadPausadoRetomavel(download):
		botoesDownPausado(por)
	case download.Estado == estadoDownConcluido:
		botoesDownConcluido(download, por)
	default:
		por("down:retry", txtDownTentar)
		por("down:remove", txtDownRemover)
	}
}

// downloadAtivoEmAndamento diz se o download esta baixando.
func downloadAtivoEmAndamento(d downloadAtivo) bool {
	return !d.Cancelado && d.Estado == 0
}

// downloadPausadoRetomavel diz se o download pode retomar.
func downloadPausadoRetomavel(d downloadAtivo) bool {
	return !d.Cancelado && d.Estado == 1 && d.PodeRetomar && !d.Reiniciado
}

// desenharLinhaDownCfg desenha uma linha da lista de downloads.
func (a *app) desenharLinhaDownCfg(download downloadAtivo, posX, larg, posY int32) int32 {
	alt := a.sc(86)
	retangulo := rectT{posX, posY, posX + larg, posY + alt}
	bar := rectT{}

	mostrar := !download.Cancelado && download.Estado == 0
	if mostrar {
		bar = a.registrarBarraDownCfg(posX, larg, posY, alt, download)
	}

	cfgUI.downs = append(cfgUI.downs, cfgDownRow{dl: download, r: retangulo, rBar: bar, mostrarBar: mostrar})
	posBotao := posX + larg
	por := func(id, rotulo string) {
		lb := a.cfgLarguraBotao(rotulo)
		posBotao -= lb + a.sc(6)
		a.cfgAddZona(
			id+":"+strconv.Itoa(download.ID), rotulo, rectT{posBotao, posY + a.sc(6), posBotao + lb, posY + a.sc(30)},
		)
	}

	a.botoesAcaoDownCfg(download, posY, posBotao, por)

	return posY + alt + a.sc(8)
}

func (a *app) cfgLayoutDown(posX, larg, y int32) int32 {
	posY := y
	r1 := a.cfgBotaoMedido("btn:pastaDown", txtCfgPastaDown, posX, posY)
	a.cfgBotaoMedido("btn:limparDown", txtCfgLimparDown, r1.Right+a.sc(8), posY)
	posY += a.sc(30) + a.sc(6)
	posY += a.sc(30) + a.sc(10)
	itens := filtrarDownloadsCfg(a)

	if len(itens) == 0 {
		return a.avisarListaVaziaDownCfg(posX, larg, posY)
	}

	for _, d := range itens {
		posY = a.desenharLinhaDownCfg(d, posX, larg, posY)
	}

	return posY
}

func (a *app) cfgLarguraBotao(rotulo string) int32 {
	larg := d2dMain.larguraTexto(rotulo, a.fontSm, 1) + a.sc(24)
	if larg < a.sc(48) {
		larg = a.sc(48)
	}

	return larg
}

func (a *app) cfgLayoutTrad(posX, larg, y int32) int32 {
	a.cfgAddTxt(txtCfgIdioma, rectT{posX, y, posX + larg, y + a.sc(24)}, corTextoFraco, a.fontSm,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(26)
	y += a.sc(32) + a.sc(10)
	a.cfgBotaoMedido("btn:salvarTrad", txtCfgSalvarTrad, posX, y)

	y += a.sc(30) + a.sc(10)
	if cfgUI.tradOK {
		a.cfgAddTxt(txtCfgTradSalva, rectT{posX, y, posX + larg, y + a.sc(22)}, corSuccess, a.fontSm,
			dtLeft|dtSingleLine|dtNoPrefix)
		y += a.sc(22) + a.sc(6)
	}

	y = a.cfgNota(txtCfgTradNota, posX, larg, y)

	return y
}

func (a *app) cfgLayoutCfg(posX, larg, y int32) int32 {
	y = a.cfgStep("step:zoom", txtCfgZoomTexto,
		strconv.Itoa(int(a.st.Zoom*fatorPorcentagemCfg+meioArredondamento))+"%", posX, larg, y)
	y = a.cfgStep("step:zoomIA", txtCfgZoomIA, strconv.Itoa(int(a.st.ZoomPainelIA+meioArredondamento))+"%", posX, larg, y)

	y += a.sc(6)
	if a.st.ForceDark {
		y = a.cfgCheck("chk:dark", txtCfgDark, true, posX, larg, y)
	} else {
		y = a.cfgCheck("chk:dark", txtCfgDark, false, posX, larg, y)
	}

	y = a.cfgCheck("chk:chat", txtCfgChat, a.st.RightVisible, posX, larg, y)
	y = a.cfgCheck("chk:sw", txtCfgSemSW, a.st.SemServiceWorker, posX, larg, y)
	y = a.cfgCheck("chk:gpu", txtCfgSemGPU, a.st.SemGPU, posX, larg, y)
	y = a.cfgCheck("chk:eco", txtCfgEcoMax, a.st.EconomiaMax, posX, larg, y)
	y = a.cfgCheck("chk:limpeza", txtCfgLimpezaAcao, a.st.LimpezaAcao, posX, larg, y)
	y = a.cfgCheck("chk:clipauto", txtCfgLimparClip, a.st.LimparClip, posX, larg, y)
	y += a.sc(6)
	a.cfgBotaoMedido("btn:pastaDados", txtCfgPastaDados, posX, y)
	y += a.sc(30) + a.sc(10)
	y = a.cfgNota(txtCfgNota, posX, larg, y)

	return y
}

func (a *app) cfgLayoutDados(posX, larg, y int32) int32 {
	y = a.cfgCockpitFiltro(posX, larg, y)
	y = a.cfgCockpitResumo(posX, larg, y)
	y = a.cfgCockpitBotoes(posX, y)
	y = a.cfgCockpitCookies(posX, larg, y)
	y = a.cfgCockpitCache(posX, larg, y)
	y = a.cfgCockpitSites(posX, larg, y)
	y = a.cfgCockpitTudo(posX, larg, y)
	y = a.cfgCockpitPasta(posX, larg, y)

	return y
}

// cfgCockpitResumo mostra o total de cookies, sites e tamanho.
func (a *app) cfgCockpitResumo(posX, larg, y int32) int32 {
	if cfgUI.cookiesLendo && len(cfgUI.cookiesCockpit) == 0 {
		y = a.cfgNota(txtCockpitCarregando, posX, larg, y)

		return y
	}

	total := len(cfgUI.cookiesCockpit)
	resumo := fmt.Sprintf(txtFmtCockpitResumo, total, len(dominiosCockpit(cfgUI.cookiesCockpit)),
		cfgTamanho(tamanhoCookiesCockpit(cfgUI.cookiesCockpit)))

	a.cfgAddTxt(resumo, rectT{posX, y, posX + larg, y + a.sc(24)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)

	if strings.TrimSpace(cfgUI.cookiesErro) != "" {
		y = a.cfgNota(cfgUI.cookiesErro, posX, larg, y)
	}

	return y
}

// cfgCockpitFiltro rotula a caixa de filtro do cockpit.
func (a *app) cfgCockpitFiltro(posX, larg, y int32) int32 {
	a.cfgAddTxt(txtCockpitFiltro, rectT{posX, y, posX + larg, y + a.sc(22)}, corTextoFaint, a.fontSm,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(24)
	y += a.sc(28) + a.sc(10)

	return y
}

// cfgCockpitBotoes desenha recarregar e apagar todos os cookies.
func (a *app) cfgCockpitBotoes(posX, y int32) int32 {
	b1 := a.cfgBotaoMedido("btn:ckRecarregar", txtCockpitRecarregar, posX, y)
	a.cfgBotaoMedido("btn:ckApagarTodos", txtCockpitApagarTodos, b1.Right+a.sc(8), y)

	return y + a.sc(30) + a.sc(10)
}

// cfgCockpitCookies desenha os dominios com apagar e os cookies com x.
func (a *app) cfgCockpitCookies(posX, larg, y int32) int32 {
	a.cfgAddTxt(txtCockpitCookies, rectT{posX, y, posX + larg, y + a.sc(24)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)

	mostrados := filtrarCookiesCockpit(cfgUI.cookiesCockpit, cfgUI.filtro)
	if len(mostrados) == 0 {
		y = a.cfgNota(txtCockpitVazio, posX, larg, y)

		return y
	}

	if len(mostrados) > maxCookiesCockpit {
		y = a.cfgNota(fmt.Sprintf(txtFmtCockpitMaisCookies, maxCookiesCockpit, len(mostrados)), posX, larg, y)
		mostrados = mostrados[:maxCookiesCockpit]
	}

	restantes := mostrados

	for len(restantes) > 0 {
		dominio := restantes[0].Dominio
		y = a.cfgCockpitDominio(posX, larg, y, dominio)

		var fora []cockpitCookie

		for _, c := range restantes {
			if c.Dominio == dominio {
				y = a.cfgCockpitCookie(posX, larg, y, c)
			} else {
				fora = append(fora, c)
			}
		}

		restantes = fora
	}

	return y
}

// cfgCockpitDominio desenha o cabecalho do dominio com apagar.
func (a *app) cfgCockpitDominio(posX, larg, y int32, dominio string) int32 {
	quantos := 0

	for _, c := range cfgUI.cookiesCockpit {
		if c.Dominio == dominio {
			quantos++
		}
	}

	rotulo := fmt.Sprintf("%s (%s)", dominio, quant(quantos, "cookie", idCookies))
	a.cfgAddTxt(rotulo, rectT{posX, y, posX + larg - a.sc(150), y + a.sc(26)}, corTexto, a.fontSm,
		dtLeft|dtSingleLine|dtNoPrefix)

	largura := a.cfgLarguraBotao(txtCockpitApagarDom)
	r := rectT{posX + larg - largura, y, posX + larg, y + a.sc(26)}
	a.cfgAddZona("ck:dom:"+dominio, txtCockpitApagarDom, r)

	return y + a.sc(26) + a.sc(4)
}

// cfgCockpitCookie desenha um cookie com o x que apaga so ele.
func (a *app) cfgCockpitCookie(posX, larg, y int32, cookie cockpitCookie) int32 {
	idx := -1

	for i := range cfgUI.cookiesCockpit {
		if cfgUI.cookiesCockpit[i].Nome == cookie.Nome && cfgUI.cookiesCockpit[i].Dominio == cookie.Dominio &&
			cfgUI.cookiesCockpit[i].Caminho == cookie.Caminho {
			idx = i

			break
		}
	}

	linha := fmt.Sprintf(txtFmtCockpitCookie, cookie.Nome, expiraCookieCockpit(cookie),
		cfgTamanho(tamanhoCookieCockpit(cookie)))
	a.cfgAddTxt(linha, rectT{posX + a.sc(16), y, posX + larg - a.sc(36), y + a.sc(24)}, corTextoFaint, a.fontSm,
		dtLeft|dtSingleLine|dtNoPrefix)

	if idx >= 0 {
		r := rectT{posX + larg - a.sc(28), y, posX + larg, y + a.sc(24)}
		a.cfgAddZona("ck:x:"+strconv.Itoa(idx), "", r)
	}

	return y + a.sc(24) + a.sc(2)
}

// cfgCockpitCache desenha a secao de cache.
func (a *app) cfgCockpitCache(posX, larg, y int32) int32 {
	y += a.sc(6)
	a.cfgAddTxt(txtCockpitCache, rectT{posX, y, posX + larg, y + a.sc(24)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)
	a.cfgBotaoMedido("btn:cacheLimpar", txtCockpitLimparCache, posX, y)
	y += a.sc(30) + a.sc(4)
	y = a.cfgNota(txtCockpitNotaCache, posX, larg, y)

	return y
}

// cfgCockpitSites desenha a limpeza por site.
func (a *app) cfgCockpitSites(posX, larg, y int32) int32 {
	y += a.sc(6)
	a.cfgAddTxt(txtCockpitSite, rectT{posX, y, posX + larg, y + a.sc(24)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)

	dominios := dominiosCockpit(cfgUI.cookiesCockpit)
	if len(dominios) > maxOrigensCockpit {
		y = a.cfgNota(fmt.Sprintf(txtFmtCockpitMaisOrigens, len(dominios)-maxOrigensCockpit), posX, larg, y)
		dominios = dominios[:maxOrigensCockpit]
	}

	for _, dominio := range dominios {
		origem := origemDeDominioCockpit(dominio)
		a.cfgAddTxt(origem, rectT{posX, y, posX + larg - a.sc(140), y + a.sc(26)}, corTextoFaint, a.fontSm,
			dtLeft|dtSingleLine|dtNoPrefix)

		largura := a.cfgLarguraBotao(txtCockpitLimparSite)
		r := rectT{posX + larg - largura, y, posX + larg, y + a.sc(26)}
		a.cfgAddZona("ck:site:"+origem, txtCockpitLimparSite, r)
		y += a.sc(26) + a.sc(4)
	}

	y = a.cfgNota(txtCockpitNotaSite, posX, larg, y)

	return y
}

// cfgCockpitTudo desenha o botao vermelho que apaga cookies e cache.
func (a *app) cfgCockpitTudo(posX, larg, y int32) int32 {
	y += a.sc(6)
	a.cfgAddTxt(txtCockpitTudo, rectT{posX, y, posX + larg, y + a.sc(24)}, corTexto, a.fontUI,
		dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)
	a.cfgBotaoMedido("btn:excluirDados", txtCockpitApagarTudo, posX, y)
	y += a.sc(30) + a.sc(4)
	y = a.cfgNota(txtCockpitNotaTudo, posX, larg, y)

	return y
}

// cfgCockpitPasta desenha o atalho da pasta e a nota final.
func (a *app) cfgCockpitPasta(posX, larg, y int32) int32 {
	y += a.sc(6)
	a.cfgBotaoMedido("btn:pastaDados", txtCfgPastaDados, posX, y)
	y += a.sc(30) + a.sc(10)
	y = a.cfgNota(txtCockpitNota, posX, larg, y)

	return y
}

func (a *app) cfgLayoutVPN(posX, larg, y int32) int32 {
	estado := ""
	cor := uint32(corTextoFaint)

	if a.vpn != nil {
		a.vpn.mu.RLock()

		switch {
		case a.vpn.connected:
			estado = txtCfgVPNStOn
			if a.vpn.endpoint != "" {
				estado += txtSeparador + a.vpn.endpoint
			}

			cor = corSuccess
		case a.vpn.connecting:
			estado = txtCfgVPNStGo
			cor = corWarning
		default:
			estado = txtCfgVPNStOff
			if a.vpn.lastErr != "" {
				estado += txtSeparador + a.vpn.lastErr
			}
		}

		a.vpn.mu.RUnlock()
	}

	a.cfgAddTxt(estado, rectT{posX, y, posX + larg, y + a.sc(24)}, cor, a.fontUI, dtLeft|dtSingleLine|dtNoPrefix)
	y += a.sc(28)
	y += a.sc(150) + a.sc(10)
	b1 := a.cfgBotaoMedido("btn:vpnOn", txtCfgConectar, posX, y)
	a.cfgBotaoMedido("btn:vpnOff", txtCfgDesconectar, b1.Right+a.sc(8), y)
	y += a.sc(30) + a.sc(10)
	kill := false

	if a.vpn != nil {
		a.vpn.mu.RLock()
		kill = a.vpn.killSwitch
		a.vpn.mu.RUnlock()
	}

	y = a.cfgCheck("chk:kill", txtCfgKill, kill, posX, larg, y)
	y += a.sc(6)
	y = a.cfgNota(txtCfgVPNNota, posX, larg, y)
	y = a.cfgNota(txtCfgVPNDNS, posX, larg, y)

	return y
}

// cfgMostrarFilho posiciona ou esconde um controle nativo do painel.
func cfgMostrarFilho(h uintptr, x, posY, largura, altura int32, vis bool) {
	if h == 0 {
		return
	}

	if !vis {
		pSetWindowPos.Call(h, 0, 0, 0, 0, 0, swpNoZOrder|swpNoMove|swpNoSize|swpHideWindow)
		pShowWindow.Call(h, swHide)

		return
	}

	pSetWindowPos.Call(h, 0, uintptr(uint32(x)), uintptr(uint32(posY)), uintptr(largura), uintptr(altura),
		swpNoZOrder|swpNoActivate|swpShowWindow)
	pShowWindow.Call(h, swShowNoActivate)
}

func (a *app) cfgPosicionarFilhos(posX, larg int32) {
	if !cfgUI.filhosOK {
		return
	}

	y := cfgUI.headH + a.sc(16) - cfgUI.scroll

	switch cfgUI.secao {
	case cfgSecDown:
		posY := y + a.sc(30) + a.sc(6) + a.sc(30) + a.sc(10)
		cfgMostrarFilho(cfgUI.editFiltro, posX, posY, larg, a.sc(28), true)
		cfgMostrarFilho(cfgUI.comboIdioma, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.editVPN, 0, 0, 0, 0, false)
	case cfgSecTrad:
		posY := y + a.sc(34) + a.sc(26)

		largura := larg
		if largura > a.sc(340) {
			largura = a.sc(340)
		}

		cfgMostrarFilho(cfgUI.editFiltro, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.comboIdioma, posX, posY, largura, a.sc(200), true)
		cfgMostrarFilho(cfgUI.editVPN, 0, 0, 0, 0, false)
	case cfgSecVPN:
		posY := y + a.sc(34) + a.sc(28)

		cfgMostrarFilho(cfgUI.editFiltro, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.comboIdioma, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.editVPN, posX, posY, larg, a.sc(150), true)
	case cfgSecCfg:
		// Secao sem painel nesta vista: esconde tudo, como o default.
		cfgMostrarFilho(cfgUI.editFiltro, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.comboIdioma, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.editVPN, 0, 0, 0, 0, false)
	case cfgSecDados:
		cfgMostrarFilho(cfgUI.comboIdioma, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.editVPN, 0, 0, 0, 0, false)

		posY := y + a.sc(34) + a.sc(24)
		cfgMostrarFilho(cfgUI.editFiltro, posX, posY, larg, a.sc(28), true)
		pSendMessageW.Call(cfgUI.editFiltro, emSetcuebanner, 0, uintptr(unsafe.Pointer(utf16(txtCockpitFiltro))))
	default:
		cfgMostrarFilho(cfgUI.editFiltro, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.comboIdioma, 0, 0, 0, 0, false)
		cfgMostrarFilho(cfgUI.editVPN, 0, 0, 0, 0, false)
	}
}

func cfgZonaEm(x, y int32) *cfgZona {
	for i := range cfgUI.zonas {
		z := &cfgUI.zonas[i]
		if dentroRect(z.r, x, y) {
			return z
		}
	}

	return nil
}

func cfgNavEm(y int32) int {
	app := theApp
	if app == nil {
		return -1
	}

	top := cfgUI.headH + app.sc(8)
	for i := range cfgSecoes() {
		r := rectT{app.sc(10), top + int32(i)*app.sc(40), cfgUI.navW - app.sc(10), top + int32(i)*app.sc(40) + app.sc(36)}
		if y >= r.Top && y < r.Bottom {
			return i
		}
	}

	return -1
}

func (a *app) cfgAcionou(id string) {
	if id == "x" {
		a.fecharCfg()

		return
	}

	if after, ok := strings.CutPrefix(id, "nav:"); ok {
		a.navegarSecaoCfg(after)

		return
	}

	if a.acionarBotaoCfg(id) {
		return
	}

	a.acionarPrefixoCfg(id)
}

// navegarSecaoCfg troca a secao aberta pelo menu lateral.
func (a *app) navegarSecaoCfg(after string) {
	n, _ := strconv.Atoi(after)
	if n < 0 || n >= len(cfgSecoes()) {
		return
	}

	cfgUI.secao = cfgSecao(n)
	cfgUI.scroll = 0
	cfgUI.hoverID = ""
	cfgUI.arrastaBarra = false
	cfgUI.pegaBarra = 0
	cfgUI.tradOK = false
	cfgUI.downAss = ""

	if cfgUI.secao == cfgSecDados {
		a.cfgRecarregarCookies()
	}

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

// salvarIdiomaCfg grava o idioma escolhido no combo.
func (a *app) salvarIdiomaCfg() {
	idx, _, _ := pSendMessageW.Call(cfgUI.comboIdioma, cbGetcursel, 0, 0)
	if int(idx) >= 0 && int(idx) < len(idiomasOferecidos) {
		a.st.Idioma = idiomasOferecidos[int(idx)].codigo
		cfgUI.idiomaPend = a.st.Idioma
		a.persist()
	}

	cfgUI.tradOK = true

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

// cfgCockpitSemPainel avisa e redesenha quando nao ha painel pronto.
func (a *app) cfgCockpitSemPainel() {
	cfgUI.cookiesErro = txtCockpitSemPainel

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

// acionarBotaoCfg despacha os botoes da secao aberta.
func (a *app) acionarBotaoCfg(id string) bool {
	if a.acionarBotaoCfgA(id) {
		return true
	}

	return a.acionarBotaoCfgB(id)
}

// acionarBotaoCfgA trata pastas, downloads e recarga de cookies.
func (a *app) acionarBotaoCfgA(id string) bool {
	switch id {
	case "btn:pastaDown":
		d := pastaDownloads()
		if d == "" {
			d = dataDir
		}

		a.abrirPasta(d)

		return true
	case "btn:limparDown":
		a.limparDownloads()
		a.cfgLayout()
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return true
	case "btn:salvarTrad":
		a.salvarIdiomaCfg()

		return true
	case "btn:pastaDados":
		a.abrirPasta(dataDir)

		return true
	case "btn:ckRecarregar":
		a.cfgRecarregarCookies()

		return true
	}

	return false
}

// acionarBotaoCfgB trata limpeza de dados e VPN.
func (a *app) acionarBotaoCfgB(id string) bool {
	switch id {
	case "btn:ckApagarTodos":
		a.cfgApagarTodosCookies()

		return true
	case "btn:cacheLimpar":
		a.cfgLimparCacheFundo()

		return true
	case "btn:excluirDados":
		a.cfgApagarTudoFundo()

		return true
	case "btn:vpnOn":
		a.vpnConectar(cfgTextoControle(cfgUI.editVPN))
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return true
	case "btn:vpnOff":
		a.vpnDesconectar()
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)

		return true
	}

	return false
}

// acionarPrefixoCfg despacha os ids com prefixo chk/step/down/ck.
func (a *app) acionarPrefixoCfg(id string) {
	switch {
	case strings.HasPrefix(id, "chk:"):
		a.cfgAlternarCheck(strings.TrimPrefix(id, "chk:"))
	case strings.HasPrefix(id, "step:"):
		a.cfgPasso(id)
	case strings.HasPrefix(id, "down:"):
		a.cfgAcaoDown(id)
	case strings.HasPrefix(id, "ck:"):
		a.cfgAcaoCockpit(strings.TrimPrefix(id, "ck:"))
	}
}

// cfgAcaoCockpit despacha apagar um cookie, um dominio ou um site.
func (a *app) cfgAcaoCockpit(acao string) {
	qual, valor, _ := strings.Cut(acao, ":")

	p := paneParaCockpit(a)
	if p == nil {
		a.cfgCockpitSemPainel()

		return
	}

	switch qual {
	case "x":
		a.cfgApagarCookieIdx(p, valor)
	case "dom":
		a.cfgApagarDominioCockpit(p, valor)
	case "site":
		a.cfgLimparSiteCockpit(p, valor)
	}
}

// cfgApagarCookieIdx apaga o cookie da posicao na grade.
func (a *app) cfgApagarCookieIdx(p *pane, valor string) {
	idx, err := strconv.Atoi(valor)
	if err != nil || idx < 0 || idx >= len(cfgUI.cookiesCockpit) {
		return
	}

	c := cfgUI.cookiesCockpit[idx]

	go func() {
		_ = apagarCookieNoPane(a, p, c.Nome, c.Dominio, c.Caminho)
		a.depois(func() { a.cfgRecarregarCookies() })
	}()
}

// cfgApagarDominioCockpit apaga todos os cookies do dominio.
func (a *app) cfgApagarDominioCockpit(p *pane, dominio string) {
	if strings.TrimSpace(dominio) == "" {
		return
	}

	lista := append([]cockpitCookie(nil), cfgUI.cookiesCockpit...)

	go func() {
		apagarCookiesDominioNoPane(a, p, lista, dominio)
		a.depois(func() { a.cfgRecarregarCookies() })
	}()
}

// cfgLimparSiteCockpit limpa cookies, storage e cache da origem.
func (a *app) cfgLimparSiteCockpit(p *pane, origem string) {
	if strings.TrimSpace(origem) == "" {
		return
	}

	go func() {
		err := limparOrigemNoPane(a, p, cockpitDominioDeOrigem(origem))
		a.depois(func() {
			if err != nil {
				a.dizerErrof("%s", err.Error())
				a.invalidateBar()

				return
			}

			a.cfgRecarregarCookies()
		})
	}()
}

// cockpitDominioDeOrigem tira o esquema da origem para o dominio.
func cockpitDominioDeOrigem(origem string) string {
	dominio := strings.TrimSpace(origem)
	dominio = strings.TrimPrefix(dominio, esquemaHTTPS+"://")
	dominio = strings.TrimPrefix(dominio, esquemaHTTP+"://")

	return dominio
}

// alternarCheckVisual troca as opcoes de aparencia e motor.
func (a *app) alternarCheckVisual(qual string) bool {
	switch qual {
	case "dark":
		a.toggleForceDark()

		return true
	case "sw":
		a.st.SemServiceWorker = !a.st.SemServiceWorker
		a.persist()

		return true
	case "gpu":
		a.st.SemGPU = !a.st.SemGPU
		a.persist()

		return true
	}

	return false
}

// alternarCheckComportamento troca chat, economia e limpeza.
func (a *app) alternarCheckComportamento(qual string) bool {
	switch qual {
	case idChat:
		a.toggleChat()
		a.persist()

		return true
	case "eco":
		a.st.EconomiaMax = !a.st.EconomiaMax
		a.persist()

		return true
	case "limpeza":
		a.st.LimpezaAcao = !a.st.LimpezaAcao
		a.persist()

		return true
	case "clipauto":
		a.st.LimparClip = !a.st.LimparClip
		clipAutoLimpar = a.st.LimparClip
		a.persist()

		return true
	}

	return false
}

// alternarCheckDados troca os checks locais da janela.
func (a *app) alternarCheckDados(qual string) bool {
	if qual == "kill" {
		a.vpnKillSwitch(!a.cfgKillAtual())

		return true
	}

	return false
}

func (a *app) cfgAlternarCheck(qual string) {
	switch {
	case a.alternarCheckVisual(qual):
	case a.alternarCheckComportamento(qual):
	default:
		a.alternarCheckDados(qual)
	}

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

func (a *app) cfgKillAtual() bool {
	if a == nil || a.vpn == nil {
		return false
	}

	a.vpn.mu.RLock()
	defer a.vpn.mu.RUnlock()

	return a.vpn.killSwitch
}

func (a *app) cfgPasso(id string) {
	partes := strings.Split(id, ":")
	if len(partes) != totalPartesAcaoCfg {
		return
	}

	qual, sinal := partes[1], partes[2]
	switch qual {
	case idZoom:
		percentual := int(a.st.Zoom*fatorPorcentagemCfg + meioArredondamento)
		if sinal == "+" {
			percentual += 5
		} else {
			percentual -= 5
		}

		if percentual < zoomMinimoTexto {
			percentual = zoomMinimoTexto
		}

		if percentual > zoomMaximoTexto {
			percentual = zoomMaximoTexto
		}

		a.setZoom(float64(percentual) / fatorPorcentagemCfg)
	case "zoomIA":
		percentual := int(a.st.ZoomPainelIA + meioArredondamento)
		if sinal == "+" {
			percentual++
		} else {
			percentual--
		}

		if percentual < zoomMinimoPainelIA {
			percentual = zoomMinimoPainelIA
		}

		if percentual > zoomMaximoPainelIA {
			percentual = zoomMaximoPainelIA
		}

		a.st.ZoomPainelIA = float64(percentual)
		a.persist()
	}

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

// checkOnVisual le os checks de aparencia e motor.
func (a *app) checkOnVisual(id string) (bool, bool) {
	switch id {
	case "chk:dark":
		return a.st.ForceDark, true
	case "chk:sw":
		return a.st.SemServiceWorker, true
	case "chk:gpu":
		return a.st.SemGPU, true
	}

	return false, false
}

// checkOnComportamento le os checks de comportamento.
func (a *app) checkOnComportamento(id string) (bool, bool) {
	switch id {
	case "chk:chat":
		return a.st.RightVisible, true
	case "chk:eco":
		return a.st.EconomiaMax, true
	case "chk:limpeza":
		return a.st.LimpezaAcao, true
	case "chk:clipauto":
		return a.st.LimparClip, true
	}

	return false, false
}

// checkOnDados le os checks locais da janela.
func (a *app) checkOnDados(id string) (bool, bool) {
	if id == "chk:kill" {
		return a.cfgKillAtual(), true
	}

	return false, false
}

func (a *app) cfgCheckOn(id string) bool {
	if v, ok := a.checkOnVisual(id); ok {
		return v
	}

	if v, ok := a.checkOnComportamento(id); ok {
		return v
	}

	if v, ok := a.checkOnDados(id); ok {
		return v
	}

	return false
}

// caminhoDownloadCfg acha o caminho do download pelo ID.
func caminhoDownloadCfg(n int) string {
	for _, r := range cfgUI.downs {
		if r.dl.ID == n {
			return r.dl.Caminho
		}
	}

	return ""
}

// executarAcaoBasicaDownloadCfg despacha pausa, retomada e cancelamento.
func (a *app) executarAcaoBasicaDownloadCfg(qual string, idDownload int) bool {
	switch qual {
	case "pause":
		a.pausarDownload(idDownload)

		return true
	case "resume":
		a.retomarDownload(idDownload)

		return true
	case "cancel":
		a.cancelarDownload(idDownload)

		return true
	case idCopy:
		a.copiarURLDownload(idDownload)

		return true
	}

	return false
}

// executarAcaoArquivoDownloadCfg despacha abrir, revelar e refazer.
func (a *app) executarAcaoArquivoDownloadCfg(qual string, idDownload int, caminho string) bool {
	switch qual {
	case "open":
		if caminho != "" {
			a.abrirPasta(caminho)
		}

		return true
	case "reveal":
		if caminho != "" {
			a.revelarArquivo(caminho)
		}

		return true
	case "retry":
		a.tentarDeNovo(idDownload)

		return true
	case "remove":
		a.removerDownload(idDownload, false)

		return true
	}

	return false
}

// executarAcaoDownloadCfg despacha a acao do download.
func (a *app) executarAcaoDownloadCfg(qual string, idDownload int, caminho string) {
	if a.executarAcaoBasicaDownloadCfg(qual, idDownload) {
		return
	}

	a.executarAcaoArquivoDownloadCfg(qual, idDownload, caminho)
}

func (a *app) cfgAcaoDown(id string) {
	partes := strings.Split(id, ":")
	if len(partes) != totalPartesAcaoCfg {
		return
	}

	n, _ := strconv.Atoi(partes[2])
	a.executarAcaoDownloadCfg(partes[1], n, caminhoDownloadCfg(n))

	a.cfgLayout()
	pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
}

func cfgDownEstado(download downloadAtivo) (string, uint32) {
	switch {
	case download.Cancelado:
		return txtDownCancelado, corDanger
	case download.Estado == estadoDownConcluido:
		return txtDownConcluido, corSuccess
	case download.Estado == 1 && download.PodeRetomar && !download.Reiniciado:
		return txtDownPausado, corWarning
	case download.Estado == 1:
		return txtDownInterromp, corDanger
	default:
		return txtDownBaixando, corAccentSoft
	}
}

// cfgAoPintar trata o wmPaint da janela de configuracao: retorno 0 = tratado.
func cfgAoPintar(aplicativo *app, hwnd uintptr) uintptr { //nolint:unparam
	if aplicativo != nil && cfgUI.ativo {
		if d2dCfg.rt == nil {
			d2dCfg.criar(hwnd, cfgUI.w, cfgUI.h)
		}

		if d2dCfg.rt != nil {
			d2dCfg.iniciar()
			aplicativo.paintCfgD2D()
			d2dCfg.apresentar()
		} else {
			pValidateRect.Call(hwnd, 0)
		}

		return 0
	}

	pValidateRect.Call(hwnd, 0)

	return 0
}

// cfgAoMouseMover atualiza o hover da navegacao e das zonas: retorno 0 = tratado.
func cfgAoMouseMover(aplicativo *app, hwnd uintptr, lparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	x, y := loword(lparam), hiword(lparam)

	if cfgUI.arrastaBarra {
		aplicativo.cfgArrastarBarra(y)

		return 0
	}

	nav := -1
	if x < cfgUI.navW {
		nav = cfgNavEm(y)
	}

	barHover := aplicativo.cfgZonaBarra(x, y) > 0

	var id string
	if z := cfgZonaEm(x, y+cfgUI.scroll); z != nil {
		id = z.id
	}

	if nav != cfgUI.hoverNav || id != cfgUI.hoverID || barHover != cfgUI.barHover {
		cfgUI.hoverNav = nav
		cfgUI.hoverID = id
		cfgUI.barHover = barHover

		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

// cfgAoClique trata o clique em fechar, navegacao ou zona: retorno 0 = tratado.
func cfgAoClique(aplicativo *app, hwnd uintptr, lparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	x, y := loword(lparam), hiword(lparam)
	if dentroRect(cfgUI.fechar, x, y) {
		aplicativo.cfgAcionou("x")

		return 0
	}

	if zona := aplicativo.cfgZonaBarra(x, y); zona > 0 {
		aplicativo.cfgCliqueBarra(zona, y)

		return 0
	}

	if x < cfgUI.navW {
		if n := cfgNavEm(y); n >= 0 {
			aplicativo.cfgAcionou("nav:" + strconv.Itoa(n))

			return 0
		}
	}

	if z := cfgZonaEm(x, y+cfgUI.scroll); z != nil {
		aplicativo.cfgAcionou(z.id)

		return 0
	}

	pSetFocus.Call(hwnd)

	return 0
}

// cfgAoRolar trata a roda do mouse na janela de configuracao: retorno 0 = tratado.
func cfgAoRolar(aplicativo *app, hwnd uintptr, wparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	delta := int32(int16(uint16(wparam >> deslocamentoWordAlta)))

	cfgUI.scroll -= delta / deltaRodaMouse * aplicativo.sc(56)
	if cfgUI.scroll < 0 {
		cfgUI.scroll = 0
	}

	if cfgUI.scroll > cfgUI.maxScroll {
		cfgUI.scroll = cfgUI.maxScroll
	}

	aplicativo.cfgPosicionarFilhos(cfgUI.navW+aplicativo.sc(16), cfgConteudoLarg())
	pInvalidateRect.Call(hwnd, 0, 0)

	return 0
}

// cfgAoSoltarCfg encerra o arrasto da barra: retorno 0 = tratado.
func cfgAoSoltarCfg(aplicativo *app, hwnd uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	if cfgUI.arrastaBarra {
		cfgUI.arrastaBarra = false
		cfgUI.pegaBarra = 0

		pReleaseCapture.Call()
		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

// cfgAoTecla fecha no Escape: retorno 0 = tratado.
func cfgAoTecla(aplicativo *app, wparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	if uint32(wparam) == vkEscape {
		aplicativo.fecharCfg()

		return 0
	}

	return 0
}

// cfgAoComando trata as trocas no filtro e no idioma: retorno 0 = tratado.
func cfgAoComando(aplicativo *app, hwnd uintptr, wparam, lparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	aviso := uint32((wparam >> deslocamentoWordAlta) & mascaraWordBaixa)
	ctl := lparam

	switch aviso {
	case enChange:
		if ctl == cfgUI.editFiltro {
			cfgUI.filtro = cfgTextoControle(cfgUI.editFiltro)

			aplicativo.cfgLayout()
			pInvalidateRect.Call(hwnd, 0, 0)

			return 0
		}
	case cbnSelchange:
		if ctl == cfgUI.comboIdioma {
			return 0
		}
	}

	return 0
}

// cfgAoTimer atualiza downloads e VPN a cada tick: retorno 0 = tratado.
func cfgAoTimer(aplicativo *app, hwnd uintptr, wparam uintptr) uintptr { //nolint:unparam
	if aplicativo == nil || !cfgUI.ativo {
		return 0
	}

	if uint32(wparam) == cfgTimer && (cfgUI.secao == cfgSecVPN || cfgUI.secao == cfgSecDown) {
		if cfgUI.secao == cfgSecDown {
			// cfgLayout reconstrói a lista: só refaz quando algo mudou.
			// O repaint do progresso continua a cada tick.
			if ass := aplicativo.assDownloads(); ass != cfgUI.downAss {
				cfgUI.downAss = ass

				aplicativo.cfgLayout()
			}
		}

		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

// cfgAoRedimensionar ajusta o layout ao tamanho novo.
func cfgAoRedimensionar(aplicativo *app, hwnd uintptr, lparam uintptr) uintptr {
	if aplicativo != nil && cfgUI.ativo {
		cfgUI.w = loword(lparam)
		cfgUI.h = hiword(lparam)

		aplicativo.cfgLayout()
		d2dCfg.redimensionar(cfgUI.w, cfgUI.h)
		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

// cfgProcJanela trata as mensagens de ciclo de vida da janela.
func cfgProcJanela(aplicativo *app, hwnd uintptr, msg uint32, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmPaint:
		return cfgAoPintar(aplicativo, hwnd), true
	case wmEraseBkgnd:
		return 1, true
	case wmClose:
		if aplicativo != nil {
			aplicativo.fecharCfg()
		}

		return 0, true
	case wmSize:
		return cfgAoRedimensionar(aplicativo, hwnd, lparam), true
	}

	return 0, false
}

// cfgProcEntrada trata mouse, teclado, comando e timer.
func cfgProcEntrada(aplicativo *app, hwnd uintptr, msg uint32, wparam, lparam uintptr) (uintptr, bool) {
	switch msg {
	case wmMouseMove:
		return cfgAoMouseMover(aplicativo, hwnd, lparam), true
	case wmLButtonDown:
		return cfgAoClique(aplicativo, hwnd, lparam), true
	case wmLButtonUp:
		return cfgAoSoltarCfg(aplicativo, hwnd), true
	case wmMouseWheel:
		return cfgAoRolar(aplicativo, hwnd, wparam), true
	case wmKeyDown:
		return cfgAoTecla(aplicativo, wparam), true
	case wmCommand:
		return cfgAoComando(aplicativo, hwnd, wparam, lparam), true
	case wmTimer:
		return cfgAoTimer(aplicativo, hwnd, wparam), true
	}

	return 0, false
}

func cfgProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	if r, ok := cfgProcJanela(aplicativo, hwnd, msg, lparam); ok {
		return r
	}

	if r, ok := cfgProcEntrada(aplicativo, hwnd, msg, wparam, lparam); ok {
		return r
	}

	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)

	return r
}
