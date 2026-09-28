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
	Found    []string `json:"found"`
	Message  string   `json:"message"`
	Progress float64  `json:"progress"`
	Done     bool     `json:"done"`
	Camber   *float64 `json:"camber"`
	Toe      *float64 `json:"toe"`
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

// TestLiveCamera is the live mode end to end, over HTTP, as a phone drives it:
// runout compensation of each wheel from a few frames of it being turned, one
// frame tying the two floor targets together, then single frames of each wheel
// — and the answer a pro aligner gives: camber and toe, live, and toe changing
// by exactly as much as the wheel was turned.
func TestLiveCamera(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	rng := rand.New(rand.NewSource(77))

	frontRef := vision.Target{Cols: 7, Rows: 6, SquareMM: 100}
	rearRef := vision.Target{Cols: 6, Rows: 5, SquareMM: 100}
	wheelTg := vision.Target{Cols: 8, Rows: 5, SquareMM: 32}

	// The camera is set directly here; learning it from frames has its own test.
	rec := sendJSON(t, srv, "/api/optical/live", map[string]any{
		"wheel_target": wheelTg, "refs": []vision.Target{frontRef, rearRef},
		"camera": cam, "camera_id": "phone1", "rim_in": 16,
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
	// its spin axis and then — for the adjustment — rotated as a whole by adj.
	type wheelState struct {
		spin    geom.Vec3
		center  geom.Vec3
		mountR  geom.Mat3
		mountT  geom.Vec3
		adjust  geom.Mat3
		clampOf float64
	}
	states := map[align.Position]*wheelState{}
	for _, p := range align.AllPositions {
		w := veh.Wheels[p]
		spin := w.SpinAxis(p)
		states[p] = &wheelState{
			spin: spin, center: w.Center,
			mountR: geom.Rodrigues(spin.Any(), geom.Rad(clampErr[p])).Mul(geom.RotationBetween(geom.V(0, 0, 1), spin)),
			mountT: w.Center.Add(spin.Scale(85)), adjust: geom.Identity(), clampOf: clampErr[p],
		}
	}
	frame := func(p align.Position, ang float64) []byte {
		st := states[p]
		spinR := geom.Rodrigues(st.spin, geom.Rad(ang))
		r := st.adjust.Mul(spinR)
		wheelPose := geom.Pose{R: r.Mul(st.mountR), T: r.MulVec(st.mountT.Sub(st.center)).Add(st.center)}
		tg, rp := refFor(p)
		camPose := lookAtSrv(eye[p], st.center.Add(rp.T).Scale(0.5))
		return renderScenePNGSuper(t, cam, []sceneBoard{
			{wheelTg, camPose.Mul(wheelPose)},
			{tg, camPose.Mul(rp)},
		}, 0.005, rng, 2)
	}

	// ── Runout compensation, wheel by wheel ─────────────────────────────
	for _, p := range align.AllPositions {
		var last liveResp
		for _, ang := range []float64{0, 50, 100, 150, 200} {
			last = postFrame(t, srv, "d=phone1&mode=spin&wheel="+p.String(), frame(p, ang))
			t.Logf("%s %.0f°: %v %s", p, ang, last.Found, last.Message)
		}
		if !last.Done {
			t.Fatalf("%s: runout compensation did not finish: %s", p, last.Message)
		}
		t.Logf("%s: %s", p, last.Message)
	}

	// ── Live: the first wheel fixes the root floor target ─────────────
	r := postFrame(t, srv, "d=phone1&mode=live&wheel=FL", frame(align.FL, 15))
	if r.Camber == nil || r.Toe != nil {
		t.Fatalf("FL alone: want camber and no toe yet, got %+v", r)
	}

	// Link frames: both floor targets, no wheel — a few, from either side, as
	// a person walking round with the phone raised would take them.
	for _, eyeL := range []geom.Vec3{geom.V(1300, 3000, 1900), geom.V(1100, -3000, 2000), geom.V(1500, 2800, 2100)} {
		linkPose := lookAtSrv(eyeL, geom.V(1300, 0, 0))
		link := renderScenePNGSuper(t, cam, []sceneBoard{
			{frontRef, linkPose.Mul(frontRefPose)},
			{rearRef, linkPose.Mul(rearRefPose)},
		}, 0.005, rng, 2)
		r = postFrame(t, srv, "d=phone1&mode=live&wheel=FL", link)
		t.Logf("link: %v %s", r.Found, r.Message)
	}

	for _, p := range []align.Position{align.FR, align.RL, align.RR} {
		r = postFrame(t, srv, "d=phone1&mode=live&wheel="+p.String(), frame(p, 60))
		if r.Camber == nil {
			t.Fatalf("%s: no camber: %s", p, r.Message)
		}
	}
	if r.Toe == nil {
		t.Fatalf("all four wheels seen, still no toe: %s", r.Message)
	}

	// Every wheel again, compared with the car we built.
	for _, p := range align.AllPositions {
		r = postFrame(t, srv, "d=phone1&mode=live&wheel="+p.String(), frame(p, 200))
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

	// ── The adjustment: turn the front-left tie rod by 0.3° of toe ─────
	before := *postFrame(t, srv, "d=phone1&mode=live&wheel=FL", frame(align.FL, 200)).Toe
	fl := states[align.FL]
	newSpin := simulate.WheelSpec{Camber: veh.Wheels[align.FL].Camber, Toe: veh.Wheels[align.FL].Toe + align.Deg(0.3)}.SpinAxis(align.FL)
	fl.adjust = geom.RotationBetween(fl.spin, newSpin)
	after := *postFrame(t, srv, "d=phone1&mode=live&wheel=FL", frame(align.FL, 200)).Toe
	t.Logf("FL toe %+.3f° → %+.3f° after turning the tie rod by 0.30°", before, after)
	if d := after - before; math.Abs(d-0.3) > 0.05 {
		t.Errorf("toe changed by %.3f°, the wheel was turned by 0.30°", d)
	}

	// The live screen got it.
	if p := srv.Hub().Frame().Params["toe_FL"]; !p.Has {
		t.Error("no toe on the adjustment screen")
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
