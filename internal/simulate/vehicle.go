// Package simulate is a forward model of vehicle wheel geometry: given the
// angles a car is supposed to have, it produces the wheel axes a sensor would
// observe.
//
// It exists for three reasons. It is the ground truth the measurement maths is
// tested against — every sign convention in package align is verified by
// round-tripping through here. It drives the demo mode, so someone can learn
// the program before jacking up a car. And it powers the "what if" preview in
// the adjustment advisor.
package simulate

import (
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// WheelSpec is the true geometry of one wheel, in the vehicle frame.
type WheelSpec struct {
	Camber align.Angle // positive: top outboard
	Toe    align.Angle // positive: toe-in, relative to the geometric centreline
	Caster align.Angle // positive: steering axis top rearward (steered wheels)
	SAI    align.Angle // positive: steering axis top inboard (steered wheels)

	Center          geom.Vec3
	RollingRadiusMM float64
	RimDiameterMM   float64
	Steered         bool
}

// SpinAxis returns the outboard-pointing unit spin axis for this wheel.
//
// Inverting the extraction in package align: camber sets how far the axis dips
// below horizontal, and toe sets its heading within the horizontal plane.
//
//	a = (cos γ · sin τ, s · cos γ · cos τ, −sin γ)
//
// where γ is camber, τ is toe and s is +1 on the left, −1 on the right.
func (w WheelSpec) SpinAxis(p align.Position) geom.Vec3 {
	g, t := w.Camber.Rad(), w.Toe.Rad()
	s := p.SideSign()
	return geom.V(
		math.Cos(g)*math.Sin(t),
		s*math.Cos(g)*math.Cos(t),
		-math.Sin(g),
	).Unit()
}

// SteeringAxis returns the upward-pointing unit steering axis.
//
// Caster and SAI are defined as projected angles — the axis' tilt seen from the
// side and from the front — so the axis is simply the up vector leant by
// tan(caster) rearward and tan(SAI) inboard.
func (w WheelSpec) SteeringAxis(p align.Position) geom.Vec3 {
	s := p.SideSign()
	return geom.V(
		-math.Tan(w.Caster.Rad()),
		-s*math.Tan(w.SAI.Rad()),
		1,
	).Unit()
}

// SpinAxisSteered returns the spin axis with the wheel steered OUTBOARD by the
// given angle — the motion of a caster sweep.
//
// Steering rotates the wheel bodily about its steering axis. A right-handed
// rotation about the upward steering axis turns the vehicle left, which steers
// the LEFT wheel outboard and the RIGHT wheel inboard; hence the side sign.
func (w WheelSpec) SpinAxisSteered(p align.Position, outboard align.Angle) geom.Vec3 {
	k := w.SteeringAxis(p)
	r := geom.Rodrigues(k, p.SideSign()*outboard.Rad())
	return r.MulVec(w.SpinAxis(p)).Unit()
}

// Vehicle is a complete four-wheel forward model.
type Vehicle struct {
	Name   string
	Wheels map[align.Position]WheelSpec
}

// Nominal returns a plausible mid-size front-wheel-drive saloon: 2600 mm
// wheelbase, 1520/1510 mm tracks, 16-inch wheels, and a mild set of
// out-of-spec angles so the demo has something to talk about.
func Nominal() Vehicle {
	const (
		wheelbase  = 2600.0
		trackFront = 1520.0
		trackRear  = 1510.0
		radius     = 315.0
		rim        = 16 * 25.4
	)
	mk := func(x, y float64, camber, toe, caster, sai float64, steered bool) WheelSpec {
		return WheelSpec{
			Camber:          align.Deg(camber),
			Toe:             align.Deg(toe),
			Caster:          align.Deg(caster),
			SAI:             align.Deg(sai),
			Center:          geom.V(x, y, radius),
			RollingRadiusMM: radius,
			RimDiameterMM:   rim,
			Steered:         steered,
		}
	}
	return Vehicle{
		Name: "Демонстрационный автомобиль",
		Wheels: map[align.Position]WheelSpec{
			align.FL: mk(wheelbase, trackFront/2, -0.9, 0.20, 3.1, 12.5, true),
			align.FR: mk(wheelbase, -trackFront/2, -0.2, -0.05, 3.6, 12.4, true),
			align.RL: mk(0, trackRear/2, -1.4, 0.18, 0, 0, false),
			align.RR: mk(0, -trackRear/2, -1.1, 0.02, 0, 0, false),
		},
	}
}

// WheelSet renders the model as a measurement, as a perfect sensor in the
// vehicle frame would see it.
func (v Vehicle) WheelSet() align.WheelSet {
	ws := make(align.WheelSet, 4)
	for _, p := range align.AllPositions {
		w := v.Wheels[p]
		obs := align.WheelObs{
			Pos:             p,
			SpinAxis:        w.SpinAxis(p),
			Center:          w.Center,
			RollingRadiusMM: w.RollingRadiusMM,
			RimDiameterMM:   w.RimDiameterMM,
			Quality:         align.Quality{RunoutCompensated: true, Frames: 30},
		}
		if w.Steered {
			k := w.SteeringAxis(p)
			obs.SteeringAxis = &k
			// A point on the steering axis: the wheel centre displaced inboard
			// by a typical kingpin offset, which puts the axis roughly through
			// the ball joints.
			pt := w.Center.Add(geom.V(0, -p.SideSign()*40, 0))
			obs.SteeringAxisPoint = &pt
		}
		ws[p] = obs
	}
	return ws
}

// Observe renders the model as a sensor in an arbitrary frame would see it,
// which is what any real rig produces. Used to prove that the reported angles
// do not depend on how the equipment happened to be set up.
func (v Vehicle) Observe(sensorPose geom.Pose) align.WheelSet {
	return v.WheelSet().Transform(sensorPose)
}

// SweepReadings produces the camber readings a two-point caster sweep would
// yield for a steered wheel, exactly as an operator with a magnetic gauge and
// turntables would record them.
func (v Vehicle) SweepReadings(p align.Position, halfSweep align.Angle) (out, in, straight align.Angle) {
	w := v.Wheels[p]
	out = align.Camber(w.SpinAxisSteered(p, halfSweep))
	in = align.Camber(w.SpinAxisSteered(p, -halfSweep))
	straight = align.Camber(w.SpinAxis(p))
	return
}
