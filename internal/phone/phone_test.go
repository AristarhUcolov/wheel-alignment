package phone

import (
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
)

// simPhone is a phone with the faults real ones have. Tests are built from the
// physical statement — "this phone, held this way against this wheel" — and
// never from the equations the code solves.
type simPhone struct {
	sign     float64   // +1 reports the reaction to gravity (Android), −1 gravity itself (iOS)
	bias     geom.Vec3 // accelerometer zero offset, m/s²
	cross    float64   // fraction of the y reading leaking into z
	mount    float64   // degrees the phone sits tipped against the bar (camera bump)
	noise    float64   // m/s², per batch after the phone's own averaging
	gyroBias geom.Vec3 // deg/s
	rng      *rand.Rand
}

func typicalPhone(seed int64, sign float64) *simPhone {
	return &simPhone{
		// 0.01 m/s² per 100 ms batch is a noisy phone: about 1 mg after the
		// phone's own averaging of six readings, 0.06° per batch.
		sign: sign, bias: geom.V(0.08, -0.12, 0.25), cross: 0.01, mount: 1.2, noise: 0.01,
		gyroBias: geom.V(0.2, -0.35, 0.15), rng: rand.New(rand.NewSource(seed)),
	}
}

// frame is the phone's orientation: its x, y, z axes in the world (+Z up).
type frame struct{ x, y, z geom.Vec3 }

func (p *simPhone) read(f frame, omega geom.Vec3, dt float64) Sample {
	up := geom.V(0, 0, G)
	ax, ay, az := up.Dot(f.x), up.Dot(f.y), up.Dot(f.z)
	az += p.cross * ay
	n := func() float64 { return p.rng.NormFloat64() * p.noise }
	s := Sample{
		DT: dt, N: 6, HasGyro: true,
		AX: p.sign * (ax + p.bias.X + n()),
		AY: p.sign * (ay + p.bias.Y + n()),
		AZ: p.sign * (az + p.bias.Z + n()),
		GX: omega.Dot(f.x) + p.gyroBias.X,
		GY: omega.Dot(f.y) + p.gyroBias.Y,
		GZ: omega.Dot(f.z) + p.gyroBias.Z,
	}
	return s
}

func flatUp() frame   { return frame{geom.V(1, 0, 0), geom.V(0, 1, 0), geom.V(0, 0, 1)} }
func flatDown() frame { return frame{geom.V(-1, 0, 0), geom.V(0, 1, 0), geom.V(0, 0, -1)} }

// onRim is the phone pressed screen-out against a bar across the rim of a
// wheel whose outboard spin axis is a, upright or upside down, tipped by the
// phone's own mount angle about its x axis.
func (p *simPhone) onRim(a geom.Vec3, upright bool) frame {
	zc := a.Unit()
	yc := geom.V(0, 0, 1).Sub(zc.Scale(zc.Z)).Unit()
	if !upright {
		yc = yc.Neg()
	}
	xc := yc.Cross(zc)
	d := p.mount * math.Pi / 180
	return frame{xc, yc.Scale(math.Cos(d)).Add(zc.Scale(math.Sin(d))), yc.Scale(-math.Sin(d)).Add(zc.Scale(math.Cos(d)))}
}

func rotate(f frame, r geom.Mat3) frame { return frame{r.MulVec(f.x), r.MulVec(f.y), r.MulVec(f.z)} }

type rig struct {
	t    *testing.T
	l    *Link
	hub  *live.Hub
	now  time.Time
	id   string
	last PhoneState
}

func newRig(t *testing.T) *rig {
	hub := live.NewHub()
	r := &rig{t: t, hub: hub, now: time.Unix(2_000_000, 0), id: "testphone1"}
	hub.SetClock(func() time.Time { return r.now })
	r.l = NewLink(hub, t.TempDir())
	r.l.now = func() time.Time { return r.now }
	return r
}

func (r *rig) cmd(req phoneRequest) PhoneState {
	r.t.Helper()
	st, err := r.l.handle(r.id, req)
	if err != nil {
		r.t.Fatalf("command %+v: %v", req, err)
	}
	r.last = st
	return st
}

// hold keeps the phone in one orientation for d, one batch per 100 ms.
func (r *rig) hold(p *simPhone, f frame, d time.Duration) PhoneState {
	for t := time.Duration(0); t < d; t += 100 * time.Millisecond {
		r.now = r.now.Add(100 * time.Millisecond)
		r.last, _ = r.l.handle(r.id, phoneRequest{Samples: []Sample{p.read(f, geom.Vec3{}, 0.1)}})
	}
	return r.last
}

// shake is the phone being handled between positions.
func (r *rig) shake(p *simPhone, f frame) {
	for i := 0; i < 5; i++ {
		r.now = r.now.Add(100 * time.Millisecond)
		s := p.read(f, geom.Vec3{}, 0.1)
		s.AX += 2 * float64(i%2*2-1)
		r.l.handle(r.id, phoneRequest{Samples: []Sample{s}})
	}
}

func (r *rig) calibrate(p *simPhone, a geom.Vec3) {
	r.t.Helper()
	r.cmd(phoneRequest{Cmd: "flat"})
	r.hold(p, flatUp(), 3500*time.Millisecond)
	if r.last.Step != StepFlatDown {
		r.t.Fatalf("flat, screen up: still at %s (%s)", r.last.Step, r.last.Error)
	}
	r.hold(p, flatDown(), 3500*time.Millisecond)
	if !r.last.Flat || r.last.Step != StepMountUp {
		r.t.Fatalf("flat calibration did not complete: %+v", r.last)
	}
	r.hold(p, p.onRim(a, true), 3500*time.Millisecond)
	if r.last.Step != StepMountDown {
		r.t.Fatalf("mount, upright: still at %s (%s)", r.last.Step, r.last.Error)
	}
	r.shake(p, p.onRim(a, false))
	r.hold(p, p.onRim(a, false), 3500*time.Millisecond)
	if !r.last.Mounted || r.last.Step != StepLive {
		r.t.Fatalf("mount calibration did not complete: %+v", r.last)
	}
}

func spinAxis(p align.Position, camber, toe float64) geom.Vec3 {
	return simulate.WheelSpec{Camber: align.Deg(camber), Toe: align.Deg(toe)}.SpinAxis(p)
}

// TestCamberThroughCalibration: a phone whose accelerometer is off by 1.5° on
// its own, tipped 1.2° by its camera bump and leaking 1% of its y axis into z,
// must still read camber to within 0.05° — on the wheel it was calibrated on,
// on another wheel, and held upside down; both platform sign conventions. The
// simulated phone is a noisy one, and the remaining error is noise in the
// calibration itself (the zero offset is found to about 0.01°); a quieter phone
// does proportionally better.
func TestCamberThroughCalibration(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		r := newRig(t)
		p := typicalPhone(7, sign)
		r.cmd(phoneRequest{Wheel: "FL"})

		// Before calibration the phone must not reach the live screen: its
		// raw error is larger than any factory tolerance.
		r.hold(p, p.onRim(spinAxis(align.FL, -0.8, 0.1), true), 2*time.Second)
		if r.hub.Frame().Params["camber_FL"].Has {
			t.Fatal("an uncalibrated phone sent camber to the live screen")
		}
		raw := r.l.devices[r.id].Cal
		uncal, _ := raw.Camber(p.read(p.onRim(spinAxis(align.FL, -0.8, 0.1), true), geom.Vec3{}, 0.1))
		t.Logf("sign %+v: uncalibrated reading %.2f° for a true −0.80°", sign, uncal)

		r.calibrate(p, spinAxis(align.FL, -0.8, 0.1))

		for _, c := range []struct {
			wheel   align.Position
			camber  float64
			upright bool
		}{
			{align.FL, -0.8, true}, {align.FL, -0.8, false},
			{align.FR, 0.35, true}, {align.RR, -1.6, true}, {align.RL, 0.0, false},
		} {
			r.cmd(phoneRequest{Wheel: c.wheel.String()})
			r.shake(p, p.onRim(spinAxis(c.wheel, c.camber, 0.1), c.upright))
			// Held until settled, as a person waits for «стабильно».
			st := r.hold(p, p.onRim(spinAxis(c.wheel, c.camber, 0.1), c.upright), 5*time.Second)
			if st.Camber == nil {
				t.Fatalf("%s: no camber shown on the phone", c.wheel)
			}
			got := r.hub.Frame().Params["camber_"+c.wheel.String()]
			if !got.Has || math.Abs(got.Value-c.camber) > 0.05 {
				t.Errorf("sign %+v, %s %s: live camber %.3f°, true %.2f°", sign, c.wheel, map[bool]string{true: "upright", false: "upside down"}[c.upright], got.Value, c.camber)
			}
			if !got.Stable {
				t.Errorf("%s: a phone held still was not reported as settled", c.wheel)
			}
		}
	}
}

// TestCalibrationSurvivesRestart: the phone calibrates once, not every day.
func TestCalibrationSurvivesRestart(t *testing.T) {
	r := newRig(t)
	p := typicalPhone(3, 1)
	r.cmd(phoneRequest{Wheel: "FL"})
	r.calibrate(p, spinAxis(align.FL, 0.2, 0))
	dir := r.l.dir

	l2 := NewLink(live.NewHub(), dir)
	cal, ok := l2.cals[r.id]
	if !ok || !cal.Ready() {
		t.Fatal("calibration not reloaded")
	}
	if math.Abs(cal.Mount-r.l.devices[r.id].Cal.Mount) > 1e-9 {
		t.Error("reloaded mount offset differs")
	}
}

// TestFlatCalibrationChecks: a phone that is not flat, or never turned over,
// must not produce a calibration.
func TestFlatCalibrationChecks(t *testing.T) {
	r := newRig(t)
	p := typicalPhone(5, 1)
	r.cmd(phoneRequest{Cmd: "flat"})
	tilted := rotate(flatUp(), geom.Rodrigues(geom.V(1, 0, 0), 20*math.Pi/180))
	st := r.hold(p, tilted, 3500*time.Millisecond)
	if st.Step != StepFlatUp || st.Error == "" {
		t.Errorf("a phone tilted 20° was accepted as flat: %+v", st)
	}
	r.hold(p, flatUp(), 3500*time.Millisecond)
	st = r.hold(p, flatUp(), 4*time.Second) // never turned over
	if st.Flat {
		t.Error("calibration completed without the phone being turned over")
	}
}

// TestRunoutCompensation: a rim face tipped 0.4° relative to the wheel reads
// camber+0.4 at one position and camber−0.4 half a turn later. After the
// procedure the phone shows the true camber.
func TestRunoutCompensation(t *testing.T) {
	r := newRig(t)
	p := typicalPhone(11, 1)
	r.cmd(phoneRequest{Wheel: "RL"})
	const camber, runout = -1.2, 0.4
	r.calibrate(p, spinAxis(align.RL, camber, 0))

	r.cmd(phoneRequest{Cmd: "runout"})
	st := r.hold(p, p.onRim(spinAxis(align.RL, camber+runout, 0), true), 2500*time.Millisecond)
	if st.Step != StepRunout2 {
		t.Fatalf("runout first reading not taken: %+v", st)
	}
	// Left on the rim without rolling the car: must not be taken as the second
	// reading.
	st = r.hold(p, p.onRim(spinAxis(align.RL, camber+runout, 0), true), 2500*time.Millisecond)
	if st.Step != StepRunout2 {
		t.Fatal("second runout reading taken without the phone being moved")
	}
	r.shake(p, p.onRim(spinAxis(align.RL, camber-runout, 0), true))
	st = r.hold(p, p.onRim(spinAxis(align.RL, camber-runout, 0), true), 2500*time.Millisecond)
	if st.Runout == nil {
		t.Fatalf("no runout after the second reading: %+v", st)
	}
	if math.Abs(*st.Runout-runout) > 0.02 {
		t.Fatalf("runout %.3f, want %.2f", *st.Runout, runout)
	}
	st = r.hold(p, p.onRim(spinAxis(align.RL, camber-runout, 0), true), 2*time.Second)
	if got := r.hub.Frame().Params["camber_RL"].Value; math.Abs(got-camber) > 0.03 {
		t.Errorf("compensated camber %.3f, true %.2f", got, camber)
	}
}

// TestCasterByGyroscope: the whole guided sweep — straight, out, in, back — with
// the steering angle measured by a drifting gyroscope, on both front wheels.
// Caster must come out within a tenth of a degree, and camber must not be
// published while the wheel is turned (it is not the straight-ahead camber).
func TestCasterByGyroscope(t *testing.T) {
	for _, pos := range []align.Position{align.FL, align.FR} {
		r := newRig(t)
		p := typicalPhone(int64(pos)+20, 1)
		w := simulate.WheelSpec{Camber: align.Deg(-0.5), Toe: align.Deg(0.08), Caster: align.Deg(3.5), SAI: align.Deg(12)}
		a0 := w.SpinAxis(pos)
		r.cmd(phoneRequest{Wheel: pos.String()})
		r.calibrate(p, a0)
		f0 := p.onRim(a0, true)

		before := r.hub.Frame().Params["camber_"+pos.String()].Value
		r.cmd(phoneRequest{Cmd: "caster"})
		r.hold(p, f0, 2500*time.Millisecond)
		if r.last.Step != StepCasterOut {
			t.Fatalf("%s: straight-ahead reading not taken: %+v", pos, r.last)
		}
		k := w.SteeringAxis(pos)
		s := pos.SideSign()
		// Steer: θ is the outboard steer angle; the body turns about k by s·θ.
		steer := func(from, to float64, secs float64) frame {
			steps := int(secs / 0.1)
			var f frame
			for i := 1; i <= steps; i++ {
				th := from + (to-from)*float64(i)/float64(steps)
				rate := (to - from) / secs // deg/s of outboard steer
				f = rotate(f0, geom.Rodrigues(k, s*th*math.Pi/180))
				omega := k.Scale(s * rate)
				r.now = r.now.Add(100 * time.Millisecond)
				r.last, _ = r.l.handle(r.id, phoneRequest{Samples: []Sample{p.read(f, omega, 0.1)}})
				// Nothing may be published while the wheel is turned: the
				// camber then is not the straight-ahead camber.
				if fr := r.hub.Frame().Params["camber_"+pos.String()]; r.last.Step != StepLive && fr.Value != before {
					t.Fatalf("%s: camber %.3f published while the wheel was turned", pos, fr.Value)
				}
			}
			return f
		}
		fOut := steer(0, 19.4, 2)
		r.hold(p, fOut, 2500*time.Millisecond)
		if r.last.Step != StepCasterIn {
			t.Fatalf("%s: outward reading not taken at 19.4° (steer %v): %+v", pos, r.last.Steer, r.last)
		}
		fIn := steer(19.4, -21.1, 4)
		r.hold(p, fIn, 2500*time.Millisecond)
		if r.last.Step != StepCasterBack || r.last.Caster == nil {
			t.Fatalf("%s: inward reading not taken: %+v", pos, r.last)
		}
		steer(-21.1, 0, 2)
		r.hold(p, f0, 2500*time.Millisecond)
		if r.last.Step != StepLive {
			t.Errorf("%s: did not return to live after straightening: %s", pos, r.last.Step)
		}
		got := r.hub.Frame().Params["caster_"+pos.String()]
		if !got.Has || math.Abs(got.Value-3.5) > 0.1 {
			t.Errorf("%s: caster %.3f°, true 3.50°", pos, got.Value)
		}
		t.Logf("%s: caster %.3f° (true 3.5°), SAI %v", pos, got.Value, r.hub.Frame().Params["sai_"+pos.String()].Value)
	}
}

// TestPhonePageNeedsTheKey: nothing is served without the current key.
func TestPhonePageNeedsTheKey(t *testing.T) {
	l := NewLink(live.NewHub(), t.TempDir())
	l.SetToken("abc123")
	h := l.Handler()
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/p/wrong", http.StatusNotFound},
		{"POST", "/p/wrong/data", http.StatusNotFound},
		{"GET", "/p/abc123", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, strings.NewReader(`{"d":"x"}`)))
		if rec.Code != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, rec.Code, c.want)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/p/abc123/data", strings.NewReader(`{"d":"phone1","wheel":"FR"}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"wheel":"FR"`) {
		t.Errorf("valid request: %d %s", rec.Code, rec.Body.String())
	}
}

func TestQRCode(t *testing.T) {
	svg, err := qrSVG("https://192.168.1.10:8701/p/abcdefghij")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") {
		t.Error("not an SVG QR code")
	}
}
