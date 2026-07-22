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

// renderBoard draws a checkerboard as the given camera would actually see it:
// every pixel is back-projected through the lens model onto the target plane
// and coloured by which square it lands in.
//
// Rendering by inverse ray casting rather than by warping an image keeps the
// test honest — perspective and distortion come out of the same camera model the
// detector will have to invert, and there is no resampling blur to flatter the
// subpixel stage. Supersampling stands in for the sensor's own area averaging.
func renderBoard(cam vision.Camera, tg vision.Target, pose geom.Pose, super int, noise float64, rng *rand.Rand) *vision.Gray {
	const (
		dark  = 0.12
		light = 0.88
		bg    = 0.45
	)
	img := vision.NewGray(cam.Width, cam.Height)
	for i := range img.Pix {
		img.Pix[i] = bg
	}

	// A board with Cols inner corners needs Cols+1 squares, and the inner
	// corners must land on the square boundaries. Centring the squares rather
	// than the corners would shift the whole pattern by half a square.
	s := tg.SquareMM
	wMM := float64(tg.Cols+1) * s
	hMM := float64(tg.Rows+1) * s
	x0 := -wMM / 2
	y0 := -hMM / 2

	// Only rasterise the board's bounding box.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{x0, y0}, {x0 + wMM, y0}, {x0 + wMM, y0 + hMM}, {x0, y0 + hMM}} {
		p, ok := cam.Project(pose.Apply(geom.V(c[0], c[1], 0)))
		if !ok {
			return img
		}
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	lo := func(v float64) int { return int(math.Max(0, math.Floor(v)-2)) }
	hiX := int(math.Min(float64(cam.Width-1), math.Ceil(maxX)+2))
	hiY := int(math.Min(float64(cam.Height-1), math.Ceil(maxY)+2))

	inv := pose.Inverse()
	normal := pose.R.Col(2)
	planeD := pose.T.Dot(normal)
	step := 1 / float64(super)

	for py := lo(minY); py <= hiY; py++ {
		for px := lo(minX); px <= hiX; px++ {
			var sum float64
			for sy := 0; sy < super; sy++ {
				for sx := 0; sx < super; sx++ {
					// Gray.Sample places pixel (px,py)'s value AT (px,py), so
					// the area it averages is centred there — hence the −0.5.
					// Getting this half-pixel wrong shifts every corner by
					// (0.5, 0.5) and shows up as a suspiciously constant
					// 1/√2 ≈ 0.707 px error at every pose.
					u := float64(px) - 0.5 + (float64(sx)+0.5)*step
					v := float64(py) - 0.5 + (float64(sy)+0.5)*step
					d := cam.Ray(vision.Point2{X: u, Y: v})
					den := d.Dot(normal)
					if math.Abs(den) < 1e-9 {
						sum += bg
						continue
					}
					hit := d.Scale(planeD / den)
					q := inv.Apply(hit)
					if q.X < x0 || q.Y < y0 || q.X >= x0+wMM || q.Y >= y0+hMM {
						sum += bg
						continue
					}
					ix := int(math.Floor((q.X - x0) / s))
					iy := int(math.Floor((q.Y - y0) / s))
					// Convention: squares with even (ix+iy) are dark, which makes
					// the square between inner corners (c,r) and (c+1,r+1) dark
					// when c+r is even — what the detector's polarity check
					// assumes.
					if (ix+iy)%2 == 0 {
						sum += dark
					} else {
						sum += light
					}
				}
			}
			val := sum / float64(super*super)
			if noise > 0 {
				val += rng.NormFloat64() * noise
			}
			img.Set(px, py, math.Max(0, math.Min(1, val)))
		}
	}
	return img
}

func detectTestCamera() vision.Camera {
	return vision.Camera{
		Width: 1280, Height: 720,
		Fx: 980, Fy: 981, Cx: 637, Cy: 361,
		K1: -0.21, K2: 0.07, P1: 0.0004, P2: -0.0003,
		Calibrated: true, CalibrationRMSPx: 0.19,
	}
}

// TestDetectorFindsEveryCorner: the detector must find all the corners, in the
// right order, and place them where they actually are.
func TestDetectorFindsEveryCorner(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(1))

	for _, pose := range []geom.Pose{
		{R: geom.RotX(geom.Rad(10)), T: geom.V(0, 0, 900)},
		{R: geom.RotY(geom.Rad(-28)).Mul(geom.RotX(geom.Rad(14))), T: geom.V(60, -30, 1000)},
		{R: geom.RotZ(geom.Rad(35)).Mul(geom.RotY(geom.Rad(22))), T: geom.V(-80, 40, 850)},
		// Steep tilts, the regime the hull-based grid search used to fail on and
		// that seed-and-grow was written to handle. A calibration series lives
		// here, so these must not be aspirational.
		{R: geom.RotY(geom.Rad(-40)).Mul(geom.RotX(geom.Rad(33))), T: geom.V(70, -40, 820)},
		{R: geom.RotZ(geom.Rad(28)).Mul(geom.RotY(geom.Rad(42))), T: geom.V(-60, 50, 900)},
	} {
		img := renderBoard(cam, tg, pose, 3, 0, rng)
		det, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: tg})
		if err != nil {
			t.Fatalf("%v: %v", pose.T, err)
		}
		if len(det.Corners) != tg.Cols*tg.Rows {
			t.Fatalf("found %d corners, want %d", len(det.Corners), tg.Cols*tg.Rows)
		}

		// Ground truth, in the same order as the model points.
		truth, ok := cam.ProjectPose(pose, tg.ModelPoints())
		if !ok {
			t.Fatal("ground truth did not project")
		}
		var ss, worst float64
		for i := range truth {
			d := det.Corners[i].DistTo(truth[i])
			ss += d * d
			worst = math.Max(worst, d)
		}
		rms := math.Sqrt(ss / float64(len(truth)))
		t.Logf("шаг клетки %.0f пикс: СКО %.4f пикс, худший угол %.4f пикс, разброс по сетке %.4f",
			det.MeanSpacingPx, rms, worst, det.GridRMSPx)

		// A corner mismatched by a whole square would show up as a huge error,
		// so this also proves the ordering is right. Steep tilts foreshorten the
		// far edge, so the ceiling is set against the steepest case rather than
		// the easy ones.
		if worst > 1.0 {
			t.Errorf("worst corner off by %.3f px — ordering or localisation is wrong", worst)
		}
		// The tilted poses come in around 0.03 px. The near-fronto-parallel one
		// is worse, at about 0.12 — but that is this test's rendering, not the
		// detector: raising the supersampling from 3 to 8 takes it to 0.044
		// while leaving the tilted poses unmoved at 0.025. Square edges that
		// line up with the pixel grid alias, and 3×3 sampling does not model
		// the sensor's area integration finely enough. A real sensor integrates
		// exactly, so the tolerance is set to admit the artefact rather than to
		// pretend the detector is worse than it is.
		if rms > 0.15 {
			t.Errorf("corner rms %.4f px, expected subpixel accuracy", rms)
		}
	}
}

// TestDetectorSubpixelUnderNoise records what sensor noise costs the corner
// locations — the number that decides whether the optical mode beats a magnetic
// angle gauge, since pose error scales with it directly.
func TestDetectorSubpixelUnderNoise(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	pose := geom.Pose{R: geom.RotY(geom.Rad(-25)).Mul(geom.RotX(geom.Rad(12))), T: geom.V(0, 0, 950)}
	truth, _ := cam.ProjectPose(pose, tg.ModelPoints())

	for _, noise := range []float64{0, 0.01, 0.03} {
		rng := rand.New(rand.NewSource(9))
		img := renderBoard(cam, tg, pose, 3, noise, rng)
		det, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: tg})
		if err != nil {
			t.Fatalf("noise %.3f: %v", noise, err)
		}
		var ss float64
		for i := range truth {
			d := det.Corners[i].DistTo(truth[i])
			ss += d * d
		}
		rms := math.Sqrt(ss / float64(len(truth)))
		t.Logf("шум сенсора %.3f → СКО положения угла %.4f пикс", noise, rms)
		if rms > 0.15 {
			t.Errorf("noise %.3f gave corner rms %.4f px", noise, rms)
		}
	}
}

// TestDetectorResolvesHalfTurn is the ambiguity that geometry cannot settle: a
// checkerboard looks identical rotated 180°. Getting it wrong would make the
// recovered pose jump between frames and destroy runout compensation.
//
// The board is rendered at a pose and again physically turned half a turn about
// its own normal. The detector must report the two as genuinely different, which
// it can only do by reading the squares' colours.
func TestDetectorResolvesHalfTurn(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(4))

	base := geom.Pose{R: geom.RotY(geom.Rad(-20)).Mul(geom.RotX(geom.Rad(15))), T: geom.V(0, 0, 950)}
	turned := geom.Pose{R: base.R.Mul(geom.RotZ(math.Pi)), T: base.T}

	solve := func(p geom.Pose) geom.Pose {
		t.Helper()
		img := renderBoard(cam, tg, p, 3, 0.005, rng)
		_, res, err := vision.DetectAndSolve(img, cam, vision.DetectOptions{Target: tg})
		if err != nil {
			t.Fatal(err)
		}
		return res.Pose
	}

	gotBase, gotTurned := solve(base), solve(turned)

	for name, pair := range map[string][2]geom.Pose{
		"как есть":          {gotBase, base},
		"повёрнута на 180°": {gotTurned, turned},
	} {
		_, ang := geom.AxisAngle(pair[0].R.Mul(pair[1].R.T()))
		if geom.Deg(ang) > 0.5 {
			t.Errorf("%s: восстановленная поза отличается от истинной на %.2f° — "+
				"ориентация мишени определена неверно", name, geom.Deg(ang))
		}
	}

	// And the two must be reported as half a turn apart, not as identical.
	_, between := geom.AxisAngle(gotBase.R.Mul(gotTurned.R.T()))
	if math.Abs(geom.Deg(between)-180) > 1 {
		t.Errorf("две ориентации мишени должны различаться на 180°, получено %.2f°", geom.Deg(between))
	}
}

// TestDetectorRejectsIrreduciblyAmbiguousBoard: with an even dimension sum, the
// colours cannot break the half-turn symmetry either, so such a board must be
// refused outright rather than silently guessed at.
func TestDetectorRejectsIrreduciblyAmbiguousBoard(t *testing.T) {
	for _, tg := range []vision.Target{
		{Cols: 8, Rows: 6, SquareMM: 30}, // 14, even
		{Cols: 9, Rows: 5, SquareMM: 30}, // 14, even
	} {
		if err := tg.Validate(); err == nil {
			t.Errorf("board %dx%d has an even dimension sum and must be rejected", tg.Cols, tg.Rows)
		}
	}
	for _, tg := range []vision.Target{
		{Cols: 9, Rows: 6, SquareMM: 30}, // 15, odd
		{Cols: 7, Rows: 6, SquareMM: 25}, // 13, odd
	} {
		if err := tg.Validate(); err != nil {
			t.Errorf("board %dx%d should be accepted: %v", tg.Cols, tg.Rows, err)
		}
	}
}

// TestDetectorSurvivesClutter: a real garage photo has bolt heads, reflections
// and tread blocks in it. Extra strong corners outside the board must not
// derail the grid search.
func TestDetectorSurvivesClutter(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(6))
	pose := geom.Pose{R: geom.RotY(geom.Rad(-18)).Mul(geom.RotX(geom.Rad(12))), T: geom.V(0, 0, 950)}
	img := renderBoard(cam, tg, pose, 3, 0.004, rng)

	// Scatter small checker-like patches around the frame, well clear of the
	// board, exactly the sort of thing that produces spurious saddle responses.
	for _, c := range [][2]int{{80, 90}, {1180, 110}, {90, 640}, {1190, 630}, {640, 60}} {
		for dy := -8; dy <= 8; dy++ {
			for dx := -8; dx <= 8; dx++ {
				v := 0.1
				if (dx >= 0) == (dy >= 0) {
					v = 0.9
				}
				img.Set(c[0]+dx, c[1]+dy, v)
			}
		}
	}

	det, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: tg})
	if err != nil {
		t.Fatalf("clutter defeated the detector: %v", err)
	}
	truth, _ := cam.ProjectPose(pose, tg.ModelPoints())
	var worst float64
	for i := range truth {
		worst = math.Max(worst, det.Corners[i].DistTo(truth[i]))
	}
	t.Logf("кандидатов найдено %d при %d нужных; худший угол %.4f пикс",
		det.CandidatesFound, tg.Cols*tg.Rows, worst)
	if worst > 0.5 {
		t.Errorf("worst corner off by %.3f px with clutter present", worst)
	}
}

// TestDetectorSteepTiltWithClutter is the case that broke the hull-based grid
// search and motivated seed-and-grow: a board tilted hard enough that its far
// edge is heavily foreshortened, with clutter scattered around it. The hull
// approach lost this because a few stray points stopped the board's real
// outline from being the largest quadrilateral. Growing the lattice locally
// does not care how much clutter is present — it is simply never adjacent to
// the pattern — so this must pass cleanly.
func TestDetectorSteepTiltWithClutter(t *testing.T) {
	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(13))
	pose := geom.Pose{R: geom.RotZ(geom.Rad(24)).Mul(geom.RotY(geom.Rad(-43))).Mul(geom.RotX(geom.Rad(30))),
		T: geom.V(50, -30, 820)}
	img := renderBoard(cam, tg, pose, 3, 0.005, rng)

	for _, c := range [][2]int{{70, 80}, {1200, 90}, {80, 650}, {1210, 640}, {620, 50}, {300, 690}} {
		for dy := -7; dy <= 7; dy++ {
			for dx := -7; dx <= 7; dx++ {
				v := 0.12
				if (dx >= 0) == (dy >= 0) {
					v = 0.88
				}
				img.Set(c[0]+dx, c[1]+dy, v)
			}
		}
	}

	det, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: tg})
	if err != nil {
		t.Fatalf("сильный наклон с мусором не распознан: %v", err)
	}
	truth, _ := cam.ProjectPose(pose, tg.ModelPoints())
	var worst float64
	for i := range truth {
		worst = math.Max(worst, det.Corners[i].DistTo(truth[i]))
	}
	t.Logf("наклон 43°, кандидатов %d при %d нужных; худший угол %.4f пикс",
		det.CandidatesFound, tg.Cols*tg.Rows, worst)
	if worst > 1.0 {
		t.Errorf("worst corner off by %.3f px — grid recovery failed under steep tilt", worst)
	}
}

// TestDetectorRefusesMissingBoard: no board means an error, not a confident
// answer built from noise.
func TestDetectorRefusesMissingBoard(t *testing.T) {
	cam := detectTestCamera()
	rng := rand.New(rand.NewSource(2))
	img := vision.NewGray(cam.Width, cam.Height)
	for i := range img.Pix {
		img.Pix[i] = 0.5 + rng.NormFloat64()*0.02
	}
	if _, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: vision.DefaultTarget()}); err == nil {
		t.Error("an image with no checkerboard must produce an error")
	}
}

// TestImageToCamber is the whole optical mode, from rendered pixels to a wheel
// angle, with nothing hand-fed in between.
//
// A wheel with known camber and toe carries a target clamped 2.5° crooked. The
// wheel is spun; each position is rendered as an image, detected, solved for
// pose, and the wheel's spin axis is recovered from the sequence. The clamp
// error must vanish, because the axis the target turns ABOUT is the wheel's own
// axis however crookedly it was bolted on.
func TestImageToCamber(t *testing.T) {
	const camberDeg, toeDeg, clampErr = -1.35, 0.22, 2.5

	cam := detectTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(21))

	wheel := simulate.WheelSpec{
		Camber: align.Deg(camberDeg), Toe: align.Deg(toeDeg),
		Center: geom.V(0, 760, 315),
	}
	spinAxis := wheel.SpinAxis(align.FL)

	camPos := geom.V(-350, 1750, 560)
	fwd := wheel.Center.Sub(camPos).Unit()
	right := geom.V(0, 0, 1).Cross(fwd).Unit()
	down := fwd.Cross(right).Unit()
	rCam := geom.FromRows(right, down, fwd)
	vehicleToCam := geom.Pose{R: rCam, T: rCam.MulVec(camPos).Neg()}

	tiltAxis := spinAxis.Any()
	mountR := geom.Rodrigues(tiltAxis, geom.Rad(clampErr)).
		Mul(geom.RotationBetween(geom.V(0, 0, 1), spinAxis))
	mountT := wheel.Center.Add(spinAxis.Scale(90))

	var poses []geom.Pose
	var worstRMS, worstGrid float64
	for _, spin := range []float64{0, 44, 91, 137, 186, 232} {
		spinR := geom.Rodrigues(spinAxis, geom.Rad(spin))
		inVehicle := geom.Pose{
			R: spinR.Mul(mountR),
			T: spinR.MulVec(mountT.Sub(wheel.Center)).Add(wheel.Center),
		}
		img := renderBoard(cam, tg, vehicleToCam.Mul(inVehicle), 3, 0.006, rng)

		det, res, err := vision.DetectAndSolve(img, cam, vision.DetectOptions{Target: tg})
		if err != nil {
			t.Fatalf("поворот %.0f°: %v", spin, err)
		}
		if res.Ambiguous {
			t.Errorf("поворот %.0f°: поза сочтена неоднозначной", spin)
		}
		worstRMS = math.Max(worstRMS, res.RMSPx)
		worstGrid = math.Max(worstGrid, det.GridRMSPx)
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
	gotCamber, gotToe := align.Camber(got), align.Toe(got, align.FL)

	t.Logf("детектор: разброс по сетке до %.3f пикс; PnP: невязка до %.3f пикс", worstGrid, worstRMS)
	t.Logf("ось восстановлена с ошибкой %.4f°", geom.Deg(got.AngleTo(spinAxis)))
	t.Logf("развал %.4f° (задано %.2f°), схождение %.4f° (задано %.2f°)",
		gotCamber.Deg(), camberDeg, gotToe.Deg(), toeDeg)

	if d := math.Abs(gotCamber.Deg() - camberDeg); d > 0.1 {
		t.Errorf("развал восстановлен с ошибкой %.4f°", d)
	}
	if d := math.Abs(gotToe.Deg() - toeDeg); d > 0.1 {
		t.Errorf("схождение восстановлено с ошибкой %.4f°", d)
	}
}
