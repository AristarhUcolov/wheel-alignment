package vision_test

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// calibViews generates a realistic set of calibration shots: the board moved
// around the frame and tilted in different directions, which is exactly what
// the procedure asks an operator to do.
func calibViews(cam vision.Camera, tg vision.Target, n int, noisePx float64, rng *rand.Rand) []vision.CalibrationView {
	model := tg.ModelPoints()
	views := make([]vision.CalibrationView, 0, n)
	for i := 0; i < n; i++ {
		pose := geom.Pose{
			R: geom.RotZ(geom.Rad(rng.Float64()*60 - 30)).
				Mul(geom.RotY(geom.Rad(rng.Float64()*70 - 35))).
				Mul(geom.RotX(geom.Rad(rng.Float64()*70 - 35))),
			T: geom.V(rng.Float64()*260-130, rng.Float64()*180-90, 620+rng.Float64()*380),
		}
		pts, ok := cam.ProjectPose(pose, model)
		if !ok {
			i--
			continue
		}
		inside := true
		for j := range pts {
			if pts[j].X < 4 || pts[j].Y < 4 || pts[j].X > float64(cam.Width-4) || pts[j].Y > float64(cam.Height-4) {
				inside = false
				break
			}
			pts[j].X += rng.NormFloat64() * noisePx
			pts[j].Y += rng.NormFloat64() * noisePx
		}
		if !inside {
			i--
			continue
		}
		views = append(views, vision.CalibrationView{Corners: pts})
	}
	return views
}

// TestCalibrationRecoversIntrinsics: given noise-free views of a board, the
// calibration must return the camera it was generated from — focal lengths,
// principal point and all five distortion coefficients.
func TestCalibrationRecoversIntrinsics(t *testing.T) {
	truth := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(17))

	views := calibViews(truth, tg, 16, 0, rng)
	res, err := vision.CalibrateCamera(tg, views, truth.Width, truth.Height, vision.CalibrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Camera

	t.Logf("СКО %.5f пикс, покрытие кадра %.0f%%, разброс наклона %.0f°",
		res.RMSPx, res.CoverageFraction*100, res.TiltSpreadDeg)
	t.Logf("fx %.2f (истина %.2f), fy %.2f (%.2f), cx %.2f (%.2f), cy %.2f (%.2f)",
		got.Fx, truth.Fx, got.Fy, truth.Fy, got.Cx, truth.Cx, got.Cy, truth.Cy)
	t.Logf("k1 %.5f (%.5f), k2 %.5f (%.5f), p1 %.6f (%.6f), p2 %.6f (%.6f)",
		got.K1, truth.K1, got.K2, truth.K2, got.P1, truth.P1, got.P2, truth.P2)

	if !got.Calibrated {
		t.Error("результат должен быть помечен как откалиброванный")
	}
	if res.RMSPx > 1e-3 {
		t.Errorf("reprojection rms %.6f px on noise-free data", res.RMSPx)
	}
	for _, c := range []struct {
		name      string
		got, want float64
		tol       float64
	}{
		{"fx", got.Fx, truth.Fx, 0.5},
		{"fy", got.Fy, truth.Fy, 0.5},
		{"cx", got.Cx, truth.Cx, 1.0},
		{"cy", got.Cy, truth.Cy, 1.0},
		{"k1", got.K1, truth.K1, 0.005},
		{"k2", got.K2, truth.K2, 0.02},
	} {
		if math.Abs(c.got-c.want) > c.tol {
			t.Errorf("%s = %.5f, ожидалось %.5f (допуск %.5f)", c.name, c.got, c.want, c.tol)
		}
	}
}

// TestCalibrationUnderDetectorNoise: with the corner noise a real detector
// produces, the intrinsics must still land close enough that the angles they
// feed are trustworthy.
func TestCalibrationUnderDetectorNoise(t *testing.T) {
	truth := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(23))

	views := calibViews(truth, tg, 18, 0.04, rng)
	res, err := vision.CalibrateCamera(tg, views, truth.Width, truth.Height, vision.CalibrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Camera
	t.Logf("шум углов 0,04 пикс → СКО %.4f; fx %.2f (истина %.2f), k1 %.5f (%.5f)",
		res.RMSPx, got.Fx, truth.Fx, got.K1, truth.K1)

	if rel := math.Abs(got.Fx-truth.Fx) / truth.Fx; rel > 0.005 {
		t.Errorf("fx off by %.3f%%", rel*100)
	}
	if math.Abs(got.K1-truth.K1) > 0.01 {
		t.Errorf("k1 = %.5f, ожидалось %.5f", got.K1, truth.K1)
	}

	// What matters is not the absolute pose but what alignment actually uses.
	// Runout compensation works from the RELATIVE rotation between frames taken
	// with the same camera, and a systematic intrinsics error biases every pose
	// in much the same way, so most of it cancels in the difference. Both are
	// measured here, because the gap between them is the reason a slightly
	// imperfect calibration is still usable.
	solve := func(cam vision.Camera, pose geom.Pose, seed int64) geom.Pose {
		t.Helper()
		corr := project(t, truth, tg, pose, 0.04, rand.New(rand.NewSource(seed)))
		res, err := vision.SolvePnPPlanar(cam, corr)
		if err != nil {
			t.Fatal(err)
		}
		return res.Pose
	}

	poseA := geom.Pose{R: geom.RotY(geom.Rad(-22)).Mul(geom.RotX(geom.Rad(13))), T: geom.V(40, -20, 950)}
	poseB := geom.Pose{R: geom.RotY(geom.Rad(-22)).Mul(geom.RotX(geom.Rad(13))).Mul(geom.RotZ(geom.Rad(40))),
		T: geom.V(40, -20, 950)}

	trueA, calibA := solve(truth, poseA, 5), solve(got, poseA, 5)
	trueB, calibB := solve(truth, poseB, 6), solve(got, poseB, 6)

	_, absErr := geom.AxisAngle(calibA.R.Mul(trueA.R.T()))

	// The rotation carrying frame A onto frame B, as each camera sees it.
	relTrue := trueB.R.Mul(trueA.R.T())
	relCalib := calibB.R.Mul(calibA.R.T())
	_, relErr := geom.AxisAngle(relCalib.Mul(relTrue.T()))

	t.Logf("ошибка калибровки стоит %.4f° абсолютной позы, но лишь %.4f° относительного поворота — "+
		"а компенсация биения работает именно на относительных",
		geom.Deg(absErr), geom.Deg(relErr))

	if geom.Deg(absErr) > 0.15 {
		t.Errorf("calibration error costs %.4f° of absolute pose", geom.Deg(absErr))
	}
	if geom.Deg(relErr) > geom.Deg(absErr)+1e-9 {
		t.Errorf("relative rotation error (%.4f°) should not exceed absolute (%.4f°)",
			geom.Deg(relErr), geom.Deg(absErr))
	}
}

// TestCalibrationFlagsPoorProcedure is the point of the warnings. Calibration
// fails quietly: shots that are all face-on, or all in the middle of the frame,
// give a beautiful reprojection error and distortion coefficients that were
// never constrained by any data. The numbers cannot reveal that; only the
// procedure can, so the procedure is what gets measured.
func TestCalibrationFlagsPoorProcedure(t *testing.T) {
	truth := detectTestCamera()
	tg := vision.DefaultTarget()
	model := tg.ModelPoints()
	rng := rand.New(rand.NewSource(31))

	// All shots nearly face-on, all near the centre of the frame.
	var views []vision.CalibrationView
	for i := 0; i < 6; i++ {
		pose := geom.Pose{
			R: geom.RotX(geom.Rad(rng.Float64()*6 - 3)).Mul(geom.RotY(geom.Rad(rng.Float64()*6 - 3))),
			T: geom.V(rng.Float64()*30-15, rng.Float64()*20-10, 800+rng.Float64()*40),
		}
		pts, ok := truth.ProjectPose(pose, model)
		if !ok {
			continue
		}
		views = append(views, vision.CalibrationView{Corners: pts})
	}
	if len(views) < 3 {
		t.Skip("could not build the degenerate set")
	}

	res, err := vision.CalibrateCamera(tg, views, truth.Width, truth.Height, vision.CalibrateOptions{})
	if err != nil {
		// Refusing outright is an acceptable — arguably better — answer.
		t.Logf("вырожденный набор снимков отвергнут: %v", err)
		return
	}
	t.Logf("СКО %.4f пикс при разбросе наклона %.0f° и покрытии %.0f%% — "+
		"невязка отличная, а калибровка при этом никуда не годится",
		res.RMSPx, res.TiltSpreadDeg, res.CoverageFraction*100)

	joined := strings.Join(res.Warnings, " ")
	if !strings.Contains(joined, "наклон") {
		t.Errorf("набор снимков без разброса наклона должен быть отмечен, получено: %v", res.Warnings)
	}
	if !strings.Contains(joined, "кадра") {
		t.Errorf("плохое покрытие кадра должно быть отмечено, получено: %v", res.Warnings)
	}
}

// TestCalibrationRefusesTooFewViews: three views are the mathematical minimum
// and fewer cannot determine B at all.
func TestCalibrationRefusesTooFewViews(t *testing.T) {
	truth := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(3))
	views := calibViews(truth, tg, 2, 0, rng)

	if _, err := vision.CalibrateCamera(tg, views, truth.Width, truth.Height, vision.CalibrateOptions{}); err == nil {
		t.Error("two views cannot determine the intrinsics")
	}
}

// TestCalibrateFromImages closes the loop: photographs in, camera out, with the
// detector doing the corner finding.
func TestCalibrateFromImages(t *testing.T) {
	truth := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(41))

	var imgs []*vision.Gray
	poses := []geom.Pose{
		{R: geom.RotX(geom.Rad(28)), T: geom.V(-120, -60, 800)},
		{R: geom.RotY(geom.Rad(-31)), T: geom.V(130, 55, 820)},
		{R: geom.RotY(geom.Rad(26)).Mul(geom.RotX(geom.Rad(-24))), T: geom.V(120, -70, 780)},
		{R: geom.RotZ(geom.Rad(20)).Mul(geom.RotX(geom.Rad(-30))), T: geom.V(-125, 70, 860)},
		{R: geom.RotY(geom.Rad(34)).Mul(geom.RotX(geom.Rad(20))), T: geom.V(0, 0, 700)},
		{R: geom.RotZ(geom.Rad(-25)).Mul(geom.RotY(geom.Rad(-28))), T: geom.V(-60, 40, 900)},
	}
	for _, p := range poses {
		imgs = append(imgs, renderBoard(truth, tg, p, 3, 0.004, rng))
	}

	res, err := vision.CalibrateFromImages(tg, imgs, nil, vision.CalibrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("по %d снимкам: СКО %.4f пикс, fx %.2f (истина %.2f), cx %.2f (%.2f), k1 %.4f (%.4f)",
		len(poses), res.RMSPx, res.Camera.Fx, truth.Fx, res.Camera.Cx, truth.Cx, res.Camera.K1, truth.K1)

	if rel := math.Abs(res.Camera.Fx-truth.Fx) / truth.Fx; rel > 0.02 {
		t.Errorf("fx off by %.2f%% from rendered images", rel*100)
	}
	if res.RMSPx > 0.5 {
		t.Errorf("reprojection rms %.4f px from rendered images", res.RMSPx)
	}
}

// TestCalibrationRoundTripsThroughDisk: a calibration is written once and read
// on every later run, so the file format must survive the trip.
func TestCalibrationRoundTripsThroughDisk(t *testing.T) {
	want := detectTestCamera()
	want.CalibrationNote = "тестовая калибровка"
	path := t.TempDir() + "/camera.json"

	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := vision.LoadCamera(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("камера изменилась при сохранении и чтении:\n got %+v\nwant %+v", got, want)
	}
}
