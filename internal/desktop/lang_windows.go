//go:build windows

package desktop

import "golang.org/x/sys/windows"

var procUILang = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// SystemLang is the language of the Windows interface, reduced to the ones the
// program speaks: Russian for Russian, Ukrainian, Belarusian and Kazakh
// Windows — where Russian is the likelier second language — and English
// everywhere else.
func SystemLang() string {
	id, _, _ := procUILang.Call()
	switch id & 0x3ff { // primary language
	case 0x19, 0x22, 0x23, 0x3f: // ru, uk, be, kk
		return "ru"
	}
	return "en"
}
