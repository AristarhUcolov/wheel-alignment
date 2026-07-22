package vision_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// TestWheelSpinAxisFromImages is the optical camber path end to end, but through
// the public entry point the web server calls: rendered photographs of a spun
// wheel in, spin axis and camber out. It repeats the guarantee of
// TestImageToCamber — a deliberately crooked target does not matter — while
// exercising WheelSpinAxis and CamberFromSpinAxis, which are what the UI uses.
func TestWheelSpinAxisFromImages(t *testing.T) {
	const camberDeg, clampErr = -1.35, 2.5

	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(71))

	wheel := simulate.WheelSpec{
		Camber: align.Deg(camberDeg), Toe: align.Deg(0.2),
		Center: geom.V(0, 760, 315),
	}
	spinAxis := wheel.SpinAxis(align.FL)

	// Camera outboard of the wheel, backed off and turned to look at it — the
	// only place a person can stand.
	camPos := geom.V(-350, 1750, 560)
	fwd := wheel.Center.Sub(camPos).Unit()
	right := geom.V(0, 0, 1).Cross(fwd).Unit()
	down := fwd.Cross(right).Unit()
	rCam := geom.FromRows(right, down, fwd)
	vehicleToCam := geom.Pose{R: rCam, T: rCam.MulVec(camPos).Neg()}

	// Gravity is straight down in vehicle coordinates; in the camera frame it is
	// the same vector rotated into it. A levelled camera would measure this.
	gravityCam := vehicleToCam.ApplyDir(geom.V(0, 0, -1))

	// Crooked mount.
	tiltAxis := spinAxis.Any()
	mountR := geom.Rodrigues(tiltAxis, geom.Rad(clampErr)).
		Mul(geom.RotationBetween(geom.V(0, 0, 1), spinAxis))
	mountT := wheel.Center.Add(spinAxis.Scale(90))

	var imgs []*vision.Gray
	for _, spin := range []float64{0, 40, 83, 129, 178, 226} {
		spinR := geom.Rodrigues(spinAxis, geom.Rad(spin))
		inVehicle := geom.Pose{
			R: spinR.Mul(mountR),
			T: spinR.MulVec(mountT.Sub(wheel.Center)).Add(wheel.Center),
		}
		imgs = append(imgs, renderBoard(cam, tg, vehicleToCam.Mul(inVehicle), 3, 0.006, rng))
	}

	res, err := vision.WheelSpinAxis(cam, tg, imgs, vision.DetectOptions{})
	if err != nil {
		t.Fatalf("WheelSpinAxis: %v", err)
	}
	if res.Used != len(imgs) {
		t.Errorf("used %d of %d frames", res.Used, len(imgs))
	}

	gotCamber := vision.CamberFromSpinAxis(res.Axis, gravityCam)
	t.Logf("поворот %.0f°, биение мишени %.2f°, невязка оси %.2f мм; развал %.4f° (задано %.2f°)",
		res.SweepDeg, res.RunoutDeg, res.AxisResidualMM, gotCamber, camberDeg)

	if d := math.Abs(gotCamber - camberDeg); d > 0.1 {
		t.Errorf("развал восстановлен с ошибкой %.4f° — оптический тракт камбера расходится", d)
	}
	// The recovered runout must report roughly the real mounting error, since
	// that is the diagnostic the operator relies on.
	if d := math.Abs(res.RunoutDeg - clampErr); d > 1.0 {
		t.Errorf("биение мишени показано как %.2f°, а реальный перекос %.1f°", res.RunoutDeg, clampErr)
	}
}

// TestWheelSpinAxisRejectsTooFewFrames: fewer than three solved frames cannot
// determine an axis, and the failure must be an explained error rather than a
// confident wrong answer.
func TestWheelSpinAxisRejectsTooFewFrames(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(2))

	// Two good frames and one blank.
	pose := geom.Pose{R: geom.RotY(geom.Rad(-20)).Mul(geom.RotX(geom.Rad(12))), T: geom.V(0, 0, 900)}
	imgs := []*vision.Gray{
		renderBoard(cam, tg, pose, 3, 0.004, rng),
		renderBoard(cam, tg, geom.Pose{R: pose.R.Mul(geom.RotZ(geom.Rad(20))), T: pose.T}, 3, 0.004, rng),
		vision.NewGray(cam.Width, cam.Height),
	}
	res, err := vision.WheelSpinAxis(cam, tg, imgs, vision.DetectOptions{})
	if err == nil {
		t.Error("three frames with only two boards must be refused")
	}
	if res.Used != 2 {
		t.Errorf("expected 2 usable frames, got %d", res.Used)
	}
	// The blank frame must be reported as failed, not silently dropped.
	if len(res.Frames) != 3 || res.Frames[2].OK {
		t.Errorf("the blank frame should be recorded as a failure: %+v", res.Frames)
	}
}

// TestCamberFromSpinAxisConventions pins the sign both ways: positive camber
// leans the top of the wheel outboard, and the formula is frame-free.
func TestCamberFromSpinAxisConventions(t *testing.T) {
	// Work directly in a vehicle-like frame: +Z up, gravity down.
	gravity := geom.V(0, 0, -1)
	for _, p := range []align.Position{align.FL, align.FR} {
		for _, c := range []float64{-2.5, -1, 0, 0.75, 3} {
			axis := simulate.WheelSpec{Camber: align.Deg(c)}.SpinAxis(p) // outboard
			got := vision.CamberFromSpinAxis(axis, gravity)
			if math.Abs(got-c) > 1e-9 {
				t.Errorf("%s camber %.2f° recovered as %.4f°", p, c, got)
			}
		}
	}
	// And it must be invariant to the frame: rotate axis and gravity together,
	// the answer cannot change.
	axis := simulate.WheelSpec{Camber: align.Deg(-1.3)}.SpinAxis(align.FL)
	rot := geom.RotZ(0.6).Mul(geom.RotX(-0.4))
	got := vision.CamberFromSpinAxis(rot.MulVec(axis), rot.MulVec(geom.V(0, 0, -1)))
	if math.Abs(got-(-1.3)) > 1e-9 {
		t.Errorf("camber changed under a shared rotation: %.6f°", got)
	}
}
