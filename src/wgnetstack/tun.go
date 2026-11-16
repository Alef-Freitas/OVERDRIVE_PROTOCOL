/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2025 WireGuard LLC. All Rights Reserved.
 */

// Package netstack adapta a pilha TCP/IP do gVisor como TUN virtual do OVERDRIVE.
package netstack

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/tun"

	"golang.org/x/net/dns/dnsmessage"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// Op e rede do dial da pilha virtual.
const (
	opDial   = "dial"
	redePing = "ping"
)

// erroTexto cria erro dinamico: ponto unico do pacote (diagnostico em
// ingles da pilha de rede, nao UI).
func erroTexto(txt string) error {
	return errors.New(txt) //nolint:err113 // Ponto unico do pacote; New direto proibido.
}

// erroTextoF e o equivalente formatado (inclusive %w: repassado intacto).
func erroTextoF(formato string, args ...any) error {
	return fmt.Errorf(formato, args...) //nolint:err113 // Ponto unico do pacote; fmt.Errorf direto continua proibido.
}

//exhaustruct:ignore // Pilha virtual: notifyHandle/hasV4/hasV6 fixados apos criacao.
type netTun struct {
	ep             *channel.Endpoint
	stack          *stack.Stack
	events         chan tun.Event
	notifyHandle   *channel.NotificationHandle
	incomingPacket chan *buffer.View
	mtu            int
	dnsServers     []netip.Addr
	hasV4, hasV6   bool
}

// Net e a pilha TCP/IP virtual do tunel (gVisor).
type Net netTun

// montarPilhaTun monta a pilha gVisor com NIC e SACK.
func montarPilhaTun(dnsServers []netip.Addr, mtu int) *netTun {
	const (
		tamanhoFilaTun       = 1024
		capacidadeEventosTun = 10
	)
	//exhaustruct:ignore // stack.Options: relogio/stats padrao do gVisor; resto zero intencional.
	opts := stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol6,
			icmp.NewProtocol4,
		},
		HandleLocal: true,
	}
	dev := &netTun{
		ep:             channel.New(tamanhoFilaTun, uint32(mtu), ""),
		stack:          stack.New(opts),
		events:         make(chan tun.Event, capacidadeEventosTun),
		incomingPacket: make(chan *buffer.View),
		dnsServers:     dnsServers,
		mtu:            mtu,
	}

	return dev
}

func protocoloDoIP(ip netip.Addr) tcpip.NetworkProtocolNumber {
	if ip.Is4() {
		return ipv4.ProtocolNumber
	}

	return ipv6.ProtocolNumber
}

func adicionarEnderecoTun(dev *netTun, endereco netip.Addr) error {
	protoAddr := tcpip.ProtocolAddress{
		Protocol:          protocoloDoIP(endereco),
		AddressWithPrefix: tcpip.AddrFromSlice(endereco.AsSlice()).WithPrefix(),
	}

	//exhaustruct:ignore // AddressProperties vazio: endereco sem propriedades especiais.
	tcpipErr := dev.stack.AddProtocolAddress(1, protoAddr, stack.AddressProperties{})
	if tcpipErr != nil {
		return erroTextoF("AddProtocolAddress(%v): %v", endereco, tcpipErr)
	}

	if endereco.Is4() {
		dev.hasV4 = true
	} else if endereco.Is6() {
		dev.hasV6 = true
	}

	return nil
}

func configurarRotasTun(dev *netTun) {
	if dev.hasV4 {
		//exhaustruct:ignore // Rota direta na NIC virtual: resto zero intencional.
		dev.stack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: 1})
	}

	if dev.hasV6 {
		//exhaustruct:ignore // Rota direta na NIC virtual: resto zero intencional.
		dev.stack.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: 1})
	}
}

// CreateNetTUN cria a pilha virtual com enderecos, DNS e MTU dados.
//
//nolint:ireturn // Assinatura imposta pelo wireguard: device.NewDevice exige tun.Device.
func CreateNetTUN(localAddresses, dnsServers []netip.Addr, mtu int) (tun.Device, *Net, error) {
	dev := montarPilhaTun(dnsServers, mtu)

	tcpipErr := dev.enableSACK()
	if tcpipErr != nil {
		return nil, nil, erroTextoF("could not enable TCP SACK: %v", tcpipErr)
	}

	dev.notifyHandle = dev.ep.AddNotify(dev)

	tcpipErr = dev.stack.CreateNIC(1, dev.ep)
	if tcpipErr != nil {
		return nil, nil, erroTextoF("CreateNIC: %v", tcpipErr)
	}

	for _, ip := range localAddresses {
		if err := adicionarEnderecoTun(dev, ip); err != nil {
			return nil, nil, err
		}
	}

	configurarRotasTun(dev)

	dev.events <- tun.EventUp

	return dev, (*Net)(dev), nil
}

func (tun *netTun) Name() (string, error) {
	return "go", nil
}

func (tun *netTun) File() *os.File {
	return nil
}

func (tun *netTun) Events() <-chan tun.Event {
	return tun.events
}

func (tun *netTun) Read(buf [][]byte, sizes []int, offset int) (int, error) {
	view, ok := <-tun.incomingPacket
	if !ok {
		return 0, os.ErrClosed
	}

	n, err := view.Read(buf[0][offset:])
	if err != nil {
		return 0, erroTextoF("ler pacote: %w", err)
	}

	sizes[0] = n

	return 1, nil
}

func (tun *netTun) Write(buf [][]byte, offset int) (int, error) {
	for _, buf := range buf {
		packet := buf[offset:]
		if len(packet) == 0 {
			continue
		}

		//exhaustruct:ignore // PacketBufferOptions: so payload; resto zero intencional.
		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})

		const (
			versaoIPv4Tun = 4
			versaoIPv6Tun = 6
		)
		switch packet[0] >> 4 {
		case versaoIPv4Tun:
			tun.ep.InjectInbound(header.IPv4ProtocolNumber, pkb)
		case versaoIPv6Tun:
			tun.ep.InjectInbound(header.IPv6ProtocolNumber, pkb)
		default:
			return 0, syscall.EAFNOSUPPORT
		}
	}

	return len(buf), nil
}

func (tun *netTun) WriteNotify() {
	pkt := tun.ep.Read()
	if pkt == nil {
		return
	}

	view := pkt.ToView()
	pkt.DecRef()

	tun.incomingPacket <- view
}

func (tun *netTun) Close() error {
	tun.stack.RemoveNIC(1)
	tun.stack.Close()
	tun.ep.RemoveNotify(tun.notifyHandle)
	tun.ep.Close()

	if tun.events != nil {
		close(tun.events)
	}

	if tun.incomingPacket != nil {
		close(tun.incomingPacket)
	}

	return nil
}

func (tun *netTun) MTU() (int, error) {
	return tun.mtu, nil
}

func (tun *netTun) BatchSize() int {
	return 1
}

//nolint:ireturn // Assinatura imposta pelo gVisor: SetTransportProtocolOption retorna tcpip.Error.
func (tun *netTun) enableSACK() tcpip.Error {
	sackEnabledOpt := tcpip.TCPSACKEnabled(true)

	return tun.stack.SetTransportProtocolOption(tcp.ProtocolNumber, &sackEnabledOpt)
}

func convertToFullAddr(endpoint netip.AddrPort) (tcpip.FullAddress, tcpip.NetworkProtocolNumber) {
	var protoNumber tcpip.NetworkProtocolNumber
	if endpoint.Addr().Is4() {
		protoNumber = ipv4.ProtocolNumber
	} else {
		protoNumber = ipv6.ProtocolNumber
	}

	//exhaustruct:ignore // LinkAddr zero: rota sem endereco de enlace.
	return tcpip.FullAddress{
		NIC:  1,
		Addr: tcpip.AddrFromSlice(endpoint.Addr().AsSlice()),
		Port: endpoint.Port(),
	}, protoNumber
}

// DialContextTCPAddrPort disca TCP pelo tunel ate o endereco dado.
func (tnet *Net) DialContextTCPAddrPort(ctx context.Context, addr netip.AddrPort) (*gonet.TCPConn, error) {
	fa, pn := convertToFullAddr(addr)

	c, err := gonet.DialContextTCP(ctx, tnet.stack, fa, pn)
	if err != nil {
		return nil, erroTextoF("discar tcp: %w", err)
	}

	return c, nil
}

// DialContextTCP disca TCP pelo tunel ate o endereco dado.
func (tnet *Net) DialContextTCP(ctx context.Context, addr *net.TCPAddr) (*gonet.TCPConn, error) {
	if addr == nil {
		return tnet.DialContextTCPAddrPort(ctx, netip.AddrPort{})
	}

	ip, _ := netip.AddrFromSlice(addr.IP)

	return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(ip, uint16(addr.Port)))
}

// DialTCPAddrPort disca TCP pelo tunel ate o endereco dado.
func (tnet *Net) DialTCPAddrPort(addr netip.AddrPort) (*gonet.TCPConn, error) {
	fa, pn := convertToFullAddr(addr)

	c, err := gonet.DialTCP(tnet.stack, fa, pn)
	if err != nil {
		return nil, erroTextoF("discar tcp: %w", err)
	}

	return c, nil
}

// DialTCP disca TCP pelo tunel ate o endereco dado.
func (tnet *Net) DialTCP(addr *net.TCPAddr) (*gonet.TCPConn, error) {
	if addr == nil {
		return tnet.DialTCPAddrPort(netip.AddrPort{})
	}

	ip, _ := netip.AddrFromSlice(addr.IP)

	return tnet.DialTCPAddrPort(netip.AddrPortFrom(ip, uint16(addr.Port)))
}

// ListenTCPAddrPort escuta TCP no tunel no endereco dado.
func (tnet *Net) ListenTCPAddrPort(addr netip.AddrPort) (*gonet.TCPListener, error) {
	fa, pn := convertToFullAddr(addr)

	l, err := gonet.ListenTCP(tnet.stack, fa, pn)
	if err != nil {
		return nil, erroTextoF("ouvir tcp: %w", err)
	}

	return l, nil
}

// ListenTCP escuta TCP no tunel no endereco dado.
func (tnet *Net) ListenTCP(addr *net.TCPAddr) (*gonet.TCPListener, error) {
	if addr == nil {
		return tnet.ListenTCPAddrPort(netip.AddrPort{})
	}

	ip, _ := netip.AddrFromSlice(addr.IP)

	return tnet.ListenTCPAddrPort(netip.AddrPortFrom(ip, uint16(addr.Port)))
}

// DialUDPAddrPort disca UDP pelo tunel.
func (tnet *Net) DialUDPAddrPort(laddr, raddr netip.AddrPort) (*gonet.UDPConn, error) {
	var (
		lfa, rfa      *tcpip.FullAddress
		protocoloRede tcpip.NetworkProtocolNumber
	)

	if laddr.IsValid() || laddr.Port() > 0 {
		var addr tcpip.FullAddress

		addr, protocoloRede = convertToFullAddr(laddr)
		lfa = &addr
	}

	if raddr.IsValid() || raddr.Port() > 0 {
		var addr tcpip.FullAddress

		addr, protocoloRede = convertToFullAddr(raddr)
		rfa = &addr
	}

	u, err := gonet.DialUDP(tnet.stack, lfa, rfa, protocoloRede)
	if err != nil {
		return nil, erroTextoF("discar udp: %w", err)
	}

	return u, nil
}

// ListenUDPAddrPort escuta UDP no tunel.
func (tnet *Net) ListenUDPAddrPort(laddr netip.AddrPort) (*gonet.UDPConn, error) {
	return tnet.DialUDPAddrPort(laddr, netip.AddrPort{})
}

// DialUDP disca UDP pelo tunel.
func (tnet *Net) DialUDP(laddr, raddr *net.UDPAddr) (*gonet.UDPConn, error) {
	var local, remoto netip.AddrPort

	if laddr != nil {
		ip, _ := netip.AddrFromSlice(laddr.IP)
		local = netip.AddrPortFrom(ip, uint16(laddr.Port))
	}

	if raddr != nil {
		ip, _ := netip.AddrFromSlice(raddr.IP)
		remoto = netip.AddrPortFrom(ip, uint16(raddr.Port))
	}

	return tnet.DialUDPAddrPort(local, remoto)
}

// ListenUDP escuta UDP no tunel.
func (tnet *Net) ListenUDP(laddr *net.UDPAddr) (*gonet.UDPConn, error) {
	return tnet.DialUDP(laddr, nil)
}

// PingConn e uma conexao ICMP (ping) pelo tunel.
type PingConn struct {
	laddr    PingAddr
	raddr    PingAddr
	wq       waiter.Queue
	ep       tcpip.Endpoint
	deadline *time.Timer
}

// PingAddr e um endereco para ping pelo tunel.
type PingAddr struct{ addr netip.Addr }

func (ia PingAddr) String() string {
	return ia.addr.String()
}

// Network devolve o nome da rede do endereco de ping.
func (ia PingAddr) Network() string {
	if ia.addr.Is4() {
		return "ping4"
	} else if ia.addr.Is6() {
		return "ping6"
	}

	return redePing
}

// Addr devolve o endereco IP do ping.
func (ia PingAddr) Addr() netip.Addr {
	return ia.addr
}

// PingAddrFromAddr converte um endereco IP em endereco de ping.
func PingAddrFromAddr(addr netip.Addr) *PingAddr {
	return &PingAddr{addr}
}

func origemPingTun(laddr, _ netip.Addr, usaIPv6 bool) (netip.Addr, bool) {
	bind := laddr.IsValid()
	if bind {
		return laddr, bind
	}

	if usaIPv6 {
		return netip.IPv6Unspecified(), bind
	}

	return netip.IPv4Unspecified(), bind
}

func protocolosPingTun(usaIPv6 bool) (tcpip.TransportProtocolNumber, tcpip.NetworkProtocolNumber) {
	if usaIPv6 {
		return icmp.ProtocolNumber6, ipv6.ProtocolNumber
	}

	return icmp.ProtocolNumber4, ipv4.ProtocolNumber
}

func novoPingConn(laddr netip.Addr) *PingConn {
	const deslocamentoPrazoMaximoPing = 10
	//exhaustruct:ignore // PingConn: raddr/wq/ep fixados no dial; resto no construtor.
	conexao := &PingConn{
		laddr:    PingAddr{laddr},
		deadline: time.NewTimer(time.Hour << deslocamentoPrazoMaximoPing),
	}
	conexao.deadline.Stop()

	return conexao
}

func ligarPingTun(conexao *PingConn, laddr netip.Addr, bind bool) error {
	fa, _ := convertToFullAddr(netip.AddrPortFrom(laddr, 0))
	if bind {
		if tcpipErr := conexao.ep.Bind(fa); tcpipErr != nil {
			return erroTextoF("ping bind: %s", tcpipErr)
		}
	}

	return nil
}

func conectarPingTun(conexao *PingConn, raddr netip.Addr) error {
	if !raddr.IsValid() {
		return nil
	}

	conexao.raddr = PingAddr{raddr}

	fa, _ := convertToFullAddr(netip.AddrPortFrom(raddr, 0))
	if tcpipErr := conexao.ep.Connect(fa); tcpipErr != nil {
		return erroTextoF("ping connect: %s", tcpipErr)
	}

	return nil
}

// DialPingAddr disca ping pelo tunel.
func (tnet *Net) DialPingAddr(laddr, raddr netip.Addr) (*PingConn, error) {
	if !laddr.IsValid() && !raddr.IsValid() {
		return nil, erroTexto("ping dial: invalid address")
	}

	v6 := laddr.Is6() || raddr.Is6()

	laddr, bind := origemPingTun(laddr, raddr, v6)
	tn, pn := protocolosPingTun(v6)

	conexao := novoPingConn(laddr)

	ep, tcpipErr := tnet.stack.NewEndpoint(tn, pn, &conexao.wq)
	if tcpipErr != nil {
		return nil, erroTextoF("ping socket: endpoint: %s", tcpipErr)
	}

	conexao.ep = ep

	if err := ligarPingTun(conexao, laddr, bind); err != nil {
		return nil, err
	}

	if err := conectarPingTun(conexao, raddr); err != nil {
		return nil, err
	}

	return conexao, nil
}

// ListenPingAddr escuta ping no tunel.
func (tnet *Net) ListenPingAddr(laddr netip.Addr) (*PingConn, error) {
	return tnet.DialPingAddr(laddr, netip.Addr{})
}

// DialPing disca ping pelo tunel.
func (tnet *Net) DialPing(laddr, raddr *PingAddr) (*PingConn, error) {
	var local, remoto netip.Addr
	if laddr != nil {
		local = laddr.addr
	}

	if raddr != nil {
		remoto = raddr.addr
	}

	return tnet.DialPingAddr(local, remoto)
}

// ListenPing escuta ping no tunel.
func (tnet *Net) ListenPing(laddr *PingAddr) (*PingConn, error) {
	var local netip.Addr
	if laddr != nil {
		local = laddr.addr
	}

	return tnet.ListenPingAddr(local)
}

// LocalAddr devolve o endereco local da conexao de ping.
func (pc *PingConn) LocalAddr() net.Addr {
	return pc.laddr
}

// RemoteAddr devolve o endereco remoto da conexao de ping.
func (pc *PingConn) RemoteAddr() net.Addr {
	return pc.raddr
}

// Close fecha a conexao de ping.
func (pc *PingConn) Close() error {
	pc.deadline.Reset(0)
	pc.ep.Close()

	return nil
}

// SetWriteDeadline fixa o prazo de escrita da conexao de ping.
func (pc *PingConn) SetWriteDeadline(_ time.Time) error {
	return erroTexto("not implemented")
}

// WriteTo envia um pacote de ping ao endereco dado.
func (pc *PingConn) WriteTo(dados []byte, addr net.Addr) (int, error) {
	var destino netip.Addr

	switch v := addr.(type) {
	case *PingAddr:
		destino = v.addr
	case *net.IPAddr:
		destino, _ = netip.AddrFromSlice(v.IP)
	default:
		return 0, erroTexto("ping write: wrong net.Addr type")
	}

	mesmaFamilia := destino.Is4() && pc.laddr.addr.Is4() || destino.Is6() && pc.laddr.addr.Is6()
	if !mesmaFamilia {
		return 0, erroTexto("ping write: mismatched protocols")
	}

	buf := bytes.NewReader(dados)
	rfa, _ := convertToFullAddr(netip.AddrPortFrom(destino, 0))
	// won't block, no deadlines
	//exhaustruct:ignore // WriteOptions: so destino; resto zero intencional.
	n64, tcpipErr := pc.ep.Write(buf, tcpip.WriteOptions{
		To: &rfa,
	})
	if tcpipErr != nil {
		return int(n64), erroTextoF("ping write: %s", tcpipErr)
	}

	return int(n64), nil
}

func (pc *PingConn) Write(p []byte) (int, error) {
	return pc.WriteTo(p, &pc.raddr)
}

// ReadFrom le um pacote de ping e diz de onde veio.
func (pc *PingConn) ReadFrom(dados []byte) (int, net.Addr, error) {
	e, notifyCh := waiter.NewChannelEntry(waiter.EventIn)

	pc.wq.EventRegister(&e)
	defer pc.wq.EventUnregister(&e)

	select {
	case <-pc.deadline.C:
		return 0, nil, os.ErrDeadlineExceeded
	case <-notifyCh:
	}

	w := tcpip.SliceWriter(dados)

	//exhaustruct:ignore // ReadOptions: so endereco remoto; resto zero intencional.
	res, tcpipErr := pc.ep.Read(&w, tcpip.ReadOptions{
		NeedRemoteAddr: true,
	})
	if tcpipErr != nil {
		return 0, nil, erroTextoF("ping read: %s", tcpipErr)
	}

	remoteAddr, _ := netip.AddrFromSlice(res.RemoteAddr.Addr.AsSlice())

	return res.Count, &PingAddr{remoteAddr}, nil
}

func (pc *PingConn) Read(p []byte) (int, error) {
	n, _, err := pc.ReadFrom(p)

	return n, err
}

// SetDeadline fixa o prazo da conexao de ping.
func (pc *PingConn) SetDeadline(t time.Time) error {
	// pc.SetWriteDeadline is unimplemented
	return pc.SetReadDeadline(t)
}

// SetReadDeadline fixa o prazo de leitura da conexao de ping.
func (pc *PingConn) SetReadDeadline(t time.Time) error {
	pc.deadline.Reset(time.Until(t))

	return nil
}

var (
	errNoSuchHost                   = erroTexto("no such host")
	errLameReferral                 = erroTexto("lame referral")
	errCannotUnmarshalDNSMessage    = erroTexto("cannot unmarshal DNS message")
	errCannotMarshalDNSMessage      = erroTexto("cannot marshal DNS message")
	errServerMisbehaving            = erroTexto("server misbehaving")
	errInvalidDNSResponse           = erroTexto("invalid DNS response")
	errNoAnswerFromDNSServer        = erroTexto("no answer from DNS server")
	errServerTemporarilyMisbehaving = erroTexto("server misbehaving")
	errCanceled                     = erroTexto("operation was canceled")
	errTimeout                      = erroTexto("i/o timeout")
	errNumericPort                  = erroTexto("port must be numeric")
	errNoSuitableAddress            = erroTexto("no suitable address found")
	errMissingAddress               = erroTexto("missing address")
)

// LookupHost resolve o host pelo DNS do tunel.
// Wrapper sincrono: sem ctx herdavel; o chamador com prazo usa LookupContextHost.
func (tnet *Net) LookupHost(host string) ([]string, error) {
	return tnet.LookupContextHost(context.Background(), host)
}

func tamanhoDominioValido(dominio string) bool {
	l := len(dominio)
	if l == 0 || l > 254 {
		return false
	}

	return l != 254 || dominio[l-1] == '.'
}

func consumirLetraDominio(letra byte, partlen *int, nonNumeric *bool) bool {
	switch {
	case 'a' <= letra && letra <= 'z' || 'A' <= letra && letra <= 'Z' || letra == '_':
		*nonNumeric = true
		*partlen++

		return true
	case '0' <= letra && letra <= '9':
		*partlen++

		return true
	default:
		return false
	}
}

func consumirTracoDominio(last byte, partlen *int, nonNumeric *bool) bool {
	if last == '.' {
		return false
	}

	*partlen++
	*nonNumeric = true

	return true
}

func consumirPontoDominio(last byte, partlen *int) bool {
	if last == '.' || last == '-' {
		return false
	}

	if *partlen > 63 || *partlen == 0 {
		return false
	}

	*partlen = 0

	return true
}

func consumirCaractereDominio(letra, last byte, partlen *int, nonNumeric *bool) bool {
	switch letra {
	case '-':
		return consumirTracoDominio(last, partlen, nonNumeric)
	case '.':
		return consumirPontoDominio(last, partlen)
	default:
		return consumirLetraDominio(letra, partlen, nonNumeric)
	}
}

func isDomainName(dominio string) bool {
	if !tamanhoDominioValido(dominio) {
		return false
	}

	last := byte('.')
	nonNumeric := false
	partlen := 0

	for i := range len(dominio) {
		letra := dominio[i]
		if !consumirCaractereDominio(letra, last, &partlen, &nonNumeric) {
			return false
		}

		last = letra
	}

	if last == '-' || partlen > 63 {
		return false
	}

	return nonNumeric
}

func randU16() uint16 {
	var par [2]byte

	_, err := rand.Read(par[:])
	if err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint16(par[:])
}

func newRequest(pergunta dnsmessage.Question) (uint16, []byte, []byte, error) {
	const (
		tamanhoCabecalhoDNS    = 2
		prefixoTamanhoTCP      = 2
		deslocamentoTamanhoTCP = 8
		tamanhoMaxMensagemDNS  = 514
	)

	identificador := randU16()
	//exhaustruct:ignore // Header DNS: so ID + recursao; flags zerados intencional.
	construtor := dnsmessage.NewBuilder(
		make([]byte, tamanhoCabecalhoDNS, tamanhoMaxMensagemDNS),
		dnsmessage.Header{ID: identificador, RecursionDesired: true})
	construtor.EnableCompression()

	if err := construtor.StartQuestions(); err != nil {
		return 0, nil, nil, erroTextoF("dns pergunta: %w", err)
	}

	if err := construtor.Question(pergunta); err != nil {
		return 0, nil, nil, erroTextoF("dns pergunta: %w", err)
	}

	tcpReq, err := construtor.Finish()
	if err != nil {
		return 0, nil, nil, erroTextoF("dns montar: %w", err)
	}

	udpReq := tcpReq[prefixoTamanhoTCP:]
	l := len(tcpReq) - prefixoTamanhoTCP
	tcpReq[0] = byte(l >> deslocamentoTamanhoTCP)
	tcpReq[1] = byte(l)

	return identificador, udpReq, tcpReq, nil
}

func equalASCIIName(nomeA, nomeB dnsmessage.Name) bool {
	if nomeA.Length != nomeB.Length {
		return false
	}

	for i := range int(nomeA.Length) {
		caractereA := nomeA.Data[i]
		caractereB := nomeB.Data[i]

		if 'A' <= caractereA && caractereA <= 'Z' {
			caractereA += 0x20
		}

		if 'A' <= caractereB && caractereB <= 'Z' {
			caractereB += 0x20
		}

		if caractereA != caractereB {
			return false
		}
	}

	return true
}

func checkResponse(reqID uint16, reqQues dnsmessage.Question, respHdr dnsmessage.Header,
	respQues dnsmessage.Question,
) bool {
	if !respHdr.Response {
		return false
	}

	if reqID != respHdr.ID {
		return false
	}

	if reqQues.Type != respQues.Type || reqQues.Class != respQues.Class || !equalASCIIName(reqQues.Name, respQues.Name) {
		return false
	}

	return true
}

func dnsPacketRoundTrip(conexao net.Conn, identificador uint16, query dnsmessage.Question,
	dados []byte,
) (dnsmessage.Parser, dnsmessage.Header, error) {
	if _, err := conexao.Write(dados); err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, erroTextoF("dns enviar: %w", err)
	}

	const tamanhoBufferDNSUDP = 512

	dados = make([]byte, tamanhoBufferDNSUDP)
	for {
		numLidos, err := conexao.Read(dados)
		if err != nil {
			//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
			return dnsmessage.Parser{}, dnsmessage.Header{}, erroTextoF("dns ler: %w", err)
		}

		var leitor dnsmessage.Parser

		cabecalho, err := leitor.Start(dados[:numLidos])
		if err != nil {
			continue
		}

		pergunta, err := leitor.Question()
		if err != nil || !checkResponse(identificador, query, cabecalho, pergunta) {
			continue
		}

		return leitor, cabecalho, nil
	}
}

func dnsStreamRoundTrip(conexao net.Conn, ident uint16, query dnsmessage.Question,
	dados []byte,
) (dnsmessage.Parser, dnsmessage.Header, error) {
	if _, err := conexao.Write(dados); err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, erroTextoF("dns enviar: %w", err)
	}

	const tamanhoMaximoDNSTCP = 1280

	dados = make([]byte, tamanhoMaximoDNSTCP)
	if _, err := io.ReadFull(conexao, dados[:2]); err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, erroTextoF("dns ler cabecalho: %w", err)
	}

	const deslocamentoTamanhoTCP = 8

	l := int(dados[0])<<deslocamentoTamanhoTCP | int(dados[1])
	if l > len(dados) {
		dados = make([]byte, l)
	}

	numLidos, err := io.ReadFull(conexao, dados[:l])
	if err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, erroTextoF("dns ler corpo: %w", err)
	}

	var leitor dnsmessage.Parser

	cabecalho, err := leitor.Start(dados[:numLidos])
	if err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, errCannotUnmarshalDNSMessage
	}

	pergunta, err := leitor.Question()
	if err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, errCannotUnmarshalDNSMessage
	}

	if !checkResponse(ident, query, cabecalho, pergunta) {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, errInvalidDNSResponse
	}

	return leitor, cabecalho, nil
}

func aplicarPrazoDNSTun(ctx context.Context, c net.Conn) error {
	if d, ok := ctx.Deadline(); ok && !d.IsZero() {
		if err := c.SetDeadline(d); err != nil {
			return erroTextoF("dns prazo: %w", err)
		}
	}

	return nil
}

func traduzirErroDNSTun(err error) error {
	if errors.Is(err, context.Canceled) {
		return errCanceled
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errTimeout
	}

	return err
}

func executarIdaVoltaDNSTun(conexao net.Conn, ident uint16, pergunta dnsmessage.Question, udpReq, tcpReq []byte,
	useUDP bool,
) (dnsmessage.Parser, dnsmessage.Header, error) {
	if useUDP {
		return dnsPacketRoundTrip(conexao, ident, pergunta, udpReq)
	}

	return dnsStreamRoundTrip(conexao, ident, pergunta, tcpReq)
}

func verificarRCodeCabecalho(h dnsmessage.Header) error {
	switch h.RCode {
	case dnsmessage.RCodeSuccess:
		return nil
	case dnsmessage.RCodeNameError:
		return errNoSuchHost
	case dnsmessage.RCodeServerFailure:
		return errServerTemporarilyMisbehaving
	default:
		return errServerMisbehaving
	}
}

func verificarSecaoResposta(leitor *dnsmessage.Parser, cabecalho dnsmessage.Header) error {
	_, err := leitor.AnswerHeader()
	if err != nil && !errors.Is(err, dnsmessage.ErrSectionDone) {
		return errCannotUnmarshalDNSMessage
	}

	if cabecalho.RCode == dnsmessage.RCodeSuccess && !cabecalho.Authoritative && !cabecalho.RecursionAvailable &&
		errors.Is(err, dnsmessage.ErrSectionDone) {
		return errLameReferral
	}

	return nil
}

func checkHeader(leitor *dnsmessage.Parser, cabecalho dnsmessage.Header) error {
	if cabecalho.RCode == dnsmessage.RCodeNameError {
		return errNoSuchHost
	}

	if err := verificarSecaoResposta(leitor, cabecalho); err != nil {
		return err
	}

	return verificarRCodeCabecalho(cabecalho)
}

func skipToAnswer(leitor *dnsmessage.Parser, qtype dnsmessage.Type) error {
	for {
		cabecalho, err := leitor.AnswerHeader()
		if errors.Is(err, dnsmessage.ErrSectionDone) {
			return errNoSuchHost
		}

		if err != nil {
			return errCannotUnmarshalDNSMessage
		}

		if cabecalho.Type == qtype {
			return nil
		}

		if err := leitor.SkipAnswer(); err != nil {
			return errCannotUnmarshalDNSMessage
		}
	}
}

func erroDNSTunTentativa(err error, name, server string) *net.DNSError {
	//exhaustruct:ignore // DNSError: flags ajustados apos (Timeout/IsNotFound); resto zero intencional.
	dnsErr := &net.DNSError{
		Err:    err.Error(),
		Name:   name,
		Server: server,
	}
	if nerr, ok := errors.AsType[net.Error](err); ok && nerr.Timeout() {
		dnsErr.IsTimeout = true
	}

	if _, ok := errors.AsType[*net.OpError](err); ok {
		dnsErr.IsTemporary = true
	}

	return dnsErr
}

func erroDNSTunCabecalho(err error, name, server string) *net.DNSError {
	//exhaustruct:ignore // DNSError: flags ajustados apos (Timeout/IsNotFound); resto zero intencional.
	dnsErr := &net.DNSError{
		Err:    err.Error(),
		Name:   name,
		Server: server,
	}
	if errors.Is(err, errServerTemporarilyMisbehaving) {
		dnsErr.IsTemporary = true
	}

	if errors.Is(err, errNoSuchHost) {
		dnsErr.IsNotFound = true
	}

	return dnsErr
}

func consultarServidorDNSTun(ctx context.Context, tnet *Net, q dnsmessage.Question, server netip.Addr,
	name string,
) (dnsmessage.Parser, string, bool, error) {
	const timeoutConsultaDNSSegundos = 5

	leitor, cabecalho, err := tnet.exchange(ctx, server, q, time.Second*timeoutConsultaDNSSegundos)
	if err != nil {
		return dnsmessage.Parser{}, "", false, erroDNSTunTentativa(err, name, server.String())
	}

	if err := checkHeader(&leitor, cabecalho); err != nil {
		dnsErr := erroDNSTunCabecalho(err, name, server.String())
		if errors.Is(err, errNoSuchHost) {
			return leitor, server.String(), true, dnsErr
		}

		return dnsmessage.Parser{}, "", false, dnsErr
	}

	return leitor, server.String(), false, nil
}

func concluirConsultaDNSTun(leitor *dnsmessage.Parser, qtype dnsmessage.Type, name, server string) (
	dnsmessage.Parser, string, bool, error,
) {
	err := skipToAnswer(leitor, qtype)
	if err == nil {
		return *leitor, server, true, nil
	}

	if errors.Is(err, errNoSuchHost) {
		//exhaustruct:ignore // DNSError: flags ajustados abaixo; resto zero intencional.
		naoAchado := &net.DNSError{
			Err:    err.Error(),
			Name:   name,
			Server: server,
		}
		if derr, ok := errors.AsType[*net.DNSError](naoAchado); ok {
			derr.IsNotFound = true
		}

		return *leitor, server, true, naoAchado
	}

	//exhaustruct:ignore // DNSError: resposta de erro simples; resto zero intencional.
	falha := &net.DNSError{
		Err:    err.Error(),
		Name:   name,
		Server: server,
	}

	return dnsmessage.Parser{}, "", false, falha
}

func tentarServidoresDNSTun(ctx context.Context, tnet *Net, pergunta dnsmessage.Question, qtype dnsmessage.Type,
	name string,
) (dnsmessage.Parser, string, bool, error) {
	var lastErr error

	for _, server := range tnet.dnsServers {
		leitor, srv, terminal, err := consultarServidorDNSTun(ctx, tnet, pergunta, server, name)
		if err != nil {
			lastErr = err
			if terminal {
				return leitor, srv, true, err
			}

			continue
		}

		rp, rsrv, concluido, rerr := concluirConsultaDNSTun(&leitor, qtype, name, srv)
		if concluido {
			return rp, rsrv, true, rerr
		}

		if rerr != nil {
			lastErr = rerr
		}
	}

	return dnsmessage.Parser{}, "", false, lastErr
}

func comprimentoZonaHost(host string) int {
	zlen := len(host)
	if strings.IndexByte(host, ':') != -1 {
		if zidx := strings.LastIndexByte(host, '%'); zidx != -1 {
			zlen = zidx
		}
	}

	return zlen
}

func erroHostInexistenteTun(host string) *net.DNSError {
	//exhaustruct:ignore // DNSError: resposta NXDOMAIN estatica; resto zero intencional.
	return &net.DNSError{Err: errNoSuchHost.Error(), Name: host, IsNotFound: true}
}

func validarHostTun(tnet *Net, host string) (string, error) {
	if host == "" || (!tnet.hasV6 && !tnet.hasV4) {
		return "", erroHostInexistenteTun(host)
	}

	if ip, err := netip.ParseAddr(host[:comprimentoZonaHost(host)]); err == nil {
		return ip.String(), nil
	}

	if !isDomainName(host) {
		return "", erroHostInexistenteTun(host)
	}

	return "", nil
}

type resultadoDNSTun struct {
	erro error

	p      dnsmessage.Parser
	server string
}

func dispararConsultasDNSTun(ctx context.Context, tnet *Net, host string, lane chan resultadoDNSTun) int {
	lanes := 0
	if tnet.hasV4 {
		lanes++

		go func() {
			p, server, err := tnet.tryOneName(ctx, host+".", dnsmessage.TypeA)
			lane <- resultadoDNSTun{erro: err, p: p, server: server}
		}()
	}

	if tnet.hasV6 {
		lanes++

		go func() {
			p, server, err := tnet.tryOneName(ctx, host+".", dnsmessage.TypeAAAA)
			lane <- resultadoDNSTun{erro: err, p: p, server: server}
		}()
	}

	return lanes
}

func acumularRespostaATun(leitor *dnsmessage.Parser, host, server string, addrsV4 []netip.Addr,
) ([]netip.Addr, error) {
	recurso, err := leitor.AResource()
	if err != nil {
		//exhaustruct:ignore // DNSError: flags ajustados apos; resto zero intencional.
		return addrsV4, &net.DNSError{
			Err:    errCannotMarshalDNSMessage.Error(),
			Name:   host,
			Server: server,
		}
	}

	return append(addrsV4, netip.AddrFrom4(recurso.A)), nil
}

func acumularRespostaAAAATun(p *dnsmessage.Parser, host, server string, addrsV6 []netip.Addr,
) ([]netip.Addr, error) {
	aaaa, err := p.AAAAResource()
	if err != nil {
		//exhaustruct:ignore // DNSError: flags ajustados apos; resto zero intencional.
		return addrsV6, &net.DNSError{
			Err:    errCannotMarshalDNSMessage.Error(),
			Name:   host,
			Server: server,
		}
	}

	return append(addrsV6, netip.AddrFrom16(aaaa.AAAA)), nil
}

func erroRespostaDNSTun(host, server string) *net.DNSError {
	//exhaustruct:ignore // DNSError: flags ajustados apos; resto zero intencional.
	return &net.DNSError{
		Err:    errCannotMarshalDNSMessage.Error(),
		Name:   host,
		Server: server,
	}
}

func acumularRespostaTipoDNSTun(resultado *resultadoDNSTun, cabecalho dnsmessage.ResourceHeader, host string,
	addrsV4, addrsV6 []netip.Addr,
) ([]netip.Addr, []netip.Addr, bool, error) {
	switch cabecalho.Type {
	case dnsmessage.TypeA:
		addrsV4, err := acumularRespostaATun(&resultado.p, host, resultado.server, addrsV4)

		return addrsV4, addrsV6, err != nil, err
	case dnsmessage.TypeAAAA:
		addrsV6, err := acumularRespostaAAAATun(&resultado.p, host, resultado.server, addrsV6)

		return addrsV4, addrsV6, err != nil, err
	default:
		if err := resultado.p.SkipAnswer(); err != nil {
			return addrsV4, addrsV6, true, erroRespostaDNSTun(host, resultado.server)
		}

		return addrsV4, addrsV6, false, nil
	}
}

func coletarRespostaDNSTun(resultado resultadoDNSTun, host string, addrsV4, addrsV6 []netip.Addr,
) ([]netip.Addr, []netip.Addr, error) {
	var lastErr error

	for {
		cabecalho, err := resultado.p.AnswerHeader()
		if err != nil {
			if !errors.Is(err, dnsmessage.ErrSectionDone) {
				lastErr = erroRespostaDNSTun(host, resultado.server)
			}

			break
		}

		var terminal bool

		addrsV4, addrsV6, terminal, lastErr = acumularRespostaTipoDNSTun(&resultado, cabecalho, host, addrsV4, addrsV6)
		if terminal {
			break
		}
	}

	return addrsV4, addrsV6, lastErr
}

func ordenarEnderecosDNSTun(tnet *Net, addrsV4, addrsV6 []netip.Addr) []netip.Addr {
	// We don't do RFC6724. Instead just put V6 addresses first if an IPv6 address is enabled
	// Copia defensiva: append direto em addrsV6/addrsV4 reaproveitaria o
	// backing array delas e corromperia as listas acumuladas no laco.
	var addrs []netip.Addr
	if tnet.hasV6 {
		addrs = make([]netip.Addr, 0, len(addrsV6)+len(addrsV4))
		addrs = append(addrs, addrsV6...)
		addrs = append(addrs, addrsV4...)
	} else {
		addrs = make([]netip.Addr, 0, len(addrsV4)+len(addrsV6))
		addrs = append(addrs, addrsV4...)
		addrs = append(addrs, addrsV6...)
	}

	return addrs
}

// LookupContextHost resolve o host pelo DNS do tunel respeitando o contexto.
func (tnet *Net) LookupContextHost(ctx context.Context, host string) ([]string, error) {
	if direto, err := validarHostTun(tnet, host); err != nil {
		return nil, err
	} else if direto != "" {
		return []string{direto}, nil
	}

	var addrsV4, addrsV6 []netip.Addr

	const viasConsultaDNSTun = 2

	lane := make(chan resultadoDNSTun, viasConsultaDNSTun)

	lanes := dispararConsultasDNSTun(ctx, tnet, host, lane)

	var lastErr error

	for range lanes {
		resultado := <-lane
		if resultado.erro != nil {
			if lastErr == nil {
				lastErr = resultado.erro
			}

			continue
		}

		var err error

		addrsV4, addrsV6, err = coletarRespostaDNSTun(resultado, host, addrsV4, addrsV6)
		if err != nil {
			lastErr = err
		}
	}

	addrs := ordenarEnderecosDNSTun(tnet, addrsV4, addrsV6)

	if len(addrs) == 0 && lastErr != nil {
		return nil, lastErr
	}

	saddrs := make([]string, 0, len(addrs))
	for _, ip := range addrs {
		saddrs = append(saddrs, ip.String())
	}

	return saddrs, nil
}

func partialDeadline(now, deadline time.Time, addrsRemaining int) (time.Time, error) {
	if deadline.IsZero() {
		return deadline, nil
	}

	timeRemaining := deadline.Sub(now)
	if timeRemaining <= 0 {
		return time.Time{}, errTimeout
	}

	timeout := timeRemaining / time.Duration(addrsRemaining)

	const saneMinimum = 2 * time.Second
	if timeout < saneMinimum {
		timeout = min(timeRemaining, saneMinimum)
	}

	return now.Add(timeout), nil
}

var protoSplitter = regexp.MustCompile(`^(tcp|udp|ping)(4|6)?$`)

func interpretarRedeDialTun(network string) (string, bool, bool, error) {
	matches := protoSplitter.FindStringSubmatch(network)
	if matches == nil {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return "", false, false, &net.OpError{Op: opDial, Err: net.UnknownNetworkError(network)}
	}

	switch {
	case len(matches[2]) == 0:
		return matches[1], true, true, nil
	case matches[2][0] == '4':
		return matches[1], true, false, nil
	default:
		return matches[1], false, true, nil
	}
}

func separarAlvoDialTun(network, address string) (string, int, error) {
	if network == redePing {
		return address, 0, nil
	}

	host, sport, err := net.SplitHostPort(address)
	if err != nil {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return "", 0, &net.OpError{Op: opDial, Err: err}
	}

	port, err := strconv.Atoi(sport)
	if err != nil || port < 0 || port > 65535 {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return "", 0, &net.OpError{Op: opDial, Err: errNumericPort}
	}

	return host, port, nil
}

func filtrarEnderecosDialTun(allAddr []string, acceptV4, acceptV6 bool, port int) ([]netip.AddrPort, error) {
	var addrs []netip.AddrPort

	for _, addr := range allAddr {
		ip, err := netip.ParseAddr(addr)
		if err == nil && ((ip.Is4() && acceptV4) || (ip.Is6() && acceptV6)) {
			addrs = append(addrs, netip.AddrPortFrom(ip, uint16(port)))
		}
	}

	if len(addrs) == 0 && len(allAddr) != 0 {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return nil, &net.OpError{Op: opDial, Err: errNoSuitableAddress}
	}

	return addrs, nil
}

func prazoParcialDialTun(
	ctx context.Context, addrs []netip.AddrPort, indice int,
) (context.Context, context.CancelFunc, error) {
	dialCtx := ctx
	deadline, hasDeadline := ctx.Deadline()

	if !hasDeadline {
		return dialCtx, nil, nil
	}

	partialDeadline, err := partialDeadline(time.Now(), deadline, len(addrs)-indice)
	if err != nil {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return nil, nil, &net.OpError{Op: opDial, Err: err}
	}

	if !partialDeadline.Before(deadline) {
		return dialCtx, nil, nil
	}

	var cancel context.CancelFunc

	dialCtx, cancel = context.WithDeadline(ctx, partialDeadline)

	return dialCtx, cancel, nil
}

func verificarCancelamentoDialTun(ctx context.Context) error {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		if errors.Is(err, context.Canceled) {
			err = errCanceled
		} else if errors.Is(err, context.DeadlineExceeded) {
			err = errTimeout
		}

		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return &net.OpError{Op: opDial, Err: err}
	default:
		return nil
	}
}

func discarUmEnderecoDialTun(dialCtx context.Context, tnet *Net, proto string, addr netip.AddrPort) (net.Conn, error) {
	switch proto {
	case "tcp":
		return tnet.DialContextTCPAddrPort(dialCtx, addr)
	case "udp":
		return tnet.DialUDPAddrPort(netip.AddrPort{}, addr)
	case redePing:
		return tnet.DialPingAddr(netip.Addr{}, addr.Addr())
	default:
		return nil, erroTexto("rede desconhecida: " + proto)
	}
}

func discarEnderecosDialTun(ctx context.Context, tnet *Net, proto string,
	addrs []netip.AddrPort,
) (net.Conn, error) {
	var firstErr error

	for i, addr := range addrs {
		if err := verificarCancelamentoDialTun(ctx); err != nil {
			return nil, err
		}

		dialCtx, cancel, err := prazoParcialDialTun(ctx, addrs, i)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			break
		}

		if cancel != nil {
			defer cancel()
		}

		c, err := discarUmEnderecoDialTun(dialCtx, tnet, proto, addr)
		if err == nil {
			return c, nil
		}

		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr == nil {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		firstErr = &net.OpError{Op: opDial, Err: errMissingAddress}
	}

	return nil, firstErr
}

// DialContext disca pelo tunel respeitando o contexto.
func (tnet *Net) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx == nil {
		panic("nil context")
	}

	proto, acceptV4, acceptV6, err := interpretarRedeDialTun(network)
	if err != nil {
		return nil, err
	}

	host, port, err := separarAlvoDialTun(proto, address)
	if err != nil {
		return nil, err
	}

	allAddr, err := tnet.LookupContextHost(ctx, host)
	if err != nil {
		//exhaustruct:ignore // OpError: rede/origem preenchidos pelo chamador net; resto zero intencional.
		return nil, &net.OpError{Op: opDial, Err: err}
	}

	addrs, err := filtrarEnderecosDialTun(allAddr, acceptV4, acceptV6, port)
	if err != nil {
		return nil, err
	}

	return discarEnderecosDialTun(ctx, tnet, proto, addrs)
}

// Dial disca pelo tunel.
func (tnet *Net) Dial(network, address string) (net.Conn, error) {
	return tnet.DialContext(context.Background(), network, address)
}

func (tnet *Net) discarDNSTun(ctx context.Context, server netip.Addr, useUDP bool,
	timeout time.Duration,
) (net.Conn, context.CancelFunc, error) {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(timeout))

	const portaDNSTun = 53

	var (
		conexao net.Conn
		err     error
	)
	if useUDP {
		conexao, err = tnet.DialUDPAddrPort(netip.AddrPort{}, netip.AddrPortFrom(server, portaDNSTun))
	} else {
		conexao, err = tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(server, portaDNSTun))
	}

	if err != nil {
		cancel()

		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return nil, nil, err
	}

	return conexao, cancel, nil
}

func (tnet *Net) exchange(ctx context.Context, server netip.Addr, pergunta dnsmessage.Question,
	timeout time.Duration,
) (dnsmessage.Parser, dnsmessage.Header, error) {
	pergunta.Class = dnsmessage.ClassINET

	identificador, udpReq, tcpReq, err := newRequest(pergunta)
	if err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, errCannotMarshalDNSMessage
	}

	for _, useUDP := range []bool{true, false} {
		conexao, cancel, err := tnet.discarDNSTun(ctx, server, useUDP, timeout)
		if err != nil {
			//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
			return dnsmessage.Parser{}, dnsmessage.Header{}, err
		}

		leitor, cabecalho, err := tnet.trocarComPrazoDNSTun(
			ctx, conexao, cancel, identificador, pergunta, udpReq, tcpReq, useUDP)

		conexao.Close()

		if err != nil {
			//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
			return dnsmessage.Parser{}, dnsmessage.Header{}, err
		}

		if cabecalho.Truncated {
			continue
		}

		return leitor, cabecalho, nil
	}

	//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
	return dnsmessage.Parser{}, dnsmessage.Header{}, errNoAnswerFromDNSServer
}

func (tnet *Net) trocarComPrazoDNSTun(ctx context.Context, conexao net.Conn, cancel context.CancelFunc, ident uint16,
	pergunta dnsmessage.Question, udpReq, tcpReq []byte, useUDP bool,
) (dnsmessage.Parser, dnsmessage.Header, error) {
	defer cancel()

	if err := aplicarPrazoDNSTun(ctx, conexao); err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, err
	}

	leitor, cabecalho, err := executarIdaVoltaDNSTun(conexao, ident, pergunta, udpReq, tcpReq, useUDP)
	if err != nil {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, traduzirErroDNSTun(err)
	}

	if err := leitor.SkipQuestion(); !errors.Is(err, dnsmessage.ErrSectionDone) {
		//exhaustruct:ignore // Parser/Header vazios em caminho de erro.
		return dnsmessage.Parser{}, dnsmessage.Header{}, errInvalidDNSResponse
	}

	return leitor, cabecalho, nil
}

func (tnet *Net) tryOneName(ctx context.Context, name string,
	qtype dnsmessage.Type,
) (dnsmessage.Parser, string, error) {
	var lastErr error

	nomeDNS, err := dnsmessage.NewName(name)
	if err != nil {
		return dnsmessage.Parser{}, "", errCannotMarshalDNSMessage
	}

	pergunta := dnsmessage.Question{
		Name:  nomeDNS,
		Type:  qtype,
		Class: dnsmessage.ClassINET,
	}

	for range 2 {
		leitor, servidor, concluido, err := tentarServidoresDNSTun(ctx, tnet, pergunta, qtype, name)
		if concluido {
			return leitor, servidor, err
		}

		if err != nil {
			lastErr = err
		}
	}

	return dnsmessage.Parser{}, "", lastErr
}
