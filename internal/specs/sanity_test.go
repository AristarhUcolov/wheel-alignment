package specs_test

import (
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

func rng(minDeg, maxDeg float64) *align.Range {
	r := align.RangeMinMax(align.Deg(minDeg), align.Deg(maxDeg))
	return &r
}

// plausible is an ordinary front-wheel-drive saloon: nothing here should be
// remarked on.
func plausible() specs.Spec {
	return specs.Spec{
		ID: "test-car", Make: "Тест", Model: "Модель", YearFrom: 2000, YearTo: 2010,
		RimDiameterIn: 15,
		Front: specs.AxleSpec{
			Camber: rng(-1.0, 0.0), Caster: rng(2.0, 3.5), SAI: rng(11, 13),
			TotalToe: rng(0.0, 0.3),
		},
		Rear: specs.AxleSpec{
			Camber: rng(-1.5, -0.5), TotalToe: rng(0.1, 0.4),
		},
		Source: specs.Source{Kind: specs.SourceFactory, Reference: "manual p.12"},
	}
}

func TestSanityAcceptsOrdinaryCar(t *testing.T) {
	if w := specs.Sanity(plausible()); len(w) != 0 {
		t.Errorf("обычный автомобиль не должен вызывать замечаний, получено: %v", w)
	}
}

// TestSanityCatchesMinutesAsDecimal is the commonest contribution mistake:
// 0°30' entered as 0.30° is harmless, but 3°30' entered as "3.30" is fine while
// 30' entered as "30" is a factor of sixty out and lands far outside anything a
// road car uses.
func TestSanityCatchesMinutesAsDecimal(t *testing.T) {
	s := plausible()
	s.Front.Camber = rng(-30, 30) // minutes typed into a degrees field
	w := specs.Sanity(s)
	if len(w) == 0 {
		t.Fatal("развал ±30° должен быть отмечен")
	}
	if !strings.Contains(strings.Join(w, " "), "минуты") {
		t.Errorf("подсказка должна упоминать путаницу градусов и минут: %v", w)
	}
}

func TestSanityCatchesSwappedCasterSign(t *testing.T) {
	s := plausible()
	s.Front.Caster = rng(-3.5, -2.0)
	joined := strings.Join(specs.Sanity(s), " ")
	if !strings.Contains(joined, "знак") {
		t.Errorf("отрицательный кастер должен быть отмечен: %q", joined)
	}
}

// TestSanityCatchesCasterOnRearAxle: a caster figure on a non-steered axle is
// almost always the front axle's, entered in the wrong block.
func TestSanityCatchesCasterOnRearAxle(t *testing.T) {
	s := plausible()
	s.Rear.Caster = rng(2, 3)
	joined := strings.Join(specs.Sanity(s), " ")
	if !strings.Contains(joined, "задней оси") {
		t.Errorf("кастер на задней оси должен быть отмечен: %q", joined)
	}
}

// TestSanityCatchesWrongRimForMillimetreToe: millimetres and rim diameter are
// inseparable, so an implausible resolved angle points at the diameter.
func TestSanityCatchesWrongRimForMillimetreToe(t *testing.T) {
	s := plausible()
	s.RimDiameterIn = 15
	s.FrontTotalToeMM = &specs.MMRange{Min: 20, Max: 30} // centimetres, or nonsense
	s.Resolve()
	joined := strings.Join(specs.Sanity(s), " ")
	if !strings.Contains(joined, "обод") {
		t.Errorf("огромное схождение в мм должно указывать на диаметр обода: %q", joined)
	}
}

func TestSanityCatchesSingleValueWithNoTolerance(t *testing.T) {
	s := plausible()
	s.Front.Camber = rng(-0.5, -0.5)
	joined := strings.Join(specs.Sanity(s), " ")
	if !strings.Contains(joined, "допуск") {
		t.Errorf("угол без допуска должен быть отмечен: %q", joined)
	}
}

func TestSanityCatchesEmptyFrontAxle(t *testing.T) {
	s := plausible()
	s.Front = specs.AxleSpec{}
	joined := strings.Join(specs.Sanity(s), " ")
	if !strings.Contains(joined, "передней оси") {
		t.Errorf("пустая передняя ось должна быть отмечена: %q", joined)
	}
}

// TestSanityIsAdvisoryOnly: an unusual but real car must still be recordable.
// Refusing the truth because it looks strange would be worse than asking.
func TestSanityDoesNotReject(t *testing.T) {
	s := plausible()
	s.Front.Camber = rng(-3.5, -2.5) // a track-focused car, genuinely
	if err := s.Validate(); err != nil {
		t.Errorf("Sanity не должна влиять на Validate: %v", err)
	}
	if len(specs.Sanity(s)) == 0 {
		t.Log("−3.5°…−2.5° развала не вызвало замечаний — допустимо, это край нормы")
	}
}

// TestBuiltInDatabaseIsSane: the entries shipped with the program must not trip
// their own plausibility checks.
func TestBuiltInDatabaseIsSane(t *testing.T) {
	db := load(t)
	for _, s := range db.All() {
		if w := specs.Sanity(s); len(w) > 0 {
			t.Errorf("%s (%s): %v", s.ID, s.Title(), w)
		}
	}
}
