package server_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// TestOpticalAlignEndpoint drives a complete four-wheel optical alignment
// through HTTP: two floor reference boards, a link photograph, five frames per
// wheel, all as multipart PNG uploads — and out of it the same alignment report
// the string-line path produces.
//
// That last point is what is really being checked. The response carries an
// ordinary align.Result and specs.Report, so the browser draws it with the code
// that draws a manual measurement. Nothing about cameras reaches the reporting
// layer.
func TestOpticalAlignEndpoint(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	rng := rand.New(rand.NewSource(909))

	frontRef := vision.Target{Cols: 7, Rows: 6, SquareMM: 100}
	rearRef := vision.Target{Cols: 6, Rows: 5, SquareMM: 100}
	wheelTg := vision.Target{Cols: 8, Rows: 5, SquareMM: 32}

	veh := simulate.Nominal()
	frontRefPose := geom.Pose{R: geom.Identity(), T: geom.V(3100, 0, 0)}
	rearRefPose := geom.Pose{R: geom.Identity(), T: geom.V(-500, 0, 0)}

	refPose := map[align.Position]geom.Pose{
		align.FL: frontRefPose, align.FR: frontRefPose,
		align.RL: rearRefPose, align.RR: rearRefPose,
	}
	refTarget := map[align.Position]vision.Target{
		align.FL: frontRef, align.FR: frontRef,
		align.RL: rearRef, align.RR: rearRef,
	}
	refIdx := map[align.Position]int{align.FL: 0, align.FR: 0, align.RL: 1, align.RR: 1}
	eye := map[align.Position]geom.Vec3{
		align.FL: geom.V(3600, 2000, 1200),
		align.FR: geom.V(3600, -2000, 1200),
		align.RL: geom.V(-1000, 2000, 1200),
		align.RR: geom.V(-1000, -2000, 1200),
	}
	clamp := map[align.Position]float64{align.FL: 2.2, align.FR: 1.6, align.RL: 2.9, align.RR: 1.0}

	fields := map[string]string{
		"wheel_cols": "8", "wheel_rows": "5", "wheel_square_mm": "32",
		"ref0_cols": "7", "ref0_rows": "6", "ref0_square_mm": "100",
		"ref1_cols": "6", "ref1_rows": "5", "ref1_square_mm": "100",
		"rim_diameter_in": "16",
	}
	camJSON, _ := json.Marshal(cam)
	fields["camera_json"] = string(camJSON)

	files := map[string][]upload{}

	for _, p := range align.AllPositions {
		w := veh.Wheels[p]
		spin := w.SpinAxis(p)
		rp := refPose[p]

		mountR := geom.Rodrigues(spin.Any(), geom.Rad(clamp[p])).
			Mul(geom.RotationBetween(geom.V(0, 0, 1), spin))
		mountT := w.Center.Add(spin.Scale(85))
		camPose := lookAtSrv(eye[p], w.Center.Add(rp.T).Scale(0.5))

		var ups []upload
		for i, ang := range []float64{0, 47, 96, 148, 199} {
			spinR := geom.Rodrigues(spin, geom.Rad(ang))
			wheelPose := geom.Pose{
				R: spinR.Mul(mountR),
				T: spinR.MulVec(mountT.Sub(w.Center)).Add(w.Center),
			}
			png := renderScenePNG(t, cam, []sceneBoard{
				{wheelTg, camPose.Mul(wheelPose)},
				{refTarget[p], camPose.Mul(rp)},
			}, 0.005, rng)
			ups = append(ups, upload{name: p.String() + "_" + strconv.Itoa(i) + ".png", data: png})
		}
		files["images_"+p.String()] = ups
		fields["ref_"+p.String()] = strconv.Itoa(refIdx[p])
	}

	// The link photograph, from up high so both floor boards are readable.
	linkPose := lookAtSrv(geom.V(1300, 3000, 1900), geom.V(1300, 0, 0))
	files["images_link"] = []upload{{name: "link.png", data: renderScenePNG(t, cam, []sceneBoard{
		{frontRef, linkPose.Mul(frontRefPose)},
		{rearRef, linkPose.Mul(rearRefPose)},
	}, 0.005, rng)}}

	rec := postMultipart(t, srv, "/api/optical/align", fields, files)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result align.Result `json:"result"`
		Report struct {
			Params []struct {
				Key    string `json:"key"`
				Status string `json:"status"`
			} `json:"params"`
		} `json:"report"`
		Optical []struct {
			Position  string  `json:"position"`
			Used      int     `json:"used"`
			Total     int     `json:"total"`
			RunoutDeg float64 `json:"runout_deg"`
			SweepDeg  float64 `json:"sweep_deg"`
		} `json:"optical"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	for _, o := range resp.Optical {
		t.Logf("%-2s кадров %d/%d, поворот %.0f°, перекос мишени %.1f°",
			o.Position, o.Used, o.Total, o.SweepDeg, o.RunoutDeg)
		if o.Used != o.Total {
			t.Errorf("%s: использовано %d кадров из %d", o.Position, o.Used, o.Total)
		}
	}

	wantThrust := (veh.Wheels[align.RL].Toe - veh.Wheels[align.RR].Toe) / 2
	t.Logf("угол тяги %.3f° (задано %.3f°)", resp.Result.ThrustAngle.Deg(), wantThrust.Deg())

	var worstCamber, worstToe float64
	for _, p := range align.AllPositions {
		want := veh.Wheels[p]
		got := resp.Result.Wheels[p.String()]
		dc := math.Abs(got.Camber.Deg() - want.Camber.Deg())
		dt := math.Abs(got.ToeGeometric.Deg() - want.Toe.Deg())
		worstCamber, worstToe = math.Max(worstCamber, dc), math.Max(worstToe, dt)
		t.Logf("%-2s развал %+.3f° (задано %+.2f°)   схождение %+.3f° (задано %+.2f°)",
			p, got.Camber.Deg(), want.Camber.Deg(), got.ToeGeometric.Deg(), want.Toe.Deg())
	}
	if worstCamber > 0.2 {
		t.Errorf("худшая ошибка развала через HTTP %.3f°", worstCamber)
	}
	if worstToe > 0.2 {
		t.Errorf("худшая ошибка схождения через HTTP %.3f°", worstToe)
	}
	if d := math.Abs(resp.Result.ThrustAngle.Deg() - wantThrust.Deg()); d > 0.15 {
		t.Errorf("угол тяги через HTTP с ошибкой %.3f°", d)
	}

	// The report must be a normal alignment report, graded as usual.
	if len(resp.Report.Params) == 0 {
		t.Error("оптический замер должен давать обычный протокол с параметрами")
	}
}

// TestOpticalAlignRequiresLinkShot: with two reference boards and no photograph
// tying them together, the wheels cannot be brought into one frame, and the
// endpoint must say so rather than produce a plausible wrong answer.
func TestOpticalAlignRequiresLinkShot(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	camJSON, _ := json.Marshal(cam)
	rng := rand.New(rand.NewSource(5))

	tg := vision.Target{Cols: 8, Rows: 5, SquareMM: 32}
	pose := geom.Pose{R: geom.RotY(geom.Rad(-20)).Mul(geom.RotX(geom.Rad(12))), T: geom.V(0, 0, 900)}
	img := renderScenePNG(t, cam, []sceneBoard{{tg, pose}}, 0.004, rng)

	files := map[string][]upload{}
	for _, p := range align.AllPositions {
		files["images_"+p.String()] = []upload{
			{name: "a.png", data: img}, {name: "b.png", data: img}, {name: "c.png", data: img},
		}
	}
	rec := postMultipart(t, srv, "/api/optical/align", map[string]string{
		"camera_json": string(camJSON),
		"wheel_cols":  "8", "wheel_rows": "5", "wheel_square_mm": "32",
		"ref0_cols": "7", "ref0_rows": "6", "ref0_square_mm": "100",
		"ref1_cols": "6", "ref1_rows": "5", "ref1_square_mm": "100",
	}, files)

	if rec.Code == http.StatusOK {
		t.Errorf("два репера без связующего снимка должны быть отвергнуты, получено 200")
	}
}
