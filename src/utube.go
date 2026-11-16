package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Constantes locais do UTube (limites, esperas e fatores sem outro dono).
const (
	maximoRedirecionaTube   = 6
	esperaClienteTube       = 12 * time.Second
	esperaResolveStream     = 40 * time.Second
	tamanhoMaxErroYtDlp     = 160
	maximoStreamsCache      = 20
	validadeStreamCache     = 4 * time.Hour
	maximoBuscasCache       = 30
	validadeBuscaCache      = 5 * time.Minute
	maximoLegendasCache     = 120
	ajusteArredondaMs       = 0.5
	milissegundosPorHora    = 3600000
	limiteLegendaBytes      = 8 << 20
	milissegundosPorSegundo = 1000
	quantidadeEagerCards    = 4
	baseHashCor             = 31
	totalCoresCanal         = 8
	segundosPorHora         = 3600
	segundosPorMinuto       = 60
	meioSegundoAjuste       = 0.5
	horasPorDia             = 24
	diasPorMes              = 30
	diasPorAno              = 365
	tamanhoIDYouTube        = 11
	capacidadeCategorias    = 6
	larguraMinimaSegmento   = 0.35
	porcentagemSegmento     = 100
	codigoHTTPSucessoTube   = 200
	limiteRespostaTube      = 4 << 20
	maximoCanaisCache       = 50
	validadeCanalCache      = 10 * time.Minute
	maximoVideosCache       = 50
	validadeVideoCache      = 10 * time.Minute
	esperaVideoRapido       = 14 * time.Second
	esperaSondaStream       = 3 * time.Second
	esperaComentarios       = 8 * time.Second
)

type utubeCard struct {
	Titulo       string
	Canal        string
	CanalURL     string
	Avatar       string
	Iniciais     string
	Views        string
	Publicado    string
	Duracao      string
	Thumb        string
	ThumbPequena string
	URL          string
	Descricao    string
	Cor          int
	AoVivo       bool
	Eager        bool
}

//exhaustruct:ignore // Canal do YouTube: construtor parcial por desenho.
type utubeCanal struct {
	Nome      string
	Avatar    string
	Banner    string
	Descricao string
	Inscritos string
	Iniciais  string
	Cor       int
}

type utubeCategoria struct {
	Nome   string
	Classe string
}

type utubeSegmento struct {
	Classe string
	Left   string
	Width  string
	Nome   string
	Tempo  string
	Inicio float64
	Fim    float64
}

type ytLegenda struct {
	Nome   string
	Codigo string
	Auto   bool
}

//exhaustruct:ignore // Track de legenda: zero-value = descartada (sentinela dos filtros).
type ytLegendaTrack struct {
	Nome    string
	Codigo  string
	Auto    bool
	BaseURL string
}

type utubeLegendaCue struct {
	Inicio  float64
	Duracao float64
	Tempo   string
	Texto   string
}

func (c utubeLegendaCue) Fim() float64 {
	return c.Inicio + c.Duracao
}

//exhaustruct:ignore // Dados do UTube: construtor parcial por desenho.
type utubeDados struct {
	Pagina          string
	Titulo          string
	Query           string
	Categoria       string
	Erro            string
	Ano             int
	Itens           []utubeCard
	Video           *ytVideo
	VideoID         string
	Start           int
	Origem          string
	OrigemJS        string
	Canal           *utubeCanal
	CanalID         string
	CanalURL        string
	CanalAvatar     string
	CanalIniciais   string
	CanalCor        int
	Relacionados    []utubeCard
	Comentarios     []ytComentario
	ComentariosErro string
	ComentariosProx string
	Legendas        []ytLegenda
	LegendasCues    []utubeLegendaCue
	LegendasErro    string
	Segmentos       []utubeSegmento
	Categorias      []utubeCategoria
	Duracao         float64
	Leve            bool
	LeveURL         string
	LeveFalhou      bool
	LeveFonte       string
	LeveErro        string
	FiltroTipo      string
	FiltroDuracao   string
}

func utubeURLPublica(url *url.URL) bool {
	if url == nil {
		return false
	}

	if url.Scheme != esquemaHTTP && url.Scheme != esquemaHTTPS {
		return false
	}

	if url.User != nil {
		return false
	}

	host := strings.ToLower(strings.TrimSuffix(url.Hostname(), "."))
	if host == "" || strings.Contains(host, "%") {
		return false
	}

	return hostPublicoURL(host)
}

// hostPublicoURL diz se o host pode ser acessado pela rede publica.
func hostPublicoURL(host string) bool {
	for _, suf := range []string{sufLocal, sufInternal, sufLan, sufHome} {
		if host == strings.TrimPrefix(suf, ".") || strings.HasSuffix(host, suf) {
			return false
		}
	}

	if host == hostLocal || strings.HasSuffix(host, ".localhost") {
		return false
	}

	if ip := net.ParseIP(host); ip != nil {
		return !ipRestritoURL(ip)
	}

	return true
}

// ipRestritoURL diz se o IP e interno e nao pode sair na rede.
func ipRestritoURL(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func redirectPublico(req *http.Request, via []*http.Request) error {
	if len(via) >= maximoRedirecionaTube {
		return erroTexto("muitos redirecionamentos")
	}

	if !utubeURLPublica(req.URL) {
		return erroTexto("redirecionamento bloqueado")
	}

	return nil
}

//nolint:gochecknoglobals // Cliente YouTube dedicado: timeout fixo, somente leitura apos init.
//exhaustruct:ignore // http.Client: so timeout + redirect; transporte/Jar padrao do Go.
var utubeHTTP = &http.Client{
	Timeout:       esperaClienteTube,
	CheckRedirect: redirectPublico,
}

//nolint:gochecknoglobals // Cache de trending + mutex: estado de processo.
var (
	utubeTrendMu    sync.Mutex
	utubeTrendCache []ytItem
	utubeTrendAt    time.Time
)

// ytFontePadrao: "ytdlp", "piped" ou fonteOriginal. O player mostra qual
// esta realmente em uso (cai sozinho nas falhas).
//
//nolint:gochecknoglobals // Fonte padrao + mutex: estado de processo.
var (
	ytFonteMu     sync.Mutex
	ytFontePadrao = "ytdlp"
)

//nolint:gochecknoglobals // Cache de streams + mutex: estado de processo.
var (
	ytStreamMu    sync.Mutex
	ytStreamCache = map[string]struct {
		url string
		exp time.Time
	}{}
)

// ytVoo é um singleflight mínimo: dois watch?v=X simultâneos resolvem uma
// vez só, em vez de empilhar processos yt-dlp+node.
//
//exhaustruct:ignore // Voo singleflight: construtor parcial por desenho.
type ytVooChamada struct {
	pronto chan struct{}
	url    string
	erro   error
}

//nolint:gochecknoglobals // Voos singleflight + mutex: estado de processo.
var (
	ytVooMu sync.Mutex
	ytVoo   = map[string]*ytVooChamada{}
)

func ytDlpStreamCtx(ctx context.Context, id string) (string, error) {
	if !ytIDValido(id) {
		return "", erroTexto("id invalido")
	}

	ytStreamMu.Lock()
	if it, ok := ytStreamCache[id]; ok && time.Now().Before(it.exp) {
		ytStreamMu.Unlock()

		return it.url, nil
	}
	ytStreamMu.Unlock()
	ytVooMu.Lock()
	if v, ok := ytVoo[id]; ok {
		ytVooMu.Unlock()

		select {
		case <-v.pronto:
			return v.url, v.erro
		case <-ctx.Done():
			return "", erroTextoF("busca cancelada: %w", ctx.Err())
		}
	}

	voo := &ytVooChamada{pronto: make(chan struct{})}
	ytVoo[id] = voo
	ytVooMu.Unlock()

	voo.url, voo.erro = ytDlpResolver(ctx, id)
	close(voo.pronto)
	ytVooMu.Lock()
	delete(ytVoo, id)
	ytVooMu.Unlock()

	return voo.url, voo.erro
}

func ytDlpResolver(ctx context.Context, id string) (string, error) {
	ytdlp := filepath.Join(exeDir, "tools", "yt-dlp.exe")
	nodejs := filepath.Join(exeDir, "tools", "node.exe")

	if fi, err := os.Stat(ytdlp); err != nil || fi.IsDir() {
		return "", erroTexto("yt-dlp ausente")
	}

	ctx, cancelar := context.WithTimeout(ctx, esperaResolveStream)
	defer cancelar()

	args := []string{"-g", "-f", "b[height<=720]/b", "--no-playlist", "--no-warnings"}
	if _, err := os.Stat(nodejs); err == nil {
		args = append(args, "--js-runtimes", "node:"+nodejs)
	}
	// GOD: binario fixo (tools/yt-dlp.exe), sem shell, id validado (ytIDValido) e URL de host fixo.
	cmd := exec.CommandContext(ctx, ytdlp, append(args, "https://www.youtube.com/watch?v="+id)...) // #nosec G702, G204

	//exhaustruct:ignore // SysProcAttr: so HideWindow; resto zero intencional.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var saida, errBuf bytes.Buffer

	cmd.Stdout = &saida
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", erroTextoF("yt-dlp: %s", mensagemErroYtDlp(err, errBuf.String()))
	}

	if linha, ok := escolherStreamYtDlp(saida.String(), id); ok {
		return linha, nil
	}

	return "", erroTexto("sem stream")
}

// mensagemErroYtDlp resume a ultima linha do erro do yt-dlp.
func mensagemErroYtDlp(err error, saida string) string {
	msg := strings.TrimSpace(saida)
	if msg != "" {
		linhas := strings.Split(msg, "\n")
		msg = strings.TrimSpace(linhas[len(linhas)-1])
	}

	if len(msg) > tamanhoMaxErroYtDlp {
		msg = msg[:157] + "..."
	}

	if msg == "" {
		msg = err.Error()
	}

	return msg
}

// escolherStreamYtDlp pega a primeira URL publica da saida; true = achou.
func escolherStreamYtDlp(saida, id string) (string, bool) {
	for linha := range strings.SplitSeq(saida, "\n") {
		linha = strings.TrimSpace(linha)
		if !strings.HasPrefix(linha, "https://") {
			continue
		}

		u, err := url.Parse(linha)
		if err != nil || !utubeURLPublica(u) {
			continue
		}

		guardarStreamYtDlp(id, linha)

		return linha, true
	}

	return "", false
}

// guardarStreamYtDlp salva a URL no cache podando expirados.
func guardarStreamYtDlp(id, linha string) {
	ytStreamMu.Lock()
	defer ytStreamMu.Unlock()

	if len(ytStreamCache) >= maximoStreamsCache {
		agora := time.Now()
		for k, it := range ytStreamCache {
			if !agora.Before(it.exp) {
				delete(ytStreamCache, k)
			}
		}
	}

	ytStreamCache[id] = struct {
		url string
		exp time.Time
	}{linha, time.Now().Add(validadeStreamCache)}
}

// Cache TTL de metadados (voltar/avancar sem martelar o InnerTube).
//
//nolint:gochecknoglobals // Cache de videos + mutex: estado de processo.
var (
	ytVideoMu    sync.Mutex
	ytVideoCache = map[string]struct {
		v   *ytVideo
		exp time.Time
	}{}
)

//nolint:gochecknoglobals // Cache de buscas + mutex: estado de processo.
var (
	ytBuscaMu    sync.Mutex
	ytBuscaCache = map[string]struct {
		itens []ytItem
		exp   time.Time
	}{}
)

func ytBuscarFiltro(ctx context.Context, consulta, params string) ([]ytItem, error) {
	if params == "" {
		params = ytParamsPadrao
	}

	chave := consulta + "\x00" + params

	ytBuscaMu.Lock()
	if it, ok := ytBuscaCache[chave]; ok && time.Now().Before(it.exp) {
		saida := append([]ytItem(nil), it.itens...)
		ytBuscaMu.Unlock()

		return saida, nil
	}
	ytBuscaMu.Unlock()

	itens, err := ytBuscarParams(ctx, consulta, params)
	if err != nil {
		return nil, err
	}

	ytBuscaMu.Lock()
	if len(ytBuscaCache) >= maximoBuscasCache {
		agora := time.Now()
		for k, it := range ytBuscaCache {
			if !agora.Before(it.exp) {
				delete(ytBuscaCache, k)
			}
		}
	}

	ytBuscaCache[chave] = struct {
		itens []ytItem
		exp   time.Time
	}{append([]ytItem(nil), itens...), time.Now().Add(validadeBuscaCache)}
	ytBuscaMu.Unlock()

	return itens, nil
}

//nolint:gochecknoglobals // Cache de templates + mutex: estado de processo.
var (
	utubeTplMu    sync.Mutex
	utubeTplCache = map[string]*template.Template{}
)

func utubeTemplate(pagina string) (*template.Template, error) {
	utubeTplMu.Lock()
	defer utubeTplMu.Unlock()

	if t, ok := utubeTplCache[pagina]; ok {
		return t, nil
	}

	base := filepath.Join(exeDir, "lib", "paginas", "utube", "base.html")
	arq := filepath.Join(exeDir, "lib", "paginas", "utube", pagina)

	modelo, err := template.ParseFiles(base, arq)
	if err != nil {
		return nil, erroTextoF("pagina: %w", err)
	}

	utubeTplCache[pagina] = modelo

	return modelo, nil
}

func utubeServir(mux *http.ServeMux) {
	mux.HandleFunc("/yt", utubePaginaInicial)
	mux.HandleFunc("/yt/results", utubeBuscarPagina)
	mux.HandleFunc("/yt/watch", utubeAssistir)
	mux.HandleFunc("/yt/captions", utubeLegendasPagina)
	mux.HandleFunc("/yt/vtt", utubeVTT)
	mux.HandleFunc("/yt/replies", utubeRespostas)
	mux.HandleFunc("/yt/channel/", utubeCanalPagina)
	mux.HandleFunc("/yt/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/yt", http.StatusFound)
	})
}

func utubePaginaInicial(escritor http.ResponseWriter, pedido *http.Request) {
	itens, err := utubeTrending(pedido.Context())
	if err != nil || len(itens) == 0 {
		itens, _ = ytBuscar(pedido.Context(), "música")
	}

	dados := utubeDados{
		Pagina: idHome,
		Titulo: "Em alta",
		Itens:  utubeCards(itens),
	}
	if len(dados.Itens) == 0 {
		dados.Erro = "Não foi possível carregar o feed agora. Tente buscar alguma coisa."
	}

	utubeRender(escritor, pedido, "home.html", dados)
}

func utubeBuscarPagina(escritor http.ResponseWriter, pedido *http.Request) {
	consulta := strings.TrimSpace(pedido.URL.Query().Get("q"))
	if consulta == "" {
		http.Redirect(escritor, pedido, "/yt", http.StatusFound)

		return
	}

	tipo := strings.ToLower(strings.TrimSpace(pedido.URL.Query().Get("tipo")))
	dur := strings.ToLower(strings.TrimSpace(pedido.URL.Query().Get("duracao")))
	params, filtroTipo, filtroDur := parametrosBuscaPagina(tipo, dur)

	itens, err := ytBuscarFiltro(pedido.Context(), consulta, params)

	dados := utubeDados{
		Pagina:        idSearch,
		Titulo:        "Resultados",
		Query:         consulta,
		Categoria:     utubeCategoriaDaBusca(consulta),
		Itens:         utubeCards(itens),
		FiltroTipo:    filtroTipo,
		FiltroDuracao: filtroDur,
	}
	if err != nil {
		dados.Erro = "Não foi possível buscar agora. Tente de novo."
	}

	utubeRender(escritor, pedido, "results.html", dados)
}

// parametrosBuscaPagina traduz tipo e duracao em params do InnerTube.
func parametrosBuscaPagina(tipo, dur string) (string, string, string) {
	params := ytParamsPadrao
	filtroTipo, filtroDur := "", ""

	switch tipo {
	case "canal":
		params, filtroTipo = "EgIQAg%3D%3D", "canal"
	case "playlist":
		params, filtroTipo = "EgIQAw%3D%3D", "playlist"
	case idVideo:
		params, filtroTipo = ytParamsPadrao, idVideo
	case modoTudo:
		params = ""
	default:
		params, filtroDur = parametroDuracaoBusca(dur, params)
	}

	return params, filtroTipo, filtroDur
}

// parametroDuracaoBusca traduz a duracao em params mantendo o padrao.
func parametroDuracaoBusca(dur, padrao string) (string, string) {
	switch dur {
	case "curta":
		return "EgQQARgB", "curta"
	case "media":
		return "EgQQARgC", "media"
	case "longa":
		return "EgQQARgD", "longa"
	}

	return padrao, ""
}

func utubeAssistir(escritor http.ResponseWriter, pedido *http.Request) {
	id, ok := idAssistir(escritor, pedido)
	if !ok {
		return
	}

	inicio := inicioTempoAssistir(pedido)
	token := pedido.URL.Query().Get("ct")
	fonte := fonteAssistir(pedido)

	if servirFragmentoComentarios(escritor, pedido, id, token) {
		return
	}

	video, com, tipoStream := buscarDadosAssistir(pedido, id, token, fonte)
	if video == nil {
		video = &ytVideo{ID: id}
	}

	segmentos, categorias, duracao := utubeSegmentos(video)

	dados := montarDadosAssistir(pedido, video, com, inicio, segmentos, categorias, duracao)
	if dados.Titulo == "" {
		dados.Titulo = "Vídeo"
	}

	if !resolverStreamLeve(pedido.Context(), &dados, <-tipoStream, video.ID, fonte) {
		return
	}

	if video.Erro != "" {
		dados.Erro = "Não foi possível obter todos os detalhes deste vídeo."
	}

	if com.err != nil {
		dados.ComentariosErro = "Não foi possível carregar os comentários agora."
	}

	utubeRender(escritor, pedido, "watch.html", dados)
}

// idAssistir extrai o id do video; false = redirecionou.
func idAssistir(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.URL.Query().Get("v"))
	if id == "" {
		http.Redirect(w, r, "/yt", http.StatusFound)

		return "", false
	}

	return id, true
}

// inicioTempoAssistir extrai o segundo inicial do video.
func inicioTempoAssistir(r *http.Request) int {
	if t := strings.TrimSpace(r.URL.Query().Get("t")); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			return n
		}
	}

	return 0
}

// fonteAssistir resolve a fonte do player leve.
func fonteAssistir(pedido *http.Request) string {
	fonteCrua := strings.ToLower(strings.TrimSpace(pedido.URL.Query().Get("fonte")))
	if fonteCrua == "piped" {
		fonteCrua = ytFontePadrao
	}

	if fonteCrua == ytFontePadrao || fonteCrua == fonteOriginal {
		fonte := fonteCrua

		ytFonteMu.Lock()
		ytFontePadrao = fonteCrua
		ytFonteMu.Unlock()

		return fonte
	}

	if lv := strings.TrimSpace(pedido.URL.Query().Get("leve")); lv == "0" {
		return fonteOriginal
	}

	ytFonteMu.Lock()
	fonte := ytFontePadrao
	ytFonteMu.Unlock()

	return fonte
}

// servirFragmentoComentarios serve o fragmento; true = servido.
func servirFragmentoComentarios(escritor http.ResponseWriter, r *http.Request, id, token string) bool {
	if r.URL.Query().Get("fragmento") != "comentarios" {
		return false
	}

	itens, prox, err := utubeComentariosRapido(r.Context(), id, token)

	d := utubeDados{VideoID: id, Comentarios: itens, ComentariosProx: prox}
	if err != nil {
		d.ComentariosErro = "Não foi possível carregar os comentários agora."
	}

	utubeRenderParcial(escritor, "fragmento-comentarios.html", "fragmento-comentarios", d)

	return true
}

// resultadoComentarios carrega itens, proximo token e erro.
type resultadoComentarios struct {
	itens []ytComentario
	prox  string
	err   error
}

// buscarDadosAssistir busca video, comentarios e stream em paralelo.
func buscarDadosAssistir(pedido *http.Request, id, token, fonte string,
) (*ytVideo, resultadoComentarios, chan ytStreamRes) {
	tipoVideo := make(chan *ytVideo, 1)
	tipoComentarios := make(chan resultadoComentarios, 1)

	tipoStream := make(chan ytStreamRes, 1)

	if fonte == ytFontePadrao {
		go func(ctx context.Context) {
			urlReserva, erroReserva := ytDlpStreamCtx(ctx, id)

			msg := ""
			if erroReserva != nil {
				msg = erroReserva.Error()
			}

			tipoStream <- ytStreamRes{urlReserva, msg}
		}(pedido.Context())
	} else {
		tipoStream <- ytStreamRes{}
	}

	go func(ctx context.Context) { tipoVideo <- utubeVideoRapido(ctx, id) }(pedido.Context())
	go func(ctx context.Context) {
		itens, prox, err := utubeComentariosRapido(ctx, id, token)
		tipoComentarios <- resultadoComentarios{itens, prox, err}
	}(pedido.Context())

	return <-tipoVideo, <-tipoComentarios, tipoStream
}

// montarDadosAssistir monta a pagina de exibicao do video.
func montarDadosAssistir(pedido *http.Request, video *ytVideo, com resultadoComentarios, inicio int,
	segmentos []utubeSegmento, categorias []utubeCategoria, duracao float64,
) utubeDados {
	origem := "http://" + pedido.Host

	return utubeDados{
		Pagina:          idWatch,
		Titulo:          video.Titulo,
		Video:           video,
		VideoID:         video.ID,
		Start:           inicio,
		Origem:          url.QueryEscape(origem),
		OrigemJS:        origem,
		CanalID:         video.CanalID,
		CanalURL:        utubeCanalURL(video.CanalID, video.Canal),
		CanalAvatar:     video.CanalAvatar,
		CanalIniciais:   utubeIniciais(video.Canal),
		CanalCor:        utubeCor(video.Canal),
		Relacionados:    utubeCards(video.Relacionados),
		Comentarios:     com.itens,
		ComentariosProx: com.prox,
		Legendas:        video.Legendas,
		Segmentos:       segmentos,
		Categorias:      categorias,
		Duracao:         duracao,
	}
}

// resolverStreamLeve escolhe a URL leve; false = contexto cancelado.
func resolverStreamLeve(ctx context.Context, dados *utubeDados, stream ytStreamRes, videoID, fonte string) bool {
	dados.Leve = fonte != fonteOriginal
	if !dados.Leve {
		dados.LeveFonte = fonteOriginal

		return true
	}

	leveURL, provedor, erro, ok := provedorStreamLeve(ctx, stream, videoID)
	if !ok {
		return false
	}

	dados.LeveURL = leveURL
	if provedor == "" {
		dados.LeveErro = erro
	}

	dados.LeveFonte = provedor
	if provedor == "" {
		dados.Leve, dados.LeveFalhou = false, true
		dados.LeveFonte = fonteOriginal
	}

	return true
}

// provedorStreamLeve sonda o stream e tenta reserva; false = cancelado.
func provedorStreamLeve(ctx context.Context, stream ytStreamRes, videoID string) (string, string, string, bool) {
	url, erro := stream.url, stream.erro
	if url != "" {
		if sondarStream(ctx, url) {
			return url, ytFontePadrao, "", true
		}

		erro = "stream indisponivel na sonda"
	}

	select {
	case <-ctx.Done():
		return "", "", "", false
	case <-time.After(esperaSondaStream):
	}

	if urlReserva, erroReserva := ytDlpStreamCtx(ctx, videoID); urlReserva != "" {
		if sondarStream(ctx, urlReserva) {
			return urlReserva, ytFontePadrao, "", true
		}

		return "", "", "stream indisponivel no retry", true
	} else if erroReserva != nil {
		return "", "", erroReserva.Error(), true
	}

	return "", "", erro, true
}

func utubeCanalPagina(escritor http.ResponseWriter, pedido *http.Request) {
	id := strings.Trim(strings.TrimPrefix(pedido.URL.Path, "/yt/channel/"), "/")
	if id == "" {
		http.Redirect(escritor, pedido, "/yt", http.StatusFound)

		return
	}

	tipoCanal := make(chan *utubeCanal, 1)
	go func(ctx context.Context) { tipoCanal <- utubeCanalInfo(ctx, id) }(pedido.Context())

	itens, err := ytCanal(pedido.Context(), id)

	canal := completarCanalPagina(utubeCopiarCanal(<-tipoCanal), itens, id)

	dados := utubeDados{
		Pagina:  "channel",
		Titulo:  canal.Nome,
		Canal:   canal,
		CanalID: id,
		Itens:   utubeCards(itens),
	}
	if err != nil && len(dados.Itens) == 0 {
		dados.Erro = "Não foi possível carregar os vídeos deste canal."
	}

	utubeRender(escritor, pedido, "channel.html", dados)
}

// utubeCopiarCanal copia o canal cacheado para completar sem mutar o cache.
func utubeCopiarCanal(canal *utubeCanal) *utubeCanal {
	// Copia o cacheado antes de completar: o objeto do cache e
	// compartilhado entre requisicoes e nao pode ser mutado aqui.
	if canal != nil {
		cp := *canal

		return &cp
	}

	return &utubeCanal{}
}

// completarCanalPagina preenche nome, avatar e itens do canal.
func completarCanalPagina(canal *utubeCanal, itens []ytItem, id string) *utubeCanal {
	if canal.Nome == "" && len(itens) > 0 {
		canal.Nome = itens[0].Canal
		canal.Avatar = itens[0].CanalAvatar
	}

	if canal.Nome == "" {
		canal.Nome = "Canal"
	}

	canal.Iniciais = utubeIniciais(canal.Nome)
	canal.Cor = utubeCor(canal.Nome)

	for idx := range itens {
		if itens[idx].Canal == "" {
			itens[idx].Canal = canal.Nome
		}

		if itens[idx].CanalID == "" {
			itens[idx].CanalID = id
		}

		if itens[idx].CanalAvatar == "" {
			itens[idx].CanalAvatar = canal.Avatar
		}
	}

	return canal
}

func utubeRespostas(escritor http.ResponseWriter, pedido *http.Request) {
	id := strings.TrimSpace(pedido.URL.Query().Get("v"))

	token := strings.TrimSpace(pedido.URL.Query().Get("t"))
	if id == "" {
		http.Redirect(escritor, pedido, "/yt", http.StatusFound)

		return
	}

	if token == "" {
		http.Redirect(escritor, pedido, "/yt/watch?v="+url.QueryEscape(id), http.StatusFound)

		return
	}

	itens, prox, err := utubeComentariosRapido(pedido.Context(), id, token)

	dados := utubeDados{
		Pagina:          idWatch,
		Titulo:          "Respostas",
		VideoID:         id,
		Comentarios:     itens,
		ComentariosProx: prox,
	}
	if err != nil {
		dados.ComentariosErro = "Não foi possível carregar as respostas agora."
	}

	if pedido.URL.Query().Get("fragmento") == "1" {
		utubeRenderParcial(escritor, "fragmento-respostas.html", "fragmento-respostas", dados)

		return
	}

	utubeRender(escritor, pedido, "replies.html", dados)
}

type utubeLegendasCacheItem struct {
	tracks []ytLegendaTrack
	at     time.Time
}

//nolint:gochecknoglobals // Cache de legendas + mutex: estado de processo.
var (
	utubeLegendasMu    sync.Mutex
	utubeLegendasCache = map[string]utubeLegendasCacheItem{}
)

func ytLegendasDoPlayer(player map[string]any) []ytLegenda {
	tracks := ytLegendasTracksDoPlayer(player)

	out := make([]ytLegenda, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, ytLegenda{Nome: t.Nome, Codigo: t.Codigo, Auto: t.Auto})
	}

	return out
}

func ytLegendasTracksDoPlayer(player map[string]any) []ytLegendaTrack {
	itens, ok := caminho(player, "captions", "playerCaptionsTracklistRenderer", "captionTracks").([]any)
	if !ok || len(itens) == 0 {
		return nil
	}

	tracks := make([]ytLegendaTrack, 0, len(itens))
	vistos := map[string]bool{}

	for _, item := range itens {
		if track, ok := extrairTrackLegenda(item, vistos); ok {
			tracks = append(tracks, track)
		}
	}

	ordenarTracksLegenda(tracks)

	return tracks
}

// extrairTrackLegenda converte o item bruto em track; false = descartar.
func extrairTrackLegenda(item any, vistos map[string]bool) (ytLegendaTrack, bool) {
	mapa, ok := item.(map[string]any)
	if !ok {
		return ytLegendaTrack{}, false
	}

	base := textoDe(mapa["baseUrl"])

	codigo := textoDe(mapa["languageCode"])
	if base == "" || codigo == "" {
		return ytLegendaTrack{}, false
	}

	auto := ehLegendaAuto(mapa)

	chave := codigo + "|" + strconv.FormatBool(auto)
	if vistos[chave] {
		return ytLegendaTrack{}, false
	}

	vistos[chave] = true

	nome := textoDe(mapa["name"])
	if nome == "" {
		nome = codigo
	}

	return ytLegendaTrack{Nome: nome, Codigo: codigo, Auto: auto, BaseURL: base}, true
}

// ehLegendaAuto diz se a track e geracao automatica por voz.
func ehLegendaAuto(m map[string]any) bool {
	vss := textoDe(m["vssId"])

	return textoDe(m["kind"]) == "asr" || strings.HasPrefix(vss, "a.") || strings.HasPrefix(vss, "a-")
}

// ordenarTracksLegenda prioriza manuais, pt, en e nome.
func ordenarTracksLegenda(tracks []ytLegendaTrack) {
	sort.SliceStable(tracks, func(anterior, seguinte int) bool {
		if tracks[anterior].Auto != tracks[seguinte].Auto {
			return !tracks[anterior].Auto
		}

		pi := strings.HasPrefix(strings.ToLower(tracks[anterior].Codigo), "pt")

		pj := strings.HasPrefix(strings.ToLower(tracks[seguinte].Codigo), "pt")
		if pi != pj {
			return pi
		}

		ei := strings.HasPrefix(strings.ToLower(tracks[anterior].Codigo), "en")

		ej := strings.HasPrefix(strings.ToLower(tracks[seguinte].Codigo), "en")
		if ei != ej {
			return ei
		}

		return tracks[anterior].Nome < tracks[seguinte].Nome
	})
}

func utubeLegendasDaAPI(ctx context.Context, id string) []ytLegendaTrack {
	if tracks := utubePlayerLegendas(ctx, id,
		"ANDROID", "3", "20.10.38",
		"com.google.android.youtube/20.10.38 (Linux; U; Android 11) gzip"); len(tracks) > 0 {
		return tracks
	}

	return utubePlayerLegendas(ctx, id,
		"IOS", "5", "20.10.4",
		"com.google.ios.youtube/20.10.4 (iPhone16,2; U; CPU iOS 18_3_2 like Mac OS X;)")
}

func utubePlayerLegendas(ctx context.Context, id, cliente, clienteID, versao, agente string) []ytLegendaTrack {
	client := map[string]any{
		"clientName": cliente, "clientVersion": versao, "hl": "pt", "gl": "BR",
	}
	switch cliente {
	case "ANDROID":
		client["androidSdkVersion"] = 30
		client["osName"] = "Android"
		client["osVersion"] = "11"
	case "IOS":
		client["deviceModel"] = "iPhone16,2"
		client["osName"] = "iPhone"
		client["osVersion"] = "18.3.2.22D82"
	}

	dados, err := ytPost(ctx, "player", clienteID, versao, agente, map[string]any{
		chaveContexto:    map[string]any{chaveClient: client},
		"videoId":        id,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	if err != nil {
		return nil
	}

	return ytLegendasTracksDoPlayer(dados)
}

func utubeLegendasDe(ctx context.Context, id string) []ytLegendaTrack {
	utubeLegendasMu.Lock()
	if it, ok := utubeLegendasCache[id]; ok && time.Since(it.at) < 10*time.Minute {
		tracks := append([]ytLegendaTrack(nil), it.tracks...)
		utubeLegendasMu.Unlock()

		return tracks
	}
	utubeLegendasMu.Unlock()

	tracks := utubeLegendasDaAPI(ctx, id)
	if len(tracks) == 0 {
		return nil
	}

	utubeGuardarLegendas(id, tracks)

	return tracks
}

func utubeLegendasAtualizadas(ctx context.Context, id string) []ytLegendaTrack {
	tracks := utubeLegendasDaAPI(ctx, id)
	if len(tracks) == 0 {
		return nil
	}

	utubeGuardarLegendas(id, tracks)

	return tracks
}

func utubeGuardarLegendas(id string, tracks []ytLegendaTrack) {
	utubeLegendasMu.Lock()
	defer utubeLegendasMu.Unlock()

	if len(utubeLegendasCache) >= maximoLegendasCache {
		for len(utubeLegendasCache) >= maximoLegendasCache {
			velho, quando := "", time.Time{}
			for k, v := range utubeLegendasCache {
				if velho == "" || v.at.Before(quando) {
					velho, quando = k, v.at
				}
			}

			if velho == "" {
				break
			}

			delete(utubeLegendasCache, velho)
		}
	}

	utubeLegendasCache[id] = utubeLegendasCacheItem{tracks: append([]ytLegendaTrack(nil), tracks...), at: time.Now()}
}

func utubeLegendasPagina(escritor http.ResponseWriter, pedido *http.Request) {
	id := strings.TrimSpace(pedido.URL.Query().Get("v"))
	lang := strings.TrimSpace(pedido.URL.Query().Get("lang"))
	auto := pedido.URL.Query().Get("auto") == "1"

	dados := utubeDados{VideoID: id}
	if id == "" {
		dados.LegendasErro = "Vídeo não informado."
		utubeRenderParcial(escritor, "fragmento-legendas.html", "fragmento-legendas", dados)

		return
	}

	carregar := func(ctx context.Context, tracks []ytLegendaTrack) bool {
		sel := selecionarTrackLegenda(tracks, lang, auto)
		if sel == nil {
			return false
		}

		cues, err := utubeBaixarLegenda(ctx, sel.BaseURL)
		if err != nil || len(cues) == 0 {
			return false
		}

		dados.LegendasCues = cues

		return true
	}
	if !carregar(pedido.Context(), utubeLegendasDe(pedido.Context(), id)) {
		if !carregar(pedido.Context(), utubeLegendasAtualizadas(pedido.Context(), id)) {
			dados.LegendasErro = "Não foi possível carregar as legendas deste vídeo agora. Tente de novo."
		}
	}

	utubeRenderParcial(escritor, "fragmento-legendas.html", "fragmento-legendas", dados)
}

// selecionarTrackLegenda escolhe a track do idioma; a primeira quando vazio.
func selecionarTrackLegenda(tracks []ytLegendaTrack, lang string, auto bool) *ytLegendaTrack {
	for i := range tracks {
		if strings.EqualFold(tracks[i].Codigo, lang) && tracks[i].Auto == auto {
			return &tracks[i]
		}
	}

	if lang == "" && len(tracks) > 0 {
		return &tracks[0]
	}

	return nil
}

// utubeVTT serve a legenda em WebVTT para o player leve (<track>).
func utubeVTT(escritor http.ResponseWriter, pedido *http.Request) {
	id := strings.TrimSpace(pedido.URL.Query().Get("v"))
	lang := strings.TrimSpace(pedido.URL.Query().Get("lang"))
	auto := pedido.URL.Query().Get("auto") == "1"

	var cues []utubeLegendaCue

	if id != "" {
		carregar := func(ctx context.Context, tracks []ytLegendaTrack) bool {
			sel := selecionarTrackLegenda(tracks, lang, auto)
			if sel == nil {
				return false
			}

			var err error

			cues, err = utubeBaixarLegenda(ctx, sel.BaseURL)

			return err == nil && len(cues) > 0
		}
		if !carregar(pedido.Context(), utubeLegendasDe(pedido.Context(), id)) {
			carregar(pedido.Context(), utubeLegendasAtualizadas(pedido.Context(), id))
		}
	}

	var conteudo strings.Builder
	conteudo.WriteString("WEBVTT\n\n")

	for _, c := range cues {
		escreverCueVTT(&conteudo, c)
	}

	escritor.Header().Set("Content-Type", "text/vtt; charset=utf-8")

	if len(cues) > 0 {
		escritor.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		escritor.Header().Set("Cache-Control", "no-store")
	}

	_, _ = io.WriteString(escritor, conteudo.String())
}

// escreverCueVTT acrescenta uma legenda no formato WebVTT.
func escreverCueVTT(construtor *strings.Builder, trecho utubeLegendaCue) {
	txt := strings.ReplaceAll(trecho.Texto, "\n", " ")
	if strings.TrimSpace(txt) == "" {
		return
	}

	construtor.WriteString(formatarTempoVTT(trecho.Inicio) + " --> " +
		formatarTempoVTT(trecho.Fim()) + "\n" + txt + "\n\n")
}

// formatarTempoVTT formata segundos como horario WebVTT.
func formatarTempoVTT(s float64) string {
	if s < 0 {
		s = 0
	}

	ms := int64(s*milissegundosPorSegundo + ajusteArredondaMs)

	return fmt.Sprintf("%02d:%02d:%02d.%03d",
		ms/milissegundosPorHora, ms/60000%segundosPorMinuto, ms/milissegundosPorSegundo%segundosPorMinuto,
		ms%milissegundosPorSegundo,
	)
}

func utubeBaixarLegenda(ctx context.Context, base string) ([]utubeLegendaCue, error) {
	endereco, err := url.Parse(base)
	if err != nil || endereco.Host == "" || !utubeURLPublica(endereco) {
		return nil, erroTexto("endereço de legenda inválido")
	}

	q := endereco.Query()
	q.Set("fmt", "json3")
	endereco.RawQuery = q.Encode()
	// GOD: URL filtrada por utubeURLPublica (sem rede local/privada); sem SSRF por desenho.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endereco.String(), nil) // #nosec G704
	if err != nil {
		return nil, erroTextoF("montar legenda: %w", err)
	}

	req.Header.Set("User-Agent", ytWebUA)
	req.Header.Set("Referer", "https://www.youtube.com/")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	// GOD: executa o request da legenda filtrada acima; sem SSRF por desenho.
	resp, err := utubeHTTP.Do(req) // #nosec G704
	if err != nil {
		return nil, erroTextoF("baixar legenda: %w", err)
	}
	defer resp.Body.Close()

	corpo, err := lerCorpoLegenda(resp)
	if err != nil {
		return nil, err
	}

	if cues := utubeLegendasJSON(corpo); len(cues) > 0 {
		return cues, nil
	}

	if cues := utubeLegendasXML(corpo); len(cues) > 0 {
		return cues, nil
	}

	return nil, erroTexto("formato de legenda desconhecido")
}

// lerCorpoLegenda valida a resposta e devolve o corpo limitado.
func lerCorpoLegenda(resp *http.Response) ([]byte, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, erroTextoF("legenda respondeu %d", resp.StatusCode)
	}

	corpo, err := io.ReadAll(io.LimitReader(resp.Body, (limiteLegendaBytes + 1)))
	if err != nil {
		return nil, erroTextoF("ler legenda: %w", err)
	}

	if len(corpo) > limiteLegendaBytes {
		return nil, erroTexto("legenda grande demais")
	}

	if len(corpo) == 0 {
		return nil, erroTexto("legenda vazia")
	}

	return corpo, nil
}

func utubeLegendasJSON(corpo []byte) []utubeLegendaCue {
	var resposta struct {
		Events []struct {
			TStartMs    int64           `json:"tStartMs"`
			DDurationMs int64           `json:"dDurationMs"`
			AAppend     json.RawMessage `json:"aAppend"`
			Segs        []struct {
				Utf8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if json.Unmarshal(corpo, &resposta) != nil || len(resposta.Events) == 0 {
		return nil
	}

	cues := make([]utubeLegendaCue, 0, len(resposta.Events))
	for _, evento := range resposta.Events {
		var b strings.Builder
		for _, s := range evento.Segs {
			b.WriteString(s.Utf8)
		}

		texto := utubeLimparTexto(limparComentario(b.String()))
		if texto == "" {
			continue
		}

		inicio := float64(evento.TStartMs) / milissegundosPorSegundo
		duracao := float64(evento.DDurationMs) / milissegundosPorSegundo

		if fundirCueLegenda(&cues, texto, inicio, duracao, acrescentaEventoLegenda(evento.AAppend)) {
			continue
		}

		if len(cues) > 0 && cues[len(cues)-1].Texto == texto {
			continue
		}

		cues = append(cues, utubeLegendaCue{Inicio: inicio, Duracao: duracao, Tempo: utubeMMSS(inicio), Texto: texto})
	}

	return cues
}

// acrescentaEventoLegenda diz se o evento continua a legenda anterior.
func acrescentaEventoLegenda(aAppend json.RawMessage) bool {
	if len(aAppend) == 0 {
		return false
	}

	v := strings.TrimSpace(string(aAppend))

	return v != "0" && v != "false" && v != "null" && v != `""`
}

// fundirCueLegenda estende a ultima legenda; true = fundido.
func fundirCueLegenda(cues *[]utubeLegendaCue, texto string, inicio, duracao float64, acrescenta bool) bool {
	if !acrescenta || len(*cues) == 0 {
		return false
	}

	ultimo := &(*cues)[len(*cues)-1]
	if ultimo.Texto != "" && !strings.HasSuffix(ultimo.Texto, texto) {
		ultimo.Texto += " " + texto
	}

	if fim := inicio + duracao; fim > ultimo.Inicio+ultimo.Duracao {
		ultimo.Duracao = fim - ultimo.Inicio
	}

	return true
}

func utubeLegendasXML(corpo []byte) []utubeLegendaCue {
	var srv1 struct {
		Texts []struct {
			Start string `xml:"start,attr"`
			Dur   string `xml:"dur,attr"`
			Text  string `xml:",innerxml"`
		} `xml:"text"`
	}
	if xml.Unmarshal(corpo, &srv1) == nil && len(srv1.Texts) > 0 {
		cues := make([]utubeLegendaCue, 0, len(srv1.Texts))
		for _, t := range srv1.Texts {
			inicio, _ := strconv.ParseFloat(strings.TrimSpace(t.Start), 64)
			duracao, _ := strconv.ParseFloat(strings.TrimSpace(t.Dur), 64)

			texto := utubeLimparTexto(limparComentario(t.Text))
			if texto == "" {
				continue
			}

			cues = append(cues, utubeLegendaCue{Inicio: inicio, Duracao: duracao, Tempo: utubeMMSS(inicio), Texto: texto})
		}

		if len(cues) > 0 {
			return cues
		}
	}

	var srv3 struct {
		Body struct {
			Ps []struct {
				T    string `xml:"t,attr"`
				D    string `xml:"d,attr"`
				Text string `xml:",innerxml"`
			} `xml:"p"`
		} `xml:"body"`
	}
	if xml.Unmarshal(corpo, &srv3) == nil && len(srv3.Body.Ps) > 0 {
		cues := make([]utubeLegendaCue, 0, len(srv3.Body.Ps))
		for _, p := range srv3.Body.Ps {
			milissegundos, _ := strconv.ParseInt(strings.TrimSpace(p.T), 10, 64)
			duracaoMs, _ := strconv.ParseInt(strings.TrimSpace(p.D), 10, 64)

			texto := utubeLimparTexto(limparComentario(p.Text))
			if texto == "" {
				continue
			}

			inicio := float64(milissegundos) / milissegundosPorSegundo
			cues = append(cues, utubeLegendaCue{
				Inicio: inicio, Duracao: float64(duracaoMs) / milissegundosPorSegundo,
				Tempo: utubeMMSS(inicio), Texto: texto,
			})
		}

		return cues
	}

	return nil
}

func utubeRender(escritor http.ResponseWriter, _ *http.Request, pagina string, dados utubeDados) {
	dados.Ano = time.Now().Year()

	modelo, err := utubeTemplate(pagina)
	if err != nil {
		http.Error(escritor, "não consegui montar a página do UTube", http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer
	if err := modelo.ExecuteTemplate(&buf, "base.html", dados); err != nil {
		http.Error(escritor, "não consegui renderizar a página do UTube", http.StatusInternalServerError)

		return
	}

	cabecalho := escritor.Header()
	cabecalho.Set("Content-Type", "text/html; charset=utf-8")
	cabecalho.Set("Cache-Control", "no-store")
	cabecalho.Set("X-Content-Type-Options", "nosniff")
	cabecalho.Set("X-Frame-Options", "DENY")
	cabecalho.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	_, _ = escritor.Write(buf.Bytes())
}

func utubeRenderParcial(escritor http.ResponseWriter, pagina, nome string, dados utubeDados) {
	modelo, err := utubeTemplate(pagina)
	if err != nil {
		http.Error(escritor, "não consegui montar a página do UTube", http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer
	if err := modelo.ExecuteTemplate(&buf, nome, dados); err != nil {
		http.Error(escritor, "não consegui renderizar a página do UTube", http.StatusInternalServerError)

		return
	}

	cabecalho := escritor.Header()
	cabecalho.Set("Content-Type", "text/html; charset=utf-8")
	cabecalho.Set("Cache-Control", "no-store")
	cabecalho.Set("X-Content-Type-Options", "nosniff")

	_, _ = escritor.Write(buf.Bytes())
}

func utubeCards(itens []ytItem) []utubeCard {
	out := make([]utubeCard, 0, len(itens))
	for idx, item := range itens {
		if item.ID == "" {
			continue
		}

		out = append(out, utubeCard{
			Titulo:       primeiroNaoVazio(item.Titulo, "Vídeo"),
			Canal:        primeiroNaoVazio(item.Canal, "YouTube"),
			CanalURL:     utubeCanalURL(item.CanalID, item.Canal),
			Avatar:       item.CanalAvatar,
			Iniciais:     utubeIniciais(primeiroNaoVazio(item.Canal, "YouTube")),
			Views:        item.Views,
			Publicado:    item.Publicado,
			Duracao:      item.Duracao,
			Thumb:        utubeThumb(item.ID, false),
			ThumbPequena: utubeThumb(item.ID, true),
			URL:          "/yt/watch?v=" + url.QueryEscape(item.ID),
			Descricao:    item.Descricao,
			Cor:          utubeCor(primeiroNaoVazio(item.Canal, "YouTube")),
			AoVivo:       item.AoVivo,
			Eager:        idx < quantidadeEagerCards,
		})
	}

	return out
}

func utubeCanalURL(id, nome string) string {
	if id != "" {
		return "/yt/channel/" + url.PathEscape(id)
	}

	if nome != "" {
		return "/yt/results?q=" + url.QueryEscape(nome)
	}

	return "/yt"
}

func utubeThumb(id string, pequena bool) string {
	if id == "" {
		return ""
	}

	if pequena {
		return "https://i.ytimg.com/vi/" + url.PathEscape(id) + "/mqdefault.jpg"
	}

	return "https://i.ytimg.com/vi/" + url.PathEscape(id) + "/hqdefault.jpg"
}

func utubeIniciais(texto string) string {
	texto = strings.TrimSpace(strings.TrimPrefix(texto, "@"))
	if texto == "" {
		return "?"
	}

	for _, r := range texto {
		return strings.ToUpper(string(r))
	}

	return "?"
}

func utubeCor(s string) int {
	acumulado := 0
	for _, r := range s {
		acumulado = acumulado*baseHashCor + int(r)
	}

	if acumulado < 0 {
		acumulado = -acumulado
	}

	return acumulado % totalCoresCanal
}

func utubeDuracao(seg int) string {
	if seg <= 0 {
		return ""
	}

	h := seg / segundosPorHora
	minutos := (seg % segundosPorHora) / segundosPorMinuto

	s := seg % segundosPorMinuto
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, minutos, s)
	}

	return fmt.Sprintf("%d:%02d", minutos, s)
}

func utubeMMSS(seg float64) string {
	t := max(int(seg+meioSegundoAjuste), 0)

	h := t / segundosPorHora
	minutos := (t % segundosPorHora) / segundosPorMinuto

	s := t % segundosPorMinuto
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, minutos, s)
	}

	return fmt.Sprintf("%d:%02d", minutos, s)
}

func utubeViews(n int64) string {
	if n <= 0 {
		return ""
	}

	return formatarNumero(n) + " visualizações"
}

func utubeQuando(milissegundos int64, fallback string) string {
	if fallback != "" {
		return utubeTempoComentario(fallback)
	}

	if milissegundos <= 0 {
		return ""
	}

	duracao := max(time.Since(time.UnixMilli(milissegundos)), 0)

	switch {
	case duracao < time.Minute:
		return "agora"
	case duracao < time.Hour:
		return "há " + strconv.Itoa(int(duracao.Minutes())) + " min"
	case duracao < horasPorDia*time.Hour:
		return "há " + strconv.Itoa(int(duracao.Hours())) + " h"
	case duracao < 30*horasPorDia*time.Hour:
		return "há " + strconv.Itoa(int(duracao.Hours()/horasPorDia)) + " dias"
	case duracao < 365*horasPorDia*time.Hour:
		return "há " + strconv.Itoa(int(duracao.Hours()/(horasPorDia*diasPorMes))) + " meses"
	default:
		return "há " + strconv.Itoa(int(duracao.Hours()/(horasPorDia*diasPorAno))) + " anos"
	}
}

func ytIDValido(s string) bool {
	if len(s) != tamanhoIDYouTube {
		return false
	}

	for _, c := range s {
		if !ehCaractereIDYouTube(c) {
			return false
		}
	}

	return true
}

// ehCaractereIDYouTube diz se o rune vale num id de video.
func ehCaractereIDYouTube(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_'
}

func utubeIDDeURL(endereco string) string {
	endereco = strings.TrimSpace(endereco)
	if endereco == "" {
		return ""
	}

	if ytIDValido(endereco) {
		return endereco
	}

	p, err := url.Parse(endereco)
	if err != nil {
		return ""
	}

	return idDeLinkYouTube(p)
}

// idDeLinkYouTube extrai o id de video de links do YouTube.
func idDeLinkYouTube(link *url.URL) string {
	host := strings.ToLower(link.Hostname())
	partes := strings.Split(strings.Trim(link.Path, "/"), "/")

	if id := idDeCurtoYouTube(host, partes); id != "" {
		return id
	}

	if v := link.Query().Get("v"); ytIDValido(v) {
		return v
	}

	if len(partes) >= 2 && caminhoVideoYouTube(partes[0]) && ytIDValido(partes[1]) {
		return partes[1]
	}

	return ""
}

// idDeCurtoYouTube extrai o id de links youtu.be.
func idDeCurtoYouTube(host string, partes []string) string {
	if host != "youtu.be" && !strings.HasSuffix(host, ".youtu.be") {
		return ""
	}

	if len(partes) >= 1 && ytIDValido(partes[0]) {
		return partes[0]
	}

	return ""
}

// caminhoVideoYouTube diz se o primeiro segmento e pagina de video.
func caminhoVideoYouTube(primeiro string) bool {
	return primeiro == idWatch || primeiro == "shorts" ||
		primeiro == "embed" || primeiro == "live"
}

func utubeCanalIDDeURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}

	partes := strings.Split(strings.Trim(p.Path, "/"), "/")
	if len(partes) >= 2 && partes[0] == "channel" {
		return partes[1]
	}

	return ""
}

func primeiroNaoVazio(valor, padrao string) string {
	if strings.TrimSpace(valor) == "" {
		return padrao
	}

	return valor
}

func utubeCategoriaDaBusca(consulta string) string {
	switch strings.ToLower(strings.TrimSpace(consulta)) {
	case "música", "musica":
		return "Música"
	case "jogos":
		return "Jogos"
	case "notícias", "noticias":
		return "Notícias"
	}

	return ""
}

//nolint:gochecknoglobals // Categorias SponsorBlock: tabela fixa, somente leitura.
var utubeCategorias = map[string]utubeCategoria{
	"sponsor":          {"Patrocínio", "sb-sponsor"},
	"selfpromo":        {"Autopromoção", "sb-selfpromo"},
	"interaction":      {"Interação", "sb-interaction"},
	"intro":            {"Introdução", "sb-intro"},
	"outro":            {"Encerramento", "sb-outro"},
	"preview":          {"Prévia", "sb-preview"},
	"music_offtopic":   {"Música fora do tema", "sb-music_offtopic"},
	"filler":           {"Enrolação", "sb-filler"},
	"poi_highlight":    {"Destaque", "sb-poi_highlight"},
	"exclusive_access": {"Acesso exclusivo", "sb-exclusive_access"},
	"hook":             {"Gancho", "sb-hook"},
	"chapter":          {"Capítulo", "sb-chapter"},
}

func utubeCategoriaDe(cat string) utubeCategoria {
	if c, ok := utubeCategorias[cat]; ok {
		return c
	}

	return utubeCategoria{Nome: primeiroNaoVazio(cat, "Trecho"), Classe: "sb-chapter"}
}

func utubeSegmentos(video *ytVideo) ([]utubeSegmento, []utubeCategoria, float64) {
	if video == nil || len(video.Patrocinio) == 0 {
		return nil, nil, 0
	}

	dur := duracaoSegmentosYouTube(video)
	if dur <= 0 {
		return nil, nil, 0
	}

	segmentos, categorias := montarSegmentosYouTube(video, dur)
	if len(segmentos) == 0 {
		return nil, nil, 0
	}

	return segmentos, categorias, dur
}

// duracaoSegmentosYouTube resolve a duracao pelo video ou patrocinios.
func duracaoSegmentosYouTube(video *ytVideo) float64 {
	dur := video.Duracao
	if dur > 0 {
		return dur
	}

	for _, s := range video.Patrocinio {
		if s.Fim > dur {
			dur = s.Fim
		}
	}

	return dur
}

// montarSegmentosYouTube converte patrocinios em segmentos de barra.
func montarSegmentosYouTube(v *ytVideo, dur float64) ([]utubeSegmento, []utubeCategoria) {
	segmentos := make([]utubeSegmento, 0, len(v.Patrocinio))
	vistos := map[string]bool{}
	categorias := make([]utubeCategoria, 0, capacidadeCategorias)

	for _, trecho := range v.Patrocinio {
		ini, fim, ok := recortarSegmentoYouTube(trecho.Inicio, trecho.Fim, dur)
		if !ok {
			continue
		}

		cat := utubeCategoriaDe(trecho.Categoria)
		segmentos = append(segmentos, novoSegmentoYouTube(cat, ini, fim, dur))

		if !vistos[trecho.Categoria] {
			vistos[trecho.Categoria] = true

			categorias = append(categorias, cat)
		}
	}

	return segmentos, categorias
}

// recortarSegmentoYouTube prende o segmento a duracao; false = vazio.
func recortarSegmentoYouTube(ini, fim, dur float64) (float64, float64, bool) {
	if ini < 0 {
		ini = 0
	}

	if fim > dur {
		fim = dur
	}

	if fim <= ini {
		return 0, 0, false
	}

	return ini, fim, true
}

// novoSegmentoYouTube monta o segmento com largura minima visivel.
func novoSegmentoYouTube(cat utubeCategoria, ini, fim, dur float64) utubeSegmento {
	largura := (fim - ini) / dur * porcentagemSegmento
	if largura < larguraMinimaSegmento {
		largura = larguraMinimaSegmento
	}

	return utubeSegmento{
		Classe: cat.Classe,
		Left:   fmt.Sprintf("%.4f", ini/dur*porcentagemSegmento),
		Width:  fmt.Sprintf("%.4f", largura),
		Nome:   cat.Nome,
		Tempo:  utubeMMSS(ini) + " – " + utubeMMSS(fim),
		Inicio: ini,
		Fim:    fim,
	}
}

type pipedTrending struct {
	URL            string `json:"url"`
	Title          string `json:"title"`
	UploaderName   string `json:"uploaderName"`
	UploaderURL    string `json:"uploaderUrl"`
	UploaderAvatar string `json:"uploaderAvatar"`
	Duration       int    `json:"duration"`
	Views          int64  `json:"views"`
	Uploaded       int64  `json:"uploaded"`
	UploadedDate   string `json:"uploadedDate"`
}

func utubeTrending(ctx context.Context) ([]ytItem, error) {
	utubeTrendMu.Lock()
	if len(utubeTrendCache) > 0 && time.Since(utubeTrendAt) < 10*time.Minute {
		saida := append([]ytItem(nil), utubeTrendCache...)
		utubeTrendMu.Unlock()

		return saida, nil
	}
	utubeTrendMu.Unlock()

	var ultimo error

	for _, base := range pipedInstancias {
		itens, err := buscarTrendingInstancia(ctx, base)
		if err != nil {
			ultimo = err

			continue
		}

		if len(itens) > 0 {
			guardarTrendingCache(itens)

			return itens, nil
		}
	}

	if ultimo == nil {
		ultimo = erroTexto("feed indisponivel")
	}

	return nil, ultimo
}

// buscarTrendingInstancia baixa o em alta de uma instancia Piped.
func buscarTrendingInstancia(ctx context.Context, base string) ([]ytItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/trending?region=BR", nil)
	if err != nil {
		return nil, erroTextoF("montar pedido em-alta: %w", err)
	}

	resp, err := utubeHTTP.Do(req)
	if err != nil {
		return nil, erroTextoF("buscar em-alta: %w", err)
	}

	if resp.StatusCode != codigoHTTPSucessoTube {
		resp.Body.Close()

		return nil, erroTextoF("piped %d", resp.StatusCode)
	}

	var lista []pipedTrending

	err = json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaTube)).Decode(&lista)
	resp.Body.Close()

	if err != nil {
		return nil, erroTextoF("decodificar em-alta: %w", err)
	}

	return converterTrendingPiped(lista), nil
}

// converterTrendingPiped converte o em alta do Piped em itens.
func converterTrendingPiped(lista []pipedTrending) []ytItem {
	itens := make([]ytItem, 0, len(lista))
	for _, emAlta := range lista {
		id := utubeIDDeURL(emAlta.URL)
		if id == "" {
			continue
		}

		item := ytItem{
			ID:          id,
			Titulo:      emAlta.Title,
			Canal:       emAlta.UploaderName,
			CanalID:     utubeCanalIDDeURL(emAlta.UploaderURL),
			CanalAvatar: emAlta.UploaderAvatar,
			Views:       utubeViews(emAlta.Views),
			Publicado:   utubeQuando(emAlta.Uploaded, emAlta.UploadedDate),
			AoVivo:      emAlta.Duration < 0,
		}
		if emAlta.Duration > 0 {
			item.Duracao = utubeDuracao(emAlta.Duration)
		}

		itens = append(itens, item)
	}

	return itens
}

// guardarTrendingCache salva o em alta por 10 minutos.
func guardarTrendingCache(itens []ytItem) {
	utubeTrendMu.Lock()

	utubeTrendCache = append([]ytItem(nil), itens...)
	utubeTrendAt = time.Now()
	utubeTrendMu.Unlock()
}

//nolint:gochecknoglobals // Cache de canais + mutex: estado de processo.
var (
	utubeCanalMu    sync.Mutex
	utubeCanalCache = map[string]struct {
		c   *utubeCanal
		exp time.Time
	}{}
)

func utubeCanalInfo(ctx context.Context, id string) *utubeCanal {
	if id == "" {
		return nil
	}

	if c, ok := consultarCanalCache(id); ok {
		return c
	}

	for _, base := range pipedInstancias {
		canal, ok := buscarCanalInstancia(ctx, id, base)
		if !ok {
			continue
		}

		guardarCanalCache(id, canal)
		cp := *canal

		return &cp
	}

	return nil
}

// consultarCanalCache devolve a copia do canal em cache; false = sem cache.
func consultarCanalCache(id string) (*utubeCanal, bool) {
	utubeCanalMu.Lock()
	defer utubeCanalMu.Unlock()

	if it, ok := utubeCanalCache[id]; ok && time.Now().Before(it.exp) {
		cp := *it.c

		return &cp, true
	}

	return nil, false
}

// buscarCanalInstancia baixa o canal de uma instancia Piped; false = falhou.
func buscarCanalInstancia(ctx context.Context, id, base string) (*utubeCanal, bool) {
	// GOD: base de lista fixa interna e id escapado; sem SSRF por desenho.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/channel/"+url.PathEscape(id), nil) // #nosec G704
	if err != nil {
		return nil, false
	}

	resp, err := utubeHTTP.Do(req) // #nosec G704
	if err != nil {
		return nil, false
	}

	if resp.StatusCode != codigoHTTPSucessoTube {
		resp.Body.Close()

		return nil, false
	}

	var resposta struct {
		Nome      string `json:"name"`
		Descricao string `json:"description"`
		Avatar    string `json:"avatarUrl"`
		Banner    string `json:"bannerUrl"`
		Inscritos int64  `json:"subscriberCount"`
	}

	err = json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaTube)).Decode(&resposta)
	resp.Body.Close()

	if err != nil {
		return nil, false
	}

	return &utubeCanal{
		Nome:      resposta.Nome,
		Avatar:    resposta.Avatar,
		Banner:    utubeBanner(resposta.Banner),
		Descricao: resposta.Descricao,
		Inscritos: utubeInscritos(resposta.Inscritos),
		Iniciais:  utubeIniciais(resposta.Nome),
		Cor:       utubeCor(resposta.Nome),
	}, true
}

// guardarCanalCache salva o canal podando expirados quando cheio.
func guardarCanalCache(id string, canal *utubeCanal) {
	utubeCanalMu.Lock()
	defer utubeCanalMu.Unlock()

	if len(utubeCanalCache) >= maximoCanaisCache {
		agora := time.Now()
		for k, it := range utubeCanalCache {
			if !agora.Before(it.exp) {
				delete(utubeCanalCache, k)
			}
		}
	}

	utubeCanalCache[id] = struct {
		c   *utubeCanal
		exp time.Time
	}{canal, time.Now().Add(validadeCanalCache)}
}

func utubeBanner(endereco string) string {
	if endereco == "" {
		return ""
	}

	endereco = strings.Replace(endereco, "w2560", "w1060", 1)

	return endereco
}

func utubeInscritos(n int64) string {
	if n <= 0 {
		return ""
	}

	return formatarNumero(n) + " inscritos"
}

// sondarStream confere se a URL toca (Range de 1 byte).
func sondarStream(ctx context.Context, u string) bool {
	// GOD: sonda URL de stream resolvida por yt-dlp/Piped (chamadores internos); sem SSRF por desenho.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil) // #nosec G704
	if err != nil {
		return false
	}

	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", ytWebUA)
	// GOD: executa o request da sonda (URL de stream resolvida); sem SSRF por desenho.
	resp, err := utubeHTTP.Do(req) // #nosec G704
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return false
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "video/") &&
		!strings.HasPrefix(ct, "audio/") {
		return false
	}

	return true
}

//exhaustruct:ignore // Resultado de stream: construtor parcial por desenho.
type ytStreamRes struct {
	url  string
	erro string
}

func utubeVideoRapido(ctx context.Context, id string) *ytVideo {
	if v := consultarVideoCache(id); v != nil {
		return v
	}

	tipo := make(chan *ytVideo, 1)
	go func(ctx context.Context) { tipo <- ytDadosDoVideo(ctx, id) }(ctx)

	select {
	case v := <-tipo:
		if videoCacheavel(id, v) {
			guardarVideoCache(id, v)
		}

		return v
	case <-time.After(esperaVideoRapido):
		return &ytVideo{ID: id, Erro: "tempo esgotado"}
	}
}

// consultarVideoCache devolve o video em cache; nil = sem cache.
func consultarVideoCache(id string) *ytVideo {
	if id == "" {
		return nil
	}

	ytVideoMu.Lock()
	defer ytVideoMu.Unlock()

	if it, ok := ytVideoCache[id]; ok && time.Now().Before(it.exp) {
		return it.v
	}

	return nil
}

// videoCacheavel diz se o video pode entrar no cache.
func videoCacheavel(id string, v *ytVideo) bool {
	return id != "" && v != nil && v.ID != "" && v.Erro == ""
}

// guardarVideoCache salva o video podando expirados quando cheio.
func guardarVideoCache(id string, video *ytVideo) {
	ytVideoMu.Lock()
	defer ytVideoMu.Unlock()

	if len(ytVideoCache) >= maximoVideosCache {
		agora := time.Now()
		for k, it := range ytVideoCache {
			if !agora.Before(it.exp) {
				delete(ytVideoCache, k)
			}
		}
	}

	ytVideoCache[id] = struct {
		v   *ytVideo
		exp time.Time
	}{video, time.Now().Add(validadeVideoCache)}
}

func utubeComentariosRapido(ctx context.Context, id, token string) ([]ytComentario, string, error) {
	tipo := make(chan struct {
		itens []ytComentario
		prox  string
		err   error
	}, 1)
	go func(ctx context.Context) {
		itens, prox, err := ytComentarios(ctx, id, token)
		tipo <- struct {
			itens []ytComentario
			prox  string
			err   error
		}{itens, prox, err}
	}(ctx)

	select {
	case resposta := <-tipo:
		for idx := range resposta.itens {
			resposta.itens[idx].Autor = strings.TrimSpace(resposta.itens[idx].Autor)
			resposta.itens[idx].Texto = utubeLimparTexto(resposta.itens[idx].Texto)
			resposta.itens[idx].Quando = utubeTempoComentario(resposta.itens[idx].Quando)
			resposta.itens[idx].Iniciais = utubeIniciais(resposta.itens[idx].Autor)
			resposta.itens[idx].Cor = utubeCor(resposta.itens[idx].Autor)
			resposta.itens[idx].VideoID = id
		}

		return resposta.itens, resposta.prox, resposta.err
	case <-time.After(esperaComentarios):
		return nil, "", erroTexto("tempo esgotado")
	}
}

func utubeLimparTexto(texto string) string {
	texto = strings.ReplaceAll(texto, "\r\n", "\n")
	linhas := strings.Split(texto, "\n")
	saida := make([]string, 0, len(linhas))
	vazio := false

	for _, linha := range linhas {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			if vazio {
				continue
			}

			vazio = true
		} else {
			vazio = false
		}

		saida = append(saida, linha)
	}

	return strings.TrimSpace(strings.Join(saida, "\n"))
}

func utubeTempoComentario(texto string) string {
	editado := strings.Contains(texto, "(edited)")
	texto = strings.TrimSpace(strings.ReplaceAll(texto, "(edited)", ""))

	if texto == "" || texto == "just now" {
		return finalizarTempoComentario(editado, "agora")
	}

	p := strings.Fields(texto)
	if len(p) != 3 || p[2] != "ago" {
		return finalizarTempoComentario(editado, texto)
	}

	return finalizarTempoComentario(editado, fraseTempoComentario(p, texto))
}

// finalizarTempoComentario aplica o sufixo de editado quando preciso.
func finalizarTempoComentario(editado bool, t string) string {
	if editado && t != "" {
		return t + " · editado"
	}

	return t
}

// fraseTempoComentario traduz "3 hours ago" em relativo portugues.
func fraseTempoComentario(p []string, original string) string {
	if nome, ok := traduzirUnidadeTempo(strings.TrimSuffix(p[1], "s"), p[0]); ok {
		return "há " + p[0] + " " + nome
	}

	return original
}

// traduzirUnidadeTempo traduz a unidade inglesa; false = desconhecida.
func traduzirUnidadeTempo(unidade, quant string) (string, bool) {
	switch unidade {
	case "second":
		unidade = "segundo"
	case "minute":
		unidade = "minuto"
	case "hour":
		unidade = "hora"
	case "day":
		unidade = "dia"
	default:
		return traduzirUnidadeLonga(unidade, quant)
	}

	if quant != "1" {
		unidade += "s"
	}

	return unidade, true
}

// traduzirUnidadeLonga traduz semana, mes e ano; false = desconhecida.
func traduzirUnidadeLonga(unidade, quant string) (string, bool) {
	switch unidade {
	case "week":
		unidade = "semana"
	case "month":
		return unidadeMesTempo(quant)
	case "year":
		unidade = "ano"
	default:
		return "", false
	}

	if quant != "1" && unidade != "meses" {
		unidade += "s"
	}

	return unidade, true
}

// unidadeMesTempo traduz o mes com singular especial.
func unidadeMesTempo(quant string) (string, bool) {
	if quant == "1" {
		return "mês", true
	}

	return "meses", true
}
