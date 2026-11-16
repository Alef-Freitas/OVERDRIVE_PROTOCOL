//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// Codepoints Segoe Fluent/MDL2 da tabela estatica de icones.
//
//odlint:somente-leitura tabela estatica de codepoints.
const (
	glifoSetaBaixo      = 0xE70D
	glifoSetaCima       = 0xE70E
	glifoEditar         = 0xE70F
	glifoAdicionar      = 0xE710
	glifoFechar         = 0xE711
	glifoMenu           = 0xE712
	glifoAjustes        = 0xE713
	glifoVideo          = 0xE714
	glifoCorreio        = 0xE715
	glifoPessoas        = 0xE716
	glifoTelefone       = 0xE717
	glifoFixar          = 0xE718
	glifoBloqueioPago   = 0xE719
	glifoParar          = 0xE71A
	glifoElo            = 0xE71B
	glifoFiltrar        = 0xE71C
	glifoAplicativos    = 0xE71D
	glifoZoom           = 0xE71E
	glifoAfastar        = 0xE71F
	glifoMicrofone      = 0xE720
	glifoBuscar         = 0xE721
	glifoCamera         = 0xE722
	glifoAnexo          = 0xE723
	glifoEnviar         = 0xE724
	glifoAcima          = 0xE74A
	glifoAbaixo         = 0xE74B
	glifoVoltarJanela   = 0xE73F
	glifoExpandir       = 0xE740
	glifoEstrela        = 0xE734
	glifoConfirmar      = 0xE73E
	glifoLeitura        = 0xE736
	glifoCompartilhar   = 0xE72D
	glifoImprimir       = 0xE749
	glifoIr             = 0xE751
	glifoVoltar         = 0xE72B
	glifoAvancar        = 0xE72A
	glifoRecarregar     = 0xE72C
	glifoBloqueio       = 0xE72E
	glifoMudo           = 0xE74F
	glifoSalvar         = 0xE74E
	glifoExcluir        = 0xE74D
	glifoSalvarComo     = 0xE792
	glifoDestrancar     = 0xE785
	glifoRemover        = 0xE738
	glifoPrivado        = 0xE727
	glifoNovaJanela     = 0xE78B
	glifoSetaEsquerda   = 0xE76B
	glifoSetaDireita    = 0xE76C
	glifoColar          = 0xE77F
	glifoDesfixar       = 0xE77A
	glifoContato        = 0xE77B
	glifoGlobo          = 0xE774
	glifoAudio          = 0xE767
	glifoYoutube        = 0xE768
	glifoFixado         = 0xE840
	glifoInicio         = 0xE80F
	glifoHistorico      = 0xE81C
	glifoSinalizador    = 0xE7C1
	glifoPagina         = 0xE7C3
	glifoDesfazer       = 0xE7A7
	glifoRefazer        = 0xE7A6
	glifoAviso          = 0xE7BA
	glifoErro           = 0xE783
	glifoEnergia        = 0xE7E8
	glifoConcluir       = 0xE930
	glifoInfo           = 0xE946
	glifoPaineis        = 0xE8A0
	glifoFecharPainel   = 0xE89F
	glifoAproximar      = 0xE8A3
	glifoFavorito       = 0xE8A4
	glifoDocumento      = 0xE8A5
	glifoBaixar         = 0xE896
	glifoSubir          = 0xE898
	glifoPasta          = 0xE8B7
	glifoFigura         = 0xE8B9
	glifoTexto          = 0xE8B3
	glifoPastaAberta    = 0xE838
	glifoPastaNova      = 0xE8F4
	glifoBiblioteca     = 0xE8F1
	glifoConversa       = 0xE8F2
	glifoCopiarLeitura  = 0xE8C8
	glifoRecortar       = 0xE8C6
	glifoDigitalizar    = 0xE8FE
	glifoLista          = 0xE8FD
	glifoMinimizar      = 0xE921
	glifoMaximizar      = 0xE922
	glifoRestaurar      = 0xE923
	glifoFecharJanela   = 0xE8BB
	glifoFerramentas    = 0xE90F
	glifoImagem         = 0xE91B
	glifoVer            = 0xE890
	glifoEscudo         = 0xEA18
	glifoCookies        = 0xEA99
	glifoExtensao       = 0xEA86
	glifoTraduzir       = 0xF2B7
	glifoAbrirPasta     = 0xED25
	glifoBaixarNuvem    = 0xEBD3
	glifoSincronizar    = 0xE895
	glifoRobo           = 0xE99A
	glifoAjustar        = 0xE9A6
	glifoCodigo         = 0xE943
	glifoRadioDesligado = 0xECCA
	glifoRadioLigado    = 0xECCB
	glifoRepouso        = 0xE708
	glifoVPN            = 0xE705
	glifoBloqueado      = 0xE733
)

//odlint:somente-leitura // Tabela de icones estatica: somente leitura.
//nolint:gochecknoglobals // Tabela de icones estatica: somente leitura.
var iconesFluent = map[string]uint16{
	idVoltar:        glifoVoltar,
	idAvancar:       glifoAvancar,
	idRecarregar:    glifoRecarregar,
	idParar:         glifoParar,
	"go":            glifoIr,
	idHome:          glifoInicio,
	idLock:          glifoBloqueio,
	idUnlock:        glifoDestrancar,
	idGlobe:         glifoGlobo,
	"chevrondown":   glifoSetaBaixo,
	"chevronup":     glifoSetaCima,
	idChevronLeft:   glifoSetaEsquerda,
	idChevronRight:  glifoSetaDireita,
	"paste":         glifoColar,
	idClipboard:     glifoColar,
	idImprimir:      glifoImprimir,
	idFerramentas:   glifoFerramentas,
	idYoutube:       glifoYoutube,
	idMenu:          glifoMenu,
	"more":          glifoMenu,
	idPanels:        glifoPaineis,
	idOpenpane:      glifoPaineis,
	"closepane":     glifoFecharPainel,
	idTranslate:     glifoTraduzir,
	idDictionary:    glifoTraduzir,
	idSend:          glifoEnviar,
	idExpand:        glifoExpandir,
	"fullscreen":    glifoExpandir,
	idZoom:          glifoZoom,
	"zoomin":        glifoAproximar,
	"zoomout":       glifoAfastar,
	"fit":           glifoAjustar,
	idPin:           glifoFixar,
	"unpin":         glifoDesfixar,
	"pinned":        glifoFixado,
	idAudio:         glifoAudio,
	"volume":        glifoAudio,
	idMute:          glifoMudo,
	"close":         glifoFechar,
	"cancel":        glifoFechar,
	idPlus:          glifoAdicionar,
	"add":           glifoAdicionar,
	"remove":        glifoRemover,
	"overflow":      glifoMenu,
	"winmin":        glifoMinimizar,
	"winmax":        glifoMaximizar,
	"winrestore":    glifoRestaurar,
	"winclose":      glifoFecharJanela,
	idUbo:           glifoEscudo,
	idShield:        glifoEscudo,
	"scan":          glifoDigitalizar,
	"ocr":           glifoDigitalizar,
	idSave:          glifoSalvar,
	"saveas":        glifoSalvarComo,
	idDelete:        glifoExcluir,
	idTrash:         glifoExcluir,
	"discard":       glifoExcluir,
	"info":          glifoInfo,
	"warning":       glifoAviso,
	"error":         glifoErro,
	"completed":     glifoConcluir,
	"check":         glifoConfirmar,
	idReader:        glifoLeitura,
	"reading":       glifoLeitura,
	idCopiarLeitura: glifoCopiarLeitura,
	idPaywall:       glifoBloqueioPago,
	"tr":            glifoTraduzir,
	idNojs:          glifoBloqueado,
	"blocked":       glifoBloqueado,
	idSleep:         glifoRepouso,
	"quiethours":    glifoRepouso,
	idVPN:           glifoVPN,
	idCookies:       glifoCookies,
	idDownload:      glifoBaixar,
	idDownloads:     glifoBaixar,
	"upload":        glifoSubir,
	"folder":        glifoPasta,
	"folderopen":    glifoPastaAberta,
	"newfolder":     glifoPastaNova,
	idSettings:      glifoAjustes,
	"extension":     glifoExtensao,
	"puzzle":        glifoExtensao,
	"apps":          glifoAplicativos,
	"image":         glifoImagem,
	"photo":         glifoImagem,
	"picture":       glifoFigura,
	"text":          glifoTexto,
	"document":      glifoDocumento,
	idPage:          glifoPagina,
	"bookmark":      glifoFavorito,
	"star":          glifoEstrela,
	"flag":          glifoSinalizador,
	"history":       glifoHistorico,
	idLibrary:       glifoBiblioteca,
	iconNovaJanela:  glifoNovaJanela,
	"private":       glifoPrivado,
	"inprivate":     glifoPrivado,
	idCopy:          glifoCopiarLeitura,
	"cut":           glifoRecortar,
	"undo":          glifoDesfazer,
	"redo":          glifoRefazer,
	idSearch:        glifoBuscar,
	"filter":        glifoFiltrar,
	"list":          glifoLista,
	"view":          glifoVer,
	"share":         glifoCompartilhar,
	idLink:          glifoElo,
	"edit":          glifoEditar,
	idLivreClique:   glifoEditar,
	"people":        glifoPessoas,
	"contact":       glifoContato,
	idRobot:         glifoRobo,
	idChat:          glifoConversa,
	"chatbubbles":   glifoConversa,
	"sparkle":       glifoConversa,
	"chatsparkle":   glifoConversa,
	idPower:         glifoEnergia,
	"signout":       glifoEnergia,
	idRadioOff:      glifoRadioDesligado,
	idRadioOn:       glifoRadioLigado,
	"globe2":        glifoGlobo,
	"openfolder":    glifoAbrirPasta,
	"clouddownload": glifoBaixarNuvem,
	"sync":          glifoSincronizar,
	"up":            glifoAcima,
	"down":          glifoAbaixo,
	"backtowindow":  glifoVoltarJanela,
	"arrowexpand":   glifoExpandir,
	"mail":          glifoCorreio,
	idVideo:         glifoVideo,
	"phone":         glifoTelefone,
	"attach":        glifoAnexo,
	"microphone":    glifoMicrofone,
	idCamera:        glifoCamera,
	idCode:          glifoCodigo,
}

func (a *app) initIconFace() {
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\Windows`
	}

	fonts := filepath.Join(windir, "Fonts")
	a.iconFace = ""
	// GOD: caminho derivado de SystemRoot (config da maquina), nao de entrada externa.
	if _, err := os.Stat(filepath.Join(fonts, "SegoeIcons.ttf")); err == nil { // #nosec G703
		a.iconFace = "Segoe Fluent Icons"
		// GOD: caminho derivado de SystemRoot (config da maquina), nao de entrada externa.
	} else if _, err := os.Stat(filepath.Join(fonts, "segmdl2.ttf")); err == nil { // #nosec G703
		a.iconFace = "Segoe MDL2 Assets"
	}
}
