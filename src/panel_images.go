//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const (
	esperaLimpezaImagens     = 1500 * time.Millisecond
	ajusteArredondamentoCSS  = 0.5
	tempoLeituraCabecalhoImg = 10 * time.Second
	tempoEscritaImg          = 30 * time.Second
	esperaDesligarImg        = 2 * time.Second
)

func (a *app) painelImagensPodeAbrir() bool {
	return a.env != nil && a.hwnd != 0 && a.st.RightVisible && a.st.ImgVisible
}

func (a *app) garantirPainelImagens() {
	a.imgInteragido = time.Now()
	if a.imgPanel == nil {
		a.imgPanel = &pane{zoom: 1, mobile: true}
	}
}

// retomarPainelImagensPronto retoma o painel ja pronto; true se tratou.
func (a *app) retomarPainelImagensPronto() bool {
	if !a.imgPanel.ready {
		return false
	}

	if a.imgPanel.descarregado {
		a.recarregarPaneDescarregado(a.imgPanel)
	}

	if a.imgPanel.wv != nil {
		a.imgPanel.wv.Resume()
	}

	return true
}

func (a *app) criarPainelImagensNovo() {
	if err := a.criarWebView(a.imgPanel, false,
		func() bool { return a.st.RightVisible && a.st.ImgVisible },
		func(w *WebView) {
			w.SetUserAgent(mobileUA)
			a.aplicarEmulacaoFixa(a.imgPanel)
			a.aplicarUAImagens(!a.imgPanel.busca)
		},
		func(_ *WebView) {
			a.navegarImagensInicial()
		}); err != nil {
		a.dizerErrof(txtFalhaCriarAba)
	}
}

// navegarImagensInicial escolhe a URL inicial do painel de imagens.
func (a *app) navegarImagensInicial() {
	endereco := strings.TrimSpace(a.imgPanel.ultimaURL)
	if !strings.HasPrefix(endereco, "https://www.google.com/") {
		endereco = imagensHomeURL
		a.imgPanel.busca = false
	}

	a.navegarImagens(endereco, !a.imgPanel.busca)
}

func (a *app) abrirPainelImagens() {
	if !a.painelImagensPodeAbrir() {
		return
	}

	a.garantirPainelImagens()

	if a.retomarPainelImagensPronto() {
		return
	}

	a.criarPainelImagensNovo()
}

func (a *app) fecharPainelImagens() {
	if a.imgPanel == nil {
		return
	}

	a.st.ImgVisible = false

	a.imgLidas = nil
	a.imgErros = 0
	a.imgNonce = ""
	a.destruirPane(a.imgPanel)
	a.agendarLimpezaMemoria(true, esperaLimpezaImagens)
}

func (a *app) pontoNoPainelImagens(x, y int32) bool {
	if a.imgPanel == nil {
		return false
	}

	r := a.imgPanel.rect

	return r.Width() > 0 && r.Height() > 0 && x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *app) cursorNoPainelImagens() bool {
	var ponto struct{ x, y int32 }
	if ret, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&ponto))); ret == 0 {
		return false
	}

	pScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	return a.pontoNoPainelImagens(ponto.x, ponto.y)
}

func idiomaScript() string {
	return idiomaJS
}

func (a *app) abrirSeletorImagens() {
	p := a.left
	if p == nil || !p.ready {
		a.dizerErrof(txtSemPagImgs)
		a.invalidateBar()

		return
	}

	a.imgNonce = fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(a.tabs))
	a.imgLidas = nil
	a.imgErros = 0
	coletor := imagensDaPaginaJS
	coletor = strings.ReplaceAll(coletor, "LIMITE_LISTA", strconv.Itoa(imgMaxLista))
	p.wv.EvalJS(coletor, func(err error, res string) {
		if err != nil {
			return
		}

		var imgs []imagemPagina
		if e := desembrulharEvalJS(res, &imgs); e != nil {
			return
		}

		if len(imgs) == 0 {
			a.depois(func() {
				a.dizerf(txtPagSemImgs)
				a.invalidateBar()
			})

			return
		}

		lista := "[]"
		if blob, e := json.Marshal(imgs); e == nil {
			lista = string(blob)
		}

		script := seletorImagensJS
		script = strings.ReplaceAll(script, "LISTA_AQUI", lista)
		script = strings.ReplaceAll(script, "NONCE_AQUI", a.imgNonce)
		script = strings.ReplaceAll(script, "LIMITE_ANEXAR", strconv.Itoa(imgMaxAnexar))
		alvo := p

		a.depois(func() {
			if alvo != nil && alvo.ready && alvo.wv != nil {
				leitor := leitorImagensJS
				leitor = strings.ReplaceAll(leitor, "LIMITE_PIXELS", strconv.Itoa(imgMaxPixels))
				alvo.wv.ExecuteScript(leitor)
				alvo.wv.ExecuteScript(script)

				a.status = txtEscolhaImgs
				a.invalidateBar()
			}
		})
	})
}

type msgImagens struct {
	T     string `json:"t"`
	N     string `json:"n"`
	Acao  string `json:"acao"`
	I     int    `json:"i"`
	Nome  string `json:"nome"`
	Tipo  string `json:"tipo"`
	Dados string `json:"dataUrl"`
	Msg   string `json:"msg"`
	Src   string `json:"src"`
	OK    int    `json:"ok"`
	Falha int    `json:"falhas"`
	Via   string `json:"via"`
	Total int    `json:"total"`
	CB    int    `json:"cb"`
}

func decodificarMsgImagens(msg string) (msgImagens, bool) {
	var mensagem msgImagens
	if e := json.Unmarshal([]byte(msg), &mensagem); e != nil {
		return mensagem, false
	}

	return mensagem, true
}

func (a *app) nonceImagensConfere(nonce string) bool {
	return nonce != "" && nonce == a.imgNonce
}

// guardarDadoImagem valida e guarda um anexo recebido.
func (a *app) guardarDadoImagem(mensagem msgImagens) {
	if !strings.HasPrefix(mensagem.Dados, "data:image/") {
		a.imgErros++

		return
	}

	if len(a.imgLidas) >= imgMaxAnexar {
		a.imgErros++

		return
	}

	a.imgLidas = append(a.imgLidas, imagemAnexo{Nome: mensagem.Nome, Tipo: mensagem.Tipo, Dados: mensagem.Dados})
	a.status = fmt.Sprintf(txtFmtLidas, quant(len(a.imgLidas), "imagem", idImagens))
	a.invalidateBar()
}

// anunciarAnexoImagens relata o resultado do anexo no chat.
func (a *app) anunciarAnexoImagens(mensagem msgImagens) {
	if mensagem.OK > 0 {
		a.dizerOkf(txtFmtColadas, quant(mensagem.OK, "imagem", idImagens), mensagem.Via)
	} else {
		a.dizerErrof(txtImgsRejeitadas)
	}

	a.invalidateBar()
}

// concluirLeituraImagens envia o lote lido para o chat.
func (a *app) concluirLeituraImagens() {
	n := len(a.imgLidas)
	if n == 0 {
		a.dizerErrof(txtSiteBloqueiaImgs)
		a.invalidateBar()

		return
	}

	itens := a.imgLidas
	a.imgLidas = nil
	a.depois(func() { a.anexarImagensNoChat(itens) })
}

// aplicarAcaoImagens despacha a acao da mensagem do coletor.
func (a *app) aplicarAcaoImagens(mensagem msgImagens) {
	switch mensagem.Acao {
	case "vazio":
		a.status = txtPagSemImgs
		a.invalidateBar()
	case "dados":
		a.guardarDadoImagem(mensagem)
	case "erro":
		a.imgErros++
	case "anexado":
		a.anunciarAnexoImagens(mensagem)
	case "fim":
		a.concluirLeituraImagens()
	}
}

func (a *app) tratarMsgImagens(msg string) bool {
	if !strings.Contains(msg, `"t":"gbimg"`) {
		return false
	}

	mensagem, decodificou := decodificarMsgImagens(msg)
	if !decodificou {
		return true
	}

	if !a.nonceImagensConfere(mensagem.N) {
		return true
	}

	a.aplicarAcaoImagens(mensagem)

	return true
}

// anunciarRetornoAnexo mostra o resultado do anexo no painel.
func (a *app) anunciarRetornoAnexo(p *pane, retorno string) {
	switch {
	case strings.HasPrefix(retorno, "agendado:"):
		a.anunciarAnexoAgendado(p, retorno)
	case retorno == "sem-campo":
		a.dizerErrof(txtSemCampoAnexo)
	case retorno == "vazio":
		a.dizerf(txtNenhumaImg)
	default:
		a.dizerErrof(txtFmtNaoAnexei, retorno)
	}

	a.invalidateBar()
}

// anunciarAnexoAgendado relata o agendamento do anexo.
func (a *app) anunciarAnexoAgendado(p *pane, retorno string) {
	qtd := strings.TrimPrefix(retorno, "agendado:")
	if n, err := strconv.Atoi(qtd); err == nil {
		qtd = quant(n, "imagem", idImagens)
	}

	a.status = fmt.Sprintf(txtFmtColando, qtd, a.paneName(p))
}

func (a *app) anexarImagensEmPainel(p *pane, itens []imagemAnexo) {
	if p == nil || !p.ready || p.wv == nil || len(itens) == 0 {
		return
	}

	corpo, err := json.Marshal(itens)
	if err != nil {
		return
	}

	jsAnexar := fmt.Sprintf("window.__gbAnexar && window.__gbAnexar(%s, %q)", string(corpo), a.imgNonce)
	p.wv.EvalJS(jsAnexar, func(_ error, res string) {
		retorno := strings.Trim(strings.TrimSpace(res), `"`)

		a.depois(func() { a.anunciarRetornoAnexo(p, retorno) })
	})
}

func (a *app) toggleImagens() {
	if !a.st.RightVisible {
		a.dizerf(txtAIPEsquerda)

		return
	}

	a.st.ImgVisible = !a.st.ImgVisible
	if a.st.ImgVisible {
		a.abrirPainelImagens()
	} else {
		a.fecharPainelImagens()
	}

	a.layout()
	a.persist()
}

func (a *app) applyMobile(p *pane) {
	if p == nil || !p.ready {
		return
	}

	a.aplicarEmulacaoFixa(p)
	a.aplicarTamanhoChat(p)
}

func (a *app) aplicarEmulacaoFixa(p *pane) {
	if p == nil || !p.ready || p.emulFixa {
		return
	}

	p.emulFixa = true
	p.wv.CallDevTools("Emulation.setUserAgentOverride", uaImagensJSON(true))
	p.wv.CallDevTools("Emulation.setLocaleOverride", `{"locale":"pt-BR"}`)
	p.wv.CallDevTools("Emulation.setTouchEmulationEnabled", `{"enabled":true,"maxTouchPoints":5}`)
}

func uaImagensJSON(mobile bool) string {
	meta := `"userAgentMetadata":{"brands":[{"brand":"Chromium","version":"` + versaoChrome +
		`"},{"brand":"Google Chrome","version":"` + versaoChrome +
		`"}],"fullVersionList":[{"brand":"Chromium","version":"` + versaoChrome +
		`.0.0.0"},{"brand":"Google Chrome","version":"` + versaoChrome +
		`.0.0.0"}],"fullVersion":"` + versaoChrome + `.0.0.0",`
	if mobile {
		return fmt.Sprintf(`{"userAgent":%q,"platform":"Android",`+
			`"acceptLanguage":"pt-BR,pt;q=0.9,en;q=0.8",%s"platform":"Android",`+
			`"platformVersion":"14.0.0","architecture":"","model":"Pixel 8",`+
			`"mobile":true,"bitness":"","wow64":false}}`, mobileUA, meta)
	}

	return fmt.Sprintf(`{"userAgent":%q,"platform":"Win32",`+
		`"acceptLanguage":"pt-BR,pt;q=0.9,en;q=0.8",%s"platform":"Windows",`+
		`"platformVersion":"10.0.0","architecture":"x86","model":"",`+
		`"mobile":false,"bitness":"64","wow64":false}}`, desktopUA, meta)
}

func (a *app) aplicarUAImagens(mobile bool) {
	p := a.imgPanel
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	p.wv.CallDevTools("Emulation.setUserAgentOverride", uaImagensJSON(mobile))
}

func (a *app) navegarImagens(endereco string, mobile bool) {
	p := a.imgPanel
	if p == nil {
		return
	}

	a.imgInteragido = time.Now()
	p.busca = !mobile
	p.ultimaURL = endereco
	p.frio = false
	p.frioManual = false
	p.ociosoDesde = time.Time{}
	p.descarregado = false
	p.urlDescarga = ""
	p.scrollDescarga = 0

	p.scrollRestaurar = 0
	if p.ready && p.wv != nil {
		p.wv.SetFrozen(false)
	}

	if !p.ready || p.wv == nil {
		return
	}

	a.aplicarUAImagens(mobile)
	a.depois(func() {
		if a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil {
			a.imgPanel.wv.Navigate(endereco)
		}
	})
}

func dimensoesViewport(largura, altura int32, zoom float64) (int32, int32) {
	if zoom <= 0 {
		zoom = 1
	}

	cssW := int32(float64(largura)/zoom + ajusteArredondamentoCSS)
	cssH := int32(float64(altura)/zoom + ajusteArredondamentoCSS)

	if cssW < 1 {
		cssW = 1
	}

	if cssH < 1 {
		cssH = 1
	}

	return cssW, cssH
}

func metricsViewportJSON(largura, altura int32, mobile bool, scale float64) string {
	if scale <= 0 {
		scale = 1
	}

	return fmt.Sprintf(`{"width":%d,"height":%d,"deviceScaleFactor":1,"mobile":%t,"scale":%s}`,
		largura, altura, mobile, strconv.FormatFloat(scale, 'f', 4, 64))
}

func aplicarViewport(p *pane, largura, altura int32, zoom float64, mobile bool) bool {
	cssW, cssH := dimensoesViewport(largura, altura, zoom)
	if cssW == p.cssW && cssH == p.cssH && zoom == p.zoom {
		return false
	}

	p.cssW, p.cssH, p.zoom = cssW, cssH, zoom
	p.wv.CallDevTools("Emulation.setDeviceMetricsOverride", metricsViewportJSON(cssW, cssH, mobile, zoom))

	return true
}

func (a *app) aplicarTamanhoImagens() {
	p := a.imgPanel
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	largura, altura := p.rect.Width(), p.rect.Height()
	if largura <= 0 || altura <= 0 {
		return
	}

	if !p.busca {
		if mudou := aplicarViewport(p, largura, altura, 1, true); !mudou {
			return
		}

		p.wv.SetZoomFactor(1)

		return
	}

	zoom := 1.0

	mudou := aplicarViewport(p, largura, altura, zoom, false)
	if !mudou {
		return
	}

	p.wv.SetZoomFactor(zoom)

	if densidade := 1.0; densidade != p.zoomLogged {
		p.zoomLogged = densidade
	}
}

func enderecoDoProxy() string {
	serverMu.Lock()
	defer serverMu.Unlock()

	if serverPort == 0 {
		return ""
	}

	return fmt.Sprintf("http://127.0.0.1:%d", serverPort)
}

func portaTCP(ouvinte net.Listener) int {
	// ouvinte sempre vem de net.Listen("tcp", ...): Addr e *net.TCPAddr por construcao.
	endereco, convertido := ouvinte.Addr().(*net.TCPAddr)
	if !convertido || endereco == nil {
		return 0
	}

	return endereco.Port
}

func iniciarServidorImagens() int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if serverPort != 0 {
		return serverPort
	}

	//exhaustruct:ignore // ListenConfig zerado: ouvinte local efemero, padroes do Go.
	ouvinte, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0") // Interno.
	if err != nil {
		return 0
	}

	mux := http.NewServeMux()
	utubeServir(mux)
	mux.HandleFunc("/ping", func(escritor http.ResponseWriter, _ *http.Request) {
		_, _ = escritor.Write([]byte("ok"))
	})
	//exhaustruct:ignore // Server local de apoio: so handler + timeouts; resto padrao do Go.
	srv := &http.Server{
		Handler:           somenteLocal(mux, portaTCP(ouvinte)),
		ReadHeaderTimeout: tempoLeituraCabecalhoImg,
		WriteTimeout:      tempoEscritaImg,
	}
	serverHTTP = srv

	serverPort = portaTCP(ouvinte)
	go func() {
		// Servidor local de apoio: se cair, as pecas que o usam falham sozinhas; sem acao util aqui.
		_ = srv.Serve(ouvinte)
	}()

	return serverPort
}

func somenteLocal(prox http.Handler, porta int) http.Handler {
	esperado := "127.0.0.1:" + strconv.Itoa(porta)

	return http.HandlerFunc(func(escritor http.ResponseWriter, pedido *http.Request) {
		host := strings.ToLower(strings.TrimSpace(pedido.Host))
		if host != esperado && host != "localhost:"+strconv.Itoa(porta) && host != "[::1]:"+strconv.Itoa(porta) {
			http.Error(escritor, "origem nao permitida", http.StatusForbidden)

			return
		}

		prox.ServeHTTP(escritor, pedido)
	})
}

func fecharServidorImagens() {
	serverMu.Lock()
	srv := serverHTTP
	serverHTTP = nil
	serverPort = 0
	serverMu.Unlock()

	if srv != nil {
		ctx, cancelar := context.WithTimeout(context.Background(), esperaDesligarImg)
		defer cancelar()

		_ = srv.Shutdown(ctx)
	}
}
