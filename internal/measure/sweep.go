// Package measure converts what a person or a sensor can actually observe into
// the wheel angles that package align reports on.
//
// Three input paths are supported, deliberately spanning the whole cost range:
//
//   - Manual: a string line, a tape measure and a magnetic angle gauge. Total
//     cost around the price of a tank of fuel. Accuracy, done carefully, is
//     within a few minutes of arc — better than many workshops achieve.
//   - Sweep: the same tools plus turntables, which unlocks caster and SAI.
//   - Optical: camera targets giving full 6-DOF wheel poses (package vision).
//
// All three end at align.RawWheel, so the reporting, the spec comparison and
// the adjustment advice are identical no matter what the person owns.
package measure

import (
	"errors"
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

var ErrSweepTooSmall = errors.New("measure: steering sweep too small to resolve caster")

// SweepReading is a two-point caster sweep, the method every workshop used
// before 3D machines existed and the one a person can do at home with
// turntables and a magnetic camber gauge.
//
// The procedure: with the car at ride height on turntables, read camber
// straight ahead, then steer the wheel OUT (away from the vehicle centreline)
// by HalfSweep and read camber, then steer it IN by the same angle and read it
// again.
type SweepReading struct {
	// CamberOut is camber with this wheel steered away from the vehicle
	// centreline; CamberIn is camber with it steered toward the centreline.
	// "Out" and "in" are per wheel, so one left-hand steering pass reads the
	// LEFT wheel out and the RIGHT wheel in — both wheels come off the same
	// pass, which is why the sweep is defined wheel-relative rather than
	// steering-wheel-relative. It also makes the formulas below identical on
	// both sides instead of mirrored.
	CamberOut align.Angle
	CamberIn  align.Angle

	// CamberStraight is camber with the wheels straight ahead — the camber
	// reading you wanted anyway. Without it only the approximate caster is
	// available and SAI is not.
	CamberStraight align.Angle
	HasStraight    bool

	// ToeStraight is this wheel's toe at the straight-ahead position. It enters
	// the exact solution only weakly; leaving it zero costs well under a
	// hundredth of a degree.
	ToeStraight align.Angle

	// HalfSweep is how far the wheel was turned each way. 20° is the classic
	// figure and gives the best signal-to-noise; 10° is used where steering
	// travel or space is limited, at roughly triple the error.
	HalfSweep align.Angle
}

// SweepSolution is what a sweep yields.
type SweepSolution struct {
	Caster align.Angle
	SAI    *align.Angle

	// Method records which of the two solutions was used, because they differ
	// by a quarter of a degree on a high-caster car and anyone comparing this
	// program against a workshop printout deserves to know which is which.
	Method   string
	Warnings []string
}

// Solve recovers caster, and where possible SAI, from the sweep.
//
// # Why the textbook formula is not enough
//
// Steering rotates the wheel bodily about the steering axis k, so the spin
// axis a obeys Rodrigues' formula, a(θ) = R_k(θ)·a₀. Taking the vertical
// component and writing it in terms of quantities that do not depend on θ:
//
//	a_z(θ) = A·cos θ + B·sin θ + C·(1 − cos θ)
//	A = a₀z,  B = (k × a₀)_z,  C = k_z·(k · a₀)
//
// The textbook derivation now approximates camber ≈ −a_z, which is where its
// accuracy goes: that step is first order in camber and it silently couples
// caster to SAI. Keeping a_z exact instead — camber and a_z are related by an
// exact arcsine, after all — makes A, B and C recoverable with no
// approximation whatsoever from the three readings:
//
//	A = −sin(camber₀)
//	B = s·[a_z(out) − a_z(in)] / (2 sin θ)
//	C = [ (a_z(out) + a_z(in))/2 − A cos θ ] / (1 − cos θ)
//
// What remains is to invert B and C for the two unknowns in k, which is a
// two-variable Newton solve that converges in a handful of iterations from the
// textbook answer as its starting point.
//
// In testing this removes a systematic error that reaches 0.27° on a car with
// 8° of caster and 14° of SAI — small on a 1970s car with 2° of caster, but
// most cars built since 2000 sit in exactly the region where it matters.
func (s SweepReading) Solve(p align.Position) (SweepSolution, error) {
	sw := math.Abs(s.HalfSweep.Rad())
	if sw < geom.Rad(5) {
		return SweepSolution{}, fmt.Errorf("%w: %.1f°, need at least 5° each way (20° recommended)",
			ErrSweepTooSmall, s.HalfSweep.Deg())
	}
	side := p.SideSign()

	// Work in a_z, where the relations are exact.
	azOut := -math.Sin(s.CamberOut.Rad())
	azIn := -math.Sin(s.CamberIn.Rad())
	bCoef := side * (azOut - azIn) / (2 * math.Sin(sw))

	if !s.HasStraight {
		// Only the odd part of the sweep is available, so C — and with it SAI —
		// is out of reach. Fall back to the textbook result, which amounts to
		// assuming the steering axis has no sideways tilt: sin κ = −s·B.
		if math.Abs(bCoef) > 1 {
			return SweepSolution{}, impossible(bCoef)
		}
		return SweepSolution{
			Caster: align.Rad(math.Asin(-side * bCoef)),
			Method: "two-point (приближённая формула)",
			Warnings: []string{"Не введён развал в положении «прямо» — кастер посчитан по приближённой формуле, " +
				"а поперечный наклон оси (SAI) не определён. Погрешность до ~0,3° на машинах с большим кастером."},
		}, nil
	}

	gamma := s.CamberStraight.Rad()
	tau := s.ToeStraight.Rad()
	aCoef := -math.Sin(gamma)
	cCoef := ((azOut+azIn)/2 - aCoef*math.Cos(sw)) / (1 - math.Cos(sw))

	// a₀ from the straight-ahead camber and toe, outboard-pointing.
	ax := math.Cos(gamma) * math.Sin(tau)
	ay := side * math.Cos(gamma) * math.Cos(tau)
	az := aCoef

	u, v, ok := solveAxis(ax, ay, az, bCoef, cCoef)
	if !ok {
		// Newton did not converge: report the textbook answer rather than
		// nothing, clearly labelled.
		if math.Abs(bCoef) > 1 {
			return SweepSolution{}, impossible(bCoef)
		}
		return SweepSolution{
			Caster:   align.Rad(math.Asin(-side * bCoef)),
			Method:   "two-point (приближённая формула, точное решение не сошлось)",
			Warnings: []string{"Точное решение не сошлось — проверьте, не перепутаны ли замеры «наружу» и «внутрь»."},
		}, nil
	}

	w := math.Sqrt(math.Max(0, 1-u*u-v*v))
	caster := align.Rad(math.Atan2(-u, w))
	sai := align.Rad(math.Atan2(-side*v, w))

	sol := SweepSolution{
		Caster: caster,
		SAI:    &sai,
		Method: "two-point (точное решение)",
	}
	// SAI comes from the even part of the sweep, whose divisor is (1 − cos θ):
	// 0.060 at a 20° sweep. Every camber error is amplified by 1/0.060 ≈ 17, so
	// a 0.1° gauge is worth about 1.7° of SAI. The number is useful for
	// comparing left against right — a bent strut shows up plainly — and not
	// much else.
	sol.Warnings = append(sol.Warnings, fmt.Sprintf(
		"SAI рассчитан из изменения развала при повороте: погрешность замеров усиливается примерно в %.0f раз "+
			"(при повороте на %.0f°). Пользуйтесь им только для сравнения левого и правого борта — "+
			"разница выдаёт погнутую деталь. Как точное значение не используйте.",
		1/(1-math.Cos(sw)), geom.Deg(sw)))
	return sol, nil
}

func impossible(v float64) error {
	return fmt.Errorf("measure: замеры дают невозможный кастер (коэффициент %.2f) — "+
		"скорее всего, перепутаны положения «наружу» и «внутрь»", v)
}

// solveAxis inverts the pair
//
//	B = u·a_y − v·a_x
//	C = w·(u·a_x + v·a_y + w·a_z),  w = √(1 − u² − v²)
//
// for the horizontal components (u, v) of the unit steering axis, by Newton's
// method. The first equation is linear, so the system is mild; the starting
// point is the textbook first-order answer, from which convergence is typically
// three iterations.
func solveAxis(ax, ay, az, b, c float64) (u, v float64, ok bool) {
	if math.Abs(ay) < 1e-6 {
		return 0, 0, false
	}
	u = b / ay
	v = (c - u*ax - az) / ay

	for i := 0; i < 40; i++ {
		s := u*u + v*v
		if s >= 0.98 { // axis nearly horizontal: not a steering axis
			return 0, 0, false
		}
		w := math.Sqrt(1 - s)
		p := u*ax + v*ay

		f1 := u*ay - v*ax - b
		f2 := w*p + w*w*az - c
		if math.Abs(f1) < 1e-14 && math.Abs(f2) < 1e-14 {
			return u, v, true
		}

		j11, j12 := ay, -ax
		j21 := -(u/w)*p + w*ax - 2*u*az
		j22 := -(v/w)*p + w*ay - 2*v*az

		det := j11*j22 - j12*j21
		if math.Abs(det) < 1e-15 {
			return 0, 0, false
		}
		du := (f1*j22 - f2*j12) / det
		dv := (j11*f2 - j21*f1) / det
		u -= du
		v -= dv
	}
	return u, v, math.Abs(u) < 1 && math.Abs(v) < 1
}

// CasterClassic is the rule-of-thumb every service manual prints:
//
//	caster = (camber_out − camber_in) / (2 sin θ)
//
// which at a 20° sweep is the familiar "multiply the camber swing by about
// 1.5". Kept because it is what a workshop's older machine computes, so it is
// the number to use when reconciling a disagreement — not because it is the
// better answer. Prefer Solve.
func (s SweepReading) CasterClassic() (align.Angle, error) {
	sw := math.Abs(s.HalfSweep.Rad())
	if sw < geom.Rad(5) {
		return 0, fmt.Errorf("%w: %.1f°", ErrSweepTooSmall, s.HalfSweep.Deg())
	}
	v := (s.CamberOut.Rad() - s.CamberIn.Rad()) / (2 * math.Sin(sw))
	if v < -1 || v > 1 {
		return 0, impossible(v)
	}
	return align.Rad(math.Asin(v)), nil
}

// SteeringAxisFromSweep recovers the steering axis directly from optical data,
// which is both far more accurate than the two-point formula and immune to its
// small-angle assumptions.
//
// The idea: steering rotates the wheel about a fixed axis k, so the wheel's
// spin axis sweeps a cone about k, and every sampled spin axis therefore has
// the same dot product with k. The tips of the sampled unit spin axes lie on a
// plane whose normal is k, and a plane fit recovers it.
//
// What makes this work in a home garage is that it is completely indifferent to
// the wheel rolling or slipping on the turntables during the sweep: spinning
// the wheel does not move its spin axis at all. It also needs no knowledge of
// how far the wheel was actually steered.
//
// spinAxes must be the OUTBOARD-pointing spin axis sampled at three or more
// distinct steering positions, in the measurement frame. upHint only fixes the
// sign of the result.
func SteeringAxisFromSweep(spinAxes []geom.Vec3, upHint geom.Vec3) (geom.Vec3, error) {
	k, err := geom.FitConeAxis(spinAxes, upHint)
	if err != nil {
		return geom.Vec3{}, fmt.Errorf("%w: need ≥3 steering positions spanning ≥10° of sweep", err)
	}
	return k, nil
}

// SteeringAxisLineFromPoses recovers the full steering axis *line* — direction
// and position — from target poses taken through a sweep. Having the line, not
// just the direction, additionally yields scrub radius and mechanical trail.
//
// This uses the rigid-body fit: a body pinned to a fixed axis satisfies
// (I − Rᵢ)·c = tᵢ for the stationary point c. It is more demanding than the
// cone fit because it assumes the wheel does not roll during the sweep, so it
// requires turntables that actually turn freely. Where that assumption is
// shaky, prefer SteeringAxisFromSweep for the direction and treat the position
// as advisory.
func SteeringAxisLineFromPoses(poses []geom.Pose, upHint geom.Vec3) (dir, point geom.Vec3, err error) {
	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		return geom.Vec3{}, geom.Vec3{}, err
	}
	if fit.Sweep < geom.Rad(8) {
		return geom.Vec3{}, geom.Vec3{}, fmt.Errorf("%w: only %.1f° of sweep observed", ErrSweepTooSmall, geom.Deg(fit.Sweep))
	}
	d := fit.Direction
	if d.Dot(upHint) < 0 {
		d = d.Neg()
	}
	return d, fit.Point, nil
}
