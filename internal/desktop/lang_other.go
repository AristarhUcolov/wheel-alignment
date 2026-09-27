//go:build !windows

package desktop

import (
	"os"
	"strings"
)

// SystemLang is the language from the environment, reduced to the ones the
// program speaks.
func SystemLang() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		l := strings.ToLower(os.Getenv(v))
		if l == "" {
			continue
		}
		for _, p := range []string{"ru", "uk", "be", "kk"} {
			if strings.HasPrefix(l, p) {
				return "ru"
			}
		}
		return "en"
	}
	return "ru"
}
