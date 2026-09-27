package server

import (
	"encoding/json"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The interface is translated in the browser: every t('…') call and every
// element marked data-t uses its Russian text as the key into js/i18n-en.js.
// These tests read the embedded files the way the program serves them and
// make sure no key is missing, stale or has lost a {placeholder}.

var (
	// t('…'), t("…") and t(`…`) without ${}: the forms the screens use. The
	// look-behind is emulated by the leading group, so get('x') is not a call.
	jsCall = regexp.MustCompile("(?:^|[^\\w$.])t\\(\\s*(?:'((?:[^'\\\\\\n]|\\\\.)*)'|\"((?:[^\"\\\\\\n]|\\\\.)*)\"|`([^`$]*)`)")
	// <tag … data-t …>text<  and  data-t-title … title="text".
	htmlText  = regexp.MustCompile(`<\w+(?:\s[^>]*)?\sdata-t(?:\s[^>]*)?>([^<]*)<`)
	htmlTitle = regexp.MustCompile(`<\w+[^>]*\sdata-t-title[^>]*\stitle="([^"]*)"`)
	holder    = regexp.MustCompile(`\{\w+\}`)
	// Examples in comments are not calls.
	lineComment = regexp.MustCompile(`(?m)^\s*//.*$`)
)

func unescapeJS(s string) string {
	r := strings.NewReplacer(`\'`, `'`, `\"`, `"`, `\\`, `\`, `\n`, "\n", "\\`", "`")
	return r.Replace(s)
}

// webKeys returns every translation key with the file it was first seen in.
func webKeys(t *testing.T) map[string]string {
	t.Helper()
	keys := map[string]string{}
	err := fs.WalkDir(webFS, "web", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, "i18n-en.js") || strings.Contains(path, "/guide/") {
			return nil
		}
		b, err := webFS.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(b)
		add := func(k string) {
			k = strings.TrimSpace(k)
			if k == "" {
				return
			}
			if _, ok := keys[k]; !ok {
				keys[k] = path
			}
		}
		switch {
		case strings.HasSuffix(path, ".js"):
			src = lineComment.ReplaceAllString(src, "")
			for _, m := range jsCall.FindAllStringSubmatch(src, -1) {
				switch {
				case m[1] != "":
					add(unescapeJS(m[1]))
				case m[2] != "":
					add(unescapeJS(m[2]))
				default:
					add(m[3])
				}
			}
		case strings.HasSuffix(path, ".html"):
			for _, m := range htmlText.FindAllStringSubmatch(src, -1) {
				add(m[1])
			}
			for _, m := range htmlTitle.FindAllStringSubmatch(src, -1) {
				add(m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// jsObject parses the object literal of `export default {…};` or `const X =
// {…};` — the dictionaries are written as JSON so that this is possible.
func jsObject(t *testing.T, src, start string) map[string]string {
	t.Helper()
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatalf("no %q in the dictionary", start)
	}
	src = lineComment.ReplaceAllString(src[i+len(start):], "")
	j := strings.Index(src, "\n};")
	if j < 0 {
		t.Fatal("the dictionary must end with a line \"};\"")
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(src[:j+2])), &m); err != nil {
		t.Fatalf("the dictionary is not valid JSON: %v", err)
	}
	return m
}

func holders(s string) []string {
	h := holder.FindAllString(s, -1)
	sort.Strings(h)
	return slices.Compact(h)
}

func TestWebTranslated(t *testing.T) {
	b, err := webFS.ReadFile("web/js/i18n-en.js")
	if err != nil {
		t.Fatal(err)
	}
	en := jsObject(t, string(b), "export default")
	keys := webKeys(t)

	var missing []string
	for k, file := range keys {
		v, ok := en[k]
		switch {
		case !ok:
			missing = append(missing, k)
			t.Errorf("%s: no English for %q", file, k)
		case strings.TrimSpace(v) == "":
			t.Errorf("%s: empty English for %q", file, k)
		case !slices.Equal(holders(k), holders(v)):
			t.Errorf("%q: placeholders %v, but the English has %v", k, holders(k), holders(v))
		}
	}
	for k := range en {
		if _, ok := keys[k]; !ok {
			t.Errorf("stale entry in i18n-en.js, no longer used: %q", k)
		}
	}
	if p := os.Getenv("I18N_DUMP"); p != "" && len(missing) > 0 {
		sort.Strings(missing)
		out := map[string]string{}
		for _, k := range missing {
			out[k] = ""
		}
		j, _ := json.MarshalIndent(out, "", "  ")
		_ = os.WriteFile(p, j, 0o644)
	}
	t.Logf("%d interface strings", len(keys))
}

// The guide is translated section by section; both languages must offer the
// same sections in the same order, or the table of contents would diverge.
func TestGuideSectionsMatch(t *testing.T) {
	ids := func(path string) []string {
		b, err := webFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, m := range regexp.MustCompile(`(?m)^\['(\w+)',`).FindAllStringSubmatch(string(b), -1) {
			out = append(out, m[1])
		}
		return out
	}
	ru, en := ids("web/js/guide/ru.js"), ids("web/js/guide/en.js")
	if len(ru) == 0 || !slices.Equal(ru, en) {
		t.Fatalf("guide sections differ:\nru %v\nen %v", ru, en)
	}
}
