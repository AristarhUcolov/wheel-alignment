package specs

import (
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
)

// Plausibility checks for contributed data.
//
// Validate rejects entries that are structurally impossible — no source, an
// inverted tolerance band, millimetres of toe with no rim diameter to interpret
// them against. These checks are different: everything here is *possible*, just
// unlikely enough to be worth a second look before somebody adjusts a car to it.
//
// They are advisory by design and never reject. A genuine oddity does exist —
// racing camber, a heavily modified suspension, an unusual old design — and a
// database that refused to record the truth because the truth looked strange
// would be worse than one that asks. But the common contribution mistakes are
// mechanical and this catches them: a sign dropped, minutes entered as decimal
// degrees, millimetres typed into a field expecting degrees, front and rear
// swapped.

// Sanity returns advisory warnings about figures that are possible but
// surprising. An empty result means nothing looked odd.
func Sanity(s Spec) []string {
	var out []string

	add := func(format string, args ...any) {
		out = append(out, fmt.Sprintf(format, args...))
	}

	for _, ax := range []struct {
		name string
		spec AxleSpec
		mm   *MMRange
	}{
		{"передняя ось", s.Front, s.FrontTotalToeMM},
		{"задняя ось", s.Rear, s.RearTotalToeMM},
	} {
		if r := ax.spec.Camber; r != nil {
			checkRange(add, ax.name, "развал", *r, -4, 3, 4)
		}
		if r := ax.spec.Caster; r != nil {
			checkRange(add, ax.name, "кастер", *r, -3, 16, 6)
			if r.Max <= 0 && r.Min < 0 {
				add("%s: кастер отрицательный на всём диапазоне (%s … %s). "+
					"Отрицательный кастер делает руль неустойчивым и почти нигде не применяется — "+
					"проверьте знак.", ax.name, r.Min.FormatDegMin(), r.Max.FormatDegMin())
			}
		}
		if r := ax.spec.SAI; r != nil {
			checkRange(add, ax.name, "поперечный наклон оси (SAI)", *r, 2, 20, 6)
		}
		if r := ax.spec.TotalToe; r != nil {
			checkRange(add, ax.name, "суммарное схождение", *r, -1.5, 1.5, 1.2)
		}

		// A millimetre figure that resolves to an implausible angle is the
		// classic sign of the rim diameter being wrong, since the two are
		// inseparable.
		if ax.mm != nil && s.RimDiameterIn > 0 {
			ang := ax.mm.ToAngle(s.RimDiameterMM())
			if math.Abs(ang.Nominal.Deg()) > 1.2 {
				add("%s: %.1f…%.1f мм схождения на ободе %.0f\" — это %s … %s. "+
					"Такой угол очень велик; проверьте диаметр обода, к которому отнесены миллиметры.",
					ax.name, ax.mm.Min, ax.mm.Max, s.RimDiameterIn,
					ang.Min.FormatDegMin(), ang.Max.FormatDegMin())
			}
		}

		if lim := ax.spec.MaxCrossCamber; lim != nil && lim.Deg() > 2 {
			add("%s: допустимая разница развала по бортам %s — это очень много, "+
				"обычно не более 0°30'.", ax.name, lim.FormatDegMin())
		}
		if lim := ax.spec.MaxCrossCaster; lim != nil && lim.Deg() > 2.5 {
			add("%s: допустимая разница кастера по бортам %s — это очень много.",
				ax.name, lim.FormatDegMin())
		}
	}

	// Caster belongs to steered wheels. A caster figure on the rear axle is
	// nearly always the front axle's, entered in the wrong block.
	if s.Rear.Caster != nil {
		add("Кастер указан для задней оси. Кастер — угол оси поворота, у неуправляемой " +
			"задней оси его не бывает. Скорее всего это данные передней оси.")
	}
	if s.Rear.Adjustable.Caster {
		add("Для задней оси отмечена регулировка кастера — у неуправляемой оси её не бывает.")
	}

	// An angle given as a single value with no tolerance is suspicious: nobody
	// can adjust to an exact figure, and manuals always print a band.
	for name, r := range map[string]*align.Range{
		"развал спереди": s.Front.Camber, "кастер спереди": s.Front.Caster,
		"развал сзади": s.Rear.Camber,
	} {
		if r != nil && r.Max == r.Min {
			add("%s задан одним числом без допуска (%s). В руководствах всегда есть диапазон — "+
				"внесите его, иначе программа будет считать «в допуске» только точное совпадение.",
				name, r.Nominal.FormatDegMin())
		}
	}

	// There is deliberately no check on the width of the year range. A long run
	// with unchanged geometry is normal for precisely the cars this project
	// exists for — ВАЗ «Классика» ran 1970–2012, the Beetle far longer — so the
	// check fired on the flagship cases while catching nothing. A warning that
	// cries wolf on the commonest correct entry teaches people to skim past the
	// accurate ones, which costs more than it could ever save.

	if s.Front.Camber == nil && s.Front.Caster == nil && s.Front.TotalToe == nil &&
		s.FrontTotalToeMM == nil {
		add("Для передней оси не заданы никакие углы — запись почти бесполезна.")
	}

	return out
}

// checkRange flags a band that lies outside what road cars use, or one so wide
// it cannot be a tolerance.
func checkRange(add func(string, ...any), axle, what string, r align.Range, lo, hi, maxWidth float64) {
	if r.Min.Deg() < lo || r.Max.Deg() > hi {
		add("%s: %s %s … %s выходит за обычные для дорожных автомобилей пределы (%.0f° … %.0f°). "+
			"Проверьте знак и единицы: минуты — это не десятые доли градуса (0°30' = 0,5°).",
			axle, what, r.Min.FormatDegMin(), r.Max.FormatDegMin(), lo, hi)
	}
	if w := r.Max.Deg() - r.Min.Deg(); w > maxWidth {
		add("%s: допуск на %s шириной %.1f° — это неправдоподобно широко для допуска.",
			axle, what, w)
	}
}
