package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
	"github.com/AristarhUcolov/wheel-alignment/internal/suspension"
)

// Config is what the live screen is measuring against.
type Config struct {
	// Vehicle is the entry the person chose; Limits is the entry whose
	// tolerances apply. They differ for a catalog entry, whose limits are the
	// class guidance it names. Either may be nil.
	Vehicle *specs.Spec
	Limits  *specs.Spec

	FrontSuspension suspension.Type
	RearSuspension  suspension.Type

	// RimDiameterMM converts toe angles to millimetres for display.
	RimDiameterMM float64

	TrackFrontMM float64
	TrackRearMM  float64
}

// Input is one reading from a sensor. Any field left nil is simply not
// measured by that sensor: a phone knows camber but not toe, a string line
// knows toe but not camber.
type Input struct {
	Wheel  align.Position
	Camber *float64 // degrees, positive = top of the wheel outboard
	Toe    *float64 // degrees, positive = toe-in, relative to the geometric centreline

	// Instant marks a reading typed in by hand: exact, never smoothed.
	Instant bool
	Source  string
}

// SweepInput is the outcome of a caster sweep on one steered wheel.
type SweepInput struct {
	Wheel    align.Position
	Caster   float64  // degrees
	SAI      *float64 // degrees, when the sweep resolved it
	Source   string
	Warnings []string
}

// SourceInfo describes a connected sensor, for the status bar.
type SourceInfo struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // "sim" | "manual" | "phone" | "sensor" | "optical"
	Name   string `json:"name"`
	Wheel  string `json:"wheel,omitempty"`
	Detail string `json:"detail,omitempty"`

	LastSeen time.Time `json:"-"`
	AgeS     float64   `json:"age_s"`
	Online   bool      `json:"online"`
}

// ParamLive is one value on the live screen.
type ParamLive struct {
	Key string `json:"key"`

	Has    bool     `json:"has"`
	Value  float64  `json:"v"`            // degrees
	MM     *float64 `json:"mm,omitempty"` // toe only, at the configured rim diameter
	Stable bool     `json:"stable"`
	Stale  bool     `json:"stale,omitempty"`
	Source string   `json:"src,omitempty"`

	Status align.Status `json:"status"`
	Spec   *align.Range `json:"spec,omitempty"`
	// ToNominal is nominal minus value: how far to go, and which way, to
	// reach the middle of the band. Zero when there is no spec.
	ToNominal float64 `json:"to_nominal"`
	// Deviation is how far outside the band the value sits, 0 when inside.
	Deviation  float64 `json:"deviation"`
	Adjustable bool    `json:"adjustable"`
}

// Frame is one update of the live screen.
type Frame struct {
	Seq    uint64               `json:"seq"`
	Params map[string]ParamLive `json:"params"`

	// ThrustReferenced reports that front toe is measured from the rear axle's
	// thrust line, which needs both rear toes. Until they are known the front
	// toe is shown relative to the geometric centreline instead, and the
	// screen should say so.
	ThrustReferenced bool `json:"thrust_referenced"`

	Overall   align.Status `json:"overall"`
	OutOfSpec int          `json:"out_of_spec"`
	Measured  int          `json:"measured"`
	AllStable bool         `json:"all_stable"`
	HasLimits bool         `json:"has_limits"`

	Sources  []SourceInfo `json:"sources"`
	Warnings []string     `json:"warnings,omitempty"`
}

type sweepState struct {
	caster   float64
	sai      *float64
	source   string
	warnings []string
	at       time.Time
}

// Hub collects readings and publishes frames.
type Hub struct {
	mu      sync.Mutex
	cfg     Config
	camber  [4]channel
	toe     [4]channel
	sweep   [4]*sweepState
	sources map[string]*SourceInfo
	seq     uint64
	dirty   bool
	subs    map[chan []byte]struct{}
	now     func() time.Time
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{
		sources: map[string]*SourceInfo{},
		subs:    map[chan []byte]struct{}{},
		now:     time.Now,
	}
}

// SetClock replaces the clock, for tests.
func (h *Hub) SetClock(now func() time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = now
}

// Configure sets the vehicle and display parameters.
func (h *Hub) Configure(c Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg = c
	h.dirty = true
}

// Config returns the current configuration.
func (h *Hub) Config() Config {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg
}

// Plausibility limits for incoming readings. Anything outside these is a
// sensor fault or a unit mix-up, and letting it through would put a wheel on
// screen lying on its side.
const (
	maxAbsCamber = 15.0
	maxAbsToe    = 10.0
	maxAbsCaster = 20.0
)

// ErrImplausible is returned for readings no road wheel can produce.
var ErrImplausible = errors.New("live: implausible reading")

// Push records a sensor reading.
func (h *Hub) Push(in Input) error {
	if in.Wheel < align.FL || in.Wheel > align.RR {
		return fmt.Errorf("live: unknown wheel %d", in.Wheel)
	}
	if in.Camber != nil && !(math.Abs(*in.Camber) <= maxAbsCamber) {
		return fmt.Errorf("%w: camber %.2f°", ErrImplausible, *in.Camber)
	}
	if in.Toe != nil && !(math.Abs(*in.Toe) <= maxAbsToe) {
		return fmt.Errorf("%w: toe %.2f°", ErrImplausible, *in.Toe)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.now()
	if in.Camber != nil {
		h.camber[in.Wheel].push(*in.Camber, t, in.Instant, in.Source)
	}
	if in.Toe != nil {
		h.toe[in.Wheel].push(*in.Toe, t, in.Instant, in.Source)
	}
	if s, ok := h.sources[in.Source]; ok {
		s.LastSeen = t
	}
	h.dirty = true
	return nil
}

// PushSweep records a caster measurement on a steered wheel.
func (h *Hub) PushSweep(in SweepInput) error {
	if !in.Wheel.IsFront() {
		return errors.New("live: caster is measured on steered (front) wheels only")
	}
	if !(math.Abs(in.Caster) <= maxAbsCaster) {
		return fmt.Errorf("%w: caster %.2f°", ErrImplausible, in.Caster)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sweep[in.Wheel] = &sweepState{
		caster: in.Caster, sai: in.SAI, source: in.Source, warnings: in.Warnings, at: h.now(),
	}
	h.dirty = true
	return nil
}

// Touch registers a source or refreshes its heartbeat.
func (h *Hub) Touch(s SourceInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s.LastSeen = h.now()
	h.sources[s.ID] = &s
	h.dirty = true
}

// RemoveSource forgets a source. Its readings stay on screen, marked stale in
// due course, so a phone dropping off Wi-Fi does not blank the display.
func (h *Hub) RemoveSource(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sources, id)
	h.dirty = true
}

// Clear drops readings. With a source id it drops only that source's readings
// (a simulator being switched off); with "" it drops everything (a new car).
func (h *Hub) Clear(source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.camber {
		if source == "" || h.camber[i].source == source {
			h.camber[i].clear()
		}
		if source == "" || h.toe[i].source == source {
			h.toe[i].clear()
		}
		if s := h.sweep[i]; s != nil && (source == "" || s.source == source) {
			h.sweep[i] = nil
		}
	}
	h.dirty = true
}

// Frame builds the current frame.
func (h *Hub) Frame() Frame {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.frameLocked()
}

func (h *Hub) frameLocked() Frame {
	now := h.now()
	h.seq++
	f := Frame{Seq: h.seq, Params: map[string]ParamLive{}, HasLimits: h.cfg.Limits != nil}

	type val struct {
		has, stable, stale bool
		v                  float64
		src                string
	}
	chVal := func(c *channel, thr float64) val {
		if !c.has {
			return val{}
		}
		return val{has: true, v: c.value, stable: c.stable(now, thr), stale: c.stale(now), src: c.source}
	}
	both := func(a, b val) bool { return a.has && b.has }

	var cam, toeGeo [4]val
	for _, p := range align.AllPositions {
		cam[p] = chVal(&h.camber[p], stableCamber)
		toeGeo[p] = chVal(&h.toe[p], stableToe)
	}

	// The thrust line bisects the rear wheels' rolling directions; front toe is
	// referenced to it once it is known — exactly as align.Assemble does.
	var thrust val
	if both(toeGeo[align.RL], toeGeo[align.RR]) {
		thrust = val{
			has:    true,
			v:      (toeGeo[align.RL].v - toeGeo[align.RR].v) / 2,
			stable: toeGeo[align.RL].stable && toeGeo[align.RR].stable,
			stale:  toeGeo[align.RL].stale || toeGeo[align.RR].stale,
		}
		f.ThrustReferenced = true
	}
	var toe [4]val
	for _, p := range align.AllPositions {
		toe[p] = toeGeo[p]
		if p.IsFront() && toe[p].has && thrust.has {
			toe[p].v -= p.SideSign() * thrust.v
			toe[p].stable = toe[p].stable && thrust.stable
		}
	}

	rim := h.cfg.RimDiameterMM
	add := func(key string, x val, isToe bool) {
		p := ParamLive{Key: key, Has: x.has, Stable: x.stable, Stale: x.stale, Source: x.src}
		ps := specs.Lookup(h.cfg.Limits, key)
		p.Spec, p.Adjustable = ps.Spec, ps.Adjustable
		p.Status = align.StatusNoSpec
		if x.has {
			p.Value = round4(x.v)
			if isToe && rim > 0 {
				mm := round4(align.Deg(x.v).ToeMM(rim))
				p.MM = &mm
			}
			if p.Spec != nil {
				a := align.Deg(x.v)
				p.Status = p.Spec.Grade(a)
				p.Deviation = round4(p.Spec.Deviation(a).Deg())
				p.ToNominal = round4(p.Spec.Nominal.Deg() - x.v)
			}
		}
		f.Params[key] = p
	}

	for _, p := range align.AllPositions {
		w := p.String()
		add(specs.KeyCamber+w, cam[p], false)
		add(specs.KeyToe+w, toe[p], true)
		if p.IsFront() {
			var cs, sai val
			if s := h.sweep[p]; s != nil {
				cs = val{has: true, v: s.caster, stable: true, src: s.source}
				if s.sai != nil {
					sai = val{has: true, v: *s.sai, stable: true, src: s.source}
				}
			}
			add(specs.KeyCaster+w, cs, false)
			add(specs.KeySAI+w, sai, false)
		}
	}

	sum := func(a, b val, sign float64) val {
		if !both(a, b) {
			return val{}
		}
		return val{has: true, v: a.v + sign*b.v, stable: a.stable && b.stable, stale: a.stale || b.stale}
	}
	add(specs.KeyFrontTotalToe, sum(toe[align.FL], toe[align.FR], 1), true)
	add(specs.KeyRearTotalToe, sum(toe[align.RL], toe[align.RR], 1), true)
	add(specs.KeyFrontCrossCamber, sum(cam[align.FL], cam[align.FR], -1), false)
	add(specs.KeyRearCrossCamber, sum(cam[align.RL], cam[align.RR], -1), false)
	var cfl, cfr val
	if s := h.sweep[align.FL]; s != nil {
		cfl = val{has: true, v: s.caster, stable: true}
	}
	if s := h.sweep[align.FR]; s != nil {
		cfr = val{has: true, v: s.caster, stable: true}
	}
	add(specs.KeyFrontCrossCaster, sum(cfl, cfr, -1), false)
	add(specs.KeyThrustAngle, thrust, false)

	// Overall verdict over what has actually been measured.
	f.AllStable = true
	graded := 0
	f.Overall = align.StatusNoSpec
	for _, p := range f.Params {
		if !p.Has {
			continue
		}
		f.Measured++
		if !p.Stable {
			f.AllStable = false
		}
		switch p.Status {
		case align.StatusBad:
			f.OutOfSpec++
		case align.StatusNoSpec:
			continue
		}
		graded++
	}
	switch {
	case f.OutOfSpec > 0:
		f.Overall = align.StatusBad
	case graded > 0:
		f.Overall = align.StatusGood
		for _, p := range f.Params {
			if p.Has && p.Status == align.StatusMarginal {
				f.Overall = align.StatusMarginal
				break
			}
		}
	}
	if f.Measured == 0 {
		f.AllStable = false
	}

	for _, s := range h.sources {
		c := *s
		c.AgeS = round4(now.Sub(s.LastSeen).Seconds())
		// Typed-in and photographed readings are one-shot: they do not go
		// stale by falling silent, so their source never shows as offline.
		c.Online = now.Sub(s.LastSeen) <= staleAfter || s.Kind == "manual" || s.Kind == "optical"
		f.Sources = append(f.Sources, c)
	}
	sort.Slice(f.Sources, func(i, j int) bool { return f.Sources[i].ID < f.Sources[j].ID })

	for _, p := range align.AllPositions {
		if s := h.sweep[p]; s != nil {
			f.Warnings = append(f.Warnings, s.warnings...)
		}
	}
	return f
}

// Result assembles a complete alignment result from the current readings, for
// the printed report. It needs camber and toe on all four wheels.
func (h *Hub) Result() (align.Result, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()

	raw := make(map[align.Position]align.RawWheel, 4)
	var missing []string
	var unsettled []string
	for _, p := range align.AllPositions {
		c, t := &h.camber[p], &h.toe[p]
		if !c.has || !t.has {
			missing = append(missing, p.RussianName())
			continue
		}
		if !c.stable(now, stableCamber) || !t.stable(now, stableToe) {
			unsettled = append(unsettled, p.RussianName())
		}
		r := align.RawWheel{
			Camber:        align.Deg(c.value),
			ToeGeometric:  align.Deg(t.value),
			RimDiameterMM: h.cfg.RimDiameterMM,
			Quality:       align.Quality{Frames: len(c.window)},
		}
		if s := h.sweep[p]; s != nil {
			cs := align.Deg(s.caster)
			r.Caster = &cs
			if s.sai != nil {
				sai := align.Deg(*s.sai)
				r.SAI = &sai
			}
		}
		raw[p] = r
	}
	if len(missing) > 0 {
		return align.Result{}, fmt.Errorf("нет замеров развала и схождения: %v", missing)
	}
	g := align.Geometry{
		Known:        h.cfg.TrackFrontMM > 0 && h.cfg.TrackRearMM > 0,
		TrackFrontMM: h.cfg.TrackFrontMM,
		TrackRearMM:  h.cfg.TrackRearMM,
	}
	res := align.Assemble(raw, g, "live")
	if len(unsettled) > 0 {
		res.Warnings = append([]string{fmt.Sprintf(
			"Показания не успокоились на колёсах: %v. Снимок сделан, но перемерьте, когда стрелки встанут.", unsettled)},
			res.Warnings...)
	}
	for _, p := range align.AllPositions {
		if s := h.sweep[p]; s != nil {
			res.Warnings = append(res.Warnings, s.warnings...)
		}
	}
	return res, nil
}

// Subscribe returns a channel of encoded frames and a function to stop.
// Frames are dropped for a subscriber that falls behind: the next one
// supersedes it anyway.
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 2)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.dirty = true
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Run publishes frames to subscribers until ctx ends: promptly after any
// change, and at least once a second so that ages and staleness keep ticking.
func (h *Hub) Run(ctx context.Context) {
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		h.mu.Lock()
		now := h.now()
		if len(h.subs) == 0 || (!h.dirty && now.Sub(last) < time.Second) {
			h.mu.Unlock()
			continue
		}
		h.dirty = false
		last = now
		b, err := json.Marshal(h.frameLocked())
		if err == nil {
			for ch := range h.subs {
				select {
				case ch <- b:
				default:
				}
			}
		}
		h.mu.Unlock()
	}
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }
