//go:build windows

package gui

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

// Win32 constants (winuser.h / windows.h).
const (
	// GWL_STYLE is -16; passed as an unsigned pointer-sized value (two's complement).
	gwlStyle = ^uintptr(15)

	wsCaption     = 0x00C00000
	wsThickFrame  = 0x00040000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000

	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020

	swMinimize = 6

	wmNCLButtonDown = 0x00A1
	htCaption       = 2
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procShowWindow        = user32.NewProc("ShowWindow")
	procReleaseCapture    = user32.NewProc("ReleaseCapture")
	procSendMessageW      = user32.NewProc("SendMessageW")

	dwmapi                       = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmExtendFrameIntoClient = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

// margins mirrors the Win32 MARGINS struct used by DwmExtendFrameIntoClientArea.
// A 1px margin on every side is enough to make DWM draw the window shadow and
// the rounded Windows 11 corners for a frameless window.
type margins struct {
	Left, Right, Top, Bottom int32
}

// enableShadow asks DWM to extend the frame into the client area, which brings
// back the native soft shadow after the caption and thick frame were removed.
func enableShadow(hwnd uintptr) {
	m := margins{Left: 1, Right: 1, Top: 1, Bottom: 1}
	procDwmExtendFrameIntoClient.Call(hwnd, uintptr(unsafe.Pointer(&m)))
}

// nativeHandle returns the HWND of a Fyne window via driver.NativeWindow (no title search).
func nativeHandle(win fyne.Window) uintptr {
	nw, ok := win.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var hwnd uintptr
	nw.RunNative(func(ctx any) {
		if c, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = c.HWND
		}
	})
	return hwnd
}

// MakeWindowFrameless removes the caption bar and the thick (resize) frame from the
// window so only the custom top bar is visible, then restores the DWM shadow.
// Call it after win.Show().
func MakeWindowFrameless(win fyne.Window) {
	hwnd := nativeHandle(win)
	if hwnd == 0 {
		return
	}

	style, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle))
	newStyle := style &^ uintptr(wsCaption|wsThickFrame|wsSysMenu|wsMinimizeBox|wsMaximizeBox)
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle), newStyle)

	// Apply the style change immediately; SWP_FRAMECHANGED forces the non-client area to recalc.
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)

	// Restore the soft DWM shadow lost with the frame.
	enableShadow(hwnd)
}

// StartNativeDrag hands the current left-button press to Windows so the user can
// drag the given window by a custom area. Call it from a mouse-down handler.
func StartNativeDrag(win fyne.Window) {
	hwnd := nativeHandle(win)
	if hwnd == 0 {
		return
	}
	procReleaseCapture.Call()
	procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
}

// MinimizeNativeWindow minimizes the given window.
func MinimizeNativeWindow(win fyne.Window) {
	hwnd := nativeHandle(win)
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swMinimize)
}
