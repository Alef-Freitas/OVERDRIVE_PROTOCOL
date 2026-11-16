//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/graphics/direct2d"
)

const (
	wmClipboardUpdate = 0x031D
	wmMouseWheel      = 0x020A
	clipClasse        = "OverdriveClipWnd"

	clipMaxItens       = 30
	clipMaxImagens     = 4
	clipMaxTextoItem   = 1 << 20
	clipMaxBytesImagem = 24 << 20
	clipMaxBytesTexto  = 4 << 20

	cfHDrop = 15
)

//nolint:gochecknoglobals // Procs clipboard + ponto de entrada: handles fixos do processo.
var (
	pAddClipboardFormatListener = user32.NewProc("AddClipboardFormatListener")
	pRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	pIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	pDragQueryFileW             = shell32.NewProc("DragQueryFileW")
	clipProcPtr                 uintptr
	clipFmtIgnorar, clipFmtExcl uintptr
	clipFmtIncluir              uintptr
)

type clipTipo uint8

const (
	clipTexto clipTipo = iota
	clipArquivos
	clipImagem
)

//exhaustruct:ignore // Item de clipboard: construtor parcial por desenho.
type clipItem struct {
	tipo   clipTipo
	texto  string
	dib    []byte
	w, h   int32
	fixo   bool
	quando time.Time
	chave  uint64
}

//nolint:gochecknoglobals // Historico de clipboard: estado global do app single-window.
var clipHist []clipItem

type clipLinha struct {
	idx   int
	r     rectT
	check rectT
	pin   rectT
	del   rectT
}

type clipPainel struct {
	hwnd      uintptr
	ativo     bool
	w, h      int32
	headerH   int32
	footerH   int32
	sel       int
	hover     int
	hoverLim  bool
	hoverEnv  bool
	confirmar bool
	scroll    int32
	total     int32
	limpar    rectT
	contar    rectT
	enviar    rectT
	marcados  map[uint64]bool
	rows      []clipLinha

	saiuFoco bool
	fechouEm time.Time
}

//nolint:gochecknoglobals // Painel de clipboard: estado de UI single-window.
var clipUI clipPainel

func registraFormatoClipboard(nome string) uintptr {
	r, _, _ := pRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(utf16(nome))))

	return r
}

func clipboardIniciar(hwnd uintptr) {
	clipFmtIgnorar = registraFormatoClipboard("Clipboard Viewer Ignore")
	clipFmtExcl = registraFormatoClipboard("ExcludeClipboardContentFromMonitorProcessing")
	clipFmtIncluir = registraFormatoClipboard("CanIncludeInClipboardHistory")

	pAddClipboardFormatListener.Call(hwnd)
}

func clipHash(item clipItem) uint64 {
	resumo := fnv.New64a()
	resumo.Write([]byte{byte(item.tipo)})

	if item.tipo == clipImagem {
		resumo.Write(item.dib)
	} else {
		resumo.Write([]byte(item.texto))
	}

	return resumo.Sum64()
}

func clipAdicionar(item clipItem) {
	if item.tipo == clipImagem && len(item.dib) > clipMaxBytesImagem {
		return
	}

	if item.tipo != clipImagem && len(item.texto) > clipMaxTextoItem {
		return
	}

	item.chave = clipHash(item)
	for i := range clipHist {
		if clipHist[i].chave == item.chave {
			item.fixo = clipHist[i].fixo
			clipHist = append(clipHist[:i], clipHist[i+1:]...)

			break
		}
	}

	item.quando = time.Now()
	clipHist = append([]clipItem{item}, clipHist...)

	clipPodar()
	clipOrdenar()

	if clipUI.ativo {
		clipUI.scroll = 0
		clipUI.hover = -1

		clipLayout()

		clipUI.sel = 0
		pInvalidateRect.Call(clipUI.hwnd, 0, 0)
	}

	if theApp != nil {
		theApp.invalidateBar()
	}
}

func clipSomarUso() (int, int, int) {
	var imgs, bytesImg, bytesTxt int

	for _, it := range clipHist {
		if it.tipo == clipImagem {
			imgs++
			bytesImg += len(it.dib)
		} else {
			bytesTxt += len(it.texto)
		}
	}

	return imgs, bytesImg, bytesTxt
}

func clipPrecisaPodar(imgs, bytesImg, bytesTxt int) bool {
	return len(clipHist) > clipMaxItens || imgs > clipMaxImagens ||
		bytesImg > clipMaxBytesImagem || bytesTxt > clipMaxBytesTexto
}

func clipTentarRemover(indice int, item clipItem, imgs, bytesImg, bytesTxt *int) bool {
	if item.fixo {
		return false
	}

	if item.tipo == clipImagem {
		*imgs--
		*bytesImg -= len(item.dib)
	} else {
		*bytesTxt -= len(item.texto)
	}

	clipHist = append(clipHist[:indice], clipHist[indice+1:]...)

	return item.tipo == clipImagem
}

func clipPodarExcesso(imgs, bytesImg, bytesTxt int) int {
	liberouImagem := 0

	for idx, item := range slices.Backward(clipHist) {
		if !clipPrecisaPodar(imgs, bytesImg, bytesTxt) {
			break
		}

		antes := len(item.dib)
		if !clipTentarRemover(idx, item, &imgs, &bytesImg, &bytesTxt) {
			continue
		}

		liberouImagem += antes
	}

	return liberouImagem
}

func clipApararMarcados() {
	if len(clipUI.marcados) == 0 {
		return
	}

	vivas := map[uint64]bool{}

	for _, it := range clipHist {
		if clipUI.marcados[it.chave] {
			vivas[it.chave] = true
		}
	}

	clipUI.marcados = vivas
}

func clipPodar() {
	imgs, bytesImg, bytesTxt := clipSomarUso()
	liberouImagem := clipPodarExcesso(imgs, bytesImg, bytesTxt)

	if liberouImagem >= 4<<20 {
		debug.FreeOSMemory()
	}

	clipApararMarcados()
}

func clipOrdenar() {
	sort.SliceStable(clipHist, func(i, j int) bool {
		return clipHist[i].fixo && !clipHist[j].fixo
	})
}

func clipCabecalhoDIB(dib []byte) (*bitmapInfoHeader, bool) {
	const tamanhoMinimoCabecalhoDIB = 40
	if len(dib) < tamanhoMinimoCabecalhoDIB {
		return nil, false
	}

	hdr := (*bitmapInfoHeader)(unsafe.Pointer(&dib[0]))
	if hdr.biSize < tamanhoMinimoCabecalhoDIB || int(hdr.biSize) > len(dib) {
		return nil, false
	}

	return hdr, true
}

func clipBppValidoDIB(bpp int) bool {
	const (
		bpp4BitsClip  = 4
		bpp8BitsClip  = 8
		bpp16BitsClip = 16
		bpp24BitsClip = 24
		bpp32BitsClip = 32
	)
	switch bpp {
	case 1, bpp4BitsClip, bpp8BitsClip, bpp16BitsClip, bpp24BitsClip, bpp32BitsClip:
		return true
	default:
		return false
	}
}

func clipCompressaoValidaDIB(comp uint32) bool {
	return comp == 0 || comp == 3
}

func clipDeslocamentoDIB(hdr *bitmapInfoHeader) int {
	const (
		tamanhoMinimoCabecalhoDIB = 40
		limitePaletaDIBBits       = 8
		bytesPorCorPaletaDIB      = 4
	)

	off := int(hdr.biSize)
	if hdr.biSize == tamanhoMinimoCabecalhoDIB && hdr.biCompression == 3 {
		off += 12
	}

	if int(hdr.biBitCount) <= limitePaletaDIBBits {
		cores := int(hdr.biClrUsed)
		if cores == 0 {
			cores = 1 << uint(hdr.biBitCount)
		}

		off += cores * bytesPorCorPaletaDIB
	}

	return off
}

func clipNormalizarDimensaoDIB(v int32) int32 {
	if v < 0 {
		return -v
	}

	return v
}

func clipDIBInfo(dib []byte) (int32, int32, int, bool) {
	hdr, ok := clipCabecalhoDIB(dib)
	if !ok {
		return 0, 0, 0, false
	}

	if !clipBppValidoDIB(int(hdr.biBitCount)) {
		return 0, 0, 0, false
	}

	if !clipCompressaoValidaDIB(hdr.biCompression) {
		return 0, 0, 0, false
	}

	off := clipDeslocamentoDIB(hdr)
	if off >= len(dib) {
		return 0, 0, 0, false
	}

	largura := clipNormalizarDimensaoDIB(hdr.biWidth)
	altura := clipNormalizarDimensaoDIB(hdr.biHeight)

	if largura <= 0 || altura <= 0 {
		return 0, 0, 0, false
	}

	return largura, altura, off, true
}

func clipBloqueadoSistema() bool {
	if clipFmtIgnorar != 0 {
		if r, _, _ := pIsClipboardFormatAvailable.Call(clipFmtIgnorar); r != 0 {
			return true
		}
	}

	if clipFmtExcl != 0 {
		if r, _, _ := pIsClipboardFormatAvailable.Call(clipFmtExcl); r != 0 {
			return true
		}
	}

	return clipIncluirVetado()
}

func clipIncluirVetado() bool {
	if clipFmtIncluir == 0 {
		return false
	}

	h, _, _ := pGetClipboardData.Call(clipFmtIncluir)
	if h == 0 {
		return false
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return false
	}

	v := *(*uint32)(unsafe.Pointer(ptr))

	pGlobalUnlock.Call(h)

	return v == 0
}

func clipLerCaminhosSistema(h uintptr) []string {
	const limiteArquivosClip = 500

	numArquivos, _, _ := pDragQueryFileW.Call(h, ^uintptr(0), 0, 0)
	if numArquivos == 0 || numArquivos >= limiteArquivosClip {
		return nil
	}

	caminhos := make([]string, 0, numArquivos)
	for idx := range numArquivos {
		tam, _, _ := pDragQueryFileW.Call(h, idx, 0, 0)
		if tam == 0 || tam > 4096 {
			continue
		}

		buf := make([]uint16, tam+1)
		pDragQueryFileW.Call(h, idx, uintptr(unsafe.Pointer(&buf[0])), tam+1)
		caminhos = append(caminhos, windows.UTF16ToString(buf))
	}

	return caminhos
}

func clipLerArquivosSistema() (clipItem, bool) {
	h, _, _ := pGetClipboardData.Call(cfHDrop)
	if h == 0 {
		return clipItem{}, false
	}

	if caminhos := clipLerCaminhosSistema(h); len(caminhos) > 0 {
		return clipItem{tipo: clipArquivos, texto: strings.Join(caminhos, "\n")}, true
	}

	return clipItem{}, false
}

func clipLerTextoSistema() (clipItem, bool) {
	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return clipItem{}, false
	}

	tam, _, _ := pGlobalSize.Call(h)
	if tam < 2 || tam > clipMaxTextoItem {
		return clipItem{}, false
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return clipItem{}, false
	}
	defer pGlobalUnlock.Call(h)

	const bytesPorUnidadeUTF16Clip = 2

	unidades := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), int(tam/bytesPorUnidadeUTF16Clip))
	txt := windows.UTF16ToString(unidades)

	pGlobalUnlock.Call(h)

	if strings.TrimSpace(txt) == "" {
		return clipItem{}, false
	}

	return clipItem{tipo: clipTexto, texto: txt}, true
}

func clipLerImagemSistema() (clipItem, bool) {
	h, _, _ := pGetClipboardData.Call(cfDib)
	if h == 0 {
		return clipItem{}, false
	}

	tam, _, _ := pGlobalSize.Call(h)
	if tam < 40 || tam > clipMaxBytesImagem {
		return clipItem{}, false
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return clipItem{}, false
	}

	dib := make([]byte, int(tam))
	copy(dib, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(tam)))
	pGlobalUnlock.Call(h)

	w, hh, _, ok := clipDIBInfo(dib)
	if !ok {
		return clipItem{}, false
	}

	return clipItem{tipo: clipImagem, dib: dib, w: w, h: hh}, true
}

func clipLerSistema() (clipItem, bool) {
	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return clipItem{}, false
	}
	defer pCloseClipboard.Call()

	if clipBloqueadoSistema() {
		return clipItem{}, false
	}

	if it, ok := clipLerArquivosSistema(); ok {
		return it, true
	}

	if it, ok := clipLerTextoSistema(); ok {
		return it, true
	}

	if it, ok := clipLerImagemSistema(); ok {
		return it, true
	}

	return clipItem{}, false
}

func (a *app) clipboardAtualizar() {
	it, ok := clipLerSistema()
	if !ok {
		return
	}

	clipAdicionar(it)
}

func clipCriarJanela() bool {
	if clipProcPtr == 0 {
		return false
	}

	if clipUI.hwnd != 0 {
		return true
	}

	hInst := getModuleHandle()

	const tamanhoInicialClip = 100

	nomeClasse := utf16(clipClasse)
	classe := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   clipProcPtr,
		hInstance:     hInst,
		hCursor:       loadCursor(idcArrow),
		hbrBackground: 0,
		lpszClassName: nomeClasse,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classe)))
	h, _, _ := pCreateWindowExW.Call(
		uintptr(wsExTopmost|wsExToolWindow),
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(utf16(txtTituloClip))),
		wsPopup,
		0, 0, tamanhoInicialClip, tamanhoInicialClip, 0, 0, hInst, 0,
	)
	clipUI.hwnd = h
	aplicarCantosDWM(h)

	return h != 0
}

func (a *app) clipFecharSeAberto() bool {
	if !clipUI.ativo {
		return false
	}

	clipUI.saiuFoco = false

	a.fecharClipboard()

	return true
}

func clipIgnorarReabertura() bool {
	if !clipUI.saiuFoco || time.Since(clipUI.fechouEm) >= 500*time.Millisecond {
		return false
	}

	clipUI.saiuFoco = false

	return true
}

func (a *app) clipDimensionarPainel() {
	clipUI.w = a.sc(470)
	clipUI.h = a.sc(580)

	wa := areaTrabalhoJanela(a.hwnd)
	if wa.Height() > 0 && clipUI.h > wa.Height()-a.sc(80) {
		clipUI.h = wa.Height() - a.sc(80)
	}

	clipUI.headerH = a.sc(52)
	clipUI.footerH = a.sc(36)
	clipUI.scroll = 0
	clipUI.hover = -1
	clipUI.hoverLim = false
	clipUI.hoverEnv = false
	clipUI.confirmar = false

	clipUI.sel = 0
	if len(clipHist) == 0 {
		clipUI.sel = -1
	}

	clipLayout()
}

func (a *app) clipEnquadrarPainel(x, y int32) (int32, int32) {
	area := areaTrabalhoPonto(x, y)

	if area.Right > area.Left {
		if x < area.Left+a.sc(8) {
			x = area.Left + a.sc(8)
		}

		if x+clipUI.w > area.Right-a.sc(8) {
			x = area.Right - a.sc(8) - clipUI.w
		}
	}

	if area.Bottom > area.Top && y+clipUI.h > area.Bottom-a.sc(8) {
		y = area.Bottom - a.sc(8) - clipUI.h
		if y < area.Top+a.sc(8) {
			y = area.Top + a.sc(8)
		}
	}

	return x, y
}

func (a *app) clipPosicionarPainel() {
	cr := a.clientRect()
	pt := struct{ x, y int32 }{cr.Right, a.tabH + a.tbHeight}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&pt)))
	x := pt.x - clipUI.w - a.sc(8)
	y := pt.y + a.sc(6)

	x, y = a.clipEnquadrarPainel(x, y)

	clipUI.ativo = true
	pSetWindowPos.Call(clipUI.hwnd, hwndTopmost, uintptr(uint32(x)), uintptr(uint32(y)),
		uintptr(clipUI.w), uintptr(clipUI.h), swpShowWindow|swpNoActivate)
	pShowWindow.Call(clipUI.hwnd, swShow)
	pSetForegroundWindow.Call(clipUI.hwnd)
	pSetFocus.Call(clipUI.hwnd)
	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) alternarClipboard() {
	if a.clipFecharSeAberto() {
		return
	}

	if clipIgnorarReabertura() {
		return
	}

	clipUI.saiuFoco = false

	if !clipCriarJanela() {
		a.dizerf(txtClipNaoAbriu)

		return
	}

	a.clipDimensionarPainel()
	a.clipPosicionarPainel()
}

func (a *app) fecharClipboard() {
	if !clipUI.ativo {
		return
	}

	clipUI.ativo = false
	clipUI.fechouEm = time.Now()
	clipUI.hover = -1
	clipUI.hoverEnv = false
	clipUI.confirmar = false

	d2dClip.soltar()
	d2dClipImgsMu.Lock()
	for _, bm := range d2dClipImgs {
		if bm != nil {
			bm.Release()
		}
	}

	d2dClipImgs = map[uint64]*direct2d.ID2D1Bitmap{}
	d2dClipImgsMu.Unlock()
	pShowWindow.Call(clipUI.hwnd, swHide)

	if a.hwnd != 0 {
		pSetForegroundWindow.Call(a.hwnd)
		pInvalidateRect.Call(a.hwnd, 0, 0)
	}
}

func clipAlturaItem(item clipItem) int32 {
	const alturaItemTextoClip = 66

	aplicativo := theApp
	if aplicativo == nil {
		return alturaItemTextoClip
	}

	if item.tipo == clipImagem {
		return aplicativo.sc(96)
	}

	return aplicativo.sc(alturaItemTextoClip)
}

// clipEnquadrarRolagem limita a rolagem ao conteudo visivel.
func clipEnquadrarRolagem(rolagem, total, visivel int32) int32 {
	maximo := max(total-visivel, 0)
	if rolagem > maximo {
		return maximo
	}

	if rolagem < 0 {
		return 0
	}

	return rolagem
}

func clipLayout() {
	aplicativo := theApp
	if aplicativo == nil {
		return
	}

	pad := aplicativo.sc(14)
	top := clipUI.headerH + aplicativo.sc(8)
	bottom := clipUI.h - clipUI.footerH
	clipUI.limpar = rectT{
		clipUI.w - aplicativo.sc(100), aplicativo.sc(12), clipUI.w - aplicativo.sc(14), aplicativo.sc(42),
	}
	clipUI.contar = rectT{
		clipUI.w - aplicativo.sc(206), aplicativo.sc(15), clipUI.w - aplicativo.sc(140), aplicativo.sc(39),
	}
	clipUI.enviar = rectT{
		clipUI.w - aplicativo.sc(134), aplicativo.sc(12), clipUI.w - aplicativo.sc(106), aplicativo.sc(42),
	}

	var total int32
	for _, item := range clipHist {
		total += clipAlturaItem(item) + aplicativo.sc(6)
	}

	visivel := bottom - top

	clipUI.scroll = clipEnquadrarRolagem(clipUI.scroll, total, visivel)

	clipUI.total = total

	clipUI.rows = clipUI.rows[:0]
	y := top - clipUI.scroll

	for idx, item := range clipHist {
		alt := clipAlturaItem(item)

		area := rectT{pad, y, clipUI.w - pad, y + alt}
		if y+alt > top && y < bottom {
			pin := rectT{
				area.Right - aplicativo.sc(62), area.Top + (alt-aplicativo.sc(24))/2,
				area.Right - aplicativo.sc(36), area.Top + (alt+aplicativo.sc(24))/2,
			}
			del := rectT{area.Right - aplicativo.sc(34), pin.Top, area.Right - aplicativo.sc(8), pin.Bottom}
			check := rectT{area.Right - aplicativo.sc(90), pin.Top, area.Right - aplicativo.sc(64), pin.Bottom}
			clipUI.rows = append(clipUI.rows, clipLinha{idx: idx, r: area, check: check, pin: pin, del: del})
		}

		y += alt + aplicativo.sc(6)
	}

	if clipUI.sel >= len(clipHist) {
		clipUI.sel = len(clipHist) - 1
	}
}

func clipLinhaEm(y int32) int {
	for i := range clipUI.rows {
		if y >= clipUI.rows[i].r.Top && y < clipUI.rows[i].r.Bottom {
			return i
		}
	}

	return -1
}

func clipboardGravarTexto(texto string) error {
	if texto == "" {
		return nil
	}

	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return erroTexto("área de transferência ocupada")
	}
	defer pCloseClipboard.Call()

	pEmptyClipboard.Call()

	const bytesPorUnidadeUTF16Clip = 2

	unidades := windows.StringToUTF16(texto)
	num := len(unidades) * bytesPorUnidadeUTF16Clip

	h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(num))
	if h == 0 {
		return erroTexto("sem memória")
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		pGlobalFree.Call(h)

		return erroTexto("falha ao copiar")
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), num), unsafe.Slice((*byte)(unsafe.Pointer(&unidades[0])), num))
	pGlobalUnlock.Call(h)

	if r, _, _ := pSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		pGlobalFree.Call(h)

		return erroTexto("falha ao copiar")
	}

	return nil
}

//exhaustruct:ignore // Espelho da API Win32 (DROPFILES): campos nao usados ficam zero.
type dropFilesW struct {
	pFiles uint32
	pt     struct{ x, y int32 }
	fNC    int32
	fWide  int32
}

// nomesArquivosClip converte as linhas em UTF-16 para o DROPFILES.
func nomesArquivosClip(texto string) ([]uint16, error) {
	partes := strings.Split(texto, "\n")
	if len(partes) == 0 {
		return nil, nil
	}

	var nomes []uint16

	for _, p := range partes {
		if p == "" {
			continue
		}

		u, err := windows.UTF16FromString(p)
		if err == nil {
			nomes = append(nomes, u...)
		}
	}

	if len(nomes) == 0 {
		return nil, erroTexto("sem arquivos")
	}

	return nomes, nil
}

func clipboardGravarArquivos(texto string) error {
	nomes, err := nomesArquivosClip(texto)
	if err != nil {
		return err
	}

	nomes = append(nomes, 0)
	df := dropFilesW{pFiles: uint32(unsafe.Sizeof(dropFilesW{})), fWide: 1}
	hdr := unsafe.Slice((*byte)(unsafe.Pointer(&df)), int(unsafe.Sizeof(df)))

	const bytesPorUnidadeUTF16Clip = 2

	total := len(hdr) + len(nomes)*bytesPorUnidadeUTF16Clip

	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return erroTexto("área de transferência ocupada")
	}

	defer pCloseClipboard.Call()

	pEmptyClipboard.Call()

	h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(total))
	if h == 0 {
		return erroTexto("sem memória")
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		pGlobalFree.Call(h)

		return erroTexto("falha ao copiar")
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), total)
	copy(dst, hdr)
	copy(dst[len(hdr):], unsafe.Slice((*byte)(unsafe.Pointer(&nomes[0])), len(nomes)*bytesPorUnidadeUTF16Clip))
	pGlobalUnlock.Call(h)

	if r, _, _ := pSetClipboardData.Call(cfHDrop, h); r == 0 {
		pGlobalFree.Call(h)

		return erroTexto("falha ao copiar")
	}

	return nil
}

func (a *app) clipCopiarItem(idx int) {
	if idx < 0 || idx >= len(clipHist) {
		return
	}

	item := clipHist[idx]

	var err error

	switch item.tipo {
	case clipTexto:
		err = clipboardGravarTexto(item.texto)
	case clipArquivos:
		err = clipboardGravarArquivos(item.texto)
	case clipImagem:
		err = clipboardGravarDIB(item.dib)
	}

	if err != nil {
		a.dizerf("%s", txtClipPrefix+err.Error())

		return
	}

	a.fecharClipboard()

	switch item.tipo {
	case clipTexto:
		a.dizerf(txtClipTxtCopiado)
	case clipArquivos:
		a.dizerf(txtClipArqCopiado)
	case clipImagem:
		a.dizerf(txtClipImgCopiada)
	}
}

func (a *app) clipRemover(idx int) {
	if idx < 0 || idx >= len(clipHist) {
		return
	}

	delete(clipUI.marcados, clipHist[idx].chave)
	clipHist = append(clipHist[:idx], clipHist[idx+1:]...)

	clipUI.hover = -1
	if clipUI.sel >= len(clipHist) {
		clipUI.sel = len(clipHist) - 1
	}

	clipLayout()
	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
	a.invalidateBar()
}

func (a *app) clipAlternarFixo(idx int) {
	if idx < 0 || idx >= len(clipHist) {
		return
	}

	clipHist[idx].fixo = !clipHist[idx].fixo

	clipOrdenar()

	clipUI.hover = -1

	clipLayout()
	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
}

func clipTextoParaIA(it clipItem) string {
	if it.tipo == clipArquivos {
		return "Arquivos:\n" + strings.TrimSpace(it.texto)
	}

	return it.texto
}

func (a *app) clipAlternarMarca(idx int) {
	if idx < 0 || idx >= len(clipHist) {
		return
	}

	if clipUI.marcados == nil {
		clipUI.marcados = map[uint64]bool{}
	}

	chave := clipHist[idx].chave
	if clipUI.marcados[chave] {
		delete(clipUI.marcados, chave)
	} else {
		clipUI.marcados[chave] = true
	}

	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
}

func (a *app) clipAlternarTudo() {
	if len(clipHist) == 0 {
		return
	}
	// Tudo marcado = toda chave do historico presente, nao so contagem
	// igual (chaves antigas podiam distorcer a comparacao).
	tudo := len(clipUI.marcados) > 0
	if tudo {
		for _, it := range clipHist {
			if !clipUI.marcados[it.chave] {
				tudo = false

				break
			}
		}
	}

	if tudo {
		clipUI.marcados = map[uint64]bool{}
	} else {
		m := make(map[uint64]bool, len(clipHist))
		for _, it := range clipHist {
			m[it.chave] = true
		}

		clipUI.marcados = m
	}

	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
}

func (a *app) clipEnviarMarcados() {
	var idxs []int

	for i, it := range clipHist {
		if clipUI.marcados[it.chave] {
			idxs = append(idxs, i)
		}
	}

	if len(idxs) == 0 {
		for i := range clipHist {
			idxs = append(idxs, i)
		}
	}

	a.clipEnviarLista(idxs)
}

func clipColetarImagemEnvio(item clipItem, indice int, imgs *[]imagemAnexo, ultimoErro *string) {
	dados, err := dibParaDataURL(item.dib)
	if err != nil {
		*ultimoErro = err.Error()

		return
	}

	*imgs = append(*imgs, imagemAnexo{Nome: fmt.Sprintf("imagem-%d.png", indice+1), Tipo: "image/png", Dados: dados})
}

func clipColetarEnvio(idxs []int) ([]string, []imagemAnexo, string) {
	var (
		textos     []string
		imgs       []imagemAnexo
		ultimoErro string
	)

	for _, idx := range idxs {
		if idx < 0 || idx >= len(clipHist) {
			continue
		}

		item := clipHist[idx]
		if item.tipo == clipImagem {
			clipColetarImagemEnvio(item, idx, &imgs, &ultimoErro)

			continue
		}

		if t := strings.TrimSpace(clipTextoParaIA(item)); t != "" {
			textos = append(textos, t)
		}
	}

	return textos, imgs, ultimoErro
}

func (a *app) clipPublicarEnvio(textos []string, imgs []imagemAnexo, ultimoErro string) {
	if len(textos) == 0 && len(imgs) == 0 {
		if ultimoErro != "" {
			a.dizerf("%s", txtClipPrefix+ultimoErro)
		} else {
			a.dizerf(txtNadaMandar)
		}

		a.invalidateBar()

		return
	}

	a.fecharClipboard()

	if len(imgs) > 0 {
		a.anexarImagensNoChat(imgs)
	}

	if len(textos) == 0 {
		return
	}

	corpo := strings.Join(textos, "\n\n---\n\n")
	n := utf8.RuneCountInString(corpo)
	a.enviarPaginaNoChat(corpo, n, n, false)
}

func (a *app) clipEnviarLista(idxs []int) {
	textos, imgs, ultimoErro := clipColetarEnvio(idxs)
	a.clipPublicarEnvio(textos, imgs, ultimoErro)
}

func dibParaDataURL(dib []byte) (string, error) {
	png, err := dibParaPNG(dib)
	if err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func dibCabecalhoPNG(dib []byte) (*bitmapInfoHeader, error) {
	const tamanhoMinimoCabecalhoDIB = 40
	if len(dib) < tamanhoMinimoCabecalhoDIB {
		return nil, erroTexto("cabeçalho curto")
	}

	hdr := (*bitmapInfoHeader)(unsafe.Pointer(&dib[0]))
	if hdr.biSize < 40 || int(hdr.biSize) > len(dib) {
		return nil, erroTexto("cabeçalho inválido")
	}

	if hdr.biCompression != 0 {
		return nil, erroTexto("DIB comprimido")
	}

	bpp := int(hdr.biBitCount)
	if bpp != 24 && bpp != 32 {
		return nil, erroTextoF("sem suporte a %dbpp", bpp)
	}

	return hdr, nil
}

func dibGeometriaPNG(hdr *bitmapInfoHeader) (int, int, bool, error) {
	var (
		larg, alt int
		deCima    bool
	)

	larg, alt = int(hdr.biWidth), int(hdr.biHeight)
	if alt < 0 {
		alt, deCima = -alt, true
	}

	if larg <= 0 || alt <= 0 || larg > 8192 || alt > 8192 {
		return 0, 0, false, erroTexto("dimensões inválidas")
	}

	return larg, alt, deCima, nil
}

func dibPassoPNG(larg, bpp int) int {
	const (
		alinhamentoPassoDIB          = 4
		ajusteArredondamentoPassoDIB = 3
	)

	return ((larg*bpp/8 + ajusteArredondamentoPassoDIB) / alinhamentoPassoDIB) * alinhamentoPassoDIB
}

func dibConverterPixels(img *image.RGBA, dib []byte, off, largura, altura, passo, porPx int, deCima bool) {
	for linha := range altura {
		sy := linha
		if !deCima {
			sy = altura - 1 - linha
		}

		fatia := dib[off+sy*passo:]

		for x := range largura {
			const alfaOpacoClip = 255

			px := fatia[x*porPx:]
			img.SetRGBA(x, linha, color.RGBA{R: px[2], G: px[1], B: px[0], A: alfaOpacoClip})
		}
	}
}

func dibParaPNG(dib []byte) ([]byte, error) {
	hdr, err := dibCabecalhoPNG(dib)
	if err != nil {
		return nil, err
	}

	largura, altura, deCima, err := dibGeometriaPNG(hdr)
	if err != nil {
		return nil, err
	}

	bpp := int(hdr.biBitCount)
	passo := dibPassoPNG(largura, bpp)

	off := int(hdr.biSize)
	if off+passo*altura > len(dib) {
		return nil, erroTexto("pixels cortados")
	}

	img := image.NewRGBA(image.Rect(0, 0, largura, altura))

	const bitsPorByteClip = 8
	dibConverterPixels(img, dib, off, largura, altura, passo, bpp/bitsPorByteClip, deCima)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err //nolint:wrapcheck // Codificacao em memoria: erro autoexplicativo, sem contexto a agregar.
	}

	return buf.Bytes(), nil
}

func (a *app) clipLimparTudo() {
	tinhaImagem := false

	for _, it := range clipHist {
		if it.tipo == clipImagem {
			tinhaImagem = true

			break
		}
	}

	clipHist = nil
	clipUI.marcados = map[uint64]bool{}
	clipUI.sel = -1
	clipUI.scroll = 0
	clipUI.confirmar = false

	clipLayout()
	pInvalidateRect.Call(clipUI.hwnd, 0, 0)
	a.invalidateBar()

	if tinhaImagem {
		debug.FreeOSMemory()
	}
}

func (a *app) clipLimparConfirmado() {
	if len(clipUI.marcados) > 0 {
		var sobra []clipItem

		for _, it := range clipHist {
			if !clipUI.marcados[it.chave] {
				sobra = append(sobra, it)
			}
		}

		clipHist = sobra

		clipUI.marcados = map[uint64]bool{}
		if clipUI.sel >= len(clipHist) {
			clipUI.sel = len(clipHist) - 1
		}

		clipUI.confirmar = false

		clipLayout()
		pInvalidateRect.Call(clipUI.hwnd, 0, 0)
		a.invalidateBar()

		return
	}

	a.clipLimparTudo()
}

func clipAoPintar(hwnd uintptr, aplicativo *app) uintptr {
	if aplicativo != nil && clipUI.ativo {
		if d2dClip.rt == nil {
			d2dClip.criar(hwnd, clipUI.w, clipUI.h)
		}

		if d2dClip.rt != nil {
			d2dClip.iniciar()
			aplicativo.paintClipboardD2D()
			d2dClip.apresentar()
		} else {
			pValidateRect.Call(hwnd, 0)
		}

		return 0
	}

	pValidateRect.Call(hwnd, 0)

	return 0
}

func clipDentroBotao(x, y int32, r rectT) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom && len(clipHist) > 0
}

func clipAoMoverMouse(hwnd uintptr, a *app, lparam uintptr) uintptr {
	if a == nil || !clipUI.ativo {
		return 0
	}

	x, y := loword(lparam), hiword(lparam)
	hoverLim := clipDentroBotao(x, y, clipUI.limpar)
	hoverEnv := clipDentroBotao(x, y, clipUI.enviar)
	linha := clipLinhaEm(y)
	mudou := hoverLim != clipUI.hoverLim || hoverEnv != clipUI.hoverEnv || linha != clipUI.hover
	clipUI.hoverLim = hoverLim
	clipUI.hoverEnv = hoverEnv
	clipUI.hover = linha

	if mudou {
		pInvalidateRect.Call(hwnd, 0, 0)
	}

	return 0
}

func clipClicarBotoes(aplicativo *app, hwnd uintptr, x, y int32) bool {
	if clipDentroBotao(x, y, clipUI.enviar) {
		aplicativo.clipEnviarMarcados()

		return true
	}

	if clipDentroBotao(x, y, clipUI.contar) {
		aplicativo.clipAlternarTudo()

		return true
	}

	if clipDentroBotao(x, y, clipUI.limpar) {
		if clipUI.confirmar {
			aplicativo.clipLimparConfirmado()
		} else {
			clipUI.confirmar = true

			pInvalidateRect.Call(hwnd, 0, 0)
		}

		return true
	}

	return false
}

func clipClicarLinha(aplicativo *app, x, y int32) {
	linha := clipLinhaEm(y)
	if linha < 0 {
		return
	}

	selecionada := clipUI.rows[linha]

	switch {
	case x >= selecionada.check.Left && x < selecionada.check.Right:
		aplicativo.clipAlternarMarca(selecionada.idx)
	case x >= selecionada.pin.Left && x < selecionada.pin.Right:
		aplicativo.clipAlternarFixo(selecionada.idx)
	case x >= selecionada.del.Left && x < selecionada.del.Right:
		aplicativo.clipRemover(selecionada.idx)
	default:
		clipUI.sel = selecionada.idx
		aplicativo.clipCopiarItem(selecionada.idx)
	}
}

func clipAoClicar(hwnd uintptr, aplicativo *app, lparam uintptr) uintptr {
	if aplicativo == nil || !clipUI.ativo {
		return 0
	}

	x, y := loword(lparam), hiword(lparam)
	if clipClicarBotoes(aplicativo, hwnd, x, y) {
		return 0
	}

	clipUI.confirmar = false

	linha := clipLinhaEm(y)
	if linha < 0 {
		pInvalidateRect.Call(hwnd, 0, 0)

		return 0
	}

	clipClicarLinha(aplicativo, x, y)

	return 0
}

func clipAoRolar(hwnd uintptr, aplicativo *app, wparam uintptr) uintptr {
	if aplicativo == nil || !clipUI.ativo {
		return 0
	}

	const deslocamentoDeltaRodaClip = 16

	delta := int32(int16(uint16(wparam >> deslocamentoDeltaRodaClip)))

	const divisorDeltaRodaClip = 120

	clipUI.scroll -= delta / divisorDeltaRodaClip * aplicativo.sc(56)

	clipLayout()
	pInvalidateRect.Call(hwnd, 0, 0)

	return 0
}

func clipTeclarEnter(aplicativo *app) {
	if getKeyDown(vkControl) {
		aplicativo.clipEnviarMarcados()

		return
	}

	if clipUI.sel >= 0 && clipUI.sel < len(clipHist) {
		aplicativo.clipCopiarItem(clipUI.sel)
	}
}

func clipTeclarEspaco(a *app) {
	if clipUI.sel >= 0 && clipUI.sel < len(clipHist) {
		a.clipAlternarMarca(clipUI.sel)
	}
}

func clipTeclarApagar(a *app) {
	if clipUI.sel >= 0 && clipUI.sel < len(clipHist) {
		a.clipRemover(clipUI.sel)
	}
}

func clipTeclarAtalho(aplicativo *app, tecla uint32) bool {
	switch tecla {
	case vkEscape:
		aplicativo.fecharClipboard()

		return true
	case vkReturn:
		clipTeclarEnter(aplicativo)

		return true
	case vkSpace:
		clipTeclarEspaco(aplicativo)

		return true
	case vkDelete:
		clipTeclarApagar(aplicativo)

		return true
	}

	return false
}

func clipTeclarPasso(aplicativo *app, tecla uint32) int {
	switch tecla {
	case vkUp:
		return -1
	case vkDown:
		return 1
	case vkA:
		if getKeyDown(vkControl) {
			aplicativo.clipAlternarTudo()
		}

		return 0
	default:
		return 0
	}
}

func clipAplicarPasso(hwnd uintptr, passo int) {
	if passo == 0 || len(clipHist) == 0 {
		return
	}

	clipUI.sel += passo
	if clipUI.sel < 0 {
		clipUI.sel = 0
	}

	if clipUI.sel >= len(clipHist) {
		clipUI.sel = len(clipHist) - 1
	}

	clipMostrarSelecionado()
	pInvalidateRect.Call(hwnd, 0, 0)
}

func clipAoTeclar(hwnd uintptr, aplicativo *app, wparam uintptr) uintptr {
	if aplicativo == nil || !clipUI.ativo {
		return 0
	}

	tecla := uint32(wparam)
	if clipTeclarAtalho(aplicativo, tecla) {
		return 0
	}

	clipAplicarPasso(hwnd, clipTeclarPasso(aplicativo, tecla))

	return 0
}

func clipAoDesativar(a *app, wparam uintptr) uintptr {
	if loword(wparam) == waInactive && a != nil && clipUI.ativo {
		clipUI.saiuFoco = true

		a.fecharClipboard()
	}

	return 0
}

func clipCursorSobreAcao(posicao struct{ x, y int32 }) bool {
	if clipDentroBotao(posicao.x, posicao.y, clipUI.limpar) {
		return true
	}

	if clipDentroBotao(posicao.x, posicao.y, clipUI.enviar) {
		return true
	}

	return clipDentroBotao(posicao.x, posicao.y, clipUI.contar)
}

func clipCursorSobreLinha(posicao struct{ x, y int32 }) bool {
	linha := clipLinhaEm(posicao.y)
	if linha < 0 {
		return false
	}

	selecionada := clipUI.rows[linha]

	return (posicao.x >= selecionada.check.Left && posicao.x < selecionada.check.Right) ||
		(posicao.x >= selecionada.pin.Left && posicao.x < selecionada.pin.Right) ||
		(posicao.x >= selecionada.del.Left && posicao.x < selecionada.del.Right)
}

func clipAoCursor(hwnd uintptr, a *app) uintptr {
	if a == nil || !clipUI.ativo {
		return 0
	}

	var posicao struct{ x, y int32 }
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&posicao)))
	pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&posicao)))

	mao := clipCursorSobreAcao(posicao) || clipCursorSobreLinha(posicao)
	if mao {
		pSetCursor.Call(loadCursor(idcHand))
	} else {
		pSetCursor.Call(loadCursor(idcArrow))
	}

	return 1
}

func clipProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmPaint:
		return clipAoPintar(hwnd, aplicativo)
	case wmSize:
		d2dClip.redimensionar(clipUI.w, clipUI.h)

		return 0
	case wmEraseBkgnd:
		return 1
	case wmMouseMove:
		return clipAoMoverMouse(hwnd, aplicativo, lparam)
	case wmLButtonDown:
		return clipAoClicar(hwnd, aplicativo, lparam)
	case wmMouseWheel:
		return clipAoRolar(hwnd, aplicativo, wparam)
	case wmKeyDown:
		return clipAoTeclar(hwnd, aplicativo, wparam)
	case wmActivate:
		return clipAoDesativar(aplicativo, wparam)
	case wmSetCursor:
		return clipAoCursor(hwnd, aplicativo)
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}

func clipMostrarSelecionado() {
	aplicativo := theApp
	if aplicativo == nil || clipUI.sel < 0 {
		return
	}

	top := clipUI.headerH + aplicativo.sc(4)
	bottom := clipUI.h - clipUI.footerH

	y := top
	for i := 0; i < clipUI.sel && i < len(clipHist); i++ {
		y += clipAlturaItem(clipHist[i]) + aplicativo.sc(2)
	}

	alt := clipAlturaItem(clipHist[clipUI.sel])
	if y < top+clipUI.scroll {
		clipUI.scroll = y - top
	} else if y+alt > bottom+clipUI.scroll {
		clipUI.scroll = y + alt - bottom
	}

	clipLayout()
}
