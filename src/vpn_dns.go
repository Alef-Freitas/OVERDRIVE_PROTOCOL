package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

//nolint:tagliatelle // Resposta JSON da API DoH (RFC 8484): Status/Answer/TTL sao o contrato.
type dohResposta struct {
	Status int `json:"Status"`
	Answer []struct {
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

type dohCacheItem struct {
	ips []net.IP
	exp time.Time
}

//nolint:gochecknoglobals // Cache DoH + cliente + lista fixa + mutex: estado de processo.
var (
	dohMu    sync.Mutex
	dohCache = map[string]dohCacheItem{}
	//exhaustruct:ignore // http.Client DoH: so timeout; resto padrao do Go.
	dohCli   = &http.Client{Timeout: tempoLimiteDoH}
	dohLista = []string{
		"https://1.1.1.1/dns-query",
		"https://cloudflare-dns.com/dns-query",
		"https://dns.google/resolve",
		"https://9.9.9.9/dns-query",
	}
)

const tempoLimiteDoH = 6 * time.Second

func hostPublico(host string) bool {
	dominio := strings.ToLower(strings.TrimSuffix(host, "."))
	if dominio == "" || net.ParseIP(dominio) != nil {
		return false
	}

	if !strings.Contains(dominio, ".") {
		return false
	}

	for _, suf := range []string{sufLocal, sufLan, sufHome, sufInternal, ".localhost"} {
		if strings.HasSuffix(dominio, suf) {
			return false
		}
	}

	return true
}

func consultarDoH(ctx context.Context, servidor, host, tipo string) ([]net.IP, int, error) {
	u := servidor + "?name=" + url.QueryEscape(host) + "&type=" + tipo
	// GOD: servidor de lista fixa (dohLista), host escapado e filtrado (hostPublico); sem SSRF por desenho.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil) // #nosec G704
	if err != nil {
		return nil, 0, erroTextoF("montar DoH: %w", err)
	}

	req.Header.Set("Accept", "application/dns-json")
	// GOD: executa o request montado acima (DoH de lista fixa); sem SSRF por desenho.
	resp, err := dohCli.Do(req) // #nosec G704
	if err != nil {
		return nil, 0, erroTextoF("DoH: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, erroTexto("doh http " + resp.Status)
	}

	var resposta dohResposta
	if err := json.NewDecoder(resp.Body).Decode(&resposta); err != nil {
		return nil, 0, erroTextoF("decodificar DoH: %w", err)
	}

	if resposta.Status != 0 {
		return nil, 0, erroTexto("doh status")
	}

	ips, ttl := extrairIPsDoH(resposta.Answer)

	return ips, normalizarTTLDoH(ttl), nil
}

// extrairIPsDoH filtra respostas A/AAAA validas e o menor TTL.
//
//nolint:tagliatelle // Contrato DNS/DoH externo: o campo se chama "TTL" (RFC 1035).
func extrairIPsDoH(respostas []struct {
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
},
) ([]net.IP, int) {
	var ips []net.IP

	const ttlPadraoDoHSegundos = 300

	ttl := ttlPadraoDoHSegundos

	for _, item := range respostas {
		if item.Type != 1 && item.Type != 28 {
			continue
		}

		ip := net.ParseIP(item.Data)
		if ip == nil {
			continue
		}

		ips = append(ips, ip)

		if item.TTL > 0 && item.TTL < ttl {
			ttl = item.TTL
		}
	}

	return ips, ttl
}

// normalizarTTLDoH limita o TTL a faixa de 30s a 1h.
func normalizarTTLDoH(ttl int) int {
	const ttlMinimoDoHSegundos = 30
	if ttl < ttlMinimoDoHSegundos {
		return ttlMinimoDoHSegundos
	}

	const ttlMaximoDoHSegundos = 3600
	if ttl > ttlMaximoDoHSegundos {
		return ttlMaximoDoHSegundos
	}

	return ttl
}

func resolverDoH(ctx context.Context, host string) ([]net.IP, error) {
	dominio := strings.ToLower(strings.TrimSuffix(host, "."))
	if ip := net.ParseIP(dominio); ip != nil {
		return []net.IP{ip}, nil
	}

	if !hostPublico(dominio) {
		return nil, erroTexto("host nao publico")
	}

	if ips, ok := consultarCacheDoH(dominio); ok {
		return ips, nil
	}

	for _, servidor := range dohLista {
		todos, ttlFinal, ok := tentarServidorDoH(ctx, servidor, dominio)
		if !ok {
			continue
		}

		guardarCacheDoH(dominio, todos, ttlFinal)

		return todos, nil
	}

	return nil, erroTexto("doh sem resposta")
}

// consultarCacheDoH devolve os IPs em cache quando ainda validos.
func consultarCacheDoH(h string) ([]net.IP, bool) {
	dohMu.Lock()
	defer dohMu.Unlock()

	it, ok := dohCache[h]
	if !ok || !time.Now().Before(it.exp) {
		return nil, false
	}

	return append([]net.IP(nil), it.ips...), true
}

// tentarServidorDoH consulta os tipos A e AAAA num servidor DoH.
func tentarServidorDoH(ctx context.Context, servidor, dominio string) ([]net.IP, int, bool) {
	var todos []net.IP

	const ttlPadraoServidorDoHSegundos = 300

	ttlFinal := ttlPadraoServidorDoHSegundos

	for _, tipo := range []string{"A", "AAAA"} {
		ips, ttl, err := consultarDoH(ctx, servidor, dominio, tipo)
		if err != nil {
			continue
		}

		todos = append(todos, ips...)

		if ttl > 0 && ttl < ttlFinal {
			ttlFinal = ttl
		}

		if len(todos) > 0 {
			break
		}
	}

	if len(todos) == 0 {
		return nil, 0, false
	}

	return todos, ttlFinal, true
}

// guardarCacheDoH salva o resultado podando entradas antigas quando cheio.
func guardarCacheDoH(dominio string, todos []net.IP, ttlFinal int) {
	const limiteCacheDoH = 64

	dohMu.Lock()
	defer dohMu.Unlock()

	if len(dohCache) >= limiteCacheDoH {
		podarCacheDoH()
	}

	dohCache[dominio] = dohCacheItem{ips: todos, exp: time.Now().Add(time.Duration(ttlFinal) * time.Second)}
}

// podarCacheDoH remove expirados e, se preciso, o mais antigo.
func podarCacheDoH() {
	const limiteCacheDoH = 64

	agora := time.Now()
	for k, it := range dohCache {
		if !agora.Before(it.exp) {
			delete(dohCache, k)
		}
	}

	for len(dohCache) >= limiteCacheDoH {
		if !removerMaisAntigoDoH() {
			break
		}
	}
}

// removerMaisAntigoDoH descarta a entrada com expiracao mais proxima.
func removerMaisAntigoDoH() bool {
	velho, quando := "", time.Time{}
	for k, it := range dohCache {
		if velho == "" || it.exp.Before(quando) {
			velho, quando = k, it.exp
		}
	}

	if velho == "" {
		return false
	}

	delete(dohCache, velho)

	return true
}

func resolverDireto(ctx context.Context, host string) ([]net.IP, error) {
	if ips, err := resolverDoH(ctx, host); err == nil && len(ips) > 0 {
		return ips, nil
	}

	//exhaustruct:ignore // Resolver zerado: resolvedor padrao do Go com ctx.
	addrs, err := (&net.Resolver{}).LookupIPAddr(ctx, host)
	if err != nil {
		return nil, erroTextoF("resolver: %w", err)
	}

	if len(addrs) == 0 {
		return nil, erroTexto("dns sem endereco")
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}

	return ips, nil
}
