package server_test

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/server"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

type liveResp struct {
	Mode     string   `json:"mode"`
	Wheel    string   `json:"wheel"`
	Found    []string `json:"found"`
	Message  string   `json:"message"`
	Progress float64  `json:"progress"`
	Done     bool     `json:"done"`
	Camber   *float64 `json:"camber"`
	Toe      *float64 `json:"toe"`
	Caster   *float64 `json:"caster"`
	SAI      *float64 `json:"sai"`
	Warn     string   `json:"warn"`
}

func postFrame(t *testing.T, srv *server.Server, query string, png []byte) liveResp {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/optical/frame?"+query, bytes.NewReader(png))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d: %s", query, rec.Code, rec.Body.String())
	}
	var r liveResp
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestLiveCamera is the live mode end to end, over HTTP, as a phone drives it,
// with a different board on each wheel so that the frames alone say which
// wheel is in view: runout compensation of each wheel, frames tying the two
// floor targets together, single frames of each wheel — and the answer a pro
// aligner gives: camber and toe, live, toe changing by exactly as much as the
// wheel was turned, and caster from steering a front wheel on its plate.
func TestLiveCamera(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	rng := rand.New(rand.NewSource(77))

	frontRef := vision.Target{Cols: 7, Rows: 6, SquareMM: 100}
	rearRef := vision.Target{Cols: 6, Rows: 5, SquareMM: 100}
	wheelTg := server.DefaultWheelTargets

	// The camera is set directly here; learning it from frames has its own test.
	rec := sendJSON(t, srv, "/api/optical/live", map[string]any{
		"refs": []vision.Target{frontRef, rearRef}, "camera": cam, "camera_id": "phone1", "rim_in": 16,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body.String())
	}

	veh := simulate.Nominal()
	frontRefPose := geom.Pose{R: geom.Identity(), T: geom.V(3100, 0, 0)}
	rearRefPose := geom.Pose{R: geom.Identity(), T: geom.V(-500, 0, 0)}
	refFor := func(p align.Position) (vision.Target, geom.Pose) {
		if p.IsFront() {
			return frontRef, frontRefPose
		}
		return rearRef, rearRefPose
	}
	eye := map[align.Position]geom.Vec3{
		align.FL: geom.V(3600, 2000, 1200), align.FR: geom.V(3600, -2000, 1200),
		align.RL: geom.V(-1000, 2000, 1200), align.RR: geom.V(-1000, -2000, 1200),
	}
	clampErr := map[align.Position]float64{align.FL: 2.3, align.FR: 1.2, align.RL: 2.8, align.RR: 0.7}

	// A wheel's target, clamped crooked, with the wheel turned by ang about
	// its spin axis, then — for the adjustment — rotated as a whole by adjust,
	// and steered by steer degrees (outboard positive) about its steering axis.
	type wheelState struct {
		spin, center, mountT geom.Vec3
		mountR, adjust       geom.Mat3
	}
	states := map[align.Position]*wheelState{}
	for _, p := range align.AllPositions {
		w := veh.Wheels[p]
		spin := w.SpinAxis(p)
		states[p] = &wheelState{
			spin: spin, center: w.Center, mountT: w.Center.Add(spin.Scale(85)), adjust: geom.Identity(),
			mountR: geom.Rodrigues(spin.Any(), geom.Rad(clampErr[p])).Mul(geom.RotationBetween(geom.V(0, 0, 1), spin)),
		}
	}
	frameSteer := func(p align.Position, ang, steer float64) []byte {
		st := states[p]
		r := st.adjust.Mul(geom.Rodrigues(st.spin, geom.Rad(ang)))
		wheelPose := geom.Pose{R: r.Mul(st.mountR), T: r.MulVec(st.mountT.Sub(st.center)).Add(st.center)}
		if steer != 0 {
			// The steering axis runs through the kingpin, inboard of the
			// wheel centre.
			k := veh.Wheels[p].SteeringAxis(p)
			pivot := st.center.Sub(st.spin.Scale(80))
			rs := geom.Rodrigues(k, p.SideSign()*geom.Rad(steer))
			wheelPose = geom.Pose{R: rs.Mul(wheelPose.R), T: rs.MulVec(wheelPose.T.Sub(pivot)).Add(pivot)}
		}
		tg, rp := refFor(p)
		camPose := lookAtSrv(eye[p], st.center.Add(rp.T).Scale(0.5))
		return renderScenePNGSuper(t, cam, []sceneBoard{
			{wheelTg[p], camPose.Mul(wheelPose)},
			{tg, camPose.Mul(rp)},
		}, 0.005, rng, 2)
	}
	frame := func(p align.Position, ang float64) []byte { return frameSteer(p, ang, 0) }

	// ── Runout compensation, wheel by wheel — no wheel named ────────────
	for _, p := range align.AllPositions {
		var last liveResp
		for _, ang := range []float64{0, 50, 100, 150, 200} {
			last = postFrame(t, srv, "d=phone1&mode=spin", frame(p, ang))
		}
		if !last.Done || last.Wheel != p.String() {
			t.Fatalf("%s: runout compensation: wheel %q, done %v: %s", p, last.Wheel, last.Done, last.Message)
		}
		t.Logf("%s: %s", p, last.Message)
	}

	// ── Live: the first wheel fixes the root floor target ─────────────
	r := postFrame(t, srv, "d=phone1&mode=live", frame(align.FL, 15))
	if r.Camber == nil || r.Toe != nil || r.Wheel != "FL" {
		t.Fatalf("FL alone: want FL camber and no toe yet, got %+v", r)
	}

	// Link frames: both floor targets, no wheel — a few, from either side, as
	// a person walking round with the phone raised would take them.
	for _, eyeL := range []geom.Vec3{geom.V(1300, 3000, 1900), geom.V(1100, -3000, 2000), geom.V(1500, 2800, 2100)} {
		linkPose := lookAtSrv(eyeL, geom.V(1300, 0, 0))
		link := renderScenePNGSuper(t, cam, []sceneBoard{
			{frontRef, linkPose.Mul(frontRefPose)},
			{rearRef, linkPose.Mul(rearRefPose)},
		}, 0.005, rng, 2)
		r = postFrame(t, srv, "d=phone1&mode=live", link)
		t.Logf("link: %v %s", r.Found, r.Message)
	}

	for _, p := range []align.Position{align.FR, align.RL, align.RR} {
		r = postFrame(t, srv, "d=phone1&mode=live", frame(p, 60))
		if r.Camber == nil || r.Wheel != p.String() {
			t.Fatalf("%s: got wheel %q: %s", p, r.Wheel, r.Message)
		}
	}
	if r.Toe == nil {
		t.Fatalf("all four wheels seen, still no toe: %s", r.Message)
	}

	// Every wheel again, compared with the car we built.
	for _, p := range align.AllPositions {
		r = postFrame(t, srv, "d=phone1&mode=live", frame(p, 200))
		want := veh.Wheels[p]
		if r.Camber == nil || r.Toe == nil {
			t.Fatalf("%s: incomplete: %+v", p, r)
		}
		dc, dt := math.Abs(*r.Camber-want.Camber.Deg()), math.Abs(*r.Toe-want.Toe.Deg())
		t.Logf("%s: camber %+.3f° (true %+.2f°), toe %+.3f° (true %+.2f°)", p, *r.Camber, want.Camber.Deg(), *r.Toe, want.Toe.Deg())
		if dc > 0.06 || dt > 0.08 {
			t.Errorf("%s: camber off by %.3f°, toe by %.3f°", p, dc, dt)
		}
	}

	// ── Caster: steer the front-left wheel on its plate ────────────────
	var cr liveResp
	for _, steer := range []float64{0, -3, -6, -9, -12, -15, -18, -21, -14, -7, 3, 6, 9, 12, 15, 18, 21} {
		cr = postFrame(t, srv, "d=phone1&mode=caster", frameSteer(align.FL, 200, steer))
		t.Logf("steer %+.0f°: %s", steer, cr.Message)
		if cr.Done {
			break
		}
	}
	if !cr.Done || cr.Caster == nil || cr.SAI == nil {
		t.Fatalf("caster not measured: %+v", cr)
	}
	wc, ws := veh.Wheels[align.FL].Caster.Deg(), veh.Wheels[align.FL].SAI.Deg()
	t.Logf("FL caster %+.3f° (true %+.2f°), SAI %.3f° (true %.2f°)", *cr.Caster, wc, *cr.SAI, ws)
	if math.Abs(*cr.Caster-wc) > 0.2 || math.Abs(*cr.SAI-ws) > 0.3 {
		t.Errorf("caster off by %.3f°, SAI by %.3f°", *cr.Caster-wc, *cr.SAI-ws)
	}

	// ── The adjustment: turn the front-left tie rod by 0.3° of toe ─────
	before := *postFrame(t, srv, "d=phone1&mode=live", frame(align.FL, 200)).Toe
	fl := states[align.FL]
	newSpin := simulate.WheelSpec{Camber: veh.Wheels[align.FL].Camber, Toe: veh.Wheels[align.FL].Toe + align.Deg(0.3)}.SpinAxis(align.FL)
	fl.adjust = geom.RotationBetween(fl.spin, newSpin)
	after := *postFrame(t, srv, "d=phone1&mode=live", frame(align.FL, 200)).Toe
	t.Logf("FL toe %+.3f° → %+.3f° after turning the tie rod by 0.30°", before, after)
	if d := after - before; math.Abs(d-0.3) > 0.05 {
		t.Errorf("toe changed by %.3f°, the wheel was turned by 0.30°", d)
	}

	// The live screen got it.
	fr := srv.Hub().Frame()
	if p := fr.Params["toe_FL"]; !p.Has {
		t.Error("no toe on the adjustment screen")
	}
	if p := fr.Params["caster_FL"]; !p.Has {
		t.Error("no caster on the adjustment screen")
	}
}

// TestLiveCameraSharedBoard: with the same board on every wheel the phone has
// to say which wheel it looks at; without it the frame is refused.
func TestLiveCameraSharedBoard(t *testing.T) {
	srv := testServer(t)
	rec := sendJSON(t, srv, "/api/optical/live", map[string]any{"wheel_target": vision.Target{Cols: 8, Rows: 5, SquareMM: 32}})
	var st server.LiveOpticalState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || st.Distinct {
		t.Fatalf("one board for all wheels should not count as distinct: %+v %v", st, err)
	}
	req := httptest.NewRequest("POST", "/api/optical/frame?d=x&mode=live", bytes.NewReader([]byte("x")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("a frame with no wheel named: %d", w.Code)
	}
	// Boards one inside another are refused.
	rec = sendJSON(t, srv, "/api/optical/live", map[string]any{"wheel_targets": map[string]vision.Target{
		"FL": {Cols: 8, Rows: 5, SquareMM: 25}, "FR": {Cols: 9, Rows: 6, SquareMM: 25},
		"RL": {Cols: 10, Rows: 3, SquareMM: 25}, "RR": {Cols: 11, Rows: 4, SquareMM: 20},
	}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("8×5 inside 9×6 accepted: %d %s", rec.Code, rec.Body.String())
	}
}

// TestTargetsPDF: the print sheet for the live mode holds all six boards.
func TestTargetsPDF(t *testing.T) {
	srv := testServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/targets/plan?paper=a4", nil))
	var plans []server.TargetPlanInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	if len(plans) != 6 {
		t.Fatalf("%d boards, want 4 wheels + 2 floor", len(plans))
	}
	for _, p := range plans[:4] {
		if p.Plan.Pages != 1 {
			t.Errorf("%s does not fit one A4 sheet: %+v", p.Label, p.Plan)
		}
	}
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/targets/pdf?paper=a4&which=wheels", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" ||
		!bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("PDF: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// TestLiveCameraCalibration: a phone learns its own camera from frames of the
// board shown at varied angles and positions — and refuses frames that add
// nothing.
func TestLiveCameraCalibration(t *testing.T) {
	srv := testServer(t)
	truth := vision.Camera{Width: 640, Height: 360, Fx: 490, Fy: 490, Cx: 321, Cy: 181, K1: -0.12, K2: 0.03, Calibrated: true}
	rng := rand.New(rand.NewSource(5))
	tg := vision.Target{Cols: 8, Rows: 5, SquareMM: 32}
	sendJSON(t, srv, "/api/optical/live", map[string]any{"wheel_target": tg})

	board := func(x, y, z, rx, ry float64) []byte {
		r := geom.Rodrigues(geom.V(1, 0, 0), geom.Rad(rx)).Mul(geom.Rodrigues(geom.V(0, 1, 0), geom.Rad(ry)))
		// Camera frame: +Z forward. A board facing the camera has its +Z
		// pointing back at it.
		face := geom.Rodrigues(geom.V(1, 0, 0), math.Pi)
		return renderScenePNGSuper(t, truth, []sceneBoard{{tg, geom.Pose{R: r.Mul(face), T: geom.V(x, y, z)}}}, 0.004, rng, 2)
	}

	// The same view twice is refused the second time.
	first := postFrame(t, srv, "d=cal1&mode=calib", board(0, 0, 650, 0, 0))
	again := postFrame(t, srv, "d=cal1&mode=calib", board(0, 0, 650, 0, 0))
	if first.Progress == 0 || again.Progress != first.Progress {
		t.Errorf("a repeated view was counted: %.2f → %.2f (%s)", first.Progress, again.Progress, again.Message)
	}

	var last liveResp
	positions := [][2]float64{{-170, -90}, {0, -90}, {170, -90}, {-170, 0}, {170, 0}, {-170, 90}, {0, 90}, {170, 90}, {0, 0}}
	tilts := [][2]float64{{25, 0}, {-25, 0}, {0, 28}, {0, -28}, {20, 20}, {-20, -20}, {18, -22}, {-22, 18}}
	for i := 0; i < 26 && !last.Done; i++ {
		// Position and tilt vary independently, as with a phone waved about.
		pos, tilt := positions[i%len(positions)], tilts[(i*5+i/len(positions))%len(tilts)]
		last = postFrame(t, srv, "d=cal1&mode=calib", board(pos[0], pos[1], 620+float64(i%3)*60, tilt[0], tilt[1]))
	}
	if !last.Done {
		t.Fatalf("calibration did not finish: %s", last.Message)
	}
	t.Log(last.Message)

	var st server.LiveOpticalState
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/optical/live", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Cameras) != 1 || st.Cameras[0].ID != "cal1" {
		t.Fatalf("calibrated cameras: %+v", st.Cameras)
	}
	if fx := st.Cameras[0].FxPx; math.Abs(fx-truth.Fx)/truth.Fx > 0.02 {
		t.Errorf("focal length learnt as %.0f px, true %.0f", fx, truth.Fx)
	}
}
