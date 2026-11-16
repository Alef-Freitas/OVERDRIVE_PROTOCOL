//go:build windows

package main

import (
	"strings"
	"unsafe"
)

//odlint:somente-leitura tabela estatica, escrita nunca.
//nolint:gochecknoglobals // Tabela de hosts bloqueados: somente leitura.
var hostsPreBloqueados = map[string]struct{}{
	"doubleclick.net": {}, "googlesyndication.com": {}, "googleadservices.com": {},
	"amazon-adsystem.com": {}, "criteo.com": {}, "criteo.net": {},
	"outbrain.com": {}, "taboola.com": {}, "taboolasyndication.com": {},
	"revcontent.com": {}, "mgid.com": {}, "adnxs.com": {}, "adsrvr.org": {},
	"rubiconproject.com": {}, "pubmatic.com": {}, "openx.net": {},
	"moatads.com": {}, "bidswitch.net": {}, "smartadserver.com": {},
	"adform.com": {}, "adform.net": {}, "flashtalking.com": {},
	"serving-sys.com": {}, "2mdn.net": {}, "ads.yahoo.com": {},
	"advertising.com": {}, "mathtag.com": {}, "bat.bing.com": {},
	"ads.linkedin.com": {}, "ads.pinterest.com": {}, "ads.snapchat.com": {},
	"ads.x.com": {}, "ads.facebook.com": {}, "ads.reddit.com": {},
	"ads.tiktok.com": {},

	"scorecardresearch.com": {}, "quantserve.com": {}, "statcounter.com": {},
	"clicky.com": {}, "crazyegg.com": {}, "mouseflow.com": {},
	"fullstory.com": {}, "hotjar.com": {}, "hotjar.io": {},
	"mixpanel.com": {}, "amplitude.com": {}, "heap.io": {},
	"heapanalytics.com": {}, "segment.com": {}, "segment.io": {},
	"optimizely.com": {}, "abtasty.com": {}, "hm.baidu.com": {},
	"mc.yandex.ru": {}, "an.yandex.ru": {}, "bam.nr-data.net": {},
	"js-agent.newrelic.com": {},

	"connect.facebook.net": {}, "analytics.twitter.com": {},
	"static.ads-twitter.com": {}, "analytics.tiktok.com": {},
	"ct.pinterest.com": {}, "analytics.pinterest.com": {}, "s.pinimg.com": {},
	"px.ads.linkedin.com": {}, "snap.licdn.com": {}, "sc-static.net": {},
	"tr.snapchat.com": {}, "events.redditmedia.com": {},

	"fingerprint.com": {}, "cdn.polyfill.io": {}, "polyfill.io": {},
}

//nolint:gochecknoglobals // GIF estatico de resposta: somente leitura.
var gifPreBloqueio = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00,
	0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
	0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
}

func restoDeURI(uri string) (string, bool) {
	switch {
	case strings.HasPrefix(uri, "http://"):
		return uri[len("http://"):], true
	case strings.HasPrefix(uri, "https://"):
		return uri[len("https://"):], true
	default:
		return "", false
	}
}

func fimDeAutoridadeURI(resto string) int {
	for i := range len(resto) {
		if c := resto[i]; c == '/' || c == '?' || c == '#' {
			return i
		}
	}

	return len(resto)
}

func separarPortaHost(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}

	return strings.TrimSuffix(host, ".")
}

func extrairHostDeURI(uri string) (string, bool) {
	resto, ok := restoDeURI(uri)
	if !ok {
		return "", false
	}

	host := resto[:fimDeAutoridadeURI(resto)]
	if host == "" || host[0] == '[' {
		return "", false
	}

	host = separarPortaHost(host)

	return host, host != ""
}

func normalizarHostMinusculo(host string) string {
	if asciiMinusculo(host) {
		return host
	}

	letras := []byte(host)
	for i := range letras {
		if letras[i] >= 'A' && letras[i] <= 'Z' {
			letras[i] += 'a' - 'A'
		}
	}

	return string(letras)
}

func hostNaListaBloqueio(host string) bool {
	for {
		if _, ok := hostsPreBloqueados[host]; ok {
			return true
		}

		i := strings.IndexByte(host, '.')
		if i < 0 {
			return false
		}

		host = host[i+1:]
	}
}

func hostPreBloqueado(uri string) bool {
	host, ok := extrairHostDeURI(uri)
	if !ok {
		return false
	}

	return hostNaListaBloqueio(normalizarHostMinusculo(host))
}

func asciiMinusculo(s string) bool {
	for i := range len(s) {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			return false
		}
	}

	return true
}

func registrarFiltrosDeHosts(w *WebView, vistos map[string]bool) {
	for host := range hostsPreBloqueados {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" || vistos[host] {
			continue
		}

		vistos[host] = true
		for _, f := range []string{"*://" + host + "/*", "*://*." + host + "/*"} {
			filtro := utf16Ptr(f)
			w.view.vtbl.AddWebResourceRequestedFilter.Call(
				uintptr(unsafe.Pointer(w.view)),
				uintptr(unsafe.Pointer(filtro)), 0)
		}
	}
}

func responderBloqueioPreFiltro(w *WebView, args *ICoreWebView2WebResourceRequestedEventArgs, ctx uint32, uri string) {
	if !hostPreBloqueado(uri) {
		return
	}
	// Contextos WebView2: 0=Document, 1=Stylesheet, 2=Image, 3=Media.
	// Imagem recebe o GIF 1x1 (placeholder); midia e o resto recebem
	// 204 para nao injetar bytes invalidos no player.
	const (
		contextoImagemPreFiltro        = 2
		httpStatusOKPreFiltro          = 200
		httpStatusSemConteudoPreFiltro = 204
	)

	if ctx == contextoImagemPreFiltro {
		w.responderRequisicao(args, gifPreBloqueio, httpStatusOKPreFiltro, "OK",
			"Content-Type: image/gif\r\nCache-Control: max-age=31536000\r\n")
	} else {
		w.responderRequisicao(args, nil, httpStatusSemConteudoPreFiltro, "No Content", "Content-Type: text/plain\r\n")
	}
}

func instalarPreFiltro(w *WebView) {
	if w == nil || w.view == nil {
		return
	}
	// Filtros por host em vez de "*": o callback em Go so e chamado para
	// requisicoes a hosts de anuncios/rastreamento, nao para toda a
	// navegacao normal. Duas formas cobrem o host exato e subdominios.
	vistos := map[string]bool{}

	registrarFiltrosDeHosts(w, vistos)

	manipuladorRecursos := newWebResourceRequestedHandler(
		func(_ *ICoreWebView2, args *ICoreWebView2WebResourceRequestedEventArgs) uintptr {
			var ctx uint32
			args.vtbl.GetResourceContext.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&ctx)))

			var req *ICoreWebView2WebResourceRequest

			hr, _, _ := args.vtbl.GetRequest.Call(uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&req)))
			if Failed(hr) || req == nil {
				return SOK
			}

			var up *uint16
			req.vtbl.GetURI.Call(uintptr(unsafe.Pointer(req)), uintptr(unsafe.Pointer(&up)))
			uri := takeString(up)

			req.vtbl.Release.Call(uintptr(unsafe.Pointer(req)))

			responderBloqueioPreFiltro(w, args, ctx, uri)

			return SOK
		})
	pin(manipuladorRecursos)

	tok := &eventToken{}
	w.view.vtbl.AddWebResourceRequested.Call(
		uintptr(unsafe.Pointer(w.view)),
		uintptr(unsafe.Pointer(manipuladorRecursos)),
		uintptr(unsafe.Pointer(tok)))

	manipuladorRecursos.vtbl.Release.Call(uintptr(unsafe.Pointer(manipuladorRecursos)))
}
