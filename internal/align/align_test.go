package align_test

import (
	"math"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
)

// tolerance for angle comparisons, in degrees
const tolDeg = 1e-9

func assertDeg(t *testing.T, got align.Angle, wantDeg, tol float64, what string) {
	t.Helper()
	if math.Abs(got.Deg()-wantDeg) > tol {
		t.Errorf("%s = %.6f°, want %.6f° (tol %.g)", what, got.Deg(), wantDeg, tol)
	}
}

// TestCamberToeRoundTrip is the sign-convention contract. Every other test in
// the project rests on these two extractions being exact inverses of the
// forward model, on both sides of the car.
func TestCamberToeRoundTrip(t *testing.T) {
	for _, p := range align.AllPositions {
		for _, camber := range []float64{-3, -1.5, -0.25, 0, 0.5, 2} {
			for _, toe := range []float64{-1.2, -0.1, 0, 0.15, 0.9} {
				w := simulate.WheelSpec{Camber: align.Deg(camber), Toe: align.Deg(toe)}
				a := w.SpinAxis(p)

				if l := a.Len(); math.Abs(l-1) > 1e-12 {
					t.Fatalf("%s: spin axis not unit: %v", p, l)
				}
				// The axis must point outboard: +Y on the left, −Y on the right.
				if p.SideSign()*a.Y <= 0 {
					t.Fatalf("%s: spin axis %v does not point outboard", p, a)
				}
				assertDeg(t, align.Camber(a), camber, tolDeg, p.String()+" camber")
				assertDeg(t, align.Toe(a, p), toe, tolDeg, p.String()+" toe")
			}
		}
	}
}

// TestCasterSAIRoundTrip pins down the steering-axis conventions: caster
// positive rearward, SAI positive inboard, mirrored correctly side to side.
func TestCasterSAIRoundTrip(t *testing.T) {
	for _, p := range []align.Position{align.FL, align.FR} {
		for _, caster := range []float64{-1, 0, 2.5, 4.75, 9} {
			for _, sai := range []float64{0, 8, 13.5} {
				w := simulate.WheelSpec{Caster: align.Deg(caster), SAI: align.Deg(sai)}
				k := w.SteeringAxis(p)

				if k.Z <= 0 {
					t.Fatalf("%s: steering axis %v does not point up", p, k)
				}
				assertDeg(t, align.Caster(k), caster, tolDeg, p.String()+" caster")
				assertDeg(t, align.SAIAngle(k, p), sai, tolDeg, p.String()+" SAI")
			}
		}
	}
	// A positive-caster axis must lean rearward: its top is behind its base.
	k := simulate.WheelSpec{Caster: align.Deg(5)}.SteeringAxis(align.FL)
	if k.X >= 0 {
		t.Errorf("positive caster should tilt the axis rearward (−X), got %v", k)
	}
	// A positive-SAI axis must lean inboard: on the left wheel, toward −Y.
	k = simulate.WheelSpec{SAI: align.Deg(10)}.SteeringAxis(align.FL)
	if k.Y >= 0 {
		t.Errorf("positive SAI should tilt the left axis inboard (−Y), got %v", k)
	}
}

// TestSteerDirection checks that "steered outboard" means what the sweep
// procedure says it means, and that positive caster raises camber when the
// wheel is steered out — the physical effect the whole sweep method exploits.
func TestSteerDirection(t *testing.T) {
	for _, p := range []align.Position{align.FL, align.FR} {
		w := simulate.WheelSpec{Caster: align.Deg(5), SAI: align.Deg(10)}

		out := align.Camber(w.SpinAxisSteered(p, align.Deg(20)))
		in := align.Camber(w.SpinAxisSteered(p, align.Deg(-20)))
		if out <= in {
			t.Errorf("%s: with positive caster, camber steered out (%s) should exceed camber steered in (%s)",
				p, out.FormatDegMin(), in.FormatDegMin())
		}

		// Steering outboard must swing the wheel's heading away from the
		// centreline, i.e. toe must go negative (toe-out).
		toeOut := align.Toe(w.SpinAxisSteered(p, align.Deg(20)), p)
		if toeOut >= 0 {
			t.Errorf("%s: steering outboard by 20° should give toe-out, got %s", p, toeOut.FormatDegMin())
		}
	}
}

// TestFrameInvariance is the headline property: the reported angles must not
// depend on where the equipment stood. The same car is measured from a sensor
// pose that is rotated 25°/−17°/40° and translated metres away, and every
// number must come out identical.
func TestFrameInvariance(t *testing.T) {
	v := simulate.Nominal()

	base, err := align.Compute(v.WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatalf("compute in vehicle frame: %v", err)
	}

	sensor := geom.Pose{
		R: geom.RotZ(geom.Rad(40)).Mul(geom.RotY(geom.Rad(-17))).Mul(geom.RotX(geom.Rad(25))),
		T: geom.V(-1234, 5678, 910),
	}
	moved, err := align.Compute(v.Observe(sensor), align.FrameOptions{})
	if err != nil {
		t.Fatalf("compute in sensor frame: %v", err)
	}

	for _, p := range align.AllPositions {
		a, b := base.Wheels[p.String()], moved.Wheels[p.String()]
		assertDeg(t, b.Camber, a.Camber.Deg(), 1e-8, p.String()+" camber")
		assertDeg(t, b.ToeThrust, a.ToeThrust.Deg(), 1e-8, p.String()+" toe")
		if a.Caster != nil {
			assertDeg(t, *b.Caster, a.Caster.Deg(), 1e-8, p.String()+" caster")
			assertDeg(t, *b.SAI, a.SAI.Deg(), 1e-8, p.String()+" SAI")
		}
	}
	assertDeg(t, moved.ThrustAngle, base.ThrustAngle.Deg(), 1e-8, "thrust angle")

	if math.Abs(moved.WheelbaseLeftMM-base.WheelbaseLeftMM) > 1e-6 {
		t.Errorf("wheelbase changed with sensor pose: %.4f vs %.4f", moved.WheelbaseLeftMM, base.WheelbaseLeftMM)
	}
}

// TestAngleRecovery checks that Compute reproduces the angles the model was
// built from. Front toe is reported against the thrust line, so it is the
// geometric toe minus the side-signed thrust angle.
func TestAngleRecovery(t *testing.T) {
	v := simulate.Nominal()
	res, err := align.Compute(v.WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range align.AllPositions {
		w := v.Wheels[p]
		got := res.Wheels[p.String()]
		assertDeg(t, got.Camber, w.Camber.Deg(), 1e-9, p.String()+" camber")
		assertDeg(t, got.ToeGeometric, w.Toe.Deg(), 1e-9, p.String()+" geometric toe")
		if w.Steered {
			assertDeg(t, *got.Caster, w.Caster.Deg(), 1e-9, p.String()+" caster")
			assertDeg(t, *got.SAI, w.SAI.Deg(), 1e-9, p.String()+" SAI")
		}
	}
}

// TestThrustLine verifies the thrust angle formula and, more importantly, that
// re-referencing front toe to the thrust line redistributes it without
// changing the total — the property that makes total toe the trustworthy
// number when the rear axle is suspect.
func TestThrustLine(t *testing.T) {
	v := simulate.Nominal()
	res, err := align.Compute(v.WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}

	wantThrust := (v.Wheels[align.RL].Toe.Deg() - v.Wheels[align.RR].Toe.Deg()) / 2
	assertDeg(t, res.ThrustAngle, wantThrust, 1e-9, "thrust angle")

	wantTotal := v.Wheels[align.FL].Toe.Deg() + v.Wheels[align.FR].Toe.Deg()
	assertDeg(t, res.Front.TotalToe, wantTotal, 1e-9, "front total toe survives thrust referencing")

	fl := res.Wheels["FL"]
	assertDeg(t, fl.ToeThrust, v.Wheels[align.FL].Toe.Deg()-wantThrust, 1e-9, "FL thrust-referenced toe")
	fr := res.Wheels["FR"]
	assertDeg(t, fr.ToeThrust, v.Wheels[align.FR].Toe.Deg()+wantThrust, 1e-9, "FR thrust-referenced toe")
}

// TestThrustZeroWhenFrontFollowsRear is the physical statement of what the
// thrust reference is for: if the front wheels are aimed along the thrust line,
// their thrust-referenced toe is zero however crooked the rear axle is.
func TestThrustZeroWhenFrontFollowsRear(t *testing.T) {
	v := simulate.Nominal()
	thrust := (v.Wheels[align.RL].Toe - v.Wheels[align.RR].Toe) / 2

	for _, p := range []align.Position{align.FL, align.FR} {
		w := v.Wheels[p]
		w.Toe = align.Angle(p.SideSign()) * thrust
		v.Wheels[p] = w
	}
	res, err := align.Compute(v.WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertDeg(t, res.Wheels["FL"].ToeThrust, 0, 1e-9, "FL toe vs thrust line")
	assertDeg(t, res.Wheels["FR"].ToeThrust, 0, 1e-9, "FR toe vs thrust line")
}

// TestIncludedAngle: SAI + camber, the invariant that survives a bent part.
func TestIncludedAngle(t *testing.T) {
	v := simulate.Nominal()
	res, err := align.Compute(v.WheelSet(), align.FrameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []align.Position{align.FL, align.FR} {
		w, got := v.Wheels[p], res.Wheels[p.String()]
		assertDeg(t, *got.IncludedAngle, w.SAI.Deg()+w.Camber.Deg(), 1e-9, p.String()+" included angle")
	}
}

// TestToeMM round-trips the millimetre conversion and pins the rim-diameter
// dependency that trips people up when a spec sheet quotes toe in mm.
func TestToeMM(t *testing.T) {
	a := align.Deg(0.25)
	rim13 := align.Inches(13)
	rim17 := align.Inches(17)

	mm13, mm17 := a.ToeMM(rim13), a.ToeMM(rim17)
	if mm17 <= mm13 {
		t.Errorf("the same angle must be more millimetres on a bigger rim: %.2f vs %.2f", mm17, mm13)
	}
	back := align.ToeAngleFromMM(mm13, rim13)
	assertDeg(t, back, a.Deg(), 1e-9, "toe mm round trip")

	// A ВАЗ-classic spec: 2 mm of total toe on a 13" rim.
	vaz := align.ToeAngleFromMM(2, rim13)
	if d := vaz.Deg(); d < 0.32 || d > 0.36 {
		t.Errorf("2 mm on a 13-inch rim should be about 0.35°, got %.3f°", d)
	}
}

// TestDegMinFormatting covers the seam where 59.996' must carry into a degree.
func TestDegMinFormatting(t *testing.T) {
	cases := []struct {
		deg  float64
		want string
	}{
		{0, "+0°00'"},
		{-0.5, "-0°30'"},
		{3.25, "+3°15'"},
		{-2.75, "-2°45'"},
		{0.99999, "+1°00'"},
	}
	for _, c := range cases {
		if got := align.Deg(c.deg).FormatDegMin(); got != c.want {
			t.Errorf("%.5f° formatted as %q, want %q", c.deg, got, c.want)
		}
	}
}

// TestRangeGrading covers the tolerance classification.
func TestRangeGrading(t *testing.T) {
	r := align.RangeMinMax(align.Deg(-1.0), align.Deg(0.0))
	if got := r.Grade(align.Deg(-0.5)); got != align.StatusGood {
		t.Errorf("mid-band should be good, got %v", got)
	}
	if got := r.Grade(align.Deg(-0.03)); got != align.StatusMarginal {
		t.Errorf("near the edge should be marginal, got %v", got)
	}
	if got := r.Grade(align.Deg(0.4)); got != align.StatusBad {
		t.Errorf("outside should be bad, got %v", got)
	}
	if got := r.Deviation(align.Deg(0.4)).Deg(); math.Abs(got-0.4) > 1e-9 {
		t.Errorf("deviation above max = %.3f, want 0.4", got)
	}
	if got := r.Deviation(align.Deg(-0.5)); got != 0 {
		t.Errorf("in-band deviation should be zero, got %v", got)
	}
}

// TestGravityReference exercises the cheap path: camber referenced to a
// measured gravity vector rather than to the plane the tyres stand on. On a
// level floor the two must agree.
func TestGravityReference(t *testing.T) {
	v := simulate.Nominal()
	ws := v.WheelSet()

	road, err := align.Compute(ws, align.FrameOptions{Mode: align.RefRoadPlane})
	if err != nil {
		t.Fatal(err)
	}
	grav, err := align.Compute(ws, align.FrameOptions{
		Mode:    align.RefGravity,
		Gravity: geom.V(0, 0, -1), // level floor: gravity straight down
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range align.AllPositions {
		assertDeg(t, grav.Wheels[p.String()].Camber, road.Wheels[p.String()].Camber.Deg(), 1e-9,
			p.String()+" camber, gravity vs road plane on a level floor")
	}
}

// TestSlopedFloorSplitsTheReferences is the reason both modes exist: on a floor
// that slopes sideways, a bubble gauge reads camber that is wrong by the slope,
// while the road-plane reference stays correct.
func TestSlopedFloorSplitsTheReferences(t *testing.T) {
	const slopeDeg = 1.5
	v := simulate.Nominal()
	// Tip the whole car sideways, as a car parked across a cambered floor sits.
	tilt := geom.Pose{R: geom.RotX(geom.Rad(slopeDeg))}
	ws := v.Observe(tilt)

	road, err := align.Compute(ws, align.FrameOptions{Mode: align.RefRoadPlane})
	if err != nil {
		t.Fatal(err)
	}
	// Gravity is still straight down in the original (world) frame.
	grav, err := align.Compute(ws, align.FrameOptions{Mode: align.RefGravity, Gravity: geom.V(0, 0, -1)})
	if err != nil {
		t.Fatal(err)
	}

	assertDeg(t, road.Wheels["FL"].Camber, v.Wheels[align.FL].Camber.Deg(), 1e-8,
		"road-plane camber is unaffected by floor slope")

	drift := grav.Wheels["FL"].Camber.Deg() - v.Wheels[align.FL].Camber.Deg()
	if math.Abs(math.Abs(drift)-slopeDeg) > 0.05 {
		t.Errorf("gravity-referenced camber should be off by the %.1f° floor slope, off by %.3f°", slopeDeg, drift)
	}
}
