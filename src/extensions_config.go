//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// localidadeExtensao valida o DefaultLocale contra traversal.
func localidadeExtensao(m extManifest) string {
	locale := strings.TrimSpace(m.DefaultLocale)
	if locale == "" {
		return "en"
	}

	// Contencao: DefaultLocale vem do manifest da extensao; sem
	// validacao, "../" escaparia de dir/_locales.
	if strings.ContainsAny(locale, `/\`) || strings.Contains(locale, "..") {
		return "en"
	}

	return locale
}

// mensagemLocalizadaExtensao busca a mensagem no _locales.
func mensagemLocalizadaExtensao(dir, locale, chave string) string {
	caminho := filepath.Join(dir, "_locales", locale, "messages.json")
	// GOD: locale validado acima (sem separadores nem ".."); sem traversal por desenho.
	if b, err := os.ReadFile(caminho); err == nil { // #nosec G304
		var mensagens map[string]struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(b, &mensagens) == nil {
			return strings.TrimSpace(mensagens[chave].Message)
		}
	}

	return ""
}

// nomeLocalizadoExtensao resolve o __MSG_chave__ para o idioma.
func nomeLocalizadoExtensao(m extManifest, dir, nome string) string {
	chave := strings.TrimSuffix(strings.TrimPrefix(nome, "__MSG_"), "__")

	if msg := mensagemLocalizadaExtensao(dir, localidadeExtensao(m), chave); msg != "" {
		return msg
	}

	if base := filepath.Base(dir); base != "" && base != "." {
		return base
	}

	return nome
}

func nomeExibicaoExtensao(m extManifest, dir string) string {
	nome := strings.TrimSpace(m.Name)
	if strings.HasPrefix(nome, "__MSG_") && strings.HasSuffix(nome, "__") {
		return nomeLocalizadoExtensao(m, dir, nome)
	}

	return nome
}

func (a *app) loadExtensions() {
	if a.left == nil || !a.left.ready || a.left.wv == nil {
		return
	}

	a.loadExtensionsEm(a.left.wv, true)
}

func (a *app) pastasExtensoesValidas() []string {
	pastas, _ := os.ReadDir(extDir)

	var out []string

	for _, entrada := range pastas {
		if !entrada.IsDir() {
			continue
		}

		if _, err := os.Stat(filepath.Join(extDir, entrada.Name(), "manifest.json")); err != nil {
			continue
		}

		out = append(out, filepath.Join("extensions", entrada.Name()))
	}

	return out
}

func (a *app) loadExtensionsEm(w *WebView, atualizarUI bool) {
	if atualizarUI {
		if a.extIniciado {
			return
		}

		a.extIniciado = true
	}
	// So instala o que esta no estado (ou e do sistema): pasta largada no
	// disco nao volta sozinha. Descoberta continua pelo disco na adicao.
	var lista []string

	for _, rel := range a.pastasExtensoesValidas() {
		base := strings.ToLower(strings.TrimSpace(filepath.Base(strings.ReplaceAll(rel, "/", "\\"))))
		if base == "" || base == "." || base == ".." {
			continue
		}

		if ehExtensaoSistema(base) || a.extensaoLigada(base) {
			lista = append(lista, rel)
		}
	}

	a.loadExtensionsListaEm(w, lista, atualizarUI)

	if atualizarUI {
		a.reconciliarExtensoes()

		return
	}
}

// dirContidoExe resolve o rel dentro do exeDir ou rejeita.
func dirContidoExe(rel string) (string, bool) {
	dir := rel
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(exeDir, rel)
	}

	absExe, err1 := filepath.Abs(exeDir)

	absDir, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil || absDir != absExe && !strings.HasPrefix(absDir, absExe+string(filepath.Separator)) {
		return "", false
	}

	return dir, true
}

// popupDoManifesto extrai o popup do manifesto.
func popupDoManifesto(manifesto extManifest) string {
	if manifesto.Action != nil && manifesto.Action.DefaultPopup != "" {
		return manifesto.Action.DefaultPopup
	}

	if manifesto.BrowserAction != nil {
		return manifesto.BrowserAction.DefaultPopup
	}

	return ""
}

// instalarUmaExtensao carrega uma extensao da lista.
func (a *app) instalarUmaExtensao(w *WebView, rel string, atualizarUI bool) {
	dir, ok := dirContidoExe(rel)
	if !ok {
		return
	}

	caminhoManifesto := filepath.Join(dir, "manifest.json")
	// GOD: dir contido em exeDir (verificado acima); nome fixo.
	if _, err := os.Stat(caminhoManifesto); err != nil { // #nosec G304
		return
	}

	var manifesto extManifest
	// GOD: mp contido em exeDir (verificado acima); nome fixo.
	if b, err := os.ReadFile(caminhoManifesto); err == nil { // #nosec G304
		_ = json.Unmarshal(b, &manifesto)
	}

	a.tryAddExtensionEm(w, dir, popupDoManifesto(manifesto), atualizarUI)
}

func (a *app) loadExtensionsListaEm(w *WebView, lista []string, atualizarUI bool) {
	if w == nil {
		return
	}

	for _, rel := range lista {
		a.instalarUmaExtensao(w, rel, atualizarUI)
	}
}

// ehDuplicataUbo diz se a extensao e um uBlock antigo a remover.
func ehDuplicataUbo(id, atual, nome string) bool {
	return id != "" && id != atual && nome == "uBlock Origin"
}

// removerExtDuplicada remove a extensao antiga sem erro visivel.
func removerExtDuplicada(ext *ICoreWebView2BrowserExtension) {
	ext.Remove(func(err error) {
		if err != nil {
			return
		}
	})
}

// cuidarExtAntiga avalia e remove duplicatas do uBlock.
func cuidarExtAntiga(ext *ICoreWebView2BrowserExtension, atual string) {
	var idp, namep *uint16
	ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))
	ext.vtbl.GetName.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&namep)))

	if ehDuplicataUbo(takeString(idp), atual, takeString(namep)) {
		removerExtDuplicada(ext)
	}

	ext.Release()
}

// varrerExtensoesAntigas percorre a lista removendo duplicatas.
func varrerExtensoesAntigas(list *ICoreWebView2BrowserExtensionList, atual string) {
	var total uint32
	list.vtbl.GetCount.Call(uintptr(unsafe.Pointer(list)), uintptr(unsafe.Pointer(&total)))

	for i := range total {
		var ext *ICoreWebView2BrowserExtension

		hr, _, _ := list.vtbl.GetValueAtIndex.Call(
			uintptr(unsafe.Pointer(list)),
			uintptr(i),
			uintptr(unsafe.Pointer(&ext)),
		)
		if Failed(hr) || ext == nil {
			continue
		}

		cuidarExtAntiga(ext, atual)
	}
}

func (a *app) limparExtensoesAntigas(w *WebView) {
	if a == nil || a.quitting || w == nil || w.profile == nil {
		return
	}

	atual := w.extIDs[idUbo]

	if atual == "" {
		return
	}

	w.GetBrowserExtensions(func(list *ICoreWebView2BrowserExtensionList, err error) {
		if err != nil {
			return
		}

		if list == nil {
			return
		}
		defer list.Release()

		varrerExtensoesAntigas(list, atual)
	})
}

// cuidarFalhaAdicaoExt trata a extensao rejeitada pelo WebView.
func (a *app) cuidarFalhaAdicaoExt(dir string, err error, atualizarUI bool) {
	base := strings.ToLower(filepath.Base(dir))
	if a.extIDs != nil && a.extIDs[base] != "" {
		if a.extLoaded != nil {
			a.extLoaded[base] = true
		}

		return
	}

	a.extLog = "extensão rejeitada"
	if a.extLoaded != nil {
		a.extLoaded[base] = false
	}

	if atualizarUI {
		a.dizerErrof(txtFmtUboErro, err.Error())
		pInvalidateRect.Call(a.hwnd, 0, 0)
	}
}

// registrarExtensaoNova guarda IDs e popup da extensao aceita.
func (a *app) registrarExtensaoNova(
	w *WebView, ext *ICoreWebView2BrowserExtension, dir, popup, id string, atualizarUI bool,
) {
	if w.extIDs == nil {
		w.extIDs = map[string]string{}
	}

	base := strings.ToLower(filepath.Base(dir))

	w.extIDs[base] = id
	if w.extPopups == nil {
		w.extPopups = map[string]string{}
	}

	w.extPopups[base] = popup
	if atualizarUI && ext != nil && !a.extensaoLigada(base) {
		ext.Enable(false, nil)
	}

	if a.extLoaded == nil {
		a.extLoaded = map[string]bool{}
	}

	a.extLoaded[base] = true
}

// promoverExtensaoPrincipal marca a primeira extensao como principal.
func promoverExtensaoPrincipal(w *WebView, popup, id, _ string) {
	if w.extID != "" {
		return
	}

	w.extID = id

	w.extPopup = popup
	if w.extPopup == "" {
		w.extPopup = paginaPopup
	}
}

// publicarExtensaoNova atualiza a UI apos carregar a extensao.
func (a *app) publicarExtensaoNova(w *WebView, name string) {
	a.extIDs = w.extIDs
	a.extPopups = w.extPopups
	a.extReady = true
	a.status = name + " ativo"
	a.layout()
	a.extLimpezaGen++
	gen := a.extLimpezaGen

	const atrasoLimpezaExtMs = 1500
	time.AfterFunc(atrasoLimpezaExtMs*time.Millisecond, func() {
		a.depois(func() {
			if gen != a.extLimpezaGen {
				return
			}

			a.limparExtensoesAntigas(w)
		})
	})
}

func (a *app) tryAddExtensionEm(w *WebView, dir, popup string, atualizarUI bool) {
	if w == nil {
		return
	}

	w.AddBrowserExtension(dir, func(ext *ICoreWebView2BrowserExtension, id, name string, err error) {
		if ext != nil {
			defer ext.Release()
		}

		if err != nil {
			a.cuidarFalhaAdicaoExt(dir, err, atualizarUI)

			return
		}

		a.registrarExtensaoNova(w, ext, dir, popup, id, atualizarUI)
		a.extLog = name + " carregada"

		promoverExtensaoPrincipal(w, popup, id, name)

		if atualizarUI {
			a.publicarExtensaoNova(w, name)
		}
	})
}

// perfilProntoParaReconciliar valida app, base e painel esquerdo.
func (a *app) perfilProntoParaReconciliar(base string) (string, bool) {
	if a == nil || base == "" {
		return "", false
	}

	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return "", false
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		return "", false
	}

	return orig, true
}

func (a *app) reconciliarExtDoPerfil(base string, cb func()) {
	done := func() {
		if cb != nil {
			a.depois(func() { cb() })
		}
	}

	orig, ok := a.perfilProntoParaReconciliar(base)
	if !ok {
		done()

		return
	}

	dir := filepath.Join(extDir, orig)
	quero, popup := alvoReconciliacaoExtensao(dir)

	if quero == "" {
		done()

		return
	}

	a.left.wv.GetBrowserExtensions(func(list *ICoreWebView2BrowserExtensionList, err error) {
		if err != nil || list == nil {
			done()

			return
		}
		defer list.Release()

		a.procurarExtNoPerfil(list, base, quero, popup, done)
	})
}

func (a *app) limparHandlersContexto() {
	if a != nil && len(a.ctxHandlers) > 0 {
		a.ctxHandlers = nil
	}
}

// alvoReconciliacaoExtensao le o manifesto e devolve nome e popup.
func alvoReconciliacaoExtensao(dir string) (string, string) {
	// GOD: orig e pasta real no disco (pastaRealExtensao); nome fixo.
	if b, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil { // #nosec G304
		var m extManifest
		if json.Unmarshal(b, &m) == nil {
			quero := strings.ToLower(strings.TrimSpace(nomeExibicaoExtensao(m, dir)))

			return quero, popupDoManifesto(m)
		}
	}

	return "", ""
}

// publicarReconciliacaoExtensao guarda o ID achado no perfil.
func (a *app) publicarReconciliacaoExtensao(base, achado, cam string, done func()) {
	a.depois(func() {
		if a.extIDs == nil {
			a.extIDs = map[string]string{}
		}

		a.extIDs[base] = achado
		if a.left != nil && a.left.wv != nil {
			if a.left.wv.extIDs == nil {
				a.left.wv.extIDs = map[string]string{}
			}

			a.left.wv.extIDs[base] = achado
		}

		if cam != "" {
			if a.extPopups == nil {
				a.extPopups = map[string]string{}
			}

			a.extPopups[base] = cam
		}

		if a.extLoaded == nil {
			a.extLoaded = map[string]bool{}
		}

		a.extLoaded[base] = true
		a.extReady = true

		done()
	})
}

// procurarExtNoPerfil varre a lista atras do nome reconciliado.
func (a *app) procurarExtNoPerfil(list *ICoreWebView2BrowserExtensionList, base, quero, popup string, done func()) {
	var total uint32
	list.vtbl.GetCount.Call(uintptr(unsafe.Pointer(list)), uintptr(unsafe.Pointer(&total)))

	for i := range total {
		var ext *ICoreWebView2BrowserExtension

		hr, _, _ := list.vtbl.GetValueAtIndex.Call(
			uintptr(unsafe.Pointer(list)),
			uintptr(i),
			uintptr(unsafe.Pointer(&ext)),
		)
		if Failed(hr) || ext == nil {
			continue
		}

		var idp, namep *uint16
		ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))
		ext.vtbl.GetName.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&namep)))

		id, nome := takeString(idp), takeString(namep)
		if !strings.EqualFold(nome, quero) {
			ext.Release()

			continue
		}

		ext.Release()

		a.publicarReconciliacaoExtensao(base, id, popup, done)

		return
	}

	done()
}

func (a *app) persist() {
	if a == nil {
		return
	}

	if a.persistTimer != nil {
		a.persistTimer.Stop()
	}

	a.persistPendente = true

	const atrasoPersistenciaMs = 500

	a.persistTimer = time.AfterFunc(atrasoPersistenciaMs*time.Millisecond, func() {
		a.depois(func() {
			if !a.persistPendente {
				return
			}

			a.persistPendente = false
			a.persistTimer = nil
			a.persistAgora()
		})
	})
}

func (a *app) tamanhoLogico(pixels int32) int32 {
	const dpiPadraoTamanhoLogico = 96

	dpi := a.DPI()
	if dpi <= 0 {
		dpi = dpiPadraoTamanhoLogico
	}

	return pixels * dpiPadraoTamanhoLogico / int32(dpi)
}

func (a *app) persistAgora() {
	const (
		minLarguraJanelaValida = 400
		minAlturaJanelaValida  = 300
	)

	var area rectT
	if r, _, _ := pGetWindowRect.Call(a.hwnd, uintptr(unsafe.Pointer(&area))); r != 0 {
		if area.Right-area.Left > minLarguraJanelaValida && area.Bottom-area.Top > minAlturaJanelaValida {
			a.st.WinX = area.Left
			a.st.WinY = area.Top
			a.st.WinW = a.tamanhoLogico(area.Right - area.Left)
			a.st.WinH = a.tamanhoLogico(area.Bottom - area.Top)
		}
	}

	if a.right != nil && a.right.ready {
		if u := a.right.wv.Source(); urlUtilizavel(u) && a.siteAtualIA().aceitaURL(u) {
			a.st.RightURL = u
		}
	}

	saveState(a.st)
}

// guardarGeometriaSaida salva a posicao da janela ao sair.
func (a *app) guardarGeometriaSaida(minimizada bool) {
	var wp windowPlacementT

	wp.length = uint32(unsafe.Sizeof(wp))
	if r, _, _ := pGetWindowPlacement.Call(a.hwnd, uintptr(unsafe.Pointer(&wp))); r != 0 {
		area := wp.rcNormalPosition
		if area.Right-area.Left > 400 && area.Bottom-area.Top > 300 {
			a.st.WinX = area.Left
			a.st.WinY = area.Top
			a.st.WinW = a.tamanhoLogico(area.Right - area.Left)
			a.st.WinH = a.tamanhoLogico(area.Bottom - area.Top)
		}

		return
	}

	if z, _, _ := pIsZoomed.Call(a.hwnd); z == 0 && !minimizada {
		var area rectT
		if r, _, _ := pGetWindowRect.Call(a.hwnd, uintptr(unsafe.Pointer(&area))); r != 0 {
			if area.Right-area.Left > 400 && area.Bottom-area.Top > 300 {
				a.st.WinX = area.Left
				a.st.WinY = area.Top
				a.st.WinW = a.tamanhoLogico(area.Right - area.Left)
				a.st.WinH = a.tamanhoLogico(area.Bottom - area.Top)
			}
		}
	}
}

// guardarEstadoSaida salva maximizado e URL da IA ao sair.
func (a *app) guardarEstadoSaida(minimizada bool) {
	if !minimizada {
		if z, _, _ := pIsZoomed.Call(a.hwnd); z != 0 {
			a.st.Maximized = true
		} else {
			a.st.Maximized = false
		}
	}

	if a.right != nil && a.right.ready {
		if u := a.right.wv.Source(); urlUtilizavel(u) && a.siteAtualIA().aceitaURL(u) {
			a.st.RightURL = u
		}
	}

	saveState(a.st)
}

// interromperDownloadsSaida marca downloads ativos como interrompidos.
func (a *app) interromperDownloadsSaida() {
	a.downloadOpsMu.Lock()
	for _, descarga := range a.downloads {
		// Ativos nao sobrevivem ao processo: marca como interrompidos para
		// tentar de novo na proxima sessao.
		if descarga != nil && descarga.op != nil {
			descarga.Estado = 1
			descarga.PodeRetomar = false
			descarga.Cancelado = false
			descarga.Reiniciado = false
			descarga.Velocidade = 0
			descarga.ETA = -1
		}
	}

	for chave, descarga := range a.downloadOps {
		if descarga != nil && descarga.op != nil {
			// Nao marca Cancelado aqui: os ativos ja foram registrados
			// como interrompidos acima para tentar de novo.
			descarga.op.vtbl.Cancel.Call(uintptr(unsafe.Pointer(descarga.op)))
			descarga.op.vtbl.Release.Call(uintptr(unsafe.Pointer(descarga.op)))
			descarga.op = nil
		}

		delete(a.downloadOps, chave)
	}
	a.downloadOpsMu.Unlock()
	a.salvarRegistroDownloads()
}

// fecharPanesSaida fecha todos os WebViews ao sair.
func (a *app) fecharPanesSaida() {
	for _, t := range a.tabs {
		if t.pane != nil && t.pane.ready {
			t.pane.wv.Close()
			t.pane.ready = false
		}
	}

	for _, p := range []*pane{a.popup, a.right, a.imgPanel, a.ytPane} {
		if p != nil && p.ready {
			p.wv.Close()
			p.ready = false
		}
	}

	if a.left != nil && a.left.ready {
		a.left.wv.Close()
		a.left.ready = false
	}

	a.fecharPainelFlutuante()

	if a.env != nil {
		a.env.Close()
		a.env = nil
	}

	pDestroyWindow.Call(a.hwnd)
}

// limparPerfilSaida limpa historico e perfil privado ao sair.
func limparPerfilSaida() {
	const pausaLimpezaPerfilMs = 250

	perfilAtual := filepath.Join(dataDir, "profile")
	if profileOverride != "" {
		perfilAtual = filepath.Join(dataDir, profileOverride)
	}

	for range 8 {
		if removerHistoricoPerfil(perfilAtual) == 0 {
			break
		}

		time.Sleep(pausaLimpezaPerfilMs * time.Millisecond)
	}

	if perfilPrivado && profileOverride != "" {
		alvo := filepath.Join(dataDir, profileOverride)

		for range 12 {
			if err := os.RemoveAll(alvo); err == nil {
				break
			}

			time.Sleep(pausaLimpezaPerfilMs * time.Millisecond)
		}

		_ = os.Remove(statePath())
		_ = os.Remove(statePath() + ".bak")
	}
}

func (a *app) saveAndQuit() {
	if a.quitting {
		return
	}

	a.quitting = true

	if a.persistTimer != nil {
		a.persistTimer.Stop()
		a.persistTimer = nil
	}

	a.persistPendente = false
	if a.scrollLimpezaTimer != nil {
		a.scrollLimpezaTimer.Stop()
		a.scrollLimpezaTimer = nil
	}

	tradCurlCancela()

	minimizada := false
	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		minimizada = true
	}

	a.guardarGeometriaSaida(minimizada)
	a.guardarEstadoSaida(minimizada)
	fecharServidorImagens()

	a.interromperDownloadsSaida()
	a.fecharPanesSaida()
	limparPerfilSaida()
}

// lerConfigOverdrive le e valida o envelope do JSON.
func lerConfigOverdrive(caminho string) (overdriveConfigJSON, error) {
	var cfg overdriveConfigJSON

	// GOD: chamado uma vez no boot com config/overdrive.json do exeDir; sem entrada externa.
	dados, err := os.ReadFile(caminho) // #nosec G304
	if err != nil {
		return cfg, erroTextoF("não consegui ler %s: %w", caminho, err)
	}

	if len(dados) == 0 || len(dados) > 2<<20 {
		return cfg, erroTextoF("%s esta vazio ou excede 2 MB", caminho)
	}

	if err := json.Unmarshal(dados, &cfg); err != nil {
		return cfg, erroTextoF("JSON invalido em %s: %w", caminho, err)
	}

	if strings.TrimSpace(cfg.BrowserName) == "" {
		return cfg, erroTexto("browserName não pode ficar vazio")
	}

	if len(cfg.Providers) == 0 {
		return cfg, erroTexto("providers precisa conter pelo menos uma IA")
	}

	if len(cfg.InjectedScript) == 0 {
		return cfg, erroTexto("injectedScripts precisa conter os scripts do navegador")
	}

	return cfg, nil
}

// montarProvedoresConfig valida cada provedor e soma a lista.
func montarProvedoresConfig(cfg overdriveConfigJSON) ([]siteIA, map[string]bool, error) {
	provedores := make([]siteIA, 0, len(cfg.Providers))

	vistos := make(map[string]bool, len(cfg.Providers))
	for _, p := range cfg.Providers {
		s, err := validarProvedorConfig(p, vistos)
		if err != nil {
			return nil, nil, err
		}

		provedores = append(provedores, s)
	}

	if !vistos[chaveDeepseek] {
		return nil, nil, erroTexto("providers precisa manter a IA deepseek (padrão do painel)")
	}

	return provedores, vistos, nil
}

func carregarConfigOverdrive(caminho string) error {
	cfg, err := lerConfigOverdrive(caminho)
	if err != nil {
		return err
	}

	provedores, _, err := montarProvedoresConfig(cfg)
	if err != nil {
		return err
	}

	if err := validarScriptsConfig(cfg); err != nil {
		return err
	}

	aplicarConfigOverdrive(cfg, provedores)

	return nil
}

// validarProvedorConfig valida um provedor e soma a lista.
func validarProvedorConfig(provedor configIAJSON, vistos map[string]bool) (siteIA, error) {
	provedor.Key = strings.ToLower(strings.TrimSpace(provedor.Key))
	provedor.Name = strings.TrimSpace(provedor.Name)
	provedor.URL = strings.TrimSpace(provedor.URL)

	if !chaveValida.MatchString(provedor.Key) || vistos[provedor.Key] {
		return siteIA{}, erroTextoF("chave de IA vazia, invalida ou duplicada: %q", provedor.Key)
	}

	if provedor.Name == "" ||
		(!strings.HasPrefix(provedor.URL, "https://") && !strings.HasPrefix(provedor.URL, "http://")) ||
		len(provedor.Hosts) == 0 {
		return siteIA{}, erroTextoF("a IA %q precisa de name, URL HTTP(S) e hosts", provedor.Key)
	}

	vistos[provedor.Key] = true

	return siteIA{
		chave: provedor.Key, nome: provedor.Name, url: provedor.URL, hosts: provedor.Hosts,
		campo: provedor.FieldSelector, enviar: provedor.SendButtonSelector,
		scripts: append([]string(nil), provedor.Scripts...),
	}, nil
}

// validarScriptsConfig confere scripts referenciados e obrigatorios.
func validarScriptsConfig(cfg overdriveConfigJSON) error {
	for nome, script := range cfg.InjectedScript {
		if nome == "" || strings.TrimSpace(script) == "" {
			return erroTexto("injectedScripts contem uma chave ou script vazio")
		}
	}

	for _, p := range cfg.Providers {
		for _, nome := range p.Scripts {
			if _, ok := cfg.InjectedScript[nome]; !ok {
				return erroTextoF("a IA %q referencia o script ausente %q", p.Key, nome)
			}
		}
	}

	for nome, obrigatorio := range map[string]bool{
		"navigation": true, "language": true, "translation": true,
		"login": true, "chatImages": true, "chatLinks": true,
	} {
		if obrigatorio && strings.TrimSpace(cfg.InjectedScript[nome]) == "" {
			return erroTextoF("injectedScripts.%s está ausente", nome)
		}
	}

	return nil
}

// aplicarConfigOverdrive publica a configuracao validada.
func aplicarConfigOverdrive(cfg overdriveConfigJSON, provedores []siteIA) {
	appTitle = strings.TrimSpace(cfg.BrowserName)
	startPageHTML = strings.ReplaceAll(startPageHTML, "Overdrive", appTitle)
	sitesIA = provedores
	pageScripts = cfg.InjectedScript
	navHelperScript = pageScripts["navigation"]
	idiomaJS = pageScripts["language"]

	aplicarJS = pageScripts["translation"]
	if strings.TrimSpace(traducaoLiteJS) != "" {
		aplicarJS = traducaoLiteJS
	}

	anexarImagensJS = pageScripts["chatImages"]
	linksChatJS = pageScripts["chatLinks"]

	if ds := sitePorChave(chaveDeepseek); ds.chave != "" {
		urlChat = ds.url
	}
}

func expandirScript(nome string, valores map[string]string) string {
	script := pageScripts[nome]
	if script == "" {
		return ""
	}

	const itensPorValorScript = 2

	chaves := make([]string, 0, len(valores)*itensPorValorScript)
	for k, v := range valores {
		chaves = append(chaves, "__"+k+"__", v)
	}

	return strings.NewReplacer(chaves...).Replace(script)
}

func scriptsDaIA(s siteIA) []string {
	saida := make([]string, 0, len(s.scripts))
	for _, nome := range s.scripts {
		if js := expandirScript(nome, map[string]string{
			"IA_NAME": jsonDeString(s.nome), "IA_KEY": jsonDeString(s.chave),
		}); js != "" {
			saida = append(saida, js)
		}
	}

	return saida
}

func adicionarInitScripts(w interface{ AddInitScript(js string) }, scripts []string) {
	for _, js := range scripts {
		if strings.TrimSpace(js) != "" {
			w.AddInitScript(js)
		}
	}
}

// migrarSenhasLogin move senhas do JSON para o cofre.
func migrarSenhasLogin(login *dadosLogin) bool {
	migrou := false

	if login.Senha != "" {
		if err := credEscrever(alvoCredPadrao, login.Senha); err == nil {
			login.Senha = ""
			migrou = true
		}
	}

	for i := range login.Contas {
		if login.Contas[i].Senha != "" {
			if err := credEscrever(alvoCredConta(i), login.Contas[i].Senha); err == nil {
				login.Contas[i].Senha = ""
				migrou = true
			}
		}
	}

	return migrou
}

// regravarLoginSemSenha persiste o login apos migrar para o cofre.
func regravarLoginSemSenha(caminho string, d dadosLogin) {
	// Best-effort: o cofre ja tem as senhas; se a gravacao falhar o
	// arquivo antigo continua valido com as senhas nele.
	if saida, err := json.MarshalIndent(d, "", "  "); err == nil {
		_ = os.WriteFile(caminho, saida, 0o600)
	}
}

// recarregarSenhasLogin le as senhas do cofre para a sessao.
func recarregarSenhasLogin(login *dadosLogin) {
	if s, ok := credLer(alvoCredPadrao); ok {
		login.Senha = s
	}

	for i := range login.Contas {
		if s, ok := credLer(alvoCredConta(i)); ok {
			login.Contas[i].Senha = s
		}
	}
}

// escolherContaLogin ajusta o email/senha para a conta ativa.
func escolherContaLogin(login *dadosLogin) {
	if len(login.Contas) == 0 {
		return
	}

	if login.Atual < 0 || login.Atual >= len(login.Contas) {
		login.Atual = 0
	}

	escolhida := login.Contas[login.Atual]
	if strings.TrimSpace(escolhida.Email) != "" {
		login.Email = escolhida.Email
		login.Senha = escolhida.Senha
	}
}

func carregarLogin() {
	caminho := filepath.Join(dataDir, "login.json")
	// GOD: caminho interno (dataDir); sem entrada externa.
	dados, err := os.ReadFile(caminho) // #nosec G304
	if err != nil {
		return
	}

	var login dadosLogin
	if err := json.Unmarshal(semBOM(dados), &login); err != nil {
		return
	}

	if migrarSenhasLogin(&login) {
		regravarLoginSemSenha(caminho, login)
	}

	recarregarSenhasLogin(&login)

	if strings.TrimSpace(login.Email) == "" || login.Senha == "" {
		return
	}

	escolherContaLogin(&login)

	loginAtual = login
}

func temLogin() bool {
	return strings.TrimSpace(loginAtual.Email) != "" && loginAtual.Senha != ""
}

func loginAutoLigado() bool {
	return !perfilPrivado && temLogin() && loginAtual.Auto
}

func loginScript() string {
	if !loginAutoLigado() {
		return "true"
	}

	return expandirScript("login", map[string]string{
		"EMAIL": jsonDeString(loginAtual.Email),
		"SENHA": jsonDeString(loginAtual.Senha),
	})
}

func contaAtiva() (contaLogin, bool) {
	if len(loginAtual.Contas) > 0 {
		i := loginAtual.Atual
		if i < 0 || i >= len(loginAtual.Contas) {
			i = 0
		}

		return loginAtual.Contas[i], true
	}

	if strings.TrimSpace(loginAtual.Email) != "" {
		return contaLogin{Apelido: loginAtual.Email, Email: loginAtual.Email, Senha: loginAtual.Senha}, true
	}

	return contaLogin{}, false
}

func rotuloContaAtiva() string {
	conta, ok := contaAtiva()
	if !ok {
		return "nenhuma conta cadastrada"
	}

	if strings.TrimSpace(conta.Apelido) != "" && conta.Apelido != conta.Email {
		return fmt.Sprintf(txtFmtRotuloConta, conta.Apelido, conta.Email)
	}

	return conta.Email
}

func proximaConta() int {
	total := len(loginAtual.Contas)
	if total == 0 {
		return 0
	}

	i := loginAtual.Atual
	if i < 0 || i >= total {
		i = 0
	}

	return (i + 1) % total
}

func gravarContaAtiva(indice int) error {
	caminho := filepath.Join(dataDir, "login.json")
	// GOD: caminho interno (dataDir); sem entrada externa.
	dados, err := os.ReadFile(caminho) // #nosec G304
	if err != nil {
		return erroTextoF("ler estado login: %w", err)
	}

	var bruto map[string]any
	if err := json.Unmarshal(dados, &bruto); err != nil {
		return erroTextoF("login invalido: %w", err)
	}

	bruto["atual"] = indice
	if indice >= 0 && indice < len(loginAtual.Contas) {
		bruto["email"] = loginAtual.Contas[indice].Email
	}

	delete(bruto, "senha")

	if lista, ok := bruto["contas"].([]any); ok {
		for _, item := range lista {
			if m, ok := item.(map[string]any); ok {
				delete(m, "senha")
			}
		}
	}

	saida, err := json.MarshalIndent(bruto, "", "  ")
	if err != nil {
		return erroTextoF("serializar login: %w", err)
	}

	if err := os.WriteFile(caminho, saida, 0o600); err != nil {
		return erroTextoF("gravar login: %w", err)
	}

	return nil
}

// painelContaPronto valida o painel de chat para trocar de conta.
func (a *app) painelContaPronto() (*pane, bool) {
	p := a.right
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtChatNaoPronto)

		return nil, false
	}

	if len(loginAtual.Contas) == 0 {
		a.dizerf(txtSemContaLogin)

		return nil, false
	}

	return p, true
}

// prenderIndiceConta limita o indice a conta valida.
func prenderIndiceConta(indice int) int {
	if indice < 0 || indice >= len(loginAtual.Contas) {
		return 0
	}

	return indice
}

// renavegarConta recarrega o chat com o login da conta nova.
func (a *app) renavegarConta(p *pane) {
	a.apagarCookiesConta(p)
	a.depois(func() {
		if a.right != nil && a.right.ready && a.right.wv != nil {
			a.right.wv.AddInitScript(loginScript())
			a.right.wv.Navigate(urlChat + "sign_in")
		}
	})
}

func (a *app) trocarConta(indice int) {
	p, ok := a.painelContaPronto()
	if !ok {
		return
	}

	indice = prenderIndiceConta(indice)

	if err := gravarContaAtiva(indice); err != nil {
		a.dizerErrof("%s", txtNaoTrocouConta)
	}

	loginAtual.Atual = indice
	loginAtual.Email = loginAtual.Contas[indice].Email
	loginAtual.Senha = loginAtual.Contas[indice].Senha
	rotulo := rotuloContaAtiva()
	a.dizerf(txtFmtTrocaConta, rotulo)

	a.depois(func() {
		a.limparStorageConta()
	})

	go func() { a.renavegarConta(p) }()
}

// limparStorageConta limpa o storage do painel para a proxima conta.
func (a *app) limparStorageConta() {
	if a.right != nil && a.right.ready && a.right.wv != nil {
		jsLimpaStorage := "(function(){try{localStorage.clear();sessionStorage.clear();return 'ok';}" +
			"catch(e){return 'erro';}})()"
		a.right.wv.EvalJS(jsLimpaStorage,
			func(error, string) {})
	}
}

// apagarCookiesConta apaga os cookies do provedor da conta atual via CDP.
func (a *app) apagarCookiesConta(p *pane) {
	cru, err := a.chamarCDP(p, "Network.getCookies", fmt.Sprintf(`{"urls":[%q,"https://deepseek.com/"]}`, urlChat))
	if err != nil {
		return
	}

	var r cdpCookiesResp
	if json.Unmarshal([]byte(cru), &r) != nil {
		return
	}

	for _, c := range r.Cookies {
		if !strings.Contains(c.Domain, chaveDeepseek) {
			continue
		}

		params := fmt.Sprintf(`{"name":%q,"url":%q}`, c.Name, urlChat)
		_, _ = a.chamarCDP(p, "Network.deleteCookies", params)
	}
}

func subContas() []menuItem {
	var itens []menuItem

	const maxContasMenu = 4
	for indice, conta := range loginAtual.Contas {
		if indice >= maxContasMenu {
			break
		}

		marca := "   "
		if indice == loginAtual.Atual {
			marca = "✓  "
		}

		rotulo := conta.Apelido
		if strings.TrimSpace(rotulo) == "" {
			rotulo = conta.Email
		}

		itens = append(itens, menuItem{
			kind:  miItem,
			label: marca + rotulo,
			id:    "conta" + strconv.Itoa(indice),
		})
	}

	if len(itens) == 0 {
		itens = append(itens, menuItem{kind: miItem, label: txtNenhumaConta, id: idContaDeepseek})
	}

	return itens
}

func defaultState() state {
	const (
		larguraPadraoJanela = 1400
		alturaPadraoJanela  = 900
		divisaoPadraoPainel = 0.66
		zoomPadraoNavegador = 0.9
		zoomPadraoPainelIA  = 100
		versaoEstadoAtual   = 2
	)

	return state{
		WinW:             larguraPadraoJanela,
		WinH:             alturaPadraoJanela,
		Split:            divisaoPadraoPainel,
		ImgSplit:         imgSplitPadrao,
		LeftPanelVisible: true,
		LeftPanelSplit:   leftPanelSplitPadrao,
		RightVisible:     true,
		ImgVisible:       true,
		Zoom:             zoomPadraoNavegador,
		ZoomPainelIA:     zoomPadraoPainelIA,
		ForceDark:        true,
		MotorBusca:       chaveGoogle,
		RightURL:         urlChat,
		Extensions:       []string{"extensions\\uBO"},
		ExtFixadas:       []string{idUbo},
		ExtFixarNovas:    true,
		ExtraArgs:        "",
		EconomiaMax:      true,
		StateVersion:     versaoEstadoAtual,
	}
}

func statePath() string {
	if profileOverride != "" {
		return filepath.Join(dataDir, "state-"+profileOverride+".json")
	}

	return filepath.Join(dataDir, "state.json")
}

// lerEstadoDisco carrega state.json com queda para o bak.
func lerEstadoDisco() state {
	estado := defaultState()

	dados, err := os.ReadFile(statePath())
	if err != nil {
		return estado
	}

	dados = bytes.TrimPrefix(dados, []byte{0xEF, 0xBB, 0xBF})

	estado.StateVersion = 0
	if json.Unmarshal(dados, &estado) != nil {
		estado = defaultState()

		if bak, err2 := os.ReadFile(statePath() + ".bak"); err2 == nil {
			bak = bytes.TrimPrefix(bak, []byte{0xEF, 0xBB, 0xBF})
			_ = json.Unmarshal(bak, &estado)
		}
	}

	return estado
}

// normalizarZoomEstado prende o zoom aos limites validos.
func normalizarZoomEstado(estado *state) {
	if estado.Zoom <= 0.3 || estado.Zoom > 2 {
		estado.Zoom = 0.9
	}

	if estado.StateVersion < 1 {
		estado.StateVersion = 1
		estado.EconomiaMax = true
	}

	const versaoEstadoAtual = 2
	if estado.StateVersion < versaoEstadoAtual {
		estado.StateVersion = versaoEstadoAtual
	}

	if estado.ZoomPainelIA == 1 {
		estado.ZoomPainelIA = 100
	}

	if estado.ZoomPainelIA < 85 || estado.ZoomPainelIA > 115 {
		estado.ZoomPainelIA = 100
	}
}

// normalizarMotorEstado valida motor de busca e splits.
func normalizarMotorEstado(estado *state) {
	if motorPorChave(estado.MotorBusca).chave != estado.MotorBusca {
		estado.MotorBusca = chaveGoogle
	}

	if estado.Split < splitMin || estado.Split > splitMax {
		estado.Split = 0.66
	}

	if estado.ImgSplit < imgSplitMin || estado.ImgSplit > imgSplitMax {
		estado.ImgSplit = imgSplitPadrao
	}

	if estado.LeftPanelSplit < leftPanelSplitMin || estado.LeftPanelSplit > leftPanelSplitMax {
		estado.LeftPanelSplit = leftPanelSplitPadrao
	}
}

// normalizarSiteEstado garante uma IA valida no estado.
func normalizarSiteEstado(estado *state) {
	for _, s := range sitesIA {
		if s.chave == estado.RightSite {
			return
		}
	}

	estado.RightSite = sitePorChave(chaveDeepseek).chave
	estado.RightURL = sitePorChave(estado.RightSite).url
}

func loadState() state {
	st := lerEstadoDisco()
	normalizarZoomEstado(&st)
	normalizarMotorEstado(&st)
	normalizarSiteEstado(&st)

	return st
}

func saveState(estado state) {
	dados, err := json.Marshal(estado)
	if err != nil || bytes.Equal(dados, stateSaved) {
		return
	}

	caminho := statePath()

	// GOD: caminho interno do estado (dataDir, sem entrada externa).
	atual, err := os.ReadFile(caminho) // #nosec G304
	if err == nil && len(atual) > 0 {
		// GOD: caminho interno do estado (dataDir, sem entrada externa).
		_ = os.WriteFile(caminho+".bak", atual, 0o600) // #nosec G703
	}

	tmp := caminho + ".tmp"
	if err := os.WriteFile(tmp, dados, 0o600); err != nil {
		return
	}

	if err := os.Rename(tmp, caminho); err != nil {
		_ = os.Remove(tmp)

		return
	}

	stateSaved = append(stateSaved[:0], dados...)
}

func (a *app) extensaoLigada(pasta string) bool {
	base, ok := baseExtensaoValida(pasta)
	if !ok {
		return false
	}

	for _, e := range a.st.Extensions {
		if baseListaExtensao(e) == base {
			return true
		}
	}

	return false
}

func (a *app) alternarExtensao(pasta string, ligada bool) {
	base, ok := baseExtensaoValida(pasta)
	if !ok {
		return
	}

	if ligada && !ehExtensaoSistema(base) && a.pastaRealExtensao(base) == "" {
		a.dizerErrof("%s", txtExtNaoAchada)

		return
	}

	var novo []string

	for _, e := range a.st.Extensions {
		if baseListaExtensao(e) == base {
			continue
		}

		novo = append(novo, e)
	}

	if ligada {
		nomePasta := a.pastaRealExtensao(base)
		if nomePasta == "" {
			nomePasta = base
		}

		novo = append(novo, "extensions\\"+nomePasta)
	}

	a.st.Extensions = novo
	a.persist()

	if a.extDominioEstado != nil {
		delete(a.extDominioEstado, strings.ToLower(strings.TrimSpace(pasta)))
	}

	a.definirExtensaoLigada(pasta, ligada)
}

// baseExtensaoValida normaliza a pasta da extensao.
func baseExtensaoValida(pasta string) (string, bool) {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(pasta, "/", "\\")))
	if base == "" || base == "." || base == ".." {
		return "", false
	}

	return base, true
}

// baseListaExtensao normaliza um item da lista de extensoes para comparar.
func baseListaExtensao(item string) string {
	base, ok := baseExtensaoValida(filepath.Base(strings.ReplaceAll(item, "/", "\\")))
	if !ok {
		return ""
	}

	return base
}

// reinstalarExtensaoAusente tenta adicionar a extensao sem ID e reagenda.
func (a *app) reinstalarExtensaoAusente(pasta, base string, ligada bool) {
	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return
	}

	dir := filepath.Join(extDir, orig)

	var manifesto extManifest
	// GOD: orig e pasta real no disco (pastaRealExtensao); nome fixo.
	if b, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil { // #nosec G304
		_ = json.Unmarshal(b, &manifesto)
	}

	a.tryAddExtensionEm(a.left.wv, dir, popupDoManifesto(manifesto), true)

	const atrasoReinstalacaoExtSegundos = 2
	time.AfterFunc(atrasoReinstalacaoExtSegundos*time.Second, func() {
		a.depois(func() { a.definirExtensaoLigada(pasta, ligada) })
	})
}

// habilitarExtEncontrada liga/desliga a extensao achada pelo ID.
func (a *app) habilitarExtEncontrada(ext *ICoreWebView2BrowserExtension, base string, ligada bool) {
	ext.Enable(ligada, func(err error) {
		ext.Release()
		a.depois(func() { a.anunciarTrocaExtensao(base, ligada, err) })
	})
}

// anunciarTrocaExtensao relata o resultado e fecha flutuante se preciso.
func (a *app) anunciarTrocaExtensao(base string, ligada bool, err error) {
	if err != nil {
		a.dizerErrof(txtFmtNaoMudei, base)

		return
	}

	if !ligada && a.extFloatOn && a.pastaDoFloat() == base {
		a.fecharPainelFlutuante()
	}

	if ligada {
		a.dizerOkf(txtFmtLigada, base)
	} else {
		a.dizerOkf(txtFmtDesligada, base)
	}
}

// procurarExtParaHabilitar varre a lista atras do ID e habilita.
func (a *app) procurarExtParaHabilitar(list *ICoreWebView2BrowserExtensionList, id, base string, ligada bool) {
	var total uint32
	list.vtbl.GetCount.Call(uintptr(unsafe.Pointer(list)), uintptr(unsafe.Pointer(&total)))

	for i := range total {
		var ext *ICoreWebView2BrowserExtension

		hr, _, _ := list.vtbl.GetValueAtIndex.Call(
			uintptr(unsafe.Pointer(list)),
			uintptr(i),
			uintptr(unsafe.Pointer(&ext)),
		)
		if Failed(hr) || ext == nil {
			continue
		}

		var idp *uint16
		ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))

		if takeString(idp) != id {
			ext.Release()

			continue
		}

		a.habilitarExtEncontrada(ext, base, ligada)

		return
	}
}

func (a *app) definirExtensaoLigada(pasta string, ligada bool) {
	base, ok := baseExtensaoValida(pasta)
	if !ok {
		return
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		a.dizerf(txtMotorFora)

		return
	}

	id := ""
	if a.extIDs != nil {
		id = a.extIDs[base]
	}

	if id == "" {
		a.reinstalarExtensaoAusente(pasta, base, ligada)

		return
	}

	a.left.wv.GetBrowserExtensions(func(list *ICoreWebView2BrowserExtensionList, err error) {
		if err != nil || list == nil {
			return
		}
		defer list.Release()

		a.procurarExtParaHabilitar(list, id, base, ligada)
	})
}
