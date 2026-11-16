package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	wgnetstack "overdrive/wgnetstack"
)

//exhaustruct:ignore // Gerente de VPN: construtor parcial por desenho (campos sob mutex).
type vpnManager struct {
	mu         sync.RWMutex
	cfg        *vpnConfig
	configText string
	dev        *device.Device
	tun        *wgnetstack.Net
	endpoint   string
	enabled    bool
	connecting bool
	connected  bool
	killSwitch bool
	lastErr    string
	proxy      *vpnProxy
	geracao    uint64
}

func newVPNManager() *vpnManager {
	gerente := &vpnManager{killSwitch: true}

	proxy, err := startVPNProxy(gerente)
	if err != nil {
		gerente.lastErr = fmt.Sprintf(txtFmtProxyLocal, err.Error())

		return gerente
	}

	gerente.proxy = proxy

	return gerente
}

func (m *vpnManager) Connect(text string) error {
	cfg, gen, antigo, err := m.prepararConexao(text)
	if err != nil {
		return err
	}

	if antigo != nil {
		antigo.Close()
	}

	tunDev, tun, err := m.criarTunVPN(cfg, gen)
	if err != nil {
		return err
	}

	dev, err := m.subirTunVPN(cfg, tunDev, gen)
	if err != nil {
		return err
	}

	return m.confirmarConexao(gen, dev, tun)
}

func (m *vpnManager) Disconnect() {
	m.mu.Lock()
	m.geracao++
	dev := m.dev
	m.dev = nil
	m.tun = nil
	m.enabled = false
	m.connected = false
	m.connecting = false
	m.endpoint = ""
	m.lastErr = ""
	m.mu.Unlock()

	if dev != nil {
		dev.Close()
	}
}

func (m *vpnManager) Close() {
	m.Disconnect()

	if m.proxy != nil {
		m.proxy.Close()
	}
}

func (m *vpnManager) SetKillSwitch(on bool) {
	m.mu.Lock()
	m.killSwitch = on
	m.mu.Unlock()
}

// prepararConexao valida o texto e reserva a geracao atual da VPN.
func (m *vpnManager) prepararConexao(text string) (*vpnConfig, uint64, *device.Device, error) {
	text = strings.TrimSpace(text)

	m.mu.RLock()

	if text == "" {
		text = m.configText
	}

	antigo := m.dev
	m.mu.RUnlock()

	cfg, err := parseWireGuardConfig(text)
	if err != nil {
		m.setErro(err.Error())

		return nil, 0, nil, err
	}

	m.mu.Lock()
	m.geracao++
	gen := m.geracao
	m.enabled = true
	m.connecting = true
	m.connected = false
	m.lastErr = ""
	m.cfg = cfg
	m.configText = text
	m.endpoint = cfg.Endpoint
	m.dev = nil
	m.tun = nil
	m.mu.Unlock()

	return cfg, gen, antigo, nil
}

// subirTunVPN cria o device WireGuard e aplica a config via UAPI.
func (m *vpnManager) subirTunVPN(cfg *vpnConfig, tunDev tun.Device, gen uint64) (*device.Device, error) {
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	if err := dev.IpcSet(cfg.uapi()); err != nil {
		dev.Close()
		m.setErroGeracao(gen, fmt.Sprintf(txtFmtTunelCfg, err.Error()))

		return nil, erroTextoF("tunel: %w", err)
	}

	if err := dev.Up(); err != nil {
		dev.Close()
		m.setErroGeracao(gen, fmt.Sprintf(txtFmtTunelUp, err.Error()))

		return nil, erroTextoF("tunel: %w", err)
	}

	return dev, nil
}

// confirmarConexao publica o device ativo se a geracao ainda for a atual.
func (m *vpnManager) confirmarConexao(gen uint64, dev *device.Device, tun *wgnetstack.Net) error {
	m.mu.Lock()
	if m.geracao != gen || !m.enabled {
		m.mu.Unlock()
		dev.Close()

		return erroTexto(txtVPNCancelada)
	}

	m.dev = dev
	m.tun = tun
	m.connected = true
	m.connecting = false
	m.mu.Unlock()

	return nil
}

// criarTunVPN cria a interface TUN do WireGuard para a geracao atual.
//
//nolint:ireturn // Assinatura imposta pelo wireguard: device.NewDevice exige tun.Device.
func (m *vpnManager) criarTunVPN(cfg *vpnConfig, gen uint64) (tun.Device, *wgnetstack.Net, error) {
	tunDev, tun, err := wgnetstack.CreateNetTUN(cfg.Addresses, cfg.DNS, cfg.MTU)
	if err != nil {
		m.setErroGeracao(gen, fmt.Sprintf(txtFmtTunelErr, err.Error()))

		return nil, nil, erroTextoF("tunel: %w", err)
	}

	return tunDev, tun, nil
}

func (m *vpnManager) proxyAddr() string {
	if m == nil || m.proxy == nil {
		return ""
	}

	return m.proxy.addr
}

func (m *vpnManager) setErro(msg string) {
	m.mu.Lock()
	m.lastErr = msg
	m.connecting = false
	m.mu.Unlock()
}

// setErroGeracao so registra a falha se ela ainda pertence a tentativa atual;
// uma conexao cancelada por Disconnect (ou por outra tentativa) nao pode
// sobrescrever o estado novo.
func (m *vpnManager) setErroGeracao(gen uint64, msg string) {
	m.mu.Lock()
	if m.geracao == gen && m.connecting {
		m.lastErr = msg
		m.connecting = false
	}
	m.mu.Unlock()
}

// avisarDireto registra que o tunel caiu e o trafego esta saindo direto
// porque o kill switch esta desligado. Sem isso a queda e silenciosa e o
// usuario continua achando que esta no tunel.
func (m *vpnManager) avisarDireto() {
	m.mu.Lock()
	m.lastErr = txtTunelDireto
	m.mu.Unlock()
}

func (m *vpnManager) textoConfig() string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.configText
}

func (m *vpnManager) dial(ctx context.Context, hostport string) (net.Conn, error) {
	host, portStr, port, err := validarAlvoDiscagem(hostport)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	tun := m.tun
	enabled := m.enabled
	kill := m.killSwitch
	m.mu.RUnlock()

	if tun != nil {
		return discarPeloTunel(ctx, tun, host, port)
	}

	return m.discarSemTunel(ctx, host, portStr, enabled, kill)
}

// validarAlvoDiscagem separa host e porta e valida a faixa da porta.
func validarAlvoDiscagem(hostport string) (string, string, int, error) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", "", 0, erroTextoF("alvo invalido: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", "", 0, erroTexto("porta invalida")
	}

	return host, portStr, port, nil
}

// escolherIPNoTunel resolve o host para um IP usando o DNS do tunel.
func escolherIPNoTunel(ctx context.Context, tun *wgnetstack.Net, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}

	addrs, err := tun.LookupContextHost(ctx, host)
	if err != nil || len(addrs) == 0 {
		if err == nil {
			err = erroTexto("DNS nao retornou endereco")
		}

		return nil, err
	}

	return preferirIPv4(addrs)
}

// preferirIPv4 escolhe o primeiro IPv4 da lista, ou o primeiro valido.
func preferirIPv4(addrs []string) (net.IP, error) {
	var primeiro net.IP

	for _, a := range addrs {
		parsed := net.ParseIP(a)
		if parsed == nil {
			continue
		}

		if primeiro == nil {
			primeiro = parsed
		}

		if parsed.To4() != nil {
			return parsed, nil
		}
	}

	if primeiro == nil {
		return nil, erroTexto("endereco invalido")
	}

	return primeiro, nil
}

// discarPeloTunel disca host:port atraves da interface do tunel.
func discarPeloTunel(ctx context.Context, tun *wgnetstack.Net, host string, port int) (net.Conn, error) {
	enderecoIP, err := escolherIPNoTunel(ctx, tun, host)
	if err != nil {
		return nil, err
	}

	if ipProibido(enderecoIP) {
		return nil, erroTexto("destino local bloqueado")
	}

	//exhaustruct:ignore // TCPAddr: IP + porta; zona so para IPv6 link-local.
	conexao, err := tun.DialContextTCP(ctx, &net.TCPAddr{IP: enderecoIP, Port: port})
	if err != nil {
		return nil, erroTextoF("discar: %w", err)
	}
	// Defesa: o gVisor pode devolver (nil, nil) em algum caminho;
	// repassar nulo faria o relay explodir no Close.
	if conexao == nil {
		return nil, erroTexto("tunel devolveu conexao nula")
	}

	return conexao, nil
}

// discarSemTunel disca direto quando nao ha tunel ativo.
func (m *vpnManager) discarSemTunel(ctx context.Context, host, portStr string, enabled, kill bool) (net.Conn, error) {
	if enabled && kill {
		return nil, erroTexto(txtVPNSemTunel)
	}

	m.avisarDireto()

	ips, err := resolverDireto(ctx, host)
	if err != nil {
		return nil, err
	}

	return discarPrimeiroPermitido(ctx, ips, portStr)
}

// discarPrimeiroPermitido tenta cada IP ate o primeiro dial com sucesso.
func discarPrimeiroPermitido(ctx context.Context, ips []net.IP, portStr string) (net.Conn, error) {
	var (
		discador  net.Dialer
		ultimo    error
		permitido bool
	)

	for _, enderecoIP := range ips {
		if ipProibido(enderecoIP) {
			continue
		}

		permitido = true

		c, err := discador.DialContext(ctx, "tcp", net.JoinHostPort(enderecoIP.String(), portStr))
		if err == nil {
			return c, nil
		}

		ultimo = err
	}

	if !permitido {
		return nil, erroTexto("destino local bloqueado")
	}

	if ultimo == nil {
		ultimo = erroTexto("sem endereco para conectar")
	}

	return nil, ultimo
}

func (a *app) vpnAtiva() bool {
	if a == nil || a.vpn == nil {
		return false
	}

	a.vpn.mu.RLock()
	defer a.vpn.mu.RUnlock()

	return a.vpn.connected || a.vpn.connecting
}

func (a *app) vpnConectar(texto string) {
	if a.vpn == nil {
		a.dizerf(txtVPNIndisp)

		return
	}

	if err := a.vpn.Connect(texto); err != nil {
		a.dizerf(txtFmtVPNErr, err)
		a.layout()

		return
	}

	a.dizerf(txtVPNLigada)
	a.layout()
}

func (a *app) vpnDesconectar() {
	if a.vpn == nil {
		return
	}

	a.vpn.Disconnect()
	a.dizerf(txtVPNDesligada)
	a.layout()
}

func (a *app) vpnKillSwitch(ligado bool) {
	if a.vpn == nil {
		return
	}

	a.vpn.SetKillSwitch(ligado)
}
