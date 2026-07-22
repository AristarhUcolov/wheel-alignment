package align

import (
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// ReferenceMode selects what "vertical" means when camber is computed.
type ReferenceMode int

const (
	// RefRoadPlane takes vertical from the plane the four tyres stand on. This
	// is what 3D aligners do, and it is the physically meaningful reference:
	// camber is the angle between the wheel and the surface it grips. It does
	// not care whether the shop floor is level.
	RefRoadPlane ReferenceMode = iota

	// RefGravity takes vertical from a supplied gravity vector — a phone's
	// accelerometer, or a digital inclinometer. This is what a bubble camber
	// gauge measures, and it is only correct on a level floor. Provided
	// because it is the cheapest possible setup and, on a decent floor, it
	// works.
	RefGravity
)

// Reference is the vehicle coordinate frame derived from a measurement, plus
// the diagnostics that say how trustworthy that frame is.
type Reference struct {
	// Pose maps measurement-frame coordinates into the vehicle frame
	// (+X forward, +Y left, +Z up; origin at the rear axle centre, on the road).
	Pose geom.Pose

	Mode ReferenceMode

	// RoadPlaneRMSMM is how far the four contact points missed a common plane.
	// A large value means the car is not sitting flat: unequal tyre pressures,
	// a sagging spring, a twisted rack, or a mis-entered rolling radius.
	RoadPlaneRMSMM float64

	// WheelbaseMM and TrackMM are byproducts worth reporting — they are a free
	// sanity check that the operator labelled the wheels correctly and entered
	// the right vehicle.
	WheelbaseLeftMM  float64
	WheelbaseRightMM float64
	TrackFrontMM     float64
	TrackRearMM      float64

	Warnings []string
}

// FrameOptions tunes reference construction.
type FrameOptions struct {
	Mode ReferenceMode

	// Gravity is the measured gravity vector in the measurement frame, used
	// when Mode is RefGravity. It points DOWN (as an accelerometer at rest
	// reports acceleration, negated — see measure.GravityFromAccel).
	Gravity geom.Vec3
}

// BuildReference derives the vehicle coordinate frame from four wheel
// observations.
//
// Vertical comes from the road plane (or gravity). Longitudinal comes from the
// geometric centreline: the line joining the midpoint of the rear axle to the
// midpoint of the front axle. Everything downstream is expressed in that frame.
func BuildReference(ws WheelSet, opt FrameOptions) (Reference, error) {
	for _, p := range AllPositions {
		if _, ok := ws[p]; !ok {
			return Reference{}, fmt.Errorf("%w: %s", ErrMissingWheel, p)
		}
	}
	fl, fr := ws[FL].Center, ws[FR].Center
	rl, rr := ws[RL].Center, ws[RR].Center

	ref := Reference{Mode: opt.Mode}

	// --- Vertical -----------------------------------------------------------
	//
	// Seed "up" from the labelled layout itself: forward × left == up, and
	// (FL−RL) is forward while (FL−FR) is left, so their cross product points
	// up regardless of how the sensor happened to be oriented. No external hint
	// is needed, and it doubles as a check that the wheels were labelled
	// correctly — swapping a pair flips this vector.
	seedUp := fl.Sub(rl).Cross(fl.Sub(fr)).Unit()
	if seedUp.Len() < 0.5 {
		return Reference{}, fmt.Errorf("%w: wheel centres are collinear", ErrBadGeometry)
	}

	up := seedUp
	if opt.Mode == RefGravity {
		if opt.Gravity.Len() < 0.5 {
			return Reference{}, fmt.Errorf("%w: gravity reference selected but no gravity vector supplied", ErrBadGeometry)
		}
		up = opt.Gravity.Unit().Neg()
		if up.Dot(seedUp) < 0 {
			ref.Warnings = append(ref.Warnings,
				"Вектор гравитации направлен противоположно расчётному «верху» — проверьте ориентацию датчика")
		}
	} else {
		// Refine: drop each wheel centre to its contact patch and refit. Two
		// passes is plenty — the correction is second-order in the radius
		// differences, which are millimetres on a metre-scale baseline.
		for pass := 0; pass < 2; pass++ {
			contacts := make([]geom.Vec3, 0, 4)
			for _, p := range AllPositions {
				w := ws[p]
				contacts = append(contacts, w.Center.Sub(up.Scale(w.RollingRadiusMM)))
			}
			fit, err := geom.FitPlane(contacts)
			if err != nil {
				return Reference{}, err
			}
			n := fit.Normal
			if n.Dot(seedUp) < 0 {
				n = n.Neg()
			}
			up = n
			ref.RoadPlaneRMSMM = fit.RMS
		}
		if ref.RoadPlaneRMSMM > 8 {
			ref.Warnings = append(ref.Warnings, fmt.Sprintf(
				"Точки контакта колёс не лежат в одной плоскости (СКО %.1f мм). "+
					"Проверьте давление в шинах, просадку пружины и введённые радиусы качения.",
				ref.RoadPlaneRMSMM))
		}
	}

	// --- Longitudinal -------------------------------------------------------
	frontMid := fl.Add(fr).Scale(0.5)
	rearMid := rl.Add(rr).Scale(0.5)
	fwdRaw := frontMid.Sub(rearMid)
	fwd := fwdRaw.Sub(up.Scale(fwdRaw.Dot(up)))
	if fwd.Len() < 1e-6 {
		return Reference{}, fmt.Errorf("%w: front and rear axles coincide", ErrBadGeometry)
	}
	x := fwd.Unit()
	y := up.Cross(x).Unit() // Z × X = Y, points left
	z := x.Cross(y).Unit()  // re-orthogonalise

	// Origin: rear axle centre dropped onto the road plane. The thrust line
	// starts here, which makes the reported thrust angle easy to reason about.
	meanRearRadius := (ws[RL].RollingRadiusMM + ws[RR].RollingRadiusMM) / 2
	origin := rearMid.Sub(z.Scale(meanRearRadius))

	rot := geom.FromRows(x, y, z) // rows = vehicle axes in measurement frame
	ref.Pose = geom.Pose{R: rot, T: rot.MulVec(origin).Neg()}

	// --- Dimensional sanity -------------------------------------------------
	ref.WheelbaseLeftMM = fl.Sub(rl).Len()
	ref.WheelbaseRightMM = fr.Sub(rr).Len()
	ref.TrackFrontMM = fl.Sub(fr).Len()
	ref.TrackRearMM = rl.Sub(rr).Len()

	return ref, nil
}
