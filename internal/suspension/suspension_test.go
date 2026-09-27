package suspension

import (
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// TestEveryDesignIsDescribed: an entry the interface would render half empty is
// worse than no entry — someone standing under their car needs every field.
func TestEveryDesignIsDescribed(t *testing.T) {
	for _, i := range All() {
		name := string(i.ID)
		if name == "" {
			name = "unknown"
		}
		if i.Name == "" || i.Summary == "" || i.Identify == "" {
			t.Errorf("%s: name, summary and identification text are all required", name)
		}
		if len(i.PreChecks) == 0 {
			t.Errorf("%s: no pre-checks — worn joints make any adjustment pointless, say what to inspect", name)
		}
		if i.Camber.How == "" || i.Toe.How == "" {
			t.Errorf("%s: camber and toe adjustment must be described, even if only to say there is none", name)
		}
		// Caster belongs to steered axles; a rear-only design describing it
		// would send someone looking for an adjustment that cannot exist.
		if i.Axle == AxleRear && i.Caster.How != "" {
			t.Errorf("%s: rear-only design describes caster", name)
		}
		if i.Axle != AxleRear && i.Caster.How == "" {
			t.Errorf("%s: steerable design does not describe caster", name)
		}
	}
}

// TestKingpinDesignsCheckKingpins: the whole reason owners of kingpin cars are
// turned away is that the kingpin has to be right first. Both kingpin designs
// must say so.
func TestKingpinDesignsCheckKingpins(t *testing.T) {
	for _, id := range []Type{DoubleWishboneKing, BeamKingpin} {
		i := MustGet(id)
		found := false
		for _, c := range i.PreChecks {
			if strings.Contains(c, "шкворн") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: pre-checks do not mention the kingpins", i.Name)
		}
	}
	// And a beam axle must never be described as having camber adjustment.
	if MustGet(BeamKingpin).Camber.Usual {
		t.Error("a beam axle on kingpins has no camber adjustment")
	}
}

func TestValidateByAxle(t *testing.T) {
	if err := Validate(TwistBeam, AxleFront); err == nil {
		t.Error("a twist beam cannot be a front suspension")
	}
	if err := Validate(BeamKingpin, AxleRear); err == nil {
		t.Error("a kingpin beam is a steered front axle")
	}
	if err := Validate(MacPherson, AxleRear); err != nil {
		t.Errorf("MacPherson struts exist at the rear too: %v", err)
	}
	if err := Validate(Unknown, AxleFront); err != nil {
		t.Errorf("an unspecified suspension must be accepted: %v", err)
	}
	if err := Validate("no_such_design", AxleFront); err == nil {
		t.Error("an unknown design id must be rejected")
	}
}

func TestForFiltersByAxle(t *testing.T) {
	for _, i := range For(AxleFront) {
		if i.Axle == AxleRear {
			t.Errorf("%s offered as a front suspension", i.Name)
		}
	}
	for _, i := range For(AxleRear) {
		if i.Axle == AxleFront {
			t.Errorf("%s offered as a rear suspension", i.Name)
		}
	}
}

// TestEveryDesignIsTranslated: someone reading the English interface under
// their car must not meet a Russian paragraph.
func TestEveryDesignIsTranslated(t *testing.T) {
	for _, s := range Strings() {
		if !i18n.Has(s) {
			t.Errorf("no English for %q", s)
		}
	}
}
