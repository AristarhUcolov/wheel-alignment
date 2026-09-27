package specs

import (
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
)

// Parameter keys. They are shared by the printed report (Compare) and the live
// adjustment screen, so that a value graded on screen and the same value on
// the printout can never be graded against different figures.
//
// Per-wheel keys are the prefix plus the wheel: "camber_FL", "toe_RR".
const (
	KeyCamber = "camber_"
	KeyToe    = "toe_"
	KeyCaster = "caster_"
	KeySAI    = "sai_"

	KeyFrontTotalToe    = "front_total_toe"
	KeyRearTotalToe     = "rear_total_toe"
	KeyFrontCrossCamber = "front_cross_camber"
	KeyRearCrossCamber  = "rear_cross_camber"
	KeyFrontCrossCaster = "front_cross_caster"
	KeyThrustAngle      = "thrust_angle"
)

// ParamSpec is what a specification says about one reported parameter.
type ParamSpec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Axle  string `json:"axle"` // "front" | "rear" | "vehicle"

	Spec   *align.Range `json:"spec,omitempty"`
	SpecMM *MMRange     `json:"spec_mm,omitempty"`

	Adjustable bool   `json:"adjustable"`
	Method     string `json:"method,omitempty"`
}

// Lookup returns the tolerance and adjustment data that apply to a parameter
// key. A nil spec, or a key the spec says nothing about, yields a ParamSpec
// with no range — which grades as "no spec", never as "in spec".
func Lookup(s *Spec, key string) ParamSpec {
	p := ParamSpec{Key: key}

	wheelKey := func(prefix string) (align.Position, bool) {
		if !strings.HasPrefix(key, prefix) {
			return 0, false
		}
		for _, pos := range align.AllPositions {
			if key == prefix+pos.String() {
				return pos, true
			}
		}
		return 0, false
	}
	axleName := func(front bool) string {
		if front {
			return "front"
		}
		return "rear"
	}

	if pos, ok := wheelKey(KeyCamber); ok {
		ax := axleOf(s, pos.IsFront())
		p.Label, p.Axle = "Развал, "+pos.RussianName(), axleName(pos.IsFront())
		p.Spec = rangeOf(ax, func(a *AxleSpec) *align.Range { return a.Camber })
		p.Adjustable = adjOf(ax, func(a *AxleSpec) bool { return a.Adjustable.Camber })
		p.Method = methodOf(ax, func(a *AxleSpec) string { return a.Adjustable.CamberMethod })
		return p
	}
	if pos, ok := wheelKey(KeyCaster); ok {
		ax := axleOf(s, pos.IsFront())
		p.Label, p.Axle = "Кастер, "+pos.RussianName(), axleName(pos.IsFront())
		p.Spec = rangeOf(ax, func(a *AxleSpec) *align.Range { return a.Caster })
		p.Adjustable = adjOf(ax, func(a *AxleSpec) bool { return a.Adjustable.Caster })
		p.Method = methodOf(ax, func(a *AxleSpec) string { return a.Adjustable.CasterMethod })
		return p
	}
	if pos, ok := wheelKey(KeySAI); ok {
		ax := axleOf(s, pos.IsFront())
		p.Label, p.Axle = "Поперечный наклон оси (SAI), "+pos.RussianName(), axleName(pos.IsFront())
		p.Spec = rangeOf(ax, func(a *AxleSpec) *align.Range { return a.SAI })
		return p
	}
	if pos, ok := wheelKey(KeyToe); ok {
		ax := axleOf(s, pos.IsFront())
		p.Label, p.Axle = "Схождение, "+pos.RussianName(), axleName(pos.IsFront())
		p.Spec = rangeOf(ax, func(a *AxleSpec) *align.Range { return a.IndividualToe })
		p.Adjustable = adjOf(ax, func(a *AxleSpec) bool { return a.Adjustable.Toe })
		p.Method = methodOf(ax, func(a *AxleSpec) string { return a.Adjustable.ToeMethod })
		return p
	}

	switch key {
	case KeyFrontTotalToe, KeyRearTotalToe:
		front := key == KeyFrontTotalToe
		ax := axleOf(s, front)
		p.Label, p.Axle = "Суммарное схождение оси", axleName(front)
		p.Spec = rangeOf(ax, func(a *AxleSpec) *align.Range { return a.TotalToe })
		p.Adjustable = adjOf(ax, func(a *AxleSpec) bool { return a.Adjustable.Toe })
		if s != nil {
			p.SpecMM = toeMMOf(s, front)
		}
	case KeyFrontCrossCamber, KeyRearCrossCamber:
		front := key == KeyFrontCrossCamber
		p.Label, p.Axle = "Разница развала (лев − прав)", axleName(front)
		p.Spec = crossRange(axleOf(s, front), true)
	case KeyFrontCrossCaster:
		p.Label, p.Axle = "Разница кастера (лев − прав)", "front"
		p.Spec = crossRange(axleOf(s, true), false)
	case KeyThrustAngle:
		p.Label, p.Axle = "Угол тяги", "vehicle"
		if s != nil && s.MaxThrustAngle != nil {
			t := align.RangeMinMax(-*s.MaxThrustAngle, *s.MaxThrustAngle)
			p.Spec = &t
		}
		rear := axleOf(s, false)
		p.Adjustable = adjOf(rear, func(a *AxleSpec) bool { return a.Adjustable.Toe })
		p.Method = methodOf(rear, func(a *AxleSpec) string { return a.Adjustable.ToeMethod })
	}
	return p
}
