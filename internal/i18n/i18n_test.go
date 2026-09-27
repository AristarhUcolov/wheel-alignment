package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// literal evaluates a string literal or a constant concatenation of them.
func literal(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(x.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		a, ok1 := literal(x.X)
		b, ok2 := literal(x.Y)
		return a + b, ok1 && ok2
	case *ast.ParenExpr:
		return literal(x.X)
	}
	return "", false
}

// SourceStrings finds every literal passed to i18n.T or i18n.F in the module.
func sourceStrings(t *testing.T) map[string]string {
	t.Helper()
	root, _ := filepath.Abs("../..")
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if id, ok := fn.X.(*ast.Ident); ok && id.Name == "i18n" {
					name = fn.Sel.Name
				}
			case *ast.Ident:
				if strings.HasSuffix(path, filepath.Join("internal", "i18n", "i18n.go")) {
					name = fn.Name
				}
			}
			if name != "T" && name != "F" && name != "N" && name != "Err" {
				return true
			}
			if s, ok := literal(call.Args[0]); ok {
				rel, _ := filepath.Rel(root, fset.Position(call.Pos()).Filename)
				found[s] = rel
			} else if name == "F" {
				t.Errorf("%s: i18n.F with a non-literal format cannot be checked", fset.Position(call.Pos()))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

var verbRe = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)

func verbs(s string) []string {
	v := verbRe.FindAllString(s, -1)
	sort.Strings(v)
	return v
}

// TestEverythingTranslated: every string the code translates has an English
// version with the same formatting verbs. Set I18N_DUMP to a file name to get
// the missing ones written out.
func TestEverythingTranslated(t *testing.T) {
	found := sourceStrings(t)
	if len(found) == 0 {
		t.Fatal("no i18n.T / i18n.F calls found — the scanner is broken")
	}
	var missing []string
	for s, file := range found {
		e, ok := en[s]
		if !ok {
			missing = append(missing, s)
			t.Errorf("%s: no English for %q", file, s)
			continue
		}
		if a, b := strings.Join(verbs(s), " "), strings.Join(verbs(e), " "); a != b {
			t.Errorf("%s: verbs differ for %q: ru [%s], en [%s]", file, s, a, b)
		}
	}
	if p := os.Getenv("I18N_DUMP"); p != "" && len(missing) > 0 {
		sort.Strings(missing)
		var b strings.Builder
		for _, m := range missing {
			b.WriteString(strconv.Quote(m) + ": \"\",\n")
		}
		_ = os.WriteFile(p, []byte(b.String()), 0o644)
	}
	t.Logf("%d translated strings in the source", len(found))
}

func TestFallbackAndSwitch(t *testing.T) {
	defer Set(RU)
	add(map[string]string{"Строка для проверки переключения": "Language switch test string"})
	if T("Строка для проверки переключения") != "Строка для проверки переключения" {
		t.Error("Russian must be returned while the language is Russian")
	}
	Set(EN)
	if T("Строка для проверки переключения") != "Language switch test string" {
		t.Error("English not returned")
	}
	if T("Нет такой строки") != "Нет такой строки" {
		t.Error("a missing translation must fall back to the Russian text")
	}
	Set("de")
	if Current() != EN {
		t.Error("an unsupported language must be ignored")
	}
}
