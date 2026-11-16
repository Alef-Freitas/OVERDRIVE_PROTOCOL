package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

//exhaustruct:ignore // Config de VPN: construtor parcial por desenho.
type vpnConfig struct {
	PrivateKey       string
	Addresses        []netip.Addr
	DNS              []netip.Addr
	MTU              int
	ListenPort       int
	PeerPublicKey    string
	PeerPresharedKey string
	Endpoint         string
	AllowedIPs       []string
	Keepalive        int
}

func wgKeyToHex(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != 32 {
		return "", erroTexto("chave WireGuard invalida")
	}

	return hex.EncodeToString(b), nil
}

func parseWireGuardConfig(text string) (*vpnConfig, error) {
	const mtuPadraoVPN = 1420

	cfg := &vpnConfig{MTU: mtuPadraoVPN}
	if err := aplicarLinhasVPN(cfg, text); err != nil {
		return nil, err
	}

	if err := validarVPNConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// aplicarLinhasVPN percorre o texto da config e preenche cfg.
func aplicarLinhasVPN(cfg *vpnConfig, text string) error {
	secao := ""

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		linha, ok := limparLinhaVPN(scanner.Text())
		if !ok {
			continue
		}

		if nova, ehSecao := detectarSecaoVPN(linha); ehSecao {
			secao = nova

			continue
		}

		if err := aplicarChaveVPN(cfg, secao, linha); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return erroTextoF("ler conf: %w", err)
	}

	return nil
}

// limparLinhaVPN remove espacos e descarta vazias e comentarios.
func limparLinhaVPN(bruta string) (string, bool) {
	linha := strings.TrimSpace(bruta)
	if linha == "" || strings.HasPrefix(linha, "#") || strings.HasPrefix(linha, ";") {
		return "", false
	}

	return linha, true
}

// detectarSecaoVPN reconhece cabecalhos como [Interface] e [Peer].
func detectarSecaoVPN(linha string) (string, bool) {
	if !strings.HasPrefix(linha, "[") || !strings.HasSuffix(linha, "]") {
		return "", false
	}

	return strings.ToLower(strings.TrimSpace(strings.Trim(linha, "[]"))), true
}

// aplicarChaveVPN interpreta uma linha chave=valor dentro da secao atual.
func aplicarChaveVPN(cfg *vpnConfig, secao, linha string) error {
	chave, valor, ok := strings.Cut(linha, "=")
	if !ok {
		return nil
	}

	chave = strings.ToLower(strings.TrimSpace(chave))

	valor = strings.TrimSpace(valor)
	if valor == "" {
		return nil
	}

	switch secao {
	case "interface":
		return aplicarChaveInterface(cfg, chave, valor)
	case "peer":
		return aplicarChavePeer(cfg, chave, valor)
	}

	return nil
}

// aplicarChaveInterface interpreta chaves da secao [Interface].
func aplicarChaveInterface(cfg *vpnConfig, chave, valor string) error {
	switch chave {
	case "privatekey":
		return aplicarChavePrivada(cfg, valor)
	case "address":
		return anexarEnderecosVPN(cfg, valor)
	case "dns":
		return anexarDNSVPN(cfg, valor)
	}

	aplicarNumericoInterface(cfg, chave, valor)

	return nil
}

// aplicarChavePrivada guarda a chave privada em hexadecimal.
func aplicarChavePrivada(cfg *vpnConfig, v string) error {
	h, err := wgKeyToHex(v)
	if err != nil {
		return err
	}

	cfg.PrivateKey = h

	return nil
}

// aplicarNumericoInterface interpreta mtu e listenport.
func aplicarNumericoInterface(cfg *vpnConfig, chave, valor string) {
	switch chave {
	case "mtu":
		if num, err := strconv.Atoi(valor); err == nil && num >= 576 && num <= 9000 {
			cfg.MTU = num
		}
	case "listenport":
		if num, err := strconv.Atoi(valor); err == nil && num >= 0 && num <= 65535 {
			cfg.ListenPort = num
		}
	}
}

// aplicarChavePeer interpreta chaves da secao [Peer].
func aplicarChavePeer(cfg *vpnConfig, chave, valor string) error {
	switch chave {
	case "publickey":
		return aplicarChavePublica(cfg, valor)
	case "presharedkey":
		return aplicarChavePreCompartilhada(cfg, valor)
	case "endpoint":
		cfg.Endpoint = valor
	case "allowedips":
		return anexarAllowedIPsVPN(cfg, valor)
	}

	aplicarKeepalivePeer(cfg, chave, valor)

	return nil
}

// aplicarChavePublica guarda a chave publica do peer.
func aplicarChavePublica(cfg *vpnConfig, v string) error {
	h, err := wgKeyToHex(v)
	if err != nil {
		return err
	}

	cfg.PeerPublicKey = h

	return nil
}

// aplicarChavePreCompartilhada guarda a chave pre-compartilhada.
func aplicarChavePreCompartilhada(cfg *vpnConfig, v string) error {
	h, err := wgKeyToHex(v)
	if err != nil {
		return err
	}

	cfg.PeerPresharedKey = h

	return nil
}

// aplicarKeepalivePeer interpreta o persistentkeepalive.
func aplicarKeepalivePeer(cfg *vpnConfig, k, v string) {
	if k != "persistentkeepalive" {
		return
	}

	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		cfg.Keepalive = n
	}
}

// anexarEnderecosVPN adiciona os enderecos da Interface a cfg.
func anexarEnderecosVPN(cfg *vpnConfig, v string) error {
	for item := range strings.SplitSeq(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			p, err := netip.ParsePrefix(item)
			if err != nil {
				return erroTextoF("campo Address invalido: %s", item)
			}

			cfg.Addresses = append(cfg.Addresses, p.Addr())
		} else {
			a, err := netip.ParseAddr(item)
			if err != nil {
				return erroTextoF("campo Address invalido: %s", item)
			}

			cfg.Addresses = append(cfg.Addresses, a)
		}
	}

	return nil
}

// anexarDNSVPN adiciona os servidores DNS a cfg.
func anexarDNSVPN(cfg *vpnConfig, v string) error {
	for item := range strings.SplitSeq(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		a, err := netip.ParseAddr(item)
		if err != nil {
			return erroTextoF("DNS invalido: %s", item)
		}

		cfg.DNS = append(cfg.DNS, a)
	}

	return nil
}

// anexarAllowedIPsVPN adiciona as rotas do peer a cfg.
func anexarAllowedIPsVPN(cfg *vpnConfig, v string) error {
	for item := range strings.SplitSeq(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if _, err := netip.ParsePrefix(item); err != nil {
			return erroTextoF("AllowedIPs invalido: %s", item)
		}

		cfg.AllowedIPs = append(cfg.AllowedIPs, item)
	}

	return nil
}

// validarVPNConfig confere os campos obrigatorios da config.
func validarVPNConfig(cfg *vpnConfig) error {
	if cfg.PrivateKey == "" {
		return erroTexto("config sem PrivateKey")
	}

	if cfg.PeerPublicKey == "" {
		return erroTexto("config sem PublicKey do peer")
	}

	if len(cfg.Addresses) == 0 {
		return erroTexto("config sem Address")
	}

	if len(cfg.DNS) == 0 {
		return erroTexto(txtVPNSemDNS)
	}

	if cfg.Endpoint == "" {
		return erroTexto("config sem Endpoint")
	}

	if len(cfg.AllowedIPs) == 0 {
		return erroTexto("config sem AllowedIPs")
	}

	return nil
}

func (c *vpnConfig) uapi() string {
	var montador strings.Builder
	fmt.Fprintf(&montador, "private_key=%s\n", c.PrivateKey)

	if c.ListenPort > 0 {
		fmt.Fprintf(&montador, "listen_port=%d\n", c.ListenPort)
	}

	montador.WriteString("replace_peers=true\n")
	fmt.Fprintf(&montador, "public_key=%s\n", c.PeerPublicKey)

	if c.PeerPresharedKey != "" {
		fmt.Fprintf(&montador, "preshared_key=%s\n", c.PeerPresharedKey)
	}

	if c.Endpoint != "" {
		fmt.Fprintf(&montador, "endpoint=%s\n", c.Endpoint)
	}

	if c.Keepalive > 0 {
		fmt.Fprintf(&montador, "persistent_keepalive_interval=%d\n", c.Keepalive)
	}

	for _, ip := range c.AllowedIPs {
		fmt.Fprintf(&montador, "allowed_ip=%s\n", ip)
	}

	return montador.String()
}
