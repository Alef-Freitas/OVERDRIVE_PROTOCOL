//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsPopup        = 0x80000000
	wsExLayered    = 0x00080000
	wsExAppwindow  = 0x00040000
	wsExTopmost    = 0x00000008
	swShow         = 5
	wmDestroy      = 0x0002
	wmPaint        = 0x000F
	wmClose        = 0x0010
	wmSetCursor    = 0x0020
	wmMouseMove    = 0x0200
	wmLButtonDown  = 0x0201
	wmLButtonUp    = 0x0202
	wmMouseWheel   = 0x020A
	wmEraseBkgnd   = 0x0014
	wmSize         = 0x0005
	smXVirtual     = 76
	smYVirtual     = 77
	smCXVirtual    = 78
	smCYVirtual    = 79
	ulwAlpha       = 0x00000002
	acSrcOver      = 0x00
	acSrcAlpha     = 0x01
	dibRgbColors   = 0
	iconSmall      = 0
	idiApplication = 32512
	idiArrow       = 32512
	verdadeiro     = 1
	mascaraPalavra = 0xFFFF
	deslocPalavra  = 16
	mascaraCanal   = 0xFF
	alfaOpaco      = 255
	deslocVerde    = 8
	deslocAzul     = 16
	deslocAlfa     = 24
	bitsPorPixel   = 32
	planoUnico     = 1
	fonteCorpo     = 11
	fonteTitulo    = 15
	espessuraBorda = 1
)

type api struct {
	user32               *windows.LazyDLL
	kernel32             *windows.LazyDLL
	gdi32                *windows.LazyDLL
	pRegisterClassExW    *windows.LazyProc
	pCreateWindowExW     *windows.LazyProc
	pShowWindow          *windows.LazyProc
	pGetMessageW         *windows.LazyProc
	pPostQuitMessage     *windows.LazyProc
	pDefWindowProcW      *windows.LazyProc
	pValidateRect        *windows.LazyProc
	pInvalidateRect      *windows.LazyProc
	pGetSystemMetrics    *windows.LazyProc
	pSetProcessDPIAware  *windows.LazyProc
	pLoadIconW           *windows.LazyProc
	pLoadCursorW         *windows.LazyProc
	pSetCursor           *windows.LazyProc
	pSendMessageW        *windows.LazyProc
	pGetModuleHandleW    *windows.LazyProc
	pUpdateLayeredWindow *windows.LazyProc
	pCreateCompatibleDC  *windows.LazyProc
	pCreateDIBSection    *windows.LazyProc
	pSelectObject        *windows.LazyProc
	pDeleteObject        *windows.LazyProc
	pDeleteDC            *windows.LazyProc
	pSetCapture          *windows.LazyProc
	pReleaseCapture      *windows.LazyProc
	pClientToScreen      *windows.LazyProc
	pScreenToClient      *windows.LazyProc
}

func novaAPI() *api {
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	gdi32 := windows.NewLazySystemDLL("gdi32.dll")

	return &api{
		user32:               user32,
		kernel32:             kernel32,
		gdi32:                gdi32,
		pRegisterClassExW:    user32.NewProc("RegisterClassExW"),
		pCreateWindowExW:     user32.NewProc("CreateWindowExW"),
		pShowWindow:          user32.NewProc("ShowWindow"),
		pGetMessageW:         user32.NewProc("GetMessageW"),
		pPostQuitMessage:     user32.NewProc("PostQuitMessage"),
		pDefWindowProcW:      user32.NewProc("DefWindowProcW"),
		pValidateRect:        user32.NewProc("ValidateRect"),
		pInvalidateRect:      user32.NewProc("InvalidateRect"),
		pGetSystemMetrics:    user32.NewProc("GetSystemMetrics"),
		pSetProcessDPIAware:  user32.NewProc("SetProcessDPIAware"),
		pLoadIconW:           user32.NewProc("LoadIconW"),
		pLoadCursorW:         user32.NewProc("LoadCursorW"),
		pSetCursor:           user32.NewProc("SetCursor"),
		pSendMessageW:        user32.NewProc("SendMessageW"),
		pGetModuleHandleW:    kernel32.NewProc("GetModuleHandleW"),
		pUpdateLayeredWindow: user32.NewProc("UpdateLayeredWindow"),
		pCreateCompatibleDC:  gdi32.NewProc("CreateCompatibleDC"),
		pCreateDIBSection:    gdi32.NewProc("CreateDIBSection"),
		pSelectObject:        gdi32.NewProc("SelectObject"),
		pDeleteObject:        gdi32.NewProc("DeleteObject"),
		pDeleteDC:            gdi32.NewProc("DeleteDC"),
		pSetCapture:          user32.NewProc("SetCapture"),
		pReleaseCapture:      user32.NewProc("ReleaseCapture"),
		pClientToScreen:      user32.NewProc("ClientToScreen"),
		pScreenToClient:      user32.NewProc("ScreenToClient"),
	}
}

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type mensagemW struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

type pontoT struct {
	X int32
	Y int32
}

type tamanhoT struct {
	CX int32
	CY int32
}

type blendT struct {
	BlendOp             uint8
	BlendFlags          uint8
	SourceConstantAlpha uint8
	AlphaFormat         uint8
}

type cabecalhoBitmap struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type infoBitmap struct {
	Header cabecalhoBitmap
}

type rectT struct {
	Left, Top, Right, Bottom int32
}

func textoUTF16(texto string) *uint16 {
	return windows.StringToUTF16Ptr(texto)
}

func (a *api) moduloHandle() uintptr {
	handle, _, _ := a.pGetModuleHandleW.Call(0)

	return handle
}

func (a *api) areaVirtual() rectT {
	x, _, _ := a.pGetSystemMetrics.Call(smXVirtual)
	y, _, _ := a.pGetSystemMetrics.Call(smYVirtual)
	largura, _, _ := a.pGetSystemMetrics.Call(smCXVirtual)
	altura, _, _ := a.pGetSystemMetrics.Call(smCYVirtual)

	return rectT{int32(x), int32(y), int32(x) + int32(largura), int32(y) + int32(altura)}
}

func ponteiroMensagem(mensagem *mensagemW) uintptr {
	return uintptr(unsafe.Pointer(mensagem))
}
