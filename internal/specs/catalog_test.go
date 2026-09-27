package specs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
	"github.com/AristarhUcolov/wheel-alignment/internal/suspension"
)

// TestCatalogEntriesCarryNoFigures: a catalog entry is construction only. If a
// figure ever crept in, it would be displayed with no provenance at all.
func TestCatalogEntriesCarryNoFigures(t *testing.T) {
	db := load(t)
	n := 0
	for _, s := range db.All() {
		if s.Source.Kind != specs.SourceCatalog {
			continue
		}
		n++
		if s.HasFigures() {
			t.Errorf("%s: catalog entry carries tolerances", s.ID)
		}
		if s.FrontSuspension == "" && s.RearSuspension == "" {
			t.Errorf("%s: catalog entry says nothing about the suspension", s.ID)
		}
		if s.Verified() {
			t.Errorf("%s: a catalog entry must never count as verified", s.ID)
		}
		if lim, ok := db.Limits(s); ok && lim.Source.Kind != specs.SourceClassGuidance {
			t.Errorf("%s: limits come from %s, expected class guidance", s.ID, lim.ID)
		}
	}
	if n == 0 {
		t.Fatal("no catalog entries loaded")
	}
}

// TestVolgaIsFound: the car this whole feature was built for. A Volga owner
// must find their car whichever way they type it, and get kingpin advice.
func TestVolgaIsFound(t *testing.T) {
	db := load(t)
	for _, q := range []string{"волга 3110", "Volga 3110", "газ 3110", "3110"} {
		m := db.Search(specs.Query{Text: q})
		if len(m) == 0 || m[0].Spec.ID != "gaz-3110" {
			t.Errorf("%q: expected gaz-3110 first, got %v", q, ids(m))
		}
	}
	s, _ := db.Get("gaz-3110")
	if s.FrontSuspension != suspension.DoubleWishboneKing {
		t.Errorf("ГАЗ-3110 default front suspension is %q, want kingpin", s.FrontSuspension)
	}
	if _, ok := db.Limits(s); !ok {
		t.Error("ГАЗ-3110 has nothing to compare against")
	}
}

// TestTranslitIsDeterministic: "газель" contains "газ". With the replacements
// applied in map order the result used to depend on the run.
func TestTranslitIsDeterministic(t *testing.T) {
	db := load(t)
	for i := 0; i < 20; i++ {
		for _, q := range []string{"газель", "gazel", "ГАЗель 3302"} {
			m := db.Search(specs.Query{Text: q})
			if len(m) == 0 || m[0].Spec.ID != "gaz-gazelle-3302" {
				t.Fatalf("run %d, %q: expected the ГАЗель entry first, got %v", i, q, ids(m))
			}
		}
	}
}

func TestClassFilter(t *testing.T) {
	db := load(t)
	for _, m := range db.Search(specs.Query{Class: specs.ClassBus}) {
		if m.Spec.Class != "" && m.Spec.Class != specs.ClassBus {
			t.Errorf("%s (%s) returned for a bus search", m.Spec.ID, m.Spec.Class)
		}
	}
	if len(db.Search(specs.Query{Text: "паз", Class: specs.ClassBus})) == 0 {
		t.Error("ПАЗ not found among buses")
	}
}

// TestOwnEntriesRoundTrip: figures an owner types in from their manual must be
// written to disk, found first, survive a restart, and override the catalog
// entry they replace.
func TestOwnEntriesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db, err := specs.LoadWithUser(dir)
	if err != nil {
		t.Fatal(err)
	}
	own := specs.Spec{
		ID: "my-gaz-3110", Make: "ГАЗ", Model: "3110 «Волга» — моя", YearFrom: 1999,
		Class: specs.ClassCar, FrontSuspension: suspension.DoubleWishboneKing, RearSuspension: suspension.LiveAxleLeaf,
		RimDiameterIn: 15,
		Front:         specs.AxleSpec{Camber: rng(-0.5, 0.5), Caster: rng(0.5, 1.5)},
		Source:        specs.Source{Kind: specs.SourceFactory, Reference: "Руководство по ремонту, стр. 100"},
	}
	if err := db.Save(own); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "my-gaz-3110.json")); err != nil {
		t.Fatalf("own entry not written to disk: %v", err)
	}
	if m := db.Search(specs.Query{Text: "волга 3110"}); len(m) == 0 || m[0].Spec.ID != "my-gaz-3110" {
		t.Errorf("own entry should rank first, got %v", ids(m))
	}

	// A fresh start sees it.
	db2, err := specs.LoadWithUser(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(db2.LoadErrors) > 0 {
		t.Fatalf("own entry failed to reload: %v", db2.LoadErrors)
	}
	got, ok := db2.Get("my-gaz-3110")
	if !ok || !db2.IsLocal("my-gaz-3110") {
		t.Fatal("own entry lost on restart")
	}
	if got.Front.Camber == nil || got.Front.Camber.Max.Deg() != 0.5 {
		t.Errorf("own entry figures changed on the round trip: %+v", got.Front.Camber)
	}

	// Built-in entries cannot be deleted; own ones can.
	if err := db2.Delete("gaz-3110"); err == nil {
		t.Error("deleting a built-in entry must be refused")
	}
	if err := db2.Delete("my-gaz-3110"); err != nil {
		t.Fatal(err)
	}
	if _, ok := db2.Get("my-gaz-3110"); ok {
		t.Error("own entry still present after delete")
	}

	// A catalog-kind entry with figures is exactly what must never be saved.
	bad := own
	bad.ID = "bad"
	bad.Source = specs.Source{Kind: specs.SourceCatalog, Reference: "x"}
	if err := db2.Save(bad); err == nil {
		t.Error("a catalog entry with figures was saved")
	}
}

func ids(m []specs.Match) []string {
	out := make([]string, len(m))
	for i, x := range m {
		out[i] = x.Spec.ID
	}
	return out
}

// TestLookupAgreesWithCompare: the live screen grades through Lookup, the
// printed report through Compare. If they ever disagreed about a tolerance, the
// screen could say "in spec" about a value the printout calls out of spec.
func TestLookupAgreesWithCompare(t *testing.T) {
	db := load(t)
	res, err := align.Compute(simulate.Nominal().WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"guidance-fwd-mcpherson", "vaz-2101-2107-classic", "guidance-truck-bus-beam"} {
		spec, ok := db.Get(id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		for _, p := range specs.Compare(res, &spec).Params {
			l := specs.Lookup(&spec, p.Key)
			if l.Label != p.Label || l.Axle != p.Axle || l.Adjustable != p.Adjustable || l.Method != p.Method {
				t.Errorf("%s/%s: lookup %+v disagrees with report", id, p.Key, l)
			}
			switch {
			case (l.Spec == nil) != (p.Spec == nil):
				t.Errorf("%s/%s: lookup and report disagree on whether a tolerance exists", id, p.Key)
			case l.Spec != nil && *l.Spec != *p.Spec:
				t.Errorf("%s/%s: lookup %v, report %v", id, p.Key, *l.Spec, *p.Spec)
			}
		}
	}
	if l := specs.Lookup(nil, "camber_FL"); l.Spec != nil {
		t.Error("a nil spec must yield no tolerance")
	}
}
