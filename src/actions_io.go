//go:build windows

// Package main implementa o navegador WEB Overdrive (interface, abas,
// downloads, midia, utilitarios e integracao com o sistema).
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func pastaSaidas() string {
	dir := filepath.Join(dataDir, "saidas")
	// GOD: diretorios internos do perfil (dataDir 0700); sem acesso de terceiros por desenho.
	_ = os.MkdirAll(dir, 0o700)

	return dir
}

func (a *app) imprimirPDF() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerErrof(txtSemPagImprimir)
		a.invalidateBar()

		return
	}

	a.dizerf(txtGerandoPDF)
	a.invalidateBar()

	falha := func() {
		a.depois(func() {
			a.dizerErrof(txtNaoGerouPDF)
			a.invalidateBar()
		})
	}

	p.wv.CallDevToolsResult("Page.printToPDF", `{"printBackground":true}`, func(err error, res string) {
		if err != nil {
			falha()

			return
		}

		var parsed struct {
			Data string `json:"data"`
		}
		if e := json.Unmarshal([]byte(res), &parsed); e != nil || parsed.Data == "" {
			falha()

			return
		}

		raw, e := base64.StdEncoding.DecodeString(parsed.Data)
		if e != nil {
			falha()

			return
		}

		nome := fmt.Sprintf("pagina-%s.pdf", time.Now().Format("20060102-150405"))

		a.depois(func() {
			a.salvarPDFGerado(nome, raw)
		})
	})
}

// salvarPDFGerado pede o destino ao usuario e grava o PDF gerado.
func (a *app) salvarPDFGerado(nome string, raw []byte) {
	caminho, ok := pedirCaminhoSalvar(a.hwnd, txtTituloSalvarPDF, nome,
		"Documentos PDF (*.pdf)\x00*.pdf\x00Todos os arquivos (*.*)\x00*.*\x00\x00", "pdf", pastaSaidas())
	if !ok {
		a.dizerf(txtPDFCancelado)
		a.invalidateBar()

		return
	}
	// GOD: caminho escolhido pelo usuario no dialogo Salvar; saida do proprio usuario.
	if e := os.WriteFile(caminho, raw, 0o644); e != nil { // #nosec G306
		a.dizerErrof(txtNaoSalvouPDF)
		a.invalidateBar()

		return
	}

	a.dizerOkf(txtFmtPDFSalvo, filepath.Base(caminho))
	a.invalidateBar()
}

// salvarCompleto abre o dialogo nativo de salvar pagina completa.
func (a *app) salvarCompleto() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerErrof(txtSemPagSalvar)
		a.invalidateBar()

		return
	}

	if !p.wv.MostrarSalvarComo() {
		a.dizerErrof(txtNaoSalvouPag)
		a.invalidateBar()
	}
}

// salvarArquivoUnico gera um HTML unico autocontido com o SingleFile.
func (a *app) salvarArquivoUnico() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerErrof(txtSemPagSalvar)
		a.invalidateBar()

		return
	}

	if singleFileJS == "" {
		a.salvarArquivoUnicoCaiu("")
		a.invalidateBar()

		return
	}

	url := p.wv.Source()

	a.dizerf(txtSalvandoUnico)
	a.invalidateBar()
	p.wv.EvalJS(expressaoInjecaoSingleFile(), func(err error, res string) {
		a.salvarArquivoUnicoMotor(url, res, err)
	})
}

// motorSingleFileJS devolve o bundle pronto para script classico: troca
// import.meta.url (invalido fora de modulo) por document.baseURI.
func motorSingleFileJS() string {
	return strings.ReplaceAll(singleFileJS, "import.meta.url", "document.baseURI")
}

// expressaoInjecaoSingleFile injeta o motor e devolve diagnostico em JSON.
func expressaoInjecaoSingleFile() string {
	return `(()=>{try{` + "\n" + motorSingleFileJS() + "\n" +
		`}catch(e){return JSON.stringify({ok:false,err:String(e&&e.stack||e)});}
if(typeof globalThis.singlefile==="undefined"){return JSON.stringify({ok:false,err:"sem global"});}
return JSON.stringify({ok:true});})()`
}

// injecaoSingleFileOK desembrulha o resultado do ExecuteScript (string JSON)
// e diz se o motor ficou disponivel. Devolve tambem o motivo da falha.
func injecaoSingleFileOK(res string) (bool, string) {
	var texto string
	if err := json.Unmarshal([]byte(res), &texto); err == nil {
		res = texto
	}

	var resposta struct {
		OK  bool   `json:"ok"`
		Err string `json:"err"`
	}
	if err := json.Unmarshal([]byte(res), &resposta); err != nil {
		return false, res
	}

	return resposta.OK, resposta.Err
}

// salvarArquivoUnicoMotor chama o motor ja injetado via CDP (awaitPromise).
func (a *app) salvarArquivoUnicoMotor(url, injecao string, err error) {
	p := a.left
	if err != nil || p == nil || !p.ready || p.wv == nil {
		motivo := injecao
		if err != nil {
			motivo = err.Error()
		}

		a.salvarArquivoUnicoCaiu(motivo)
		a.invalidateBar()

		return
	}

	if ok, motivo := injecaoSingleFileOK(injecao); !ok {
		if motivo == "" {
			motivo = "injecao sem motor"
		}

		a.salvarArquivoUnicoCaiu(motivo)
		a.invalidateBar()

		return
	}

	avaliarArquivoUnico(p, invocacaoArquivoUnico(), func(err error, res string) {
		a.salvarArquivoUnicoResposta(url, err, res)
	})
}

// salvarArquivoUnicoMagra tenta de novo enxuto antes de cair no dialogo.
func (a *app) salvarArquivoUnicoMagra(url, motivo string) {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.salvarArquivoUnicoCaiu(motivo)

		return
	}

	avaliarArquivoUnico(p, invocacaoArquivoUnicoMagra(), func(err error, res string) {
		a.salvarArquivoUnicoRespostaMagra(url, err, res, motivo)
	})
}

// salvarArquivoUnicoRespostaMagra grava a versao enxuta ou cai no dialogo.
func (a *app) salvarArquivoUnicoRespostaMagra(url string, err error, res, motivo string) {
	if err == nil {
		motivoCDP, html := respostaArquivoUnicoCDP(err, res)
		if html != "" {
			a.salvarArquivoUnicoConteudo(url, html)

			return
		}

		if motivoCDP != "" {
			motivo = motivoCDP
		}
	}

	a.salvarArquivoUnicoCaiu(motivo)
}

// salvarArquivoUnicoCaiu abre o dialogo nativo quando o motor falha;
// sem dialogo, mostra o motivo na barra.
func (a *app) salvarArquivoUnicoCaiu(motivo string) {
	if a.salvarArquivoUnicoAlternativo() {
		a.avisarFalhaSalvar(motivo)

		return
	}

	a.avisarFalhaSalvar(motivo)
}

// salvarArquivoUnicoAlternativo abre o dialogo nativo quando o motor falha.
func (a *app) salvarArquivoUnicoAlternativo() bool {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		return false
	}

	return p.wv.MostrarSalvarComo()
}

// salvarArquivoUnicoResposta grava o HTML gerado pelo SingleFile.
func (a *app) salvarArquivoUnicoResposta(url string, err error, res string) {
	motivo, html := respostaArquivoUnicoCDP(err, res)
	if html == "" {
		a.salvarArquivoUnicoMagra(url, motivo)
		a.invalidateBar()

		return
	}

	a.salvarArquivoUnicoConteudo(url, html)
}

// salvarArquivoUnicoConteudo grava o HTML com o dialogo de destino.
func (a *app) salvarArquivoUnicoConteudo(url, html string) {
	nome := fmt.Sprintf("pagina-unica-%s.html", time.Now().Format("20060102-150405"))
	cab := "<!-- salvo pelo Overdrive em " + time.Now().Format("02/01/2006 15:04") + " -->\n" +
		"<!-- origem: " + url + " -->\n"

	a.depois(func() {
		caminho, ok := pedirCaminhoSalvar(a.hwnd, txtTituloSalvarPag, nome,
			"Página HTML (*.html)\x00*.html\x00Todos os arquivos (*.*)\x00*.*\x00\x00", "html", pastaSaidas())
		if !ok {
			a.dizerf(txtSalvarCancelado)
			a.invalidateBar()

			return
		}
		// GOD: caminho escolhido pelo usuario no dialogo Salvar; saida do proprio usuario.
		if e := os.WriteFile(caminho, []byte(cab+html), 0o644); e != nil { // #nosec G306
			a.dizerErrof(txtNaoSalvouPag)
			a.invalidateBar()

			return
		}

		a.dizerOkf(txtFmtPagSalva, filepath.Base(caminho))
		a.invalidateBar()
	})
}

// resumirErroSalvar limita o motivo a uma linha curta para a barra.
func resumirErroSalvar(motivo string) string {
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		return "falha desconhecida"
	}

	if indice := strings.Index(motivo, "\n"); indice >= 0 {
		motivo = motivo[:indice]
	}

	const limiteMotivoSalvar = 120

	if len(motivo) > limiteMotivoSalvar {
		motivo = motivo[:limiteMotivoSalvar] + "..."
	}

	return motivo
}

// invocacaoArquivoUnico chama o motor SingleFile ja injetado acima.
// Retorna um objeto (nao string) para o CDP com returnByValue.
func invocacaoArquivoUnico() string {
	return `(async()=>{try{
if(typeof singlefile==="undefined"||!singlefile.getPageData){return {ok:false,err:"motor ausente"};}
const d=await singlefile.getPageData({removeFrames:true});
if(!d||!d.content){return {ok:false,err:"conteudo vazio"};}
return {ok:true,html:d.content};
}catch(e){return {ok:false,err:String(e&&e.message||e)};}})()`
}

// invocacaoArquivoUnicoMagra repete sem ocultos e com teto de 2MB por recurso.
func invocacaoArquivoUnicoMagra() string {
	return `(async()=>{try{
if(typeof singlefile==="undefined"||!singlefile.getPageData){return {ok:false,err:"motor ausente"};}
const d=await singlefile.getPageData({removeFrames:true,removeHiddenElements:true,
maxResourceSizeEnabled:true,maxResourceSize:2});
if(!d||!d.content){return {ok:false,err:"conteudo vazio"};}
return {ok:true,html:d.content};
}catch(e){return {ok:false,err:String(e&&e.message||e)};}})()`
}

// avaliarArquivoUnico roda a expressao assincrona via CDP, que espera a
// Promise; EvalJS nao espera e sempre devolvia resultado vazio.
func avaliarArquivoUnico(p *pane, expressao string, retorno func(error, string)) {
	params := `{"expression":` + cdpString(expressao) + `,"awaitPromise":true,"returnByValue":true}`
	p.wv.CallDevToolsResult("Runtime.evaluate", params, retorno)
}

// cdpString codifica uma string para JSON sem depender de html.EscapeString.
func cdpString(texto string) string {
	codificado, err := json.Marshal(texto)
	if err != nil {
		return `""`
	}

	return string(codificado)
}

// respostaArquivoUnicoCDP extrai o objeto do resultado do Runtime.evaluate.
func respostaArquivoUnicoCDP(err error, res string) (string, string) {
	if err != nil {
		return "execucao", ""
	}

	var cdp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if e := json.Unmarshal([]byte(res), &cdp); e != nil {
		return "resposta-cdp", ""
	}

	if cdp.Exception.Text != "" || cdp.Exception.Exception.Description != "" {
		motivo := cdp.Exception.Exception.Description
		if motivo == "" {
			motivo = cdp.Exception.Text
		}

		return "cdp: " + resumirErroSalvar(motivo), ""
	}

	var resposta struct {
		OK   bool   `json:"ok"`
		Err  string `json:"err"`
		HTML string `json:"html"`
	}
	if e := json.Unmarshal(cdp.Result.Value, &resposta); e != nil {
		return "resposta", ""
	}

	if !resposta.OK || resposta.HTML == "" {
		return resposta.Err, ""
	}

	return "", resposta.HTML
}

func (a *app) salvarPagina() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerErrof(txtSemPagSalvar)
		a.invalidateBar()

		return
	}

	url := p.wv.Source()
	p.wv.EvalJS(`(function(){
  try { return {ok:true, html:document.documentElement.outerHTML}; }
  catch (e) { return {ok:false, err:String(e)}; }
})()`, func(err error, res string) {
		if err != nil {
			return
		}

		var resposta struct {
			OK   bool   `json:"ok"`
			Err  string `json:"err"`
			HTML string `json:"html"`
		}
		if e := json.Unmarshal([]byte(res), &resposta); e != nil || !resposta.OK || resposta.HTML == "" {
			return
		}

		nome := fmt.Sprintf("pagina-%s.html", time.Now().Format("20060102-150405"))
		cab := "<!-- salvo pelo Overdrive em " + time.Now().Format("02/01/2006 15:04") + " -->\n" +
			"<!-- origem: " + url + " -->\n"

		a.depois(func() {
			caminho, ok := pedirCaminhoSalvar(a.hwnd, txtTituloSalvarPag, nome,
				"Página HTML (*.html)\x00*.html\x00Todos os arquivos (*.*)\x00*.*\x00\x00", "html", pastaSaidas())
			if !ok {
				a.dizerf(txtSalvarCancelado)
				a.invalidateBar()

				return
			}
			// GOD: caminho escolhido pelo usuario no dialogo Salvar; saida do proprio usuario.
			if e := os.WriteFile(caminho, []byte(cab+resposta.HTML), 0o644); e != nil { // #nosec G306
				a.dizerErrof(txtNaoSalvouPag)
				a.invalidateBar()

				return
			}

			a.dizerOkf(txtFmtPagSalva, filepath.Base(caminho))
			a.invalidateBar()
		})
	})
}

func (a *app) selecionarTudo() {
	if p := a.left; p != nil && p.ready && p.wv != nil {
		p.wv.ExecuteScript(`(function(){
  try {
    var s = window.getSelection();
    s.removeAllRanges();
    var r = document.createRange();
    r.selectNodeContents(document.body);
    s.addRange(r);
  } catch (e) {}
})()`)
	}
}

func (a *app) copiarEndereco() {
	endereco := ""
	if p := a.left; p != nil && p.ready && p.wv != nil {
		endereco = p.wv.Source()
	}

	switch {
	case endereco == "":
		a.dizerErrof(txtSemEndCopiar)
	case copiarTexto(endereco):
		a.dizerOkf(txtFmtEndCopiado, endereco)
	default:
		a.dizerErrof(txtNaoCopiouEnd)
	}

	a.invalidateBar()
}

func (a *app) recarregarFrio() {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtPagAbrindo)

		return
	}

	if p.readerAtivo {
		a.sairDoLeitor(p)

		return
	}

	p.wv.CallDevTools("Network.clearBrowserCache", "{}")
	p.wv.Reload()
	a.dizerf(txtRecSemCache)
}

func (a *app) recarregarSemOtimizacoes() {
	p := a.focused()
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtNenhumaPagRec)

		return
	}

	if p.readerAtivo {
		a.sairDoLeitor(p)

		return
	}

	if p.frio {
		a.retomarPaneGenerico(p)
	}

	if p.descarregado {
		a.recarregarPaneDescarregado(p)

		return
	}

	jsLiteOff := `(function(){try{sessionStorage.setItem('__od_lite_off__','1');return 'ok';}catch(e){return 'erro';}})()`
	p.wv.EvalJS(jsLiteOff, func(error, string) {
		a.depois(func() {
			if p.ready && p.wv != nil {
				p.wv.Reload()
			}
		})
	})
	a.dizerf(txtRecSemOtim)
	a.invalidateBar()
}

func (a *app) recarregarSemJS() {
	p := a.focused()
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtNenhumaPagRec)

		return
	}

	if p.readerAtivo {
		a.sairDoLeitor(p)

		return
	}

	if p.frio {
		a.retomarPaneGenerico(p)
	}

	if p.descarregado {
		a.recarregarPaneDescarregado(p)

		return
	}

	p.nojsPendente = true
	p.nojsCarregando = true
	p.nojsAtivo = false

	p.wv.CallDevToolsResult("Emulation.setScriptExecutionDisabled", `{"value":true}`, func(error, string) {
		a.depois(func() {
			if p.ready && p.wv != nil && p.nojsPendente {
				p.wv.Reload()
			}
		})
	})
	a.agendarReversaoNoJS(p)
	a.agendarAtivacaoNoJS(p)
	a.dizerf(txtRecSemJS)
	a.invalidateBar()
}

// agendarReversaoNoJS desliga o modo sem-JS apos o atraso se a pagina ainda estiver carregando.
func (a *app) agendarReversaoNoJS(p *pane) {
	const atrasoReversaoNoJSSegundos = 6
	time.AfterFunc(atrasoReversaoNoJSSegundos*time.Second, func() {
		a.depois(func() {
			if p.nojsCarregando {
				p.nojsCarregando = false
				p.nojsPendente = false

				p.nojsAtivo = false
				if p.ready && p.wv != nil {
					p.wv.SetScriptsEnabled(true)
				}
			}
		})
	})
}

// agendarAtivacaoNoJS confirma o modo sem-JS apos o atraso se o pedido continuar pendente.
func (a *app) agendarAtivacaoNoJS(p *pane) {
	const atrasoAtivacaoNoJSSegundos = 20
	time.AfterFunc(atrasoAtivacaoNoJSSegundos*time.Second, func() {
		a.depois(func() {
			if p.nojsPendente {
				p.nojsPendente = false
				p.nojsAtivo = true
			}
		})
	})
}

func (a *app) recarregarPaineisDireita() {
	if a.leituraAtiva || a.envioAtivo {
		a.dizerf(txtAguardeEnvio)

		return
	}

	a.retomarChatGPTFrio()

	num := 0

	for _, p := range []*pane{a.right, a.imgPanel} {
		if p == nil || !p.ready || p.wv == nil {
			continue
		}

		p.frio = false
		p.frioManual = false
		p.ociosoDesde = time.Time{}
		p.wv.SetFrozen(false)

		if p.wv.IsSuspended() {
			p.wv.Resume()
		}

		p.wv.Reload()

		num++
	}

	if num > 0 {
		a.dizerf(txtFmtRecDir, quant(num, "painel", "painéis"))
	} else {
		a.dizerf(txtNenhumPainelRec)
	}

	a.invalidateBar()
}

func (a *app) recarregarPaneDescarregado(p *pane) {
	if p == nil || !p.descarregado || p.wv == nil {
		return
	}

	endereco := p.urlDescarga
	p.descarregado = false
	p.scrollRestaurar = p.scrollDescarga
	p.scrollDescarga = 0
	p.urlDescarga = ""

	if endereco == "" {
		return
	}

	p.wv.Navigate(endereco)
}

func (a *app) abrirPasta(caminho string) {
	pShellExecuteW.Call(a.hwnd,
		uintptr(unsafe.Pointer(utf16("open"))),
		uintptr(unsafe.Pointer(utf16(caminho))),
		0, 0, swShow)
}

func pastaDownloads() string {
	if p := pastaDownloadsConhecida(); p != "" {
		return p
	}

	casa, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	dir := filepath.Join(casa, "Downloads")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}

	return dir
}

// pastaDownloadsConhecida usa FOLDERID_Downloads: respeita pastas
// relocadas/localizadas em vez de chutar <home>\Downloads.
func pastaDownloadsConhecida() string {
	const folderidDownloads = "{374DE290-123F-4565-9164-39C4925E467B}"

	guid, err := windows.GUIDFromString(folderidDownloads)
	if err != nil {
		return ""
	}

	var ponteiroCaminho *uint16

	r, _, _ := pSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(&guid)), 0, 0, uintptr(unsafe.Pointer(&ponteiroCaminho)))
	if r != 0 || ponteiroCaminho == nil {
		return ""
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(ponteiroCaminho)))

	s := windows.UTF16PtrToString(ponteiroCaminho)
	if _, err := os.Stat(s); err != nil {
		return ""
	}

	return s
}

// clipAutoLimpar espelha st.LimparClip para o caminho sem app (copiarTexto
// e chamada de varios lugares sem acesso ao app). Padrao desligado: limpar
// o clipboard sozinho e destrutivo.
//
//nolint:gochecknoglobals // Espelho de LimparClip para o caminho sem app: default desligado (destrutivo).
var clipAutoLimpar bool

func copiarTexto(texto string) bool {
	if texto == "" {
		return false
	}

	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return false
	}
	defer pCloseClipboard.Call()

	pEmptyClipboard.Call()

	const bytesPorUnidadeUTF16 = 2

	unidades := windows.StringToUTF16(texto)
	num := len(unidades) * bytesPorUnidadeUTF16

	h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(num))
	if h == 0 {
		return false
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		pGlobalFree.Call(h)

		return false
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), num)
	src := unsafe.Slice((*byte)(unsafe.Pointer(&unidades[0])), num)
	copy(dst, src)
	pGlobalUnlock.Call(h)

	if r, _, _ := pSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		pGlobalFree.Call(h)

		return false
	}

	aguardado := texto

	if clipAutoLimpar {
		const atrasoLimpezaClipSegundos = 60
		time.AfterFunc(atrasoLimpezaClipSegundos*time.Second, func() {
			if lerTextoClipboard() == aguardado {
				if r, _, _ := pOpenClipboard.Call(0); r != 0 {
					pEmptyClipboard.Call()
					pCloseClipboard.Call()
				}
			}
		})
	}

	return true
}

func lerTextoClipboard() string {
	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return ""
	}
	defer pCloseClipboard.Call()

	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}

	return textoDoHandleClipboard(h)
}

// textoDoHandleClipboard le e decodifica o bloco de memoria global do clipboard.
func textoDoHandleClipboard(h uintptr) string {
	tam, _, _ := pGlobalSize.Call(h)
	if tam == 0 || tam > 16<<20 {
		return ""
	}

	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer pGlobalUnlock.Call(h)

	raw := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), tam)

	num := 0
	for num+1 < len(raw) && (raw[num] != 0 || raw[num+1] != 0) {
		num += 2
	}

	if num == 0 {
		return ""
	}

	const bytesPorUnidadeUTF16 = 2

	u16 := make([]uint16, num/bytesPorUnidadeUTF16)
	for i := range u16 {
		const deslocamentoByteAltoUTF16 = 8

		u16[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<deslocamentoByteAltoUTF16
	}

	return windows.UTF16ToString(u16)
}
