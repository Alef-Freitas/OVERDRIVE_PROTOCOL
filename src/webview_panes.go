//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unsafe"
)

func (a *app) startEnvironment() {
	browserDir, ver := findFixedRuntime()
	if browserDir == "" {
		a.status = txtRuntimeSistema
	} else if ver != "" && versaoRuntimeAntiga(ver) {
		a.status = fmt.Sprintf(txtFmtRuntimeVelho, ver)
	}

	target := ver
	if target == "" {
		target = "1.0.622.22"
	}

	profileDir := filepath.Join(dataDir, "profile")
	if profileOverride != "" {
		profileDir = filepath.Join(dataDir, profileOverride)
	}

	_ = os.MkdirAll(profileDir, 0o700)
	limparCachePerfil(profileDir)

	argsPrincipal := buildBrowserArgs(a, a.st)
	if porta := strings.TrimSpace(os.Getenv("OVERDRIVE_DEBUG")); portaDebugValida(porta) {
		argsPrincipal += " --remote-debugging-port=" + porta
	}

	err := CreateEnvironment(CreateOptions{
		BrowserExecutableFolder:        browserDir,
		UserDataFolder:                 profileDir,
		AdditionalBrowserArguments:     argsPrincipal,
		Language:                       idiomaPtBR,
		TargetCompatibleBrowserVersion: target,
		EnableExtensions:               len(a.st.Extensions) > 0,
	}, func(env *Environment, err error) {
		if err != nil {
			a.status = fmt.Sprintf(txtFmtFalhaWV, err.Error())
			pInvalidateRect.Call(a.hwnd, 0, 0)

			msg := fmt.Sprintf(txtFmtWVFatal, err.Error())

			const iconeErroWin = 0x00000010

			pMessageBoxW.Call(a.hwnd,
				uintptr(unsafe.Pointer(utf16(msg))),
				uintptr(unsafe.Pointer(utf16(appTitle))),
				iconeErroWin)

			return
		}

		a.env = env
		alinharVersaoChrome(env.BrowserVersion)
		a.status = fmt.Sprintf(txtFmtVersaoWV, env.BrowserVersion)
		a.createPanes()

		// Modo Marble: esconde o pane assim que fica pronto (o card so abre
		// o WebView mediante duplo clique).
		espOcultarWebView(a)
		pInvalidateRect.Call(a.hwnd, 0, 1)
	})
	if err != nil {
		a.status = err.Error()
	}
}

func (a *app) createPanes() {
	// Modo Marble Overdrive: nao restaurar abas no arranque. O WebView so
	// nasce quando o usuario abre um card (via espAbrirCard).
	// a.restaurarAbas()  // preservado: reativar se voltar ao OVERDRIVE classico.
	espOcultarWebView(a)
}

func (a *app) prepararPane(p *pane, w *WebView, configurarMobile bool) {
	if p.wv != nil && p.wv != w {
		p.wv.Close()
	}

	p.wv = w
	p.ready = true
	p.visivelSet = false
	p.boundsSet = false
	p.cssW, p.cssH = 0, 0
	p.zoom = 1
	p.zoomLogged = 0
	p.emulFixa = false
	p.frio = false
	p.frioManual = false
	p.frioPendente = false
	p.nojsPendente = false
	p.nojsAtivo = false
	p.nojsCarregando = false
	p.sonoDormindo = false
	p.frioDesde = time.Time{}
	p.ociosoDesde = time.Time{}
	p.readerAtivo = false
	p.readerCarregando = false
	p.readerPronto = false
	p.readerURL = ""
	p.descarregado = false
	p.urlDescarga = ""
	p.scrollDescarga = 0
	p.scrollRestaurar = 0

	p.tocandoAudio = false
	p.mudo = false
	a.configureWebView(p, configurarMobile)
}

func (a *app) destruirPane(p *pane) {
	if p == nil {
		return
	}

	tradCancelarSe(p)

	if p.wv != nil {
		p.wv.Close()
		p.wv = nil
	}

	p.ready = false
	p.boundsSet = false
	p.visivel = false
	p.visivelSet = false
	p.frio = false
	p.frioManual = false
	p.frioPendente = false
	p.nojsPendente = false
	p.nojsAtivo = false
	p.nojsCarregando = false
	p.sonoDormindo = false
	p.frioDesde = time.Time{}
	p.ociosoDesde = time.Time{}
	p.readerAtivo = false
	p.readerCarregando = false
	p.readerPronto = false
	p.readerURL = ""
	p.descarregado = false
	p.urlDescarga = ""
	p.scrollDescarga = 0
	p.scrollRestaurar = 0
	p.emulFixa = false
	p.cssW, p.cssH = 0, 0
	p.zoom = 1
	p.zoomLogged = 0
	p.tocandoAudio = false
	p.mudo = false
}

func (a *app) criarWebView(p *pane, configurarMobile bool, permitido func() bool, antes, depois func(*WebView)) error {
	return a.criarWebViewEm(a.env, p, configurarMobile, permitido, antes, depois)
}

func (a *app) criarWebViewEm(env *Environment, p *pane, configurarMobile bool, permitido func() bool,
	antes, depois func(*WebView),
) error {
	if env == nil || a.hwnd == 0 {
		return erroTexto("ambiente WebView2 indisponivel")
	}

	return env.CreateController(a.hwnd, func(w *WebView, err error) {
		if err != nil {
			return
		}

		if permitido != nil && !permitido() {
			w.Close()

			return
		}

		w.ownerEnv = env
		a.prepararPane(p, w, configurarMobile)

		if antes != nil {
			antes(w)
		}

		a.layout()

		if depois != nil {
			depois(w)
		}
	})
}

const pisoRuntimeWebView2 = "130.0.0.0"

func versaoRuntimeAntiga(ver string) bool {
	parte := func(s string) int {
		const baseDecimalVersao = 10

		num := 0

		for _, r := range s {
			if r < '0' || r > '9' {
				break
			}

			num = num*baseDecimalVersao + int(r-'0')
		}

		return num
	}

	atual, minimo := strings.Split(ver, "."), strings.Split(pisoRuntimeWebView2, ".")
	for idx := 0; idx < len(atual) && idx < len(minimo); idx++ {
		if parte(atual[idx]) != parte(minimo[idx]) {
			return parte(atual[idx]) < parte(minimo[idx])
		}
	}

	return len(atual) < len(minimo)
}

func findFixedRuntime() (string, string) {
	base := filepath.Join(runtimeDir, "WebView2")

	entries, err := os.ReadDir(base)
	if err != nil {
		return "", ""
	}

	for _, entrada := range entries {
		if !entrada.IsDir() {
			continue
		}

		pasta := filepath.Join(base, entrada.Name())
		if _, err := os.Stat(filepath.Join(pasta, "msedgewebview2.exe")); err == nil {
			ver := ""
			if m := rxVersion.FindString(entrada.Name()); m != "" {
				ver = m
			}

			return pasta, ver
		}
	}

	return "", ""
}

func modeloDeProcessos(estado state) string {
	return fmt.Sprintf("--renderer-process-limit=%d", limiteRenderers(estado))
}

func buildBrowserArgs(a *app, estado state, extras ...string) string {
	desativadas := browserDesativadas(estado, extras)
	base := browserBaseArgs(estado, desativadas)
	base = browserProxyArgs(a, base)

	if estado.ExtraArgs != "" {
		base = append(base, filtrarExtraArgs(estado.ExtraArgs))
	}

	return strings.Join(base, " ")
}

// browserDesativadas monta a lista de features desligadas por economia/privacidade.
func browserDesativadas(estado state, extras []string) []string {
	desativadas := []string{
		"Translate", "msEdgeTranslate", "msEdgeSidebar", "msEdgeIdentity", "msEdgeAutofill",
		"msEdgeCollections", "msEdgeShoppingAssistant", "msSmartScreenProtection", "msWebOOUI",
		"msPdfOOUI", "OptimizationHints", "OptimizationGuideModelDownloading",
		"AutofillServerCommunication", "BackForwardCache", "SpareRendererForSitePerProcess",
		"SpeculationRules", "Prerender2", "AudioServiceOutOfProcess", "MediaRouter",
		"NetworkPrediction", "PreconnectToSearch", "NoStatePrefetch",
		"FluentOverlayScrollbar", "FluentScrollbar", "OverlayScrollbar",
	}

	desativadas = append(desativadas, extras...)
	if estado.SemGPU && estado.EconomiaMax {
		desativadas = append(desativadas, "IsolateOrigins", "site-per-process", "StrictOriginIsolation")
	}

	return desativadas
}

// browserBaseArgs monta os flags fixos mais os condicionais simples.
func browserBaseArgs(estado state, desativadas []string) []string {
	base := []string{
		modeloDeProcessos(estado),
		"--disk-cache-size=134217728",
		"--media-cache-size=33554432",
		"--disable-background-networking",
		"--dns-prefetch-disable",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--disable-sync",
		"--disable-breakpad",
		"--disable-crash-reporter",
		"--no-pings",
		"--disable-client-side-phishing-detection",
		"--safebrowsing-disable-auto-update",
		"--no-first-run",
		"--no-default-browser-check",
		"--autoplay-policy=document-user-activation-required",
		"--msWebView2SimulateMemoryPressureWhenInactive",
		"--msWebView2TreatAppSuspendAsDeviceSuspend",
		"--force-gpu-mem-available-mb=256",
		"--enable-low-end-device-mode",
		"--js-flags=--max-old-space-size=256 --lite-mode",
		"--disable-default-apps",
		"--disable-features=" + strings.Join(desativadas, ","),
	}
	if estado.SemServiceWorker {
		base = append(base, "--disable-blink-features=ServiceWorker")
	}

	if estado.SemGPU {
		base = append(base, "--disable-gpu", "--disable-gpu-compositing")
	}

	return base
}

// browserProxyArgs acrescenta o proxy da VPN quando ativo.
func browserProxyArgs(a *app, base []string) []string {
	if a.vpn == nil {
		return base
	}

	if addr := a.vpn.proxyAddr(); addr != "" {
		base = append(base,
			"--proxy-server=http://"+addr,
			"--proxy-bypass-list=localhost;127.0.0.1;<local>",
			"--disable-quic",
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp")
	}

	return base
}

func filtrarExtraArgs(texto string) string {
	perigosas := []string{
		"load-extension", "load-component-extension", "renderer-cmd-prefix",
		"gpu-launcher", "utility-cmd-prefix", "remote-debugging-port",
		"remote-debugging-address", "no-sandbox", "single-process",
		"in-process-gpu", "disable-web-security", "allow-running-insecure-content",
		"disable-site-isolation", "enable-automation", "disable-setuid-sandbox",
		"no-zygote",
	}

	var mantidas []string

	for campo := range strings.FieldsSeq(texto) {
		nome := strings.TrimPrefix(campo, "--")
		if i := strings.IndexByte(nome, '='); i >= 0 {
			nome = nome[:i]
		}

		nome = strings.ToLower(nome)
		barrada := slices.Contains(perigosas, nome)

		if barrada {
			continue
		}

		mantidas = append(mantidas, campo)
	}

	return strings.Join(mantidas, " ")
}

func limiteRenderers(_ state) int {
	const limiteRenderersPadrao = 2

	return limiteRenderersPadrao
}

func (a *app) paneZoom(p *pane) float64 {
	if p != nil && p.mobile {
		if p.zoom > 0 {
			return p.zoom
		}

		return 1
	}

	return a.st.Zoom
}

func (a *app) cursorNoPainel(p *pane) bool {
	if p == nil {
		return false
	}

	var pos struct{ x, y int32 }
	if r, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&pos))); r == 0 {
		return false
	}

	pScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&pos)))

	r := p.rect

	return r.Width() > 0 && r.Height() > 0 && pos.x >= r.Left && pos.x < r.Right && pos.y >= r.Top && pos.y < r.Bottom
}

// capacidadeAlvosFoco cobre navegador, chat e painel de imagens.
const capacidadeAlvosFoco = 3

// alvoFoco junta o painel focavel ao rotulo que a barra mostra.
type alvoFoco struct {
	nome   string
	painel *pane
}

func (a *app) paineisFocaveis() []alvoFoco {
	alvos := make([]alvoFoco, 0, capacidadeAlvosFoco)

	add := func(nome string, p *pane) {
		if p != nil && p.ready && p.wv != nil && p.visivel && p.rect.Width() > 0 {
			alvos = append(alvos, alvoFoco{nome: nome, painel: p})
		}
	}
	add("navegador", a.left)
	add(idChat, a.right)
	add(idImagens, a.imgPanel)

	return alvos
}

func (a *app) ciclarFocoPainel() {
	alvos := a.paineisFocaveis()
	if len(alvos) == 0 {
		return
	}

	atual := -1

	for i, alvo := range alvos {
		if a.paneFocado(alvo.painel) {
			atual = i

			break
		}
	}

	prox := (atual + 1) % len(alvos)
	alvos[prox].painel.wv.Focus()
	a.dizerf(txtFmtFoco, alvos[prox].nome)
}

func (a *app) paneFocado(p *pane) bool {
	if p == nil {
		return false
	}

	if p == a.left {
		return a.focus == idLeft && !a.urlEditing
	}

	if p == a.right {
		return a.focus == idRight
	}

	if p == a.imgPanel {
		return a.focus == idImg
	}

	return false
}

func (a *app) panesGerenciados() []*pane {
	return []*pane{a.left, a.right, a.imgPanel}
}

func setPaneBounds(p *pane) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if !p.boundsSet || p.boundsApplied != p.rect {
		p.wv.SetBounds(p.rect)
		p.boundsApplied = p.rect
		p.boundsSet = true
	}
}

func setPaneVisible(p *pane, visible bool) {
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	if !p.visivelSet || p.visivel != visible {
		p.wv.SetVisible(visible)

		if visible && (!p.visivelSet || !p.visivel) {
			p.wv.Resume()
			p.wv.SetFrozen(false)
			p.frio = false
			p.frioPendente = false
		}

		p.visivel = visible
		p.visivelSet = true
	}
}

func precisaMexerVisibilidade(p *pane, quer bool) bool {
	if p == nil {
		return false
	}

	return !p.visivelSet || p.visivel != quer
}

func (a *app) raisePanes() {
	pEnumChildWindows.Call(a.hwnd, enumRaiseCb, 0)
}

func (a *app) focused() *pane {
	if a.focus == idRight && a.st.RightVisible {
		return a.right
	}

	return a.left
}

const popupCleanJS = `(function(){
  function limpar(){
    try {
      var h = document.getElementById('hostname');
      if (h) {
        var t = String(h.textContent || '').replace(/[\s\xad]*/g, '').toLowerCase();
        var invalido = t === '' || t.indexOf('chrome-extension') >= 0 || /^[a-z0-9]{32}$/.test(t);
        h.style.display = invalido ? 'none' : '';
      }
    } catch (e) {}
  }
  if(document.readyState==='loading'){ document.addEventListener('DOMContentLoaded',limpar); } else { limpar(); }
  try{ new MutationObserver(limpar).observe(document.documentElement,
    {childList:true,subtree:true,characterData:true}); }catch(e){}
})();`

func (a *app) agendarUboViaPerfil() {
	a.reconciliarExtDoPerfil(idUbo, func() {
		if a.abrirPopupUboRetry() {
			return
		}

		if a.chutarExtensao(idUbo) {
			a.dizerProgressoIndetf(txtExtAbrindoPainel)

			const atrasoReaberturaUboMs = 2000

			time.AfterFunc(atrasoReaberturaUboMs*time.Millisecond, func() {
				a.depois(func() {
					if a.quitting {
						return
					}

					if !a.abrirPopupUboRetry() {
						a.dizerf(txtUboNaoCarr2)
					}
				})
			})

			return
		}

		a.dizerf(txtUboNaoCarr2)
	})
}

func (a *app) abrirPopupUbo() {
	if a.extLoaded[idUbo] {
		if id, caminho, nome, err := a.painelDaExtensao(idUbo); err == nil {
			a.abrirPopupGen(id, caminho, nome)

			return
		}
	}

	if a.pastaRealExtensao(idUbo) != "" {
		a.agendarUboViaPerfil()

		return
	}

	a.dizerf(txtUboNaoCarr2)
}

func (a *app) abrirPopupUboRetry() bool {
	if a.extLoaded[idUbo] {
		if id, caminho, nome, err := a.painelDaExtensao(idUbo); err == nil {
			if a.popupOn && a.popupExtID == id {
				return true
			}

			a.abrirPopupGen(id, caminho, nome)

			return true
		}
	}

	return false
}

func (a *app) alternarPopupAberto(id string, uboPopup bool) bool {
	if uboPopup {
		return false
	}

	if a.popupOn && a.popupExtID == id && a.popup != nil && a.popup.ready && a.popup.wv != nil {
		a.fecharPopup()
		a.layout()

		return true
	}

	return false
}

func (a *app) prepararPopupNovo(uboPopup bool) bool {
	if a.popup != nil && a.popup.ready && a.popup.wv != nil {
		return false
	}

	a.popup = nil

	a.popupOn = true
	if !uboPopup {
		a.vigiarAlturaPopup()
	}

	a.createPopupPane()

	return true
}

func (a *app) retomarPopupExistente(uboPopup bool) {
	a.popupOn = true
	if a.popup.ready && a.popup.wv != nil {
		a.popup.wv.Resume()
	}

	a.openPopupURL()

	if !uboPopup {
		a.vigiarAlturaPopup()
	}

	a.layout()
}

func (a *app) normalizarDestinoPopup(id, caminho, nome string) (string, string, string, bool) {
	if nome == "" {
		nome = "a extensão"
	}

	if id == "" {
		a.dizerErrof(txtFmtNaoCarregado, nome)

		return "", "", "", false
	}

	if caminho == "" {
		caminho = paginaPopup
	}

	return id, caminho, nome, true
}

func (a *app) abrirPopupGen(id, caminho, nome string) {
	id, caminho, nome, ok := a.normalizarDestinoPopup(id, caminho, nome)
	if !ok {
		return
	}

	uboPopup := a.ehPopupUbo(id, caminho)
	if a.alternarPopupAberto(id, uboPopup) {
		return
	}

	if uboPopup {
		a.popupVigiaGen++
		if a.popup != nil {
			a.fecharPopup()
		}
	}

	a.popupExtID, a.popupPath, a.popupExtNome = id, caminho, nome
	if a.prepararPopupNovo(uboPopup) {
		return
	}

	a.retomarPopupExistente(uboPopup)
}

func (a *app) nomeDoManifestoExtensao(pasta, atual string) string {
	abs, erroAbs := filepath.Abs(filepath.Join(extDir, strings.TrimSpace(pasta)))
	if erroAbs != nil {
		return atual
	}

	absExe, erroExe := filepath.Abs(exeDir)
	if erroExe != nil || abs != absExe && !strings.HasPrefix(abs, absExe+string(filepath.Separator)) {
		return atual
	}

	// GOD: pasta sanitizada na instalacao; leitura contida em extDir.
	dados, erroLeitura := os.ReadFile(filepath.Join(abs, "manifest.json")) // #nosec G304
	if erroLeitura != nil {
		return atual
	}

	var m extManifest
	if json.Unmarshal(dados, &m) != nil {
		return atual
	}

	if n := nomeExibicaoExtensao(m, abs); strings.TrimSpace(n) != "" {
		return n
	}

	return atual
}

func (a *app) painelDaExtensao(pasta string) (string, string, string, error) {
	base, ok := baseExtensaoValida(pasta)

	nome := strings.TrimSpace(pasta)
	if !ok {
		return "", "", nome, erroTexto("extensão inválida")
	}

	nome = a.nomeDoManifestoExtensao(pasta, nome)

	id := a.extIDs[base]
	if id == "" || !a.extLoaded[base] {
		return "", "", nome, erroTextoF("%s não está carregada · reabra o app", nome)
	}

	caminho := ""
	if a.extPopups != nil {
		caminho = a.extPopups[base]
	}

	if caminho == "" {
		return "", "", nome, erroTextoF("%s não tem painel", nome)
	}

	return id, caminho, nome, nil
}

func (a *app) tentarAbrirPainelExtensao(pasta string) bool {
	if id, caminho, nome, err := a.painelDaExtensao(pasta); err == nil {
		a.abrirPainelFlutuante(id, caminho, nome)

		return true
	}

	return false
}

func (a *app) relatarPainelExtensao(pasta string) {
	if _, _, _, err := a.painelDaExtensao(pasta); err != nil {
		a.dizerErrof("%s", err)
	}
}

func (a *app) aguardarPainelExtensao(pasta, base string) {
	a.depois(func() {
		if a.quitting {
			return
		}

		if a.extFloatOn && a.pastaDoFloat() == base {
			return
		}

		if id, caminho, nome, err := a.painelDaExtensao(pasta); err == nil {
			a.abrirPainelFlutuante(id, caminho, nome)
		} else if a.extLoaded != nil && a.extLoaded[base] {
			a.dizerErrof("%s", err)
		}
	})
}

func (a *app) abrirPainelExtensao(pasta string) {
	base := strings.ToLower(strings.TrimSpace(pasta))
	if id, caminho, nome, err := a.painelDaExtensao(pasta); err == nil {
		a.abrirPainelFlutuante(id, caminho, nome)

		return
	}

	a.reconciliarExtDoPerfil(base, func() {
		if a.extFloatOn && a.pastaDoFloat() == base {
			return
		}

		if a.tentarAbrirPainelExtensao(pasta) {
			return
		}

		if !a.chutarExtensao(base) {
			a.relatarPainelExtensao(pasta)

			return
		}

		a.dizerProgressoIndetf(txtExtAbrindoPainel)

		const atrasoAberturaPainelExtMs = 2000

		time.AfterFunc(atrasoAberturaPainelExtMs*time.Millisecond, func() {
			a.aguardarPainelExtensao(pasta, base)
		})
	})
}

func (a *app) pastaChuteExtensao(base string) (string, bool) {
	if base == "" || base == "." || base == ".." {
		return "", false
	}

	orig := a.pastaRealExtensao(base)
	if orig == "" || (!a.extensaoLigada(base) && !ehExtensaoSistema(base)) {
		return "", false
	}

	if a.left == nil || !a.left.ready || a.left.wv == nil {
		return "", false
	}

	return filepath.Join(extDir, orig), true
}

func popupDoManifestoExtensao(dir string) string {
	var manifesto extManifest
	// GOD: pasta sanitizada na instalacao; leitura contida em extDir.
	if b, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil { // #nosec G304
		_ = json.Unmarshal(b, &manifesto)
	}

	if manifesto.Action != nil && manifesto.Action.DefaultPopup != "" {
		return manifesto.Action.DefaultPopup
	}

	if manifesto.BrowserAction != nil {
		return manifesto.BrowserAction.DefaultPopup
	}

	return ""
}

func (a *app) chutarExtensao(base string) bool {
	dir, ok := a.pastaChuteExtensao(base)
	if !ok {
		return false
	}

	a.tryAddExtensionEm(a.left.wv, dir, popupDoManifestoExtensao(dir), true)

	return true
}

func (a *app) fecharPopup() {
	a.popupGen++
	if a.popup == nil {
		a.popupOn = false
		a.popupAltura = 0
		a.popupVigiaGen++

		return
	}

	a.destruirPane(a.popup)
	a.popup = nil
	a.popupOn = false
	a.popupAltura = 0
	a.popupVigiaGen++

	const atrasoLimpezaPopupMs = 1500

	a.agendarLimpezaMemoria(true, atrasoLimpezaPopupMs*time.Millisecond)
}

func (a *app) vigiarAlturaPopup() {
	if a.ehPopupUbo(a.popupExtID, a.popupPath) {
		a.popupVigiaGen++

		return
	}

	a.popupVigiaGen++
	gen := a.popupVigiaGen

	const intervaloMedicaoPopupSegundos = 2
	time.AfterFunc(intervaloMedicaoPopupSegundos*time.Second, func() {
		a.depois(func() { a.medirAlturaPopup(gen) })
	})
}

func (a *app) popupProntoParaMedir(gen uint64) (*pane, bool) {
	if gen != a.popupVigiaGen || !a.popupOn || a.ehPopupUbo(a.popupExtID, a.popupPath) {
		return nil, false
	}

	if ic, _, _ := pIsIconic.Call(a.hwnd); ic != 0 {
		return nil, false
	}

	p := a.popup
	if p == nil || !p.ready || p.wv == nil {
		return nil, false
	}

	return p, true
}

func (a *app) processarMedidaAltura(res string, err error) {
	if err != nil {
		return
	}

	var medida struct {
		OK bool `json:"ok"`
		H  int  `json:"h"`
		IH int  `json:"ih"`
	}
	if desembrulharEvalJS(res, &medida) != nil || !medida.OK {
		return
	}

	h, ih := medida.H, medida.IH

	a.depois(func() { a.ajustarAlturaPopup(h, ih) })
}

func (a *app) medirAlturaPopup(gen uint64) {
	const intervaloRemedicaoPopupSegundos = 2

	p, ok := a.popupProntoParaMedir(gen)
	if !ok {
		return
	}

	time.AfterFunc(intervaloRemedicaoPopupSegundos*time.Second, func() {
		a.depois(func() { a.medirAlturaPopup(gen) })
	})

	p.wv.EvalJS(medidaAlturaJS, func(err error, res string) {
		a.processarMedidaAltura(res, err)
	})
}

func (a *app) limitesAlturaPopup() (int32, int32) {
	minAlt := a.sc(240)

	maxAlt := int32(0)
	if a.left != nil {
		maxAlt = a.left.rect.Height() - a.sc(20)
	}

	return minAlt, maxAlt
}

func (a *app) calcularNovaAlturaPopup(hCSS, ihCSS int, altAtual int32) (int32, bool) {
	nova := int32(float64(hCSS) * float64(altAtual) / float64(ihCSS))
	minAlt, maxAlt := a.limitesAlturaPopup()

	if nova < minAlt {
		nova = minAlt
	}

	if maxAlt > minAlt && nova > maxAlt {
		nova = maxAlt
	}

	if nova < 0 {
		return 0, false
	}

	if absI32(nova-altAtual) < a.sc(8) {
		return 0, false
	}

	return nova, true
}

func (a *app) ajustarAlturaPopup(hCSS, ihCSS int) {
	p := a.popup
	if p == nil || !a.popupOn || a.ehPopupUbo(a.popupExtID, a.popupPath) || hCSS <= 0 || ihCSS <= 0 {
		return
	}

	altAtual := p.rect.Height()
	if altAtual <= 0 {
		return
	}

	nova, ok := a.calcularNovaAlturaPopup(hCSS, ihCSS, altAtual)
	if !ok {
		return
	}

	p.rect.Bottom = p.rect.Top + nova
	a.popupAltura = nova

	setPaneBounds(p)
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) openPopupURL() {
	if a.popup == nil || !a.popup.ready {
		return
	}

	path := a.popupPath
	if path == "" {
		path = paginaPopup
	}

	id := a.popupExtID
	if id == "" {
		return
	}

	url := a.urlPopupExtensao(id, path)
	a.popup.wv.Navigate(url)

	if !a.ehPopupUbo(id, path) {
		a.limparPopupDepois()
	}
}

func (a *app) limparPopupDepois() {
	if a.ehPopupUbo(a.popupExtID, a.popupPath) {
		return
	}

	p := a.popup
	if p == nil || p.wv == nil {
		return
	}

	const atrasoLimpezaPopupDepoisMs = 1200
	time.AfterFunc(atrasoLimpezaPopupDepoisMs*time.Millisecond, func() {
		a.depois(func() {
			if p == a.popup && p.ready && p.wv != nil {
				p.wv.EvalJS(popupCleanJS, nil)
			}
		})
	})
}

func (a *app) createPopupPane() {
	if a.env == nil {
		a.popupOn = false
		a.dizerErrof(txtFmtIndispMotor, nomePopup(a))

		return
	}

	a.popupGen++
	gen := a.popupGen

	err := a.env.CreateController(a.hwnd, func(w *WebView, err error) {
		if err != nil {
			a.popupOn = false
			a.dizerErrof(txtFmtNaoAbri, nomePopup(a))

			return
		}

		if a.quitting || !a.popupOn || gen != a.popupGen {
			w.Close()

			return
		}

		p := &pane{wv: w, zoom: a.st.Zoom}
		p.ready = true
		a.popup = p
		a.popupOn = true
		a.configureWebView(p, false)

		if !a.ehPopupUbo(a.popupExtID, a.popupPath) {
			p.wv.AddInitScript(popupCleanJS)
		}

		a.layout()
		a.openPopupURL()
	})
	if err != nil {
		a.popupOn = false
		a.dizerErrof(txtFmtNaoAbri, nomePopup(a))
	}
}

func nomePopup(a *app) string {
	if a != nil && strings.TrimSpace(a.popupExtNome) != "" {
		return a.popupExtNome
	}

	return "a extensão"
}

func (a *app) paneName(p *pane) string {
	switch p {
	case nil:
		return "?"
	case a.left:
		return "esquerdo"
	case a.right:
		return idChat
	case a.imgPanel:
		return idImagens
	case a.popup:
		return "popup"
	}

	return "outro"
}

// Valores de ICoreWebView2ProcessFailedKind que exigem acao.
const (
	falhaProcessoNavegador    = 0
	falhaProcessoRenderizador = 1
	falhaProcessoQuadro       = 3
)

// periodoRecuperacaoAba impede recriar o WebView em loop quando o site
// derruba o renderizador logo apos carregar.
const periodoRecuperacaoAba = 30 * time.Second

// processoDerrubaDocumento diz se a falha mata a pagina da aba.
func processoDerrubaDocumento(kind uint32) bool {
	switch kind {
	case falhaProcessoRenderizador, falhaProcessoQuadro:
		return true
	default:
		return false
	}
}

// abaDoPane acha a aba dona do painel, inclusive quando esta oculta.
func (a *app) abaDoPane(p *pane) *tab {
	if p == nil || a == nil {
		return nil
	}

	for _, aba := range a.tabs {
		if aba != nil && aba.pane == p {
			return aba
		}
	}

	return nil
}

// enderecoAproveitavel diz se a URL pode ser reaberta.
func enderecoAproveitavel(endereco string) bool {
	return endereco != "" && !strings.HasPrefix(endereco, "about:")
}

// enderecoPaneFalho preserva a ultima pagina real para recriar a aba.
func enderecoPaneFalho(p *pane, aba *tab) string {
	endereco := strings.TrimSpace(p.ultimaURL)
	if !enderecoAproveitavel(endereco) {
		endereco = strings.TrimSpace(aba.url)
	}

	if !enderecoAproveitavel(endereco) {
		endereco = strings.TrimSpace(p.wv.Source())
	}

	return endereco
}

// esquecerPaneFalho solta o WebView morto para a aba recriar depois.
func esquecerPaneFalho(aba *tab, endereco string) {
	if aba == nil || aba.pane == nil {
		return
	}

	desmontarPaneAba(aba)
	aba.pane.tocandoAudio = false
	aba.pane.mudo = false
	aba.pane.frioPendente = false
	aba.pane.nojsPendente = false
	aba.pane.sonoDormindo = false

	if endereco != "" {
		aba.url = endereco
		aba.pane.ultimaURL = endereco
	}
}

// podeRecarregarAbaFalha respeita o intervalo entre recuperacoes.
func (a *app) podeRecarregarAbaFalha() bool {
	return time.Since(a.ultimaRecuperacaoAba) >= periodoRecuperacaoAba
}

// socorrerPaneFalho recicla o painel da aba cujo renderizador morreu.
// Aba oculta fica apenas marcada; a proxima ativacao recria o WebView.
func (a *app) socorrerPaneFalho(p *pane) {
	if p == nil || p.wv == nil || a.quitting {
		return
	}

	aba := a.abaDoPane(p)
	if aba == nil {
		return
	}

	ativa := a.indiceDaAba(aba) == a.ativa
	endereco := enderecoPaneFalho(p, aba)
	esquecerPaneFalho(aba, endereco)

	if !a.podeRecarregarAbaFalha() {
		return
	}

	a.ultimaRecuperacaoAba = time.Now()

	if ativa {
		a.carregarAba(aba)
		a.invalidateBar()
	}
}

func (a *app) cuidarTituloWebView(p *pane, titulo string) {
	if p != nil {
		p.titulo = titulo
		if p != a.right && p != a.popup {
			a.invalidateFaixa()
		}
	}

	if p == a.left {
		a.setTitle(titulo)
	}
}

func (a *app) cuidarOrigemLeitor(p *pane) bool {
	if p.readerAtivo {
		if p.readerURL != "" {
			p.ultimaURL = p.readerURL
			a.setAddress(p.readerURL)
		}

		return true
	}

	return false
}

func (a *app) cuidarOrigemEsquerda(p *pane, uri string) {
	if p != a.left {
		if p != a.right && p != a.popup {
			a.invalidateFaixa()
		}

		return
	}

	if a.traduzido && !ehProxyGoogle(uri) {
		a.traduzido = false
		a.traducaoOrigemURL = ""
	}

	a.setAddress(uri)

	if a.extFloatOn && a.extFloat != nil && a.extFloat.ready && a.extFloat.wv != nil {
		a.extFloat.wv.EvalJS(`window.__odDefinirAba&&window.__odDefinirAba(`+jsonDeString(uri)+`)`, nil)
	}
}

func (a *app) cuidarOrigemWebView(p *pane, uri string) {
	if a.cuidarOrigemLeitor(p) {
		return
	}

	p.ultimaURL = uri
	if p == a.left {
		a.aplicarRegrasDominio()
		a.avaliarLeituraUsavel(p, uri)
	}

	if p.wv != nil && ehHostLojaExt(uri) {
		p.wv.EvalJS(lojaExtJS, nil)
	}

	if p == a.right {
		a.travarChatNaURL(p, uri)

		return
	}

	a.cuidarOrigemEsquerda(p, uri)
}

func (a *app) limparLeitorAoNavegar(p *pane, uri string) {
	p.sonoDormindo = false
	if !p.readerAtivo || (p.readerCarregando && !p.readerPronto) || uri == "" || strings.HasPrefix(uri, "about:") {
		return
	}

	limparLeitor(p)

	if p == a.left {
		a.readerBtn.on = false
		a.invalidateBar()
	}
}

func (a *app) ajustarScriptAoNavegar(w *WebView, p *pane) {
	if p.nojsCarregando {
		p.nojsCarregando = false
	} else if p.nojsAtivo {
		p.nojsAtivo = false

		w.SetScriptsEnabled(true)
	}
}

func (a *app) guardarOrigemTraducao(p *pane, uri string) {
	if a.modoTraducao() != chaveGoogle || !ehProxyGoogle(uri) {
		return
	}

	if a.traducaoOrigemURL != "" || p == nil || p.wv == nil {
		return
	}

	if u := strings.TrimSpace(p.wv.Source()); u != "" && !ehProxyGoogle(u) {
		a.traducaoOrigemURL = u
	}
}

func (a *app) cuidarInicioNavegacaoWebView(p *pane, w *WebView, uri string) {
	a.limparLeitorAoNavegar(p, uri)
	a.ajustarScriptAoNavegar(w, p)

	if p == a.left && p.frio {
		a.retomarPaneGenerico(p)
	}

	if p != a.left {
		return
	}

	a.loading = true
	a.loadStart = time.Now()

	if a.modoTraducao() == chaveGoogle && ehProxyGoogle(uri) {
		a.guardarOrigemTraducao(p, uri)
	} else if a.modoTraducao() != chaveGoogle {
		a.traduzido = false
		a.traducaoOrigemURL = ""
	}

	a.invalidateBar()
}

func (a *app) cuidarNavegadoEsquerda(p *pane, w *WebView, sucesso bool) {
	a.loading = false
	a.loadStart = time.Time{}

	if p.readerAtivo {
		p.readerPronto = true
	}

	p.readerCarregando = false

	a.invalidateBar()
	a.syncAddressBar()

	if !sucesso {
		return
	}

	if endereco := w.Source(); endereco != "" && !strings.HasPrefix(endereco, "about:") {
		p.ultimaURL = endereco

		if len(a.tabs) > 0 {
			a.invalidateFaixa()
		}

		a.avaliarLeituraUsavel(p, endereco)
	}
}

func (a *app) tentarNavegarImagens(p *pane, w *WebView, sucesso bool) {
	p.cssW, p.cssH, p.zoom = -1, -1, -1

	a.aplicarTamanhoImagens()

	if sucesso {
		p.tentativas = 0

		return
	}

	const maxTentativasImagens = 3
	if p.tentativas >= maxTentativasImagens || !strings.HasPrefix(strings.ToLower(w.Source()), "about:") {
		return
	}

	p.tentativas++

	endereco := strings.TrimSpace(p.ultimaURL)
	if !strings.HasPrefix(endereco, "https://www.google.com/") {
		endereco = imagensHomeURL
	}

	mobile := !p.busca

	const atrasoTentativaImagensMs = 700
	time.AfterFunc(atrasoTentativaImagensMs*time.Millisecond, func() {
		a.depois(func() { a.navegarImagens(endereco, mobile) })
	})
}

func (a *app) cuidarNavegadoDireita(p *pane, sucesso bool) {
	if p.frio {
		p.frio = false
		p.frioManual = false
		p.wv.SetFrozen(false)
	}

	if a.chatFrio {
		a.chatFrio = false
		pKillTimer.Call(a.hwnd, timerChatFrio)
	}

	p.ociosoDesde = time.Time{}
	if sucesso {
		p.tentativas = 0
		a.chatVazioCuidaDaPaginaEmBranco(p, sucesso)

		if a.chatPendente != nil {
			f := a.chatPendente
			a.chatPendente = nil
			a.depois(f)
		}

		if len(a.anexosPendentes1) > 0 {
			itens := append([]imagemAnexo(nil), a.anexosPendentes1...)
			a.anexosPendentes1 = nil
			a.depois(func() { a.anexarImagensEmPainel(a.right, itens) })
		}

		return
	}

	const maxTentativasChat = 4
	if p.tentativas < maxTentativasChat && p.queriaURL != "" {
		p.tentativas++

		const intervaloTentativaChatMs = 2500
		pSetTimer.Call(a.hwnd, uintptr(timerRetryChat), intervaloTentativaChatMs, 0)
	}
}

func (a *app) restaurarRolagemPane(p *pane, w *WebView) {
	if p.scrollRestaurar <= 0 {
		return
	}

	alvo := p.scrollRestaurar
	p.scrollRestaurar = 0
	jsRolar := fmt.Sprintf(`(function(){var y=%.1f;`+
		`var t=document.querySelector('.thread-scroll-container');`+
		`if(!t){var e=document.scrollingElement||document.documentElement;`+
		`if(e&&e.scrollHeight>e.clientHeight+4)t=e;}`+
		`if(!t){var all=document.querySelectorAll('div,main,section,article');var bh=0;`+
		`for(var i=0;i<all.length;i++){var a=all[i];`+
		`if(a.scrollHeight>a.clientHeight+40&&a.clientHeight>bh){bh=a.clientHeight;t=a;}}}`+
		`if(t){t.scrollTop=y;}})();`, alvo)
	w.ExecuteScript(jsRolar)

	const atrasoRolagemPaneMs = 1500

	time.AfterFunc(atrasoRolagemPaneMs*time.Millisecond, func() {
		a.depois(func() {
			if p.ready && p.wv != nil {
				p.wv.ExecuteScript(jsRolar)
			}
		})
	})
}

// ativarModosNoPaneAtivo reaplica os interruptores que vivem no documento.
func (a *app) ativarModosNoPaneAtivo(p *pane) {
	if p != a.left {
		return
	}

	if a.st.SleepLite {
		a.depois(func() { a.ativarSonoLiteNoPane(p) })
	}

	if a.st.ForceDark {
		a.depois(func() { a.ativarForceDarkNoPane(p) })
	}

	if a.st.LivreClique {
		a.depois(func() { a.ativarLivreCliqueNoPane(p) })
	}
}

func (a *app) cuidarNavegadoWebView(p *pane, w *WebView, mobile bool, sucesso bool) {
	if p.nojsPendente {
		p.nojsPendente = false
		p.nojsAtivo = true
	}

	if p == a.left {
		a.cuidarNavegadoEsquerda(p, w, sucesso)
		a.ativarModosNoPaneAtivo(p)
	}

	if p == a.imgPanel {
		a.tentarNavegarImagens(p, w, sucesso)
	}

	if p == a.right {
		a.cuidarNavegadoDireita(p, sucesso)
	}

	a.restaurarRolagemPane(p, w)
	a.persist()

	if mobile {
		a.applyMobile(p)
	}

	a.aplicarZoomNativo(p, a.paneZoom(p))
	w.ExecuteScript(cookieRejectJS)
}

func (a *app) cuidarNovaJanelaWebView(p *pane, uri string, userInitiated bool) bool {
	if uri == "" || !userInitiated {
		return true
	}

	if p == a.right {
		a.depois(func() { a.escolherOndeAbrir(uri, p) })

		return true
	}

	a.depois(func() {
		a.novaAba(uri, false)
		a.status = txtAbaFundo
		a.invalidateBar()
	})

	return true
}

func (a *app) cuidarFalhaProcessoWebView(p *pane, kind uint32) {
	if p == a.left {
		a.loading = false
		a.loadPhase = 0
		a.invalidateBar()
	}

	if kind == falhaProcessoNavegador {
		a.dizerErrof(txtMotorCaiuDizer)
	} else {
		a.dizerErrof(txtProcessoFalhou)
	}

	// Fechar o WebView dentro do proprio callback de falha e inseguro:
	// a reciclagem sempre passa por a.depois.
	if processoDerrubaDocumento(kind) {
		a.depois(func() { a.socorrerPaneFalho(p) })
	}

	pInvalidateRect.Call(a.hwnd, 0, 0)
}

func (a *app) cuidarFocoWebView(p *pane) {
	p.ociosoDesde = time.Time{}
	switch p {
	case a.right:
		a.focus = idRight
	case a.left:
		a.focus = idLeft
	case a.imgPanel:
		a.focus = idImg
		a.imgInteragido = time.Now()
	}

	if p.frio && !p.frioManual {
		if p == a.right && a.chatFrio {
			a.retomarChatGPTFrio()
		} else {
			a.retomarPaneGenerico(p)
		}
	}
}

func (a *app) cuidarMsgLeitor(p *pane, tipo, msg string) bool {
	if p == nil || !p.readerAtivo || tipo != idReader {
		return false
	}

	alvo := p

	a.depois(func() { a.tratarMsgLeitor(alvo, msg) })

	return true
}

func (a *app) cuidarMsgRolagem(p *pane) {
	if p == a.ytPane && a.ytAberto {
		return
	}

	alvo := p

	a.depois(func() { a.agendarLimpezaScroll(alvo) })
}

func (a *app) cuidarMsgSono(p *pane) {
	alvo := p

	a.depois(func() {
		if alvo == nil || !a.st.SleepLite || alvo.sonoDormindo {
			return
		}

		alvo.sonoDormindo = true

		a.layout()
		a.invalidateBar()
		a.limparMemoriaSono(alvo)

		const atrasoLimpezaSonoMs = 900
		a.agendarLimpezaMemoria(true, atrasoLimpezaSonoMs*time.Millisecond)
	})
}

func (a *app) cuidarMsgDespertar(p *pane) {
	alvo := p

	a.depois(func() {
		if alvo == nil || !alvo.sonoDormindo {
			return
		}

		alvo.sonoDormindo = false

		a.layout()
		a.invalidateBar()
	})
}

func (a *app) cuidarMsgAppPane(p *pane, msg string) bool {
	if p == nil || !ehPaginaInterna(p.ultimaURL) {
		return true
	}

	alvo := p

	a.depois(func() { a.tratarMsgApp(alvo, msg) })

	return true
}

func (a *app) cuidarMsgAlturaPopup(msg string) {
	var medida struct {
		H  int `json:"h"`
		IH int `json:"ih"`
	}
	if json.Unmarshal([]byte(msg), &medida) != nil {
		return
	}

	h, ih := medida.H, medida.IH

	a.depois(func() { a.ajustarAlturaPopup(h, ih) })
}

func (a *app) cuidarMsgBuscaImagens(msg string) {
	var pedido struct {
		Q string `json:"q"`
	}
	if json.Unmarshal([]byte(msg), &pedido) != nil {
		return
	}

	termo := strings.TrimSpace(pedido.Q)

	imgURL := imagensHomeURL
	if termo != "" {
		imgURL += url.QueryEscape(termo)
	}

	a.imgPanel.ultimaURL = imgURL
	a.imgPanel.busca = termo == ""
	a.navegarImagens(imgURL, termo == "")
}

func (a *app) cuidarMsgChatAtivo(msg string) {
	var estado struct {
		Streaming bool `json:"streaming"`
		Active    bool `json:"active"`
	}
	if json.Unmarshal([]byte(msg), &estado) != nil {
		return
	}

	a.depois(func() {
		if a.chatGerando != estado.Streaming || a.chatAtivo != estado.Active {
			a.chatGerando = estado.Streaming
			a.chatAtivo = estado.Active
		}

		if (estado.Streaming || estado.Active) && !a.chatUsado {
			a.chatUsado = true
		}

		a.reavaliarChatGPT()
	})
}

func (a *app) extrairEnderecoDeMsg(msg string) {
	if _, after, ok := strings.Cut(msg, "\"u\":\""); ok {
		rest := after
		if j := strings.LastIndex(rest, "\""); j > 0 {
			a.setAddress(rest[:j])
		}
	}
}

func (a *app) cuidarMensagemPaneWebView(p *pane, tipo, msg string) bool {
	if a.cuidarMsgLeitor(p, tipo, msg) {
		return true
	}

	if tipo == "scroll" {
		a.cuidarMsgRolagem(p)

		return true
	}

	if tipo == "senhas" {
		a.depois(func() { a.cuidarMsgSenhas(p, msg) })

		return true
	}

	if tipo == idSleep {
		a.cuidarMsgSono(p)

		return true
	}

	if tipo == "wake" {
		a.cuidarMsgDespertar(p)

		return true
	}

	if tipo == "app" {
		return a.cuidarMsgAppPane(p, msg)
	}

	if tipo == "gbimg" {
		a.depois(func() { a.tratarMsgImagens(msg) })

		return true
	}

	if tipo == "gbpopuph" && p == a.popup {
		a.cuidarMsgAlturaPopup(msg)

		return true
	}

	return false
}

func (a *app) cuidarMsgBuscaImagensPane(_ *pane, msg string) {
	if !a.st.RightVisible || !a.st.ImgVisible || a.imgPanel == nil || !a.imgPanel.ready || a.imgPanel.wv == nil {
		return
	}

	a.cuidarMsgBuscaImagens(msg)
}

func (a *app) cuidarMsgLinkChat(p *pane, msg string) {
	var m struct {
		URL string `json:"url"`
	}
	if json.Unmarshal([]byte(msg), &m) == nil && m.URL != "" {
		a.escolherOndeAbrir(m.URL, p)
	}
}

func (a *app) cuidarMsgLojaExt(p *pane, _ string) bool {
	if p == nil {
		return true
	}

	alvo := p

	a.depois(func() { a.instalarExtDaLoja(alvo) })

	return true
}

func (a *app) cuidarMensagemChatWebView(p *pane, tipo, msg string) bool {
	if tipo == "imgq" && p == a.right {
		a.cuidarMsgBuscaImagensPane(p, msg)

		return true
	}

	if tipo == "gblink" && p == a.right {
		a.cuidarMsgLinkChat(p, msg)

		return true
	}

	if tipo == "odext" {
		return a.cuidarMsgLojaExt(p, msg)
	}

	if tipo == "odchat" && p == a.right {
		a.cuidarMsgChatAtivo(msg)

		return true
	}

	return false
}

func (a *app) cuidarMensagemWebView(p *pane, msg string) {
	var envelope struct {
		T string `json:"t"`
	}

	_ = json.Unmarshal([]byte(msg), &envelope)

	tipo := envelope.T
	if a.cuidarMensagemPaneWebView(p, tipo, msg) {
		return
	}

	if a.cuidarMensagemChatWebView(p, tipo, msg) {
		return
	}

	if p != a.left {
		return
	}

	a.extrairEnderecoDeMsg(msg)
}

func (a *app) configurarBaseWebView(p *pane, w *WebView) {
	w.SetDevToolsEnabled(true)
	w.SetContextMenuEnabled(true)
	w.SetStatusBarEnabled(false)
	w.SetErrorPageEnabled(true)
	w.SetZoomControlEnabled(false)
	w.SetBrowserAcceleratorKeys(false)
	w.SetAutofill(false, false)
	w.SetPinchZoom(false)
	w.SetSwipeNavigation(false)

	if p != nil && !p.mobile {
		w.SetBackgroundColor(ColorWhite())
	} else {
		w.SetBackgroundColor(ColorBlack())
	}

	w.SetRawPixelBounds()
	w.AllowPermissions = false
	w.AllowExternalDrop = false
	w.OnPermissionRequested = func(uri string, kind uint32) bool {
		return a.permissaoPermitida(uri, kind)
	}
	w.OnAudioChanged = func(playing bool) {
		if p != nil {
			p.tocandoAudio = playing

			a.layoutFaixa()
			a.invalidateFaixa()
		}
	}

	w.OnMutedChanged = func(m bool) {
		if p != nil && p.mudo != m {
			p.mudo = m

			a.layoutFaixa()
			a.invalidateFaixa()
		}
	}
}

func (a *app) configurarAparenciaWebView(p *pane, w *WebView, mobile bool) {
	if p == a.left {
		w.OnDownloadStarting = func(args *ICoreWebView2DownloadStartingEventArgs) {
			a.iniciarDownload(args)
		}
	}

	w.OnContextAction = func(id, valor string) bool {
		a.depois(func() { a.acaoMenuContexto(id, valor) })

		return true
	}
	w.DisableExternalDrop()
	w.Register()

	if mobile {
		w.SetUserAgent(mobileUA)
	}

	scheme := ColorSchemeDefault
	if a.st.ForceDark {
		scheme = ColorSchemeDark
	}

	w.SetPreferredColorScheme(scheme)
	w.SetReducedMotion(true)
	w.SetSaveData()
	w.SetBlockedFonts()
}

func (a *app) configurarScriptsBaseWebView(p *pane, w *WebView) {
	w.SetZoomFactor(a.paneZoom(p))
	w.AddInitScript(navHelperScript)

	if a.paneUsaDeepSeek(p) {
		w.AddInitScript(idiomaScript())

		if loginAutoLigado() {
			w.AddInitScript(loginScript())
		}
	}

	w.AddInitScript(aplicarJS)
	w.AddInitScript(scrollbarFixJS)
	w.AddInitScript(imgLiteJS)
	w.AddInitScript(sleepLiteJS)
	w.AddInitScript(darkReaderJS)
	w.AddInitScript(forceDarkJS)
	w.AddInitScript(livreCliqueJS)
	w.AddInitScript(domLiteJS)
	w.AddInitScript(semAutoplayJS)
	w.AddInitScript(cookieRejectJS)
	w.AddInitScript(scrollLiteJS)
	w.AddInitScript(senhasLiteJS)
	w.AddInitScript(lojaExtJS)
}

func (a *app) configurarScriptsIAWebView(p *pane, w *WebView) {
	if a.paneUsaChatGPT(p) && os.Getenv("OVERDRIVE_CHATLITE") != "0" {
		w.AddInitScript(chatGPTLiteJS)
	}

	if a.paneUsaIA(p) && !a.paneUsaChatGPT(p) && os.Getenv("OVERDRIVE_AILITE") != "0" {
		w.AddInitScript(aiLiteJS)
	}

	if p != nil && p.mobile {
		w.AddInitScript(anexarImagensJS)
		w.AddInitScript(linksChatJS)
	}

	if p == a.right {
		w.AddInitScript(imgSearchInitJS)
		adicionarInitScripts(w, scriptsDaIA(a.siteAtualIA()))
	}
}

func (a *app) finalizarConfigWebView(p *pane, w *WebView, mobile bool) {
	a.configurarBaseWebView(p, w)
	a.configurarAparenciaWebView(p, w, mobile)
	a.configurarScriptsBaseWebView(p, w)
	a.configurarScriptsIAWebView(p, w)
}

func (a *app) configureWebView(p *pane, mobile bool) {
	w := p.wv
	w.OnTitleChanged = func(t string) {
		a.cuidarTituloWebView(p, t)
	}
	w.OnSourceChanged = func(uri string) {
		a.cuidarOrigemWebView(p, uri)
	}
	w.OnNavigationStarting = func(uri string) {
		a.cuidarInicioNavegacaoWebView(p, w, uri)
	}
	w.OnNavigated = func(ok bool) {
		a.cuidarNavegadoWebView(p, w, mobile, ok)
	}
	w.OnNewWindow = func(uri string, userInitiated bool) bool {
		return a.cuidarNovaJanelaWebView(p, uri, userInitiated)
	}
	w.OnProcessFail = func(kind uint32) {
		a.cuidarFalhaProcessoWebView(p, kind)
	}
	w.OnAccelKey = func(vk uint32, ctrl, shift, alt bool) bool {
		if p == a.left {
			p.ociosoDesde = time.Now()
		}

		return a.handleShortcut(vk, ctrl, shift, alt)
	}
	w.OnGotFocus = func() {
		a.cuidarFocoWebView(p)
	}
	w.OnMessage = func(msg string) {
		a.cuidarMensagemWebView(p, msg)
	}
	a.finalizarConfigWebView(p, w, mobile)
}
