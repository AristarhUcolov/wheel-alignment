// Package desktop holds the few things the program does to the computer it
// runs on, outside its own window.
package desktop

import (
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// allowed lists the sites the interface may open in the person's browser.
// The interface cannot ask for anything else: a link it opens runs outside
// every protection of the program's window.
var allowed = map[string]bool{
	"ko-fi.com":               true,
	"www.ko-fi.com":           true,
	"buymeacoffee.com":        true,
	"www.buymeacoffee.com":    true,
	"www.donationalerts.com":  true,
	"donationalerts.com":      true,
	"github.com":              true,
	"go.dev":                  true,
	"developer.microsoft.com": true,
}

// ErrNotAllowed is returned for addresses outside the allowed list.
var ErrNotAllowed = i18n.Err("этот адрес программа не открывает")

// Allowed reports whether OpenURL would open u.
func Allowed(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" || p.User != nil {
		return false
	}
	return allowed[strings.ToLower(p.Hostname())]
}

// OpenURL opens an allowed https address in the default browser.
func OpenURL(u string) error {
	if !Allowed(u) {
		return ErrNotAllowed
	}
	return Open(u)
}

// Open opens any address in the default browser, without the allow-list.
// For the program's own interface address at start-up.
func Open(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
