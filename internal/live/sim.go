package live

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// SimSource is the source id the simulator publishes under.
const SimSource = "sim"

// Simulator is a demonstration car on the live screen. It exists so that the
// screen can be learnt — what the needles do, what "settled" looks like, what
// order the adjustments go in — before anyone is lying under a real car.
//
// It behaves like a car being adjusted rather than like a slideshow: each
// angle moves toward its target with the lag of a suspension settling, the
// readings carry sensor noise, and changing caster or camber disturbs toe the
// way it does on a real double-wishbone front end. That last part is the whole
// reason toe is adjusted last, and it is easier to see than to read about.
type Simulator struct {
	hub *Hub

	mu      sync.Mutex
	truth   map[string]float64
	target  map[string]float64
	running bool
	auto    bool
	stop    chan struct{}
	done    chan struct{}
	rng     *rand.Rand
	lastPub map[string]float64
	autoAt  time.Time
}

// Simulated noise, in degrees: about what a phone accelerometer shows after
// its own averaging.
const simNoise = 0.006

// settleTau is how quickly a simulated angle follows its adjuster.
const settleTau = 0.6 // seconds

// NewSimulator returns a stopped simulator attached to h.
func NewSimulator(h *Hub) *Simulator {
	s := &Simulator{hub: h, rng: rand.New(rand.NewSource(1))}
	s.reset()
	return s
}

// The demonstration car: a mid-size front-wheel-drive saloon with a lopsided
// front camber, a toed-out right front and a rear axle pointing slightly
// right — enough wrong for every part of the screen to have something to show.
var simStart = map[string]float64{
	"camber_FL": -0.90, "toe_FL": 0.20, "caster_FL": 3.10, "sai_FL": 12.5,
	"camber_FR": -0.20, "toe_FR": -0.05, "caster_FR": 3.60, "sai_FR": 12.4,
	"camber_RL": -1.40, "toe_RL": 0.18,
	"camber_RR": -1.10, "toe_RR": 0.02,
}

func (s *Simulator) reset() {
	s.truth = map[string]float64{}
	s.target = map[string]float64{}
	s.lastPub = map[string]float64{}
	for k, v := range simStart {
		s.truth[k], s.target[k] = v, v
	}
}

// Start begins publishing. Calling it on a running simulator does nothing.
func (s *Simulator) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.hub.Touch(SourceInfo{ID: SimSource, Kind: "sim", Name: i18n.N("Демонстрационный автомобиль"),
		Detail: i18n.N("все четыре колеса, кастер")})
	go s.loop(s.stop, s.done)
}

// Stop halts the simulator and removes its readings from the screen.
func (s *Simulator) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running, s.auto = false, false
	close(s.stop)
	done := s.done
	s.mu.Unlock()
	<-done
	s.hub.RemoveSource(SimSource)
	s.hub.Clear(SimSource)
}

// Running reports whether the simulator is publishing.
func (s *Simulator) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Restart puts the car back to its initial, misaligned state.
func (s *Simulator) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset()
	s.auto = false
}

// SetAuto switches the demonstration of an adjustment on or off: the
// simulator then walks every out-of-spec angle into its band, in the order a
// person should.
func (s *Simulator) SetAuto(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auto = on
	s.autoAt = time.Time{}
}

// Auto reports whether the adjustment demonstration is running.
func (s *Simulator) Auto() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.auto
}

// ErrNotAdjustable is returned for a key the simulator has no adjuster for.
var ErrNotAdjustable = i18n.Err("этот параметр в демонстрации не регулируется")

// Adjust turns a simulated adjuster: the target for key moves by delta
// degrees, and the reading follows with a lag.
func (s *Simulator) Adjust(key string, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.target[key]; !ok || len(key) < 4 || key[:4] == "sai_" {
		return ErrNotAdjustable
	}
	s.adjustLocked(key, delta)
	return nil
}

// adjustLocked applies the coupling a real suspension has: moving caster or
// camber drags toe with it. The coefficients are illustrative, not a model of
// any particular car; what matters is that the effect exists and is visible.
func (s *Simulator) adjustLocked(key string, delta float64) {
	s.target[key] += delta
	w := key[len(key)-2:]
	switch key[:len(key)-2] {
	case specs.KeyCaster:
		s.target[specs.KeyToe+w] += 0.08 * delta
	case specs.KeyCamber:
		s.target[specs.KeyToe+w] += 0.05 * delta
	}
}

func (s *Simulator) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	const dt = 50 * time.Millisecond
	t := time.NewTicker(dt)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.step(dt.Seconds())
		}
	}
}

// step advances the simulation by dt seconds and publishes a reading.
func (s *Simulator) step(dt float64) {
	s.mu.Lock()
	alpha := 1 - math.Exp(-dt/settleTau)
	for k, v := range s.target {
		s.truth[k] += alpha * (v - s.truth[k])
	}
	if s.auto {
		s.autoStepLocked()
	}
	type out struct {
		p   align.Position
		c   float64
		toe float64
	}
	var readings []out
	for _, p := range align.AllPositions {
		w := p.String()
		readings = append(readings, out{p,
			s.truth[specs.KeyCamber+w] + s.rng.NormFloat64()*simNoise,
			s.truth[specs.KeyToe+w] + s.rng.NormFloat64()*simNoise})
	}
	// Caster is published only when it has moved: in reality it comes from a
	// sweep, not a continuous stream, and a live value that jitters would
	// suggest a precision the method does not have.
	var sweeps []SweepInput
	for _, p := range []align.Position{align.FL, align.FR} {
		w := p.String()
		c, sai := s.truth[specs.KeyCaster+w], s.truth[specs.KeySAI+w]
		if last, ok := s.lastPub[specs.KeyCaster+w]; !ok || math.Abs(last-c) > 0.004 {
			s.lastPub[specs.KeyCaster+w] = c
			sweeps = append(sweeps, SweepInput{Wheel: p, Caster: round4(c), SAI: &sai, Source: SimSource})
		}
	}
	s.mu.Unlock()

	for _, r := range readings {
		c, t := r.c, r.toe
		_ = s.hub.Push(Input{Wheel: r.p, Camber: &c, Toe: &t, Source: SimSource})
	}
	for _, sw := range sweeps {
		_ = s.hub.PushSweep(sw)
	}
}

// autoOrder is the adjustment order the demonstration follows — the same one
// the printed procedure prescribes: rear axle first (it defines the thrust
// line), then caster, then camber, and toe last because both of those move it.
var autoOrder = []string{
	"camber_RL", "camber_RR", "toe_RL", "toe_RR",
	"caster_FL", "caster_FR", "camber_FL", "camber_FR", "toe_FL", "toe_FR",
}

// autoStepLocked makes one move of the demonstration every so often: finds the
// first angle in autoOrder whose reading is outside the middle of its band and
// turns its adjuster a notch toward nominal.
func (s *Simulator) autoStepLocked() {
	now := time.Now()
	if now.Sub(s.autoAt) < 700*time.Millisecond {
		return
	}
	s.autoAt = now
	lim := s.hub.Config().Limits
	if lim == nil {
		s.auto = false
		return
	}
	for _, key := range autoOrder {
		r := specs.Lookup(lim, key).Spec
		if r == nil {
			continue
		}
		// Aim for the nominal; stop once within a fifth of the band of it —
		// nobody should chase the last hundredth of a degree.
		off := r.Nominal.Deg() - s.target[key]
		band := (r.Max - r.Min).Deg()
		if math.Abs(off) <= band/5 && math.Abs(r.Nominal.Deg()-s.truth[key]) <= band/4 {
			continue
		}
		if math.Abs(s.target[key]-s.truth[key]) > 0.03 {
			return // still settling from the previous notch
		}
		step := math.Copysign(math.Min(math.Abs(off), 0.12), off)
		s.adjustLocked(key, step)
		return
	}
	s.auto = false // done
}
