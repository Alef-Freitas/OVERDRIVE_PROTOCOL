//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const extFloatClasse = "OverdriveExtFloatWnd"

const (
	esperaLimpezaFloat    = 1500 * time.Millisecond
	esperaMedidaFloat     = 2 * time.Second
	limiteEstavelFloat    = 3
	meioFloat             = 2
	toleranciaQuadroFloat = 8
)

//nolint:gochecknoglobals // Ponto de entrada da janela flutuante registrado uma vez no registrarCallbacks.
var extFloatProcPtr uintptr

// registrarProcFloat fixa o ponto de entrada da janela flutuante.
// Ficava em init(); virou chamada explicita via registrarCallbacks.
func registrarProcFloat() {
	extFloatProcPtr = windows.NewCallback(extFloatProc)
}

const medidaAlturaJS = `(function(){var d=document.documentElement;if(!d){return{ok:false};}` +
	`var b=document.body;var iw=window.innerWidth||0,ih=window.innerHeight||0;` +
	`var maxR=0,minL=0;try{var all=d.getElementsByTagName('*');` +
	`for(var i=0;i<all.length;i++){var el=all[i];` +
	`if(el===b||el===d){continue;}var r=el.getBoundingClientRect();` +
	`if(r.width<=0||r.height<=0){continue;}` +
	`if(r.right<0||r.left>2*iw){continue;}var sw=0;` +
	`try{sw=el.scrollWidth||0;}catch(e){}var w=r.width;` +
	`if(sw>w){w=sw;}var right=r.left+w;` +
	`if(right>maxR){maxR=right;}if(r.left<minL){minL=r.left;}}}` +
	`catch(e){}var need=maxR;var neg=minL<0?-minL:0;` +
	`if(iw+neg>need){need=iw+neg;}` +
	`return{ok:true,h:Math.max(d.scrollHeight||0,b?b.scrollHeight||0:0),` +
	`ih:ih,w:Math.max(d.scrollWidth||0,b?b.scrollWidth||0:0),iw:iw,` +
	`need:Math.ceil(need)};})()`

const shimAbaExtJSBase = `(function(){
  if(window.__odAbaShim) return;
  window.__odAbaShim=true;
  var alvo=__ABA_ALVO_JSON__;
  window.__odDefinirAba=function(u){ try{ alvo=String(u||""); }catch(e){} };
  function semFrag(u){ u=String(u||""); var i=u.indexOf("#"); return i<0?u:u.slice(0,i); }
  var eu=semFrag(location.href);
  function escolher(todas){
    var a=semFrag(alvo), eu=semFrag(location.href);
    var achou=null, unicaVis=null, nVis=0;
    for(var i=0;i<todas.length;i++){
      var t=todas[i];
      if(!t) continue;
      var u=String(t.url||"");
      if(u.indexOf("about:")===0) continue;
      if(u.indexOf("chrome-extension://")===0) continue;
      if(u!==""&&semFrag(u)===eu) continue;
      // Sem a permissão "tabs" a URL da página vem oculta — e identificar
      // sem URL não habilita mensagem mesmo assim: vazio honesto.
      if(u==="") continue;
      nVis++; unicaVis=t;
      if(a!==""&&semFrag(u)===a) achou=t;
    }
    if(achou) return achou;
    // Uma única página visível (URL mudou sem o host ver): ela é a atual.
    if(nVis===1) return unicaVis;
    return null;
  }
  function consultaAtiva(q){
    if(!q||typeof q!=="object") return false;
    return q.active===true||q.lastFocusedWindow===true||q.currentWindow===true;
  }
  try{
    if(window.chrome&&chrome.tabs&&typeof chrome.tabs.query==="function"){
      var orig=chrome.tabs.query.bind(chrome.tabs);
      var ok=false;
      try{
        chrome.tabs.query=function(q,cb){
          if(typeof q==="function"){ cb=q; q={}; }
          if(typeof cb!=="function") return orig(q,cb);
          if(!consultaAtiva(q)) return orig(q,cb);
          return orig({},function(todas){
            var t=null;
            try{ t=escolher(todas||[]); }catch(e){}
            if(t){ try{ cb([t]); }catch(e){} return; }
            // Nulo pode ser página ainda carregando: uma nova tentativa.
            setTimeout(function(){
              orig({},function(t2){
                var u=null;
                try{ u=escolher(t2||[]); }catch(e){}
                try{ cb(u?[u]:[]); }catch(e){}
              });
            },800);
          });
        };
        ok=(chrome.tabs.query!==orig);
      }catch(e){ ok=false; }
      window.__odAbaShimAtivo=ok;
    }
  }catch(e){}
})();`

func shimAbaExtJS(alvo string) string {
	q := jsonDeString(alvo)
	q = strings.ReplaceAll(q, "<", "\\u003c")
	q = strings.ReplaceAll(q, ">", "\\u003e")

	return strings.Replace(shimAbaExtJSBase, "__ABA_ALVO_JSON__", q, 1)
}

func (a *app) alvoAbaAtual() string {
	if a == nil || a.left == nil {
		return ""
	}

	if endereco := strings.TrimSpace(a.left.ultimaURL); endereco != "" {
		return endereco
	}

	if a.left.wv != nil {
		return strings.TrimSpace(a.left.wv.Source())
	}

	return ""
}

func (a *app) abrirPainelFlutuante(id, caminho, nome string) {
	if nome == "" {
		nome = txtExtSemNome
	}

	if id == "" {
		a.dizerErrof(txtFmtExtNaoCarr, nome)

		return
	}

	if caminho == "" {
		caminho = paginaPopup
	}

	url := a.urlPopupExtensao(id, caminho)
	a.abrirFlutuanteURL(url, id, nome, true)
}

func (a *app) urlPopupExtensao(id, caminho string) string {
	endereco := "chrome-extension://" + id + "/" + strings.TrimLeft(caminho, "/")
	if a.ehPopupUbo(id, caminho) && !strings.Contains(strings.ToLower(caminho), "portrait") {
		sep := "?"
		if strings.Contains(endereco, "?") {
			sep = "&"
		}

		endereco += sep + "portrait"
	}

	return endereco
}

func (a *app) ehPopupUbo(id, caminho string) bool {
	if a == nil || id == "" || a.extIDs == nil {
		return false
	}

	base := a.baseUbo()
	if base == "" || a.extIDs[base] != id {
		return false
	}

	pagina := strings.TrimLeft(strings.TrimSpace(caminho), "/")
	if i := strings.IndexAny(pagina, "?#"); i >= 0 {
		pagina = pagina[:i]
	}

	if i := strings.LastIndex(pagina, "/"); i >= 0 {
		pagina = pagina[i+1:]
	}

	return strings.EqualFold(pagina, "popup-fenix.html")
}

func (a *app) flutuanteAlternarSeMesmo(id string) bool {
	if a.extFloatOn && a.extFloatID == id && id != "" && a.extFloat != nil && a.extFloat.ready && a.extFloat.wv != nil {
		a.fecharPainelFlutuante()

		return true
	}

	return false
}

func (a *app) flutuanteFecharConflitos() {
	if a.popupOn {
		a.fecharPopup()
		a.layout()
	}
}

func (a *app) flutuanteDimensionarInicial(url, id string, comShim bool) (int32, int32, bool) {
	larg := a.sc(380)
	alt := a.sc(480)

	uboPopup := comShim && a.ehPopupUbo(id, url)
	if !uboPopup {
		return larg, alt, false
	}

	larg = a.sc(360)

	area := areaTrabalhoJanela(a.hwnd)
	if maxW := area.Width() - a.sc(24); maxW > 0 && larg > maxW {
		larg = maxW
	}

	if maxH := area.Height() - a.sc(24); maxH > 0 && alt > maxH {
		alt = maxH
	}

	return larg, alt, true
}

func (a *app) flutuanteNavegar(p *pane, url string, comShim bool, _, _ int32, uboPopup bool) {
	a.extFloat = p
	a.configureWebView(p, false)

	if comShim {
		p.wv.AddInitScript(shimAbaExtJS(a.alvoAbaAtual()))
	}

	p.wv.SetBounds(p.rect)
	p.wv.SetVisible(true)
	p.wv.Navigate(url)

	if !uboPopup {
		a.vigiarAlturaFloat()
	}
}

func (a *app) flutuanteAoControlador(
	url, _, nome string, comShim bool, larg, alt int32, uboPopup bool, gen uint64,
) func(*WebView, error) {
	return func(w *WebView, err error) {
		if err != nil {
			a.extFloatOn = false
			a.dizerErrof(txtFmtNaoAbri, nome)

			return
		}

		if a.quitting || !a.extFloatOn || gen != a.extFloatGen {
			w.Close()

			return
		}

		if a.extFloatHWND == 0 {
			w.Close()

			return
		}

		p := &pane{wv: w, zoom: a.st.Zoom}
		p.ready = true
		p.rect = Rect{Left: 0, Top: 0, Right: larg, Bottom: alt}
		a.flutuanteNavegar(p, url, comShim, larg, alt, uboPopup)
	}
}

func (a *app) flutuantePrepararControle(nome, url, id string, comShim bool) (int32, int32, bool, uint64, bool) {
	larg, alt, uboPopup := a.flutuanteDimensionarInicial(url, id, comShim)

	a.extFloatFixo = uboPopup

	if !a.criarJanelaFloat(nome, larg, alt) {
		a.dizerErrof(txtFmtNaoAbri, nome)

		return 0, 0, false, 0, false
	}

	a.extFloatID = id
	a.extFloatNome = nome
	a.extFloatAltura = alt
	a.extFloatMedido = false
	a.extFloatMedH, a.extFloatMedW, a.extFloatMedN = 0, 0, 0
	a.extFloatGen++
	gen := a.extFloatGen
	a.extFloatOn = true

	if a.extFloat != nil {
		a.destruirPane(a.extFloat)
		a.extFloat = nil
	}

	return larg, alt, uboPopup, gen, true
}

func (a *app) abrirFlutuanteURL(url, id, nome string, comShim bool) {
	if nome == "" {
		nome = txtExtSemNome
	}

	if url == "" {
		a.dizerErrof(txtExtNadaAbrir)

		return
	}

	if a.flutuanteAlternarSeMesmo(id) {
		return
	}

	a.flutuanteFecharConflitos()

	if a.env == nil {
		a.dizerErrof(txtFmtIndispMotor, nome)

		return
	}

	larg, alt, uboPopup, gen, ok := a.flutuantePrepararControle(nome, url, id, comShim)
	if !ok {
		return
	}

	err := a.env.CreateController(
		a.extFloatHWND, a.flutuanteAoControlador(url, id, nome, comShim, larg, alt, uboPopup, gen),
	)
	if err != nil {
		a.extFloatOn = false
		a.dizerErrof(txtFmtNaoAbri, nome)
	}
}

func (a *app) flutuantePontoAncorado(largura int32) (int32, int32) {
	if a.extFloatFixo && a.uboBtn.rect.Width() > 0 {
		y := max(a.uboBtn.rect.Bottom, a.tabH+a.tbHeight) + a.sc(4)
		x := (a.uboBtn.rect.Left + a.uboBtn.rect.Right) / meioFloat

		ponto := struct{ x, y int32 }{x, y}
		pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))
		ponto.x -= largura / meioFloat

		return ponto.x, ponto.y
	}

	recorte := a.clientRect()
	ponto := struct{ x, y int32 }{recorte.Right - largura - a.sc(12), a.tabH + a.tbHeight + a.sc(8)}
	pClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&ponto)))

	return ponto.x, ponto.y
}

func (a *app) flutuanteEnquadrarTela(largura, altura, x, y int32) (int32, int32) {
	area := areaTrabalhoJanela(a.hwnd)

	margem := a.sc(12)
	if x+largura > area.Right-margem {
		x = area.Right - largura - margem
	}

	if x < area.Left+margem {
		x = area.Left + margem
	}

	if y+altura > area.Bottom-margem {
		y = area.Bottom - altura - margem
	}

	if y < area.Top+a.sc(4) {
		y = area.Top + a.sc(4)
	}

	return x, y
}

func (a *app) flutuanteRecolocar(x, y, larg, alt int32) bool {
	if a.extFloatHWND == 0 {
		return false
	}

	pSetWindowPos.Call(a.extFloatHWND, 0, uintptr(x), uintptr(y),
		uintptr(larg), uintptr(alt), swpNoZOrder|swpNoActivate)

	return true
}

func (a *app) criarJanelaFloat(nome string, largura, altura int32) bool {
	if extFloatProcPtr == 0 {
		return false
	}

	hInst := getModuleHandle()

	nomeClasse := utf16(extFloatClasse)
	if a.extFloatHWND == 0 {
		classeJanela := wndClassExW{
			cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
			lpfnWndProc:   extFloatProcPtr,
			hInstance:     hInst,
			hCursor:       loadCursor(idcArrow),
			hbrBackground: 0,
			lpszClassName: nomeClasse,
		}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&classeJanela)))
	}

	x, y := a.flutuantePontoAncorado(largura)
	x, y = a.flutuanteEnquadrarTela(largura, altura, x, y)

	if a.flutuanteRecolocar(x, y, largura, altura) {
		return true
	}

	titulo := utf16(fmt.Sprintf(txtFmtTituloFloat, nome))

	handle, _, _ := pCreateWindowExW.Call(
		wsExToolWindow,
		uintptr(unsafe.Pointer(nomeClasse)),
		uintptr(unsafe.Pointer(titulo)),
		wsPopup,
		uintptr(x), uintptr(y), uintptr(largura), uintptr(altura),
		a.hwnd, 0, hInst, 0,
	)
	if handle == 0 {
		return false
	}

	a.extFloatHWND = handle
	aplicarCantosDWM(handle)
	pShowWindow.Call(handle, swShowNoActivate)

	return true
}

func (a *app) fecharPainelFlutuante() {
	if !a.extFloatOn && a.extFloatHWND == 0 {
		return
	}

	a.extFloatGen++

	a.extFloatVigia++
	if a.extFloat != nil {
		a.destruirPane(a.extFloat)
		a.extFloat = nil
	}

	a.extFloatOn = false
	a.extFloatID = ""
	a.extFloatNome = ""
	a.extFloatAltura = 0
	a.extFloatFixo = false
	a.extFloatMedido = false

	a.extFloatMedH, a.extFloatMedW, a.extFloatMedN = 0, 0, 0
	if a.extFloatHWND != 0 {
		handle := a.extFloatHWND
		a.extFloatHWND = 0

		pDestroyWindow.Call(handle)
	}

	a.extFloatEstavel = 0
	a.agendarLimpezaMemoria(true, esperaLimpezaFloat)
}

func (a *app) vigiarAlturaFloat() {
	if a.extFloatFixo {
		return
	}

	a.extFloatVigia++
	gen := a.extFloatVigia
	a.extFloatEstavel = 0

	time.AfterFunc(esperaMedidaFloat, func() {
		a.depois(func() { a.medirAlturaFloat(gen) })
	})
}

func (a *app) flutuanteDeveMedir(gen uint64) bool {
	if gen != a.extFloatVigia || !a.extFloatOn || a.extFloatFixo {
		return false
	}
	// Minimizado ou altura estavel: para de reagendar em vez de girar
	// para sempre a cada 2 s.
	if minimizado, _, _ := pIsIconic.Call(a.hwnd); minimizado != 0 {
		return false
	}

	return a.extFloatEstavel < limiteEstavelFloat
}

func (a *app) flutuanteReagendarMedida(gen uint64) {
	time.AfterFunc(esperaMedidaFloat, func() {
		a.depois(func() { a.medirAlturaFloat(gen) })
	})
}

func (a *app) flutuanteColetarMedida() {
	p := a.extFloat
	if p == nil || !p.ready || p.wv == nil {
		return
	}

	p.wv.EvalJS(medidaAlturaJS, func(err error, res string) {
		a.flutuanteReceberMedida(err, res)
	})
}

func (a *app) flutuanteReceberMedida(err error, res string) {
	if err != nil {
		return
	}

	var medida struct {
		OK bool `json:"ok"`
		H  int  `json:"h"`
		IH int  `json:"ih"`
		W  int  `json:"w"`
		IW int  `json:"iw"`
		NE int  `json:"need"`
	}

	if desembrulharEvalJS(res, &medida) != nil || !medida.OK {
		return
	}

	altCSS, altInterna, largCSS, largInterna, necessidade := medida.H, medida.IH, medida.W, medida.IW, medida.NE

	a.depois(func() { a.ajustarTamanhoFloat(altCSS, altInterna, largCSS, largInterna, necessidade) })
}

func (a *app) medirAlturaFloat(gen uint64) {
	if !a.flutuanteDeveMedir(gen) {
		return
	}

	a.flutuanteReagendarMedida(gen)
	a.flutuanteColetarMedida()
}

func (a *app) flutuanteAlturaAlvo(altCSS, altInterna, altAtual int32, area rectT) int32 {
	novaAlt := int32(float64(altCSS) * float64(altAtual) / float64(altInterna))
	altMin := a.sc(240)
	altMax := area.Bottom - area.Top - a.sc(24)

	if novaAlt < altMin {
		novaAlt = altMin
	}

	if altMax > altMin && novaAlt > altMax {
		novaAlt = altMax
	}

	return novaAlt
}

func (a *app) flutuanteLarguraAlvo(largCSS, largInterna, needCSS int, novaLarg int32, area rectT) int32 {
	if a.extFloatFixo || largInterna <= 0 || novaLarg <= 0 {
		return novaLarg
	}

	alvo := needCSS
	if largCSS > largInterna && largCSS > alvo {
		alvo = largCSS
	}

	if alvo <= 0 {
		return novaLarg
	}

	novaLarg = int32(float64(alvo) * float64(novaLarg) / float64(largInterna))
	minLarg := a.sc(280)
	maxLarg := area.Right - area.Left - a.sc(24)

	if novaLarg < minLarg {
		novaLarg = minLarg
	}

	if maxLarg > minLarg && novaLarg > maxLarg {
		novaLarg = maxLarg
	}

	return novaLarg
}

func flutuanteDistancia(atual, medido int) int {
	distancia := atual - medido
	if distancia < 0 {
		distancia = -distancia
	}

	return distancia
}

func (a *app) flutuanteEstavel(altCSS, largCSS, needCSS int) bool {
	dh := flutuanteDistancia(altCSS, a.extFloatMedH)
	dw := flutuanteDistancia(largCSS, a.extFloatMedW)
	dn := flutuanteDistancia(needCSS, a.extFloatMedN)

	estavel := a.extFloatMedido && dh < limiteEstavelFloat && dw < limiteEstavelFloat && dn < limiteEstavelFloat
	a.extFloatMedH, a.extFloatMedW, a.extFloatMedN = altCSS, largCSS, needCSS

	a.extFloatMedido = true
	if estavel {
		a.extFloatEstavel++
	} else {
		a.extFloatEstavel = 0
	}

	return estavel
}

func (a *app) flutuanteSuavizar(novaAlt, novaLarg, altAtual, largAtual int32, estavel bool) (int32, int32) {
	if !estavel {
		if novaAlt < altAtual {
			novaAlt = altAtual
		}

		if novaLarg < largAtual {
			novaLarg = largAtual
		}
	}

	if absI32(novaAlt-altAtual) < a.sc(8) {
		novaAlt = altAtual
	}

	if absI32(novaLarg-largAtual) < a.sc(8) {
		novaLarg = largAtual
	}

	return novaAlt, novaLarg
}

func (a *app) flutuanteProntoParaAjuste(altCSS, altInterna int) (*pane, bool) {
	p := a.extFloat
	if p == nil || !a.extFloatOn || a.extFloatFixo || a.extFloatHWND == 0 || altCSS <= 0 || altInterna <= 0 {
		return nil, false
	}

	if p.rect.Height() <= 0 {
		return nil, false
	}

	return p, true
}

func (a *app) flutuanteEnquadrarQuadro(novaLarg, novaAlt, x, y int32, area rectT) (int32, int32) {
	margem := a.sc(12)
	if x+novaLarg > area.Right-margem {
		x = area.Right - novaLarg - margem
	}

	if x < area.Left+margem {
		x = area.Left + margem
	}

	if y+novaAlt > area.Bottom-margem {
		y = area.Bottom - novaAlt - margem
	}

	if y < area.Top+a.sc(4) {
		y = area.Top + a.sc(4)
	}

	return x, y
}

func (a *app) flutuanteAplicarQuadro(p *pane, novaLarg, novaAlt int32, area rectT) {
	var quadro rectT

	chamou, _, _ := pGetWindowRect.Call(a.extFloatHWND, uintptr(unsafe.Pointer(&quadro)))
	if chamou == 0 {
		p.rect.Bottom = p.rect.Top + novaAlt
		p.rect.Right = p.rect.Left + novaLarg

		a.flutuanteFinalizarQuadro(p, novaAlt)

		return
	}

	x, y := a.flutuanteEnquadrarQuadro(novaLarg, novaAlt, quadro.Left, quadro.Top, area)
	pSetWindowPos.Call(a.extFloatHWND, 0,
		uintptr(x), uintptr(y), uintptr(novaLarg), uintptr(novaAlt),
		swpNoZOrder|swpNoActivate)
	p.rect = Rect{Left: 0, Top: 0, Right: novaLarg, Bottom: novaAlt}

	a.flutuanteFinalizarQuadro(p, novaAlt)
}

func (a *app) flutuanteFinalizarQuadro(p *pane, novaAlt int32) {
	a.extFloatAltura = novaAlt

	p.wv.SetBounds(p.rect)
}

func (a *app) ajustarTamanhoFloat(altCSS, altInterna, largCSS, largInterna, needCSS int) {
	p, pronto := a.flutuanteProntoParaAjuste(altCSS, altInterna)
	if !pronto {
		return
	}

	altAtual := p.rect.Height()
	largAtual := p.rect.Width()
	area := areaTrabalhoJanela(a.hwnd)

	novaAlt := a.flutuanteAlturaAlvo(int32(altCSS), int32(altInterna), altAtual, area)
	novaLarg := a.flutuanteLarguraAlvo(largCSS, largInterna, needCSS, largAtual, area)

	if novaAlt < 0 || novaLarg <= 0 {
		return
	}

	estavel := a.flutuanteEstavel(altCSS, largCSS, needCSS)
	novaAlt, novaLarg = a.flutuanteSuavizar(novaAlt, novaLarg, altAtual, largAtual, estavel)

	if novaAlt == altAtual && novaLarg == largAtual {
		return
	}

	a.flutuanteAplicarQuadro(p, novaLarg, novaAlt, area)
}

func flutuanteProntoParaRedimensionar(aplicativo *app, hwnd uintptr) bool {
	return aplicativo != nil && hwnd == aplicativo.extFloatHWND && aplicativo.extFloat != nil &&
		aplicativo.extFloat.ready && aplicativo.extFloat.wv != nil
}

func flutuanteAoRedimensionar(aplicativo *app, hwnd uintptr, lparam uintptr) uintptr {
	if flutuanteProntoParaRedimensionar(aplicativo, hwnd) {
		largura, altura := loword(lparam), hiword(lparam)
		if largura > 0 && altura > 0 {
			aplicativo.extFloat.rect = Rect{Left: 0, Top: 0, Right: largura, Bottom: altura}
			aplicativo.extFloat.wv.SetBounds(aplicativo.extFloat.rect)
			aplicativo.extFloat.wv.SetVisible(true)
		}
	}

	return 0
}

func flutuanteAoDesativar(aplicativo *app, wparam uintptr) uintptr {
	if aplicativo != nil && loword(wparam) == waInactive {
		aplicativo.fecharPainelFlutuante()
	}

	return 0
}

// flutuanteAoTeclar: assinatura de callback Win32 (wndProc): retorno uintptr 0 = tratado.
func flutuanteAoTeclar(aplicativo *app, wparam uintptr) (uintptr, bool) { //nolint:unparam
	if wparam != vkEscape {
		return 0, false
	}

	if aplicativo != nil {
		aplicativo.fecharPainelFlutuante()
	}

	return 0, true
}

func flutuanteAoFechar(aplicativo *app) uintptr {
	if aplicativo != nil {
		aplicativo.fecharPainelFlutuante()
	}

	return 0
}

func flutuanteAoDestruir(aplicativo *app, hwnd uintptr) uintptr {
	if aplicativo != nil && aplicativo.extFloatHWND == hwnd {
		aplicativo.extFloatHWND = 0
	}

	return 0
}

func extFloatProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	aplicativo := theApp

	switch msg {
	case wmSize:
		return flutuanteAoRedimensionar(aplicativo, hwnd, lparam)
	case wmActivate:
		return flutuanteAoDesativar(aplicativo, wparam)
	case wmKeyDown:
		if saida, confirmou := flutuanteAoTeclar(aplicativo, wparam); confirmou {
			return saida
		}
	case wmClose:
		return flutuanteAoFechar(aplicativo)
	case wmDestroy:
		return flutuanteAoDestruir(aplicativo, hwnd)
	}

	return defWindowProc(hwnd, msg, wparam, lparam)
}
