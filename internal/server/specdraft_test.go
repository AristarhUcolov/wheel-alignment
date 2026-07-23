package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postJSON(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

type specCheck struct {
	OK         bool     `json:"ok"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
	Title      string   `json:"title"`
	Verified   bool     `json:"verified"`
	Disclaimer string   `json:"disclaimer"`
	File       string   `json:"file"`
	Resolved   []struct {
		Axle, Name, Range, Detail string
	} `json:"resolved"`
}

func checkSpec(t *testing.T, body string) specCheck {
	t.Helper()
	rec := postJSON(t, "/api/specs/check", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out specCheck
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCheckSpecAcceptsGoodEntry: a well-formed entry comes back OK, with a file
// ready to contribute and the millimetre toe resolved into degrees so the
// contributor can see what it means.
func TestCheckSpecAcceptsGoodEntry(t *testing.T) {
	got := checkSpec(t, `{
      "id": "test-good", "make": "Тест", "model": "Модель",
      "year_from": 1990, "year_to": 2000, "rim_diameter_in": 13,
      "front_total_toe_mm": {"min": 2, "max": 4},
      "front": {"camber": {"min": 0.17, "nominal": 0.5, "max": 0.83},
                "caster": {"min": 3, "nominal": 3.5, "max": 4}},
      "rear": {},
      "source": {"kind": "factory", "reference": "Руководство, стр. 42"}
    }`)

	if !got.OK {
		t.Fatalf("корректная запись должна пройти, ошибки: %v", got.Errors)
	}
	if !got.Verified || got.Disclaimer != "" {
		t.Errorf("заводской источник не должен требовать предупреждения (verified=%v, disclaimer=%q)",
			got.Verified, got.Disclaimer)
	}
	if !strings.Contains(got.File, `"specs"`) || !strings.Contains(got.File, "test-good") {
		t.Errorf("должен вернуться готовый файл, получено: %.120s", got.File)
	}

	// 2–4 mm on a 13" rim is about 0°21'…0°42'; the point is that it is shown.
	var seen bool
	for _, r := range got.Resolved {
		if strings.Contains(r.Name, "мм") && strings.Contains(r.Detail, "ободе") {
			seen = true
			t.Logf("миллиметры показаны как градусы: %s → %s", r.Range, r.Detail)
		}
	}
	if !seen {
		t.Errorf("схождение в мм должно быть показано в градусах: %+v", got.Resolved)
	}
}

// TestCheckSpecRejectsMissingProvenance is the database's safety rule reaching
// the contributor directly, at the moment they can still fix it.
func TestCheckSpecRejectsMissingProvenance(t *testing.T) {
	got := checkSpec(t, `{
      "id": "no-source", "make": "Тест", "model": "Модель", "year_from": 1990,
      "front": {"camber": {"min": -1, "nominal": -0.5, "max": 0}},
      "rear": {}, "source": {}
    }`)
	if got.OK {
		t.Error("запись без источника должна быть отвергнута")
	}
	if len(got.Errors) == 0 || !strings.Contains(strings.Join(got.Errors, " "), "источник") {
		t.Errorf("ошибка должна называть отсутствующий источник: %v", got.Errors)
	}
}

// TestCheckSpecRejectsMillimetresWithoutRim: millimetres of toe with no rim
// diameter cannot be interpreted at all.
func TestCheckSpecRejectsMillimetresWithoutRim(t *testing.T) {
	got := checkSpec(t, `{
      "id": "no-rim", "make": "Тест", "model": "Модель", "year_from": 1990,
      "front_total_toe_mm": {"min": 2, "max": 4},
      "front": {}, "rear": {},
      "source": {"kind": "factory", "reference": "стр. 1"}
    }`)
	if got.OK {
		t.Error("миллиметры без диаметра обода должны быть отвергнуты")
	}
	if !strings.Contains(strings.Join(got.Errors, " "), "обод") {
		t.Errorf("ошибка должна указывать на диаметр обода: %v", got.Errors)
	}
}

// TestCheckSpecWarnsOnImplausibleFigures: possible but surprising values are
// advisory, so the entry still passes while the contributor is asked to look.
func TestCheckSpecWarnsOnImplausibleFigures(t *testing.T) {
	got := checkSpec(t, `{
      "id": "odd", "make": "Тест", "model": "Модель", "year_from": 1990,
      "rim_diameter_in": 14,
      "front": {"camber": {"min": -30, "nominal": 0, "max": 30}},
      "rear": {},
      "source": {"kind": "unverified", "reference": "форум"}
    }`)
	if !got.OK {
		t.Errorf("неправдоподобные, но возможные значения не должны отвергаться: %v", got.Errors)
	}
	if len(got.Warnings) == 0 {
		t.Error("развал ±30° должен вызвать предупреждение")
	}
	if got.Verified || got.Disclaimer == "" {
		t.Error("непроверенный источник должен получить предупреждение о недостоверности")
	}
	t.Logf("предупреждения: %v", got.Warnings)
}

// TestCheckSpecHandlesGarbage: a malformed body is answered, not crashed on.
func TestCheckSpecHandlesGarbage(t *testing.T) {
	got := checkSpec(t, `{ this is not json`)
	if got.OK || len(got.Errors) == 0 {
		t.Error("испорченный JSON должен вернуть понятную ошибку")
	}
}
