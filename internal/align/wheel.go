package align

import (
	"errors"
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Position identifies one of the four measured wheels.
type Position int

const (
	FL Position = iota // front left
	FR                 // front right
	RL                 // rear left
	RR                 // rear right
)

var AllPositions = [4]Position{FL, FR, RL, RR}

func (p Position) String() string {
	switch p {
	case FL:
		return "FL"
	case FR:
		return "FR"
	case RL:
		return "RL"
	case RR:
		return "RR"
	}
	return "??"
}

// RussianName is the label shown in the UI: «переднее левое» etc.
func (p Position) RussianName() string {
	switch p {
	case FL:
		return "Переднее левое"
	case FR:
		return "Переднее правое"
	case RL:
		return "Заднее левое"
	case RR:
		return "Заднее правое"
	}
	return "?"
}

func (p Position) IsLeft() bool  { return p == FL || p == RL }
func (p Position) IsFront() bool { return p == FL || p == FR }

// SideSign is +1 for left-hand wheels and -1 for right-hand wheels. It appears
// in nearly every formula below, because "outboard" and "toe-in" are mirrored
// across the vehicle.
func (p Position) SideSign() float64 {
	if p.IsLeft() {
		return 1
	}
	return -1
}

// Opposite returns the wheel on the same axle, other side.
func (p Position) Opposite() Position {
	switch p {
	case FL:
		return FR
	case FR:
		return FL
	case RL:
		return RR
	}
	return RL
}

// WheelObs is a single wheel as measured, expressed in the *measurement frame*
// — whatever arbitrary frame the sensor produced (camera frame, rig frame).
// Converting to the vehicle frame is Reference's job.
type WheelObs struct {
	Pos Position

	// SpinAxis is the wheel's axis of rotation as a unit vector, pointing
	// OUTBOARD (away from the vehicle's centreline). Sign matters: every angle
	// formula assumes outboard. Normalising the sign at the point of
	// measurement rather than deep in the maths is what makes the left/right
	// formulas identical instead of mirrored.
	SpinAxis geom.Vec3

	// Center is the wheel centre — the point on the spin axis at the middle of
	// the wheel. Used for track width, wheelbase, setback and the road plane.
	Center geom.Vec3

	// RollingRadiusMM is the loaded radius from wheel centre to ground. Used to
	// place the road plane. Optional: if zero, the road plane is taken through
	// the wheel centres, which is correct whenever all four radii are equal.
	RollingRadiusMM float64

	// RimDiameterMM is the reference diameter for millimetre toe readouts.
	RimDiameterMM float64

	// SteeringAxis, when non-nil, is the unit steering-axis direction pointing
	// UP, recovered from a caster sweep. Only meaningful on steered wheels.
	SteeringAxis *geom.Vec3

	// SteeringAxisPoint, when non-nil, is a point on the steering axis line,
	// which lets us also report scrub radius and the steering axis' ground
	// intersection.
	SteeringAxisPoint *geom.Vec3

	// Quality carries per-wheel diagnostics from the measurement stage.
	Quality Quality
}

// Quality records how much to trust a wheel observation.
type Quality struct {
	RunoutCompensated bool    `json:"runout_compensated"`
	RunoutMagnitude   Angle   `json:"runout_magnitude"` // wobble of the target relative to the true spin axis
	PoseRMSPx         float64 `json:"pose_rms_px"`      // reprojection error of the pose solve
	Frames            int     `json:"frames"`           // frames averaged into this observation
	Warnings          []string
}

var (
	ErrMissingWheel = errors.New("align: measurement is missing a wheel")
	ErrBadGeometry  = errors.New("align: wheel positions do not form a plausible vehicle")
)

// WheelSet is a complete four-wheel measurement.
type WheelSet map[Position]WheelObs

// NewWheelSet validates and collects four observations.
func NewWheelSet(obs ...WheelObs) (WheelSet, error) {
	ws := make(WheelSet, 4)
	for _, o := range obs {
		if o.SpinAxis.Len() < 0.5 {
			return nil, fmt.Errorf("%w: %s has a degenerate spin axis", ErrBadGeometry, o.Pos)
		}
		o.SpinAxis = o.SpinAxis.Unit()
		ws[o.Pos] = o
	}
	for _, p := range AllPositions {
		if _, ok := ws[p]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingWheel, p)
		}
	}
	return ws, nil
}

// Transform maps every observation into another frame, given the pose that
// takes measurement-frame coordinates into that frame.
func (ws WheelSet) Transform(p geom.Pose) WheelSet {
	out := make(WheelSet, len(ws))
	for pos, o := range ws {
		o.SpinAxis = p.ApplyDir(o.SpinAxis).Unit()
		o.Center = p.Apply(o.Center)
		if o.SteeringAxis != nil {
			v := p.ApplyDir(*o.SteeringAxis).Unit()
			o.SteeringAxis = &v
		}
		if o.SteeringAxisPoint != nil {
			v := p.Apply(*o.SteeringAxisPoint)
			o.SteeringAxisPoint = &v
		}
		out[pos] = o
	}
	return out
}
