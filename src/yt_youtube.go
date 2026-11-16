//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	codigoHTTPSucessoYouTube = 200
	limiteRespostaYouTube    = 4 << 20
	limitePaginaYouTube      = 8 << 20
	maximoRelacionados       = 20
	extrasParalelosVideo     = 2
	limiteMilhaoViews        = 1000000
	limiteMilharViews        = 1000
	esperaLimpezaYouTube     = 1500 * time.Millisecond
)

func ytPost(ctx context.Context, endpoint, cliente, versao, agente string,
	corpo map[string]any,
) (map[string]any, error) {
	b, err := json.Marshal(corpo)
	if err != nil {
		return nil, err //nolint:wrapcheck // Serializacao em memoria do corpo: erro autoexplicativo.
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ytBase+endpoint+"?prettyPrint=false", bytes.NewReader(b))
	if err != nil {
		return nil, erroTextoF("montar pedido youtube: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", agente)
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	req.Header.Set("X-Youtube-Client-Name", cliente)
	req.Header.Set("X-Youtube-Client-Version", versao)

	resp, err := ytHTTP.Do(req)
	if err != nil {
		return nil, erroTextoF("youtube: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != codigoHTTPSucessoYouTube {
		return nil, erroTextoF("youtube respondeu %d", resp.StatusCode)
	}

	var saida map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaYouTube)).Decode(&saida); err != nil {
		return nil, erroTextoF("decodificar youtube: %w", err)
	}

	return saida, nil
}

func ytContextoWEB() map[string]any {
	return map[string]any{
		chaveClient: map[string]any{
			"clientName": "WEB", "clientVersion": "2.20260601.00.00",
			"hl": "pt", "gl": "BR",
		},
	}
}

func ytPaginaDoVideo(ctx context.Context, id string) (map[string]any, map[string]any, error) {
	// GOD: host fixo (youtube.com) e id escapado; sem SSRF por desenho.
	urlPagina := "https://www.youtube.com/watch?v=" + url.QueryEscape(id) + "&hl=pt&gl=BR"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlPagina, nil) // #nosec G704
	if err != nil {
		return nil, nil, erroTextoF("montar pagina: %w", err)
	}

	req.Header.Set("User-Agent", ytWebUA)
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	// GOD: executa o request montado acima (host fixo); sem SSRF por desenho.
	resp, err := ytHTTP.Do(req) // #nosec G704
	if err != nil {
		return nil, nil, erroTextoF("baixar pagina: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != codigoHTTPSucessoYouTube {
		return nil, nil, erroTextoF("pagina respondeu %d", resp.StatusCode)
	}

	corpo, err := io.ReadAll(io.LimitReader(resp.Body, limitePaginaYouTube))
	if err != nil {
		return nil, nil, erroTextoF("ler pagina: %w", err)
	}

	pagina := string(corpo)

	dados, err := ytObjetoDoHTML(pagina, "ytInitialData")
	if err != nil {
		return nil, nil, err
	}

	player, _ := ytObjetoDoHTML(pagina, "ytInitialPlayerResponse")

	return dados, player, nil
}

func ytObjetoDoHTML(texto, chave string) (map[string]any, error) {
	pos := strings.Index(texto, chave)
	if pos < 0 {
		return nil, erroTextoF("sem %s", chave)
	}

	pos = strings.Index(texto[pos:], "{")
	if pos < 0 {
		return nil, erroTexto("sem json")
	}

	ini := strings.Index(texto, chave) + pos

	fim := varrerJSONDoHTML(texto, ini)
	if fim < 0 {
		return nil, erroTexto("json truncado")
	}

	var dados map[string]any
	if err := json.Unmarshal([]byte(texto[ini:fim]), &dados); err != nil {
		return nil, erroTextoF("json pagina: %w", err)
	}

	return dados, nil
}

// varrerJSONDoHTML acha o fim do objeto JSON ignorando strings; -1 = truncado.
func varrerJSONDoHTML(texto string, ini int) int {
	prof := 0
	fim := -1
	dentroString := false
	escape := false

	for idx := ini; idx < len(texto); idx++ {
		caractere := texto[idx]
		if dentroString {
			dentroString, escape = avancarStringJSON(caractere, escape)

			continue
		}

		switch caractere {
		case '"':
			dentroString = true
		case '{':
			prof++
		case '}':
			prof--
			if prof == 0 {
				fim = idx + 1
			}
		}

		if fim > 0 {
			break
		}
	}

	return fim
}

// avancarStringJSON consome um caractere dentro de string JSON.
func avancarStringJSON(caractere byte, escape bool) (bool, bool) {
	if escape {
		return true, false
	}

	if caractere == '\\' {
		return true, true
	}

	if caractere == '"' {
		return false, false
	}

	return true, false
}

func idDeVideo(dados map[string]any) string {
	for _, chave := range []string{"videoId"} {
		if s, ok := dados[chave].(string); ok && s != "" {
			return s
		}
	}

	if s := textoDe(caminho(dados, "navigationEndpoint", "watchEndpoint", "videoId")); s != "" {
		return s
	}

	return textoDe(caminho(dados, "onTap", "innertubeCommand", "watchEndpoint", "videoId"))
}

func miniaturaDe(dados any) string {
	melhor := ""

	switch dado := dados.(type) {
	case map[string]any:
		if arr, ok := dado["thumbnails"].([]any); ok && len(arr) > 0 {
			if u := textoDe(caminho(arr[len(arr)-1], "url")); u != "" {
				melhor = u
			}
		}

		for _, f := range dado {
			if melhor == "" {
				melhor = miniaturaDe(f)
			}
		}
	case []any:
		for _, f := range dado {
			if melhor == "" {
				melhor = miniaturaDe(f)
			}
		}
	}

	return melhor
}

func itemDeVideo(dados map[string]any) ytItem {
	item := ytItem{
		ID:      idDeVideo(dados),
		Titulo:  textoDe(dados["title"]),
		AoVivo:  dados["isLiveNow"] == true || dados["badges"] != nil && textoDe(dados["badges"]) != "",
		Duracao: textoDe(caminho(dados, "lengthText")),
		Views:   textoDe(caminho(dados, "viewCountText")),
		Canal:   textoDe(caminho(dados, "ownerText", "runs", 0, "text")),
	}
	if item.Titulo == "" {
		completarTituloVideo(&item, dados)
	}

	completarCanalVideo(&item, dados)
	completarIDsVideo(&item, dados)

	item.Descricao = textoDe(caminho(dados, "detailedMetadataSnippets", 0, "snippetText"))

	item.CanalAvatar = miniaturaDe(caminho(dados, "channelThumbnailSupportedRenderers",
		"channelThumbnailWithLinkRenderer", "thumbnail"))
	if item.CanalAvatar == "" {
		item.CanalAvatar = miniaturaDe(dados["channelThumbnail"])
	}

	return item
}

// completarTituloVideo busca titulo e views alternativos do video.
func completarTituloVideo(item *ytItem, dados map[string]any) {
	item.Titulo = textoDe(caminho(dados, "headline"))
	if item.Titulo == "" {
		item.Titulo = textoDe(caminho(dados, "overlayMetadata", "primaryText"))
	}

	if item.Views == "" {
		item.Views = textoDe(caminho(dados, "overlayMetadata", "secondaryText"))
	}
}

// completarCanalVideo busca canal e contadores alternativos do video.
func completarCanalVideo(item *ytItem, dados map[string]any) {
	if item.Canal == "" {
		item.Canal = textoDe(caminho(dados, "longBylineText", "runs", 0, "text"))
	}

	if item.Canal == "" {
		item.Canal = textoDe(caminho(dados, "shortBylineText", "runs", 0, "text"))
	}

	if item.Views == "" {
		item.Views = textoDe(caminho(dados, "shortViewCountText"))
	}
}

// completarIDsVideo busca publicado, canal e id alternativos do video.
func completarIDsVideo(item *ytItem, dados map[string]any) {
	if item.Publicado == "" {
		item.Publicado = textoDe(caminho(dados, "publishedTimeText"))
	}

	item.CanalID = textoDe(caminho(dados, "ownerText", "runs", 0, "navigationEndpoint", "browseEndpoint", "browseId"))
	if item.CanalID == "" {
		item.CanalID = textoDe(caminho(dados, "longBylineText", "runs", 0,
			"navigationEndpoint", "browseEndpoint", "browseId"))
	}

	if item.ID == "" {
		item.ID = textoDe(caminho(dados, "onTap", "innertubeCommand", "reelWatchEndpoint", "videoId"))
	}

	if item.ID == "" {
		if e := textoDe(caminho(dados, "entityId")); strings.HasPrefix(e, "shorts-shelf-item-") {
			item.ID = strings.TrimPrefix(e, "shorts-shelf-item-")
		}
	}
}

func ytBuscar(ctx context.Context, q string) ([]ytItem, error) {
	return ytBuscarParams(ctx, q, ytParamsPadrao)
}

func ytBuscarParams(ctx context.Context, consulta, params string) ([]ytItem, error) {
	if params == "" {
		params = ytParamsPadrao
	}

	corpo := map[string]any{
		chaveContexto: ytContextoWEB(), "query": consulta,
		"params": params,
	}

	dados, err := ytPost(ctx, idSearch, "1", "2.20250101.00.00", ytWebUA, corpo)
	if err != nil {
		return nil, err
	}

	var (
		itens []ytItem
		vids  []map[string]any
	)
	acharTodos(dados, "videoRenderer", &vids, 0)

	for _, v := range vids {
		if it := itemDeVideo(v); it.ID != "" {
			itens = append(itens, it)
		}
	}

	if len(itens) == 0 {
		itens = itensDeLockups(dados)
	}

	return itens, nil
}

func ytDadosDoVideo(ctx context.Context, id string) *ytVideo {
	video := &ytVideo{ID: id}
	pag, player, errPag := ytPaginaDoVideo(ctx, id)
	video.Legendas = ytLegendasDoPlayer(player)

	if errPag == nil {
		preencherVideoPrimario(video, pag)
		preencherVideoSecundario(video, pag)
		preencherVideoDetalhes(video, pag)
		preencherVideoRelacionados(video, pag, id)
	} else {
		video.Erro = errPag.Error()
	}

	aguardarExtrasVideo(ctx, video, id)

	return video
}

// preencherVideoPrimario extrai titulo, views e likes do video.
func preencherVideoPrimario(video *ytVideo, pag map[string]any) {
	var prim []map[string]any
	acharTodos(pag, "videoPrimaryInfoRenderer", &prim, 0)

	if len(prim) == 0 {
		return
	}

	primeiro := prim[0]
	video.Titulo = textoDe(primeiro["title"])
	video.Views = textoDe(caminho(primeiro, "viewCount", "videoViewCountRenderer", "viewCount"))
	video.Data = textoDe(caminho(primeiro, "dateText"))

	if s := textoDe(caminho(primeiro, "videoActions", "menuRenderer", "topLevelButtons", 0,
		"segmentedLikeDislikeButtonViewModel", "likeButtonViewModel", "likeButtonViewModel",
		"toggleButtonViewModel", "toggleButtonViewModel", "defaultButtonViewModel",
		"buttonViewModel", "title")); s != "" {
		video.Likes = s
	}

	if video.Likes == "" {
		video.Likes = textoDe(caminho(primeiro, "videoActions", "menuRenderer", "topLevelButtons", 0,
			"segmentedLikeDislikeButtonViewModel", "likeButtonViewModel", "likeButtonViewModel",
			"toggleButtonViewModel", "toggleButtonViewModel", "defaultButtonViewModel",
			"segmentedButtonViewModel", "segmentedButton", "segments", 0,
			"segmentedButtonSegment", "segment", "buttonViewModel", "title"))
	}
}

// preencherVideoSecundario extrai canal e descricao do video.
func preencherVideoSecundario(video *ytVideo, pag map[string]any) {
	var seg []map[string]any
	acharTodos(pag, "videoSecondaryInfoRenderer", &seg, 0)

	if len(seg) == 0 {
		return
	}

	segundo := seg[0]
	video.Canal = textoDe(caminho(segundo, "owner", "videoOwnerRenderer", "title", "runs", 0, "text"))
	video.CanalID = textoDe(caminho(segundo, "owner", "videoOwnerRenderer",
		"navigationEndpoint", "browseEndpoint", "browseId"))
	video.CanalAvatar = miniaturaDe(caminho(segundo, "owner", "videoOwnerRenderer", "thumbnail"))
	video.Inscritos = textoDe(caminho(segundo, "owner", "videoOwnerRenderer", "subscriberCountText"))

	video.Descricao = textoDe(caminho(segundo, "attributedDescription", "content"))
	if video.Descricao == "" {
		video.Descricao = textoDe(caminho(segundo, "description", "runs"))
	}
}

// preencherVideoDetalhes completa com os detalhes estaticos do video.
func preencherVideoDetalhes(video *ytVideo, pag map[string]any) {
	if video.Titulo == "" {
		if m := textoDe(caminho(pag, "videoDetails", "title")); m != "" {
			video.Titulo = m
		}
	}

	if video.Canal == "" {
		video.Canal = textoDe(caminho(pag, "videoDetails", "author"))
		video.CanalID = textoDe(caminho(pag, "videoDetails", "channelId"))
	}

	if video.Views == "" {
		if n, ok := caminho(pag, "videoDetails", "viewCount").(string); ok {
			video.Views = n + " visualizacoes"
		}
	}

	if video.Descricao == "" {
		if d, ok := caminho(pag, "videoDetails", "shortDescription").(string); ok {
			video.Descricao = d
		}
	}
}

// preencherVideoRelacionados completa os videos relacionados.
func preencherVideoRelacionados(video *ytVideo, pag map[string]any, id string) {
	var rel []map[string]any
	acharTodos(pag, "compactVideoRenderer", &rel, 0)

	if len(rel) == 0 {
		for _, it := range itensDeLockups(pag) {
			if it.ID != id && len(video.Relacionados) < maximoRelacionados {
				video.Relacionados = append(video.Relacionados, it)
			}
		}

		return
	}

	for _, r := range rel {
		if it := itemDeVideo(r); it.ID != "" && it.ID != id {
			video.Relacionados = append(video.Relacionados, it)
			if len(video.Relacionados) >= maximoRelacionados {
				break
			}
		}
	}
}

// aguardarExtrasVideo busca patrocinio e deslikes em paralelo.
func aguardarExtrasVideo(ctx context.Context, video *ytVideo, id string) {
	var grupoEspera sync.WaitGroup

	grupoEspera.Add(extrasParalelosVideo)
	go func(ctx context.Context) {
		defer grupoEspera.Done()

		video.Patrocinio, video.Duracao = ytPatrocinio(ctx, id)
	}(ctx)
	go func(ctx context.Context) {
		defer grupoEspera.Done()

		if d := ytDeslikes(ctx, id); d > 0 {
			video.Deslikes = formatarNumero(d)
		}
	}(ctx)

	grupoEspera.Wait()
}

func formatarNumero(numero int64) string {
	if numero >= limiteMilhaoViews {
		return strconv.FormatFloat(float64(numero)/limiteMilhaoViews, 'f', 1, 64) + " mi"
	}

	if numero >= limiteMilharViews {
		return strconv.FormatFloat(float64(numero)/limiteMilharViews, 'f', 1, 64) + " mil"
	}

	return strconv.FormatInt(numero, 10)
}

func ytPatrocinio(ctx context.Context, id string) ([]ytTrecho, float64) {
	cats := `["sponsor","selfpromo","interaction","intro","outro","preview","music_offtopic","filler"]`
	u := "https://sponsor.ajay.app/api/skipSegments?videoID=" + url.QueryEscape(id) +
		"&categories=" + url.QueryEscape(cats)
	// GOD: host fixo (sponsor.ajay.app) e parametros escapados; sem SSRF por desenho.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil) // #nosec G704
	if err != nil {
		return nil, 0
	}

	req.Header.Set("User-Agent", "Overdrive (UTube)")
	// GOD: executa o request montado acima (host fixo); sem SSRF por desenho.
	resp, err := ytHTTP.Do(req) // #nosec G704
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != codigoHTTPSucessoYouTube {
		return nil, 0
	}

	var bruto []struct {
		Categoria string     `json:"category"`
		Segmento  [2]float64 `json:"segment"`
		Duracao   float64    `json:"videoDuration"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaYouTube)).Decode(&bruto) != nil {
		return nil, 0
	}

	saida := make([]ytTrecho, 0, len(bruto))
	duracao := 0.0

	for _, b := range bruto {
		saida = append(saida, ytTrecho{Inicio: b.Segmento[0], Fim: b.Segmento[1], Categoria: b.Categoria})
		if b.Duracao > duracao {
			duracao = b.Duracao
		}
	}

	sort.Slice(saida, func(i, j int) bool { return saida[i].Inicio < saida[j].Inicio })

	return saida, duracao
}

func ytDeslikes(ctx context.Context, id string) int64 {
	// GOD: host fixo (returnyoutubedislikeapi.com) e id escapado; sem SSRF por desenho.
	urlVotos := "https://returnyoutubedislikeapi.com/votes?videoId=" + url.QueryEscape(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlVotos, nil) // #nosec G704
	if err != nil {
		return 0
	}

	resp, err := ytHTTP.Do(req) // #nosec G704
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != codigoHTTPSucessoYouTube {
		return 0
	}

	var resposta struct {
		Dislikes int64 `json:"dislikes"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaYouTube)).Decode(&resposta) != nil {
		return 0
	}

	return resposta.Dislikes
}

func limparComentario(texto string) string {
	if texto == "" {
		return ""
	}

	texto = html.UnescapeString(texto)
	if strings.Contains(texto, "<") {
		texto = tagComentario.ReplaceAllString(texto, "")
	}

	texto = html.UnescapeString(texto)

	return strings.TrimSpace(texto)
}

// comentarioPiped espelha um comentario da API Piped.
type comentarioPiped struct {
	Autor     string `json:"author"`
	Texto     string `json:"commentText"`
	Quando    string `json:"commentedTime"`
	Likes     int64  `json:"likeCount"`
	Respostas int    `json:"replyCount"`
	PagRespos string `json:"repliesPage"`
	Fixado    bool   `json:"pinned"`
	Coracao   bool   `json:"hearted"`
}

// respostaComentariosPiped espelha a pagina de comentarios do Piped.
type respostaComentariosPiped struct {
	Comentarios []comentarioPiped `json:"comments"`
	Proximo     *string           `json:"nextpage"`
}

func ytComentarios(ctx context.Context, id, token string) ([]ytComentario, string, error) {
	var ultimoErro error

	for _, base := range pipedInstancias {
		saida, prox, ok, err := consultarComentariosInstancia(ctx, id, token, base)
		if !ok {
			if err != nil {
				ultimoErro = err
			}

			continue
		}

		return saida, prox, nil
	}

	if ultimoErro == nil {
		ultimoErro = erroTexto("comentarios indisponiveis")
	}

	return nil, "", ultimoErro
}

// consultarComentariosInstancia busca comentarios numa instancia; false = tentar proxima.
func consultarComentariosInstancia(ctx context.Context, id, token, base string) ([]ytComentario, string, bool, error) {
	u := urlComentariosInstancia(id, token, base)
	// GOD: base de lista fixa interna (pipedInstancias) e id escapado; sem SSRF por desenho.
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil) // #nosec G704
	if err != nil {
		return nil, "", false, erroTextoF("montar pedido comentarios: %w", err)
	}

	pedido.Header.Set("User-Agent", "Overdrive (UTube)")
	// GOD: executa o request montado acima (lista fixa); sem SSRF por desenho.
	resp, err := ytHTTP.Do(pedido) // #nosec G704
	if err != nil {
		return nil, "", false, erroTextoF("buscar comentarios: %w", err)
	}

	if resp.StatusCode != codigoHTTPSucessoYouTube {
		resp.Body.Close()

		return nil, "", false, erroTextoF("piped %d", resp.StatusCode)
	}

	var resposta respostaComentariosPiped

	err = json.NewDecoder(io.LimitReader(resp.Body, limiteRespostaYouTube)).Decode(&resposta)
	resp.Body.Close()

	if err != nil {
		return nil, "", false, erroTextoF("decodificar comentarios: %w", err)
	}

	saida, prox := converterComentariosPiped(resposta)

	return saida, prox, true, nil
}

// urlComentariosInstancia monta a URL de comentarios da instancia.
func urlComentariosInstancia(id, token, base string) string {
	if token != "" {
		return base + "/nextpage/comments/" + url.PathEscape(id) + "?nextpage=" + url.QueryEscape(token)
	}

	return base + "/comments/" + url.PathEscape(id)
}

// converterComentariosPiped converte a resposta em comentarios limpos.
func converterComentariosPiped(resposta respostaComentariosPiped) ([]ytComentario, string) {
	saida := make([]ytComentario, 0, len(resposta.Comentarios))
	for _, comentario := range resposta.Comentarios {
		curtidas := ""
		if comentario.Likes > 0 {
			curtidas = formatarNumero(comentario.Likes)
		}

		saida = append(saida, ytComentario{
			Autor: limparComentario(comentario.Autor), Texto: limparComentario(comentario.Texto), Quando: comentario.Quando,
			Likes: curtidas, Respostas: comentario.Respostas,
			Token: comentario.PagRespos, Fixo: comentario.Fixado, Coracao: comentario.Coracao,
		})
	}

	prox := ""
	if resposta.Proximo != nil {
		prox = *resposta.Proximo
	}

	return saida, prox
}

func ytCanal(ctx context.Context, id string) ([]ytItem, error) {
	corpo := map[string]any{
		chaveContexto: ytContextoWEB(), "browseId": id,
		"params": "EgZ2aWRlb3PyBgQKAjoA",
	}

	dados, err := ytPost(ctx, "browse", "1", "2.20250101.00.00", ytWebUA, corpo)
	if err != nil {
		return nil, err
	}

	var itens []ytItem

	var locks []map[string]any
	acharTodos(dados, "lockupViewModel", &locks, 0)

	for _, cartao := range locks {
		item := ytItem{ID: textoDe(caminho(cartao, "contentId"))}
		if item.ID == "" {
			continue
		}

		item.Titulo = textoDe(caminho(cartao, "metadata", "lockupMetadataViewModel", "title", "content"))

		var partes []map[string]any
		acharTodos(caminho(cartao, "metadata", "lockupMetadataViewModel", "metadata", "contentMetadataViewModel"),
			"metadataParts", &partes, 0)

		if len(partes) > 0 {
			item.Views = textoDe(caminho(partes[0], "metadataParts", 0, "text", "content"))
			if len(partes) > 1 {
				item.Publicado = textoDe(caminho(partes[len(partes)-1], "metadataParts", 0, "text", "content"))
			}
		}

		item.Duracao = textoDe(caminho(cartao, "contentImage", "thumbnailViewModel", "overlays", 0,
			"thumbnailBottomOverlayViewModel", "badges", 0, "thumbnailBadgeViewModel", "text"))
		if item.Duracao == "" {
			item.Duracao = textoDe(caminho(cartao, "contentImage", "thumbnailViewModel", "overlays", 1,
				"thumbnailOverlayBadgeViewModel", "thumbnailBadges", 0, "thumbnailBadgeViewModel", "text"))
		}

		itens = append(itens, item)
	}

	if len(itens) == 0 {
		var vids []map[string]any
		acharTodos(dados, "videoRenderer", &vids, 0)

		for _, v := range vids {
			if it := itemDeVideo(v); it.ID != "" {
				itens = append(itens, it)
			}
		}
	}

	return itens, nil
}

func itensDeLockups(dados map[string]any) []ytItem {
	var locks []map[string]any
	acharTodos(dados, "lockupViewModel", &locks, 0)

	saida := make([]ytItem, 0, len(locks))
	for _, cartao := range locks {
		item := ytItem{ID: textoDe(caminho(cartao, "contentId"))}
		if item.ID == "" || len(item.ID) != 11 {
			continue
		}

		item.Titulo = textoDe(caminho(cartao, "metadata", "lockupMetadataViewModel", "title", "content"))
		preencherMetadadosLockup(&item, cartao)

		saida = append(saida, item)
	}

	return saida
}

// preencherMetadadosLockup extrai views, publicado e canal do lockup.
func preencherMetadadosLockup(item *ytItem, lk map[string]any) {
	var linhas []map[string]any
	acharTodos(caminho(lk, "metadata", "lockupMetadataViewModel", "metadata", "contentMetadataViewModel"),
		"metadataRows", &linhas, 0)

	for _, ln := range linhas {
		textos := textosDeLinhaLockup(ln)
		if len(textos) == 0 {
			continue
		}

		classificarTextoLockup(item, strings.Join(textos, " "))
	}
}

// textosDeLinhaLockup junta os textos de uma linha de metadados.
func textosDeLinhaLockup(ln map[string]any) []string {
	var partes []any
	if l, ok := ln["metadataParts"].([]any); ok {
		partes = l
	}

	var textos []string

	for _, pe := range partes {
		if s := textoDe(caminho(pe, "text", "content")); s != "" {
			textos = append(textos, s)
		}
	}

	return textos
}

// classificarTextoLockup encaixa o texto em views, publicado ou canal.
func classificarTextoLockup(item *ytItem, junto string) {
	switch {
	case strings.Contains(junto, "visualiza") || strings.Contains(junto, "view"):
		if item.Views == "" {
			item.Views = junto
		}
	case ehPublicadoLockup(item, junto):
		if item.Publicado == "" && item.Views != "" {
			item.Publicado = junto
		}
	case item.Canal == "":
		item.Canal = junto
	}
}

// ehPublicadoLockup diz se o texto parece data de publicacao.
func ehPublicadoLockup(it *ytItem, junto string) bool {
	if strings.Contains(junto, "atr") || strings.Contains(junto, "ago") {
		return true
	}

	return strings.Contains(junto, "h") && len(it.Publicado) == 0
}

func estadoDoCliqueYouTube(aberto, oculto bool) estadoYouTube {
	if !aberto {
		return ytAbrirCheio
	}

	if !oculto {
		return ytOcultar
	}

	return ytMostrar
}

func (a *app) abrirYouTube() {
	if a.ytAberto {
		a.mostrarYouTube()

		return
	}

	if a.env == nil {
		a.dizerErrof(txtFmtIndispMotor, "YouTube")

		return
	}

	iniciarServidorImagens()

	base := enderecoDoProxy()
	if base == "" {
		a.dizerErrof(txtTubeSemServidor)
		a.invalidateBar()

		return
	}

	if a.ytPane == nil {
		a.ytPane = &pane{zoom: 1}
	}

	a.ytAberto = true
	a.ytOculto = false
	a.suspenderParaYouTube()
	a.dizerProgressoIndetf(txtTubeAbrindo)
	a.layout()
	a.applyBounds()
	a.invalidateBar()

	if a.ytPane.ready && a.ytPane.wv != nil {
		a.ytPane.wv.Resume()

		return
	}

	err := a.env.CreateController(a.hwnd, a.aoCriarYouTube(base))
	if err != nil {
		a.ytAberto = false
		a.ytOculto = false
		a.dizerErrof(txtTubeNaoAbriu)
		a.layout()
		a.applyBounds()
		a.invalidateBar()
	}
}

// aoCriarYouTube devolve o callback de criacao do controller do painel YouTube.
func (a *app) aoCriarYouTube(base string) func(w *WebView, err error) {
	return func(w *WebView, err error) {
		a.depois(func() {
			if err != nil {
				a.ytAberto = false
				a.ytOculto = false
				a.dizerErrof(txtTubeNaoAbriu)
				a.layout()
				a.applyBounds()
				a.invalidateBar()

				return
			}

			p := a.ytPane
			if p == nil || !a.ytAberto {
				w.Close()

				return
			}

			p.wv = w
			p.boundsSet = false
			p.visivelSet = false
			p.ready = true

			p.tocandoAudio = false
			p.mudo = false
			p.ultimaURL = ""
			p.tentativas = 0
			a.configureWebView(p, false)
			a.layout()
			a.applyBounds()

			p.queriaURL = base + "/yt"
			w.Navigate(p.queriaURL)
		})
	}
}

func (a *app) mostrarYouTube() {
	if !a.ytAberto || a.ytPane == nil || !a.ytPane.ready {
		a.dizerf(txtTubeAguarde)

		return
	}

	a.ytOculto = false
	a.suspenderParaYouTube()
	a.layout()
	a.applyBounds()

	if a.ytPane.wv != nil {
		setPaneVisible(a.ytPane, true)
		a.ytPane.wv.Resume()
	}

	a.status = txtTubeNaTela
	a.invalidateBar()
}

func (a *app) ocultarYouTube() {
	if !a.ytAberto || a.ytPane == nil || !a.ytPane.ready {
		a.dizerf(txtTubeAguarde)

		return
	}

	a.ytOculto = true
	a.layout()
	a.applyBounds()
	a.retomarDoYouTube()
	a.status = txtTubeFundo
	a.invalidateBar()

	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.Focus()
	}
}

func (a *app) fecharYouTube() {
	if !a.ytAberto && a.ytPane == nil {
		return
	}

	a.ytAberto = false

	a.ytOculto = false
	if a.ytPane != nil {
		if a.ytPane.wv != nil {
			a.ytPane.wv.SetMuted(true)
			a.ytPane.wv.Stop()
			a.ytPane.wv.Navigate("about:blank")
		}

		a.destruirPane(a.ytPane)
	}

	a.layout()
	a.applyBounds()
	a.dizerOkf(txtTubeFechado)
	a.retomarDoYouTube()
	a.invalidateBar()
	a.agendarLimpezaMemoria(true, esperaLimpezaYouTube)
}

func (a *app) alternarYouTube() {
	estado := estadoDoCliqueYouTube(a.ytAberto, a.ytOculto)
	switch estado {
	case ytOcultar:
		if a.ytPane == nil || !a.ytPane.tocandoAudio {
			a.fecharYouTube()

			return
		}

		a.ocultarYouTube()
	case ytMostrar:
		a.mostrarYouTube()
	case ytAbrirCheio:
		a.abrirYouTube()
	default:
		a.abrirYouTube()
	}
}

func (a *app) suspenderParaYouTube() {
	for _, t := range a.tabs {
		if t != nil {
			suspenderPane(t.pane)
		}
	}

	for _, p := range []*pane{a.right, a.imgPanel} {
		suspenderPane(p)
	}
}

func (a *app) retomarDoYouTube() {
	a.retomarEsquerdoYouTube()
	a.retomarDireitoYouTube()
	a.retomarImagensYouTube()
	a.retomarTodosFrios()
}

// retomarEsquerdoYouTube reativa a aba principal apos o YouTube.
func (a *app) retomarEsquerdoYouTube() {
	if a.left != nil && a.left.ready && a.left.wv != nil {
		a.left.wv.Resume()
	}
}

// retomarDireitoYouTube reativa o painel direito apos o YouTube.
func (a *app) retomarDireitoYouTube() {
	if a.st.RightVisible && a.right != nil && a.right.ready && a.right.wv != nil {
		a.right.wv.Resume()
	}
}

// retomarImagensYouTube reativa o painel de imagens apos o YouTube.
func (a *app) retomarImagensYouTube() {
	if a.st.RightVisible && a.st.ImgVisible && a.imgPanel != nil && a.imgPanel.ready && a.imgPanel.wv != nil {
		a.imgPanel.wv.Resume()
	}
}
