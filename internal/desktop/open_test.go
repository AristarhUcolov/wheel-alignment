package desktop

import "testing"

func TestAllowed(t *testing.T) {
	for u, want := range map[string]bool{
		"https://ko-fi.com/AristarhUcolov":                  true,
		"https://buymeacoffee.com/Aristarh.Ucolov":          true,
		"https://www.donationalerts.com/c/aristarh_ucolov":  true,
		"https://github.com/AristarhUcolov/wheel-alignment": true,
		"http://ko-fi.com/AristarhUcolov":                   false, // not https
		"https://evil.example/ko-fi.com":                    false,
		"https://ko-fi.com.evil.example/":                   false,
		"https://user@ko-fi.com/":                           false,
		"file:///C:/Windows/System32/calc.exe":              false,
		"javascript:alert(1)":                               false,
	} {
		if got := Allowed(u); got != want {
			t.Errorf("%s: %v, want %v", u, got, want)
		}
	}
}
