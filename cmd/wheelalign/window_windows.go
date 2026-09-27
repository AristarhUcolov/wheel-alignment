//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

var errNoWindow = i18n.Err("движок WebView2 недоступен")

// runWindow shows the interface in a native window and blocks until the window
// is closed. WebView2 ships with Windows 11 and every updated Windows 10, so on
// the machines this is for it is simply there; when it is not, the caller falls
// back to the browser.
func runWindow(url, data string) error {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		// The browser engine's own profile (cache, the permission the camera
		// prompt remembers) goes to the data directory, not next to the
		// program — which may be in Program Files or on a read-only stick.
		DataPath: filepath.Join(data, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  i18n.T("Сход-развал — открытый стенд"),
			Width:  1440,
			Height: 900,
			IconId: 1, // из rsrc_windows_amd64.syso
			Center: true,
		},
	})
	if w == nil {
		return errNoWindow
	}
	defer w.Destroy()
	w.SetSize(1024, 640, webview2.HintMin)
	maximize(w.Window())
	w.Navigate(url)
	w.Run()
	return nil
}

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procShowWindow = user32.NewProc("ShowWindow")
)

// maximize opens the window full screen: the adjustment screen is meant to be
// read from under a car, and every pixel of digit height counts.
func maximize(hwnd unsafe.Pointer) {
	const swMaximize = 3
	_, _, _ = procShowWindow.Call(uintptr(hwnd), swMaximize)
}

// notify shows a message box: a program started from a shortcut has no
// console to print to.
func notify(msg string) {
	t, _ := syscall.UTF16PtrFromString(msg)
	c, _ := syscall.UTF16PtrFromString(i18n.T("Сход-развал"))
	_, _ = windows.MessageBox(0, t, c, windows.MB_OK|windows.MB_ICONINFORMATION)
}
