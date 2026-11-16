//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unsafe"
)

const (
	prefixoAlvoSenha = "Overdrive/senha/"
	maxUsuarioSenha  = 256
	maxSegredoSenha  = 1024
	maxDominioSenha  = 253
)

// credencialSenha e o conteudo do cofre por dominio.
type credencialSenha struct {
	Usuario string `json:"u"`
	Segredo string `json:"p"`
}

// senhaSalva e uma entrada para a lista e o menu.
type senhaSalva struct {
	Dominio string
	Usuario string
}

// msgSenhas e o envelope do senhas-lite.js.
type msgSenhas struct {
	Acao    string `json:"acao"`
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

// alvoSenha monta o alvo no cofre para o dominio.
func alvoSenha(dominio string) (string, bool) {
	dominio = strings.ToLower(strings.TrimSpace(dominio))
	if dominio == "" || dominio == "." || dominio == ".." {
		return "", false
	}

	if len(dominio) > maxDominioSenha || strings.ContainsAny(dominio, "/\\") {
		return "", false
	}

	return prefixoAlvoSenha + dominio, true
}

// dominioAlvoSenha extrai o dominio de um alvo do cofre.
func dominioAlvoSenha(alvo string) (string, bool) {
	dominio, ok := strings.CutPrefix(alvo, prefixoAlvoSenha)
	if !ok {
		return "", false
	}

	if _, valida := alvoSenha(dominio); !valida {
		return "", false
	}

	return dominio, true
}

// senhaValida diz se o par pode ser guardado (usuario pode ser vazio).
func senhaValida(usuario, senha string) bool {
	if len(usuario) > maxUsuarioSenha || len(senha) == 0 || len(senha) > maxSegredoSenha {
		return false
	}

	return true
}

// blobSenha serializa a credencial para o cofre.
func blobSenha(usuario, senha string) string {
	dados, err := json.Marshal(credencialSenha{Usuario: usuario, Segredo: senha})
	if err != nil {
		return ""
	}

	return string(dados)
}

// abrirBlobSenha le a credencial do cofre.
func abrirBlobSenha(blob string) (credencialSenha, bool) {
	//exhaustruct:ignore // Credencial vazia: zero-value intencional no erro.
	vazia := credencialSenha{}

	var cred credencialSenha
	if blob == "" || json.Unmarshal([]byte(blob), &cred) != nil {
		return vazia, false
	}

	if cred.Segredo == "" {
		return vazia, false
	}

	return cred, true
}

// dominioPaneSenha resolve o dominio da pagina do painel.
func (a *app) dominioPaneSenha(p *pane) (string, bool) {
	if p == nil || !p.ready || p.wv == nil {
		return "", false
	}

	endereco := strings.TrimSpace(p.wv.Source())
	if endereco == "" || strings.HasPrefix(endereco, "about:") {
		return "", false
	}

	host := dominioDeURL(endereco)
	if host == "" {
		return "", false
	}

	if _, valida := alvoSenha(host); !valida {
		return "", false
	}

	return host, true
}

// lerSenhaDominio le a credencial do dominio.
func (a *app) lerSenhaDominio(dominio string) (credencialSenha, bool) {
	//exhaustruct:ignore // Credencial vazia: zero-value intencional no erro.
	vazia := credencialSenha{}

	alvo, temAlvo := alvoSenha(dominio)
	if !temAlvo {
		return vazia, false
	}

	blob, achou := credLer(alvo)
	if !achou {
		return vazia, false
	}

	return abrirBlobSenha(blob)
}

// confirmarSenha pergunta sim/nao com caixa de dialogo.
func (a *app) confirmarSenha(pergunta, titulo string) bool {
	if a == nil || a.hwnd == 0 {
		return false
	}

	const (
		mbYesNo        = 0x04
		mbIconPergunta = 0x20
		idYes          = 6
	)

	r, _, _ := pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(utf16(pergunta))),
		uintptr(unsafe.Pointer(utf16(titulo))), mbYesNo|mbIconPergunta)

	return int32(r) == idYes
}

// confirmarGuardarSenha pergunta salvar ou atualizar; false = desiste.
func (a *app) confirmarGuardarSenha(dominio, usuario string, tem bool) bool {
	if tem {
		return a.confirmarSenha(
			formatoPerguntaSenha(txtFmtSenhaAtualizar, dominio, usuario),
			txtTituloSenhas)
	}

	return a.confirmarSenha(
		formatoPerguntaSenha(txtFmtSenhaSalvar, dominio, usuario),
		txtTituloSenhas)
}

// chaveSenhaNegada monta a chave do "nao perguntar" da sessao.
func chaveSenhaNegada(dominio, usuario, senha string) string {
	return dominio + "\x00" + usuario + "\x00" + senha
}

// senhaJaTratada diz se nao ha o que perguntar: negada ou identica.
func (a *app) senhaJaTratada(dominio, usuario, senha string, atual credencialSenha, tem bool) bool {
	if a.senhasNegadas[chaveSenhaNegada(dominio, usuario, senha)] {
		return true
	}

	return tem && atual.Usuario == usuario && atual.Segredo == senha
}

// lembrarSenhaNegada marca o trio para nao perguntar de novo na sessao.
func (a *app) lembrarSenhaNegada(dominio, usuario, senha string) {
	if a.senhasNegadas == nil {
		a.senhasNegadas = map[string]bool{}
	}

	a.senhasNegadas[chaveSenhaNegada(dominio, usuario, senha)] = true
}

// salvarSenhaOuvida guarda ou atualiza a senha capturada no submit.
func (a *app) salvarSenhaOuvida(p *pane, usuario, senha string) {
	usuario = strings.TrimSpace(usuario)
	if !senhaValida(usuario, senha) {
		return
	}

	dominio, valido := a.dominioPaneSenha(p)
	if !valido {
		return
	}

	atual, tem := a.lerSenhaDominio(dominio)
	if a.senhaJaTratada(dominio, usuario, senha, atual, tem) {
		return
	}

	if !a.confirmarGuardarSenha(dominio, usuario, tem) {
		a.lembrarSenhaNegada(dominio, usuario, senha)

		return
	}

	alvo, ok := alvoSenha(dominio)
	if !ok {
		return
	}

	blob := blobSenha(usuario, senha)
	if blob == "" {
		a.dizerErrof(txtFmtSenhaNaoSalvei, dominio)

		return
	}

	if err := credEscrever(alvo, blob); err != nil {
		a.dizerErrof(txtFmtSenhaNaoSalvei, dominio)

		return
	}

	a.dizerOkf(txtFmtSenhaSalva, dominio)
	a.invalidateBar()
}

// formatoPerguntaSenha monta a pergunta com ou sem usuario.
func formatoPerguntaSenha(formato, dominio, usuario string) string {
	if usuario == "" {
		return fmt.Sprintf(formato, dominio)
	}

	return fmt.Sprintf(formato, dominio+" ("+usuario+")")
}

// scriptPreencherSenha monta o JS que preenche usuario e senha.
func scriptPreencherSenha(usuario, senha string) string {
	return `(function(u,p){try{
var pw=document.querySelector('input[type="password"]');
if(!pw||pw.disabled)return 'sem-campo';
var us=null;
var escopo=pw.form||document;
var cs=escopo.querySelectorAll('input[type="email"],input[type="text"],input:not([type])');
for(var i=0;i<cs.length;i++){var c=cs[i];if(c&&!c.disabled&&c!==pw){us=c;break;}}
if(us){us.focus();us.value=u;` +
		`us.dispatchEvent(new Event('input',{bubbles:true}));us.dispatchEvent(new Event('change',{bubbles:true}));}` +
		`pw.focus();pw.value=p;pw.dispatchEvent(new Event('input',{bubbles:true}));` +
		`pw.dispatchEvent(new Event('change',{bubbles:true}));
return 'ok';}catch(e){return 'erro';}})(` + jsonDeString(usuario) + `,` + jsonDeString(senha) + `)`
}

// preencherSenhaDominio preenche o login do dominio no painel atual.
func (a *app) preencherSenhaDominio(dominio string) {
	p := a.left
	if p == nil || !p.ready || p.wv == nil {
		a.dizerf(txtSemPagPronta)

		return
	}

	cred, ok := a.lerSenhaDominio(dominio)
	if !ok {
		a.dizerf(txtFmtSenhaNaoAchada, dominio)

		return
	}

	p.wv.ExecuteScript(scriptPreencherSenha(cred.Usuario, cred.Segredo))
	a.dizerOkf(txtFmtSenhaPreenchida, dominio)
	a.invalidateBar()
}

// oferecerSenhaTemCampo preenche sozinho ou avisa, conforme o modo.
func (a *app) oferecerSenhaTemCampo(p *pane) {
	if p == nil || p != a.left {
		return
	}

	dominio, ok := a.dominioPaneSenha(p)
	if !ok {
		return
	}

	if _, tem := a.lerSenhaDominio(dominio); !tem {
		return
	}

	if a.st.SenhasAuto {
		a.preencherSenhaDominio(dominio)

		return
	}

	a.dizerf(txtFmtSenhaDisponivel, dominio)
	a.invalidateBar()
}

// cuidarMsgSenhas trata salvar e temcampo vindos da pagina.
func (a *app) cuidarMsgSenhas(p *pane, msg string) {
	var pedido msgSenhas
	if json.Unmarshal([]byte(msg), &pedido) != nil {
		return
	}

	switch pedido.Acao {
	case "salvar":
		a.depois(func() { a.salvarSenhaOuvida(p, pedido.Usuario, pedido.Senha) })
	case "temcampo":
		a.depois(func() { a.oferecerSenhaTemCampo(p) })
	}
}

// listaSenhas devolve as entradas do cofre ordenadas por dominio.
func (a *app) listaSenhas() []senhaSalva {
	var out []senhaSalva

	for _, alvo := range credListar(prefixoAlvoSenha) {
		dominio, ok := dominioAlvoSenha(alvo)
		if !ok {
			continue
		}

		//exhaustruct:ignore // Entrada parcial: usuario preenchido abaixo quando houver.
		item := senhaSalva{Dominio: dominio}

		if blob, achou := credLer(alvo); achou {
			if cred, valida := abrirBlobSenha(blob); valida {
				item.Usuario = cred.Usuario
			}
		}

		out = append(out, item)
	}

	return out
}

// apagarSenhaSalva remove a entrada do dominio.
func (a *app) apagarSenhaSalva(dominio string) {
	alvo, ok := alvoSenha(dominio)
	if !ok {
		a.dizerErrof(txtFmtSenhaNaoAchada, dominio)

		return
	}

	if err := credApagar(alvo); err != nil {
		a.dizerErrof(txtFmtSenhaNaoApaguei, dominio)

		return
	}

	a.dizerOkf(txtFmtSenhaApagada, dominio)
	a.invalidateBar()
}

// copiarSegredoSenha copia a senha do dominio.
func (a *app) copiarSegredoSenha(dominio string) {
	cred, ok := a.lerSenhaDominio(dominio)
	if !ok {
		a.dizerf(txtFmtSenhaNaoAchada, dominio)

		return
	}

	if copiarTexto(cred.Segredo) {
		a.dizerOkf(txtFmtSenhaCopiada, dominio)
	} else {
		a.dizerErrof(txtNaoCopiouSenha)
	}

	a.invalidateBar()
}

// copiarUsuarioSenha copia o usuario do dominio.
func (a *app) copiarUsuarioSenha(dominio string) {
	cred, ok := a.lerSenhaDominio(dominio)
	if !ok || cred.Usuario == "" {
		a.dizerf(txtFmtSenhaNaoAchada, dominio)

		return
	}

	if copiarTexto(cred.Usuario) {
		a.dizerOkf(txtFmtUsuarioCopiado, dominio)
	} else {
		a.dizerErrof(txtNaoCopiouSenha)
	}

	a.invalidateBar()
}

// alternarSenhasAuto liga/desliga o preenchimento automatico.
func (a *app) alternarSenhasAuto() {
	a.st.SenhasAuto = !a.st.SenhasAuto
	a.persist()

	if a.st.SenhasAuto {
		a.dizerOkf(txtSenhasAutoLigado)
	} else {
		a.dizerOkf(txtSenhasAutoDesligado)
	}

	a.invalidateBar()
}

// itemSenhaDominio monta o submenu de um dominio.
func itemSenhaDominio(item senhaSalva) menuItem {
	rotulo := item.Dominio
	if item.Usuario != "" {
		rotulo += " (" + item.Usuario + ")"
	}

	return menuItem{kind: miSub, label: rotulo, sub: []menuItem{
		{kind: miItem, label: txtSenhasPreencher, id: "senhas:preencher:" + item.Dominio},
		{kind: miItem, label: txtSenhasCopiarUsuario, id: "senhas:copiaru:" + item.Dominio},
		{kind: miItem, label: txtSenhasCopiarSenha, id: "senhas:copiars:" + item.Dominio},
		{kind: miItem, label: txtSenhasApagar, id: "senhas:apagar:" + item.Dominio},
	}}
}

// abrirMenuSenhas mostra as senhas salvas com acoes por dominio.
func (a *app) abrirMenuSenhas() {
	if menu.open && a.menuOrigem == idSenhas {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	a.contextAba = -1
	a.menuOrigem = idSenhas
	menu.items = []menuItem{{kind: miHeader, label: txtHdrSenhas}}
	menu.items = append(menu.items,
		menuItem{kind: miCheck, label: txtSenhasAuto, id: "senhasauto", check: a.st.SenhasAuto})
	menu.items = append(menu.items, menuItem{kind: miSep})

	itens := a.listaSenhas()
	if len(itens) == 0 {
		menu.items = append(menu.items, menuItem{kind: miItem, label: txtSenhasVazio, id: ""})
	}

	for _, item := range itens {
		menu.items = append(menu.items, itemSenhaDominio(item))
	}

	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	if menuSub.hwnd == 0 && !a.createSubMenuWindow() {
		return
	}

	a.abrirMenuAncorado(a.toolsBtn.rect)
}

// dominioIDSenha extrai o dominio do id de acao.
func dominioIDSenha(id, prefixo string) (string, bool) {
	dominio, ok := strings.CutPrefix(id, prefixo)
	if !ok {
		return "", false
	}

	if _, valida := alvoSenha(dominio); !valida {
		return "", false
	}

	return dominio, true
}

// doActionSenhas trata senhas e acoes por dominio; true = tratado.
func (a *app) doActionSenhas(id string) bool {
	switch id {
	case idSenhas:
		a.abrirMenuSenhas()

		return true
	case "senhasauto":
		a.alternarSenhasAuto()

		return true
	}

	if dominio, ok := dominioIDSenha(id, "senhas:preencher:"); ok {
		a.preencherSenhaDominio(dominio)

		return true
	}

	if dominio, ok := dominioIDSenha(id, "senhas:copiaru:"); ok {
		a.copiarUsuarioSenha(dominio)

		return true
	}

	if dominio, ok := dominioIDSenha(id, "senhas:copiars:"); ok {
		a.copiarSegredoSenha(dominio)

		return true
	}

	if dominio, ok := dominioIDSenha(id, "senhas:apagar:"); ok {
		if a.confirmarSenha(
			"apagar a senha salva de "+dominio+"?", txtTituloSenhas) {
			a.apagarSenhaSalva(dominio)
		}

		return true
	}

	return false
}
