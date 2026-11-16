//go:build windows

package main

import (
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

//nolint:gochecknoglobals // DLL do cofre (advapi32): handle fixo do processo.
var advapi32 = windows.NewLazySystemDLL("advapi32.dll")

//nolint:gochecknoglobals // Procs do cofre: handles fixos do processo.
var (
	pCredWriteW     = advapi32.NewProc("CredWriteW")
	pCredReadW      = advapi32.NewProc("CredReadW")
	pCredFree       = advapi32.NewProc("CredFree")
	pCredEnumerateW = advapi32.NewProc("CredEnumerateW")
	pCredDeleteW    = advapi32.NewProc("CredDeleteW")
)

const (
	credTipoGenerico   = 1
	credPersistMaquina = 2
)

const (
	alvoCredPadrao = "Overdrive/login"
)

// maxNomeAlvoCred limita a leitura do nome vindo do cofre.
const maxNomeAlvoCred = 32768

func alvoCredConta(i int) string {
	return "Overdrive/conta/" + strconv.Itoa(i)
}

//exhaustruct:ignore // Espelho da API Win32 (CREDENTIALW): campos nao usados ficam zero.
type credencialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        uint64
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func credEscrever(alvo, segredo string) error {
	if alvo == "" {
		return erroTexto("cofre: alvo vazio")
	}

	target, err := windows.UTF16PtrFromString(alvo)
	if err != nil {
		return erroTextoF("cofre: alvo: %w", err)
	}

	blob := []byte(segredo)

	var ptr *byte
	if len(blob) > 0 {
		ptr = &blob[0]
	}

	credencial := credencialW{
		Type:               credTipoGenerico,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     ptr,
		Persist:            credPersistMaquina,
	}
	chamada, _, codigoErro := pCredWriteW.Call(uintptr(unsafe.Pointer(&credencial)), 0)

	runtime.KeepAlive(target)
	runtime.KeepAlive(blob)

	if chamada == 0 {
		if codigoErro != nil {
			return erroTextoF("cofre: gravar: %w", codigoErro)
		}

		return erroTexto("cofre: gravar falhou")
	}

	return nil
}

func credLer(alvo string) (string, bool) {
	if alvo == "" {
		return "", false
	}

	target, err := windows.UTF16PtrFromString(alvo)
	if err != nil {
		return "", false
	}

	var ponteiroCredencial uintptr

	r, _, _ := pCredReadW.Call(
		uintptr(unsafe.Pointer(target)), uintptr(credTipoGenerico), 0, uintptr(unsafe.Pointer(&ponteiroCredencial)),
	)
	runtime.KeepAlive(target)

	if r == 0 || ponteiroCredencial == 0 {
		return "", false
	}

	c := (*credencialW)(unsafe.Pointer(ponteiroCredencial))
	out := ""

	if c.CredentialBlobSize > 0 && c.CredentialBlob != nil && c.CredentialBlobSize < 64<<10 {
		b := unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)
		tmp := make([]byte, len(b))
		copy(tmp, b)
		out = string(tmp)
	}

	pCredFree.Call(ponteiroCredencial)

	return out, true
}

// credApagar remove uma credencial do cofre.
func credApagar(alvo string) error {
	if alvo == "" {
		return erroTexto("cofre: alvo vazio")
	}

	target, err := windows.UTF16PtrFromString(alvo)
	if err != nil {
		return erroTextoF("cofre: alvo: %w", err)
	}

	r, _, codigoErro := pCredDeleteW.Call(
		uintptr(unsafe.Pointer(target)), uintptr(credTipoGenerico), 0)
	runtime.KeepAlive(target)

	if r == 0 {
		if codigoErro != nil {
			return erroTextoF("cofre: apagar: %w", codigoErro)
		}

		return erroTexto("cofre: apagar falhou")
	}

	return nil
}

// credListar devolve os alvos do cofre com o prefixo (filtro com * no fim).
func credListar(prefixo string) []string {
	if prefixo == "" {
		return nil
	}

	filtro, err := windows.UTF16PtrFromString(prefixo + "*")
	if err != nil {
		return nil
	}

	var total uint32

	var listaPtr uintptr

	chamada, _, _ := pCredEnumerateW.Call(
		uintptr(unsafe.Pointer(filtro)), 0,
		uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&listaPtr)),
	)
	runtime.KeepAlive(filtro)

	if chamada == 0 || listaPtr == 0 || total == 0 {
		return nil
	}
	defer pCredFree.Call(listaPtr)

	ponteiros := unsafe.Slice((*uintptr)(unsafe.Pointer(listaPtr)), total)

	var out []string

	for _, ponteiro := range ponteiros {
		if ponteiro == 0 {
			continue
		}

		credencial := (*credencialW)(unsafe.Pointer(ponteiro))
		if credencial.TargetName == nil {
			continue
		}

		out = append(out, windows.UTF16ToString(
			unsafe.Slice(credencial.TargetName, maxNomeAlvoCred)))
	}

	return out
}
