package vision_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// TestFullOpticalAlignment is the whole thing: a car, four wheels, two floor
// reference boards, one camera moved freely — and out of it a complete alignment
// with toe and thrust, not just camber.
//
// The scene is the honest one. A single floor board cannot be seen from all four
// wheels of a real car, so there are two — one ahead of the front axle, one
// behind the rear — plus a link photograph taken from the side in which both
// appear. Each wheel is registered against whichever board is near it; the link
// carries the rear pair into the front board's frame. The camera never sees
// everything at once, which is exactly the constraint a person with one phone is
// under.
//
// Every wheel target is clamped crooked, by a different amount and about a
// different axis, so the result also demonstrates once more that clamp accuracy
// does not matter.
func TestFullOpticalAlignment(t *testing.T) {
	cam := detectTestCamera()
	rng := rand.New(rand.NewSource(4242))

	// Distinct layouts: the two references and the wheel target must all be
	// tellable apart, and each must have an odd dimension sum so its 180°
	// orientation is resolvable.
	// The floor boards are large-squared on purpose. Lying flat, they are always
	// seen at a grazing angle, which compresses one of their lattice axes
	// several times over; a board with small squares simply cannot be resolved
	// that way from across a car. 100 mm squares is what makes the link shot
	// possible at all.
	frontRef := vision.Target{Name: "передняя напольная", Cols: 7, Rows: 6, SquareMM: 100}
	rearRef := vision.Target{Name: "задняя напольная", Cols: 6, Rows: 5, SquareMM: 100}
	wheelTg := vision.Target{Name: "колёсная", Cols: 8, Rows: 5, SquareMM: 32}
	for _, tg := range []vision.Target{frontRef, rearRef, wheelTg} {
		if err := tg.Validate(); err != nil {
			t.Fatalf("%s: %v", tg.Name, err)
		}
	}

	veh := simulate.Nominal()

	// Floor boards lie flat, face up: their own +Z is the road normal.
	frontRefPose := geom.Pose{R: geom.Identity(), T: geom.V(3100, 0, 0)}
	rearRefPose := geom.Pose{R: geom.Identity(), T: geom.V(-500, 0, 0)}

	// Which board each wheel is registered against.
	refFor := map[align.Position]struct {
		tg   vision.Target
		pose geom.Pose
	}{
		align.FL: {frontRef, frontRefPose},
		align.FR: {frontRef, frontRefPose},
		align.RL: {rearRef, rearRefPose},
		align.RR: {rearRef, rearRefPose},
	}
	// Where the operator stands for each wheel: outboard of it, far enough back
	// that the wheel target and its floor board are both in shot.
	eyeFor := map[align.Position]geom.Vec3{
		align.FL: geom.V(3600, 2000, 1200),
		align.FR: geom.V(3600, -2000, 1200),
		align.RL: geom.V(-1000, 2000, 1200),
		align.RR: geom.V(-1000, -2000, 1200),
	}
	// A different clamp error per wheel, to prove none of it matters.
	clampErr := map[align.Position]float64{align.FL: 2.5, align.FR: 1.4, align.RL: 3.1, align.RR: 0.8}

	registered := map[align.Position]vision.RegisteredWheel{}

	for _, p := range align.AllPositions {
		w := veh.Wheels[p]
		spin := w.SpinAxis(p)
		ref := refFor[p]

		mountR := geom.Rodrigues(spin.Any(), geom.Rad(clampErr[p])).
			Mul(geom.RotationBetween(geom.V(0, 0, 1), spin))
		mountT := w.Center.Add(spin.Scale(85))

		// Aim between the wheel and the floor board so both land in frame.
		camPose := lookAt(eyeFor[p], w.Center.Add(ref.pose.T).Scale(0.5))

		var imgs []*vision.Gray
		for fi, ang := range []float64{0, 47, 96, 148, 199} {
			spinR := geom.Rodrigues(spin, geom.Rad(ang))
			wheelPose := geom.Pose{
				R: spinR.Mul(mountR),
				T: spinR.MulVec(mountT.Sub(w.Center)).Add(w.Center),
			}
			boards := []placedBoard{
				{wheelTg, camPose.Mul(wheelPose)},
				{ref.tg, camPose.Mul(ref.pose)},
			}
			if fi == 0 {
				assertVisible(t, cam, boards[0], p.String()+": мишень на колесе")
				assertVisible(t, cam, boards[1], p.String()+": напольная мишень")
			}
			imgs = append(imgs, renderScene(cam, boards, 3, 0.005, rng))
		}

		reg, err := vision.RegisterWheel(cam, wheelTg, ref.tg, imgs, vision.DetectOptions{})
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		t.Logf("%-2s кадров %d/%d, поворот %.0f°, перекос мишени %.1f° (задан %.1f°), невязка оси %.2f мм",
			p, reg.Used, len(imgs), reg.SweepDeg, reg.RunoutDeg, clampErr[p], reg.AxisResidualMM)
		registered[p] = reg
	}

	// ── Link the two floor boards ───────────────────────────────────────────
	// One photograph from the side with both boards in it. It has to be taken
	// from up high — phone held above head height — because two boards lying
	// flat, seen from standing height across a car, are almost edge-on and
	// cannot be read at all.
	linkPose := lookAt(geom.V(1300, 3000, 1900), geom.V(1300, 0, 0))
	linkBoards := []placedBoard{
		{frontRef, linkPose.Mul(frontRefPose)},
		{rearRef, linkPose.Mul(rearRefPose)},
	}
	assertVisible(t, cam, linkBoards[0], "связующий кадр: передняя напольная")
	assertVisible(t, cam, linkBoards[1], "связующий кадр: задняя напольная")
	linkImg := renderScene(cam, linkBoards, 3, 0.005, rng)

	link, err := vision.LinkReferences(cam, []vision.Target{frontRef, rearRef},
		[]*vision.Gray{linkImg}, vision.DetectOptions{})
	if err != nil {
		t.Fatalf("связывание напольных мишеней: %v", err)
	}

	// Check the link against the truth: it should recover the real offset
	// between the two boards.
	trueRearToFront := frontRefPose.Inverse().Mul(rearRefPose)
	gotRearToFront := link.ToRoot[1]
	posErr := gotRearToFront.T.Sub(trueRearToFront.T).Len()
	_, angErr := geom.AxisAngle(gotRearToFront.R.Mul(trueRearToFront.R.T()))
	t.Logf("связь мишеней: смещение восстановлено с ошибкой %.1f мм, поворот %.3f°",
		posErr, geom.Deg(angErr))
	if posErr > 25 {
		t.Errorf("связь мишеней даёт ошибку положения %.1f мм", posErr)
	}
	if geom.Deg(angErr) > 0.5 {
		t.Errorf("связь мишеней даёт ошибку поворота %.3f°", geom.Deg(angErr))
	}

	// ── Bring every wheel into the front board's frame and assemble ─────────
	inRoot := map[align.Position]vision.RegisteredWheel{}
	for _, p := range align.AllPositions {
		idx := 0
		if p == align.RL || p == align.RR {
			idx = 1
		}
		inRoot[p] = registered[p].InFrameOf(link.ToRoot[idx])
	}

	// The root frame is the front board's, which sits at frontRefPose in vehicle
	// coordinates, so truth in root coordinates is the vehicle value minus that.
	//
	// Each recovered centre sits one clamp offset (85 mm) outboard of the true
	// wheel centre, because the axial station of a spin axis is fixed by where
	// the target sits on it and nothing in the rotation reveals the clamp's
	// depth. That is harmless precisely because it is symmetric: it cancels in
	// every axle midpoint, so the vehicle's longitudinal axis — and with it toe
	// and thrust — is unaffected. Both halves of that claim are checked.
	lateral := map[align.Position]float64{}
	for _, p := range align.AllPositions {
		wantC := frontRefPose.Inverse().Apply(veh.Wheels[p].Center)
		gotC := inRoot[p].Center
		d := gotC.Sub(wantC)
		lateral[p] = d.Y
		t.Logf("%-2s центр: получено (%+.0f %+.0f %+.0f), истина (%+.0f %+.0f %+.0f), Δ=(%+.0f %+.0f %+.0f)",
			p, gotC.X, gotC.Y, gotC.Z, wantC.X, wantC.Y, wantC.Z, d.X, d.Y, d.Z)

		if math.Abs(d.X) > 15 || math.Abs(d.Z) > 25 {
			t.Errorf("%s: центр смещён не только вдоль оси: Δx=%.0f Δz=%.0f", p, d.X, d.Z)
		}
	}
	for _, axle := range [][2]align.Position{{align.FL, align.FR}, {align.RL, align.RR}} {
		if mid := lateral[axle[0]] + lateral[axle[1]]; math.Abs(mid) > 20 {
			t.Errorf("смещения центров оси %s/%s не гасятся в середине оси: сумма %.0f мм",
				axle[0], axle[1], mid)
		}
	}

	sess := measure.OpticalSession{Wheels: inRoot, RimDiameterMM: align.Inches(16)}
	res, err := sess.Result()
	if err != nil {
		t.Fatalf("сборка протокола: %v", err)
	}

	// ── Compare against the car we built ────────────────────────────────────
	t.Logf("угол тяги %.3f° (задано %.3f°)", res.ThrustAngle.Deg(), truthThrust(veh).Deg())
	var worstCamber, worstToe float64
	for _, p := range align.AllPositions {
		want := veh.Wheels[p]
		got := res.Wheels[p.String()]
		dc := math.Abs(got.Camber.Deg() - want.Camber.Deg())
		dt := math.Abs(got.ToeGeometric.Deg() - want.Toe.Deg())
		worstCamber, worstToe = math.Max(worstCamber, dc), math.Max(worstToe, dt)
		t.Logf("%-2s развал %+.3f° (задано %+.2f°, Δ%.3f)   схождение %+.3f° (задано %+.2f°, Δ%.3f)",
			p, got.Camber.Deg(), want.Camber.Deg(), dc,
			got.ToeGeometric.Deg(), want.Toe.Deg(), dt)
	}

	if worstCamber > 0.15 {
		t.Errorf("худшая ошибка развала %.3f°", worstCamber)
	}
	if worstToe > 0.15 {
		t.Errorf("худшая ошибка схождения %.3f°", worstToe)
	}
	if d := math.Abs(res.ThrustAngle.Deg() - truthThrust(veh).Deg()); d > 0.1 {
		t.Errorf("угол тяги восстановлен с ошибкой %.3f°", d)
	}

	// The rolling radius is measured, not assumed: it is the wheel centre's
	// height above the floor board's plane.
	for _, p := range align.AllPositions {
		h := inRoot[p].Center.Z
		if math.Abs(h-veh.Wheels[p].RollingRadiusMM) > 25 {
			t.Errorf("%s: радиус качения восстановлен как %.0f мм вместо %.0f",
				p, h, veh.Wheels[p].RollingRadiusMM)
		}
	}
}

func truthThrust(v simulate.Vehicle) align.Angle {
	return (v.Wheels[align.RL].Toe - v.Wheels[align.RR].Toe) / 2
}
