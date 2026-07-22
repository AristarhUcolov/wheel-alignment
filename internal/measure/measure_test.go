package measure_test

import (
	"math"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
)

// TestSweepSolveIsExact validates the exact sweep inversion. A wheel with known
// caster and SAI is steered in the forward model, the camber swing is read off
// exactly as an operator with a magnetic gauge would, and both angles must come
// back to what we started with — to machine precision, across the whole range
// of cars from a 1970s saloon to a modern one with 8° of caster.
func TestSweepSolveIsExact(t *testing.T) {
	for _, p := range []align.Position{align.FL, align.FR} {
		for _, caster := range []float64{-1, 0, 1.5, 3, 5.5, 8} {
			for _, sai := range []float64{0, 9, 14} {
				for _, camber := range []float64{-1.5, 0, 0.75} {
					for _, half := range []float64{10, 20} {
						v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
							p: {
								Caster: align.Deg(caster),
								SAI:    align.Deg(sai),
								Camber: align.Deg(camber),
								Toe:    align.Deg(0.15),
							},
						}}
						out, in, straight := v.SweepReadings(p, align.Deg(half))
						s := measure.SweepReading{
							CamberOut: out, CamberIn: in,
							CamberStraight: straight, HasStraight: true,
							ToeStraight: align.Deg(0.15),
							HalfSweep:   align.Deg(half),
						}
						sol, err := s.Solve(p)
						if err != nil {
							t.Fatalf("%s caster=%.1f sai=%.1f: %v", p, caster, sai, err)
						}
						if d := math.Abs(sol.Caster.Deg() - caster); d > 1e-6 {
							t.Errorf("%s sweep±%.0f°: caster %.2f° sai %.1f° camber %.2f° -> %.6f° (off by %.6f°)",
								p, half, caster, sai, camber, sol.Caster.Deg(), d)
						}
						if sol.SAI == nil {
							t.Fatalf("%s: SAI missing from an exact solve", p)
						}
						if d := math.Abs(sol.SAI.Deg() - sai); d > 1e-6 {
							t.Errorf("%s sweep±%.0f°: SAI %.1f° -> %.6f° (off by %.6f°)",
								p, half, sai, sol.SAI.Deg(), d)
						}
					}
				}
			}
		}
	}
}

// TestExactSolveBeatsClassicFormula quantifies what the exact inversion buys.
// The textbook "camber swing × 1.5" is first order in camber and silently
// couples caster to SAI; on a modern high-caster car that costs a quarter of a
// degree, which is more than the whole factory tolerance on some vehicles.
func TestExactSolveBeatsClassicFormula(t *testing.T) {
	const caster, sai = 8.0, 14.0
	v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
		align.FL: {Caster: align.Deg(caster), SAI: align.Deg(sai), Camber: align.Deg(-1.5), Toe: align.Deg(0.15)},
	}}
	out, in, straight := v.SweepReadings(align.FL, align.Deg(20))
	s := measure.SweepReading{
		CamberOut: out, CamberIn: in, CamberStraight: straight, HasStraight: true,
		ToeStraight: align.Deg(0.15), HalfSweep: align.Deg(20),
	}

	sol, err := s.Solve(align.FL)
	if err != nil {
		t.Fatal(err)
	}
	classic, err := s.CasterClassic()
	if err != nil {
		t.Fatal(err)
	}
	exactErr := math.Abs(sol.Caster.Deg() - caster)
	classicErr := math.Abs(classic.Deg() - caster)

	t.Logf("caster %.1f° / SAI %.1f°: exact solve off by %.4f°, classic formula off by %.4f°",
		caster, sai, exactErr, classicErr)
	if classicErr < 0.15 {
		t.Errorf("expected the classic formula to be materially wrong here, it was off by only %.4f°", classicErr)
	}
	if exactErr > 1e-6 {
		t.Errorf("exact solve should be exact, off by %.6f°", exactErr)
	}
}

// TestSweepWithoutStraightReading falls back gracefully rather than refusing.
func TestSweepWithoutStraightReading(t *testing.T) {
	v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
		align.FR: {Caster: align.Deg(3), SAI: align.Deg(10), Camber: align.Deg(-0.5)},
	}}
	out, in, _ := v.SweepReadings(align.FR, align.Deg(20))
	sol, err := measure.SweepReading{CamberOut: out, CamberIn: in, HalfSweep: align.Deg(20)}.Solve(align.FR)
	if err != nil {
		t.Fatal(err)
	}
	if sol.SAI != nil {
		t.Error("SAI cannot be known without a straight-ahead reading")
	}
	if len(sol.Warnings) == 0 {
		t.Error("the approximate path must say so")
	}
	if d := math.Abs(sol.Caster.Deg() - 3); d > 0.2 {
		t.Errorf("approximate caster off by %.3f°, expected within 0.2°", d)
	}
}

// TestSweepCasterAccuracyVsSweepAngle documents the practical trade-off: a 20°
// sweep is materially better than a 10° one, which is why every service manual
// asks for 20° where the steering allows it.
func TestSweepCasterAccuracyVsSweepAngle(t *testing.T) {
	const trueCaster = 4.0
	v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
		align.FL: {Caster: align.Deg(trueCaster), SAI: align.Deg(12), Camber: align.Deg(-1)},
	}}

	errAt := func(halfSweep float64) float64 {
		out, in, st := v.SweepReadings(align.FL, align.Deg(halfSweep))
		// Add a realistic gauge quantisation of 0.05° to each reading.
		s := measure.SweepReading{
			CamberOut: out + align.Deg(0.05), CamberIn: in - align.Deg(0.05),
			CamberStraight: st, HasStraight: true,
			HalfSweep: align.Deg(halfSweep),
		}
		sol, err := s.Solve(align.FL)
		if err != nil {
			t.Fatal(err)
		}
		return math.Abs(sol.Caster.Deg() - trueCaster)
	}
	e10, e20 := errAt(10), errAt(20)
	if e20 >= e10 {
		t.Errorf("a 20° sweep should beat a 10° sweep: %.3f° vs %.3f°", e20, e10)
	}
	t.Logf("gauge error 0.05° gives caster error %.3f° at 10° sweep, %.3f° at 20° sweep", e10, e20)
}

// TestSweepRejectsSwappedReadings: entering "out" and "in" the wrong way round
// is the commonest sweep mistake. It cannot always be detected, but it does
// flip the sign, so at minimum the sign must be wrong rather than silently
// plausible.
func TestSweepRejectsSwappedReadings(t *testing.T) {
	v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
		align.FL: {Caster: align.Deg(4), SAI: align.Deg(12)},
	}}
	out, in, st := v.SweepReadings(align.FL, align.Deg(20))
	base := measure.SweepReading{CamberStraight: st, HasStraight: true, HalfSweep: align.Deg(20)}

	g, s2 := base, base
	g.CamberOut, g.CamberIn = out, in
	s2.CamberOut, s2.CamberIn = in, out

	good, err := g.Solve(align.FL)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := s2.Solve(align.FL)
	if err != nil {
		return // refusing outright is an acceptable response too
	}
	if math.Signbit(good.Caster.Rad()) == math.Signbit(bad.Caster.Rad()) {
		t.Errorf("swapping the sweep readings must flip the sign of caster: %v vs %v",
			good.Caster.FormatDegMin(), bad.Caster.FormatDegMin())
	}
}

// TestSweepTooSmall guards the divide-by-nearly-zero.
func TestSweepTooSmall(t *testing.T) {
	s := measure.SweepReading{CamberOut: align.Deg(0.1), CamberIn: align.Deg(-0.1), HalfSweep: align.Deg(2)}
	if _, err := s.Solve(align.FL); err == nil {
		t.Error("a 2° sweep should be rejected")
	}
}

// TestSAINoiseAmplification is the honest limitation of getting SAI out of a
// camber sweep: the even part of the sweep is divided by (1 − cos θ), which is
// 0.06 at 20°, so gauge error is magnified about seventeen-fold. The maths is
// exact; the measurement is not, and the program says so rather than printing a
// confident number.
func TestSAINoiseAmplification(t *testing.T) {
	const trueSAI, gaugeErr = 12.0, 0.1
	v := simulate.Vehicle{Wheels: map[align.Position]simulate.WheelSpec{
		align.FL: {Caster: align.Deg(3.5), SAI: align.Deg(trueSAI), Camber: align.Deg(-0.8)},
	}}
	out, in, st := v.SweepReadings(align.FL, align.Deg(20))

	clean, err := measure.SweepReading{
		CamberOut: out, CamberIn: in, CamberStraight: st, HasStraight: true, HalfSweep: align.Deg(20),
	}.Solve(align.FL)
	if err != nil {
		t.Fatal(err)
	}
	if d := math.Abs(clean.SAI.Deg() - trueSAI); d > 1e-6 {
		t.Errorf("with perfect readings SAI must be exact, off by %.6f°", d)
	}
	if len(clean.Warnings) == 0 {
		t.Error("the SAI result must carry its amplification warning")
	}

	// Now perturb both swept readings the same way, which is what a gauge with
	// a small zero offset does.
	noisy, err := measure.SweepReading{
		CamberOut: out + align.Deg(gaugeErr), CamberIn: in + align.Deg(gaugeErr),
		CamberStraight: st, HasStraight: true, HalfSweep: align.Deg(20),
	}.Solve(align.FL)
	if err != nil {
		t.Fatal(err)
	}
	saiErr := math.Abs(noisy.SAI.Deg() - trueSAI)
	casterErr := math.Abs(noisy.Caster.Deg() - 3.5)
	t.Logf("a %.2f° gauge offset costs %.3f° of SAI but only %.3f° of caster", gaugeErr, saiErr, casterErr)
	if saiErr < 5*casterErr {
		t.Errorf("SAI should be far more noise-sensitive than caster: %.3f° vs %.3f°", saiErr, casterErr)
	}
}

// TestConeFitBeatsTwoPointSweep is the case for the optical path: fitting the
// steering axis directly recovers caster and SAI essentially exactly, with no
// small-angle assumption and no error amplification.
func TestConeFitSteeringAxis(t *testing.T) {
	for _, p := range []align.Position{align.FL, align.FR} {
		w := simulate.WheelSpec{
			Caster: align.Deg(4.25), SAI: align.Deg(13), Camber: align.Deg(-1.1), Toe: align.Deg(0.1),
		}
		var axes []geom.Vec3
		for _, steer := range []float64{-20, -10, 0, 12, 22} {
			axes = append(axes, w.SpinAxisSteered(p, align.Deg(steer)))
		}
		k, err := measure.SteeringAxisFromSweep(axes, geom.V(0, 0, 1))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if d := math.Abs(align.Caster(k).Deg() - 4.25); d > 1e-6 {
			t.Errorf("%s: cone-fit caster off by %.8f°", p, d)
		}
		if d := math.Abs(align.SAIAngle(k, p).Deg() - 13); d > 1e-6 {
			t.Errorf("%s: cone-fit SAI off by %.8f°", p, d)
		}
	}
}

// TestConeFitRejectsTinySweep: with almost no steering the plane through the
// sampled axes is arbitrary, and returning a confident wrong answer would be
// worse than refusing.
func TestConeFitRejectsTinySweep(t *testing.T) {
	w := simulate.WheelSpec{Caster: align.Deg(4), SAI: align.Deg(12)}
	var axes []geom.Vec3
	for _, steer := range []float64{-0.1, 0, 0.1} {
		axes = append(axes, w.SpinAxisSteered(align.FL, align.Deg(steer)))
	}
	if _, err := measure.SteeringAxisFromSweep(axes, geom.V(0, 0, 1)); err == nil {
		t.Error("a 0.2° sweep should be refused")
	}
}

// TestStringToe covers the tape-measure path in both string arrangements.
func TestStringToe(t *testing.T) {
	span := align.Inches(15)

	// String outside the wheel: toe-in pushes the front of the rim away.
	in := measure.StringToe{FrontMM: 52.0, RearMM: 50.0, SpanMM: span, Side: measure.LineOutside}
	got, err := in.Toe()
	if err != nil {
		t.Fatal(err)
	}
	if got <= 0 {
		t.Errorf("front reading larger with an outside string means toe-in, got %s", got.FormatDegMin())
	}
	want := math.Atan2(2.0, span)
	if d := math.Abs(got.Rad() - want); d > 1e-12 {
		t.Errorf("toe = %v, want %v", got.Rad(), want)
	}

	// Inside string: the same numbers now mean toe-out.
	inside := measure.StringToe{FrontMM: 52.0, RearMM: 50.0, SpanMM: span, Side: measure.LineInside}
	got2, _ := inside.Toe()
	if got2 != -got {
		t.Errorf("flipping the string side should flip the sign: %v vs %v", got2, got)
	}

	if _, err := (measure.StringToe{FrontMM: 1, RearMM: 2}).Toe(); err == nil {
		t.Error("a missing span should be an error, not a division by zero")
	}
}

// TestStringBoxCheck is the setup error this program exists to prevent: with
// unequal front and rear tracks, equal string offsets front and rear mean the
// string is NOT parallel to the vehicle centreline.
func TestStringBoxCheck(t *testing.T) {
	// Front track 1520, rear 1500: the strings must sit 10 mm further out at
	// the front hubs than at the rear ones.
	b := measure.StringBox{
		LeftFrontMM: 100, LeftRearMM: 110,
		RightFrontMM: 100, RightRearMM: 110,
		TrackFrontMM: 1520, TrackRearMM: 1500,
	}
	if want := b.RequiredFrontMinusRear(); math.Abs(want-(-10)) > 1e-9 {
		t.Fatalf("required front−rear offset = %.2f, want −10", want)
	}
	if problems := b.Check(1.0); len(problems) != 0 {
		t.Errorf("a correctly squared box should pass, got %v", problems)
	}

	// The "obvious" setup — equal offsets front and rear — is wrong here.
	bad := b
	bad.LeftFrontMM, bad.LeftRearMM = 110, 110
	bad.RightFrontMM, bad.RightRearMM = 110, 110
	if problems := bad.Check(1.0); len(problems) != 2 {
		t.Errorf("equal offsets with unequal tracks should flag both strings, got %v", problems)
	}
}

// TestInclinometerRunout: reading the same spot on the rim half a revolution
// later cancels the rim's own error.
func TestInclinometerRunout(t *testing.T) {
	const trueCamber, rimError = -1.2, 0.4
	i := measure.Inclinometer{
		At0Deg: trueCamber + rimError,
		// The gauge sits on the same spot on the rim, now carried to the other
		// side of the wheel, so the rim's contribution has reversed.
		At180Deg: trueCamber - rimError,
		Has180:   true,
	}
	cam, runout := i.Camber()
	if d := math.Abs(cam.Deg() - trueCamber); d > 1e-9 {
		t.Errorf("compensated camber = %.4f°, want %.4f°", cam.Deg(), trueCamber)
	}
	if d := math.Abs(runout.Deg() - rimError); d > 1e-9 {
		t.Errorf("detected rim runout = %.4f°, want %.4f°", runout.Deg(), rimError)
	}

	// Without the second reading the rim error passes straight through.
	single := measure.Inclinometer{At0Deg: trueCamber + rimError}
	if cam, _ := single.Camber(); math.Abs(cam.Deg()-(trueCamber+rimError)) > 1e-9 {
		t.Error("an uncompensated reading should be reported as-is")
	}
}

// TestManualSessionEndToEnd walks the whole cheap path: string readings and
// gauge readings in, a full report out, with thrust angle computed from the
// rear wheels just as the optical path does.
func TestManualSessionEndToEnd(t *testing.T) {
	span := align.Inches(15)
	mk := func(camberDeg, toeMM float64) measure.ManualWheel {
		return measure.ManualWheel{
			Camber:        measure.Inclinometer{At0Deg: camberDeg, At180Deg: camberDeg, Has180: true},
			Toe:           measure.StringToe{FrontMM: 50 + toeMM, RearMM: 50, SpanMM: span, Side: measure.LineOutside},
			RimDiameterMM: span,
		}
	}
	s := measure.ManualSession{
		TrackFrontMM: 1520, TrackRearMM: 1500,
		Box: &measure.StringBox{
			LeftFrontMM: 100, LeftRearMM: 110, RightFrontMM: 100, RightRearMM: 110,
			TrackFrontMM: 1520, TrackRearMM: 1500,
		},
		Wheels: map[align.Position]measure.ManualWheel{
			align.FL: mk(-0.9, 1.0),
			align.FR: mk(-0.2, 1.0),
			align.RL: mk(-1.4, 1.5),
			align.RR: mk(-1.1, 0.5),
		},
	}
	res, err := s.Result()
	if err != nil {
		t.Fatal(err)
	}

	if d := math.Abs(res.Wheels["FL"].Camber.Deg() - (-0.9)); d > 1e-9 {
		t.Errorf("FL camber = %.4f°", res.Wheels["FL"].Camber.Deg())
	}
	// Rear left is toed in 1.5 mm and rear right 0.5 mm, so the rear axle aims
	// right: a positive thrust angle.
	if res.ThrustAngle <= 0 {
		t.Errorf("thrust angle should be positive here, got %s", res.ThrustAngle.FormatDegMin())
	}
	// Total front toe is 2 mm across a 15" span, split evenly.
	wantTotal := 2 * math.Atan2(1.0, span)
	if d := math.Abs(res.Front.TotalToe.Rad() - wantTotal); d > 1e-9 {
		t.Errorf("front total toe = %v, want %v", res.Front.TotalToe.Rad(), wantTotal)
	}
	if len(res.Warnings) == 0 {
		t.Log("no warnings raised")
	}
}

// TestManualSessionFlagsUnsquaredBox: a bad string setup must surface before
// anyone starts turning tie rods.
func TestManualSessionFlagsUnsquaredBox(t *testing.T) {
	span := align.Inches(15)
	w := measure.ManualWheel{
		Camber:        measure.Inclinometer{At0Deg: -1, At180Deg: -1, Has180: true},
		Toe:           measure.StringToe{FrontMM: 51, RearMM: 50, SpanMM: span, Side: measure.LineOutside},
		RimDiameterMM: span,
	}
	s := measure.ManualSession{
		TrackFrontMM: 1520, TrackRearMM: 1500,
		Box: &measure.StringBox{
			LeftFrontMM: 110, LeftRearMM: 110, RightFrontMM: 110, RightRearMM: 110,
			TrackFrontMM: 1520, TrackRearMM: 1500,
		},
		Wheels: map[align.Position]measure.ManualWheel{
			align.FL: w, align.FR: w, align.RL: w, align.RR: w,
		},
	}
	res, err := s.Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) < 2 {
		t.Errorf("an unsquared string box should be flagged loudly, got %v", res.Warnings)
	}
}
