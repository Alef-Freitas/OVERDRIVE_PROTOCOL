//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

const maxExtFixadas = 4

const (
	bytesPorKBExt      = 1024
	tamanhoMinimoIcone = 12
	maxPermsExt        = 12
)

//odlint:somente-leitura conjunto fixo em tempo de compilacao.
//nolint:gochecknoglobals // Extensoes embutidas: somente leitura.
var extensoesSistema = map[string]bool{
	idUbo: true,
}

func ehExtensaoSistema(base string) bool {
	return extensoesSistema[strings.ToLower(strings.TrimSpace(base))]
}

func pastaCanonSistema(base string) string {
	if strings.EqualFold(strings.TrimSpace(base), idUbo) {
		return "uBO"
	}

	return strings.TrimSpace(base)
}

//nolint:gochecknoglobals // Ordem de icones: tabela fixa, somente leitura.
var ordemIconeExt = []string{"48", "64", "32", "128", "96", "38", "16", "19", "24"}

//exhaustruct:ignore // Item de extensao na UI: construtor parcial por desenho.
type itemExtUI struct {
	Pasta, Base, Nome, Versao, Descricao string
	IconeAbs, IconeURL, IDExt            string
	Tamanho, Acesso, Home                string
	Perms                                []string
	Ligada, Ativa, Painel                bool
	Fixada, Sistema, Ausente             bool
	Opcoes                               bool
	TemPopup                             bool

	Amplio bool
}

func tamanhoPastaExt(dir string) string {
	var total int64

	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}

		return nil
	})

	if total < bytesPorKBExt {
		return fmt.Sprintf("%d bytes", total)
	}

	if total < bytesPorKBExt*bytesPorKBExt {
		return strings.Replace(fmt.Sprintf("%.1f KB", float64(total)/bytesPorKBExt), ".", ",", 1)
	}

	return strings.Replace(fmt.Sprintf("%.1f MB", float64(total)/(bytesPorKBExt*bytesPorKBExt)), ".", ",", 1)
}

func iconeArquivoExt(dir string, manifesto extManifest) string {
	for _, chave := range ordemIconeExt {
		rel, ok := manifesto.Icons[chave]
		if !ok || strings.TrimSpace(rel) == "" {
			continue
		}

		abs, err := filepath.Abs(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		// Contencao: o manifest e controlado pela extensao; "icons" com
		// "../" nao pode escapar da pasta dela.
		base, err := filepath.Abs(dir)
		if err != nil || abs != base && !strings.HasPrefix(abs, base+string(filepath.Separator)) {
			continue
		}

		if st, err := os.Stat(abs); err == nil && !st.IsDir() {
			return abs
		}
	}

	return ""
}

func iconeURLArquivo(abs string) string {
	if abs == "" {
		return ""
	}

	//exhaustruct:ignore // url.URL: so esquema + caminho; resto zero intencional.
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
}

func (a *app) temIconeExt(base string) bool {
	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return false
	}

	dir := filepath.Join(extDir, orig)
	// GOD: orig e pasta real no disco (pastaRealExtensao); nome fixo.
	dados, err := os.ReadFile(filepath.Join(dir, "manifest.json")) // #nosec G304
	if err != nil {
		return false
	}

	var m extManifest
	if json.Unmarshal(dados, &m) != nil {
		return false
	}

	return iconeArquivoExt(dir, m) != ""
}

func (a *app) tamanhoIconeExt() int32 {
	tam := max(a.sc(16), tamanhoMinimoIcone)

	return tam
}

func (a *app) esquecerIconeExt(base string) {
	for _, motor := range []*d2dMotor{
		&d2dMain, &d2dMenu, &d2dSub, &d2dSub2, &d2dPanel, &d2dClip, &d2dRec, &d2dAviso, &d2dTip, &d2dCfg, &d2dCalendario,
	} {
		if motor.bmps == nil {
			continue
		}

		for _, chave := range []string{base, strings.ToLower(base)} {
			if bm, ok := motor.bmps[chave]; ok {
				if bm != nil {
					bm.Release()
				}

				delete(motor.bmps, chave)
			}
		}
	}
}

func lerManifestoExt(dir string) (extManifest, bool) {
	var manifesto extManifest
	// GOD: nome vem do proprio disco (ReadDir, sem separadores); nome fixo.
	blob, err := os.ReadFile(filepath.Join(dir, "manifest.json")) // #nosec G304
	if err != nil {
		return manifesto, false
	}

	if json.Unmarshal(blob, &manifesto) != nil {
		return manifesto, false
	}

	return manifesto, true
}

func amploDaExtensao(manifesto extManifest) bool {
	if len(manifesto.HostPermissions) > 0 {
		return true
	}

	for _, cs := range manifesto.ContentScripts {
		if len(cs.Matches) > 0 {
			return true
		}
	}

	return false
}

func montarItemExt(dir, nomeDisco string, manifesto extManifest) itemExtUI {
	base := strings.ToLower(nomeDisco)

	perms := append(append([]string{}, manifesto.Permissions...), manifesto.HostPermissions...)
	if len(perms) > maxPermsExt {
		perms = perms[:maxPermsExt]
	}

	iconAbs := iconeArquivoExt(dir, manifesto)
	temOpcoes := (manifesto.OptionsUI != nil && strings.TrimSpace(manifesto.OptionsUI.Page) != "") ||
		strings.TrimSpace(manifesto.OptionsPage) != ""
	temPopup := (manifesto.Action != nil && strings.TrimSpace(manifesto.Action.DefaultPopup) != "") ||
		(manifesto.BrowserAction != nil && strings.TrimSpace(manifesto.BrowserAction.DefaultPopup) != "")

	acesso := ""
	if len(manifesto.HostPermissions) > 0 {
		acesso = "Em todos os sites"
	}

	item := itemExtUI{
		Pasta: "extensions\\" + nomeDisco, Base: base,
		Nome:   nomeExibicaoExtensao(manifesto, dir),
		Versao: manifesto.Version, Descricao: strings.TrimSpace(manifesto.Description),
		IconeAbs: iconAbs, IconeURL: iconeURLArquivo(iconAbs),
		Perms: perms, Opcoes: temOpcoes, TemPopup: temPopup,
		Tamanho: tamanhoPastaExt(dir), Acesso: acesso, Home: strings.TrimSpace(manifesto.HomepageURL),
	}
	item.Sistema = ehExtensaoSistema(base)
	item.Amplio = amploDaExtensao(manifesto)

	return item
}

func (a *app) completarEstadoExt(item itemExtUI) itemExtUI {
	if a == nil {
		return item
	}

	item.Ligada = a.extensaoLigada(item.Base)

	item.Ativa = a.extLoaded[item.Base]
	if a.extIDs != nil {
		item.IDExt = a.extIDs[item.Base]
	}

	if a.extPopups != nil && a.extPopups[item.Base] != "" && item.Ativa {
		item.Painel = true
	}

	item.Fixada = a.extFixada(item.Base)

	return item
}

func (a *app) anexarAusentesExt(out []itemExtUI) []itemExtUI {
	if a == nil {
		return out
	}

	vista := map[string]bool{}
	for _, item := range out {
		vista[item.Base] = true
	}

	for _, extensao := range a.st.Extensions {
		base := strings.ToLower(filepath.Base(strings.ReplaceAll(extensao, "/", "\\")))
		if base == "" || base == "." || vista[base] {
			continue
		}

		out = append(out, itemExtUI{
			Pasta: extensao, Base: base, Nome: pastaCanonSistema(base),
			Ligada: true, Ausente: true, Sistema: ehExtensaoSistema(base),
		})
	}

	return out
}

func (a *app) listaExtensoesUI() []itemExtUI {
	pastas, _ := os.ReadDir(extDir)

	var out []itemExtUI

	for _, entrada := range pastas {
		if !entrada.IsDir() {
			continue
		}

		dir := filepath.Join(extDir, entrada.Name())

		manifesto, ok := lerManifestoExt(dir)
		if !ok {
			continue
		}

		item := a.completarEstadoExt(montarItemExt(dir, entrada.Name(), manifesto))

		out = append(out, item)
	}

	return a.anexarAusentesExt(out)
}

func (a *app) baseUbo() string {
	if a.extLoaded[idUbo] {
		return idUbo
	}

	if a.extensaoLigada(idUbo) && a.pastaRealExtensao(idUbo) != "" {
		return idUbo
	}

	return ""
}

func (a *app) pastaRealExtensao(pasta string) string {
	base := strings.ToLower(strings.TrimSpace(pasta))
	if base == "" {
		return ""
	}

	pastas, _ := os.ReadDir(extDir)
	for _, d := range pastas {
		if d.IsDir() && strings.ToLower(d.Name()) == base {
			return d.Name()
		}
	}

	return ""
}

func (a *app) extFixada(base string) bool {
	base = strings.ToLower(strings.TrimSpace(base))
	for _, f := range a.st.ExtFixadas {
		if strings.ToLower(strings.TrimSpace(f)) == base {
			return true
		}
	}

	return false
}

func (a *app) alternarFixada(pasta string, fixar bool) {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(pasta, "/", "\\")))
	if base == "" || base == "." {
		return
	}

	if base == idUbo && !fixar {
		return
	}

	var novo []string

	for _, f := range a.st.ExtFixadas {
		if strings.ToLower(strings.TrimSpace(f)) == base {
			continue
		}

		novo = append(novo, f)
	}

	if fixar {
		if !a.extensaoLigada(base) {
			a.dizerErrof(txtExtLigueAntes)

			return
		}

		if len(novo) >= maxExtFixadas {
			a.dizerf(txtFmtMaxFixadas, maxExtFixadas)

			return
		}

		novo = append(novo, base)
	}

	a.st.ExtFixadas = novo
	a.persist()
	a.layout()
	a.invalidateBar()
}

func (a *app) reconciliarMapasExt() {
	existe := func(base string) bool { return a.pastaRealExtensao(base) != "" }
	for _, m := range []map[string]string{a.extIDs, a.extPopups} {
		for base := range m {
			if !existe(base) {
				delete(m, base)
			}
		}
	}

	for base := range a.extLoaded {
		if !existe(base) {
			delete(a.extLoaded, base)
		}
	}
}

func (a *app) reconciliarFixadasExt() {
	var fix []string

	for _, f := range a.st.ExtFixadas {
		if a.pastaRealExtensao(f) != "" {
			fix = append(fix, f)
		}
	}

	temUbo := false

	for _, f := range fix {
		if strings.ToLower(strings.TrimSpace(f)) == idUbo {
			temUbo = true

			break
		}
	}

	if !temUbo && a.pastaRealExtensao(idUbo) != "" {
		fix = append(fix, idUbo)
	}

	if len(fix) != len(a.st.ExtFixadas) {
		a.st.ExtFixadas = fix
		a.persist()
	}
}

func (a *app) reconciliarExtensoes() {
	a.reconciliarMapasExt()
	a.reconciliarFixadasExt()
}

func (a *app) pastaDoFloat() string {
	if !a.extFloatOn || a.extFloatID == "" || a.extIDs == nil {
		return ""
	}

	for base, id := range a.extIDs {
		if id == a.extFloatID {
			return base
		}
	}

	return ""
}

func dicaLinhaExt(item itemExtUI) string {
	if !item.Ligada {
		return ""
	}

	if !item.Ativa {
		return txtHintCarregando
	}

	if !item.TemPopup {
		return txtHintSemPainel
	}

	return ""
}

func (a *app) menuDominioAtual() string {
	if a.left == nil {
		return ""
	}

	if dom := dominioDeURL(strings.TrimSpace(a.left.ultimaURL)); dom != "" {
		return dom
	}

	if a.left.wv != nil {
		return dominioDeURL(a.left.wv.Source())
	}

	return ""
}

func menuLinhaDesligada(item itemExtUI) menuItem {
	return menuItem{kind: miItem, label: item.Nome, hint: txtHintDesligada, id: "extligar:" + item.Base}
}

func (a *app) menuSubDominioExt(item itemExtUI, domAtual string) []menuItem {
	sub := []menuItem{{kind: miHeader, label: txtDomSecao, desc: a.resumoDominios(item.Base)}}

	sub = append(sub, menuItem{kind: miItem, label: txtDomSempreItem, id: "extdom:off:" + item.Base})
	if domAtual == "" {
		return sub
	}

	sub = append(sub, menuItem{
		kind: miItem, label: fmt.Sprintf(txtFmtDomSoNeste, domAtual),
		id: "extdom:so:" + item.Base + ":" + domAtual,
	})
	sub = append(sub, menuItem{
		kind: miItem, label: fmt.Sprintf(txtFmtDomSoNesteSub, domAtual),
		id: "extdom:sow:" + item.Base + ":" + domAtual,
	})
	sub = append(sub, menuItem{
		kind: miItem, label: fmt.Sprintf(txtFmtDomExcetoNeste, domAtual),
		id: "extdom:nao:" + item.Base + ":" + domAtual,
	})
	sub = append(sub, menuItem{
		kind: miItem, label: fmt.Sprintf(txtFmtDomExcetoNesteSub, domAtual),
		id: "extdom:naow:" + item.Base + ":" + domAtual,
	})

	return sub
}

func (a *app) menuSubExtensao(item itemExtUI, domAtual string) []menuItem {
	var sub []menuItem
	if item.Opcoes {
		sub = append(sub, menuItem{kind: miItem, label: txtExtOpcoes, id: "extopcoes:" + item.Base})
	}

	sub = append(sub, menuItem{kind: miItem, label: txtExtDesativar, id: "extdesligar:" + item.Base})

	sub = append(sub, a.subMenuUpdateExt(item)...)

	if item.Sistema {
		return sub
	}

	sub = append(sub, menuItem{kind: miItem, label: txtExtRemoverItem, id: "extremover:" + item.Base})
	sub = append(sub, menuItem{kind: miSep})
	sub = append(sub, a.menuSubDominioExt(item, domAtual)...)

	return sub
}

// subMenuUpdateExt monta os itens de verificacao e aplicacao de update.
func (a *app) subMenuUpdateExt(item itemExtUI) []menuItem {
	var saida []menuItem

	saida = append(saida, menuItem{kind: miItem, label: txtExtVerificarUpdate, id: "extatualizar:" + item.Base})

	if nova := a.novaVersaoUpdateExt(item.Base); nova != "" {
		saida = append(saida, menuItem{
			kind: miItem, label: fmt.Sprintf(txtFmtUpdateAplicar, nova), id: "extaplicar:" + item.Base,
		})
	}

	return saida
}

func (a *app) menuLinhaExtensao(item itemExtUI, domAtual string) menuItem {
	if !item.Ligada {
		return menuLinhaDesligada(item)
	}

	return menuItem{
		kind: miExt, label: item.Nome, hint: dicaLinhaExt(item), id: "ext:" + item.Base,
		iconeBase: item.Base, ext: item.Base, sub: a.menuSubExtensao(item, domAtual),
	}
}

func (a *app) menuLinhasExtensoes(lista []itemExtUI, domAtual string) []menuItem {
	saidas := make([]menuItem, 0, len(lista))

	for _, item := range lista {
		saidas = append(saidas, a.menuLinhaExtensao(item, domAtual))
	}

	return saidas
}

func menuSepararAmplas(ui []itemExtUI) ([]itemExtUI, []itemExtUI) {
	var total, resto []itemExtUI

	for _, item := range ui {
		if ehExtensaoSistema(item.Base) {
			continue
		}

		if item.Amplio {
			total = append(total, item)
		} else {
			resto = append(resto, item)
		}
	}

	return total, resto
}

func (a *app) buildMenuExtensoes() []menuItem {
	itens := []menuItem{{kind: miHeader, label: txtHdrExtensoes}}
	ui := a.listaExtensoesUI()
	domAtual := a.menuDominioAtual()

	total, resto := menuSepararAmplas(ui)
	itens = append(itens, a.menuLinhasExtensoes(total, domAtual)...)

	itens = append(itens, a.menuLinhasExtensoes(resto, domAtual)...)
	if len(total)+len(resto) == 0 {
		itens = append(itens, menuItem{kind: miItem, label: txtExtNenhuma, id: ""})
	}

	return itens
}

func (a *app) abrirMenuExtensoes() {
	if a.extFloatOn {
		a.fecharPainelFlutuante()
	}

	if menu.open && a.menuOrigem == idExtensoes {
		a.closeMenu()

		return
	}

	if menu.open {
		a.closeMenu()
	}

	a.contextAba = -1
	a.menuOrigem = idExtensoes
	menu.items = a.buildMenuExtensoes()
	menu.headerX = true

	a.menuMetrics()

	if menu.hwnd == 0 && !a.createMenuWindow() {
		return
	}

	if menuSub.hwnd == 0 && !a.createSubMenuWindow() {
		return
	}

	a.abrirMenuAncorado(a.extBtn.rect)
}

func (a *app) opcoesDaExtensao(pasta string) (string, bool, string, error) {
	base := strings.ToLower(strings.TrimSpace(pasta))
	nome := strings.TrimSpace(pasta)

	if base == "" {
		return "", false, "", erroTexto("extensão inválida")
	}

	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return "", false, "", erroTexto("extensão não encontrada")
	}

	dir := filepath.Join(extDir, orig)
	// GOD: orig e pasta real no disco (pastaRealExtensao); nome fixo.
	dados, errLeitura := os.ReadFile(filepath.Join(dir, "manifest.json")) // #nosec G304
	if errLeitura != nil {
		return "", false, "", erroTexto("sem manifesto")
	}

	var manifesto extManifest
	if json.Unmarshal(dados, &manifesto) != nil {
		return "", false, "", erroTexto("manifesto inválido")
	}

	if n := nomeExibicaoExtensao(manifesto, dir); strings.TrimSpace(n) != "" {
		nome = n
	}

	pagina := ""
	emAba := false

	if manifesto.OptionsUI != nil && strings.TrimSpace(manifesto.OptionsUI.Page) != "" {
		pagina = strings.TrimSpace(manifesto.OptionsUI.Page)
		emAba = manifesto.OptionsUI.OpenInTab
	} else if strings.TrimSpace(manifesto.OptionsPage) != "" {
		pagina = strings.TrimSpace(manifesto.OptionsPage)
	}

	if pagina == "" {
		return "", false, "", erroTextoF("%s não tem página de opções", nome)
	}

	return pagina, emAba, nome, nil
}

func (a *app) abrirOpcoesExtensao(pasta string) {
	pagina, emAba, nome, err := a.opcoesDaExtensao(pasta)
	if err != nil {
		a.dizerErrof("%s", err)

		return
	}

	base := strings.ToLower(strings.TrimSpace(pasta))

	id := ""
	if a.extIDs != nil {
		id = a.extIDs[base]
	}

	if id == "" || (a.extLoaded != nil && !a.extLoaded[base]) {
		a.dizerErrof(txtFmtExtNaoCarr2, nome)

		return
	}

	dest := "chrome-extension://" + id + "/" + strings.TrimLeft(pagina, "/")
	if emAba {
		a.novaAba(dest, true)

		return
	}

	a.abrirFlutuanteURL(dest, id, nome+" — opções", false)
}

func (a *app) confirmarExclusao(nome string) bool {
	if a == nil || a.hwnd == 0 {
		return true
	}

	const (
		mbYesNo       = 0x04
		mbIconWarning = 0x30
		idYes         = 6
	)

	pergunta := utf16(fmt.Sprintf(txtFmtConfirmarExt, nome))
	titulo := utf16(appTitle)
	r, _, _ := pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(pergunta)), uintptr(unsafe.Pointer(titulo)),
		mbYesNo|mbIconWarning)

	return int32(r) == idYes
}

func (a *app) removerPastaValidada(pasta string) (string, string, string, bool) {
	orig := a.pastaRealExtensao(pasta)
	if orig == "" {
		a.dizerErrof(txtExtNaoAchada)

		return "", "", "", false
	}

	base := strings.ToLower(orig)
	if ehExtensaoSistema(base) {
		a.dizerErrof(txtFmtExtSistema, orig)

		return "", "", "", false
	}

	absDir, err1 := filepath.Abs(filepath.Join(extDir, orig))

	absExe, err2 := filepath.Abs(exeDir)
	if err1 != nil || err2 != nil || absDir != absExe && !strings.HasPrefix(absDir, absExe+string(filepath.Separator)) {
		a.dizerErrof(txtExtForaArea)

		return "", "", "", false
	}

	return orig, base, absDir, true
}

func removerNomeExt(orig, absDir string) string {
	nome := orig
	// GOD: absDir contido em exeDir (verificado acima); nome fixo.
	if b, err := os.ReadFile(filepath.Join(absDir, "manifest.json")); err == nil { // #nosec G304
		var m extManifest
		if json.Unmarshal(b, &m) == nil {
			if n := nomeExibicaoExtensao(m, absDir); strings.TrimSpace(n) != "" {
				nome = n
			}
		}
	}

	return nome
}

func (a *app) removerEstadoLocal(orig, base, absDir, nome string) {
	_ = os.RemoveAll(absDir)

	a.alternarExtensao(orig, false)

	if a.extIDs != nil {
		delete(a.extIDs, base)
	}

	if a.extPopups != nil {
		delete(a.extPopups, base)
	}

	if a.extLoaded != nil {
		delete(a.extLoaded, base)
	}

	a.esquecerIconeExt(base)

	if a.extFloatOn && a.pastaDoFloat() == base {
		a.fecharPainelFlutuante()
	}

	var fix []string

	for _, f := range a.st.ExtFixadas {
		if strings.ToLower(strings.TrimSpace(f)) != base {
			fix = append(fix, f)
		}
	}

	a.st.ExtFixadas = fix
	a.persist()
	a.layout()
	a.invalidateBar()
	a.dizerOkf(txtFmtExtRemovida, nome)
}

func removerAlvoExtensao(list *ICoreWebView2BrowserExtensionList, id string) *ICoreWebView2BrowserExtension {
	var total uint32
	list.vtbl.GetCount.Call(uintptr(unsafe.Pointer(list)), uintptr(unsafe.Pointer(&total)))

	for i := range total {
		var ext *ICoreWebView2BrowserExtension

		hr, _, _ := list.vtbl.GetValueAtIndex.Call(
			uintptr(unsafe.Pointer(list)),
			uintptr(i),
			uintptr(unsafe.Pointer(&ext)),
		)
		if Failed(hr) || ext == nil {
			continue
		}

		var idp *uint16
		ext.vtbl.GetID.Call(uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(&idp)))

		if takeString(idp) == id {
			return ext
		}

		ext.Release()
	}

	return nil
}

func (a *app) removerAoConfirmarNavegador(alvo *ICoreWebView2BrowserExtension, err error, nome string, final func()) {
	alvo.Release()
	a.depois(func() {
		if err != nil {
			a.dizerErrof(txtFmtNaoRemovi, nome)

			return
		}

		final()
	})
}

func (a *app) removerExtensao(pasta string) {
	orig, base, absDir, ok := a.removerPastaValidada(pasta)
	if !ok {
		return
	}

	nome := removerNomeExt(orig, absDir)

	if !a.confirmarExclusao(nome) {
		return
	}

	id := ""
	if a.extIDs != nil {
		id = a.extIDs[base]
	}

	final := func() { a.removerEstadoLocal(orig, base, absDir, nome) }
	a.removerViaNavegadorComID(id, nome, final)
}

func (a *app) removerViaNavegadorComID(id, nome string, final func()) {
	if id == "" || a.left == nil || !a.left.ready || a.left.wv == nil {
		final()

		return
	}

	a.left.wv.GetBrowserExtensions(func(list *ICoreWebView2BrowserExtensionList, err error) {
		if err != nil || list == nil {
			a.depois(func() { a.dizerErrof(txtFmtNaoRemoviAgora, nome) })

			return
		}
		defer list.Release()

		alvo := removerAlvoExtensao(list, id)
		if alvo == nil {
			a.depois(final)

			return
		}

		alvo.Remove(func(err error) {
			a.removerAoConfirmarNavegador(alvo, err, nome, final)
		})
	})
}
