//go:build windows

//nolint:wsl_v5,exhaustruct_v5,varnamelen // Spawn: geometria do Marble intencional.

package main

// Spawn de links do Marble: clicar num link dentro de uma pagina cria um card
// filho ao lado do card de origem, ligado por uma aresta de trilha. Equivale a
// tools::placement::place_spawn + record_trail_edge (extraido do binario).

import "net/url"

// marbleLinkScript e o script de interceptacao de links original do Marble,
// extraido do binario. window.open dispara OnNewWindow -> marbleSpawnDeLink.
const marbleLinkScript = `(function(){try{
document.addEventListener('click',(e)=>{
  var a=e.target.closest&&e.target.closest('a[href]'); if(!a) return;
  var href=a.getAttribute('href'); var u;
  try{ u=new URL(href, location.href); }catch(_){ return; }
  var p=u.protocol;
  if(p==='javascript:'||p==='mailto:'||p==='tel:') return;
  if(u.href.indexOf('#')>=0 &&
     u.origin===location.origin &&
     u.pathname===location.pathname &&
     u.search===location.search) return;
  if(e.altKey){ e.preventDefault(); location.href=u.href; return; }
  e.preventDefault();
  e.stopImmediatePropagation();
  window.open(u.href,'_blank','noopener');
},true);
}catch(e){}})();`

const (
	marbleSpawnLargura = 240
	marbleSpawnAltura  = 160
	marbleSpawnGap     = 60
)

func marbleTituloDaURL(endereco string) string {
	parseada, erro := url.Parse(endereco)
	if erro != nil || parseada.Hostname() == "" {
		return endereco
	}

	host := parseada.Hostname()
	if len(host) > 4 && host[:4] == "www." {
		host = host[4:]
	}

	return host
}

// marbleSpawnDeLink cria o card filho a direita do card aberto e o conecta.
// Devolve true se criou (para o WebView nao abrir nova aba).
func marbleSpawnDeLink(app *app, endereco string) bool {
	pai := espCanvas.aberto
	if pai < 0 || pai >= len(espCanvas.cartoes) {
		return false
	}

	rectPai := espCanvas.cartoes[pai].rect
	largura := app.sc(marbleSpawnLargura)
	altura := app.sc(marbleSpawnAltura)

	filho := rectT{
		rectPai.Right + app.sc(marbleSpawnGap), rectPai.Top,
		rectPai.Right + app.sc(marbleSpawnGap) + largura, rectPai.Top + altura,
	}

	espCanvas.cartoes = append(espCanvas.cartoes, cardEspacial{
		id:     0,
		rect:   filho,
		titulo: marbleTituloDaURL(endereco),
		url:    endereco,
		cr:     espBordaCard,
	})

	idFilho := len(espCanvas.cartoes) - 1
	espCanvas.foco = idFilho

	boardSincronizar()
	boardConectarFoco(
		espCanvas.cartoes[pai].id,
		espCanvas.cartoes[idFilho].id,
	)

	pInvalidateRect.Call(app.hwnd, 0, 1)

	return true
}
