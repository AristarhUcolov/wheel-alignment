package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/desktop"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// The interface's settings survive a restart of the program: they go to the
// settings file next to the language, not to the browser's storage.
func TestUISettingsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := desktop.SaveSettings(path, desktop.Settings{Lang: "ru"}); err != nil {
		t.Fatal(err)
	}
	srv := testServer(t)
	srv.SetSettingsFile(path)

	if got := decode[map[string]json.RawMessage](t, get(t, srv, "/api/settings"))["ui"]; string(got) != "{}" {
		t.Fatalf("fresh settings: ui = %s, want {}", got)
	}
	want := map[string]any{"theme": "light", "size": "large", "workshop": map[string]any{"name": "Гараж №5"}}
	if rec := sendJSON(t, srv, "/api/settings", map[string]any{"ui": want}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}

	// The language stays where it was, and a new server reads the same file.
	st := desktop.LoadSettings(path)
	if st.Lang != "ru" {
		t.Errorf("language lost: %q", st.Lang)
	}
	again := testServer(t)
	again.SetSettingsFile(path)
	var back struct {
		UI map[string]any `json:"ui"`
	}
	if err := json.Unmarshal(get(t, again, "/api/settings").Body.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if back.UI["theme"] != "light" || back.UI["size"] != "large" {
		t.Errorf("settings not read back: %v", back.UI)
	}
	if ws, _ := back.UI["workshop"].(map[string]any); ws["name"] != "Гараж №5" {
		t.Errorf("workshop not read back: %v", back.UI["workshop"])
	}
}

func TestUISettingsRefuseJunk(t *testing.T) {
	srv := testServer(t)
	for name, body := range map[string]any{
		"not an object": map[string]any{"ui": []int{1, 2}},
		"null":          map[string]any{"ui": nil},
		"too big":       map[string]any{"ui": map[string]any{"x": strings.Repeat("a", 20<<10)}},
	} {
		if rec := sendJSON(t, srv, "/api/settings", body); rec.Code < 400 {
			t.Errorf("%s: accepted with %d", name, rec.Code)
		}
	}
}

func TestAbout(t *testing.T) {
	srv := testServer(t)
	srv.SetVersion("9.9.9")
	dir := t.TempDir()
	srv.SetDataDir(dir)
	var a struct {
		Version  string `json:"version"`
		DataDir  string `json:"data_dir"`
		Vehicles int    `json:"vehicles"`
		Makes    int    `json:"makes"`
	}
	if err := json.Unmarshal(get(t, srv, "/api/about").Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Version != "9.9.9" || a.DataDir != dir || a.Vehicles < 200 || a.Makes < 40 {
		t.Errorf("about: %+v", a)
	}
}

// Browsing by make: every make comes with its region, and its key works as
// the make filter of the search.
func TestMakesForBrowsing(t *testing.T) {
	srv := testServer(t)
	var makes []struct {
		Key        string `json:"key"`
		Name       string `json:"name"`
		Region     string `json:"region"`
		RegionName string `json:"region_name"`
		Count      int    `json:"count"`
	}
	if err := json.Unmarshal(get(t, srv, "/api/specs/makes").Body.Bytes(), &makes); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range makes {
		seen[m.Region] = true
		if m.RegionName == "" || m.Count == 0 {
			t.Errorf("%+v", m)
		}
	}
	for _, r := range []specs.Region{specs.RegionUSSR, specs.RegionUSA, specs.RegionEurope, specs.RegionAsia} {
		if !seen[string(r)] {
			t.Errorf("no make in region %s", r)
		}
	}
	var res struct {
		Results []struct{ Make string } `json:"results"`
	}
	if err := json.Unmarshal(get(t, srv, "/api/specs/search?make="+url.QueryEscape("ГАЗ")).Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Results) < 10 {
		t.Fatalf("make filter found %d ГАЗ entries", len(res.Results))
	}
	for _, r := range res.Results {
		if r.Make != "ГАЗ" {
			t.Errorf("make filter let through %q", r.Make)
		}
	}
}
