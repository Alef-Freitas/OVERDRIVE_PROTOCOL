//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const lojaExtJS = `(function(){
  if(window.top!==window) return;
  var h=(location.hostname||'').toLowerCase();
  var loja=(h==='chromewebstore.google.com'||h==='chrome.google.com')?'chrome':
    (h==='microsoftedge.microsoft.com'?'edge':
    (h==='addons.opera.com'?'opera':''));
  if(!loja) return;
  function ehDetalhe(){
    var p=location.pathname||'';
    if(loja==='opera') return /\/extensions\/details\/[^\/]+\/?/.test(p);
    return /\/detail\//.test(p);
  }
  function injetar(){
    try{
      var antigo=document.getElementById('od-instalar-ext');
      if(!ehDetalhe()){ if(antigo){ antigo.remove(); } return; }
      if(antigo) return;
      var b=document.createElement('button');
      b.id='od-instalar-ext';
      b.type='button';
      b.textContent='Instalar no Overdrive';
      b.style.cssText='position:fixed;right:18px;bottom:18px;z-index:2147483647;'+
        'background:#1c1c1c;color:#f2f2f2;border:1px solid #3a3a3a;border-radius:10px;'+
        'padding:10px 16px;font:600 13px "Segoe UI",system-ui,sans-serif;cursor:pointer;';
      b.addEventListener('mouseenter',function(){b.style.borderColor='#67e8f9';});
      b.addEventListener('mouseleave',function(){b.style.borderColor='#3a3a3a';});
      b.addEventListener('click',function(){
        try{
          b.disabled=true;b.textContent='Instalando...';
          if(window.chrome&&window.chrome.webview&&window.chrome.webview.postMessage){
            window.chrome.webview.postMessage(JSON.stringify({t:'odext'}));
          }
        }catch(e){}
      });
      (document.body||document.documentElement).appendChild(b);
    }catch(e){}
  }
  // Reavalia a cada injeção (navegação SPA não recarrega o documento):
  // a flag só registra o listener de DOMContentLoaded uma vez.
  injetar();
  if(window.__odLojaExt) return;
  window.__odLojaExt=true;
  if(!document.body) document.addEventListener('DOMContentLoaded',injetar,{once:true});
})();`

//exhaustruct:ignore // Referencia de extensao da loja: construtor parcial por desenho.
type lojaExtRef struct {
	Loja string
	Ref  string
}

var (
	reLojaChrome = regexp.MustCompile(`^https?://chromewebstore\.google\.com/detail/(?:[^/?#]+/)?([a-p]{32})(?:[/?#]|$)`)
	reLojaEdge   = regexp.MustCompile(`^https?://microsoftedge\.microsoft\.com/addons/detail/` +
		`(?:[^/?#]+/)?([a-z0-9]{32})(?:[/?#]|$)`)
	reLojaOpera = regexp.MustCompile(`^https?://addons\.opera\.com/(?:[a-z-]+/)?extensions/details/` +
		`([a-z0-9][a-z0-9-]*)(?:[/?#]|$)`)
)

func detectarLojaExt(endereco string) (lojaExtRef, bool) {
	endereco = strings.TrimSpace(endereco)
	if m := reLojaChrome.FindStringSubmatch(endereco); m != nil {
		return lojaExtRef{Loja: "chrome", Ref: m[1]}, true
	}

	if m := reLojaEdge.FindStringSubmatch(endereco); m != nil {
		return lojaExtRef{Loja: idEdge, Ref: m[1]}, true
	}

	if m := reLojaOpera.FindStringSubmatch(endereco); m != nil {
		return lojaExtRef{Loja: "opera", Ref: m[1]}, true
	}

	return lojaExtRef{}, false
}

func ehHostLojaExt(endereco string) bool {
	endereco = strings.ToLower(strings.TrimSpace(endereco))

	return strings.Contains(endereco, "chromewebstore.google.com/") ||
		strings.Contains(endereco, "chrome.google.com/") ||
		strings.Contains(endereco, "microsoftedge.microsoft.com/") ||
		strings.Contains(endereco, "addons.opera.com/")
}

const (
	maxCRXBytes = 150 << 20
)

// versaoChrome e a fonte unica da versao do Chromium anunciada nas lojas e
// na emulacao do painel de imagens. Comeca em 155 e ajustada para o major
// do runtime WebView2 real quando o ambiente fica pronto.
//
//nolint:gochecknoglobals // Versao do Chromium anunciada: ajustada uma vez quando o ambiente fica pronto.
var versaoChrome = "155"

//nolint:gochecknoglobals // UAs de loja: construidos uma vez, somente leitura depois.
var (
	uaChromeLoja = uaLoja("chrome")
	uaEdgeLoja   = uaLoja(idEdge)
)

// uaLoja deriva o UA da loja a partir do major detectado do runtime, com
// fallback para o valor estatico.
func uaLoja(navegador string) string {
	versao := strings.TrimSpace(versaoChrome)
	if versao == "" {
		versao = "155"
	}

	base := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" +
		versao + ".0.0.0 Safari/537.36"
	if navegador == idEdge {
		return base + " Edg/" + versao + ".0.0.0"
	}

	return base
}

// alinharVersaoChrome ajusta UAs e prodversion ao major do runtime WebView2
// real (ex. "140.0.3485.14" -> "140"). Sem major valido, mantem 155.
func alinharVersaoChrome(browserVersion string) {
	major := ""

	for campo := range strings.SplitSeq(strings.TrimSpace(browserVersion), ".") {
		if n, err := strconv.Atoi(strings.TrimSpace(campo)); err == nil && n >= 80 && n <= 999 {
			major = strconv.Itoa(n)

			break
		}
	}

	if major == "" {
		return
	}

	versaoChrome = major
	uaChromeLoja = uaLoja("chrome")
	uaEdgeLoja = uaLoja(idEdge)
}

func urlCRXDaLoja(ref lojaExtRef) (string, string) {
	versao := strings.TrimSpace(versaoChrome)
	if versao == "" {
		versao = "155"
	}

	switch ref.Loja {
	case "chrome":
		return "https://clients2.google.com/service/update2/crx?response=redirect&prodversion=" + versao +
			".0.0.0&acceptformat=crx2,crx3&x=id%3D" + ref.Ref + "%26installsource%3Dondemand%26uc", uaChromeLoja
	case idEdge:
		return "https://edge.microsoft.com/extensionwebstorebase/v1/crx?response=redirect" +
				"&os=win&arch=x64&os_arch=x86_64&nacl_arch=x86-64&prod=edgecrx&prodchannel=stable&prodversion=" + versao +
				".0.0.0&lang=en-US&acceptformat=crx2,crx3&x=id%3D" + ref.Ref + "%26installsource%3Dondemand%26uc",
			uaEdgeLoja
	default:
		return "https://addons.opera.com/extensions/download/" + ref.Ref + "/", uaChromeLoja
	}
}

// novoPedidoLoja monta o GET da loja com o UA certo.
func novoPedidoLoja(ref lojaExtRef) (*http.Request, error) {
	url, agente := urlCRXDaLoja(ref)

	// Sem ctx ascendente no fluxo de instalacao: escopo e a propria chamada.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, erroTextoF("montar pedido loja: %w", err)
	}

	req.Header.Set("User-Agent", agente)

	return req, nil
}

// verificarRespostaLoja valida o status da loja.
func verificarRespostaLoja(resp *http.Response) error {
	if resp.StatusCode == http.StatusNoContent {
		return erroTexto(txtLojaRecusou)
	}

	if resp.StatusCode == http.StatusNotFound {
		return erroTexto(txtLojaNaoAchada)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return erroTextoF(txtFmtLojaHTTP, resp.StatusCode)
	}

	return nil
}

// relatarProgressoLoja avisa o progresso de 5 em 5%.
func relatarProgressoLoja(progresso func(pct int), lido, total int64, ultimoPct *int) {
	if progresso == nil || total <= 0 {
		return
	}

	pct := int(lido * 100 / total)
	if pct != *ultimoPct && pct%5 == 0 {
		*ultimoPct = pct
		progresso(pct)
	}
}

// lerCorpoLojaComProgresso le o corpo com teto e progresso.
func lerCorpoLojaComProgresso(resp *http.Response, progresso func(pct int)) ([]byte, error) {
	var buf bytes.Buffer

	lido := int64(0)
	total := resp.ContentLength

	const (
		tamanhoLeituraLojaKB = 128
		bytesPorKBLoja       = 1024
	)

	tmp := make([]byte, tamanhoLeituraLojaKB*bytesPorKBLoja)
	ultimoPct := -1

	for {
		numLidos, err := resp.Body.Read(tmp)
		if numLidos > 0 {
			lido += int64(numLidos)
			if lido > maxCRXBytes {
				return nil, erroTexto(txtLojaGrande)
			}

			buf.Write(tmp[:numLidos])
			relatarProgressoLoja(progresso, lido, total, &ultimoPct)
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, erroTextoF(txtFmtLojaInterromp, err)
		}
	}

	if buf.Len() == 0 {
		return nil, erroTexto(txtLojaVazia)
	}

	return buf.Bytes(), nil
}

func baixarCRXDaLoja(ref lojaExtRef, progresso func(pct int)) ([]byte, error) {
	req, err := novoPedidoLoja(ref)
	if err != nil {
		return nil, err
	}

	const timeoutDownloadLojaSegundos = 180
	//exhaustruct:ignore // Cliente HTTP de loja: so timeout custom; resto padrao do Go.
	cli := &http.Client{Timeout: timeoutDownloadLojaSegundos * time.Second}

	resp, err := cli.Do(req)
	if err != nil {
		return nil, erroTextoF(txtFmtLojaDownload, err)
	}
	defer resp.Body.Close()

	if err := verificarRespostaLoja(resp); err != nil {
		return nil, err
	}

	return lerCorpoLojaComProgresso(resp, progresso)
}

// inicioZipDeCRX devolve o offset do zip conforme a versao do CRX.
func inicioZipDeCRX(dados []byte) (int, error) {
	const (
		versaoCRX2            = 2
		versaoCRX3            = 3
		tamanhoCabecalhoCRX2  = 16
		tamanhoCabecalhoCRX3  = 12
		deslocamentoCabecalho = 8
	)

	versao := binary.LittleEndian.Uint32(dados[4:8])

	switch versao {
	case versaoCRX2:
		if len(dados) < tamanhoCabecalhoCRX2 {
			return 0, erroTexto(txtLojaCRXTruncado)
		}

		pk := binary.LittleEndian.Uint32(dados[deslocamentoCabecalho:12])
		sig := binary.LittleEndian.Uint32(dados[12:tamanhoCabecalhoCRX2])

		return tamanhoCabecalhoCRX2 + int(pk) + int(sig), nil
	case versaoCRX3:
		tam := binary.LittleEndian.Uint32(dados[deslocamentoCabecalho:12])

		return tamanhoCabecalhoCRX3 + int(tam), nil
	default:
		return 0, erroTextoF(txtFmtLojaCRXVersao, versao)
	}
}

// localizarZipEmCRX acha o inicio do zip dentro do CRX.
func localizarZipEmCRX(dados []byte, ini int) ([]byte, error) {
	if ini <= 0 || ini >= len(dados) {
		return nil, erroTexto(txtLojaCRXInvalido)
	}

	conteudo := dados[ini:]
	if len(conteudo) >= 4 && string(conteudo[:4]) == "PK\x03\x04" {
		return conteudo, nil
	}

	if i := bytes.Index(conteudo, []byte("PK\x03\x04")); i >= 0 {
		return conteudo[i:], nil
	}

	return nil, erroTexto(txtLojaSemZip)
}

func zipDeCRX(dados []byte) ([]byte, error) {
	const tamanhoMinimoCRX = 12
	if len(dados) < tamanhoMinimoCRX || string(dados[:4]) != "Cr24" {
		return nil, erroTexto(txtLojaCRXNaoPacote)
	}

	ini, err := inicioZipDeCRX(dados)
	if err != nil {
		return nil, err
	}

	return localizarZipEmCRX(dados, ini)
}

//nolint:tagliatelle // Manifesto dentro do CRX: manifest_version e o contrato Chrome.
type manifestoLoja struct {
	Nome            string `json:"name"`
	Versao          string `json:"version"`
	VersaoManifesto int    `json:"manifest_version"`
}

func manifestoDoZip(z *zip.Reader) (manifestoLoja, error) {
	const limiteLeituraManifestoBytes = 1 << 20

	var manifesto manifestoLoja

	for _, f := range z.File {
		if f.Name != "manifest.json" {
			continue
		}

		leitor, err := f.Open()
		if err != nil {
			return manifesto, erroTextoF("abrir manifesto: %w", err)
		}

		corpo, err := io.ReadAll(io.LimitReader(leitor, limiteLeituraManifestoBytes))
		leitor.Close()

		if err != nil {
			return manifesto, erroTextoF("ler manifesto: %w", err)
		}

		if json.Unmarshal(corpo, &manifesto) != nil {
			return manifesto, erroTexto(txtLojaManifestRuim)
		}

		if manifesto.VersaoManifesto != 2 && manifesto.VersaoManifesto != 3 {
			return manifesto, erroTextoF(txtFmtLojaManifestV, manifesto.VersaoManifesto)
		}

		if strings.TrimSpace(manifesto.Nome) == "" || strings.TrimSpace(manifesto.Versao) == "" {
			return manifesto, erroTexto(txtLojaManifestSeco)
		}

		return manifesto, nil
	}

	return manifesto, erroTexto(txtLojaSemManifest)
}

// acumularRunaPastaExt soma uma runa no nome sanitizado.
func acumularRunaPastaExt(montador *strings.Builder, antesHifen *bool, caractere rune) {
	switch {
	case caractere >= 'a' && caractere <= 'z', caractere >= '0' && caractere <= '9':
		montador.WriteRune(caractere)

		*antesHifen = false
	case caractere == '-' || caractere == '_' || caractere == ' ' || caractere == '.':
		if !*antesHifen && montador.Len() > 0 {
			montador.WriteByte('-')

			*antesHifen = true
		}
	}
}

func sanitizarPastaExt(texto string) string {
	texto = strings.ToLower(strings.TrimSpace(texto))

	var montador strings.Builder

	antesHifen := false

	const maxPastaExt = 48

	for _, r := range texto {
		acumularRunaPastaExt(&montador, &antesHifen, r)

		if montador.Len() >= maxPastaExt {
			break
		}
	}

	texto = strings.Trim(montador.String(), "-")

	return texto
}

// alvoSeguroExt resolve o destino contido na pasta da extensao.
func alvoSeguroExt(destino, absDestino, nome string) (string, error) {
	alvo, err := filepath.Abs(filepath.Join(destino, nome))
	if err != nil || alvo != absDestino && !strings.HasPrefix(alvo, absDestino+string(filepath.Separator)) {
		return "", erroTextoF(txtFmtLojaSuspeito, nome)
	}

	return alvo, nil
}

// extrairArquivoZipExt grava uma entrada do zip no disco.
func extrairArquivoZipExt(f *zip.File, alvo string) error {
	leitor, err := f.Open()
	if err != nil {
		return erroTextoF("abrir entrada: %w", err)
	}
	// GOD: extracao contida (alvo validado contra absDestino acima); sem traversal por desenho.
	out, err := os.OpenFile(alvo, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304
	if err != nil {
		leitor.Close()

		return erroTextoF("criar arquivo: %w", err)
	}

	_, err = io.Copy(out, io.LimitReader(leitor, maxCRXBytes))
	leitor.Close()
	out.Close()

	if err != nil {
		return erroTextoF("extrair arquivo: %w", err)
	}

	return nil
}

// extrairEntradaZipExt trata um diretorio ou arquivo do zip.
func extrairEntradaZipExt(entrada *zip.File, destino, absDestino string) error {
	nome := filepath.FromSlash(entrada.Name)
	if nome == "" || strings.HasPrefix(nome, ".") && len(nome) == 1 {
		return nil
	}

	alvo, err := alvoSeguroExt(destino, absDestino, nome)
	if err != nil {
		return err
	}

	if entrada.FileInfo().IsDir() {
		if err := os.MkdirAll(alvo, 0o700); err != nil {
			return erroTextoF("criar pasta: %w", err)
		}

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(alvo), 0o700); err != nil {
		return erroTextoF("criar pasta: %w", err)
	}

	return extrairArquivoZipExt(entrada, alvo)
}

func instalarZipNaPasta(arquivoZip *zip.Reader, pasta string) error {
	// GOD: extracao contida (prefixo absDestino verificado no laco acima); sem traversal por desenho.
	destino := filepath.Join(extDir, pasta)
	if err := os.MkdirAll(destino, 0o700); err != nil {
		return erroTextoF("criar pasta extensao: %w", err)
	}

	concluido := false
	defer func() {
		if !concluido {
			_ = os.RemoveAll(destino)
		}
	}()

	absDestino, err := filepath.Abs(destino)
	if err != nil {
		return erroTextoF("resolver pasta extensao: %w", err)
	}

	for _, f := range arquivoZip.File {
		if err := extrairEntradaZipExt(f, destino, absDestino); err != nil {
			return err
		}
	}

	concluido = true

	return nil
}

func (a *app) instalarExtDaLoja(p *pane) {
	if a == nil || p == nil {
		return
	}

	if a.extInstalando {
		a.dizerf(txtLojaInstalando)

		return
	}

	u := strings.TrimSpace(p.ultimaURL)
	if u == "" && p.wv != nil {
		u = strings.TrimSpace(p.wv.Source())
	}

	ref, ok := detectarLojaExt(u)
	if !ok {
		a.dizerErrof(txtLojaAbraPagina)

		return
	}

	a.extInstalando = true
	a.dizerProgressoIndetf(txtLojaBaixando)

	var w *WebView
	if p.wv != nil {
		w = p.wv
	}

	go a.baixarEInstalarExt(ref, w)
}

// pastaInicialExt sugere a pasta a partir do nome ou da referencia.
func pastaInicialExt(nome, ref string) string {
	const prefixoRefExt = 8

	pasta := sanitizarPastaExt(nome)
	if pasta != "" {
		return pasta
	}

	if len(ref) >= prefixoRefExt {
		return "ext-" + strings.ToLower(ref[:prefixoRefExt])
	}

	return "ext-nova"
}

// mesmaVersaoInstalada diz se a pasta ja tem a mesma versao instalada.
func mesmaVersaoInstalada(pasta, versao string) bool {
	// GOD: pasta sanitizada (sanitizarPastaExt); leitura contida em extDir.
	// GOD: pasta sanitizada na instalacao (sanitizarPastaExt); leitura contida em extDir.
	if b, err := os.ReadFile(filepath.Join(extDir, pasta, "manifest.json")); err == nil { // #nosec G304 // #nosec G304
		var atual manifestoLoja
		if json.Unmarshal(b, &atual) == nil && atual.Versao == versao {
			return true
		}
	}

	return false
}

// resolverPastaExtLivre acha pasta livre ou detecta mesma versao instalada.
func resolverPastaExtLivre(nome, ref, versao string) (string, bool) {
	pasta := pastaInicialExt(nome, ref)
	base := pasta

	for sufixo := 2; ; sufixo++ {
		if _, err := os.Stat(filepath.Join(extDir, pasta, "manifest.json")); os.IsNotExist(err) {
			return pasta, false
		}

		if mesmaVersaoInstalada(pasta, versao) {
			return pasta, true
		}

		pasta = fmt.Sprintf("%s-%d", base, sufixo)
	}
}

// popupDoZipExt extrai o popup do manifesto dentro do zip.
func popupDoZipExt(leitorZip *zip.Reader) string {
	for _, f := range leitorZip.File {
		if f.Name != "manifest.json" {
			continue
		}

		leitor, err := f.Open()
		if err != nil {
			break
		}

		//nolint:tagliatelle // Leitura de manifesto Chrome: action/browser_action sao o contrato.
		var cheio struct {
			Acao struct {
				Popup string `json:"default_popup"`
			} `json:"action"`
			Barra struct {
				Popup string `json:"default_popup"`
			} `json:"browser_action"`
		}

		const limiteLeituraPopupBytes = 1 << 20

		b, _ := io.ReadAll(io.LimitReader(leitor, limiteLeituraPopupBytes))
		leitor.Close()

		_ = json.Unmarshal(b, &cheio)

		if cheio.Acao.Popup != "" {
			return cheio.Acao.Popup
		}

		return cheio.Barra.Popup
	}

	return ""
}

// avisarExtJaInstalada garante a extensao ligada e avisa.
func (a *app) avisarExtJaInstalada(pasta, nome, versao string) {
	a.depois(func() {
		a.alternarExtensao(pasta, true)

		if a.st.ExtFixarNovas {
			a.alternarFixada(pasta, true)
		}

		a.extInstalando = false
		a.dizerOkf(txtFmtLojaJaInstalada, nome, versao)
	})
}

// concluirInstalacaoExt liga a extensao nova e avisa o resultado.
func (a *app) concluirInstalacaoExt(pasta, nome, versao, popup string, w *WebView) {
	a.depois(func() {
		a.alternarExtensao(pasta, true)

		if a.st.ExtFixarNovas {
			a.alternarFixada(pasta, true)
		}

		if w != nil {
			a.tryAddExtensionEm(w, filepath.Join(extDir, pasta), popup, true)
		}

		a.extInstalando = false
		if popup == "" {
			a.dizerOkf(txtFmtLojaSemPainel, nome, versao)
		} else {
			a.dizerOkf(txtFmtLojaInstalada, nome, versao)
		}
	})
}

// prepararZipExt baixa o CRX e devolve o zip lido com o manifesto.
func (a *app) prepararZipExt(ref lojaExtRef, falha func(error)) (*zip.Reader, manifestoLoja, bool) {
	var vazio manifestoLoja

	progr := func(pct int) {
		p := pct

		a.depois(func() { a.dizerProgressof(p, txtFmtLojaBaixandoPct, p) })
	}

	dados, err := baixarCRXDaLoja(ref, progr)
	if err != nil {
		falha(err)

		return nil, vazio, false
	}

	a.depois(func() { a.dizerProgressoIndetf(txtLojaExtraindo) })

	zbytes, err := zipDeCRX(dados)
	if err != nil {
		falha(err)

		return nil, vazio, false
	}

	leitorZip, err := zip.NewReader(bytes.NewReader(zbytes), int64(len(zbytes)))
	if err != nil {
		falha(erroTextoF("zip inválido: %w", err))

		return nil, vazio, false
	}

	manifesto, err := manifestoDoZip(leitorZip)
	if err != nil {
		falha(err)

		return nil, vazio, false
	}

	return leitorZip, manifesto, true
}

func (a *app) baixarEInstalarExt(ref lojaExtRef, w *WebView) {
	falha := func(err error) {
		a.depois(func() {
			a.extInstalando = false
			a.dizerErrof(txtFmtLojaFalhaInst, err)
		})
	}

	leitorZip, manifesto, pronto := a.prepararZipExt(ref, falha)
	if !pronto {
		return
	}

	pasta, jaInstalada := resolverPastaExtLivre(manifesto.Nome, ref.Ref, manifesto.Versao)
	// Mesma versao ja instalada: nao reextrai nem re-adiciona, so garante
	// que esta ligada e avisa. Evita duplicar o trabalho e o relato.
	if jaInstalada {
		a.avisarExtJaInstalada(pasta, manifesto.Nome, manifesto.Versao)

		return
	}

	if err := instalarZipNaPasta(leitorZip, pasta); err != nil {
		falha(err)

		return
	}

	a.concluirInstalacaoExt(pasta, manifesto.Nome, manifesto.Versao, popupDoZipExt(leitorZip), w)
}
