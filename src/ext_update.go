//go:build windows

package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	tempoLimiteUpdateExtSeg = 20
	pastaTmpUpdateExt       = "tmp"
	sufixoReservaUpdateExt  = ".bak"
	nomeManifestoUpdateExt  = "manifest.json"
	sufixoZipUpdateExt      = ".zip"
	maxBytesPacoteUpdateExt = 256 << 20
	maxRespostaReleaseExt   = 1 << 20
)

// estadoUpdateExt guarda uma atualizacao encontrada para a entrada.
type estadoUpdateExt struct {
	nova    string
	url     string
	arquivo string
}

// ativoReleaseExt e um arquivo publicado na release do git.
//
//nolint:tagliatelle // Contrato da API do GitHub: snake_case e o formato oficial.
type ativoReleaseExt struct {
	Nome string `json:"name"`
	Link string `json:"browser_download_url"`
}

// lancamentoReleaseExt e a ultima release consultada no git.
//
//nolint:tagliatelle // Contrato da API do GitHub: snake_case e o formato oficial.
type lancamentoReleaseExt struct {
	Tag    string            `json:"tag_name"`
	Ativos []ativoReleaseExt `json:"assets"`
}

// fonteConhecidaExt resolve repo e ativo embutidos para extensoes conhecidas.
// Nem toda pasta traz repo: o mapeado aqui cobre o essencial.
func fonteConhecidaExt(base string) (string, string, bool) {
	if strings.ToLower(strings.TrimSpace(base)) == idUbo {
		return "gorhill/uBlock", "uBlock0.chromium.zip", true
	}

	return "", "", false
}

// fonteUpdateExtensao resolve de onde verificar: update.json da pasta ou
// mapa interno. Pasta sem fonte nao tem de onde atualizar.
func (a *app) fonteUpdateExtensao(base string) (string, string) {
	if repo, ativo := a.fonteArquivoUpdateExt(base); repo != "" {
		return repo, ativo
	}

	if repo, ativo, ok := fonteConhecidaExt(base); ok {
		return repo, ativo
	}

	return "", ""
}

// lerJSONPequenoExt le manifesto/update.json com teto anti-bomba.
func lerJSONPequenoExt(caminho string) ([]byte, bool) {
	const maxJSONExt = 256 << 10

	info, err := os.Stat(caminho)
	if err != nil || info.IsDir() || info.Size() > maxJSONExt {
		return nil, false
	}

	dados, err := os.ReadFile(caminho) // #nosec G304
	if err != nil || len(dados) > maxJSONExt {
		return nil, false
	}

	return dados, true
}

// fonteArquivoUpdateExt le o update.json opcional da pasta da extensao.
func (a *app) fonteArquivoUpdateExt(base string) (string, string) {
	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return "", ""
	}

	dados, ok := lerJSONPequenoExt(filepath.Join(extDir, orig, "update.json"))
	if !ok {
		return "", ""
	}

	var fonte struct {
		Repo  string `json:"repo"`
		Ativo string `json:"asset"`
	}

	if json.Unmarshal(dados, &fonte) != nil {
		return "", ""
	}

	return strings.TrimSpace(fonte.Repo), strings.TrimSpace(fonte.Ativo)
}

// versaoLocalExtensao le a versao instalada no manifesto.
func (a *app) versaoLocalExtensao(base string) string {
	orig := a.pastaRealExtensao(base)
	if orig == "" {
		return ""
	}

	dados, ok := lerJSONPequenoExt(filepath.Join(extDir, orig, "manifest.json"))
	if !ok {
		return ""
	}

	var manifesto extManifest
	if json.Unmarshal(dados, &manifesto) != nil {
		return ""
	}

	return strings.TrimSpace(manifesto.Version)
}

// limparVersaoExt tira prefixo v e espacos da tag.
func limparVersaoExt(versao string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(versao), "v"))
}

// numeroInicialVersao extrai os digitos iniciais do segmento.
func numeroInicialVersao(segmento string) (int, string) {
	segmento = strings.TrimSpace(segmento)
	digitos := 0

	for digitos < len(segmento) && segmento[digitos] >= '0' && segmento[digitos] <= '9' {
		digitos++
	}

	if digitos == 0 {
		return 0, segmento
	}

	numero, _ := strconv.Atoi(segmento[:digitos])

	return numero, segmento[digitos:]
}

// compararVersoesExt compara pontilhadas: 1 se nova maior, -1 se menor, 0 igual.
// Sufixo sem numero (ex. "-beta") perde da versao limpa: 1.0 > 1.0-beta.
func compararVersoesExt(nova, atual string) int {
	novas := strings.Split(limparVersaoExt(nova), ".")
	atuais := strings.Split(limparVersaoExt(atual), ".")

	for indice := range min(len(novas), len(atuais)) {
		valorNovo, restoNovo := numeroInicialVersao(novas[indice])
		valorAtual, restoAtual := numeroInicialVersao(atuais[indice])

		if valorNovo != valorAtual {
			if valorNovo > valorAtual {
				return 1
			}

			return -1
		}

		if restoNovo != restoAtual {
			if restoNovo == "" {
				return 1
			}

			if restoAtual == "" {
				return -1
			}

			if restoNovo > restoAtual {
				return 1
			}

			return -1
		}
	}

	if len(novas) != len(atuais) {
		if len(novas) > len(atuais) {
			return 1
		}

		return -1
	}

	return 0
}

// novaVersaoUpdateExt devolve a versao pendente da entrada, se houver.
func (a *app) novaVersaoUpdateExt(base string) string {
	if a == nil || a.extUpdate == nil {
		return ""
	}

	return a.extUpdate[strings.ToLower(strings.TrimSpace(base))].nova
}

// guardarUpdateExt registra o achado para o submenu oferecer aplicar.
func (a *app) guardarUpdateExt(base, versao, url, arquivo string) {
	if a.extUpdate == nil {
		a.extUpdate = map[string]estadoUpdateExt{}
	}

	a.extUpdate[strings.ToLower(strings.TrimSpace(base))] = estadoUpdateExt{nova: versao, url: url, arquivo: arquivo}
}

// verificarUpdateExtensao inicia a consulta a ultima release no git.
func (a *app) verificarUpdateExtensao(pasta string) {
	base := strings.ToLower(strings.TrimSpace(pasta))
	if base == "" {
		return
	}

	repo, ativo := a.fonteUpdateExtensao(base)
	if repo == "" {
		a.dizerErrof("%s", txtUpdateSemFonte)

		return
	}

	a.dizerf("%s", txtUpdateVerificando)
	go a.consultarReleaseExt(base, repo, ativo)
}

// clienteUpdateExt monta o HTTP so com timeout; resto padrao.
func clienteUpdateExt() *http.Client {
	//exhaustruct:ignore // Cliente HTTP de update: so timeout custom; resto padrao do Go.
	cli := &http.Client{Timeout: tempoLimiteUpdateExtSeg * time.Second}

	return cli
}

// consultarReleaseExt busca a ultima release e anuncia o resultado na UI.
func (a *app) consultarReleaseExt(base, repo, ativo string) {
	versao, url, arquivo, err := ultimaReleaseExt(repo, ativo)

	a.depois(func() {
		if a.quitting {
			return
		}

		if err != nil {
			a.dizerErrof(txtFmtUpdateErro, err.Error())

			return
		}

		local := a.versaoLocalExtensao(base)
		if compararVersoesExt(versao, local) <= 0 {
			a.dizerf(txtFmtUpdateJaAtual, local)

			return
		}

		a.guardarUpdateExt(base, versao, url, arquivo)
		a.dizerf(txtFmtUpdateDisponivel, versao)
	})
}

// ultimaReleaseExt devolve versao, url e nome do pacote da ultima release.
func ultimaReleaseExt(repo, ativo string) (string, string, string, error) {
	endereco := "https://api.github.com/repos/" + repo + "/releases/latest"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endereco, nil)
	if err != nil {
		return "", "", "", erroTextoF("montar consulta: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", uaChromeLoja)

	resp, err := clienteUpdateExt().Do(req)
	if err != nil || resp == nil {
		return "", "", "", erroTextoF("consultar git: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", erroTextoF("git respondeu %d", resp.StatusCode)
	}

	var lancamento lancamentoReleaseExt

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespostaReleaseExt)).Decode(&lancamento); err != nil {
		return "", "", "", erroTextoF("ler resposta: %w", err)
	}

	versao := strings.TrimSpace(lancamento.Tag)
	if versao == "" {
		return "", "", "", erroTexto("release sem versao")
	}

	nome, url := escolherAtivoUpdateExt(lancamento.Ativos, ativo)
	if url == "" {
		return "", "", "", erroTexto("release sem pacote chromium")
	}

	return versao, url, nome, nil
}

// escolherAtivoUpdateExt prefere o nome exato, depois chromium, senao o primeiro zip.
func escolherAtivoUpdateExt(ativos []ativoReleaseExt, ativo string) (string, string) {
	for _, candidato := range ativos {
		if ativo != "" && candidato.Nome == ativo {
			return candidato.Nome, candidato.Link
		}
	}

	for _, candidato := range ativos {
		nome := strings.ToLower(candidato.Nome)
		if strings.HasSuffix(nome, sufixoZipUpdateExt) && strings.Contains(nome, "chromium") {
			return candidato.Nome, candidato.Link
		}
	}

	for _, candidato := range ativos {
		if strings.HasSuffix(strings.ToLower(candidato.Nome), sufixoZipUpdateExt) {
			return candidato.Nome, candidato.Link
		}
	}

	return "", ""
}

// aplicarUpdateExtensao baixa o pacote pendente e troca a pasta.
func (a *app) aplicarUpdateExtensao(pasta string) {
	base := strings.ToLower(strings.TrimSpace(pasta))
	if base == "" || a.extUpdate == nil {
		return
	}

	pendente := a.extUpdate[base]
	if pendente.url == "" {
		a.dizerErrof("%s", txtUpdateSemPendente)

		return
	}

	a.dizerf(txtFmtUpdateBaixando, pendente.nova)
	go a.baixarAplicarUpdateExt(base, pendente)
}

// baixarAplicarUpdateExt baixa, valida, troca a pasta e reinstala.
func (a *app) baixarAplicarUpdateExt(base string, pendente estadoUpdateExt) {
	orig := a.pastaRealExtensao(base)
	ligada := a.extensaoLigada(base)

	err := trocarPastaUpdateExt(orig, pendente)

	a.depois(func() {
		if a.quitting {
			return
		}

		if err != nil {
			a.dizerErrof(txtFmtUpdateFalhou, err.Error())

			return
		}

		delete(a.extUpdate, base)
		a.reinstalarExtensaoAusente(orig, base, ligada)
		a.dizerf(txtFmtUpdateOk, pendente.nova)
	})
}

// trocarPastaUpdateExt baixa o pacote, valida e troca com reserva.
func trocarPastaUpdateExt(orig string, pendente estadoUpdateExt) error {
	base, ok := baseExtensaoValida(orig)
	if !ok || orig == "" {
		return erroTexto("pasta desconhecida")
	}

	dir := filepath.Join(extDir, orig)
	reserva := dir + sufixoReservaUpdateExt

	pacote, err := baixarPacoteUpdateExt(pendente.url, base)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pacote) }()

	if err := validarPacoteUpdateExt(pacote, pendente.nova); err != nil {
		return err
	}

	_ = os.RemoveAll(reserva)
	if err := os.Rename(dir, reserva); err != nil {
		return erroTextoF("reservar atual: %w", err)
	}

	if err := extrairPacoteUpdateExt(pacote, dir); err != nil {
		_ = os.RemoveAll(dir)
		if rerr := os.Rename(reserva, dir); rerr != nil {
			return erroTextoF("extrair: %v; reserva: %v", err, rerr)
		}

		return err
	}

	return nil
}

// nomePacoteUpdateExt monta o nome do zip temporario por extensao.
func nomePacoteUpdateExt(base string) string {
	saneada, ok := baseExtensaoValida(base)
	if !ok {
		saneada = "ext"
	}

	return "update-" + saneada + sufixoZipUpdateExt
}

// baixarPacoteUpdateExt salva o zip em tmp com teto de tamanho.
func baixarPacoteUpdateExt(url, base string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", erroTextoF("montar baixa: %w", err)
	}

	req.Header.Set("User-Agent", uaChromeLoja)

	resp, err := clienteUpdateExt().Do(req)
	if err != nil || resp == nil {
		return "", erroTextoF("baixar pacote: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", erroTextoF("baixa respondeu %d", resp.StatusCode)
	}

	tmp := filepath.Join(exeDir, "data", pastaTmpUpdateExt)
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return "", erroTextoF("criar tmp: %w", err)
	}

	pacote := filepath.Join(tmp, nomePacoteUpdateExt(base))

	if err := gravarCorpoPacoteUpdateExt(resp.Body, pacote); err != nil {
		return "", err
	}

	return pacote, nil
}

// gravarCorpoPacoteUpdateExt salva o corpo com teto e valida o tamanho.
func gravarCorpoPacoteUpdateExt(corpo io.Reader, pacote string) error {
	saida, err := os.Create(pacote) // #nosec G304
	if err != nil {
		return erroTextoF("criar pacote: %w", err)
	}

	if _, err := io.Copy(saida, io.LimitReader(corpo, maxBytesPacoteUpdateExt+1)); err != nil {
		_ = saida.Close()
		_ = os.Remove(pacote)

		return erroTextoF("gravar pacote: %w", err)
	}

	if info, err := saida.Stat(); err != nil || info.Size() > maxBytesPacoteUpdateExt {
		_ = saida.Close()
		_ = os.Remove(pacote)

		return erroTexto("pacote grande demais")
	}

	if err := saida.Close(); err != nil {
		_ = os.Remove(pacote)

		return erroTextoF("fechar pacote: %w", err)
	}

	return nil
}

// segmentoRaizUpdateExt devolve o primeiro segmento se a entrada permite pasta unica.
func segmentoRaizUpdateExt(entrada *zip.File) (string, bool) {
	limpo := filepath.ToSlash(filepath.Clean(entrada.Name))
	if limpo == "" || limpo == "." {
		return "", true
	}

	partes := strings.Split(limpo, "/")
	if len(partes) < 2 || partes[0] == "" || strings.HasPrefix(partes[0], ".") {
		return "", false
	}

	return partes[0], true
}

// raizPacoteUpdateExt acha o prefixo quando o zip embrulha tudo numa pasta.
// Devolve "" quando o manifesto ja esta na raiz.
func raizPacoteUpdateExt(leitor *zip.ReadCloser) string {
	if leitor == nil {
		return ""
	}

	prefixo := ""

	for _, entrada := range leitor.File {
		segmento, ok := segmentoRaizUpdateExt(entrada)
		if !ok {
			return ""
		}

		if segmento == "" {
			continue
		}

		if prefixo == "" {
			prefixo = segmento
		} else if prefixo != segmento {
			return ""
		}
	}

	if prefixo == "" {
		return ""
	}

	if temManifestoSobPrefixoUpdateExt(leitor, prefixo) {
		return prefixo + "/"
	}

	return ""
}

// temManifestoSobPrefixoUpdateExt diz se o manifesto mora sob a pasta unica.
func temManifestoSobPrefixoUpdateExt(leitor *zip.ReadCloser, prefixo string) bool {
	for _, entrada := range leitor.File {
		if filepath.ToSlash(filepath.Clean(entrada.Name)) == prefixo+"/"+nomeManifestoUpdateExt {
			return true
		}
	}

	return false
}

// validarPacoteUpdateExt exige manifesto com versao esperada, na raiz ou numa pasta unica.
func validarPacoteUpdateExt(pacote, versao string) error {
	leitor, err := zip.OpenReader(pacote)
	if err != nil {
		return erroTextoF("abrir pacote: %w", err)
	}
	defer func() { _ = leitor.Close() }()

	alvo := nomeManifestoUpdateExt
	if prefixo := raizPacoteUpdateExt(leitor); prefixo != "" {
		alvo = prefixo + nomeManifestoUpdateExt
	}

	for _, entrada := range leitor.File {
		if filepath.ToSlash(filepath.Clean(entrada.Name)) != alvo {
			continue
		}

		aberto, err := entrada.Open()
		if err != nil {
			return erroTextoF("ler manifesto: %w", err)
		}

		dados, err := io.ReadAll(io.LimitReader(aberto, maxRespostaReleaseExt))
		_ = aberto.Close()

		if err != nil {
			return erroTextoF("ler manifesto: %w", err)
		}

		var manifesto extManifest
		if json.Unmarshal(dados, &manifesto) != nil {
			return erroTexto("manifesto invalido")
		}

		if compararVersoesExt(manifesto.Version, versao) != 0 {
			return erroTextoF("pacote e %s, esperado %s", manifesto.Version, versao)
		}

		return nil
	}

	return erroTexto("pacote sem manifesto")
}

// extrairPacoteUpdateExt despeja o zip contido na pasta destino.
func extrairPacoteUpdateExt(pacote, destino string) error {
	leitor, err := zip.OpenReader(pacote)
	if err != nil {
		return erroTextoF("abrir pacote: %w", err)
	}
	defer func() { _ = leitor.Close() }()

	const maxEntradasUpdateExt = 10000

	const maxTotalUpdateExt = int64(512 << 20)

	if len(leitor.File) > maxEntradasUpdateExt {
		return erroTexto("pacote com entradas demais")
	}

	var total int64

	for _, entrada := range leitor.File {
		total += int64(entrada.UncompressedSize64)
		if total > maxTotalUpdateExt {
			return erroTexto("pacote grande demais")
		}
	}

	if err := os.MkdirAll(destino, 0o700); err != nil {
		return erroTextoF("criar destino: %w", err)
	}

	absDestino, err := filepath.Abs(destino)
	if err != nil {
		return erroTextoF("resolver destino: %w", err)
	}

	prefixo := raizPacoteUpdateExt(leitor)

	for _, entrada := range leitor.File {
		if err := despejarEntradaValidadaUpdateExt(entrada, destino, absDestino, prefixo); err != nil {
			return err
		}
	}

	return nil
}

// despejarEntradaValidadaUpdateExt valida o caminho e despeja a entrada.
func despejarEntradaValidadaUpdateExt(entrada *zip.File, destino, absDestino, prefixo string) error {
	limpo := filepath.ToSlash(filepath.Clean(entrada.Name))
	if prefixo != "" {
		if !strings.HasPrefix(limpo, prefixo) {
			return erroTextoF("entrada fora da pasta: %s", entrada.Name)
		}

		limpo = strings.TrimPrefix(strings.TrimPrefix(limpo, prefixo), "/")
	}

	rel := filepath.FromSlash(limpo)
	if rel == "" || rel == "." {
		return nil
	}

	abs, err := filepath.Abs(filepath.Join(destino, rel))
	if err != nil || abs != absDestino && !strings.HasPrefix(abs, absDestino+string(filepath.Separator)) {
		return erroTextoF("entrada fora da pasta: %s", entrada.Name)
	}

	if entrada.FileInfo().IsDir() {
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return erroTextoF("criar pasta: %w", err)
		}

		return nil
	}

	return despejarEntradaUpdateExt(entrada, abs)
}

// despejarEntradaUpdateExt grava um arquivo do pacote no destino.
func despejarEntradaUpdateExt(entrada *zip.File, destino string) error {
	aberto, err := entrada.Open()
	if err != nil {
		return erroTextoF("ler entrada: %w", err)
	}
	defer func() { _ = aberto.Close() }()

	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return erroTextoF("criar pasta: %w", err)
	}

	saida, err := os.Create(destino) // #nosec G304
	if err != nil {
		return erroTextoF("criar arquivo: %w", err)
	}

	if _, err := io.Copy(saida, io.LimitReader(aberto, maxBytesPacoteUpdateExt+1)); err != nil {
		_ = saida.Close()

		return erroTextoF("gravar arquivo: %w", err)
	}

	if info, err := saida.Stat(); err != nil || info.Size() >= maxBytesPacoteUpdateExt {
		_ = saida.Close()

		return erroTexto("arquivo grande demais no pacote")
	}

	if err := saida.Close(); err != nil {
		return erroTextoF("fechar arquivo: %w", err)
	}

	return nil
}
