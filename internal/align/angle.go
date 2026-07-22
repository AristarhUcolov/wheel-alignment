// Package align turns observed wheel geometry into the numbers that appear on
// an alignment printout: camber, toe, caster, SAI, included angle, thrust
// angle and setback — plus the comparison against the vehicle's factory spec.
//
// Everything in here is pure arithmetic on already-measured wheel axes. How
// those axes were obtained (cameras, lasers, a phone on a bracket, or typed in
// by hand from a bubble gauge) is deliberately somebody else's problem: see
// package vision and package measure.
package align

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Angle is an angle in radians. Alignment work is done in three different
// units depending on who printed the spec sheet — decimal degrees (most
// modern), degrees and minutes (older European and Japanese), and millimetres
// of toe (Soviet/Russian, older European, and every ВАЗ manual ever). Keeping
// one canonical unit internally and converting only at the edges is what keeps
// those three from ever being mixed up.
type Angle float64

// Deg builds an Angle from decimal degrees.
func Deg(d float64) Angle { return Angle(geom.Rad(d)) }

// DegMinAngle builds an Angle from degrees and minutes. The sign of the whole
// value follows `deg`; for values between -1° and 0° pass deg as -0.0 or use
// a negative minutes value.
func DegMinAngle(deg, min float64) Angle {
	s := 1.0
	if deg < 0 || math.Signbit(deg) || min < 0 {
		s = -1
	}
	return Deg(s * (math.Abs(deg) + math.Abs(min)/60))
}

// Rad builds an Angle from radians.
func Rad(r float64) Angle { return Angle(r) }

func (a Angle) Rad() float64 { return float64(a) }
func (a Angle) Deg() float64 { return geom.Deg(float64(a)) }

// DegMin splits the angle into whole degrees and minutes, both carrying the
// sign of the angle in their magnitude via the returned string form. Use
// FormatDegMin for display.
func (a Angle) DegMin() (deg int, min float64) {
	d := math.Abs(a.Deg())
	whole := math.Floor(d)
	m := (d - whole) * 60
	// Guard the 59.97' → 60.0' rounding seam.
	if m >= 59.995 {
		whole++
		m = 0
	}
	if a < 0 {
		return -int(whole), m
	}
	return int(whole), m
}

// FormatDegMin renders as e.g. `-0°30'` or `+3°15'`.
func (a Angle) FormatDegMin() string {
	d, m := a.DegMin()
	sign := "+"
	if a < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%d°%02.0f'", sign, int(math.Abs(float64(d))), m)
}

// FormatDeg renders as decimal degrees with a sign, e.g. `-0.50°`.
func (a Angle) FormatDeg() string { return fmt.Sprintf("%+.2f°", a.Deg()) }

// FormatMagnitude renders the size of an angle without a sign, e.g. `6°48'`.
// For phrases that already carry the direction — "увеличить на …" — a signed
// value reads as a contradiction.
func (a Angle) FormatMagnitude() string {
	d, m := a.DegMin()
	if d < 0 {
		d = -d
	}
	return fmt.Sprintf("%d°%02.0f'", d, m)
}

// ToeMM converts a toe *angle* into the linear toe measurement used by most
// pre-1990s spec sheets: the difference between the rim's rear and front
// separation, measured across a rim of diameter refDiaMM.
//
//	toe_mm = D · tan(τ)
//
// The reference diameter matters enormously and is the single most common way
// a DIY alignment goes wrong: "3 mm" on a 13" ВАЗ rim is a different angle
// than "3 mm" on a 17" rim. Always carry the diameter with the number.
func (a Angle) ToeMM(refDiaMM float64) float64 {
	return refDiaMM * math.Tan(float64(a))
}

// ToeInch is ToeMM in inches, for spec sheets quoted in fractions of an inch.
func (a Angle) ToeInch(refDiaMM float64) float64 { return a.ToeMM(refDiaMM) / 25.4 }

// ToeAngleFromMM is the inverse of ToeMM.
func ToeAngleFromMM(mm, refDiaMM float64) Angle {
	if refDiaMM <= 0 {
		return 0
	}
	return Angle(math.Atan(mm / refDiaMM))
}

// Inches converts a rim size in inches to millimetres, for use as refDiaMM.
func Inches(in float64) float64 { return in * 25.4 }

// MarshalJSON writes an Angle as decimal degrees. The wire format and the
// on-disk spec files are in degrees because that is what a human editing them
// expects; radians never leave the Go layer.
func (a Angle) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(round(a.Deg(), 4), 'f', -1, 64)), nil
}

// UnmarshalJSON reads decimal degrees.
func (a *Angle) UnmarshalJSON(b []byte) error {
	var d float64
	if err := json.Unmarshal(b, &d); err != nil {
		return fmt.Errorf("angle must be a number of degrees: %w", err)
	}
	*a = Deg(d)
	return nil
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// Range is a factory tolerance: a nominal value with a permitted band.
type Range struct {
	Min     Angle `json:"min"`
	Nominal Angle `json:"nominal"`
	Max     Angle `json:"max"`
}

// NewRange builds a Range from a nominal value and a symmetric tolerance.
func NewRange(nominal, tol Angle) Range {
	return Range{Min: nominal - tol, Nominal: nominal, Max: nominal + tol}
}

// RangeMinMax builds a Range from limits, taking the midpoint as nominal.
func RangeMinMax(min, max Angle) Range {
	return Range{Min: min, Nominal: (min + max) / 2, Max: max}
}

func (r Range) Contains(a Angle) bool { return a >= r.Min && a <= r.Max }

// Deviation returns how far outside the band the value sits: zero when in
// spec, negative when below Min, positive when above Max.
func (r Range) Deviation(a Angle) Angle {
	switch {
	case a < r.Min:
		return a - r.Min
	case a > r.Max:
		return a - r.Max
	default:
		return 0
	}
}

// Status grades a measurement against the band.
type Status string

const (
	StatusGood     Status = "good"     // comfortably inside the band
	StatusMarginal Status = "marginal" // inside, but within 15% of an edge
	StatusBad      Status = "bad"      // outside the band
	StatusNoSpec   Status = "no_spec"  // nothing to compare against
)

// Grade classifies a value against the range. The marginal band exists because
// a wheel sitting exactly on the tolerance edge will drift out of spec with
// the first pothole, and telling someone "you are technically fine" in that
// situation is not useful.
func (r Range) Grade(a Angle) Status {
	if r.Max <= r.Min {
		return StatusNoSpec
	}
	if !r.Contains(a) {
		return StatusBad
	}
	width := r.Max - r.Min
	margin := width * 0.15
	if a-r.Min < margin || r.Max-a < margin {
		return StatusMarginal
	}
	return StatusGood
}
