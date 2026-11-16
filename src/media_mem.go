//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//nolint:gochecknoglobals // Estado de QoS/eco + mutex: estado de processo.
var (
	qosMu      sync.Mutex
	wsUltimo   time.Time
	ecoEstado  bool
	ecoInicial = true
)

const (
	esperaWorkingSetMem     = 900 * time.Millisecond
	flagSnapshotProcessoMem = 0x00000002
	acessoConsultaMem       = 0x0100
	acessoQuotaMem          = 0x0400
	esperaAvisoMemLiberada  = 1200 * time.Millisecond
	esperaDescarteOciosoMem = 90 * time.Second
	esperaCongelarChatMem   = 4 * time.Second
	intervaloChatFrioMem    = 250
	esperaLimpezaPesadaMem  = 1500 * time.Millisecond
)

func (a *app) agendarLimpezaMemoria(pesada bool, atraso time.Duration) {
	if a == nil {
		return
	}

	if pesada && a.limpezaPendente {
		return
	}

	if pesada {
		a.limpezaPendente = true
	}

	time.AfterFunc(atraso, func() {
		a.depois(func() {
			if !pesada {
				go limparWorkingSet()

				return
			}

			a.limpezaPendente = false
			if time.Since(a.limpezaUltima) < 30*time.Second {
				return
			}

			a.limpezaUltima = time.Now()
			a.limpezaPesadaMemoria()
		})
	})
}

func (a *app) limpezaPesadaMemoria() {
	if !atomic.CompareAndSwapInt32(&memoriaPesadaRodando, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&memoriaPesadaRodando, 0)

	vistos := map[*pane]bool{}

	limpar := func(p *pane) {
		if p == nil || vistos[p] {
			return
		}

		vistos[p] = true
		if !p.ready || p.wv == nil {
			return
		}

		p.wv.CallDevTools("HeapProfiler.collectGarbage", "{}")
		p.wv.CallDevTools("Memory.simulatePressureNotification", `{"level":"critical"}`)
	}
	for _, p := range a.panesGerenciados() {
		limpar(p)
	}

	for _, t := range a.tabs {
		if t != nil {
			limpar(t.pane)
		}
	}

	limpar(a.popup)
	limpar(a.extFloat)

	if !a.ytAberto {
		limpar(a.ytPane)
	}

	go debug.FreeOSMemory()

	time.AfterFunc(esperaWorkingSetMem, func() { limparWorkingSet() })
}

type procDesc struct {
	pid  uint32
	nome string
}

func processosDescendentes() []procDesc {
	snap, _, _ := pCreateToolhelp32Snapshot.Call(flagSnapshotProcessoMem, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil
	}
	defer pCloseHandle.Call(snap)

	pai := map[uint32]uint32{}
	nomes := map[uint32]string{}

	var entradaProc processEntry32W

	entradaProc.Size = uint32(unsafe.Sizeof(entradaProc))

	ret, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entradaProc)))
	for ret != 0 {
		pai[entradaProc.Th32ProcessID] = entradaProc.Th32ParentProcessID
		nomes[entradaProc.Th32ProcessID] = strings.ToLower(windows.UTF16ToString(entradaProc.SzExeFile[:]))
		entradaProc.Size = uint32(unsafe.Sizeof(entradaProc))
		ret, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entradaProc)))
	}

	self, _, _ := pGetCurrentProcessID.Call()
	incluidos := map[uint32]bool{uint32(self): true}
	//exhaustruct:ignore // procDesc: nome preenchido na triagem abaixo; resto zero aqui.
	out := []procDesc{{pid: uint32(self)}}

	for range 8 {
		add := false

		for pid, ppid := range pai {
			if !incluidos[pid] && incluidos[ppid] {
				incluidos[pid] = true
				out = append(out, procDesc{pid: pid, nome: nomes[pid]})
				add = true
			}
		}

		if !add {
			break
		}
	}

	return out
}

func limparWorkingSet() {
	qosMu.Lock()

	if !wsUltimo.IsZero() && time.Since(wsUltimo) < time.Minute {
		qosMu.Unlock()

		return
	}

	wsUltimo = time.Now()
	qosMu.Unlock()
	// So o proprio processo: esvaziar o working set dos renderers/GPU do
	// WebView2 forca re-page-fault e engasga a navegacao. A economia nos
	// renderers ja e feita via TrySuspend/congelamento.
	self, _, _ := pGetCurrentProcessID.Call()

	handleProc, _, _ := pOpenProcess.Call(acessoConsultaMem|acessoQuotaMem, 0, self)
	if handleProc == 0 {
		return
	}

	pEmptyWorkingSet.Call(handleProc)
	pSetProcessWorkingSetSize.Call(handleProc, ^uintptr(0), ^uintptr(0))
	pCloseHandle.Call(handleProc)
}

func aplicarEcoQoS(ligar bool) {
	const (
		processPowerThrottling = 4
		executionSpeed         = 0x1
		queryLimitedInfo       = 0x1000
		setInformation         = 0x0200
	)

	//exhaustruct:ignore // Estado de energia: mascara zerada = sem mudanca; resto zero intencional.
	estado := struct {
		Version     uint32
		ControlMask uint32
		StateMask   uint32
	}{Version: 1, ControlMask: executionSpeed}
	if ligar {
		estado.StateMask = executionSpeed
	}

	qosMu.Lock()

	if !ecoInicial && ligar == ecoEstado {
		qosMu.Unlock()

		return
	}

	ecoInicial = false
	ecoEstado = ligar
	qosMu.Unlock()

	for _, pr := range processosDescendentes() {
		h, _, _ := pOpenProcess.Call(queryLimitedInfo|setInformation, 0, uintptr(pr.pid))
		if h == 0 {
			continue
		}

		pSetProcessInformation.Call(h, processPowerThrottling, uintptr(unsafe.Pointer(&estado)), unsafe.Sizeof(estado))
		pCloseHandle.Call(h)
	}
}

func removerHistoricoPerfil(profileDir string) int {
	if profileDir == "" {
		return 0
	}

	base := resolverBasePerfil(profileDir)
	def := filepath.Join(base, "Default")
	removidos := removerNomesPerfil(def)

	removidos += removerExtrasPerfil(base, def)
	removidos += removerLogsPerfil(def)

	return removidos
}

// resolverBasePerfil devolve a base com os dados do perfil.
func resolverBasePerfil(profileDir string) string {
	base := filepath.Join(profileDir, "EBWebView")
	if _, err := os.Stat(base); err != nil {
		base = profileDir
	}

	return base
}

// removerNomesPerfil remove os arquivos de historico conhecidos.
func removerNomesPerfil(def string) int {
	nomes := []string{
		"History", "History-journal", "History Provider Cache",
		"Top Sites", "Top Sites-journal",
		"Favicons", "Favicons-journal",
		"Visited Links",
		"Shortcuts", "Shortcuts-journal",
		"Network Action Predictor", "Network Action Predictor-journal",
		"Media History", "Media History-journal",
		"Current Session", "Last Session", "Current Tabs", "Last Tabs",
		"Sessions", "EdgeSessions",
	}
	removidos := 0

	for _, n := range nomes {
		if removerSeExistir(filepath.Join(def, n)) {
			removidos++
		}
	}

	return removidos
}

// removerExtrasPerfil remove pastas extras e logs fixos.
func removerExtrasPerfil(base, def string) int {
	removidos := 0

	for _, p := range []string{
		filepath.Join(base, "Crashpad"),
		filepath.Join(def, "LOG"),
		filepath.Join(def, "LOG.old"),
	} {
		if removerSeExistir(p) {
			removidos++
		}
	}

	return removidos
}

// removerLogsPerfil remove os arquivos *.log* do perfil.
func removerLogsPerfil(def string) int {
	removidos := 0

	if logs, err := filepath.Glob(filepath.Join(def, "*.log*")); err == nil {
		for _, p := range logs {
			if os.RemoveAll(p) == nil {
				removidos++
			}
		}
	}

	return removidos
}

// removerSeExistir remove o caminho quando ele existe.
func removerSeExistir(p string) bool {
	if _, err := os.Stat(p); err != nil {
		return false
	}

	return os.RemoveAll(p) == nil
}

func limparCachePerfil(profileDir string) {
	if profileDir == "" {
		return
	}

	base := resolverBasePerfil(profileDir)

	alvos := alvosFixosCache(base)
	alvos = append(alvos, coletarCachesAntigos(base)...)

	for _, caminho := range alvos {
		if _, err := os.Stat(caminho); err != nil {
			continue
		}

		// Best-effort: o que nao sair agora sai na proxima limpeza.
		_ = os.RemoveAll(caminho)
	}

	removerHistoricoPerfil(profileDir)
}

// alvosFixosCache lista as pastas de cache conhecidas.
func alvosFixosCache(base string) []string {
	return []string{
		filepath.Join(base, "Default", "Cache"),
		filepath.Join(base, "Default", "Code Cache"),
		filepath.Join(base, "Default", "GPUCache"),
		filepath.Join(base, "Default", "DawnWebGPUCache"),
		filepath.Join(base, "Default", "DawnGraphiteCache"),
		filepath.Join(base, "GrShaderCache"),
		filepath.Join(base, "ShaderCache"),
		filepath.Join(base, "GPUPersistentCache"),
	}
}

// coletarCachesAntigos acha pastas old_gpucache/old_grshadercache.
func coletarCachesAntigos(base string) []string {
	var extras []string

	for _, dir := range []string{base, filepath.Join(base, "Default")} {
		entradas, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entradas {
			n := strings.ToLower(e.Name())
			if e.IsDir() && (strings.HasPrefix(n, "old_gpucache") || strings.HasPrefix(n, "old_grshadercache")) {
				extras = append(extras, filepath.Join(dir, e.Name()))
			}
		}
	}

	return extras
}

func (a *app) matarServiceWorkers(origem string) {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		p = a.right
	}

	if p == nil || !p.ready || p.wv == nil {
		return
	}

	alvo := p
	alvo.wv.CallDevToolsResult("Target.getTargets", "{}", func(err error, res string) {
		if err != nil {
			return
		}

		var resposta struct {
			TargetInfos []struct {
				TargetID string `json:"targetId"`
				Type     string `json:"type"`
				URL      string `json:"url"`
			} `json:"targetInfos"`
		}
		if desembrulharEvalJS(res, &resposta) != nil {
			return
		}

		ids := coletarWorkers(resposta.TargetInfos, origem)
		if len(ids) == 0 {
			return
		}

		a.depois(func() {
			a.fecharWorkers(alvo, ids)
		})
	})
}

// coletarWorkers filtra os ids de workers da origem pedida.
func coletarWorkers(infos []struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}, origem string,
) []string {
	ids := []string{}

	for _, info := range infos {
		if info.Type != "service_worker" && info.Type != "shared_worker" {
			continue
		}

		if origem != "" && info.URL != origem+"/" && !strings.HasPrefix(info.URL, origem+"/") {
			continue
		}

		ids = append(ids, info.TargetID)
	}

	return ids
}

// fecharWorkers fecha os alvos workers no painel ainda pronto.
func (a *app) fecharWorkers(alvo *pane, ids []string) {
	if alvo.wv == nil || !alvo.ready {
		return
	}

	for _, id := range ids {
		if params, err := cdpFecharAlvo(id); err == nil {
			alvo.wv.CallDevTools("Target.closeTarget", params)
		}
	}
}

func (a *app) limparServiceWorkerDoSite(u string) {
	origem := origemDe(u)
	if origem == "" {
		return
	}

	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		p = a.right
	}

	if p == nil || !p.ready || p.wv == nil {
		return
	}

	params := fmt.Sprintf(`{"origin":%s,"storageTypes":"service_workers,cache_storage"}`, jsonDeString(origem))
	p.wv.CallDevTools("Storage.clearDataForOrigin", params)

	a.matarServiceWorkers(origem)
}

func (a *app) pararServicosDeFundo() {
	if a != nil && a.ytAberto {
		return
	}

	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		p = a.right
	}

	if p == nil || !p.ready || p.wv == nil {
		return
	}

	p.wv.CallDevTools("ServiceWorker.stopAllWorkers", "{}")

	a.matarServiceWorkers("")
}

func (a *app) liberarMemoriaAgora() {
	a.limpezaPesadaMemoria()
	time.AfterFunc(esperaAvisoMemLiberada, func() {
		a.depois(func() { a.dizerf(txtMemLiberada) })
	})
}

func (a *app) limpezaAposAcao(id string) {
	if a == nil || !a.st.LimpezaAcao || id == idLiberarMemoria {
		return
	}

	a.limpezaUltima = time.Now()
	a.limpezaPesadaMemoria()
}

func (a *app) suspenderTudo() {
	if a.tudoSuspenso {
		return
	}

	a.tudoSuspenso = true
	a.chatFrioGen++

	a.chatManual = false
	if a.chatFrio {
		a.chatFrio = false
		pKillTimer.Call(a.hwnd, timerChatFrio)
	}

	if a.right != nil {
		a.right.frio = false
		a.right.frioManual = false
	}

	a.chatFrioPendente = false
	for _, t := range a.tabs {
		if t != nil {
			suspenderPane(t.pane)
		}
	}

	for _, p := range []*pane{a.right, a.imgPanel, a.popup, a.extFloat} {
		suspenderPane(p)
	}

	a.agendarDescarteOcioso()

	if !a.ytAberto {
		a.pararServicosDeFundo()
		aplicarEcoQoS(true)
	}
}

func (a *app) agendarDescarteOcioso() {
	if !a.st.EconomiaMax || a.quitting || a.ytAberto {
		return
	}

	if !a.haConteudoParaDescarte() {
		return
	}

	a.descarteGen++
	gen := a.descarteGen

	time.AfterFunc(esperaDescarteOciosoMem, func() {
		a.depois(func() {
			a.dispararDescarteOcioso(gen)
		})
	})
}

// haConteudoParaDescarte diz se existe algo vivo para descartar.
func (a *app) haConteudoParaDescarte() bool {
	temChat := a.siteAtualIA().chave == chaveChatgpt && a.right != nil && a.right.ready && a.right.wv != nil
	temImg := a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil

	return temChat || temImg || a.haAbaViva()
}

// haAbaViva diz se alguma aba tem webview pronta.
func (a *app) haAbaViva() bool {
	for _, t := range a.tabs {
		if t != nil && t.pane != nil && t.pane.ready && t.pane.wv != nil {
			return true
		}
	}

	return false
}

// dispararDescarteOcioso executa o descarte quando a geracao segue atual.
func (a *app) dispararDescarteOcioso(gen uint64) {
	if gen != a.descarteGen || !a.tudoSuspenso || a.quitting {
		return
	}

	a.persist()
	a.descartarChatGPT()
	a.descartarImagensOcioso()
	a.descartarAbasEsquerdas()
}

func (a *app) descartarImagensOcioso() {
	if a.imgPanel == nil || !a.imgPanel.ready || a.imgPanel.wv == nil {
		return
	}

	a.fecharPainelImagens()
}

func (a *app) agendarCongelamentoChatGPT() {
	if !a.chatProntoParaCongelar() {
		return
	}

	a.chatFrioPendente = true
	a.chatFrioGen++
	gen := a.chatFrioGen

	time.AfterFunc(esperaCongelarChatMem, func() {
		a.depois(func() {
			a.dispararCongelamentoChatGPT(gen)
		})
	})
}

// chatProntoParaCongelar confere as pre-condicoes do congelamento.
func (a *app) chatProntoParaCongelar() bool {
	return a.congelamentoLigado() && a.chatElegivelParaCongelar() && a.chatOciosoParaCongelar()
}

// congelamentoLigado diz se a economia permite congelar o chat.
func (a *app) congelamentoLigado() bool {
	return a.st.EconomiaMax && !a.quitting && !a.ytAberto && a.st.RightVisible
}

// chatElegivelParaCongelar diz se o chat certo esta com painel pronto.
func (a *app) chatElegivelParaCongelar() bool {
	if !a.chatUsado || a.siteAtualIA().chave != chaveChatgpt {
		return false
	}

	return a.right != nil && a.right.ready && a.right.wv != nil
}

// chatOciosoParaCongelar diz se o chat esta sem atividade pendente.
func (a *app) chatOciosoParaCongelar() bool {
	return !a.chatFrio && !a.chatFrioPendente && !a.chatGerando && !a.chatAtivo
}

// dispararCongelamentoChatGPT congela quando a geracao segue atual.
func (a *app) dispararCongelamentoChatGPT(gen uint64) {
	a.chatFrioPendente = false
	if gen != a.chatFrioGen || !a.st.EconomiaMax || a.chatFrio || a.chatGerando || a.chatAtivo || a.quitting {
		return
	}

	a.congelarChatGPT()
}

func (a *app) congelarChatGPT() {
	if !a.chatCongelavel() {
		return
	}

	a.chatFrioPendente = false
	a.chatFrio = true
	a.chatManual = false
	a.right.frio = true
	a.right.frioManual = false
	a.right.frioDesde = time.Now()
	a.right.wv.SetFrozen(true)
	pSetTimer.Call(a.hwnd, timerChatFrio, intervaloChatFrioMem, 0)
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

// chatCongelavel confere se o painel do chat pode congelar agora.
func (a *app) chatCongelavel() bool {
	return a.painelChatPronto() && a.chatLivreDeAudioEIA() && a.chatSemFocoNemMinimizado()
}

// painelChatPronto diz se o painel do chat tem webview pronta.
func (a *app) painelChatPronto() bool {
	return a.right != nil && a.right.ready && a.right.wv != nil
}

// chatLivreDeAudioEIA diz se nada prende o chat ao vivo.
func (a *app) chatLivreDeAudioEIA() bool {
	if a.right.tocandoAudio && !a.right.mudo {
		return false
	}

	return a.chatUsado && a.siteAtualIA().chave == chaveChatgpt
}

// chatSemFocoNemMinimizado diz se o chat esta fora de uso e visivel.
func (a *app) chatSemFocoNemMinimizado() bool {
	if a.focus == idRight || a.cursorNoPainelChat() {
		return false
	}

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return false
	}

	return true
}

func (a *app) retomarChatGPTFrio() {
	if a.right != nil && a.right.descarregado {
		a.recarregarPaneDescarregado(a.right)
	}

	if !a.chatFrio && (a.right == nil || !a.right.frio) {
		return
	}

	a.chatFrio = false
	a.chatManual = false
	pKillTimer.Call(a.hwnd, timerChatFrio)

	if a.right != nil {
		a.right.frio = false
		a.right.frioManual = false

		a.right.frioDesde = time.Time{}
		if a.right.descarregado {
			a.recarregarPaneDescarregado(a.right)
		}

		if a.right.ready && a.right.wv != nil {
			a.right.wv.SetFrozen(false)
			setPaneVisible(a.right, true)
		}
	}

	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) congelarPaneGenerico(p *pane) {
	if p == nil || !p.ready || p.wv == nil || p.frio {
		return
	}

	if p.tocandoAudio && !p.mudo {
		return
	}

	if p == a.right && a.siteAtualIA().chave == chaveChatgpt {
		return
	}

	p.frio = true
	p.frioManual = false
	p.frioPendente = false
	p.frioDesde = time.Now()

	if p == a.left {
		p.wv.setMemoryLow(true)

		return
	}

	p.wv.SetFrozen(true)
}

func (a *app) retomarPaneGenerico(p *pane) {
	if p == nil || !p.frio {
		return
	}

	p.frio = false
	p.frioManual = false
	p.frioPendente = false
	p.frioDesde = time.Time{}

	p.ociosoDesde = time.Time{}
	if p.descarregado {
		a.recarregarPaneDescarregado(p)

		return
	}

	if p.ready && p.wv != nil {
		p.wv.SetFrozen(false)
	}
}

func (a *app) retomarFriosSobCursor() {
	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return
	}

	for _, p := range a.panesGerenciados() {
		if p == nil {
			continue
		}

		if !p.frio || p.frioManual {
			continue
		}

		if p == a.right && a.chatFrio {
			continue
		}

		if a.cursorNoPainel(p) {
			a.retomarPaneGenerico(p)
		}
	}
}

func (a *app) descarregarFrios(limite time.Duration) {
	descarregou := false

	for _, p := range a.panesGerenciados() {
		if !a.paneFrioDescarregavel(p, limite) {
			continue
		}

		a.descarregarPaneFrio(p)

		descarregou = true
	}

	if descarregou {
		a.pararServicosDeFundo()
		a.agendarLimpezaMemoria(true, esperaLimpezaPesadaMem)
	}
}

// paneFrioDescarregavel diz se o painel frio pode ser descarregado.
func (a *app) paneFrioDescarregavel(p *pane, limite time.Duration) bool {
	return a.paneFrioCandidato(p) && a.paneFrioMaduro(p, limite)
}

// paneFrioCandidato diz se o painel e candidato a descarte.
func (a *app) paneFrioCandidato(p *pane) bool {
	if p == nil || p == a.left {
		return false
	}

	return p.ready && p.wv != nil && p.frio && !p.frioManual && !p.descarregado
}

// paneFrioMaduro diz se o frio dura o bastante sem uso nem audio.
func (a *app) paneFrioMaduro(p *pane, limite time.Duration) bool {
	if p.tocandoAudio && !p.mudo {
		return false
	}

	if p.frioDesde.IsZero() || time.Since(p.frioDesde) < limite {
		return false
	}

	return !a.paneFocado(p) && !a.cursorNoPainel(p)
}

func (a *app) descarregarOcultos(limite time.Duration) {
	if !a.st.EconomiaMax || a.tudoSuspenso {
		return
	}

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return
	}

	for _, p := range []*pane{a.imgPanel, a.popup} {
		a.avaliarPaneOculto(p, limite)
	}
}

// avaliarPaneOculto descarrega o painel auxiliar quando ocioso ha tempo.
func (a *app) avaliarPaneOculto(p *pane, limite time.Duration) {
	if !paneOcultoCandidato(p) {
		return
	}

	if a.zerarOciosoOculto(p) {
		return
	}

	if a.iniciarOciosidadeOculta(p) {
		return
	}

	if a.ociosidadeOcultaCurta(p, limite) {
		return
	}

	if p == a.imgPanel {
		a.descarregarImagensOcultas(p)

		return
	}

	p.ociosoDesde = time.Time{}
	p.wv.Navigate("about:blank")
}

// paneOcultoCandidato diz se o painel pode entrar na conta de ociosos.
func paneOcultoCandidato(p *pane) bool {
	return p != nil && p.ready && p.wv != nil && !p.descarregado
}

// zerarOciosoOculto limpa a ociosidade de painel em uso; true = em uso.
func (a *app) zerarOciosoOculto(p *pane) bool {
	if p.tocandoAudio && !p.mudo {
		p.ociosoDesde = time.Time{}

		return true
	}

	if a.paneOcultoEmUso(p) {
		p.ociosoDesde = time.Time{}

		return true
	}

	return false
}

// iniciarOciosidadeOculta carimba o inicio; true = acabou de carimbar.
func (a *app) iniciarOciosidadeOculta(p *pane) bool {
	if p.ociosoDesde.IsZero() {
		p.ociosoDesde = time.Now()

		return true
	}

	return false
}

// ociosidadeOcultaCurta diz se a ociosidade ainda esta no limite.
func (a *app) ociosidadeOcultaCurta(p *pane, limite time.Duration) bool {
	return time.Since(p.ociosoDesde) < limite
}

// paneOcultoEmUso diz se o painel auxiliar esta visivel ou focado.
func (a *app) paneOcultoEmUso(p *pane) bool {
	return (p.visivel && p.rect.Width() > 0) || a.paneFocado(p)
}

// descarregarImagensOcultas guarda a URL e descarrega o painel de imagens.
func (a *app) descarregarImagensOcultas(p *pane) {
	endereco := strings.TrimSpace(p.ultimaURL)
	if endereco == "" {
		endereco = strings.TrimSpace(p.wv.Source())
	}

	p.urlDescarga = endereco
	p.descarregado = true
	p.ociosoDesde = time.Time{}
	p.wv.Navigate("about:blank")
}

func (a *app) descarregarPaneFrio(p *pane) {
	if !paneProntoParaDescarga(p) {
		return
	}

	endereco, pronto := urlParaDescarga(p)
	if !pronto {
		return
	}

	p.descarregado = true
	p.urlDescarga = endereco
	p.frio = false
	p.frioManual = false
	p.frioDesde = time.Time{}
	p.ociosoDesde = time.Time{}
	p.wv.SetFrozen(false)
	p.wv.EvalJS("String(window.scrollY||0)", func(err error, res string) {
		guardarScrollDescarga(p, err, res)

		a.depois(func() {
			if p.ready && p.wv != nil && p.descarregado {
				p.wv.Navigate("about:blank")
			}
		})
	})
}

// paneProntoParaDescarga diz se o painel frio pode ser descarregado.
func paneProntoParaDescarga(p *pane) bool {
	return p != nil && p.ready && p.wv != nil && !p.descarregado && p.frio && !p.frioManual
}

// urlParaDescarga resolve a URL a guardar antes de descarregar.
func urlParaDescarga(p *pane) (string, bool) {
	endereco := strings.TrimSpace(p.ultimaURL)
	if endereco == "" {
		endereco = strings.TrimSpace(p.wv.Source())
	}

	if endereco == "" || strings.HasPrefix(endereco, "about:") {
		return "", false
	}

	return endereco, true
}

// guardarScrollDescarga salva a posicao de rolagem do painel.
func guardarScrollDescarga(p *pane, err error, res string) {
	if err != nil {
		return
	}

	if v, e := strconv.ParseFloat(strings.TrimSpace(res), 64); e == nil {
		p.scrollDescarga = v
	}
}

func (a *app) gerenciarFrios() {
	if !a.st.EconomiaMax || a.tudoSuspenso || a.ytAberto {
		return
	}

	for _, p := range a.panesGerenciados() {
		a.gerenciarUmFrio(p)
	}
}

// gerenciarUmFrio avalia congelar, retomar ou marcar ociosidade do painel.
func (a *app) gerenciarUmFrio(p *pane) {
	if !paneFrioVisivel(p) {
		return
	}

	if p.descarregado {
		a.recarregarFrioAtivo(p)

		return
	}

	if p == a.right && a.siteAtualIA().chave == chaveChatgpt {
		return
	}

	if a.cuidarPaneAtivo(p) {
		return
	}

	if p.frio || p.frioManual {
		return
	}

	if a.marcarOciosoFrio(p) {
		return
	}

	a.congelarPorOciosidade(p)
}

// paneFrioVisivel diz se o painel entra na conta de frios.
func paneFrioVisivel(p *pane) bool {
	return p != nil && p.ready && p.wv != nil && p.visivel && p.rect.Width() > 0
}

// recarregarFrioAtivo recarrega o painel descarregado sob foco ou cursor.
func (a *app) recarregarFrioAtivo(p *pane) {
	if a.paneFocado(p) || a.cursorNoPainel(p) {
		a.recarregarPaneDescarregado(p)
	}
}

// cuidarPaneAtivo retoma painel com audio ou foco e zera ociosidade.
func (a *app) cuidarPaneAtivo(p *pane) bool {
	if p.tocandoAudio && !p.mudo {
		a.zerarOciosidadeFrio(p)

		return true
	}

	focado := p != a.left && a.paneFocado(p)
	if focado || a.cursorNoPainel(p) {
		a.zerarOciosidadeFrio(p)

		return true
	}

	return false
}

// zerarOciosidadeFrio limpa a ociosidade e retoma o painel frio automatico.
func (a *app) zerarOciosidadeFrio(p *pane) {
	p.ociosoDesde = time.Time{}
	if p.wv.memoryLow {
		p.wv.setMemoryLow(false)
	}

	if p.frio && !p.frioManual {
		a.retomarPaneGenerico(p)
	}
}

// marcarOciosoFrio carimba o inicio da ociosidade; true = so marcou.
func (a *app) marcarOciosoFrio(p *pane) bool {
	if p.ociosoDesde.IsZero() {
		p.ociosoDesde = time.Now()

		return true
	}

	if time.Since(p.ociosoDesde) >= 3*time.Second {
		p.wv.setMemoryLow(true)
	}

	return false
}

// congelarPorOciosidade congela o painel conforme o tempo ocioso.
func (a *app) congelarPorOciosidade(p *pane) {
	if p == a.left {
		if time.Since(p.ociosoDesde) >= 5*time.Minute {
			a.congelarAbaPrincipal(p)
		}

		return
	}

	if time.Since(p.ociosoDesde) >= 10*time.Second {
		a.congelarPaneGenerico(p)
	}
}

func (a *app) retomarTodosFrios() {
	for _, p := range a.panesGerenciados() {
		if p == nil {
			continue
		}

		if p.frio {
			p.frio = false
			p.frioManual = false
			p.frioPendente = false
			p.ociosoDesde = time.Time{}

			p.frioDesde = time.Time{}
			if p.ready && p.wv != nil {
				p.wv.SetFrozen(false)
				p.wv.setMemoryLow(false)
			}
		}
	}
}

func (a *app) descartarChatGPT() {
	if a.chatGerando {
		return
	}

	if a.right == nil || !a.right.ready || a.right.wv == nil {
		return
	}

	if a.siteAtualIA().chave != chaveChatgpt {
		return
	}

	a.chatFrioGen++
	a.chatFrio = false
	a.chatManual = false
	a.chatAtivo = false
	a.chatUsado = false
	a.chatFrioPendente = false
	a.envioGen++
	a.leituraGen++
	a.leituraAtiva = false
	a.envioAtivo = false
	a.chatPendente = nil
	a.anexosPendentes1 = nil
	a.right.frio = false
	a.right.frioManual = false
	a.right.tocandoAudio = false
	a.right.mudo = false
	pKillTimer.Call(a.hwnd, timerChatFrio)
	a.persist()
	a.right.wv.Close()
	a.right.wv = nil
	a.right.ready = false
	a.right.boundsSet = false
	a.right.visivel = false
	a.right.visivelSet = false
	a.right.emulFixa = false
	a.right.cssW, a.right.cssH = 0, 0
	a.chatGerando = false
	a.layout()
	a.pararServicosDeFundo()
	a.agendarLimpezaMemoria(true, esperaLimpezaPesadaMem)
}

func (a *app) resumirPane(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if p.descarregado {
		a.recarregarPaneDescarregado(p)

		return
	}

	p.frio = false
	p.frioManual = false
	p.wv.SetFrozen(false)
	p.wv.Resume()
}

func (a *app) retomarTudo() {
	if !a.tudoSuspenso {
		return
	}

	a.descarteGen++
	a.chatFrioGen++

	a.chatManual = false
	if a.chatFrio {
		a.chatFrio = false
		pKillTimer.Call(a.hwnd, timerChatFrio)
	}

	a.chatFrioPendente = false
	a.tudoSuspenso = false

	aplicarEcoQoS(false)

	if a.retomarCasoEspecial() {
		return
	}

	rightPronto := a.right != nil && a.right.ready
	if a.st.RightVisible && !rightPronto {
		a.chatGerando = false
		a.chatAtivo = false
		a.chatUsado = false
		a.chatFrio = false
		a.chatFrioPendente = false
		a.abrirPainelChat()
	}

	a.retomarAbaPrincipal()
	a.retomarPaineis(rightPronto)

	a.retomarTodosFrios()
	a.applyBounds()
}

// retomarCasoEspecial trata painel cheio, modo so-IA e YouTube; true = tratado.
func (a *app) retomarCasoEspecial() bool {
	if a.painelCheioAtivo() {
		a.applyBounds()

		return true
	}

	if a.soIA {
		a.resumirPane(a.right)
		a.applyBounds()

		return true
	}

	if a.ytAberto && !a.ytOculto {
		if a.ytPane != nil && a.ytPane.ready && a.ytPane.wv != nil {
			a.ytPane.wv.Resume()
		}

		a.applyBounds()

		return true
	}

	return false
}

// retomarAbaPrincipal resume a aba principal ou reativa a aba corrente.
func (a *app) retomarAbaPrincipal() {
	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.resumirPane(a.left)
	} else if a.ativa >= 0 && a.ativa < len(a.tabs) {
		a.ativarAba(a.ativa)
	}
}

// retomarPaineis resume chat, imagens e flutuante visiveis.
func (a *app) retomarPaineis(rightPronto bool) {
	a.retomarImagensAntesDoChat(rightPronto)
	a.retomarChatVisivel()
	a.retomarImagensVisivel()
	a.retomarFlutuanteVisivel()
}

// retomarImagensAntesDoChat resume as imagens quando o chat ja esta pronto.
func (a *app) retomarImagensAntesDoChat(rightPronto bool) {
	if rightPronto && a.st.RightVisible && a.imgPanel != nil && a.imgPanel.ready &&
		a.imgPanel.wv != nil && a.st.ImgVisible {
		a.resumirPane(a.imgPanel)
	}
}

// retomarChatVisivel resume o painel do chat quando visivel.
func (a *app) retomarChatVisivel() {
	if a.right != nil && a.right.ready && a.right.wv != nil && a.st.RightVisible {
		a.resumirPane(a.right)
	}
}

// retomarImagensVisivel resume o painel de imagens quando visivel.
func (a *app) retomarImagensVisivel() {
	if a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil && a.st.RightVisible && a.st.ImgVisible {
		a.resumirPane(a.imgPanel)
	}
}

// retomarFlutuanteVisivel reativa o painel flutuante de extensao.
func (a *app) retomarFlutuanteVisivel() {
	if a.extFloatOn && a.extFloat != nil && a.extFloat.ready && a.extFloat.wv != nil {
		a.extFloat.wv.Resume()
		a.extFloat.wv.SetVisible(true)
	}
}

func suspenderPane(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	setPaneVisible(p, false)

	if p.wv != nil {
		p.wv.setMemoryLow(true)
	}

	if !p.wv.IsSuspended() {
		p.wv.TrySuspend()
	}
}
