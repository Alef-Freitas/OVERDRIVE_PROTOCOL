package main

import (
	"errors"
	"fmt"
)

// erroTexto cria erro de UI a partir do catalogo central deste arquivo.
func erroTexto(txt string) error {
	return errors.New(txt) //nolint:err113 // Catalogo unico: 170+ textos, zero errors.Is real; New direto proibido.
}

// erroTextoF e o equivalente formatado (inclusive %w: repassado intacto).
func erroTextoF(formato string, args ...any) error {
	return fmt.Errorf(formato, args...) //nolint:err113 // Formato central, um so sitio; Errof direto proibido.
}

// Catálogo único de textos visíveis do navegador (estilo i18n).
// Lei: nenhum literal de UI fora daqui — menus, dicas, status, avisos,
// nomes de idiomas e formatos com plural vivem neste arquivo. Código
// (JS, URLs, chaves, logs de diagnóstico) continua onde está.
const (
	// --- janela ---.
	txtSufixoPrivado = " · privado"

	// --- instrução de tradução enviada ao chat ---.
	instrucaoTraduzir = "Traduza para português do Brasil o texto desta página, abaixo. " +
		"Responda SOMENTE com a tradução, mantendo os títulos e a estrutura. " +
		"Se algo já estiver em português, mantenha como está.\n\n"

	// --- idioma padrão (sem escolha salva) ---.
	txtIdiomaPadrao = "Português do Brasil"
)

type idiomaTrad struct {
	codigo string
	nome   string
}

//nolint:gochecknoglobals // Idiomas oferecidos: tabela fixa, somente leitura.
var idiomasOferecidos = []idiomaTrad{
	{idiomaPtBR, "Português do Brasil"},
	{"en", "Inglês"},
	{"es", "Espanhol"},
	{"fr", "Francês"},
	{"de", "Alemão"},
	{"it", "Italiano"},
	{"ja", "Japonês"},
	{"ru", "Russo"},
	{"zh-CN", "Chinês simplificado"},
}

// quant devolve "1 <sing>" ou "N <plur>" (textos de UI sem parênteses).
func quant(n int, sing, plur string) string {
	if n == 1 {
		return "1 " + sing
	}

	return formataMilhar(n) + " " + plur
}

const (
	// --- menu "abrir link em" ---.
	txtAbrirLinkEm     = "Abrir o link em:"
	txtAbaNovaEsquerda = "Aba nova à esquerda"
	txtAquiNoChat      = "Aqui no chat"
	txtCancelar        = "Cancelar"
	txtHintEsc         = "Esc"
	txtHintF12         = "F12"
	txtHintRecarregar  = sufAbaRecarregar

	// --- menu de contexto da página ---.
	txtCtxAbrirLinkNova = "Abrir link em nova guia"
	txtCtxCopiarLink    = "Copiar link"
	txtCtxAbrirImgNova  = "Abrir imagem em nova guia"
	txtCtxCopiarURLImg  = "Copiar URL da imagem"
	txtCtxPesquisarSel  = "Pesquisar a seleção"
	txtCtxCopiarSel     = "Copiar seleção"

	// --- menu Ferramentas ---.
	txtSubPaginaWeb      = "Página da web"
	txtSubSiteRede       = "Site e rede"
	txtSubDiagnostico    = "Diagnóstico"
	txtFerramentasCopiar = "Copiar artigo do modo leitura"
	txtModoLeitura       = "Modo leitura Readability"
	txtSairLeitura       = "Sair do modo leitura"
	txtTraduzirPagina    = "Traduzir a página"
	txtVerOriginal       = "Ver a página no idioma original"
	txtSemJS             = "Carregar sem JavaScript"
	txtSonoLeve          = "Sono leve"
	txtVirtAbas          = "Virtualização de abas"
	txtLivreClique       = "Botão direito e copiar/colar livres"
	txtLimparCookies     = "Limpar cookies deste site"
	txtPaywall           = "Paywall: arquivo ou leitor"
	txtVPN               = "VPN WireGuard"
	txtDevTools          = "Ferramentas de desenvolvedor"
	txtLiberarMemoria    = "Liberar memória agora"

	// --- menu de tradução ---.
	txtHdrModoTraducao = "MODO DE TRADUÇÃO"
	txtMotorOverdrive  = "Motor do Overdrive"
	txtGoogleProxy     = "Google Tradutor via proxy"
	txtSufixoAtual     = " · atual"
	txtHdrTraduzirPara = "Traduzir a página para"
	txtModoTrGoogle    = "modo de tradução: Google Tradutor via proxy"
	txtModoTrNativo    = "modo de tradução: motor do Overdrive"
	txtPaywallHTTP     = "paywall: abra uma página pública http ou https primeiro"

	// --- segurança do site ---.
	txtHdrPagInterna    = "PÁGINA INTERNA DO OVERDRIVE"
	txtHdrSegura        = "CONEXÃO SEGURA HTTPS"
	txtHdrNaoSegura     = "CONEXÃO NÃO SEGURA HTTP"
	txtTentarHTTPS      = "Tentar HTTPS"
	txtHdrPermissoes    = "PERMISSÕES DESTE SITE SÓ NESTA SESSÃO"
	txtHintBloqueado    = "Bloqueado"
	txtHintPermitido    = "Permitido"
	txtDicaPagInterna   = "Página interna do Overdrive"
	txtFmtDicaSegura    = "Conexão segura HTTPS: %s"
	txtFmtDicaNaoSegura = "Conexão não segura HTTP: %s - clique para opções"
	txtFmtPermLiberada  = "%s liberado para %s · recarregue a página; vale até fechar o app"
	txtFmtPermBloqueada = "%s bloqueado para %s · recarregue a página; vale até fechar o app"
	txtCopiarEndereco   = "Copiar endereço"
	txtFalhaMenuSeg     = "não consegui abrir o menu de segurança"

	// --- dicas de botões e abas ---.
	txtDicaAbrePainel     = " · abre o painel"
	txtDicaExtensao       = "Extensão · abre o painel"
	txtDicaNovaGuia       = "Nova guia · Ctrl+T"
	txtDicaFecharGuia     = "Fechar guia · Ctrl+W"
	txtDicaReativarSom    = "Reativar o som desta guia"
	txtDicaSilenciar      = "Silenciar esta guia"
	txtFmtGuiasOcultas    = "%d guias ocultas · abrir lista"
	txtDicaSilenciada     = "\nSilenciada · clique no ícone para reativar"
	txtDicaTocando        = "\nReproduzindo áudio · clique no ícone para silenciar"
	txtDicaMinimizar      = "Minimizar"
	txtDicaRestaurar      = "Restaurar · F11"
	txtDicaMaximizar      = "Maximizar · F11"
	txtDicaFechar         = "Fechar"
	txtDicaVoltar         = "Voltar · Alt+seta esquerda"
	txtDicaAvancar        = "Avançar · Alt+seta direita"
	txtDicaParar          = "Parar o carregamento"
	txtDicaRecarregar     = "Recarregar · Ctrl+R"
	txtDicaIrEndereco     = "Ir para o endereço digitado · Enter"
	txtDicaMotorBusca     = "Mecanismo de busca: %s · clique para trocar"
	txtDicaFerramentas    = "Ferramentas: barra de endereço, leitura, print, tradução, cookies e rede"
	txtDicaPrint          = "Print: arraste para selecionar uma área da tela · copiar ou salvar"
	txtDicaClipboard      = "Histórico da área de transferência · Ctrl+Shift+H"
	txtDicaTraduzir       = "Traduzir a página"
	txtDicaPagInicial     = "Página inicial · Alt+Home"
	txtFmtCookiesDom      = "Limpar os cookies de %s"
	txtDicaCookies        = "Limpar os cookies deste domínio"
	txtDicaPaywall        = "Abrir esta página em arquivo ou leitor sem paywall"
	txtDicaLeitura        = "Modo leitura Readability · descarta a página original"
	txtDicaCopiarLeitura  = "Copiar o artigo para a área de transferência · uma entrada"
	txtDicaSemJS          = "Carregar esta página sem JavaScript"
	txtDicaVPN            = "VPN WireGuard · configuração só na memória"
	txtDicaSonoDormindo   = "Sono leve: dormindo agora · clique no conteúdo ou tecla acorda"
	txtDicaSonoLigado     = "Sono leve: ligado"
	txtDicaSonoDeslig     = "Sono leve: desligado"
	txtDicaRecolherPainel = "Recolher o painel da esquerda"
	txtDicaExpandirPainel = "Expandir o painel da esquerda"
	txtDicaSairFullPainel = "Sair da tela cheia do painel"
	txtDicaFullPainel     = "Painel em tela cheia · esconde o resto do navegador"
	txtEspacoAppsTitulo   = "Espaço de apps"
	txtEspacoAppsSub      = "Em breve: programas em Go com Direct2D"
	txtPainelEsqFecheModo = "painel: feche o modo atual antes de usar a tela cheia"
	txtPainelEsqFull      = "painel esquerdo em tela cheia"
	txtPainelEsqNormal    = "painel esquerdo no tamanho normal"
	txtDicaTubeFundo      = "UTube tocando de fundo · ▶ volta a tela, Esc fecha e para"
	txtDicaTubeTela       = "UTube na tela · ▶ esconde e continua tocando"
	txtDicaTube           = "UTube · Ctrl+Shift+Y"
	txtDicaExtensoes      = "Extensões: painéis, fixar e remover"
	txtDicaMenu           = "Menu: texto, ferramentas e extensões"
	txtDicaZoomMenos      = "Diminuir o texto da página · Ctrl −"
	txtDicaZoomMais       = "Aumentar o texto da página · Ctrl +"
	txtDicaZoomReset      = "Voltar ao tamanho padrão 100% · Ctrl 0"
	txtDicaZoomFit        = "Maximizar / restaurar a janela · F11"
	txtDicaSplitChat      = "Arraste para redimensionar  ·  clique para recolher o chat  ·  Ctrl+B"
	txtDicaSplitAbrir     = "Arraste para redimensionar  ·  clique para expandir o chat  ·  Ctrl+B"
	txtDicaOcultarIA      = "Ocultar o AI mode · libera a memória dele"
	txtDicaMostrarIA      = "Mostrar o AI mode"
	txtDicaChatHome       = "Voltar à página inicial do painel de IA"
	txtDicaPanelMenu      = "Assistente e configurações do painel"
	txtDicaPanelReload    = "Recarregar painéis da direita · chat + AI mode · F5"
	txtDicaPanelTrad      = "Traduzir esta página no painel de IA"
	txtDicaPanelSend      = "Enviar a página inteira ao painel de IA"
	txtDicaVoltarNav      = "Voltar a mostrar o navegador"
	txtDicaPanelFull      = "Tela cheia do painel de IA · esconde navegador e AI mode"
	txtDicaEndereco       = "Endereço · Ctrl+L para digitar"

	// --- tradução na página ---.
	txtTraduzindoGoogle = "traduzindo via Google para %s..."
	txtServicoLimitado  = "servico limitou o uso · 429 — aguarde um pouco e tente de novo"
	txtFmtFalhaTraducao = "falha na tradução: %s"
	txtFmtPagTraduzida  = "página traduzida para %s"
	txtFmtPagTrechos    = "página traduzida para %s · %s trechos"
	txtTraducaoCalma    = "o serviço de tradução pediu calma · 429. Tente de novo em %d s"

	// --- painel de IA / chat ---.
	txtEnvioCancelado       = "envio cancelado · painel fechado"
	txtFmtChatOcupado       = "o campo do chat já tem texto · %s car. — apague antes de mandar"
	txtBuscaNativa          = "busca nativa do WebView · Ctrl+F"
	txtFmtSemTexto          = "essa página não tem texto para mandar · %s"
	txtFmtTextoExtraido     = "texto extraído · %s car. - enviando ao chat..."
	txtAbrindoChat          = "abrindo o painel do chat para receber o conteudo..."
	txtFmtTextoLimite       = "texto no chat: %d de %d caracteres · cortei no limite"
	txtFmtTextoEnter        = "texto no chat · %d caracteres — aperte Enter para enviar"
	txtTextoColado          = "copiei o texto da página - cole no chat com Ctrl+V"
	txtFmtNaoEntregue       = "não consegui entregar o texto · %s"
	txtNaoEntregue          = "não consegui entregar o texto"
	txtFmtPainelEsq         = "painel de %s: página aberta na esquerda"
	txtFmtNaoEnviou         = "%s não enviou: o texto ficou no campo"
	txtBuscaFalhou          = "não foi possível abrir a busca nativa"
	txtChatGPTNaoPronto     = "o painel do ChatGPT não está pronto"
	txtNaoEChatGPT          = "o painel atual não é o ChatGPT"
	txtPainelIaIndisponivel = "painel de IA indisponivel"
	txtSeparador            = " · "
	txtChatGPTDados         = "ChatGPT: Controles de dados. Use Excluir todos os chats e confirme na janela do ChatGPT."
	txtFmtPainelNome        = "painel: %s"
	txtImgsAnexadas         = "imagens anexadas; revise e envie no chat"
	txtPagIndisp            = "painel da página indisponível"
	txtFaltaReadability     = "falta lib\\Readability.js"
	txtJaLendo              = "já estou lendo a página..."
	txtJaEnviando           = "já estou enviando a página..."
	txtLendoPagina          = "lendo a página..."
	txtLeituraTravou        = "leitura travou; tente de novo"
	txtPagGrande            = "página grande, ainda lendo..."
	txtNaoLiPagina          = "não consegui ler a página"
	txtNadaMandar           = "nada para mandar"
	txtAbrindoChatEnvio     = "abrindo o painel do chat para enviar o conteudo..."
	txtTextoEnviado         = "texto enviado ao painel de IA; aguardando a resposta"
	txtIAFechado            = "painel de IA fechado"
	txtFmtAbrindoIA         = "abrindo o painel do %s..."
	txtPainelFull           = "painel direito em tela cheia"
	txtPainelNormal         = "painel direito no tamanho normal"
	txtIAAbrindo            = "painel de IA ainda abrindo..."
	txtFmtPainelReinic      = "painel da %s reiniciado"
	txtChatVazioDeep        = "chat vazio: abrindo a pagina do DeepSeek para entrar de novo"
	txtFmtZoom              = "zoom %.0f%%"

	// --- painel de imagens ---.
	txtSemPagImgs       = "sem página para buscar imagens"
	txtPagSemImgs       = "esta página nao tem imagens para anexar"
	txtEscolhaImgs      = "escolha as imagens na janela do painel esquerdo"
	txtFmtLidas         = "lidas %s..."
	txtFmtColadas       = "%s coladas no chat por %s - confira e mande"
	txtImgsRejeitadas   = "o chat não aceitou as imagens · nem colagem, nem campo de anexo"
	txtSiteBloqueiaImgs = "este site bloqueia a leitura das imagens que você escolheu"
	txtFmtColando       = "colando %s em %s..."
	txtSemCampoAnexo    = "o chat ainda não tem campo de anexo · entre na conversa e tente de novo"
	txtNenhumaImg       = "nenhuma imagem para anexar"
	txtFmtNaoAnexei     = "nao consegui anexar: %s"
	txtAIPEsquerda      = "AI mode precisa do painel direito visível"

	// --- cookies do domínio ---.
	txtCookiesSemDom    = "cookies: não há um domínio válido na barra"
	txtCookiesSemPainel = "cookies: nenhum painel pronto"
	txtFmtLimpandoCook  = "limpando cookies de %s..."
	txtFmtLerCookies    = "nao consegui ler os cookies de %s"
	txtFmtCookiesRestam = "%s: apaguei %d cookies, mas %d restaram · o site recria ao carregar"
	txtCookiesNenhum    = "%s: nenhum cookie; limpei o resto do site e recarreguei"
	txtFmtCookiesOK     = "cookies de %s apagados · %d — página recarregada"

	// --- motor WebView2 / painéis ---.
	txtRuntimeSistema  = "runtime do sistema · fixo não encontrado"
	txtFmtRuntimeVelho = "runtime WebView2 desatualizado · %s"
	txtFmtFalhaWV      = "falha no WebView2: %s"
	txtFmtVersaoWV     = "WebView2 %s"
	txtFmtFoco         = "foco: %s"
	txtFmtNaoCarregado = "%s nao carregado"
	txtFmtIndispMotor  = "%s indisponível · motor do app não subiu"
	txtFmtNaoAbri      = "não consegui abrir %s"
	txtAbaFundo        = "nova aba aberta em segundo plano"
	txtWVCaiu          = "WebView2 caiu; feche e reabra o Overdrive"
	txtMotorCaiu       = "O mecanismo de navegacao caiu. Feche e reabra o Overdrive."
	txtProcessoFalhou  = "processo falhou · recarregue"

	// --- impressão / salvar / recarga ---.
	txtSemPagImprimir  = "sem página para imprimir"
	txtGerandoPDF      = "gerando PDF..."
	txtNaoGerouPDF     = "não consegui gerar o PDF"
	txtTituloSalvarPDF = "Salvar PDF como"
	txtPDFCancelado    = "PDF cancelado"
	txtNaoSalvouPDF    = "não consegui salvar o PDF"
	txtFmtPDFSalvo     = "PDF salvo: %s"
	txtSemPagSalvar    = "sem página para salvar"
	txtTituloSalvarPag = "Salvar página como"
	txtSalvarCancelado = "salvar página cancelado"
	txtNaoSalvouPag    = "não consegui salvar a página"
	txtFmtPagSalva     = "página salva: %s"
	txtSemEndCopiar    = "sem endereço para copiar"
	txtFmtEndCopiado   = "endereço copiado: %s"
	txtNaoCopiouEnd    = "não consegui copiar o endereço"
	txtPagAbrindo      = "página ainda abrindo..."
	txtRecSemCache     = "recarregando sem cache"
	txtNenhumaPagRec   = "nenhuma página ativa para recarregar"
	txtRecSemOtim      = "recarregando sem otimizações neste site"
	txtRecSemJS        = "recarregando sem JavaScript"
	txtAguardeEnvio    = "aguarde o envio terminar para recarregar"
	txtFmtRecDir       = "recarregando %s da direita · F5"
	txtNenhumPainelRec = "nenhum painel da direita pronto para recarregar"

	// --- print / OCR ---.
	txtPrintAberto       = "print já aberto"
	txtFmtPrintEm        = "print em %d segundos... · Esc cancela"
	txtFmtPrintEmX       = "print em %d..."
	txtPrintCancelado    = "print cancelado"
	txtPrintRestaure     = "print: restaure a janela primeiro"
	txtPrintPrefix       = "print: "
	txtPrintSemSelecao   = "print: não consegui abrir a seleção"
	txtFalhaLerPrint     = "falha ao ler o print"
	txtFmtPrintCopiado   = "print copiado %d x %d"
	txtOCRPrefix         = "OCR: "
	txtOCRLendo          = "OCR: lendo texto..."
	txtOCRNaoCopiou      = "OCR: não consegui copiar: "
	txtFmtOCRCopiado     = "OCR: %d caracteres copiados"
	txtFmtPrintSalvoEm   = "print salvo em %s"
	txtTituloSalvarPrint = "Salvar print como"

	// --- área de transferência ---.
	txtClipNaoAbriu    = "não consegui abrir o clipboard"
	txtClipVazio       = "Nada copiado nesta sessão."
	txtClipComoUsar    = "Copie texto, imagem ou arquivos em qualquer app."
	txtClipMemoria     = "O histórico fica só na memória e zera ao fechar o Overdrive."
	txtClipDica        = "Clique copia · Espaço marca · Ctrl+Enter envia · Esc fecha"
	txtClipTitulo      = "Área de transferência"
	txtClipVazioChip   = "vazio"
	txtClipLimpar      = "Limpar"
	txtClipConfirmar   = "Confirmar?"
	txtClipTudoMarcado = "tudo marcado"
	txtClipTipoTexto   = "texto"
	txtClipTipoArq     = "arquivo"
	txtClipTipoArqs    = "arquivos"
	txtClipTipoImg     = "imagem"
	txtFmtClipPrev     = "%s · +%d %s"
	txtClipPrefix      = "clipboard: "
	txtClipTxtCopiado  = "texto copiado do histórico"
	txtClipArqCopiado  = "arquivos copiados do histórico"
	txtClipImgCopiada  = "imagem copiada do histórico"

	// --- modo leitura ---.
	txtLeituraSemPag     = "modo leitura: nenhuma página pronta"
	txtLeituraFecheTube  = "modo leitura: feche o UTube primeiro"
	txtLeituraAtivo      = "modo leitura já está ativo; use Voltar ao site"
	txtLeituraHTTP       = "modo leitura: abra uma página http ou https primeiro"
	txtLeituraSemLib     = "modo leitura: falta lib\\Readability.js"
	txtLeituraPrep       = "modo leitura: já estou preparando esta página"
	txtLeituraTempo      = "modo leitura: a página não respondeu em 30 s"
	txtLeituraGerando    = "preparando modo leitura..."
	txtLeituraFalhou     = "modo leitura: não consegui ler a página"
	txtLeituraSemArtigo  = "não achei o artigo"
	txtFmtLeituraMotivo  = "modo leitura: %s"
	txtLeituraGrande     = "modo leitura: artigo grande demais"
	txtLeituraRecarga    = "recarregando a página normal..."
	txtLeituraTitulo     = "Leitura"
	txtLeituraSufTitulo  = " — Leitura"
	txtLeituraVoltar     = "← Voltar ao site"
	txtLeituraTopo       = "Topo"
	txtLeituraResumo     = "Resumo"
	txtLeituraCopiada    = "leitura copiada: uma entrada na área de transferência"
	txtLeituraPecaResumo = "Resuma esta matéria"
	txtFmtLeituraMin     = "%d min de leitura"
	txtFmtLeituraMinMin  = "%d-%d min de leitura"

	// --- downloads ---.
	txtFmtDlInterromp = "download interrompido: %s"
	txtFmtDlRetomando = "retomando download: %s"
	txtFmtDlBaixando  = "baixando: %s"
	txtFmtDlConcluido = "download concluído: %s"
	txtFmtDlPausado   = "download pausado: %s"
	txtFmtDlRetoma    = "retomando: %s"
	txtDlSemPainel    = "painel principal não está pronto para baixar de novo"
	txtDlURLRuim      = "URL original invalida para baixar de novo"
	txtFmtDlDeNovo    = "baixando de novo: %s"
	txtFmtDlRemovido  = "removido da lista: %s"
	txtFmtURLCopiada  = "URL copiada: %s"
	txtDlForaPasta    = "caminho fora da pasta de downloads"
	txtDlSumiu        = "arquivo nao existe mais no disco"
	txtFmtDlCancelado = "download cancelado: %s"
	txtFmtDlLimpos    = "lista de downloads limpa: %s"
	txtDlNadaLimpar   = "nada para limpar na lista de downloads"

	// --- flutuante de extensão ---.
	txtFmtExtNaoCarr = "%s não carregada"
	txtExtNadaAbrir  = "nada a abrir"

	// --- extensões ---.
	txtExtLigueAntes     = "ligue a extensão antes de fixar"
	txtFmtMaxFixadas     = "máximo de %d extensões fixadas"
	txtFmtExtNaoCarr2    = "%s não está carregada · reabra o app"
	txtExtNaoAchada      = "extensão não encontrada"
	txtExtSemNome        = "a extensão"
	txtFmtExtSistema     = "%s é do sistema e não pode ser removida"
	txtExtForaArea       = "pasta fora da área de extensões"
	txtFmtConfirmarExt   = "Remover \"%s\"?\n\nA pasta da extensão será apagada do disco."
	txtFmtExtRemovida    = "%s removida"
	txtFmtNaoRemoviAgora = "não consegui remover %s agora"
	txtFmtNaoRemovi      = "não consegui remover %s"

	// --- UTube ---.
	txtTubeSemServidor = "UTube: o servidor local do app não subiu"
	txtTubeAbrindo     = "abrindo UTube..."
	txtTubeNaoAbriu    = "não consegui abrir o UTube"
	txtTubeAguarde     = "UTube abrindo... aguarde"
	txtTubeNaTela      = "UTube · ▶ esconde e continua tocando, Esc fecha e para"
	txtTubeFundo       = "UTube tocando de fundo · ▶ volta a tela, Esc fecha e para"
	txtTubeFechado     = "UTube fechado — som parado"

	// --- abas ---.
	txtNovaGuia       = "Nova guia"
	txtHintCtrlT      = "Ctrl + T"
	txtReabrirGuia    = "Reabrir guia fechada"
	txtHintCtrlShiftT = "Ctrl + Shift + T"
	txtFixarGuia      = "Fixar esta guia"
	txtDesafixarGuia  = "Desafixar guia"
	txtAtivarGuia     = "Ativar esta guia"
	txtDuplicarGuia   = "Duplicar guia"
	txtRecarregarGuia = "Recarregar guia"
	txtHintCtrlR      = "Ctrl + R"
	txtFecharGuia     = "Fechar esta guia"
	txtHintCtrlW      = "Ctrl + W"
	txtFecharOutras   = "Fechar outras guias"
	txtFecharDireita  = "Fechar guias à direita"
	txtVirtLigada     = "virtualização de abas ligada: trocar de aba recarrega"
	txtVirtDesligada  = "virtualização de abas desligada"
	txtMotorAbrindo   = "motor do navegador ainda abrindo..."
	txtFalhaCriarAba  = "falha ao criar a aba"
	txtSemGuiaFechada = "não há guia fechada para reabrir"
	txtGuiaFixadaMsg  = "guia fixada: desafixe antes de fechar"
	txtGuiaMuda       = "guia silenciada"
	txtGuiaSom        = "som da guia reativado"
	txtGuiaFixada     = "guia fixada"
	txtGuiaDesafixada = "guia desafixada"
	txtNaoAbreJanela  = "não consegui abrir outra janela"
)

// nomes das permissões por site (menu de segurança).
//
//nolint:gochecknoglobals // Permissoes por site: tabela fixa, somente leitura.
var permissoesSite = []struct {
	kind uint32
	nome string
}{
	{1, "Microfone"},
	{2, "Câmera"},
	{3, "Localização"},
	{4, "Notificações"},
	{6, "Área de transferência"},
	{7, "Vários downloads"},
	{9, "Reprodução automática"},
}

const (
	txtTraducaoNaoLi       = "não consegui ler a página"
	txtTraducaoSemTexto    = "não achei texto novo para traduzir nesta página"
	txtFmtTraduzindoN      = "traduzindo %d trechos..."
	txtTraducaoNaoAplica   = "falha ao aplicar a tradução"
	txtTraducaoFalhou      = "não consegui traduzir"
	txtTraducaoFalha       = "falha na tradução"
	txtFmtTraducaoFalhaMsg = "falha na tradução: %s"
)

const (
	// --- loja de extensões ---.
	txtFmtLojaDownload  = "download: %s"
	txtLojaRecusou      = "a loja recusou o download · extensão pode ter saído da loja"
	txtLojaNaoAchada    = "extensão não encontrada na loja"
	txtFmtLojaHTTP      = "a loja respondeu HTTP %d"
	txtLojaGrande       = "arquivo grande demais · acima de 150MB"
	txtFmtLojaInterromp = "download interrompido: %s"
	txtLojaVazia        = "a loja devolveu um arquivo vazio"
	txtLojaCRXNaoPacote = "não é um pacote de extensão .crx válido"
	txtLojaCRXTruncado  = "cabeçalho .crx truncado"
	txtFmtLojaCRXVersao = "versão .crx %d não suportada"
	txtLojaCRXInvalido  = "cabeçalho .crx inválido"
	txtLojaSemZip       = "conteúdo zip não encontrado no pacote"
	txtLojaManifestRuim = "manifest.json inválido"
	txtFmtLojaManifestV = "manifest_version %d não suportado"
	txtLojaManifestSeco = "manifest.json sem nome/versão"
	txtLojaSemManifest  = "pacote sem manifest.json na raiz"
	txtFmtLojaSuspeito  = "pacote com caminho suspeito · %q"
	txtLojaInstalando   = "já há uma instalação em andamento..."
	txtLojaAbraPagina   = "abra a página da extensão na loja antes de instalar"
	txtLojaBaixando     = "baixando extensão..."
)

const (
	// --- extensões / contas ---.
	txtFmtUboErro     = "uBlock: %s"
	txtChatNaoPronto  = "o painel do chat ainda não está pronto"
	txtSemContaLogin  = "nenhuma conta cadastrada em data\\login.json"
	txtFmtTrocaConta  = "conta do DeepSeek: %s - saindo da anterior e entrando de novo"
	txtNaoTrocouConta = "não foi possível trocar de conta"
	txtFmtRotuloConta = "%s · %s"
	txtMotorFora      = "motor fora — reabra o app para aplicar"
	txtFmtNaoMudei    = "não consegui mudar %s"
	txtFmtLigada      = "%s ligada"
	txtFmtDesligada   = "%s desligada"
)

const (
	// --- contas / boot / fatal ---.
	txtNenhumaConta    = "Nenhuma conta cadastrada"
	txtFmtErroFatal    = "O Overdrive encontrou um erro e vai fechar.\n\n%s"
	txtFmtFalhaBoot    = "Não foi possível iniciar o Overdrive. Verifique config\\overdrive.json.\n\n%s"
	txtTituloConfig    = "Overdrive - configuração"
	txtBoasVindas      = "bem-vindo ao Overdrive: Ctrl+L usa a barra de endereço"
	txtPlaceholderAddr = "Pesquise ou digite um endereço"
	txtBadgePrivado    = "PRIVADO"
)

//nolint:gochecknoglobals // Motores de busca: tabela fixa, somente leitura.
var motoresBusca = []motorBusca{
	{chaveGoogle, "Google", "G", "https://www.google.com/search?q="},
	{"bing", "Bing", "B", "https://www.bing.com/search?q="},
	{"duckduckgo", "DuckDuckGo", "D", "https://duckduckgo.com/?q="},
	{"startpage", "Startpage", "S", "https://www.startpage.com/sp/search?query="},
	{"wikipedia", "Wikipedia", "W", "https://pt.wikipedia.org/w/index.php?search="},
	{idYoutube, "YouTube", "Y", "https://www.youtube.com/results?search_query="},
	{"github", "GitHub", "Gh", "https://github.com/search?q="},
	{"perplexity", "Perplexity", "P", "https://www.perplexity.ai/search?q="},
}

const (
	// --- buscador ---.
	txtHdrBuscador     = "Pesquisar com"
	txtHintAtual       = "atual"
	txtBuscadorFalhou  = "não consegui abrir o menu de buscadores"
	txtFmtPesquisarCom = "pesquisar com %s"
)

const (
	// --- ações da barra ---.
	txtExtLigadaReabra   = "extensão ligada — reabra o app para carregar"
	txtExtLigando        = "ligando a extensão..."
	txtSemPagPronta      = "sem página pronta"
	txtPagAbrindoAcao    = "página ainda abrindo..."
	txtSemHistorico      = "sem página no histórico"
	txtAbaTentandoDeNovo = "aba presa — tentando abrir de novo..."

	// --- gerenciador de senhas ---.
	txtTituloSenhas        = "Senhas"
	txtFmtSenhaSalvar      = "salvar senha de %s?"
	txtFmtSenhaAtualizar   = "atualizar senha de %s?"
	txtFmtSenhaSalva       = "senha salva para %s"
	txtFmtSenhaNaoSalvei   = "não salvei a senha de %s"
	txtFmtSenhaNaoAchada   = "sem senha salva para %s"
	txtFmtSenhaPreenchida  = "login preenchido em %s"
	txtFmtSenhaDisponivel  = "senha salva para %s — abra o menu senhas"
	txtFmtSenhaApagada     = "senha de %s apagada"
	txtFmtSenhaNaoApaguei  = "não apaguei a senha de %s"
	txtFmtSenhaCopiada     = "senha de %s copiada"
	txtFmtUsuarioCopiado   = "usuário de %s copiado"
	txtNaoCopiouSenha      = "não copiei"
	txtSenhasAutoLigado    = "preenchimento automático ligado"
	txtSenhasAutoDesligado = "preenchimento automático desligado"
	txtHdrSenhas           = "Senhas salvas"
	txtSenhasAuto          = "Preencher sozinho"
	txtSenhasPreencher     = "Preencher login"
	txtSenhasCopiarUsuario = "Copiar usuário"
	txtSenhasCopiarSenha   = "Copiar senha"
	txtSenhasApagar        = "Apagar"
	txtSenhasVazio         = "Nenhuma senha salva"
	txtSenhasVer           = "Senhas salvas"

	// --- janelas nativas ---.
	txtTituloPrint    = "Overdrive print"
	txtTituloClip     = "Overdrive clipboard"
	txtFmtTituloFloat = "Overdrive — %s"

	// --- alternar tradução ---.
	txtSemPagTraduzir  = "sem página para traduzir"
	txtJaTraduzindo    = "já estou traduzindo esta página..."
	txtPagOriginal     = "página no idioma original"
	txtNaoAbriuTradG   = "não consegui abrir a tradução do Google para esta página"
	txtProcurandoTexto = "procurando o texto da página para traduzir..."
)

const (
	// --- menu de extensões ---.
	txtHdrExtensoes     = "Extensões"
	txtExtOpcoes        = "Opções"
	txtExtDesativar     = "Desativar"
	txtExtDesligada     = "extensão desligada"
	txtExtRemoverItem   = "Remover"
	txtHintDesligada    = "desligada"
	txtHintCarregando   = "carregando..."
	txtHintSemPainel    = "sem painel"
	txtExtNenhuma       = "Nenhuma extensão instalada"
	txtExtAbrindoPainel = "abrindo o painel da extensão..."

	// --- atualizacao de extensoes ---.
	txtExtVerificarUpdate  = "Verificar atualização"
	txtFmtUpdateAplicar    = "Atualizar para %s"
	txtUpdateVerificando   = "verificando atualização..."
	txtUpdateSemFonte      = "sem fonte de atualização para esta extensão"
	txtUpdateSemPendente   = "verifique primeiro para achar a versão nova"
	txtFmtUpdateErro       = "atualização: %s"
	txtFmtUpdateJaAtual    = "já está atualizada (%s)"
	txtFmtUpdateDisponivel = "atualização disponível: %s (abra o menu de novo para aplicar)"
	txtFmtUpdateBaixando   = "baixando atualização %s..."
	txtFmtUpdateOk         = "extensão atualizada para %s"
	txtFmtUpdateFalhou     = "falha ao atualizar: %s"

	// --- domínios por extensão ---.
	txtDomSempre            = "sempre ativa"
	txtFmtDomSoEm           = "só em %s"
	txtFmtDomExcetoEm       = "exceto em %s"
	txtDomBaseRuim          = "extensão desconhecida"
	txtDomNaoSistema        = "uBlock vale em todo lugar"
	txtDomPatRuim           = "domínio inválido (ex.: exemplo.com, *.exemplo.com)"
	txtFmtDomSalva          = "%s: %s"
	txtDomSecao             = "Funciona em"
	txtDomSempreItem        = "Em todos os sites"
	txtFmtDomSoNeste        = "Só em %s"
	txtFmtDomSoNesteSub     = "Só em %s e subdomínios"
	txtFmtDomExcetoNeste    = "Exceto em %s"
	txtFmtDomExcetoNesteSub = "Exceto em %s e subdomínios"

	// --- memória ---.
	txtMemLiberada  = "memória liberada"
	txtMemLiberando = "liberando memória..."

	// --- segurança: ações ---.
	txtSegNaoHTTP    = "esta página não está em HTTP"
	txtSegSemEnd     = "não há endereço para copiar"
	txtSegEndCopiado = "endereço copiado"
	txtSegNaoCopiou  = "não consegui copiar o endereço"
	txtSegAbertoGuia = "aberto em nova guia"
	txtSegCopiado    = "copiado"

	// --- loja: instalar ---.
	txtFmtLojaFalhaInst   = "falha ao instalar: %s"
	txtFmtLojaBaixandoPct = "baixando extensão... %d%%"
	txtLojaExtraindo      = "extraindo extensão..."
	txtFmtLojaInstalada   = "extensão %s %s instalada"
	txtFmtLojaJaInstalada = "extensão %s %s já instalada"
	txtFmtLojaSemPainel   = "extensão %s %s instalada · sem painel (funciona em segundo plano)"

	// --- entrada / VPN / lembrete ---.
	txtPagAbrindoInput = "página ainda abrindo..."
	txtEndTelaCheia    = "endereço indisponível em tela cheia"
	txtVPNIndisp       = "VPN indisponível"
	txtFmtVPNErr       = "VPN: %v"
	txtVPNLigada       = "VPN conectada"
	txtVPNDesligada    = "VPN desconectada"
	txtUboNaoCarr2     = "uBlock não carregado"
	txtMotorCaiuDizer  = "O mecanismo de navegação caiu. Feche e reabra o Overdrive."
	txtAgua            = "Hora de beber água — faça uma pausa e hidrate-se"
)

const (
	// --- menu do painel de IA / principal ---.
	txtHdrAssistente       = "ASSISTENTE"
	txtHdrContaDS          = "CONTA DEEPSEEK · "
	txtHdrContaVazia       = "CONTA DEEPSEEK · SEM CONTAS CONFIGURADAS"
	txtApagarChatsGPT      = "Apagar todos os chats do ChatGPT..."
	txtHintAbreDados       = "abre Controles de dados"
	txtDuplicarAtual       = "Duplicar guia atual"
	txtFecharAtual         = "Fechar guia atual"
	txtNovaJanela          = "Nova janela"
	txtHintCtrlN           = "Ctrl + N"
	txtNovaPrivada         = "Nova janela privada"
	txtHintCtrlShiftN      = "Ctrl + Shift + N"
	txtAbrirDownloads      = "Abrir downloads"
	txtSubGuias            = "Guias"
	txtSubJanelas          = "Janelas"
	txtSubBiblioteca       = "Biblioteca"
	txtSubPagAtual         = "Página atual"
	txtSubSalvarPagina     = "Salvar conteúdo da página"
	txtSubSalvarPagina2    = "Salvar página"
	txtSelecionarTudo      = "Selecionar tudo"
	txtImprimir            = "Imprimir..."
	txtHintCtrlP           = "Ctrl + P"
	txtSalvarPagHTML       = "Salvar página como HTML"
	txtSalvarPagFull       = "Salvar página completa"
	txtSalvarPagUnica      = "Salvar arquivo único"
	txtSalvandoUnico       = "gerando arquivo único..."
	txtSalvandoAlternativo = "abrindo diálogo do navegador..."
	txtAvisoFalhaTitulo    = "salvar único: fallback usado"
	txtAvisoCopiar         = "copiar conteúdo"
	txtAvisoDispensar      = "dispensar"
	txtHintCtrlS           = "Ctrl + S"
	txtRecSemCacheM        = "Recarregar sem cache"
	txtHintCtrlShiftR      = "Ctrl + Shift + R"
	txtRecSemOtimM         = "Recarregar sem otimizações"
	txtSubPrivacidade      = "Privacidade e dados"
	txtCockpitDados        = "Cookies, cache e dados…"
	txtHintCtrlShiftDel    = "Ctrl + Shift + Del"
	txtAbrirPastaDados     = "Abrir pasta de dados"
	txtConfiguracoes       = "Configurações"
	txtSair                = "Sair"
)

const (
	// --- menu captura / paywall / feedback ---.
	txtHdrCaptura      = "CAPTURA DE TELA"
	txtCapturarAgora   = "Capturar agora"
	txtHintPadrao      = "padrão"
	txtPrintEm1        = "Em 1 segundo"
	txtFmtPrintEmN     = "Em %d segundos"
	txtHdrPaywallAbrir = "Abrir a página atual em:"
	txtSubArquivos     = "Arquivos da web"
	txtHintArquivo     = "arquivo"
	txtSubLeitores     = "Leitores sem paywall"
	txtHintLeitor      = "leitor"
	txtHintArquivos    = "arquivos"
	txtSubTextoLimpo   = "Texto limpo e impressão"
	txtHintTexto       = "texto"
	txtHintImpressao   = "impressão"
	txtPaywallSemMenu  = "paywall: não consegui abrir o menu"
	txtPaywallSemURL   = "paywall: não há uma URL válida selecionada"
	txtMenuNaoAbriu    = "não consegui abrir o menu"
	txtMenuIdiomas     = "não consegui abrir o menu de idiomas"
	txtZoomRotulo      = "Zoom"
	txtFmtZoomPct      = "%.0f%%"
)

const (
	// --- janela de configuração nativa ---.
	txtCfgTitulo             = "Configurações"
	txtCfgSecDown            = "Downloads"
	txtCfgSecTrad            = "Tradução"
	txtCfgSecCfg             = "Configurações"
	txtCfgSecDados           = "Dados do navegador"
	txtCockpitCookies        = "Cookies do navegador"
	txtCockpitFiltro         = "filtrar por domínio ou nome"
	txtCockpitRecarregar     = "Recarregar lista"
	txtCockpitApagarTodos    = "Apagar todos os cookies"
	txtCockpitApagarDom      = "apagar domínio"
	txtCockpitVazio          = "nenhum cookie no perfil"
	txtCockpitCarregando     = "lendo cookies…"
	txtCockpitApagando       = "apagando cookies…"
	txtCockpitLimpando       = "limpando…"
	txtCockpitSemPainel      = "sem painel pronto para ler os dados"
	txtCockpitSessao         = "sessão"
	txtFmtCockpitResumo      = "%d cookies em %d sites · %s"
	txtFmtCockpitCookie      = "%s · %s · %s"
	txtFmtCockpitMaisCookies = "mostrando %d de %d cookies…"
	txtCockpitCache          = "Cache do navegador"
	txtCockpitLimparCache    = "Limpar cache agora"
	txtCockpitNotaCache      = "Limpa arquivos temporários das páginas. Não encerra sessões."
	txtCockpitSite           = "Dados por site"
	txtCockpitLimparSite     = "limpar site"
	txtFmtCockpitMaisOrigens = "e mais %d sites na lista de cookies…"
	txtCockpitNotaSite       = "Limpa cookies, armazenamento local e cache do site. Encerra a sessão nele."
	txtCockpitTudo           = "Tudo de uma vez"
	txtCockpitApagarTudo     = "Apagar cookies e cache"
	txtCockpitNotaTudo       = "Apaga todos os cookies e o cache do perfil. Encerra todas as sessões."
	txtCockpitNota           = "Apagar cookies encerra as sessões dos sites. O perfil salvo no disco não é apagado."
	txtCfgSecVPN             = "VPN"
	txtCfgFiltro             = "filtrar por nome"
	txtCfgPastaDown          = "Abrir a pasta"
	txtCfgLimparDown         = "Limpar lista"
	txtCfgDownVazio          = "Nada baixado nesta sessão."
	txtCfgDownSemFiltro      = "Nada para esse filtro nesta sessão."
	txtDownCopiarURL         = "Copiar URL"
	txtDownPausar            = "Pausar"
	txtDownCancelar          = "Cancelar"
	txtDownRetomar           = "Retomar"
	txtDownRemover           = "Remover da lista"
	txtDownAbrir             = "Abrir arquivo"
	txtDownRevelar           = "Revelar na pasta"
	txtDownTentar            = "Tentar de novo"
	txtDownCancelado         = "Cancelado"
	txtDownConcluido         = "Concluído"
	txtDownInterromp         = "Interrompido"
	txtDownPausado           = "Pausado"
	txtDownBaixando          = "Baixando"
	txtCfgIdioma             = "Idioma de destino"
	txtCfgSalvarTrad         = "Salvar tradução"
	txtCfgTradSalva          = "Tradução salva."
	txtCfgTradNota           = "O mecanismo de tradução usa o Google, é rápido, não exige conta ou chave " +
		"e envia apenas os trechos de texto necessários. O idioma escolhido vale para o botão de " +
		"tradução na barra e para a opção Traduzir página do menu de IA."
	txtCfgZoomTexto   = "Tamanho do texto das páginas"
	txtCfgZoomIA      = "Tamanho da fonte do painel de IA"
	txtCfgDark        = "Forçar modo escuro nas páginas"
	txtCfgChat        = "Mostrar o painel de IA ao abrir"
	txtCfgSemSW       = "Desativar service workers · reduz o uso de memória"
	txtCfgSemGPU      = "Desativar aceleração gráfica · vídeos podem usar mais CPU"
	txtCfgEcoMax      = "Economia máxima · congela o ChatGPT ocioso e reduz processos sem GPU"
	txtCfgLimpezaAcao = "Limpar memória a cada ação · roda a limpeza pesada após cada clique/atalho"
	txtCfgLimparClip  = "Apagar o clipboard 60 s após copiar"
	txtCfgPastaDados  = "Abrir pasta de dados"
	txtCfgNota        = "Ajustes de leitura, memória e aparência. " +
		"Algumas mudanças valem a partir da próxima página aberta."
	txtCfgConectar    = "Conectar"
	txtCfgDesconectar = "Desconectar"
	txtCfgKill        = "Kill switch · sem túnel, sem rede"
	txtCfgVPNNota     = "Cole a configuração WireGuard do seu provedor. Ela fica apenas na memória e é perdida " +
		"ao fechar o Overdrive. A VPN vale somente para o Overdrive."
	txtCfgVPNDNS = "DNS seguro já vem ligado por padrão. Bloqueios por IP exigem uma configuração " +
		"WireGuard com servidor e chaves."
	txtCfgVPNStOff  = "Desconectado"
	txtCfgVPNStOn   = "Conectado"
	txtCfgVPNStGo   = "Conectando..."
	txtCfgVPNSemCfg = "sem configuração na memória"
)

const (
	// --- links / cookies / sono ---.
	txtLinkAbaNova          = "link aberto em aba nova"
	txtLinkCancelado        = "ação cancelada"
	txtLinkPainelIA         = "link aberto no painel de IA"
	txtLinkNaoAberto        = "link não aberto"
	txtCookiesApagados      = "cookies apagados: entre de novo no chat do DeepSeek"
	txtDadosApagados        = "dados de navegação apagados"
	txtSonoLigadoSt         = "sono leve ligado"
	txtSonoDesligadoSt      = "sono leve desligado"
	txtLivreLigadoSt        = "botão direito e copiar/colar liberados"
	txtLivreDesligadoSt     = "botão direito e copiar/colar bloqueados"
	txtForceDarkLigadoSt    = "modo escuro ligado"
	txtForceDarkDesligadoSt = "modo escuro desligado"
	txtFmtWVFatal           = "Não foi possível iniciar o WebView2.\r\n\r\n%s\r\n\r\nVerifique se:\r\n" +
		"- a pasta runtime\\WebView2 contém msedgewebview2.exe;\r\n" +
		"- nenhuma outra cópia do Overdrive está aberta;\r\n" +
		"- a pasta data pode ser escrita."

	// --- overlay do recorte ---.
	txtRecorteArraste = "Arraste para selecionar a área   ·   Esc cancela"
	txtFmtRecorteTam  = "%d × %d"
	txtRecorteCopiar  = "Copiar"
	txtRecorteOCR     = "OCR"
	txtRecorteSalvar  = "Salvar..."
	txtRecorteDescart = "Descartar"
	txtRecorteTeclas  = "C copia  ·  O OCR  ·  S salva  ·  Esc descarta"

	// --- VPN (motor) ---.
	txtFmtProxyLocal   = "proxy local: %s"
	txtTunelDireto     = "tunel fora do ar: trafego direto · kill switch desligado"
	txtVPNCancelada    = "conexao cancelada"
	txtFmtTunelErr     = "tunel: %s"
	txtFmtTunelCfg     = "configuracao do tunel: %s"
	txtFmtTunelUp      = "subir tunel: %s"
	txtVPNSemTunel     = "VPN desconectada · kill switch ativo"
	txtVPNSemDNS       = "config sem DNS · necessario para o tunel"
	txtFmtAvisoPagina  = "%s car. -> chat [%s]"
	txtAvisoArtigo     = "artigo"
	txtAvisoPagInteira = "página inteira"
	txtFmtAvisoFalhou  = "aviso na página: %v"
	txtAvisoNaoConf    = "resposta inesperada do script"
)
