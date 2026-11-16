//go:build windows

package main

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	hresultFalhaGenericaOCR = 0x80004005
	hresultModoThreadOCR    = 0x80010106
	esperaResultadoOCR      = 60 * time.Second
	formatoPixelRgba8OCR    = 87
	esperaOperacaoOCR       = 15 * time.Second
	intervaloSondagemOCR    = 20 * time.Millisecond
	bytesPorPixelOCR        = 4
)

//nolint:gochecknoglobals // Handles FFI (combase.dll) resolvidos uma vez; processo tem uma pilha OCR.
var (
	combaseOcr              = windows.NewLazySystemDLL("combase.dll")
	pRoInitialize           = combaseOcr.NewProc("RoInitialize")
	pRoUninitialize         = combaseOcr.NewProc("RoUninitialize")
	pRoGetActivationFactory = combaseOcr.NewProc("RoGetActivationFactory")
	pWindowsCreateString    = combaseOcr.NewProc("WindowsCreateString")
	pWindowsDeleteString    = combaseOcr.NewProc("WindowsDeleteString")
	pWindowsGetStringRaw    = combaseOcr.NewProc("WindowsGetStringRawBuffer")
)

//nolint:gochecknoglobals // IIDs da API WinRT de OCR: constantes de processo, imutaveis apos init.
var (
	iidOcrEngineStatics   = ocrGUID("5bffa85a-3384-3540-9940-699120d428a8")
	iidSoftwareBitmapStcs = ocrGUID("df0385db-672f-4a9d-806e-c2442f343e86")
	iidCryptoBufferStcs   = ocrGUID("320b7e22-3cb0-4cdf-8663-1d28910065eb")
	iidAsyncInfo          = ocrGUID("00000036-0000-0000-c000-000000000046")
)

func ocrGUID(s string) *GUID {
	g, err := parseGUID(s)
	if err != nil {
		panic("OCR: IID invalido: " + s)
	}

	return g
}

type ocrIfaceVtbl struct {
	QueryInterface, AddRef, Release uintptr
	GetIids, GetRuntimeClassName    uintptr
	GetTrustLevel                   uintptr
}

type ocrEngineStaticsVtbl struct {
	ocrIfaceVtbl

	MaxImageDimension, AvailableLanguages, IsLanguageSupported, TryCreateFromLanguage,
	TryCreateFromUserProfileLanguages uintptr
}

type ocrEngineVtbl struct {
	ocrIfaceVtbl

	RecognizeAsync, RecognizerLanguage uintptr
}

type ocrOperationVtbl struct {
	ocrIfaceVtbl

	SetCompleted, Completed, GetResults uintptr
}

type asyncInfoVtbl struct {
	ocrIfaceVtbl

	ID, Status, ErrorCode, Cancel, Close uintptr
}

type ocrResultVtbl struct {
	ocrIfaceVtbl

	Lines, TextAngle, Text uintptr
}

type softwareBitmapStaticsVtbl struct {
	ocrIfaceVtbl

	Copy, Convert, ConvertWithAlpha     uintptr
	CreateCopyFromBuffer                uintptr
	CreateCopyWithAlphaFromBuffer       uintptr
	CreateCopyFromSurfaceAsync          uintptr
	CreateCopyWithAlphaFromSurfaceAsync uintptr
}

type cryptoBufferStaticsVtbl struct {
	ocrIfaceVtbl

	Compare, GenerateRandom, GenerateRandomNumber uintptr
	CreateFromByteArray                           uintptr
}

func comCall(fn uintptr, args ...uintptr) uintptr {
	if fn == 0 {
		return hresultFalhaGenericaOCR
	}

	r, _, _ := syscall.SyscallN(fn, args...)

	return r
}

func comVtable(obj uintptr) uintptr {
	if obj == 0 {
		return 0
	}

	return *(*uintptr)(unsafe.Pointer(obj))
}

func comRelease(obj uintptr) {
	if obj == 0 {
		return
	}

	vt := (*ocrIfaceVtbl)(unsafe.Pointer(comVtable(obj)))
	comCall(vt.Release, obj)
}

func ocrHString(s string) (uintptr, error) {
	textoUtf16, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, erroTextoF("ocr: texto: %w", err)
	}

	var handle uintptr

	hr := comCall(pWindowsCreateString.Addr(),
		uintptr(unsafe.Pointer(&textoUtf16[0])), uintptr(len(textoUtf16)-1), uintptr(unsafe.Pointer(&handle)))
	if Failed(hr) || handle == 0 {
		return 0, erroTextoF("HSTRING: 0x%08X", uint32(hr))
	}

	return handle, nil
}

func ocrHStringTexto(handle uintptr) string {
	if handle == 0 {
		return ""
	}

	var tamanho uint32

	ptr, _, _ := pWindowsGetStringRaw.Call(handle, uintptr(unsafe.Pointer(&tamanho)))
	if ptr == 0 || tamanho == 0 {
		return ""
	}

	texto := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), tamanho)

	return windows.UTF16ToString(texto)
}

func ocrFactory(classe string, iid *GUID) (uintptr, error) {
	handle, err := ocrHString(classe)
	if err != nil {
		return 0, err
	}
	defer pWindowsDeleteString.Call(handle)

	var fabrica uintptr

	hr := comCall(pRoGetActivationFactory.Addr(), handle,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&fabrica)))
	if Failed(hr) || fabrica == 0 {
		return 0, erroTextoF("WinRT %s: 0x%08X", classe, uint32(hr))
	}

	return fabrica, nil
}

func validarImagemOCR(pix []byte, largura, altura int32) bool {
	return len(pix) >= int(largura)*int(altura)*bytesPorPixelOCR && largura > 0 && altura > 0
}

func adquirirVezOCR() bool {
	select {
	case ocrSema <- struct{}{}:
		return true
	default:
		return false
	}
}

func iniciarTempoOCR() (bool, error) {
	hr, _, _ := pRoInitialize.Call(1)
	if hr == 0 || hr == 1 {
		return true, nil
	}

	if hr == hresultModoThreadOCR {
		return false, nil
	}

	return false, erroTextoF("WinRT init: 0x%08X", uint32(hr))
}

//exhaustruct:ignore // Resultado interno: texto OU erro; um sempre zero por desenho.
type resultadoOCR struct {
	texto string
	err   error
}

func executarOCRNaThread(pix []byte, largura, altura int32, canal chan resultadoOCR) {
	defer func() {
		if r := recover(); r != nil {
			select {
			case canal <- resultadoOCR{err: erroTexto("OCR falhou internamente")}:
			default:
			}
		}
	}()

	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	iniciou, err := iniciarTempoOCR()
	if err != nil {
		canal <- resultadoOCR{err: err}

		return
	}

	if iniciou {
		defer pRoUninitialize.Call()
	}

	txt, err := ocrTextoMTA(pix, largura, altura)
	canal <- resultadoOCR{txt, err}
}

func aguardarResultadoOCR(canal chan resultadoOCR) (string, error) {
	// Timeout externo: o polling interno tem 15 s, mas um travamento do COM
	// antes do envio deixaria o chamador preso para sempre.
	select {
	case resp := <-canal:
		return resp.texto, resp.err
	case <-time.After(esperaResultadoOCR):
		return "", erroTexto("OCR demorou demais")
	}
}

func ocrTexto(pix []byte, largura, altura int32) (string, error) {
	if !validarImagemOCR(pix, largura, altura) {
		return "", erroTexto("imagem invalida para OCR")
	}
	// Single-flight: OCR e pesado (thread + WinRT); segunda chamada
	// concorrente volta ocupada em vez de empilhar.
	if !adquirirVezOCR() {
		return "", erroTexto("OCR ja em andamento")
	}

	defer func() { <-ocrSema }()

	canal := make(chan resultadoOCR, 1)

	go executarOCRNaThread(pix, largura, altura, canal)

	return aguardarResultadoOCR(canal)
}

//nolint:gochecknoglobals // Semaforo de OCR (capacidade 1): serializa o motor WinRT.
var ocrSema = make(chan struct{}, 1)

func abrirFabricasOCR() (uintptr, uintptr, uintptr, error) {
	var (
		ocrF, sbF, cbF uintptr
		err            error
	)

	ocrF, err = ocrFactory("Windows.Media.Ocr.OcrEngine", iidOcrEngineStatics)
	if err != nil {
		return 0, 0, 0, err
	}

	sbF, err = ocrFactory("Windows.Graphics.Imaging.SoftwareBitmap", iidSoftwareBitmapStcs)
	if err != nil {
		comRelease(ocrF)

		return 0, 0, 0, err
	}

	cbF, err = ocrFactory("Windows.Security.Cryptography.CryptographicBuffer", iidCryptoBufferStcs)
	if err != nil {
		comRelease(sbF)
		comRelease(ocrF)

		return 0, 0, 0, err
	}

	return ocrF, sbF, cbF, nil
}

func criarBufferImagemOCR(cbFabrica uintptr, pix []byte) (uintptr, error) {
	var bufferImg uintptr

	cripto := (*cryptoBufferStaticsVtbl)(unsafe.Pointer(comVtable(cbFabrica)))

	hr := comCall(cripto.CreateFromByteArray, cbFabrica, uintptr(len(pix)),
		uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&bufferImg)))
	if Failed(hr) || bufferImg == 0 {
		return 0, erroTextoF("IBuffer: 0x%08X", uint32(hr))
	}

	return bufferImg, nil
}

func criarBitmapOCR(sbFabrica, bufferImg uintptr, largura, altura int32) (uintptr, error) {
	var bitmap uintptr

	bitmapVtbl := (*softwareBitmapStaticsVtbl)(unsafe.Pointer(comVtable(sbFabrica)))

	hr := comCall(bitmapVtbl.CreateCopyFromBuffer, sbFabrica, bufferImg,
		uintptr(formatoPixelRgba8OCR),
		uintptr(largura), uintptr(altura), uintptr(unsafe.Pointer(&bitmap)))
	if Failed(hr) || bitmap == 0 {
		return 0, erroTextoF("SoftwareBitmap: 0x%08X", uint32(hr))
	}

	return bitmap, nil
}

func criarMotorOCR(ocrFabrica uintptr) (uintptr, error) {
	var engine uintptr

	estaticos := (*ocrEngineStaticsVtbl)(unsafe.Pointer(comVtable(ocrFabrica)))

	hr := comCall(estaticos.TryCreateFromUserProfileLanguages, ocrFabrica, uintptr(unsafe.Pointer(&engine)))
	if Failed(hr) || engine == 0 {
		return 0, erroTexto("OCR do Windows indisponivel · instale um pacote de idioma de OCR")
	}

	return engine, nil
}

func iniciarReconhecimentoOCR(engine, bitmap uintptr) (uintptr, error) {
	var operacao uintptr

	vtEngine := (*ocrEngineVtbl)(unsafe.Pointer(comVtable(engine)))

	hr := comCall(vtEngine.RecognizeAsync, engine, bitmap, uintptr(unsafe.Pointer(&operacao)))
	if Failed(hr) || operacao == 0 {
		return 0, erroTextoF("RecognizeAsync: 0x%08X", uint32(hr))
	}

	return operacao, nil
}

func aguardarOperacaoOCR(info uintptr) error {
	vtAsync := (*asyncInfoVtbl)(unsafe.Pointer(comVtable(info)))
	fim := time.Now().Add(esperaOperacaoOCR)

	for {
		var status int32

		hr := comCall(vtAsync.Status, info, uintptr(unsafe.Pointer(&status)))
		if Failed(hr) {
			return erroTextoF("OCR status: 0x%08X", uint32(hr))
		}

		if status == 1 {
			return nil
		}

		if status == 2 || status == 3 {
			return erroTexto("OCR falhou")
		}

		if time.Now().After(fim) {
			return erroTexto("OCR demorou demais")
		}

		time.Sleep(intervaloSondagemOCR)
	}
}

func extrairTextoResultadoOCR(operacao uintptr) (string, error) {
	vtOperacao := (*ocrOperationVtbl)(unsafe.Pointer(comVtable(operacao)))

	var info uintptr

	hr := comCall(
		vtOperacao.QueryInterface, operacao, uintptr(unsafe.Pointer(iidAsyncInfo)), uintptr(unsafe.Pointer(&info)),
	)
	if Failed(hr) || info == 0 {
		return "", erroTextoF("IAsyncInfo: 0x%08X", uint32(hr))
	}
	defer comRelease(info)

	if err := aguardarOperacaoOCR(info); err != nil {
		return "", err
	}

	var result uintptr

	hr = comCall(vtOperacao.GetResults, operacao, uintptr(unsafe.Pointer(&result)))
	if Failed(hr) || result == 0 {
		return "", erroTextoF("OCR GetResults: 0x%08X", uint32(hr))
	}
	defer comRelease(result)

	var handleTexto uintptr

	vtResultado := (*ocrResultVtbl)(unsafe.Pointer(comVtable(result)))

	hr = comCall(vtResultado.Text, result, uintptr(unsafe.Pointer(&handleTexto)))
	if Failed(hr) {
		return "", erroTextoF("OCR Text: 0x%08X", uint32(hr))
	}

	if handleTexto == 0 {
		return "", erroTexto("nenhum texto encontrado")
	}
	defer pWindowsDeleteString.Call(handleTexto)

	txt := ocrHStringTexto(handleTexto)
	if txt == "" {
		return "", erroTexto("nenhum texto encontrado")
	}

	return txt, nil
}

func ocrTextoMTA(pix []byte, largura, altura int32) (string, error) {
	ocrF, sbF, cbF, err := abrirFabricasOCR()
	if err != nil {
		return "", err
	}
	defer comRelease(ocrF)
	defer comRelease(sbF)
	defer comRelease(cbF)

	bufferImg, err := criarBufferImagemOCR(cbF, pix)
	if err != nil {
		return "", err
	}
	defer comRelease(bufferImg)

	bitmap, err := criarBitmapOCR(sbF, bufferImg, largura, altura)
	if err != nil {
		return "", err
	}
	defer comRelease(bitmap)

	engine, err := criarMotorOCR(ocrF)
	if err != nil {
		return "", err
	}
	defer comRelease(engine)

	operacao, err := iniciarReconhecimentoOCR(engine, bitmap)
	if err != nil {
		return "", err
	}
	defer comRelease(operacao)

	return extrairTextoResultadoOCR(operacao)
}
