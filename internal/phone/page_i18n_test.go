package phone

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// The phone page carries its own small dictionary, since it is served apart
// from the main interface. Every tr('…') and data-t text must be in it.
func TestPhonePageTranslated(t *testing.T) {
	b, err := pageFS.ReadFile("web/phone.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	i := strings.Index(src, "const EN = ")
	j := strings.Index(src, "\n};\n// end EN")
	if i < 0 || j < i {
		t.Fatal("the page has no EN dictionary")
	}
	var en map[string]string
	if err := json.Unmarshal([]byte(src[i+len("const EN = "):j+2]), &en); err != nil {
		t.Fatalf("the EN dictionary is not valid JSON: %v", err)
	}

	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:^|[^\w$.])tr\('((?:[^'\\\n]|\\.)*)'\)`).FindAllStringSubmatch(src, -1) {
		used[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`<\w+(?:\s[^>]*)?\sdata-t(?:\s[^>]*)?>([^<]*)<`).FindAllStringSubmatch(src, -1) {
		used[strings.TrimSpace(m[1])] = true
	}
	if len(used) < 10 {
		t.Fatalf("found only %d strings — has the markup changed?", len(used))
	}
	for k := range used {
		if strings.TrimSpace(en[k]) == "" {
			t.Errorf("no English for %q", k)
		}
	}
	for k := range en {
		if !used[k] {
			t.Errorf("stale entry: %q", k)
		}
	}
}

func TestPhonePageGetsLanguage(t *testing.T) {
	b, _ := pageFS.ReadFile("web/phone.html")
	if !strings.Contains(string(b), `<html lang="ru">`) {
		t.Fatal(`page() swaps <html lang="ru"> for the current language; the tag has changed`)
	}
}
