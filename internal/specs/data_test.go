package specs

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// The built-in files, read the way the loader reads them but without its
// forgiveness: a duplicate id there is silently resolved at run time, which
// for our own data would hide a mistake.
func builtinFiles(t *testing.T) map[string][]Spec {
	t.Helper()
	out := map[string][]Spec{}
	err := fs.WalkDir(builtin, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := builtin.ReadFile(path)
		if err != nil {
			return err
		}
		var f specFile
		if err := json.Unmarshal(b, &f); err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		out[path] = f.Specs
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBuiltinIDsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for path, list := range builtinFiles(t) {
		for _, s := range list {
			if prev, dup := seen[s.ID]; dup {
				t.Errorf("%s: id %q is already used in %s", path, s.ID, prev)
			}
			seen[s.ID] = path
		}
	}
}

// One file per make: a make's entries must not be scattered, or a
// contributor fixing a model will miss the other half.
func TestOneFilePerMake(t *testing.T) {
	fileOf := map[string]string{}
	for path, list := range builtinFiles(t) {
		for _, s := range list {
			if s.Source.Kind == SourceClassGuidance {
				continue
			}
			if prev, ok := fileOf[s.Make]; ok && prev != path {
				t.Errorf("make %q is split between %s and %s", s.Make, prev, path)
			}
			fileOf[s.Make] = path
		}
	}
}

// Every built-in entry carries its words in English as well, so that the
// English interface never falls back to Russian for our own data.
func TestBuiltinEntriesHaveEnglish(t *testing.T) {
	need := func(id, what, ru, en string) {
		if strings.TrimSpace(ru) != "" && strings.TrimSpace(en) == "" {
			t.Errorf("%s: no English %s", id, what)
		}
	}
	for _, list := range builtinFiles(t) {
		for _, s := range list {
			e := s.EN
			if e == nil {
				t.Errorf("%s: no \"en\" block", s.ID)
				continue
			}
			need(s.ID, "make", s.Make, e.Make)
			need(s.ID, "model", s.Model, e.Model)
			need(s.ID, "trim", s.Trim, e.Trim)
			need(s.ID, "notes", s.Notes, e.Notes)
			need(s.ID, "reference", s.Source.Reference, e.Reference)

			c, ec := s.Conditions, Conditions{}
			if e.Conditions != nil {
				ec = *e.Conditions
			}
			need(s.ID, "load", c.Load, ec.Load)
			need(s.ID, "pressure", c.TyrePressure, ec.TyrePressure)
			need(s.ID, "fuel", c.FuelState, ec.FuelState)
			need(s.ID, "ride height note", c.RideHeightNote, ec.RideHeightNote)
			need(s.ID, "settle", c.SettleProcedure, ec.SettleProcedure)
			need(s.ID, "checks", c.AdditionalChecks, ec.AdditionalChecks)

			for _, ax := range []struct {
				name string
				a    Adjustability
				m    *MethodText
			}{{"front", s.Front.Adjustable, e.Front}, {"rear", s.Rear.Adjustable, e.Rear}} {
				m := MethodText{}
				if ax.m != nil {
					m = *ax.m
				}
				need(s.ID, ax.name+" camber method", ax.a.CamberMethod, m.Camber)
				need(s.ID, ax.name+" caster method", ax.a.CasterMethod, m.Caster)
				need(s.ID, ax.name+" toe method", ax.a.ToeMethod, m.Toe)
			}
		}
	}
}

// Only the source addresses of shipped entries may be opened from the
// interface; anything else, including look-alikes, is refused.
func TestIsSourceURL(t *testing.T) {
	db, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := db.Get("gaz-3110")
	if !ok || s.Source.URL == "" {
		t.Fatal("gaz-3110 should cite a page")
	}
	if !db.IsSourceURL(s.Source.URL) {
		t.Errorf("%s is the source of gaz-3110 but is refused", s.Source.URL)
	}
	for _, u := range []string{"", "https://evil.example/", s.Source.URL + "x",
		"javascript:alert(1)", "file:///C:/Windows/win.ini", "https://user@autoruk.ru/"} {
		if db.IsSourceURL(u) {
			t.Errorf("%q must not be allowed", u)
		}
	}
}
