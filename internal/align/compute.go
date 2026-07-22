package align

import (
	"math"
	"strconv"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// WheelResult holds the angles for a single wheel, all in the vehicle frame.
type WheelResult struct {
	Pos      Position `json:"pos"`
	PosName  string   `json:"pos_name"`
	Position string   `json:"position_ru"`

	// Camber: positive when the top of the wheel leans OUTBOARD.
	Camber Angle `json:"camber"`

	// ToeGeometric is individual toe relative to the geometric centreline.
	// ToeThrust is individual toe relative to the rear axle's thrust line —
	// the number that belongs on a front-axle spec sheet, because it is what
	// determines whether the steering wheel ends up straight.
	//
	// Both are positive for TOE-IN (front of the wheel turned toward the
	// vehicle centreline).
	ToeGeometric Angle `json:"toe_geometric"`
	ToeThrust    Angle `json:"toe_thrust"`

	// ToeMM is ToeThrust expressed as a linear measurement across the rim.
	ToeMM float64 `json:"toe_mm"`

	// Caster: positive when the top of the steering axis leans REARWARD.
	// Nil on wheels where no steering sweep was performed.
	Caster *Angle `json:"caster,omitempty"`

	// SAI (steering axis inclination; ГОСТ: поперечный наклон оси поворота):
	// positive when the top of the steering axis leans INBOARD.
	SAI *Angle `json:"sai,omitempty"`

	// IncludedAngle is SAI + Camber. It is the number that survives a bent
	// strut or spindle: SAI and camber can each read wrong while their sum
	// stays right, which says the fault is in how the part is mounted rather
	// than in the part. A side-to-side difference in included angle means
	// something is bent, and no adjustment will fix it.
	IncludedAngle *Angle `json:"included_angle,omitempty"`

	ScrubRadiusMM *float64 `json:"scrub_radius_mm,omitempty"`
	TrailMM       *float64 `json:"trail_mm,omitempty"`

	Quality Quality `json:"quality"`
}

// AxleResult holds per-axle summaries.
type AxleResult struct {
	// TotalToe is the sum of the two individual toes. It is independent of
	// which longitudinal reference is used, which makes it the number to trust
	// when the rear axle geometry is itself in doubt.
	TotalToe   Angle   `json:"total_toe"`
	TotalToeMM float64 `json:"total_toe_mm"`

	// CrossCamber and CrossCaster are left minus right. These drive pulling far
	// more directly than the absolute values: a car with both cambers at −1.5°
	// tracks straight, a car at −0.2°/−1.3° pulls hard.
	CrossCamber Angle  `json:"cross_camber"`
	CrossCaster *Angle `json:"cross_caster,omitempty"`

	// Setback: positive when the RIGHT wheel sits rearward of the left.
	SetbackMM    float64 `json:"setback_mm"`
	SetbackAngle Angle   `json:"setback_angle"`

	TrackMM float64 `json:"track_mm"`
}

// Geometry carries the dimensional measurements that come out of a 3D
// measurement. In manual (tape-and-string) mode most of it is unavailable and
// the zero value is used; nothing in the angle maths depends on it.
type Geometry struct {
	WheelbaseLeftMM  float64
	WheelbaseRightMM float64
	TrackFrontMM     float64
	TrackRearMM      float64
	FrontSetbackMM   float64
	RearSetbackMM    float64
	AxleOffsetMM     float64
	RoadPlaneRMSMM   float64
	Known            bool
}

// Result is a complete alignment measurement.
type Result struct {
	Wheels map[string]WheelResult `json:"wheels"`

	Front AxleResult `json:"front"`
	Rear  AxleResult `json:"rear"`

	// ThrustAngle: positive when the rear axle steers the car to the RIGHT of
	// the geometric centreline. Non-zero thrust makes a car crab down the road
	// and makes a "perfectly aligned" front end still pull.
	ThrustAngle Angle `json:"thrust_angle"`

	HasDimensions    bool    `json:"has_dimensions"`
	WheelbaseLeftMM  float64 `json:"wheelbase_left_mm"`
	WheelbaseRightMM float64 `json:"wheelbase_right_mm"`
	// WheelbaseDiffMM and TrackDiffMM are the frame-damage indicators. On an
	// old car these are often the most valuable output of the whole procedure:
	// they say whether alignment is even achievable.
	WheelbaseDiffMM float64 `json:"wheelbase_diff_mm"`
	TrackDiffMM     float64 `json:"track_diff_mm"`
	AxleOffsetMM    float64 `json:"axle_offset_mm"`

	Reference ReferenceInfo `json:"reference"`
	Warnings  []string      `json:"warnings,omitempty"`
}

// ReferenceInfo is the serialisable part of the reference frame.
type ReferenceInfo struct {
	Mode           string   `json:"mode"`
	RoadPlaneRMSMM float64  `json:"road_plane_rms_mm"`
	Warnings       []string `json:"warnings,omitempty"`
}

// RawWheel is one wheel reduced to bare angles, whatever the sensor was. This
// is the seam between measurement and reporting: a camera rig, a laser jig, a
// phone inclinometer and a length of string all converge here.
type RawWheel struct {
	Camber       Angle
	ToeGeometric Angle // toe-in positive, relative to the geometric centreline

	Caster *Angle
	SAI    *Angle

	RimDiameterMM float64
	ScrubRadiusMM *float64
	TrailMM       *float64

	Quality Quality
}

// Assemble turns four sets of per-wheel angles into a full report: thrust
// line, thrust-referenced front toe, totals, cross values and warnings.
func Assemble(raw map[Position]RawWheel, g Geometry, mode string) Result {
	res := Result{Wheels: make(map[string]WheelResult, 4)}
	res.Reference.Mode = mode

	// The thrust line bisects the rear wheels' rolling directions. Positive
	// thrust points right, so it is (left toe − right toe)/2: a rear left wheel
	// toed in and a rear right wheel toed out both aim the axle to the right.
	thrust := (raw[RL].ToeGeometric - raw[RR].ToeGeometric) / 2
	res.ThrustAngle = thrust

	for _, p := range AllPositions {
		r := raw[p]
		wr := WheelResult{
			Pos:           p,
			PosName:       p.String(),
			Position:      p.RussianName(),
			Camber:        r.Camber,
			ToeGeometric:  r.ToeGeometric,
			Caster:        r.Caster,
			SAI:           r.SAI,
			ScrubRadiusMM: r.ScrubRadiusMM,
			TrailMM:       r.TrailMM,
			Quality:       r.Quality,
		}

		// Front toe is re-referenced to the thrust line; rear toe defines that
		// line and stays on the geometric centreline. Re-referencing shifts the
		// two front wheels in opposite directions and leaves total toe
		// untouched — exactly the intent.
		if p.IsFront() {
			wr.ToeThrust = r.ToeGeometric - Angle(p.SideSign())*thrust
		} else {
			wr.ToeThrust = r.ToeGeometric
		}

		if r.SAI != nil {
			ia := *r.SAI + r.Camber
			wr.IncludedAngle = &ia
		}
		wr.ToeMM = wr.ToeThrust.ToeMM(rimOrDefault(r.RimDiameterMM))
		res.Wheels[p.String()] = wr
	}

	res.Front = axleSummary(res.Wheels, raw, FL, FR, g.TrackFrontMM, g.FrontSetbackMM)
	res.Rear = axleSummary(res.Wheels, raw, RL, RR, g.TrackRearMM, g.RearSetbackMM)

	if g.Known {
		res.HasDimensions = true
		res.WheelbaseLeftMM = g.WheelbaseLeftMM
		res.WheelbaseRightMM = g.WheelbaseRightMM
		res.WheelbaseDiffMM = g.WheelbaseLeftMM - g.WheelbaseRightMM
		res.TrackDiffMM = g.TrackFrontMM - g.TrackRearMM
		res.AxleOffsetMM = g.AxleOffsetMM
		res.Reference.RoadPlaneRMSMM = g.RoadPlaneRMSMM
	}

	res.Warnings = structuralWarnings(res)
	return res
}

// Compute derives every reported angle from a four-wheel 3D measurement.
//
// The observations may be in any frame; BuildReference establishes the vehicle
// frame and everything after that works exclusively in it.
func Compute(ws WheelSet, opt FrameOptions) (Result, error) {
	ref, err := BuildReference(ws, opt)
	if err != nil {
		return Result{}, err
	}
	v := ws.Transform(ref.Pose)

	raw := make(map[Position]RawWheel, 4)
	for _, p := range AllPositions {
		w := v[p]
		r := RawWheel{
			Camber:        Camber(w.SpinAxis),
			ToeGeometric:  Toe(w.SpinAxis, p),
			RimDiameterMM: w.RimDiameterMM,
			Quality:       w.Quality,
		}
		if w.SteeringAxis != nil {
			k := w.SteeringAxis.Unit()
			if k.Z < 0 {
				k = k.Neg()
			}
			c, s := Caster(k), SAIAngle(k, p)
			r.Caster, r.SAI = &c, &s

			if w.SteeringAxisPoint != nil {
				if scrub, trail, ok := steeringGroundGeometry(k, *w.SteeringAxisPoint, w, p, r.Camber); ok {
					r.ScrubRadiusMM, r.TrailMM = &scrub, &trail
				}
			}
		}
		raw[p] = r
	}

	frontMid := v[FL].Center.Add(v[FR].Center).Scale(0.5)
	rearMid := v[RL].Center.Add(v[RR].Center).Scale(0.5)
	g := Geometry{
		Known:            true,
		WheelbaseLeftMM:  ref.WheelbaseLeftMM,
		WheelbaseRightMM: ref.WheelbaseRightMM,
		TrackFrontMM:     ref.TrackFrontMM,
		TrackRearMM:      ref.TrackRearMM,
		FrontSetbackMM:   v[FL].Center.X - v[FR].Center.X,
		RearSetbackMM:    v[RL].Center.X - v[RR].Center.X,
		AxleOffsetMM:     rearMid.Y - frontMid.Y,
		RoadPlaneRMSMM:   ref.RoadPlaneRMSMM,
	}

	mode := "road_plane"
	if opt.Mode == RefGravity {
		mode = "gravity"
	}
	res := Assemble(raw, g, mode)
	res.Reference.Warnings = ref.Warnings
	res.Warnings = append(ref.Warnings, res.Warnings...)
	return res, nil
}

func axleSummary(wr map[string]WheelResult, raw map[Position]RawWheel, l, r Position, trackMM, setbackMM float64) AxleResult {
	left, right := wr[l.String()], wr[r.String()]
	out := AxleResult{
		TotalToe:    left.ToeThrust + right.ToeThrust,
		CrossCamber: left.Camber - right.Camber,
		TrackMM:     trackMM,
		SetbackMM:   setbackMM,
	}
	out.TotalToeMM = out.TotalToe.ToeMM(rimOrDefault(raw[l].RimDiameterMM))
	if left.Caster != nil && right.Caster != nil {
		cc := *left.Caster - *right.Caster
		out.CrossCaster = &cc
	}
	if trackMM > 1 {
		out.SetbackAngle = Angle(math.Atan2(setbackMM, trackMM))
	}
	return out
}

func rimOrDefault(d float64) float64 {
	if d <= 0 {
		return Inches(15) // neutral placeholder; the UI always asks for the real one
	}
	return d
}

// Camber returns the camber angle for an OUTBOARD-pointing unit spin axis.
//
// Derivation: a wheel standing vertical has a horizontal spin axis. Leaning the
// top of the wheel outboard by γ rotates the wheel plane, and hence its normal
// — the spin axis — so the axis dips below horizontal by γ. That gives
// a_z = −sin γ on both sides once the axis points outboard, which is precisely
// why the outboard convention is enforced at measurement time: it makes the
// left and right formulas identical instead of mirrored.
func Camber(outboardAxis geom.Vec3) Angle {
	a := outboardAxis.Unit()
	return Angle(-math.Asin(clamp1(a.Z)))
}

// Toe returns individual toe for an OUTBOARD-pointing unit spin axis, positive
// for toe-in. Only the horizontal projection of the axis enters, so camber
// cannot leak into the toe reading.
func Toe(outboardAxis geom.Vec3, p Position) Angle {
	a := outboardAxis.Unit()
	return Angle(math.Atan2(a.X, p.SideSign()*a.Y))
}

// Caster returns the caster angle from an UPWARD-pointing unit steering axis:
// its tilt in side view, positive when the top leans rearward.
func Caster(upSteeringAxis geom.Vec3) Angle {
	k := upSteeringAxis.Unit()
	return Angle(math.Atan2(-k.X, k.Z))
}

// SAIAngle returns steering axis inclination from an UPWARD-pointing unit
// steering axis: its tilt in front view, positive when the top leans inboard.
func SAIAngle(upSteeringAxis geom.Vec3, p Position) Angle {
	k := upSteeringAxis.Unit()
	return Angle(math.Atan2(-p.SideSign()*k.Y, k.Z))
}

// steeringGroundGeometry intersects the steering axis with the road plane and
// reports scrub radius and mechanical trail.
//
// Scrub radius is positive when the axis meets the ground INBOARD of the tyre's
// contact patch (the "positive scrub" of most older cars). Trail is positive
// when the axis meets the ground AHEAD of the contact patch, which is what
// makes the steering self-centre.
func steeringGroundGeometry(k, axisPoint geom.Vec3, w WheelObs, p Position, cam Angle) (scrub, trail float64, ok bool) {
	if math.Abs(k.Z) < 0.1 {
		return 0, 0, false // axis nearly horizontal: nonsense input
	}
	ground := axisPoint.Add(k.Scale(-axisPoint.Z / k.Z))

	r := w.RollingRadiusMM
	if r <= 0 {
		r = w.Center.Z // wheel centre height above the road plane
	}
	// The contact patch shifts outboard by r·sin(camber) — small, but the same
	// order as the scrub radius itself on a modern car.
	contactY := w.Center.Y + p.SideSign()*r*math.Sin(cam.Rad())

	return p.SideSign() * (contactY - ground.Y), ground.X - w.Center.X, true
}

func clamp1(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// structuralWarnings flags geometry that no amount of adjustment will fix.
func structuralWarnings(r Result) []string {
	var out []string
	if r.HasDimensions {
		if math.Abs(r.WheelbaseDiffMM) > 10 {
			out = append(out, "Колёсная база слева и справа отличается на "+fmtMM(r.WheelbaseDiffMM)+
				" — вероятна деформация кузова или рычагов. Регулировка углов сама по себе увод не устранит.")
		}
		if math.Abs(r.Front.SetbackMM) > 12 {
			out = append(out, "Сдвиг передних колёс (setback) "+fmtMM(r.Front.SetbackMM)+
				" — проверьте геометрию подрамника и лонжеронов.")
		}
	}
	if math.Abs(r.ThrustAngle.Deg()) > 0.5 {
		out = append(out, "Большой угол тяги ("+r.ThrustAngle.FormatDegMin()+
			"): автомобиль будет ехать «боком», а руль не встанет ровно, пока не отрегулирована задняя ось.")
	}
	for _, p := range [2]Position{FL, FR} {
		w := r.Wheels[p.String()]
		o := r.Wheels[p.Opposite().String()]
		if w.IncludedAngle == nil || o.IncludedAngle == nil {
			continue
		}
		if math.Abs((*w.IncludedAngle - *o.IncludedAngle).Deg()) > 1.0 {
			out = append(out, "Включённый угол (SAI + развал) слева и справа отличается более чем на 1° — "+
				"это признак погнутой стойки, поворотного кулака или рычага, а не ошибки регулировки.")
			break
		}
	}
	return out
}

func fmtMM(v float64) string {
	return strconv.FormatFloat(math.Abs(v), 'f', 1, 64) + " мм"
}
