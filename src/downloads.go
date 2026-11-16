package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"
)

//exhaustruct:ignore // Download ativo: construtor parcial por desenho.
type downloadAtivo struct {
	ID          int    `json:"id"`
	Nome        string `json:"nome"`
	Caminho     string `json:"caminho"`
	URI         string `json:"uri"`
	Recebido    int64  `json:"recebido"`
	Total       int64  `json:"total"`
	Estado      int32  `json:"estado"`
	PodeRetomar bool   `json:"poderetomar"`
	Cancelado   bool   `json:"cancelado"`
	Quando      string `json:"quando"`
	Velocidade  int64  `json:"velocidade"`
	ETA         int64  `json:"eta"`
	Reiniciado  bool   `json:"reiniciado"`

	tentativas int
	caminhoOk  bool
	op         *ICoreWebView2DownloadOperation
	ultimoPct  int
	ultimoEst  int32
	amostraT   int64
	amostraB   int64
}

func downloadExtrairInfo(operacao *ICoreWebView2DownloadOperation) (string, string, string) {
	var (
		uri, caminho, nome string
		uriPtr, caminhoPtr *uint16
	)

	operacao.vtbl.GetURI.Call(uintptr(unsafe.Pointer(operacao)), uintptr(unsafe.Pointer(&uriPtr)))
	operacao.vtbl.GetResultFilePath.Call(uintptr(unsafe.Pointer(operacao)), uintptr(unsafe.Pointer(&caminhoPtr)))

	uri = takeString(uriPtr)
	caminho = takeString(caminhoPtr)

	nome = filepath.Base(caminho)
	if nome == "." || nome == "" {
		if u := strings.LastIndexAny(uri, "/?"); u >= 0 {
			nome = uri[u+1:]
		}

		if nome == "" {
			nome = idDownload
		}
	}

	return uri, caminho, nome
}

func (a *app) downloadProcurarDuplicado(caminho, uri string) *downloadAtivo {
	for _, antigo := range a.downloads {
		if antigo == nil || antigo.Cancelado || antigo.Estado == 2 {
			continue
		}

		if antigo.Caminho == caminho && antigo.URI == uri {
			return antigo
		}
	}

	return nil
}

func (a *app) downloadEncerrarPorTentativas(antigo *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	const maxTentativasDownload = 4

	antigo.tentativas++
	if antigo.tentativas <= maxTentativasDownload {
		return
	}

	operacao.vtbl.Cancel.Call(uintptr(unsafe.Pointer(operacao)))
	operacao.vtbl.Release.Call(uintptr(unsafe.Pointer(operacao)))

	if antigo.op != nil {
		a.downloadOpsMu.Lock()
		delete(a.downloadOps, uintptr(unsafe.Pointer(antigo.op)))
		a.downloadOpsMu.Unlock()
		antigo.op.vtbl.Cancel.Call(uintptr(unsafe.Pointer(antigo.op)))
		antigo.op.vtbl.Release.Call(uintptr(unsafe.Pointer(antigo.op)))
		antigo.op = nil
	}

	antigo.Estado = 1
	antigo.PodeRetomar = false
	antigo.Recebido = 0
	antigo.Velocidade = 0
	antigo.ETA = -1
	a.dizerErrof(txtFmtDlInterromp, antigo.Nome)
	a.invalidateBar()
	a.salvarRegistroDownloads()
	a.refreshDownloadsPage()
}

func (a *app) downloadSubstituirOp(antigo *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	if antigo.op == nil || antigo.op == operacao {
		return
	}

	a.downloadOpsMu.Lock()
	delete(a.downloadOps, uintptr(unsafe.Pointer(antigo.op)))
	a.downloadOpsMu.Unlock()
	antigo.op.vtbl.Cancel.Call(uintptr(unsafe.Pointer(antigo.op)))
	antigo.op.vtbl.Release.Call(uintptr(unsafe.Pointer(antigo.op)))
}

func (a *app) downloadCriarNovo(operacao *ICoreWebView2DownloadOperation, nome, caminho, uri string) {
	a.downloadSeq++
	descarga := &downloadAtivo{
		ID:        a.downloadSeq,
		Nome:      nome,
		Caminho:   caminho,
		URI:       uri,
		Total:     -1,
		Quando:    time.Now().Format(time.RFC3339),
		ETA:       -1,
		op:        operacao,
		ultimoEst: -1,
	}
	a.downloads = append(a.downloads, descarga)
	a.podarDownloads()
	a.ligarHandlersDownload(descarga, operacao)
	a.atualizarDownload(descarga)

	a.dizerf(txtFmtDlBaixando, descarga.Nome)
	a.invalidateBar()

	a.refreshDownloadsPage()
}

func (a *app) iniciarDownload(args *ICoreWebView2DownloadStartingEventArgs) {
	if a == nil || args == nil {
		return
	}

	var operacao *ICoreWebView2DownloadOperation

	hr, _, _ := args.vtbl.GetDownloadOperation.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&operacao)))
	if Failed(hr) || operacao == nil {
		return
	}

	args.vtbl.PutHandled.Call(uintptr(unsafe.Pointer(args)), 1)

	uri, caminho, nome := downloadExtrairInfo(operacao)

	if antigo := a.downloadProcurarDuplicado(caminho, uri); antigo != nil {
		const maxTentativasRetomada = 4
		if antigo.tentativas >= maxTentativasRetomada {
			a.downloadEncerrarPorTentativas(antigo, operacao)

			return
		}
		// Reaproveita o registro existente em vez de duplicar a linha.
		a.downloadSubstituirOp(antigo, operacao)

		antigo.op = operacao
		antigo.Nome = nome
		antigo.Caminho = caminho
		antigo.URI = uri
		antigo.Estado = 0
		antigo.Cancelado = false
		antigo.Reiniciado = false
		antigo.Recebido = 0
		antigo.Total = -1
		antigo.caminhoOk = false
		antigo.ultimoPct = -1
		antigo.ultimoEst = -1
		antigo.Quando = time.Now().Format(time.RFC3339)
		a.ligarHandlersDownload(antigo, operacao)
		a.atualizarDownload(antigo)
		a.status = fmt.Sprintf(txtFmtDlRetomando, antigo.Nome)
		a.invalidateBar()

		a.refreshDownloadsPage()

		return
	}

	a.downloadCriarNovo(operacao, nome, caminho, uri)
}

// podarDownloads limita a lista em memória: concluídos/cancelados antigos
// saem primeiro; downloads em andamento nunca são descartados.
func (a *app) podarDownloads() {
	const maximo = 100
	if len(a.downloads) <= maximo {
		return
	}

	restantes := make([]*downloadAtivo, 0, maximo)

	sobra := len(a.downloads) - maximo
	for _, descarga := range a.downloads {
		if sobra > 0 && descarga != nil &&
			(descarga.Estado == 2 || descarga.Cancelado || (descarga.Estado == 1 && !descarga.PodeRetomar)) {
			sobra--

			continue
		}

		restantes = append(restantes, descarga)
	}

	a.downloads = restantes
}

func (a *app) downloadRegistrarOp(descarga *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	a.downloadOpsMu.Lock()
	if a.downloadOps == nil {
		a.downloadOps = map[uintptr]*downloadAtivo{}
	}

	a.downloadOps[uintptr(unsafe.Pointer(operacao))] = descarga
	a.downloadOpsMu.Unlock()
}

func (a *app) downloadAoReceber(sender *ICoreWebView2DownloadOperation) {
	if sender == nil {
		return
	}

	a.downloadOpsMu.Lock()
	item := a.downloadOps[uintptr(unsafe.Pointer(sender))]
	a.downloadOpsMu.Unlock()

	if item != nil {
		a.atualizarDownload(item)
	}
}

func (a *app) downloadAoMudarEstado(sender *ICoreWebView2DownloadOperation) {
	if sender == nil {
		return
	}

	a.downloadOpsMu.Lock()
	item := a.downloadOps[uintptr(unsafe.Pointer(sender))]
	a.downloadOpsMu.Unlock()

	if item == nil {
		return
	}

	a.atualizarDownload(item)

	const estadoDownloadConcluido = 2
	if item.Estado != estadoDownloadConcluido {
		return
	}

	a.downloadConcluir(item, sender)
}

func (a *app) downloadConcluir(item *downloadAtivo, sender *ICoreWebView2DownloadOperation) {
	a.dizerOkf(txtFmtDlConcluido, item.Nome)
	a.invalidateBar()

	a.downloadOpsMu.Lock()
	delete(a.downloadOps, uintptr(unsafe.Pointer(sender)))
	a.downloadOpsMu.Unlock()

	if item.op == sender {
		item.op = nil

		sender.vtbl.Release.Call(uintptr(unsafe.Pointer(sender)))
	}

	a.salvarRegistroDownloads()
}

func (a *app) downloadArmarTimeout(descarga *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	const timeoutParadoDownloadSegundos = 12
	time.AfterFunc(timeoutParadoDownloadSegundos*time.Second, func() {
		a.depois(func() { a.downloadExpirarParado(descarga, operacao) })
	})
}

func (a *app) downloadExpirarParado(descarga *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	if descarga.op != operacao || descarga.Estado != 0 || descarga.Cancelado || descarga.Recebido > 0 {
		return
	}

	a.downloadOpsMu.Lock()
	delete(a.downloadOps, uintptr(unsafe.Pointer(operacao)))
	a.downloadOpsMu.Unlock()
	operacao.vtbl.Cancel.Call(uintptr(unsafe.Pointer(operacao)))
	operacao.vtbl.Release.Call(uintptr(unsafe.Pointer(operacao)))

	descarga.op = nil
	descarga.Estado = 1
	descarga.PodeRetomar = false
	descarga.Reiniciado = false
	descarga.Velocidade = 0
	descarga.ETA = -1
	a.dizerErrof(txtFmtDlInterromp, descarga.Nome)
	a.invalidateBar()
	a.salvarRegistroDownloads()
	a.refreshDownloadsPage()
}

func (a *app) ligarHandlersDownload(descarga *downloadAtivo, operacao *ICoreWebView2DownloadOperation) {
	if descarga == nil || operacao == nil {
		return
	}

	a.downloadRegistrarOp(descarga, operacao)

	manipuladorBytes := newDownloadBytesHandler(func(sender *ICoreWebView2DownloadOperation, _ *IUnknown) uintptr {
		a.downloadAoReceber(sender)

		return SOK
	})
	pin(manipuladorBytes)

	var tok1 eventToken
	operacao.vtbl.AddBytesReceivedChanged.Call(
		uintptr(unsafe.Pointer(operacao)),
		uintptr(unsafe.Pointer(manipuladorBytes)),
		uintptr(unsafe.Pointer(&tok1)))
	manipuladorBytes.vtbl.Release.Call(uintptr(unsafe.Pointer(manipuladorBytes)))

	manipuladorEstado := newDownloadStateHandler(func(sender *ICoreWebView2DownloadOperation, _ *IUnknown) uintptr {
		a.downloadAoMudarEstado(sender)

		return SOK
	})
	pin(manipuladorEstado)

	var tok2 eventToken
	operacao.vtbl.AddStateChanged.Call(
		uintptr(unsafe.Pointer(operacao)),
		uintptr(unsafe.Pointer(manipuladorEstado)),
		uintptr(unsafe.Pointer(&tok2)))
	manipuladorEstado.vtbl.Release.Call(uintptr(unsafe.Pointer(manipuladorEstado)))

	a.downloadArmarTimeout(descarga, operacao)
}

func downloadLerEstado(descarga *downloadAtivo) (int64, int64, int32, int32) {
	var (
		recebido, total int64
		estado, pode    int32
	)

	descarga.op.vtbl.GetBytesReceived.Call(uintptr(unsafe.Pointer(descarga.op)), uintptr(unsafe.Pointer(&recebido)))
	descarga.op.vtbl.GetTotalBytesToReceive.Call(uintptr(unsafe.Pointer(descarga.op)), uintptr(unsafe.Pointer(&total)))
	descarga.op.vtbl.GetState.Call(uintptr(unsafe.Pointer(descarga.op)), uintptr(unsafe.Pointer(&estado)))
	descarga.op.vtbl.GetCanResume.Call(uintptr(unsafe.Pointer(descarga.op)), uintptr(unsafe.Pointer(&pode)))

	return recebido, total, estado, pode
}

func downloadCorrigirCaminho(descarga *downloadAtivo, estado int32) {
	if estado != 2 || descarga.op == nil || descarga.caminhoOk {
		return
	}

	_, statErr := os.Stat(descarga.Caminho)
	if descarga.ultimoEst != 2 || statErr != nil {
		var p *uint16
		descarga.op.vtbl.GetResultFilePath.Call(uintptr(unsafe.Pointer(descarga.op)), uintptr(unsafe.Pointer(&p)))

		if s := caminhoCorrigidoUnicode(takeString(p)); s != "" && s != descarga.Caminho {
			descarga.Caminho = s
			descarga.Nome = filepath.Base(s)
		}
	}

	if _, err := os.Stat(descarga.Caminho); err == nil {
		descarga.caminhoOk = true
	}
}

func downloadAmostrarVelocidade(descarga *downloadAtivo, recebido int64, agoraNano int64) {
	if descarga.amostraT <= 0 || agoraNano <= descarga.amostraT {
		descarga.amostraT, descarga.amostraB = agoraNano, recebido

		return
	}

	const nanosPorSegundoDownload = 1e9

	dt := float64(agoraNano-descarga.amostraT) / nanosPorSegundoDownload

	db := recebido - descarga.amostraB
	if dt > 0 && db >= 0 {
		velocidade := int64(float64(db) / dt)
		if descarga.Velocidade == 0 {
			descarga.Velocidade = velocidade
		} else {
			const (
				pesoHistoricoVelocidade = 3
				divisorMediaVelocidade  = 4
			)

			descarga.Velocidade = (descarga.Velocidade*pesoHistoricoVelocidade + velocidade) / divisorMediaVelocidade
		}
	}

	descarga.amostraT, descarga.amostraB = agoraNano, recebido
}

func downloadCalcularETA(descarga *downloadAtivo, recebido, total int64) {
	if descarga.Velocidade > 0 && total > recebido {
		descarga.ETA = (total - recebido) / descarga.Velocidade
	} else {
		descarga.ETA = -1
	}
}

func downloadNotificarProgresso(
	aplicativo *app, descarga *downloadAtivo, recebido, anterior int64, pct int, estado int32,
) {
	const limiteNotificacaoProgressoKB = 512
	if pct == descarga.ultimoPct && estado == descarga.ultimoEst && recebido-anterior < limiteNotificacaoProgressoKB*1024 {
		return
	}

	descarga.ultimoPct = pct
	descarga.ultimoEst = estado

	aplicativo.refreshDownloadsPage()
	aplicativo.invalidateDownBar()
}

func (a *app) atualizarDownload(descarga *downloadAtivo) {
	if a == nil || descarga == nil || descarga.op == nil {
		return
	}

	anterior := descarga.Recebido
	recebido, total, estado, pode := downloadLerEstado(descarga)
	descarga.Recebido = recebido
	descarga.Total = total
	descarga.Estado = estado
	descarga.PodeRetomar = pode != 0

	downloadCorrigirCaminho(descarga, estado)
	downloadAmostrarVelocidade(descarga, recebido, time.Now().UnixNano())
	downloadCalcularETA(descarga, recebido, total)

	pct := -1
	if total > 0 {
		pct = int(recebido * 100 / total)
	}

	downloadNotificarProgresso(a, descarga, recebido, anterior, pct, estado)
}

func (a *app) refreshDownloadsPage() {
	if a == nil || a.hwnd == 0 {
		return
	}

	if cfgUI.ativo && cfgUI.hwnd != 0 && cfgUI.secao == cfgSecDown {
		a.cfgLayout()
		pInvalidateRect.Call(cfgUI.hwnd, 0, 0)
	}
}

func (a *app) downloadPorID(ident int) *downloadAtivo {
	for _, descarga := range a.downloads {
		if descarga.ID == ident {
			return descarga
		}
	}

	return nil
}

// caminhoCorrigidoUnicode conserta o mojibake que o WebView2 as vezes
// devolve em nomes com acentos (bytes UTF-8 lidos como Latin-1). So troca
// quando o caminho reinterpretado existe de verdade no disco.
func caminhoCorrigidoUnicode(caminho string) string {
	const maxRunaLatina1 = 0xFF

	if caminho == "" {
		return caminho
	}

	if _, err := os.Stat(caminho); err == nil {
		return caminho
	}

	tentativa := make([]byte, 0, len(caminho))
	for _, r := range caminho {
		if r > maxRunaLatina1 {
			return caminho
		}

		tentativa = append(tentativa, byte(r))
	}

	if !utf8.Valid(tentativa) {
		return caminho
	}

	alt := string(tentativa)
	if alt == caminho {
		return caminho
	}

	if _, err := os.Stat(alt); err == nil {
		return alt
	}

	return caminho
}

func (a *app) pausarDownload(ident int) {
	descarga := a.downloadPorID(ident)
	if descarga == nil || descarga.op == nil {
		return
	}

	descarga.op.vtbl.Pause.Call(uintptr(unsafe.Pointer(descarga.op)))
	a.atualizarDownload(descarga)
	a.status = fmt.Sprintf(txtFmtDlPausado, descarga.Nome)
	a.invalidateBar()

	a.refreshDownloadsPage()
}

func (a *app) retomarDownload(ident int) {
	descarga := a.downloadPorID(ident)
	if descarga == nil {
		return
	}

	if descarga.op == nil {
		a.tentarDeNovo(ident)

		return
	}

	descarga.Cancelado = false
	descarga.op.vtbl.Resume.Call(uintptr(unsafe.Pointer(descarga.op)))
	a.atualizarDownload(descarga)
	a.status = fmt.Sprintf(txtFmtDlRetoma, descarga.Nome)
	a.invalidateBar()

	a.refreshDownloadsPage()
}

// tentarDeNovo re-baixa pela URL original (para interrompidos de outra
// sessao ou falhas; URLs expiradas vao falhar de novo e tudo bem).
func (a *app) tentarDeNovo(ident int) {
	descarga := a.downloadPorID(ident)
	if descarga == nil || descarga.URI == "" {
		return
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtDlSemPainel)

		return
	}

	if !urlUtilizavel(descarga.URI) {
		a.dizerf(txtDlURLRuim)

		return
	}

	a.removerDownload(ident, true)
	a.status = fmt.Sprintf(txtFmtDlDeNovo, descarga.Nome)
	a.invalidateBar()
	a.left.wv.Navigate(descarga.URI)
}

// removerDownload tira um item da lista (silencioso para uso interno).
func (a *app) removerDownload(ident int, silencioso bool) {
	for idx, descarga := range a.downloads {
		if descarga == nil || descarga.ID != ident {
			continue
		}

		if descarga.op != nil {
			a.downloadOpsMu.Lock()
			delete(a.downloadOps, uintptr(unsafe.Pointer(descarga.op)))
			a.downloadOpsMu.Unlock()
			descarga.op.vtbl.Release.Call(uintptr(unsafe.Pointer(descarga.op)))
			descarga.op = nil
		}

		a.downloads = append(a.downloads[:idx], a.downloads[idx+1:]...)
		if !silencioso {
			a.status = fmt.Sprintf(txtFmtDlRemovido, descarga.Nome)
			a.invalidateBar()
		}

		break
	}

	a.salvarRegistroDownloads()
	a.refreshDownloadsPage()
}

// copiarURLDownload copia a URL original (para colar no gerenciador externo).
func (a *app) copiarURLDownload(ident int) {
	descarga := a.downloadPorID(ident)
	if descarga == nil || descarga.URI == "" {
		return
	}

	if copiarTexto(descarga.URI) {
		a.dizerf(txtFmtURLCopiada, descarga.Nome)
	}
}

func downloadAtivoParaBarra(d *downloadAtivo) bool {
	if d == nil || d.Cancelado || d.Estado == 2 {
		return false
	}

	return d.Estado == 0 || (d.Estado == 1 && d.PodeRetomar)
}

func downloadSomarProgresso(downloads []*downloadAtivo) (int64, int64, int) {
	var (
		rec, tot int64
		num      int
	)

	for _, descarga := range downloads {
		if !downloadAtivoParaBarra(descarga) {
			continue
		}

		num++

		rec += descarga.Recebido
		if descarga.Total > 0 {
			tot += descarga.Total
		}
	}

	return rec, tot, num
}

func downloadPorcentagemBarra(rec, tot int64, num int) (int, int, bool) {
	if num == 0 {
		return 0, 0, false
	}

	if tot <= 0 {
		return -1, num, true
	}

	return int(rec * 100 / tot), num, true
}

// progressoDownloads agrega os ativos para a barra da UI principal.
// Devolve pct 0-100 (-1 = indeterminado), quantidade e se ha algo ativo.
func (a *app) progressoDownloads() (int, int, bool) {
	if a == nil {
		return 0, 0, false
	}

	rec, tot, n := downloadSomarProgresso(a.downloads)

	return downloadPorcentagemBarra(rec, tot, n)
}

// revelarArquivo abre o Explorer com o arquivo selecionado.
func (a *app) revelarArquivo(caminho string) {
	abs, err := filepath.Abs(caminho)
	if err != nil {
		return
	}

	dir := pastaDownloads()
	if dir == "" {
		return
	}

	absDir, _ := filepath.Abs(dir)

	rel, err := filepath.Rel(absDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		a.dizerf(txtDlForaPasta)

		return
	}

	if _, err := os.Stat(abs); err != nil {
		a.dizerf(txtDlSumiu)

		return
	}

	pShellExecuteW.Call(a.hwnd,
		uintptr(unsafe.Pointer(utf16("open"))),
		uintptr(unsafe.Pointer(utf16("explorer.exe"))),
		uintptr(unsafe.Pointer(utf16(`/select,"`+abs+`"`))),
		0, swShow)
}

func (a *app) cancelarDownload(ident int) {
	descarga := a.downloadPorID(ident)
	if descarga == nil || descarga.op == nil {
		return
	}

	descarga.Cancelado = true
	descarga.op.vtbl.Cancel.Call(uintptr(unsafe.Pointer(descarga.op)))
	a.atualizarDownload(descarga)
	a.status = fmt.Sprintf(txtFmtDlCancelado, descarga.Nome)
	a.invalidateBar()
	a.salvarRegistroDownloads()
	a.refreshDownloadsPage()
}

func (a *app) limparDownloads() {
	if a == nil {
		return
	}

	restantes := make([]*downloadAtivo, 0, len(a.downloads))
	removidos := 0

	for _, descarga := range a.downloads {
		if descarga == nil {
			continue
		}

		finalizado := descarga.Estado == 2 || descarga.Cancelado || (descarga.Estado == 1 && !descarga.PodeRetomar)
		if finalizado {
			removidos++

			if descarga.op != nil {
				a.downloadOpsMu.Lock()
				delete(a.downloadOps, uintptr(unsafe.Pointer(descarga.op)))
				a.downloadOpsMu.Unlock()
				descarga.op.vtbl.Release.Call(uintptr(unsafe.Pointer(descarga.op)))
				descarga.op = nil
			}

			continue
		}

		restantes = append(restantes, descarga)
	}

	a.downloads = restantes
	if removidos > 0 {
		a.status = fmt.Sprintf(txtFmtDlLimpos, quant(removidos, "removido", "removidos"))
	} else {
		a.status = txtDlNadaLimpar
	}

	a.invalidateBar()
	a.salvarRegistroDownloads()
	a.refreshDownloadsPage()
}

func caminhoRegistroDownloads() string {
	return filepath.Join(dataDir, "downloads.json")
}

// salvarRegistroDownloads persiste o historico (concluidos, cancelados e
// interrompidos, limite de 100). Downloads ativos nao sao persistidos: sem
// o handle do WebView2 nao da para retomar o byte exato — na saida eles
// sao marcados como interrompidos para tentar de novo.
func (a *app) salvarRegistroDownloads() {
	if a == nil {
		return
	}

	a.podarDownloads()

	lista := make([]downloadAtivo, 0, len(a.downloads))
	for _, descarga := range a.downloads {
		if descarga == nil || descarga.op != nil {
			continue
		}

		lista = append(lista, *descarga)
	}

	caminho := caminhoRegistroDownloads()
	if len(lista) == 0 {
		_ = os.Remove(caminho)

		return
	}

	conteudo, err := json.Marshal(lista)
	if err != nil {
		return
	}

	tmp := caminho + ".tmp"
	if err := os.WriteFile(tmp, conteudo, 0o600); err != nil {
		return
	}

	if err := os.Rename(tmp, caminho); err != nil {
		_ = os.Remove(tmp)
	}
}

func downloadNormalizarItem(item downloadAtivo, maxID *int) downloadAtivo {
	item.op = nil
	item.tentativas = 0
	item.caminhoOk = false
	item.ultimoPct = -1
	item.ultimoEst = -1
	item.amostraT = 0
	item.amostraB = 0
	item.Velocidade = 0

	item.ETA = -1
	if item.Estado != 2 && !item.Cancelado {
		item.Estado = 1
		item.PodeRetomar = false
		item.Reiniciado = false
	}

	if item.ID <= 0 {
		*maxID++
		item.ID = *maxID
	} else if item.ID > *maxID {
		*maxID = item.ID
	}

	return item
}

func downloadCarregarLista(b []byte) ([]downloadAtivo, bool) {
	var lista []downloadAtivo
	if json.Unmarshal(semBOM(b), &lista) != nil {
		return nil, false
	}

	return lista, true
}

// carregarRegistroDownloads restaura o historico da sessao anterior.
// Entradas nao finais viram interrompidas (sem resume automatico).
func (a *app) carregarRegistroDownloads() {
	if a == nil || a.downloadRegCarreg {
		return
	}

	a.downloadRegCarreg = true

	b, err := os.ReadFile(caminhoRegistroDownloads())
	if err != nil || len(b) == 0 {
		return
	}

	lista, ok := downloadCarregarLista(b)
	if !ok {
		return
	}

	maxID := a.downloadSeq
	for i := range lista {
		cp := downloadNormalizarItem(lista[i], &maxID)
		a.downloads = append(a.downloads, &cp)
	}

	a.downloadSeq = maxID
	a.podarDownloads()
}

func (a *app) downloadsAtivos() []downloadAtivo {
	if a == nil {
		return nil
	}

	lista := make([]downloadAtivo, 0, len(a.downloads))
	for _, descarga := range a.downloads {
		if descarga == nil {
			continue
		}

		if descarga.Estado == 2 && descarga.Caminho != "" && !descarga.caminhoOk {
			if _, err := os.Stat(descarga.Caminho); err != nil {
				if s := caminhoCorrigidoUnicode(descarga.Caminho); s != descarga.Caminho {
					descarga.Caminho = s
					descarga.Nome = filepath.Base(s)
				}
			}

			if _, err := os.Stat(descarga.Caminho); err == nil {
				descarga.caminhoOk = true
			}
		}

		lista = append(lista, *descarga)
	}

	return lista
}
