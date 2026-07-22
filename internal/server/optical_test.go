package server_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/server"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

func testServer(t *testing.T) *server.Server {
	t.Helper()
	db, err := specs.Load()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func opticalTestCamera() vision.Camera {
	return vision.Camera{
		Width: 1280, Height: 720, Fx: 980, Fy: 981, Cx: 637, Cy: 361,
		K1: -0.21, K2: 0.07, P1: 0.0004, P2: -0.0003,
		Calibrated: true, CalibrationRMSPx: 0.2,
	}
}

// renderBoardPNG draws a checkerboard as the camera sees it and encodes it as
// PNG, so the server's own decode path is exercised, not a shortcut. Inverse ray
// casting through the real lens model, as in the vision tests.
func renderBoardPNG(t *testing.T, cam vision.Camera, tg vision.Target, pose geom.Pose, noise float64, rng *rand.Rand) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, cam.Width, cam.Height))
	s := tg.SquareMM
	wMM, hMM := float64(tg.Cols+1)*s, float64(tg.Rows+1)*s
	x0, y0 := -wMM/2, -hMM/2
	inv := pose.Inverse()
	normal := pose.R.Col(2)
	planeD := pose.T.Dot(normal)

	for py := 0; py < cam.Height; py++ {
		for px := 0; px < cam.Width; px++ {
			var sum float64
			const super = 3
			for sy := 0; sy < super; sy++ {
				for sx := 0; sx < super; sx++ {
					u := float64(px) - 0.5 + (float64(sx)+0.5)/super
					v := float64(py) - 0.5 + (float64(sy)+0.5)/super
					d := cam.Ray(vision.Point2{X: u, Y: v})
					den := d.Dot(normal)
					if math.Abs(den) < 1e-9 {
						sum += 0.45
						continue
					}
					q := inv.Apply(d.Scale(planeD / den))
					if q.X < x0 || q.Y < y0 || q.X >= x0+wMM || q.Y >= y0+hMM {
						sum += 0.45
						continue
					}
					ix, iy := int(math.Floor((q.X-x0)/s)), int(math.Floor((q.Y-y0)/s))
					if (ix+iy)%2 == 0 {
						sum += 0.12
					} else {
						sum += 0.88
					}
				}
			}
			val := sum/(super*super) + rng.NormFloat64()*noise
			img.SetGray(px, py, color.Gray{Y: uint8(math.Max(0, math.Min(255, val*255)))})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type upload struct {
	name string
	data []byte
}

func postMultipart(t *testing.T, srv *server.Server, path string, fields map[string]string, files map[string][]upload) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for field, ups := range files {
		for _, u := range ups {
			fw, err := mw.CreateFormFile(field, u.name)
			if err != nil {
				t.Fatal(err)
			}
			fw.Write(u.data)
		}
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// TestCalibrateEndpoint drives calibration through the HTTP layer: PNGs posted
// as multipart, a camera and a quality verdict returned.
func TestCalibrateEndpoint(t *testing.T) {
	srv := testServer(t)
	truth := opticalTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(101))

	poses := []geom.Pose{
		{R: geom.RotX(geom.Rad(28)), T: geom.V(-120, -60, 800)},
		{R: geom.RotY(geom.Rad(-31)), T: geom.V(130, 55, 820)},
		{R: geom.RotY(geom.Rad(26)).Mul(geom.RotX(geom.Rad(-24))), T: geom.V(120, -70, 780)},
		{R: geom.RotZ(geom.Rad(20)).Mul(geom.RotX(geom.Rad(-30))), T: geom.V(-125, 70, 860)},
		{R: geom.RotY(geom.Rad(34)).Mul(geom.RotX(geom.Rad(20))), T: geom.V(0, 0, 700)},
		{R: geom.RotZ(geom.Rad(-25)).Mul(geom.RotY(geom.Rad(-28))), T: geom.V(-60, 40, 900)},
	}
	var ups []upload
	for i, p := range poses {
		ups = append(ups, upload{name: "shot" + strconv.Itoa(i) + ".png", data: renderBoardPNG(t, truth, tg, p, 0.004, rng)})
	}

	rec := postMultipart(t, srv, "/api/optical/calibrate",
		map[string]string{"cols": "9", "rows": "6", "square_mm": "30"},
		map[string][]upload{"images": ups})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Camera    vision.Camera `json:"camera"`
		RMSPx     float64       `json:"rms_px"`
		ViewsUsed int           `json:"views_used"`
		Quality   string        `json:"quality"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	t.Logf("камера с эндпойнта: fx %.1f (истина %.1f), СКО %.3f, кадров %d, качество %s",
		resp.Camera.Fx, truth.Fx, resp.RMSPx, resp.ViewsUsed, resp.Quality)

	if !resp.Camera.Calibrated {
		t.Error("returned camera must be flagged calibrated")
	}
	if rel := math.Abs(resp.Camera.Fx-truth.Fx) / truth.Fx; rel > 0.02 {
		t.Errorf("fx off by %.2f%% through the HTTP layer", rel*100)
	}
}

// TestOpticalCamberEndpoint drives the per-wheel camber measurement through
// HTTP, with the camera calibration posted alongside the frames. A deliberately
// crooked target must not matter — the same runout-compensation guarantee, now
// over the wire.
func TestOpticalCamberEndpoint(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(202))
	const camberDeg, clampErr = -1.1, 2.0

	wheel := simulate.WheelSpec{Camber: align.Deg(camberDeg), Toe: align.Deg(0.15), Center: geom.V(0, 760, 315)}
	spinAxis := wheel.SpinAxis(align.FL)

	// Camera outboard, level: its down axis (+Y) is world-down, so the level
	// assumption the endpoint defaults to actually holds here.
	camPos := geom.V(-300, 1700, 315)
	fwd := wheel.Center.Sub(camPos)
	fwd = geom.V(fwd.X, fwd.Y, 0).Unit() // keep the optical axis horizontal → level camera
	right := geom.V(0, 0, 1).Cross(fwd).Unit()
	down := geom.V(0, 0, -1)
	rCam := geom.FromRows(right, down, fwd)
	vehicleToCam := geom.Pose{R: rCam, T: rCam.MulVec(camPos).Neg()}

	tiltAxis := spinAxis.Any()
	mountR := geom.Rodrigues(tiltAxis, geom.Rad(clampErr)).Mul(geom.RotationBetween(geom.V(0, 0, 1), spinAxis))
	mountT := wheel.Center.Add(spinAxis.Scale(90))

	var ups []upload
	for i, spin := range []float64{0, 42, 88, 133, 181, 228} {
		spinR := geom.Rodrigues(spinAxis, geom.Rad(spin))
		inVehicle := geom.Pose{R: spinR.Mul(mountR), T: spinR.MulVec(mountT.Sub(wheel.Center)).Add(wheel.Center)}
		png := renderBoardPNG(t, cam, tg, vehicleToCam.Mul(inVehicle), 0.005, rng)
		ups = append(ups, upload{name: "f" + strconv.Itoa(i) + ".png", data: png})
	}

	camJSON, _ := json.Marshal(cam)
	rec := postMultipart(t, srv, "/api/optical/camber",
		map[string]string{"position": "FL", "camera_json": string(camJSON), "cols": "9", "rows": "6", "square_mm": "30"},
		map[string][]upload{"images": ups})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		CamberDeg    float64 `json:"camber_deg"`
		RunoutDeg    float64 `json:"runout_deg"`
		SweepDeg     float64 `json:"sweep_deg"`
		Used         int     `json:"used"`
		LevelAssumed bool    `json:"level_assumed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	t.Logf("развал с эндпойнта %.4f° (задано %.2f°), биение %.2f°, поворот %.0f°, кадров %d",
		resp.CamberDeg, camberDeg, resp.RunoutDeg, resp.SweepDeg, resp.Used)

	if resp.Used != len(ups) {
		t.Errorf("used %d of %d frames", resp.Used, len(ups))
	}
	if d := math.Abs(resp.CamberDeg - camberDeg); d > 0.15 {
		t.Errorf("camber off by %.4f° through the HTTP layer", d)
	}
	if !resp.LevelAssumed {
		t.Error("with no gravity supplied, the level assumption must be reported")
	}
}

// TestOpticalCamberRejectsMissingCamera: without a calibration the endpoint must
// refuse rather than invent intrinsics.
func TestOpticalCamberRejectsMissingCamera(t *testing.T) {
	srv := testServer(t)
	cam := opticalTestCamera()
	tg := vision.DefaultTarget()
	rng := rand.New(rand.NewSource(3))
	pose := geom.Pose{R: geom.RotY(geom.Rad(-20)).Mul(geom.RotX(geom.Rad(12))), T: geom.V(0, 0, 900)}

	ups := []upload{
		{name: "a.png", data: renderBoardPNG(t, cam, tg, pose, 0.004, rng)},
		{name: "b.png", data: renderBoardPNG(t, cam, tg, geom.Pose{R: pose.R.Mul(geom.RotZ(geom.Rad(20))), T: pose.T}, 0.004, rng)},
		{name: "c.png", data: renderBoardPNG(t, cam, tg, geom.Pose{R: pose.R.Mul(geom.RotZ(geom.Rad(40))), T: pose.T}, 0.004, rng)},
	}
	rec := postMultipart(t, srv, "/api/optical/camber",
		map[string]string{"position": "FL"},
		map[string][]upload{"images": ups})

	if rec.Code == http.StatusOK {
		t.Errorf("missing camera calibration must be rejected, got 200: %s", rec.Body.String())
	}
}
