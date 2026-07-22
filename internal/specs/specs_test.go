package specs_test

import (
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

func load(t *testing.T) *specs.DB {
	t.Helper()
	db, err := specs.Load()
	if err != nil {
		t.Fatalf("loading the built-in database: %v", err)
	}
	if len(db.LoadErrors) > 0 {
		t.Fatalf("built-in database has invalid entries: %v", db.LoadErrors)
	}
	if db.Count() == 0 {
		t.Fatal("built-in database is empty")
	}
	return db
}

// TestEveryEntryHasProvenance is the safety rule of this package expressed as a
// test: nothing ships without a stated source, and anything that is not from a
// factory or licensed document must carry a disclaimer.
func TestEveryEntryHasProvenance(t *testing.T) {
	db := load(t)
	for _, s := range db.All() {
		if s.Source.Kind == "" {
			t.Errorf("%s: no source", s.ID)
		}
		if err := s.Source.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if !s.Verified() && s.Disclaimer() == "" {
			t.Errorf("%s: unverified data with no disclaimer", s.ID)
		}
		if s.Verified() && s.Source.Reference == "" {
			t.Errorf("%s: claims a factory source but cites no document", s.ID)
		}
	}
}

// TestGuidanceIsLabelledAsGuidance: class-based ranges must never be mistakable
// for a specific car's factory data.
func TestGuidanceIsLabelledAsGuidance(t *testing.T) {
	db := load(t)
	found := 0
	for _, s := range db.All() {
		if s.Source.Kind != specs.SourceClassGuidance {
			continue
		}
		found++
		if !strings.Contains(s.Disclaimer(), "НЕ ЗАВОДСКИЕ ДАННЫЕ") {
			t.Errorf("%s: guidance disclaimer is too soft: %q", s.ID, s.Disclaimer())
		}
	}
	if found == 0 {
		t.Error("no class-guidance entries: someone with an unlisted car gets nothing")
	}
	// And guidance must stay out of ordinary searches.
	for _, m := range db.Search(specs.Query{Text: "лада"}) {
		if m.Spec.Source.Kind == specs.SourceClassGuidance {
			t.Errorf("guidance entry %s leaked into a search for a real car", m.Spec.ID)
		}
	}
}

// TestSearchFindsCarsHowPeopleTypeThem: Cyrillic, Latin, with and without the
// model number.
func TestSearchFindsCarsHowPeopleTypeThem(t *testing.T) {
	db := load(t)
	for _, q := range []string{"ваз", "VAZ", "лада 2109", "lada samara", "2101", "жигули", "классика"} {
		if got := db.Search(specs.Query{Text: q}); len(got) == 0 {
			t.Errorf("search %q found nothing", q)
		}
	}
	if got := db.Search(specs.Query{Text: "тепловоз"}); len(got) != 0 {
		t.Errorf("search for nonsense returned %d hits", len(got))
	}
}

// TestYearFiltering keeps a 1975 car from matching a 1990s-only entry.
func TestYearFiltering(t *testing.T) {
	db := load(t)
	if got := db.Search(specs.Query{Text: "самара", Year: 1975}); len(got) != 0 {
		t.Errorf("a 1975 Samara should not exist, got %d hits", len(got))
	}
	if got := db.Search(specs.Query{Text: "самара", Year: 1995}); len(got) == 0 {
		t.Error("a 1995 Samara should be found")
	}
}

// TestMillimetreToeResolves: manuals quote toe in millimetres, the engine works
// in angles, and the conversion must happen once at load with the rim diameter
// the manual assumed.
func TestMillimetreToeResolves(t *testing.T) {
	db := load(t)
	s, ok := db.Get("vaz-2101-2107-classic")
	if !ok {
		t.Skip("entry not present")
	}
	if s.Front.TotalToe == nil {
		t.Fatal("millimetre toe did not resolve to an angle")
	}
	// 2–4 mm on a 13" rim is roughly 0.35–0.69°.
	if d := s.Front.TotalToe.Min.Deg(); d < 0.30 || d > 0.40 {
		t.Errorf("2 mm on a 13-inch rim resolved to %.3f°, expected about 0.35°", d)
	}
	// Individual toe must be half of total.
	if s.Front.IndividualToe == nil {
		t.Fatal("individual toe not derived from total")
	}
	if d := s.Front.IndividualToe.Max - s.Front.TotalToe.Max/2; d > 1e-12 || d < -1e-12 {
		t.Error("individual toe is not half of total toe")
	}
}

// TestCompareProducesActionableReport walks the whole pipeline: a measured car,
// a specification, a graded report and an ordered procedure.
func TestCompareProducesActionableReport(t *testing.T) {
	db := load(t)
	spec, ok := db.Get("guidance-fwd-mcpherson")
	if !ok {
		t.Fatal("guidance entry missing")
	}
	res, err := align.Compute(simulate.Nominal().WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rep := specs.Compare(res, &spec)

	if !rep.HasSpec || rep.SpecTitle == "" {
		t.Error("report does not identify the specification it used")
	}
	if rep.Disclaimer == "" {
		t.Error("a guidance-based report must carry its disclaimer")
	}
	if len(rep.Params) == 0 {
		t.Fatal("no parameters compared")
	}
	// The demo car has a deliberately lopsided front camber, so something must
	// be flagged and the procedure must be non-trivial.
	if rep.OutOfSpec == 0 {
		t.Error("the demo car is out of spec by construction, nothing was flagged")
	}
	if len(rep.Steps) < 2 {
		t.Errorf("expected an ordered procedure, got %d steps", len(rep.Steps))
	}

	// Toe must be the last adjustment mentioned before the verification step:
	// camber and caster both move it.
	var toeAt, camberAt int
	for _, s := range rep.Steps {
		if strings.Contains(s.Title, "схождение") {
			toeAt = s.Order
		}
		if strings.Contains(s.Title, "развал") {
			camberAt = s.Order
		}
	}
	if toeAt != 0 && camberAt != 0 && toeAt < camberAt {
		t.Errorf("toe (step %d) must be adjusted after camber (step %d)", toeAt, camberAt)
	}

	// Every flagged parameter needs advice, and unadjustable ones must say so
	// rather than sending someone hunting for a bolt that does not exist.
	for _, p := range rep.Params {
		if p.Status != align.StatusBad {
			continue
		}
		if p.Advice == "" {
			t.Errorf("%s is out of spec but carries no advice", p.Key)
		}
		if !p.Adjustable && p.Method == "" && !strings.Contains(p.Advice, "нет") {
			t.Errorf("%s is not adjustable but the advice does not say so: %q", p.Key, p.Advice)
		}
	}
}

// TestCompareWithoutSpec: an unlisted car still gets a useful report.
func TestCompareWithoutSpec(t *testing.T) {
	res, err := align.Compute(simulate.Nominal().WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rep := specs.Compare(res, nil)
	if rep.HasSpec {
		t.Error("report claims a spec it does not have")
	}
	if len(rep.Params) == 0 {
		t.Error("measured values should still be reported without a spec")
	}
	for _, p := range rep.Params {
		if p.Status != align.StatusNoSpec {
			t.Errorf("%s graded as %q with no specification loaded", p.Key, p.Status)
		}
	}
}

// TestValidateRejectsDangerousEntries covers the checks that keep unusable data
// out of the database.
func TestValidateRejectsDangerousEntries(t *testing.T) {
	base := specs.Spec{
		ID: "x", Make: "M", Model: "D", YearFrom: 2000,
		Source: specs.Source{Kind: specs.SourceFactory, Reference: "manual p.1"},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("a well-formed spec was rejected: %v", err)
	}

	noSource := base
	noSource.Source = specs.Source{}
	if noSource.Validate() == nil {
		t.Error("a spec with no source must be rejected")
	}

	factoryNoRef := base
	factoryNoRef.Source = specs.Source{Kind: specs.SourceFactory}
	if factoryNoRef.Validate() == nil {
		t.Error("a factory claim with no document cited must be rejected")
	}

	mmNoRim := base
	mmNoRim.FrontTotalToeMM = &specs.MMRange{Min: 2, Max: 4}
	if mmNoRim.Validate() == nil {
		t.Error("millimetre toe without a rim diameter is uninterpretable and must be rejected")
	}

	inverted := base
	r := align.RangeMinMax(align.Deg(1), align.Deg(-1))
	inverted.Front.Camber = &r
	if inverted.Validate() == nil {
		t.Error("an inverted tolerance band must be rejected")
	}
}
