package measure

import (
	"errors"
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
)

// ---------------------------------------------------------------------------
// Camber by angle gauge
// ---------------------------------------------------------------------------

// Inclinometer is a camber reading taken with a magnetic angle gauge, a digital
// level, or a phone lying against the rim.
//
// Readings are in degrees, positive when the TOP of the wheel leans outboard —
// the same convention as the rest of the program. A gauge that reads the other
// way round should be entered with Invert set rather than by the operator
// flipping signs in their head, which is where mistakes happen.
type Inclinometer struct {
	At0Deg float64

	// At180Deg is the reading after rolling the car until the wheel has turned
	// half a revolution and re-seating the gauge on the SAME spot on the rim.
	//
	// This is runout compensation for the poor: if the rim face sits at angle ε
	// to the true wheel plane, the first reading is (camber + ε) and the second
	// is (camber − ε), because half a turn carries the error to the other side.
	// Their average is the true camber, and their difference is twice the rim's
	// error — which is itself worth knowing, since a bent rim makes every other
	// reading on that corner suspect.
	At180Deg    float64
	Has180      bool
	Invert      bool
	OnRightSide bool
}

// Camber returns the compensated camber angle and the detected rim runout.
func (i Inclinometer) Camber() (camber align.Angle, rimError align.Angle) {
	a := i.At0Deg
	if i.Invert {
		a = -a
	}
	if !i.Has180 {
		return align.Deg(a), 0
	}
	b := i.At180Deg
	if i.Invert {
		b = -b
	}
	return align.Deg((a + b) / 2), align.Deg(math.Abs(a-b) / 2)
}

// ---------------------------------------------------------------------------
// Toe by string line
// ---------------------------------------------------------------------------

// LineSide says whether the reference string runs outside the wheels (the usual
// arrangement) or between them.
type LineSide int

const (
	LineOutside LineSide = iota
	LineInside
)

// StringToe is one wheel measured against a reference string.
//
// FrontMM and RearMM are the perpendicular distances from the string to the rim
// at two points equidistant ahead of and behind the wheel centre, at hub
// height. SpanMM is the horizontal distance between those two points — for
// readings taken on the rim's front and rear edges this is the rim diameter.
//
// Measuring at hub height matters: taken higher or lower, camber tilts the rim
// and contaminates the toe reading.
type StringToe struct {
	FrontMM float64
	RearMM  float64
	SpanMM  float64
	Side    LineSide
}

// Toe returns the individual toe angle, positive for toe-in.
//
// With the string outside the wheel, toeing the wheel in swings the front of
// the rim toward the vehicle centreline, i.e. away from the string, so the
// front reading grows. With the string inside, the sense reverses.
func (s StringToe) Toe() (align.Angle, error) {
	if s.SpanMM <= 0 {
		return 0, errors.New("measure: span between the two measuring points must be given (usually the rim diameter)")
	}
	d := s.FrontMM - s.RearMM
	if s.Side == LineInside {
		d = -d
	}
	return align.Rad(math.Atan2(d, s.SpanMM)), nil
}

// StringBox describes how the two reference strings were set up, so that their
// squareness can be checked before any of the readings are believed.
//
// Getting the box parallel to the vehicle's geometric centreline is the single
// hardest part of a string alignment, and the usual advice — "make the string
// the same distance from the hub at the front and the rear" — is wrong whenever
// the front and rear tracks differ, which is on most cars.
type StringBox struct {
	// Perpendicular distance from each string to the corresponding wheel
	// centre, at hub height.
	LeftFrontMM  float64
	LeftRearMM   float64
	RightFrontMM float64
	RightRearMM  float64

	// Track widths from the vehicle's specification, hub centre to hub centre.
	TrackFrontMM float64
	TrackRearMM  float64
}

// RequiredFrontMinusRear is how much larger the front offset must be than the
// rear offset for the string to be parallel to the geometric centreline.
//
// With the centreline as the X axis, the left string sits at a fixed y = S, so
// its offset at the front hub is S − Tf/2 and at the rear hub is S − Tr/2. The
// difference is therefore (Tr − Tf)/2 — independent of where the string is, and
// zero only when the two tracks happen to be equal.
func (b StringBox) RequiredFrontMinusRear() float64 {
	return (b.TrackRearMM - b.TrackFrontMM) / 2
}

// Check reports set-up problems, worst first. A tolerance of 1 mm over a 2.5 m
// wheelbase is about 1.4 minutes of arc of box misalignment, which is below
// what the rest of the method can resolve anyway.
func (b StringBox) Check(tolMM float64) []string {
	if tolMM <= 0 {
		tolMM = 1.0
	}
	want := b.RequiredFrontMinusRear()
	var out []string

	gotL := b.LeftFrontMM - b.LeftRearMM
	gotR := b.RightFrontMM - b.RightRearMM

	if math.Abs(gotL-want) > tolMM {
		out = append(out, fmt.Sprintf(
			"Левая струна не параллельна оси автомобиля: спереди минус сзади = %.1f мм, должно быть %.1f мм. "+
				"Сдвиньте передний конец левой струны на %.1f мм.",
			gotL, want, want-gotL))
	}
	if math.Abs(gotR-want) > tolMM {
		out = append(out, fmt.Sprintf(
			"Правая струна не параллельна оси автомобиля: спереди минус сзади = %.1f мм, должно быть %.1f мм. "+
				"Сдвиньте передний конец правой струны на %.1f мм.",
			gotR, want, want-gotR))
	}
	if b.TrackFrontMM <= 0 || b.TrackRearMM <= 0 {
		out = append(out, "Не заданы колеи осей — проверка параллельности струн выполнена как для равных колей, "+
			"что верно далеко не для всех автомобилей. Возьмите колеи из данных автомобиля.")
	}
	return out
}

// ---------------------------------------------------------------------------
// Toe by toe plates / tape across the tyres
// ---------------------------------------------------------------------------

// PlateToe is the classic two-measurement total toe: the distance across the
// axle taken ahead of the wheel centres and behind them, at equal height and
// equal distance from the centre.
//
// It gives TOTAL toe only. It cannot say how that total is split between the
// two wheels, so it cannot tell you whether the steering wheel will end up
// straight. Use it as a quick check, not as the basis for an adjustment.
type PlateToe struct {
	FrontMM float64 // distance between the two front measuring points
	RearMM  float64 // distance between the two rear measuring points
	SpanMM  float64
}

// TotalToe returns total toe for the axle, positive for toe-in.
func (p PlateToe) TotalToe() (align.Angle, error) {
	if p.SpanMM <= 0 {
		return 0, errors.New("measure: span between the front and rear measuring points must be given")
	}
	// Toe-in narrows the front of the axle relative to the rear.
	return align.Rad(math.Atan2(p.RearMM-p.FrontMM, p.SpanMM)), nil
}

// ---------------------------------------------------------------------------
// Assembling a manual session
// ---------------------------------------------------------------------------

// ManualWheel is everything a person can measure at one corner without optics.
type ManualWheel struct {
	Camber Inclinometer
	Toe    StringToe

	// Sweep, when set, adds caster (and optionally a rough SAI) for a steered
	// wheel.
	Sweep *SweepReading

	RimDiameterMM float64
}

// ManualSession is a complete four-corner manual measurement.
type ManualSession struct {
	Wheels map[align.Position]ManualWheel
	Box    *StringBox

	// Track and wheelbase from the vehicle data, used only for reporting and
	// for the string-box check — never for the angles themselves.
	TrackFrontMM float64
	TrackRearMM  float64
}

// Result reduces the session to the same report the optical path produces.
func (s ManualSession) Result() (align.Result, error) {
	raw := make(map[align.Position]align.RawWheel, 4)

	for _, p := range align.AllPositions {
		w, ok := s.Wheels[p]
		if !ok {
			return align.Result{}, fmt.Errorf("%w: %s", align.ErrMissingWheel, p)
		}
		cam, rimErr := w.Camber.Camber()
		toe, err := w.Toe.Toe()
		if err != nil {
			return align.Result{}, fmt.Errorf("%s: %w", p, err)
		}

		q := align.Quality{
			RunoutCompensated: w.Camber.Has180,
			RunoutMagnitude:   rimErr,
			Frames:            1,
		}
		if !w.Camber.Has180 {
			q.Warnings = append(q.Warnings,
				"Развал измерен без компенсации биения — прокатите машину на пол-оборота колеса и замерьте повторно")
		}
		if rimErr.Deg() > 0.5 {
			q.Warnings = append(q.Warnings, fmt.Sprintf(
				"Биение диска %s — диск погнут или на нём грязь; все замеры на этом колесе ненадёжны",
				rimErr.FormatDegMin()))
		}

		r := align.RawWheel{
			Camber:        cam,
			ToeGeometric:  toe,
			RimDiameterMM: w.RimDiameterMM,
			Quality:       q,
		}

		if w.Sweep != nil && p.IsFront() {
			sweep := *w.Sweep
			// The straight-ahead camber and toe are already measured at this
			// corner; feeding them in is what unlocks the exact solution.
			if !sweep.HasStraight {
				sweep.CamberStraight, sweep.HasStraight = cam, true
			}
			sweep.ToeStraight = toe

			sol, err := sweep.Solve(p)
			if err != nil {
				return align.Result{}, fmt.Errorf("%s: %w", p, err)
			}
			r.Caster, r.SAI = &sol.Caster, sol.SAI
			r.Quality.Warnings = append(r.Quality.Warnings, sol.Warnings...)
		}
		raw[p] = r
	}

	g := align.Geometry{
		Known:        s.TrackFrontMM > 0 && s.TrackRearMM > 0,
		TrackFrontMM: s.TrackFrontMM,
		TrackRearMM:  s.TrackRearMM,
	}
	res := align.Assemble(raw, g, "manual")

	if s.Box != nil {
		if problems := s.Box.Check(1.0); len(problems) > 0 {
			res.Warnings = append(problems, res.Warnings...)
		}
	} else {
		res.Warnings = append(res.Warnings,
			"Не введены параметры установки струн — невозможно проверить, параллельны ли они оси автомобиля. "+
				"Это самая частая причина ошибки при замере схождения «на шнурке».")
	}
	return res, nil
}
