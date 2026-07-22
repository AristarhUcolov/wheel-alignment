package vision_test

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// testCamera is a plausible 1080p webcam with real barrel distortion, so that
// nothing here accidentally passes because the lens model was trivial.
func testCamera() vision.Camera {
	return vision.Camera{
		Width: 1920, Height: 1080,
		Fx: 1450, Fy: 1452,
		Cx: 953, Cy: 542,
		K1: -0.28, K2: 0.11, P1: 0.0007, P2: -0.0004, K3: -0.02,
		Calibrated: true, CalibrationRMSPx: 0.21,
	}
}

// TestCameraRoundTrip: projecting a point and then un-projecting the pixel must
// return the original ray. This pins the distortion model against its own
// iterative inverse.
func TestCameraRoundTrip(t *testing.T) {
	cam := testCamera()
	for _, p := range []geom.Vec3{
		{X: 0, Y: 0, Z: 2000},
		{X: 400, Y: -250, Z: 1800},
		{X: -900, Y: 500, Z: 2500},
		{X: 1100, Y: 600, Z: 2000}, // well out toward the frame corner
	} {
		px, ok := cam.Project(p)
		if !ok {
			t.Fatalf("%v did not project", p)
		}
		if px.X < 0 || px.X > 1920 || px.Y < 0 || px.Y > 1080 {
			t.Fatalf("%v projected outside the frame: %v", p, px)
		}
		ray := cam.Ray(px)
		want := p.Unit()
		if ang := geom.Deg(ray.AngleTo(want)); ang > 1e-9 {
			t.Errorf("%v: ray off by %.10f° after distort/undistort round trip", p, ang)
		}
	}

	// A point behind the camera has no projection and must say so.
	if _, ok := cam.Project(geom.V(0, 0, -100)); ok {
		t.Error("a point behind the camera must not project")
	}
}

// project renders a target at a known pose, optionally with detector noise.
func project(t *testing.T, cam vision.Camera, tg vision.Target, pose geom.Pose, noisePx float64, rng *rand.Rand) []vision.Correspondence {
	t.Helper()
	model := tg.ModelPoints()
	corr := make([]vision.Correspondence, len(model))
	for i, m := range model {
		px, ok := cam.Project(pose.Apply(m))
		if !ok {
			t.Fatalf("model point %d did not project", i)
		}
		if noisePx > 0 {
			px.X += rng.NormFloat64() * noisePx
			px.Y += rng.NormFloat64() * noisePx
		}
		corr[i] = vision.Correspondence{Model: m, Image: px}
	}
	return corr
}

func poseError(got, want geom.Pose) (angDeg, transMM float64) {
	_, ang := geom.AxisAngle(got.R.Mul(want.R.T()))
	return geom.Deg(ang), got.T.Sub(want.T).Len()
}

// TestPnPExactOnCleanData: with noise-free projections the solver must return
// the pose it was given, to numerical precision. Any looseness here would hide
// real errors in every test below.
func TestPnPExactOnCleanData(t *testing.T) {
	cam := testCamera()
	tg := vision.DefaultTarget()
	if err := tg.Validate(); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(1))

	for _, tilt := range []float64{5, 15, 30, 45} {
		for _, pan := range []float64{-30, 0, 20} {
			want := geom.Pose{
				R: geom.RotY(geom.Rad(pan)).Mul(geom.RotX(geom.Rad(tilt))).Mul(geom.RotZ(geom.Rad(12))),
				T: geom.V(120, -60, 2200),
			}
			corr := project(t, cam, tg, want, 0, rng)

			res, err := vision.SolvePnPPlanar(cam, corr)
			if err != nil {
				t.Fatalf("tilt %.0f pan %.0f: %v", tilt, pan, err)
			}
			ang, trans := poseError(res.Pose, want)
			// The floor here is set by Levenberg–Marquardt with a
			// central-difference Jacobian, which converges to roughly ε^(2/3)
			// relative — a few parts in 10⁻⁶ of a degree. That is four orders
			// of magnitude below anything the detector can deliver.
			if ang > 1e-5 || trans > 1e-2 {
				t.Errorf("tilt %.0f pan %.0f: pose off by %.9f° and %.6f mm (rms %.2e px)",
					tilt, pan, ang, trans, res.RMSPx)
			}
			if res.RMSPx > 1e-6 {
				t.Errorf("tilt %.0f pan %.0f: reprojection rms %.3e px on exact data", tilt, pan, res.RMSPx)
			}
		}
	}
}

// TestPnPWithDetectorNoise quantifies what corner-detection noise costs. A good
// subpixel detector achieves 0.05–0.1 px; this records what that buys in
// angular terms, which is the number that decides whether the optical mode can
// beat a magnetic angle gauge.
func TestPnPWithDetectorNoise(t *testing.T) {
	cam := testCamera()
	tg := vision.DefaultTarget()
	want := geom.Pose{
		R: geom.RotY(geom.Rad(25)).Mul(geom.RotX(geom.Rad(10))),
		T: geom.V(100, 0, 2200),
	}

	for _, noise := range []float64{0.05, 0.1, 0.3} {
		rng := rand.New(rand.NewSource(42))
		var sumAng, maxAng float64
		const trials = 60
		for i := 0; i < trials; i++ {
			corr := project(t, cam, tg, want, noise, rng)
			res, err := vision.SolvePnPPlanar(cam, corr)
			if err != nil {
				t.Fatal(err)
			}
			ang, _ := poseError(res.Pose, want)
			sumAng += ang
			maxAng = math.Max(maxAng, ang)
		}
		mean := sumAng / trials
		t.Logf("шум детектора %.2f пикс → средняя ошибка позы %.4f°, худшая %.4f°", noise, mean, maxAng)
		if noise <= 0.1 && mean > 0.1 {
			t.Errorf("noise %.2f px gave mean pose error %.4f°, expected better than 0.1°", noise, mean)
		}
	}
}

// TestPnPDetectsPlanarAmbiguity is the safety property. A tilted flat target
// that is small in the frame has two poses fitting nearly equally; picking the
// wrong one flips its tilt and puts a sign error into camber. The solver must
// notice and say so rather than pick one and look confident.
//
// What distinguishes the two is perspective foreshortening across the target,
// so the same target at the same tilt is ambiguous far away and unambiguous up
// close. Both regimes are checked, because a detector that cries ambiguity
// always would be as useless as one that never does.
func TestPnPDetectsPlanarAmbiguity(t *testing.T) {
	cam := testCamera()
	tg := vision.DefaultTarget()
	tilt := geom.RotY(geom.Rad(30)).Mul(geom.RotX(geom.Rad(15)))

	// Far away: weak perspective, the twin survives.
	rng := rand.New(rand.NewSource(7))
	far := geom.Pose{R: tilt, T: geom.V(0, 0, 9000)}
	corr := project(t, cam, tg, far, 0.25, rng)
	res, err := vision.SolvePnPPlanar(cam, corr)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("мишень далеко (9 м): rms %.3f, альтернатива %.3f, расхождение наклона %.1f°, неоднозначно=%v",
		res.RMSPx, res.AlternateRMSPx, res.AlternateTiltDeg, res.Ambiguous)
	if !res.Ambiguous {
		t.Error("a small, distant, tilted target must be reported as ambiguous")
	}
	if len(res.Warnings) == 0 {
		t.Error("an ambiguous pose must carry a warning explaining what to do")
	}

	// Up close: strong perspective, the twin is clearly worse.
	rng = rand.New(rand.NewSource(7))
	near := geom.Pose{R: tilt, T: geom.V(0, 0, 1200)}
	corr = project(t, cam, tg, near, 0.25, rng)
	res, err = vision.SolvePnPPlanar(cam, corr)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("мишень близко (1,2 м): rms %.3f, альтернатива %.3f, расхождение наклона %.1f°, неоднозначно=%v",
		res.RMSPx, res.AlternateRMSPx, res.AlternateTiltDeg, res.Ambiguous)
	if res.Ambiguous {
		t.Errorf("the same target at 1.2 m should be unambiguous (rms %.3f vs alternate %.3f)",
			res.RMSPx, res.AlternateRMSPx)
	}
}

// TestSquareTargetRejected: a square board is 90°-symmetric, so its orientation
// cannot be recovered — and on a wheel that means reading camber as toe.
func TestSquareTargetRejected(t *testing.T) {
	if err := (vision.Target{Cols: 6, Rows: 6, SquareMM: 30}).Validate(); err == nil {
		t.Error("a square checkerboard must be rejected")
	}
	if err := (vision.Target{Cols: 9, Rows: 6, SquareMM: 0}).Validate(); err == nil {
		t.Error("a target with no square size must be rejected")
	}
}

// TestUncalibratedCameraIsFlagged: guessed intrinsics give a systematic error
// that averaging cannot remove, so every result derived from them must say so.
func TestUncalibratedCameraIsFlagged(t *testing.T) {
	cam := vision.GuessFromFOV(1920, 1080, 60)
	if cam.Calibrated {
		t.Fatal("a guessed camera must not claim to be calibrated")
	}
	rng := rand.New(rand.NewSource(3))
	corr := project(t, cam, vision.DefaultTarget(),
		geom.Pose{R: geom.RotX(geom.Rad(20)), T: geom.V(0, 0, 2000)}, 0, rng)

	res, err := vision.SolvePnPPlanar(cam, corr)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "не откалибрована") {
			found = true
		}
	}
	if !found {
		t.Errorf("results from an uncalibrated camera must be flagged, got %v", res.Warnings)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// End-to-end: pixels to camber
// ─────────────────────────────────────────────────────────────────────────────

// TestOpticalPipelineRecoversWheelAngles is the whole optical path in one test.
//
// A wheel with known camber and toe carries a target that is deliberately
// clamped crooked — 2.5° out of the wheel plane, which is worse than any real
// clamp and far more than the entire camber tolerance. The wheel is spun about
// its own axis, each frame is rendered through the camera with detector noise,
// each frame is solved for pose, and the wheel's spin axis is recovered from
// the sequence.
//
// Runout compensation is the point: the target's own mounting error must vanish
// entirely, because the axis the target rotates ABOUT is the wheel's axis
// whatever angle the target sits at. If this test passes with a crooked clamp,
// clamps do not need to be precise — which is what makes home-made hardware
// viable.
func TestOpticalPipelineRecoversWheelAngles(t *testing.T) {
	const (
		camberDeg = -1.35
		toeDeg    = 0.22
		clampErr  = 2.5 // deliberate target mounting error, degrees
	)
	cam := testCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(11))

	// The wheel, in vehicle coordinates: front left, spin axis outboard.
	wheel := simulate.WheelSpec{
		Camber: align.Deg(camberDeg),
		Toe:    align.Deg(toeDeg),
		Center: geom.V(0, 760, 315),
	}
	spinAxis := wheel.SpinAxis(align.FL)

	// The camera stands outboard of the wheel, backed off and turned to look at
	// it — roughly where a person would put a tripod.
	camPos := geom.V(-600, 2400, 700)
	fwd := wheel.Center.Sub(camPos).Unit()
	right := geom.V(0, 0, 1).Cross(fwd).Unit()
	down := fwd.Cross(right).Unit()
	// Camera frame is X right, Y down, Z forward; its rows are those axes.
	rCam := geom.FromRows(right, down, fwd)
	vehicleToCam := geom.Pose{R: rCam, T: rCam.MulVec(camPos).Neg()}

	// The target is bolted to the wheel, mis-clamped by clampErr about an axis
	// in the wheel plane, and offset outboard as a real clamp would put it.
	tiltAxis := spinAxis.Any()
	mountR := geom.Rodrigues(tiltAxis, geom.Rad(clampErr)).
		Mul(geom.RotationBetween(geom.V(0, 0, 1), spinAxis))
	mountT := wheel.Center.Add(spinAxis.Scale(90))

	var poses []geom.Pose
	var worstRMS float64
	for _, spin := range []float64{0, 37, 75, 118, 152, 195, 240} {
		// Spinning the wheel rotates the target about the wheel's spin axis,
		// through the wheel centre — a jacked-up wheel turned by hand.
		spinR := geom.Rodrigues(spinAxis, geom.Rad(spin))
		inVehicle := geom.Pose{
			R: spinR.Mul(mountR),
			T: spinR.MulVec(mountT.Sub(wheel.Center)).Add(wheel.Center),
		}
		trueCamPose := vehicleToCam.Mul(inVehicle)

		corr := project(t, cam, tg, trueCamPose, 0.08, rng)
		res, err := vision.SolvePnPPlanar(cam, corr)
		if err != nil {
			t.Fatalf("spin %.0f°: %v", spin, err)
		}
		if res.Ambiguous {
			t.Errorf("spin %.0f°: pose reported ambiguous in a well-tilted view", spin)
		}
		worstRMS = math.Max(worstRMS, res.RMSPx)

		// Back to vehicle coordinates, which on a real rig is the fixed
		// camera-to-floor calibration.
		poses = append(poses, vehicleToCam.Inverse().Mul(res.Pose))
	}

	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		t.Fatal(err)
	}
	got := fit.Direction
	if got.Dot(spinAxis) < 0 {
		got = got.Neg()
	}

	gotCamber := align.Camber(got)
	gotToe := align.Toe(got, align.FL)

	t.Logf("худшая невязка PnP %.3f пикс; ось восстановлена с ошибкой %.4f°",
		worstRMS, geom.Deg(got.AngleTo(spinAxis)))
	t.Logf("развал %.4f° (задано %.4f°), схождение %.4f° (задано %.4f°)",
		gotCamber.Deg(), camberDeg, gotToe.Deg(), toeDeg)

	if d := math.Abs(gotCamber.Deg() - camberDeg); d > 0.05 {
		t.Errorf("развал восстановлен с ошибкой %.4f° — компенсация биения не сработала", d)
	}
	if d := math.Abs(gotToe.Deg() - toeDeg); d > 0.05 {
		t.Errorf("схождение восстановлено с ошибкой %.4f°", d)
	}

	// And the axis must pass through the wheel centre, which is what gives
	// track width, wheelbase and setback.
	off := fit.Point.Sub(wheel.Center)
	perp := off.Sub(got.Scale(off.Dot(got)))
	if perp.Len() > 3 {
		t.Errorf("восстановленная ось проходит мимо центра колеса на %.2f мм", perp.Len())
	}
}

// TestOpticalCasterSweep completes the picture: with the spin axis known from
// runout compensation, steering the wheel sweeps that axis around a cone whose
// axis is the steering axis. This is the optical caster measurement, and unlike
// the two-point camber sweep it involves no small-angle approximation and no
// error amplification.
func TestOpticalCasterSweep(t *testing.T) {
	const casterDeg, saiDeg = 4.6, 12.8

	wheel := simulate.WheelSpec{
		Camber: align.Deg(-1.1), Toe: align.Deg(0.15),
		Caster: align.Deg(casterDeg), SAI: align.Deg(saiDeg),
	}
	// Spin axes as the optical stage would recover them at each steering
	// position, with a residual error of 0.02° left over from pose noise.
	rng := rand.New(rand.NewSource(5))
	var axes []geom.Vec3
	for _, steer := range []float64{-18, -9, 0, 11, 21} {
		a := wheel.SpinAxisSteered(align.FL, align.Deg(steer))
		wobble := geom.Rodrigues(a.Any(), geom.Rad(0.02*rng.NormFloat64()))
		axes = append(axes, wobble.MulVec(a))
	}

	k, err := measure.SteeringAxisFromSweep(axes, geom.V(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	gotCaster := align.Caster(k)
	gotSAI := align.SAIAngle(k, align.FL)

	t.Logf("кастер %.3f° (задано %.1f°), SAI %.3f° (задано %.1f°)",
		gotCaster.Deg(), casterDeg, gotSAI.Deg(), saiDeg)

	// The optical route tolerates axis noise far better than the camber sweep
	// tolerates gauge noise: 0.02° of spin-axis error costs well under 0.1° of
	// caster, where a 0.1° gauge offset costs 1.8° of SAI on the manual path.
	if d := math.Abs(gotCaster.Deg() - casterDeg); d > 0.1 {
		t.Errorf("кастер восстановлен с ошибкой %.3f°", d)
	}
	if d := math.Abs(gotSAI.Deg() - saiDeg); d > 0.3 {
		t.Errorf("SAI восстановлен с ошибкой %.3f°", d)
	}
}
