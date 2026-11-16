package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//nolint:gochecknoglobals // iphlpapi + cache de PIDs + mutex: estado de processo com peer-verificacao.
var (
	ipHlpAPI             = windows.NewLazySystemDLL("iphlpapi.dll")
	pGetExtendedTCPTable = ipHlpAPI.NewProc("GetExtendedTcpTable")
	proxyPidsMu          sync.Mutex
	proxyPidsCache       map[uint32]bool
	proxyPidsCacheQuando time.Time
)

//nolint:containedctx // Ctx de lifetime do proxy (criado no start, cancelado no Close).
type vpnProxy struct {
	ln   net.Listener
	addr string
	mgr  *vpnManager

	ctx      context.Context
	cancelar context.CancelFunc
}

func startVPNProxy(mgr *vpnManager) (*vpnProxy, error) {
	//exhaustruct:ignore // ListenConfig zerado: ouvinte local efemero, padroes do Go.
	lc := &net.ListenConfig{}
	// Ouvinte interno do ciclo de vida do app: sem ctx herdavel aqui.
	ouvinte, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0") // Ouvinte interno.
	if err != nil {
		return nil, erroTextoF("ouvir local: %w", err)
	}

	ctx, cancelar := context.WithCancel(context.Background())
	p := &vpnProxy{ln: ouvinte, addr: ouvinte.Addr().String(), mgr: mgr, ctx: ctx, cancelar: cancelar}

	go p.loop()

	return p, nil
}

func (p *vpnProxy) Close() {
	if p == nil {
		return
	}

	if p.cancelar != nil {
		p.cancelar()
	}

	if p.ln != nil {
		_ = p.ln.Close()
	}
}

func (p *vpnProxy) loop() {
	for {
		conexao, err := p.ln.Accept()
		if err != nil {
			return
		}

		if !p.parLocal(conexao) {
			_ = conexao.Close()

			continue
		}
		go p.handle(conexao)
	}
}

// parLocal so aceita conexoes vindas do proprio processo ou de
// descendentes (os renderers msedgewebview2.exe do Chromium). Sem isso
// qualquer processo local que descobrisse a porta usaria o tunel como
// saida. Se a tabela TCP nao puder ser lida, permite (fail-open: volta
// ao comportamento antigo em vez de quebrar a navegacao).
func (p *vpnProxy) parLocal(conexao net.Conn) bool {
	rem, ok := conexao.RemoteAddr().(*net.TCPAddr)
	if !ok || !rem.IP.IsLoopback() {
		return false
	}

	_, porto, err := net.SplitHostPort(p.addr)
	if err != nil {
		return true
	}

	var porta int

	_, _ = fmt.Sscanf(porto, "%d", &porta)

	dono, ok := donoConexaoTCP(porta, rem.Port)
	if !ok {
		return true
	}

	return pidProprioOuFilho(dono)
}

func pidProprioOuFilho(pid uint32) bool {
	self, _, _ := pGetCurrentProcessID.Call()
	if pid == uint32(self) {
		return true
	}

	proxyPidsMu.Lock()
	if proxyPidsCache != nil && time.Since(proxyPidsCacheQuando) < 10*time.Second {
		ok := proxyPidsCache[pid]
		proxyPidsMu.Unlock()

		return ok
	}
	proxyPidsMu.Unlock()

	conj := map[uint32]bool{}
	for _, pr := range processosDescendentes() {
		conj[pr.pid] = true
	}

	proxyPidsMu.Lock()
	proxyPidsCache = conj
	proxyPidsCacheQuando = time.Now()
	ok := conj[pid]
	proxyPidsMu.Unlock()

	return ok
}

type linhaTCPDono struct {
	Estado      uint32
	AddrLocal   uint32
	PortaLocal  uint32
	AddrRemoto  uint32
	PortaRemota uint32
	PidDono     uint32
}

func portaOrdemRede(porta uint32) int {
	const (
		deslocamentoByteRede = 8
		mascaraByteRede      = 0xFF
	)

	return int(((porta & mascaraByteRede) << deslocamentoByteRede) | ((porta >> deslocamentoByteRede) & mascaraByteRede))
}

// donoConexaoTCP acha o PID dono da conexao local portaLocal->portaRemota
// via GetExtendedTcpTable (MIB_TCPTABLE_OWNER_PID, IPv4).
func donoConexaoTCP(portaLocal, portaRemota int) (uint32, bool) {
	const (
		tcpTableOwnerPid = 5
		afInet           = 2
	)

	var tamanho uint32

	resultado, _, _ := pGetExtendedTCPTable.Call(0, uintptr(unsafe.Pointer(&tamanho)), 0, tcpTableOwnerPid, afInet, 0)
	if resultado != 122 || tamanho < 4 {
		return 0, false
	}

	buf := make([]byte, tamanho)

	resultado, _, _ = pGetExtendedTCPTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&tamanho)),
		0, tcpTableOwnerPid, afInet, 0)
	if resultado != 0 {
		return 0, false
	}

	n := *(*uint32)(unsafe.Pointer(&buf[0]))

	return procurarDonoTCP(buf, n, portaLocal, portaRemota)
}

// procurarDonoTCP varre a tabela TCP atras da conexao local indicada.
func procurarDonoTCP(buf []byte, total uint32, portaLocal, portaRemota int) (uint32, bool) {
	const (
		loopback                 = 0x0100007F // 127.0.0.1 em little-endian
		estadoEstab              = 5
		passo                    = 24 // 6 x uint32
		tamanhoContagemTabelaTCP = 4
	)

	for i := range total {
		off := tamanhoContagemTabelaTCP + i*passo
		if int(off+passo) > len(buf) {
			break
		}

		linha := (*linhaTCPDono)(unsafe.Pointer(&buf[off]))
		if !casaConexaoTCP(linha, portaLocal, portaRemota, loopback, estadoEstab) {
			continue
		}

		return linha.PidDono, true
	}

	return 0, false
}

// casaConexaoTCP diz se a linha e a conexao estabelecida procurada.
func casaConexaoTCP(linha *linhaTCPDono, portaLocal, portaRemota int, loopback, estadoEstab uint32) bool {
	if linha.Estado != estadoEstab {
		return false
	}

	if linha.AddrLocal != loopback || linha.AddrRemoto != loopback {
		return false
	}

	if portaOrdemRede(linha.PortaLocal) != portaLocal {
		return false
	}

	return portaOrdemRede(linha.PortaRemota) == portaRemota
}

func (p *vpnProxy) handle(conexao net.Conn) {
	defer conexao.Close()

	const timeoutLeituraProxySegundos = 45

	_ = conexao.SetDeadline(time.Now().Add(timeoutLeituraProxySegundos * time.Second))

	const (
		tamanhoBufferProxyKB = 32
		bytesPorKBProxy      = 1024
	)

	br := bufio.NewReaderSize(conexao, tamanhoBufferProxyKB*bytesPorKBProxy)

	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if strings.EqualFold(req.Method, http.MethodConnect) {
		p.handleConnect(conexao, req)

		return
	}

	p.handleHTTP(conexao, req)
}

func (p *vpnProxy) handleConnect(conexao net.Conn, req *http.Request) {
	alvo := req.Host
	if !strings.Contains(alvo, ":") {
		alvo += ":443"
	}

	if p.alvoProibido(alvo) {
		escreverErro(conexao, erroTexto("destino local bloqueado"))

		return
	}

	// GOD: dial com timeout + cancelamento no shutdown (ctx de lifetime do proxy).
	const timeoutDiscagemProxySegundos = 30

	ctx, cancel := context.WithTimeout(p.ctx, timeoutDiscagemProxySegundos*time.Second)
	defer cancel()

	//nolint:contextcheck // Raiz e o ctx de lifetime possuido pelo proxy (p.ctx), nao Background solto.
	conexaoTunel, err := p.mgr.dial(ctx, alvo)
	if err != nil {
		escreverErro(conexao, err)

		return
	}
	defer conexaoTunel.Close()

	if _, err := io.WriteString(conexao, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	if f, ok := conexao.(*net.TCPConn); ok {
		_ = f.SetDeadline(time.Time{})
	}

	if f, ok := conexaoTunel.(*net.TCPConn); ok {
		_ = f.SetDeadline(time.Time{})
	}

	relay(conexao, conexaoTunel)
}

func (p *vpnProxy) handleHTTP(conexao net.Conn, req *http.Request) {
	alvo := req.URL.Host
	if alvo == "" {
		alvo = req.Host
	}

	if !strings.Contains(alvo, ":") {
		alvo += ":80"
	}

	if p.alvoProibido(alvo) {
		escreverErro(conexao, erroTexto("destino local bloqueado"))

		return
	}

	const timeoutDiscagemHTTPProxySegundos = 30

	ctx, cancel := context.WithTimeout(p.ctx, timeoutDiscagemHTTPProxySegundos*time.Second)
	defer cancel()

	//nolint:contextcheck // Raiz e o ctx de lifetime possuido pelo proxy (p.ctx), nao Background solto.
	conexaoTunel, err := p.mgr.dial(ctx, alvo)
	if err != nil {
		escreverErro(conexao, err)

		return
	}
	defer conexaoTunel.Close()

	req.RequestURI = ""
	req.Header.Del("Proxy-Connection")
	req.Header.Set("Connection", "close")

	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Overdrive")
	}

	if err := req.Write(conexaoTunel); err != nil {
		return
	}

	_ = conexao.SetDeadline(time.Time{})
	_, _ = io.Copy(conexao, conexaoTunel)
}

func (p *vpnProxy) alvoProibido(alvo string) bool {
	host, _, err := net.SplitHostPort(alvo)
	if err != nil {
		return false
	}

	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if strings.Contains(host, "%") {
		return true
	}

	if host == hostLocal || strings.HasSuffix(host, ".localhost") {
		return true
	}

	for _, suf := range []string{sufLocal, sufLan, sufHome, sufInternal, ".intranet", ".corp", ".invalid", ".test"} {
		if strings.HasSuffix(host, suf) {
			return true
		}
	}

	if ip := net.ParseIP(host); ip != nil {
		return ipProibido(ip)
	}

	return false
}

// ipProibido bloqueia destinos locais, privados e reservados. Vale tanto
// para IP literal quanto para nomes que resolvem para essas faixas (evita
// DNS rebinding contra servicos da propria maquina ou da LAN).
func ipProibido(enderecoIP net.IP) bool {
	if enderecoIP == nil {
		return false
	}

	return enderecoIP.IsLoopback() || enderecoIP.IsUnspecified() || enderecoIP.IsPrivate() ||
		enderecoIP.IsLinkLocalUnicast() || enderecoIP.IsLinkLocalMulticast() ||
		enderecoIP.IsMulticast() || isCGNAT(enderecoIP) || !enderecoIP.IsGlobalUnicast()
}

// isCGNAT cobre 100.64.0.0/10 (CG-NAT de operadora): o Go nao inclui
// essa faixa em IsPrivate nem a exclui de IsGlobalUnicast.
func isCGNAT(enderecoIP net.IP) bool {
	v4 := enderecoIP.To4()

	return v4 != nil && v4[0] == 100 && v4[1]&0xC0 == 0x40
}

func escreverErro(conexao net.Conn, err error) {
	msg := "VPN indisponivel: " + err.Error()

	const limiteMensagemErroProxy = 300

	if len(msg) > limiteMensagemErroProxy {
		msg = msg[:limiteMensagemErroProxy]
	}

	_, _ = fmt.Fprintf(conexao,
		"HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain; charset=utf-8\r\n"+
			"Content-Length: %d\r\nConnection: close\r\n\r\n%s",
		len(msg), msg)
}

func relay(conexaoA, conexaoB net.Conn) {
	const direcoesRelay = 2

	done := make(chan struct{}, direcoesRelay)

	copiar := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		if t, ok := dst.(*net.TCPConn); ok {
			_ = t.CloseWrite()
		}

		done <- struct{}{}
	}
	go copiar(conexaoA, conexaoB)
	go copiar(conexaoB, conexaoA)

	<-done
	// Depois do primeiro sentido terminar, da um tempo para o outro drenar
	// antes de fechar os dois lados. Fechar na hora truncava respostas que
	// ainda estavam em transito.
	const esperaDrenoRelaySegundos = 5
	select {
	case <-done:
	case <-time.After(esperaDrenoRelaySegundos * time.Second):
	}

	_ = conexaoA.Close()
	_ = conexaoB.Close()
}
