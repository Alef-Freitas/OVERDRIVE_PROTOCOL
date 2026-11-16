//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	tradCacheMax  = 6000
	tradIntervalo = 320 * time.Millisecond
	// MyMemory recebe os trechos por GET: o corte precisa caber na URL
	// depois do percent-encoding (texto nao-ASCII triplica de tamanho).
	tradMaxCodMyMemory = 1500
)

//nolint:gochecknoglobals // Caches e geracao da traducao + mutexes: estado de processo.
var (
	tradCacheMu sync.Mutex
	tradCache   = map[string]string{}

	trad429Mu    sync.Mutex
	trad429Ate   time.Time
	trad429Vezes int

	tradRitmoMu sync.Mutex
	tradUltimo  time.Time

	tradCurlMu     sync.Mutex
	tradCurlCtx    context.Context
	tradCurlCancel context.CancelFunc
)

func tradCurlNovaGeracao() {
	tradCurlMu.Lock()
	defer tradCurlMu.Unlock()

	if tradCurlCancel != nil {
		tradCurlCancel()
	}

	// Gera uma nova geracao: cancela traducoes em voo e cria ctx novo.
	//nolint:fatcontext // Ciclo de vida deliberado: geracao global cancelavel pelo usuario.
	tradCurlCtx, tradCurlCancel = context.WithCancel(context.Background())
}

func tradCurlCancela() {
	tradCurlMu.Lock()
	defer tradCurlMu.Unlock()

	if tradCurlCancel != nil {
		tradCurlCancel()
		tradCurlCancel = nil
	}

	tradCurlCtx = nil //nolint:fatcontext // Ciclo de vida deliberado: geracao global cancelavel pelo usuario.
}

func tradCurlCtxAtivo() context.Context {
	tradCurlMu.Lock()
	defer tradCurlMu.Unlock()

	if tradCurlCtx == nil {
		return context.Background()
	}

	return tradCurlCtx
}

func tradCacheGet(idioma, txt string) (string, bool) {
	tradCacheMu.Lock()
	defer tradCacheMu.Unlock()

	v, ok := tradCache[idioma+"\x00"+txt]

	return v, ok
}

func tradCachePut(idioma, txt, traduzido string) {
	if traduzido == "" {
		return
	}

	tradCacheMu.Lock()
	defer tradCacheMu.Unlock()

	if len(tradCache) >= tradCacheMax {
		for k := range tradCache {
			delete(tradCache, k)

			if len(tradCache) < tradCacheMax-500 {
				break
			}
		}
	}

	tradCache[idioma+"\x00"+txt] = traduzido
}

func tradEsperaRestante() time.Duration {
	trad429Mu.Lock()
	defer trad429Mu.Unlock()

	return time.Until(trad429Ate)
}

func tradMarcar429() {
	trad429Mu.Lock()
	trad429Vezes++

	const tetoEspera429Segundos = 60

	espera := min(time.Duration(trad429Vezes*trad429Vezes)*4*time.Second, tetoEspera429Segundos*time.Second)

	trad429Ate = time.Now().Add(espera)
	trad429Mu.Unlock()
}

//nolint:gochecknoglobals // Geracao de traducao + mutex: estado de processo.
var (
	tradGenMu    sync.Mutex
	tradGen      uint64
	traducaoAlvo *pane
)

func tradGeracaoAtual(p *pane) uint64 {
	tradGenMu.Lock()
	defer tradGenMu.Unlock()

	traducaoAlvo = p

	return tradGen
}

func tradCancelarSe(p *pane) {
	tradGenMu.Lock()

	mesmo := traducaoAlvo == p
	if mesmo {
		tradGen++
		traducaoAlvo = nil
	}
	tradGenMu.Unlock()

	if mesmo {
		tradCurlCancela()

		traduzindo = false
	}
}

func tradValida(gen uint64) bool {
	tradGenMu.Lock()
	defer tradGenMu.Unlock()

	return gen == tradGen
}

func traduzirMyMemory(txt, idioma string) (string, error) {
	partes := tradCortarCodificado(txt)

	var montador strings.Builder

	for _, parte := range partes {
		u := "https://api.mymemory.translated.net/get?q=" + urlQueryEscape(parte) + "&langpair=Autodetect|" + idioma

		corpo, status, err := tradCurlGet(u)
		if err != nil {
			return "", err
		}

		const httpStatusOKTraducao = 200
		if status != httpStatusOKTraducao {
			return "", erroTextoF("mymemory HTTP %d", status)
		}

		var resposta struct {
			ResponseData struct {
				TranslatedText string `json:"translatedText"`
			} `json:"responseData"`
			ResponseStatus any `json:"responseStatus"`
		}
		if e := json.Unmarshal(corpo, &resposta); e != nil {
			return "", erroTexto("mymemory resposta invalida")
		}

		t := strings.TrimSpace(resposta.ResponseData.TranslatedText)
		if t == "" || strings.Contains(strings.ToUpper(t), "INVALID SOURCE LANGUAGE") {
			return "", erroTexto("mymemory sem traducao")
		}

		montador.WriteString(t)
		montador.WriteString(" ")
	}

	return strings.TrimSpace(montador.String()), nil
}

// maiorPrefixoCodificado busca o maior prefixo (em runas) que cabe no teto.
func maiorPrefixoCodificado(runas []rune, maxCod int) int {
	const divisorPontoMedio = 2

	piso, teto := 1, len(runas)
	for piso < teto {
		meio := (piso + teto + 1) / divisorPontoMedio
		if len(urlQueryEscape(string(runas[:meio]))) <= maxCod {
			piso = meio
		} else {
			teto = meio - 1
		}
	}

	return piso
}

// recuarParaSeparador recua o corte para fim de frase ou espaco.
func recuarParaSeparador(prefixo string) int {
	recuo := -1

	for _, sep := range []string{". ", "! ", "? ", "\n", "; ", ": ", ", ", " "} {
		if i := strings.LastIndex(prefixo, sep); i > len(prefixo)/2 {
			if i+len(sep) > recuo {
				recuo = i + len(sep)
			}
		}
	}

	return recuo
}

// fatiarProximoCorte tira a proxima parte de resto, forcando o corte sem progresso.
func fatiarProximoCorte(resto string, maxCod int) (string, string, bool) {
	runas := []rune(resto)
	corte := maiorPrefixoCodificado(runas, maxCod)
	prefixo := string(runas[:corte])

	if recuo := recuarParaSeparador(prefixo); recuo > 0 {
		corte = len([]rune(prefixo[:recuo]))
	}

	parte := strings.TrimSpace(string(runas[:corte]))
	if parte == "" {
		parte = strings.TrimSpace(prefixo)
		if parte == "" {
			return "", resto, true
		}
	}

	sobra := strings.TrimSpace(string(runas[corte:]))

	return parte, sobra, false
}

// tradCortarCodificado corta pelo tamanho apos o percent-encoding (o que
// vale para a URL), preferindo fim de frase e depois espacos. Evita que um
// trecho "de 420 bytes" vire ~1,5 KB codificado e seja rejeitado.
func tradCortarCodificado(txt string) []string {
	maxCod := tradMaxCodMyMemory

	txt = strings.TrimSpace(txt)

	if txt == "" {
		return nil
	}

	if len(urlQueryEscape(txt)) <= maxCod {
		return []string{txt}
	}

	var partes []string

	resto := txt
	for len(resto) > 0 {
		if len(urlQueryEscape(resto)) <= maxCod {
			partes = append(partes, resto)

			break
		}

		parte, sobra, travou := fatiarProximoCorte(resto, maxCod)
		if travou {
			break
		}

		partes = append(partes, parte)

		resto = sobra
		if resto == "" {
			break
		}
	}

	return partes
}

// byteQueryLivre diz se o byte pode ir cru na URL (sem %XX).
func byteQueryLivre(codigo byte) bool {
	if codigo == '-' || codigo == '_' || codigo == '.' || codigo == '~' {
		return true
	}

	if codigo >= 'a' && codigo <= 'z' {
		return true
	}

	if codigo >= 'A' && codigo <= 'Z' {
		return true
	}

	return codigo >= '0' && codigo <= '9'
}

func urlQueryEscape(s string) string {
	var montador strings.Builder

	for _, letra := range []byte(s) {
		switch {
		case letra == ' ':
			montador.WriteByte('+')
		case byteQueryLivre(letra):
			montador.WriteByte(letra)
		default:
			fmt.Fprintf(&montador, "%%%02X", letra)
		}
	}

	return montador.String()
}

func tradCurlGet(endpoint string) ([]byte, int, error) {
	curl := tradCurlExe()
	if curl == "" {
		return nil, 0, erroTexto("curl.exe nao encontrado")
	}

	tradRitmoSegura()

	args := []string{
		"-s", "-S", "-i", "-m", "20", endpoint,
		"-H", "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" +
			versaoChrome + ".0.0.0 Safari/537.36",
	}
	// GOD: curl.exe do sistema, sem shell, endpoint fixo e args via flag propria; sem injecao por desenho.
	cmd := exec.CommandContext(tradCurlCtxAtivo(), curl, args...) // #nosec G204
	//exhaustruct:ignore // SysProcAttr: so HideWindow; resto zero intencional.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	saida, err := cmd.Output()
	if err != nil {
		return nil, 0, erroTextoF("tradutor: %w", err)
	}

	txt := string(saida)
	status := 0

	corpo := txt
	if before, after, ok := strings.Cut(txt, "\r\n\r\n"); ok {
		bloco := before
		corpo = after

		if j := strings.IndexByte(bloco, '\n'); j > 0 {
			campos := strings.Fields(bloco[:j])

			const minCamposStatusHTTP = 2

			if len(campos) >= minCamposStatusHTTP {
				fmt.Sscanf(campos[1], "%d", &status)
			}
		}
	}

	return []byte(corpo), status, nil
}

func tradLimpar429() {
	trad429Mu.Lock()
	trad429Vezes = 0
	trad429Ate = time.Time{}
	trad429Mu.Unlock()
}

//nolint:gochecknoglobals // Bloqueio de ritmo da traducao + mutex: estado de processo.
var (
	tradBloqMu         sync.Mutex
	tradBloqAte        time.Time
	tradIntervaloAtual = tradIntervalo
)

func tradBloqueioRestante() time.Duration {
	tradBloqMu.Lock()
	defer tradBloqMu.Unlock()

	return time.Until(tradBloqAte)
}

func tradMarcarBloqueio() {
	const (
		duracaoBloqueioTraducaoMinutos = 10
		intervaloBloqueioTraducaoMs    = 500
	)

	tradBloqMu.Lock()
	tradBloqAte = time.Now().Add(duracaoBloqueioTraducaoMinutos * time.Minute)
	tradIntervaloAtual = intervaloBloqueioTraducaoMs * time.Millisecond
	tradBloqMu.Unlock()
}

func tradIntervaloMin() time.Duration {
	tradBloqMu.Lock()
	defer tradBloqMu.Unlock()

	if tradIntervaloAtual < tradIntervalo {
		tradIntervaloAtual = tradIntervalo
	}

	return tradIntervaloAtual
}

func tradRitmoSegura() {
	tradRitmoMu.Lock()
	defer tradRitmoMu.Unlock()

	intervalo := tradIntervaloMin()
	if !tradUltimo.IsZero() {
		if falta := intervalo - time.Since(tradUltimo); falta > 0 {
			time.Sleep(falta)
		}
	}

	tradUltimo = time.Now()
}

func tradCurlExe() string {
	if sysroot := strings.TrimSpace(os.Getenv("SystemRoot")); sysroot != "" {
		for _, candidato := range []string{
			filepath.Join(sysroot, "System32", "curl.exe"),
			filepath.Join(sysroot, "SysWOW64", "curl.exe"),
		} {
			// GOD: caminho derivado de SystemRoot (config da maquina), nao de entrada externa.
			if _, err := os.Stat(candidato); err == nil { // #nosec G703
				return candidato
			}
		}
	}

	if caminhoCurl, err := exec.LookPath("curl.exe"); err == nil {
		return caminhoCurl
	}

	return ""
}

func tradCurl(endpoint string, pares [][2]string) ([]byte, int, error) {
	curl := tradCurlExe()
	if curl == "" {
		return nil, 0, erroTexto("curl.exe nao encontrado no Windows")
	}

	tradRitmoSegura()

	args := []string{
		"-s", "-S", "-i", "-m", "25", "-X", "POST", endpoint,
		"-H", "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" +
			versaoChrome + ".0.0.0 Safari/537.36",
	}
	for _, par := range pares {
		args = append(args, "--data-urlencode", par[0]+"="+par[1])
	}
	// GOD: curl.exe do sistema, sem shell, endpoint fixo e args via flag propria; sem injecao por desenho.
	cmd := exec.CommandContext(tradCurlCtxAtivo(), curl, args...) // #nosec G204
	//exhaustruct:ignore // SysProcAttr: so HideWindow; resto zero intencional.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	saida, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			msg := strings.TrimSpace(string(ee.Stderr))

			const limiteMsgErroTraducao = 140

			if len(msg) > limiteMsgErroTraducao {
				msg = msg[:limiteMsgErroTraducao]
			}

			return nil, 0, erroTextoF("%s", msg)
		}

		return nil, 0, erroTextoF("tradutor: %w", err)
	}

	txt := string(saida)
	status := 0
	corpo := txt

	if before, after, ok := strings.Cut(txt, "\r\n\r\n"); ok {
		bloco := before
		corpo = after

		if j := strings.IndexByte(bloco, '\n'); j > 0 {
			campos := strings.Fields(bloco[:j])

			const minCamposStatusHTTP = 2

			if len(campos) >= minCamposStatusHTTP {
				fmt.Sscanf(campos[1], "%d", &status)
			}
		}
	}

	return []byte(corpo), status, nil
}

func traduzirUm(txt, idioma string) (string, string, error) {
	pares := [][2]string{
		{chaveClient, "gtx"},
		{"sl", "auto"},
		{"tl", idioma},
		{"dt", "t"},
		{"dj", "1"},
		{"q", txt},
	}

	corpo, status, err := tradCurl("https://translate.googleapis.com/translate_a/single", pares)
	if err != nil {
		return "", "", err
	}

	const httpStatusLimiteTraducao = 429
	if status == httpStatusLimiteTraducao {
		tradMarcar429()

		return "", "", erroTexto("limite de uso do servico · 429")
	}

	const httpStatusOKGoogle = 200
	if status != httpStatusOKGoogle {
		return "", "", erroTextoF("HTTP %d", status)
	}

	var resposta struct {
		Sentences []struct {
			Trans string `json:"trans"`
		} `json:"sentences"`
		Src string `json:"src"`
	}
	if e := json.Unmarshal(corpo, &resposta); e != nil {
		return "", "", erroTexto("resposta invalida")
	}

	var montador strings.Builder
	for _, frase := range resposta.Sentences {
		montador.WriteString(frase.Trans)
	}

	saida := strings.TrimSpace(montador.String())
	if saida == "" {
		return "", resposta.Src, erroTexto("traducao vazia")
	}

	return saida, resposta.Src, nil
}

func montarFilaTraducao(textos []string, idioma string, saida map[string]string) []string {
	var fila []string

	vistos := map[string]bool{}
	for _, texto := range textos {
		if vistos[texto] {
			continue
		}

		vistos[texto] = true
		if v, ok := tradCacheGet(idioma, texto); ok {
			saida[texto] = v

			continue
		}

		fila = append(fila, texto)
	}

	return fila
}

// tentarReservaMyMemory tenta o MyMemory como reserva durante bloqueio.
func tentarReservaMyMemory(texto, idioma string) (string, bool) {
	v2, e2 := traduzirMyMemory(texto, idioma)
	if e2 == nil && v2 != "" {
		return v2, true
	}

	return "", false
}

// esperarBloqueioTraducao dorme no maximo o teto e pede nova tentativa.
func esperarBloqueioTraducao(espera time.Duration) {
	const esperaMaximaBloqueioSegundos = 5
	if espera > esperaMaximaBloqueioSegundos*time.Second {
		espera = esperaMaximaBloqueioSegundos * time.Second
	}

	time.Sleep(espera)
}

// traduzirTextoComReserva traduz um texto com tentativas e reserva MyMemory.
func traduzirTextoComReserva(texto, idioma string) (string, string, error) {
	var valor, src string

	var err error

	const tentativasTraducao = 3
	for range tentativasTraducao {
		if espera := tradBloqueioRestante(); espera > 0 {
			if v2, ok := tentarReservaMyMemory(texto, idioma); ok {
				return v2, "", nil
			}

			esperarBloqueioTraducao(espera)

			continue
		}

		valor, src, err = traduzirUm(texto, idioma)
		if err == nil {
			return valor, src, nil
		}

		if ehErroBloqueioTraducao(err) {
			tradMarcarBloqueio()

			if v2, ok := tentarReservaMyMemory(texto, idioma); ok {
				return v2, "", nil
			}

			continue
		}

		if ehErroPassageiroTraducao(err) {
			const pausaRetryTraducaoMs = 300
			time.Sleep(pausaRetryTraducaoMs * time.Millisecond)

			continue
		}

		return valor, src, err
	}

	return valor, src, err
}

// ehErroBloqueioTraducao diz se o erro pede reserva e nova tentativa.
func ehErroBloqueioTraducao(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "bloqueio") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "302")
}

// ehErroPassageiroTraducao diz se o erro merece uma pausa curta e retry.
func ehErroPassageiroTraducao(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "curl") || strings.Contains(msg, "vazia")
}

// ventilarParcialTraducao despeja o buffer parcial no intervalo e pede nova tentativa.
func ventilarParcialTraducao(parar chan struct{}, parcial func(map[string]string), recolher func() map[string]string) {
	const intervaloVentilacaoTraducaoMs = 400

	tique := time.NewTicker(intervaloVentilacaoTraducaoMs * time.Millisecond)
	defer tique.Stop()

	for {
		select {
		case <-parar:
			return
		case <-tique.C:
			if envio := recolher(); len(envio) > 0 && parcial != nil {
				parcial(envio)
			}
		}
	}
}

// guardarTraducaoOk registra acerto no cache e no buffer parcial.
func guardarTraducaoOk(
	saida map[string]string, idioma, texto, valor, src string, origem *string, parcialBuf map[string]string,
	parcialMu *sync.Mutex,
) {
	saida[texto] = valor

	if *origem == "" {
		*origem = src
	}

	tradCachePut(idioma, texto, valor)
	tradLimpar429()
	parcialMu.Lock()
	parcialBuf[texto] = valor
	parcialMu.Unlock()
}

// rodarFilaTraducao consome a fila com trabalhadores e devolve origem/erro.
func rodarFilaTraducao(
	fila []string, idioma string, saida map[string]string, parcialBuf map[string]string, parcialMu *sync.Mutex,
	cancelado func() bool,
) (string, error) {
	var mutex sync.Mutex

	var ultimo error

	origem := ""
	trabalhos := make(chan string)

	var grupoEspera sync.WaitGroup

	const trabalhadoresTraducao = 4
	for range trabalhadoresTraducao {
		grupoEspera.Go(func() {
			for texto := range trabalhos {
				if cancelado != nil && cancelado() {
					continue
				}

				valor, src, err := traduzirTextoComReserva(texto, idioma)

				mutex.Lock()
				if err == nil && valor != "" {
					guardarTraducaoOk(saida, idioma, texto, valor, src, &origem, parcialBuf, parcialMu)
				} else {
					ultimo = err
				}

				mutex.Unlock()
			}
		})
	}

	for _, texto := range fila {
		if cancelado != nil && cancelado() {
			break
		}

		trabalhos <- texto
	}

	close(trabalhos)
	grupoEspera.Wait()

	return origem, ultimo
}

func traduzirTextos(textos []string, idioma string, parcial func(map[string]string),
	cancelado func() bool,
) (map[string]string, string, error) {
	tradCurlNovaGeracao()

	saida := map[string]string{}
	fila := montarFilaTraducao(textos, idioma, saida)

	if len(fila) == 0 {
		return saida, "", nil
	}

	if r := tradEsperaRestante(); r > 0 {
		return saida, "", erroTextoF("limite do servico; tente de novo em %d s", int(r.Seconds())+1)
	}

	parcialBuf := map[string]string{}

	var parcialMu sync.Mutex

	recolher := func() map[string]string {
		parcialMu.Lock()
		defer parcialMu.Unlock()

		if len(parcialBuf) == 0 {
			return nil
		}

		envio := parcialBuf
		parcialBuf = map[string]string{}

		return envio
	}

	parar := make(chan struct{})

	var flushWG sync.WaitGroup

	flushWG.Go(func() {
		ventilarParcialTraducao(parar, parcial, recolher)
	})

	origem, ultimo := rodarFilaTraducao(fila, idioma, saida, parcialBuf, &parcialMu, cancelado)

	close(parar)
	flushWG.Wait()

	if resto := recolher(); parcial != nil && len(resto) > 0 {
		parcial(resto)
	}

	if len(saida) == 0 && ultimo != nil {
		return saida, origem, ultimo
	}

	return saida, origem, nil
}
