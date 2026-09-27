package live

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// fakeClock is a hand-advanced clock.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time      { return c.t }
func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }
func f(v float64) *float64               { return &v }
func near(a, b, tol float64) bool        { return math.Abs(a-b) <= tol }
func newTestHub() (*Hub, *fakeClock) {
	c := &fakeClock{t: time.Unix(1_000_000, 0)}
	h := NewHub()
	h.SetClock(c.now)
	return h, c
}
func pushAll(h *Hub, cam, toe [4]float64, instant bool, src string) {
	for _, p := range align.AllPositions {
		_ = h.Push(Input{Wheel: p, Camber: f(cam[p]), Toe: f(toe[p]), Instant: instant, Source: src})
	}
}

func guidance(t *testing.T) *specs.Spec {
	t.Helper()
	db, err := specs.Load()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := db.Get("guidance-fwd-mcpherson")
	if !ok {
		t.Fatal("guidance entry missing")
	}
	return &s
}

// TestLiveMatchesAssemble: the live screen and the printed report must agree
// to the last digit on every derived value — thrust line, thrust-referenced
// front toe, totals, cross camber. Anything else would mean two implementations
// of the same maths, and one of them wrong.
func TestLiveMatchesAssemble(t *testing.T) {
	h, _ := newTestHub()
	h.Configure(Config{Limits: guidance(t), RimDiameterMM: align.Inches(15)})
	cam := [4]float64{-0.9, -0.2, -1.4, -1.1}
	toe := [4]float64{0.20, -0.05, 0.18, 0.02}
	pushAll(h, cam, toe, true, "manual")

	fr := h.Frame()
	res, err := h.Result()
	if err != nil {
		t.Fatal(err)
	}
	if !fr.ThrustReferenced {
		t.Fatal("both rear toes are known, front toe must be thrust-referenced")
	}
	check := func(key string, want float64) {
		t.Helper()
		p := fr.Params[key]
		if !p.Has || !near(p.Value, want, 1e-4) {
			t.Errorf("%s: live %.5f (has=%v), report %.5f", key, p.Value, p.Has, want)
		}
	}
	for _, p := range align.AllPositions {
		w := res.Wheels[p.String()]
		check("camber_"+p.String(), w.Camber.Deg())
		check("toe_"+p.String(), w.ToeThrust.Deg())
	}
	check("thrust_angle", res.ThrustAngle.Deg())
	check("front_total_toe", res.Front.TotalToe.Deg())
	check("rear_total_toe", res.Rear.TotalToe.Deg())
	check("front_cross_camber", res.Front.CrossCamber.Deg())

	// Grading comes from the same tolerance as the report's.
	rep := specs.Compare(res, guidance(t))
	for _, p := range rep.Params {
		if lp, ok := fr.Params[p.Key]; ok && lp.Has && lp.Status != p.Status {
			t.Errorf("%s: live grades %s, report grades %s", p.Key, lp.Status, p.Status)
		}
	}
}

// TestMissingWheelsAreNotZero: a wheel nobody has measured must show as
// unmeasured, not as a perfect 0°00'.
func TestMissingWheelsAreNotZero(t *testing.T) {
	h, _ := newTestHub()
	h.Configure(Config{Limits: guidance(t)})
	_ = h.Push(Input{Wheel: align.FL, Camber: f(-0.5), Instant: true, Source: "manual"})
	fr := h.Frame()
	if !fr.Params["camber_FL"].Has {
		t.Fatal("measured camber not shown")
	}
	for _, k := range []string{"camber_FR", "toe_FL", "front_total_toe", "front_cross_camber", "thrust_angle", "caster_FL"} {
		p := fr.Params[k]
		if p.Has {
			t.Errorf("%s shown as measured", k)
		}
		if p.Status != align.StatusNoSpec {
			t.Errorf("%s graded %q although unmeasured", k, p.Status)
		}
	}
	if fr.ThrustReferenced {
		t.Error("thrust line claimed without rear toe")
	}
	if fr.Measured != 1 {
		t.Errorf("measured count %d, want 1", fr.Measured)
	}
	if _, err := h.Result(); err == nil {
		t.Error("a report was assembled from one wheel")
	}
}

// TestSmoothingAndSettling: a noisy stream converges, is reported unsettled
// while the adjuster is moving, and settled once it stops.
func TestSmoothingAndSettling(t *testing.T) {
	h, c := newTestHub()
	// A ramp: somebody turning an adjuster.
	for i := 0; i < 20; i++ {
		_ = h.Push(Input{Wheel: align.FL, Camber: f(-1.0 + 0.02*float64(i)), Source: "phone"})
		c.add(100 * time.Millisecond)
	}
	if h.Frame().Params["camber_FL"].Stable {
		t.Error("a steadily changing reading was reported as settled")
	}
	// Then it stops at -0.6 with a little jitter.
	jit := []float64{0.004, -0.003, 0.002, -0.004, 0.003, 0, -0.002, 0.001, 0.003, -0.001, 0.002, -0.003, 0.001, 0, -0.002, 0.002}
	for _, j := range jit {
		_ = h.Push(Input{Wheel: align.FL, Camber: f(-0.6 + j), Source: "phone"})
		c.add(100 * time.Millisecond)
	}
	p := h.Frame().Params["camber_FL"]
	if !p.Stable {
		t.Error("a settled reading was not reported as settled")
	}
	if !near(p.Value, -0.6, 0.01) {
		t.Errorf("smoothed value %.4f, want about -0.6", p.Value)
	}

	// Silence makes it stale — but it stays on screen.
	c.add(5 * time.Second)
	p = h.Frame().Params["camber_FL"]
	if !p.Stale || p.Stable || !p.Has {
		t.Errorf("after 5 s of silence: stale=%v stable=%v has=%v", p.Stale, p.Stable, p.Has)
	}
}

// TestTypedReadingIsExact: a value typed in by hand is shown as typed, at once.
func TestTypedReadingIsExact(t *testing.T) {
	h, _ := newTestHub()
	_ = h.Push(Input{Wheel: align.RR, Toe: f(0.1), Instant: true, Source: "manual"})
	_ = h.Push(Input{Wheel: align.RR, Toe: f(0.3), Instant: true, Source: "manual"})
	p := h.Frame().Params["toe_RR"]
	if p.Value != 0.3 || !p.Stable {
		t.Errorf("typed toe shown as %.4f stable=%v, want exactly 0.3 and settled", p.Value, p.Stable)
	}
}

func TestImplausibleReadingsRejected(t *testing.T) {
	h, _ := newTestHub()
	if err := h.Push(Input{Wheel: align.FL, Camber: f(45), Source: "x"}); err == nil {
		t.Error("45° of camber accepted")
	}
	if err := h.Push(Input{Wheel: align.FL, Toe: f(math.NaN()), Source: "x"}); err == nil {
		t.Error("NaN toe accepted")
	}
	if err := h.PushSweep(SweepInput{Wheel: align.RL, Caster: 3}); err == nil {
		t.Error("caster accepted on a rear wheel")
	}
}

// TestOverallVerdict: green only when everything measured is inside its band.
func TestOverallVerdict(t *testing.T) {
	h, _ := newTestHub()
	g := guidance(t)
	h.Configure(Config{Limits: g})
	nomCam := func(front bool) float64 {
		if front {
			return g.Front.Camber.Nominal.Deg()
		}
		return g.Rear.Camber.Nominal.Deg()
	}
	nomToe := func(front bool) float64 {
		if front {
			return g.Front.IndividualToe.Nominal.Deg()
		}
		return g.Rear.IndividualToe.Nominal.Deg()
	}
	var cam, toe [4]float64
	for _, p := range align.AllPositions {
		cam[p], toe[p] = nomCam(p.IsFront()), nomToe(p.IsFront())
	}
	pushAll(h, cam, toe, true, "manual")
	if fr := h.Frame(); fr.Overall != align.StatusGood || fr.OutOfSpec != 0 {
		t.Errorf("a car at nominal everywhere graded %s with %d out of spec", fr.Overall, fr.OutOfSpec)
	}
	_ = h.Push(Input{Wheel: align.FL, Camber: f(3), Instant: true, Source: "manual"})
	if fr := h.Frame(); fr.Overall != align.StatusBad || fr.OutOfSpec == 0 {
		t.Errorf("3° of camber not flagged: %s, %d", fr.Overall, fr.OutOfSpec)
	}
}

// TestSubscribersGetFrames: the broadcaster delivers decodable frames.
func TestSubscribersGetFrames(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	ch, stop := h.Subscribe()
	defer stop()
	_ = h.Push(Input{Wheel: align.FL, Camber: f(-0.25), Instant: true, Source: "manual"})
	deadline := time.After(3 * time.Second)
	for {
		select {
		case b := <-ch:
			var fr Frame
			if err := json.Unmarshal(b, &fr); err != nil {
				t.Fatal(err)
			}
			if p := fr.Params["camber_FL"]; p.Has && p.Value == -0.25 {
				return
			}
		case <-deadline:
			t.Fatal("no frame carrying the reading arrived")
		}
	}
}

// TestSimulatorDemonstrationConverges: the automatic demonstration must end
// with every adjustable angle in its band — otherwise it teaches nothing.
func TestSimulatorDemonstrationConverges(t *testing.T) {
	h := NewHub()
	g := guidance(t)
	h.Configure(Config{Limits: g})
	s := NewSimulator(h)
	s.mu.Lock()
	s.auto = true
	s.mu.Unlock()
	for i := 0; i < 4000 && s.Auto(); i++ {
		s.mu.Lock()
		s.autoAt = time.Time{} // do not wait for the wall clock
		s.mu.Unlock()
		s.step(0.05)
	}
	if s.Auto() {
		t.Fatal("the demonstration never finished")
	}
	for i := 0; i < 100; i++ { // let the last notch settle
		s.step(0.05)
	}
	fr := h.Frame()
	for _, k := range autoOrder {
		p := fr.Params[k]
		if p.Spec != nil && p.Status == align.StatusBad {
			t.Errorf("%s ended at %.3f°, outside %v", k, p.Value, *p.Spec)
		}
	}
}

// TestSimulatorCouplesToe: turning caster must drag toe with it, which is the
// lesson the simulator exists to teach.
func TestSimulatorCouplesToe(t *testing.T) {
	s := NewSimulator(NewHub())
	before := s.target["toe_FL"]
	if err := s.Adjust("caster_FL", 1); err != nil {
		t.Fatal(err)
	}
	if s.target["toe_FL"] == before {
		t.Error("caster adjustment left toe untouched")
	}
	if err := s.Adjust("sai_FL", 1); err == nil {
		t.Error("SAI has no adjuster and must be refused")
	}
}
