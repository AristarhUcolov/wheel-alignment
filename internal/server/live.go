package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
	"github.com/AristarhUcolov/wheel-alignment/internal/suspension"
)

// The live side of the HTTP API: the session (which car, which suspension,
// which rim), the stream of frames for the adjustment screen, and the ways
// readings get in — typed by hand, sent by a sensor, or simulated.

// session is the car on the stand right now.
type session struct {
	mu sync.Mutex

	specID    string
	frontSusp suspension.Type
	rearSusp  suspension.Type
	rimIn     float64
	trackF    float64
	trackR    float64

	before *Snapshot
	after  *Snapshot
}

// Snapshot is the state of the car at one moment, for the before/after report.
type Snapshot struct {
	Label  string       `json:"label"`
	Time   time.Time    `json:"time"`
	Result align.Result `json:"result"`
	Report specs.Report `json:"report"`
}

// SessionRequest changes the session. Zero values leave a field as it is,
// except that choosing a new vehicle resets suspension and rim to the new
// vehicle's own.
type SessionRequest struct {
	SpecID          *string  `json:"spec_id"`
	FrontSuspension *string  `json:"front_suspension"`
	RearSuspension  *string  `json:"rear_suspension"`
	RimDiameterIn   *float64 `json:"rim_diameter_in"`
	TrackFrontMM    *float64 `json:"track_front_mm"`
	TrackRearMM     *float64 `json:"track_rear_mm"`
}

// VehicleView is a spec as the interface needs it.
type VehicleView struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Make        string `json:"make"`
	Model       string `json:"model"`
	Class       string `json:"class,omitempty"`
	ClassName   string `json:"class_name,omitempty"`
	SourceKind  string `json:"source_kind"`
	SourceLabel string `json:"source_label"`
	SourceRef   string `json:"source_reference,omitempty"`
	Verified    bool   `json:"verified"`
	Disclaimer  string `json:"disclaimer,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Local       bool   `json:"local,omitempty"`
	HasFigures  bool   `json:"has_figures"`
	YearFrom    int    `json:"year_from,omitempty"`
	YearTo      int    `json:"year_to,omitempty"`
}

// SessionView is the whole session as the interface needs it.
type SessionView struct {
	Vehicle *VehicleView `json:"vehicle"`
	// Limits is the entry whose tolerances apply — the vehicle itself, or for
	// a catalog entry the class guidance it names. Nil when nothing applies.
	Limits *VehicleView `json:"limits"`
	Spec   *specs.Spec  `json:"spec,omitempty"`

	FrontSuspension suspension.Info `json:"front_suspension"`
	RearSuspension  suspension.Info `json:"rear_suspension"`

	RimDiameterIn float64 `json:"rim_diameter_in"`
	TrackFrontMM  float64 `json:"track_front_mm"`
	TrackRearMM   float64 `json:"track_rear_mm"`
	Conditions    string  `json:"conditions,omitempty"`

	// Params describes every parameter on the live screen: label, tolerance,
	// whether and how it is adjusted on this car.
	Params map[string]specs.ParamSpec `json:"params"`

	SimRunning bool `json:"sim_running"`
	SimAuto    bool `json:"sim_auto"`
	HasBefore  bool `json:"has_before"`
	HasAfter   bool `json:"has_after"`
}

func (s *Server) vehicleView(sp specs.Spec) *VehicleView {
	v := &VehicleView{
		ID: sp.ID, Title: sp.Title(), Make: sp.Make, Model: sp.Model,
		Class: string(sp.Class), SourceKind: string(sp.Source.Kind),
		SourceLabel: sp.Source.Kind.RussianName(), SourceRef: sp.Source.Reference,
		Verified: sp.Verified(), Disclaimer: sp.Disclaimer(), Notes: sp.Notes,
		Local: s.db.IsLocal(sp.ID), HasFigures: sp.HasFigures(),
		YearFrom: sp.YearFrom, YearTo: sp.YearTo,
	}
	if sp.Class != "" {
		v.ClassName = sp.Class.RussianName()
	}
	return v
}

// liveKeys lists every parameter the live screen shows.
func liveKeys() []string {
	var keys []string
	for _, p := range align.AllPositions {
		keys = append(keys, specs.KeyCamber+p.String(), specs.KeyToe+p.String())
		if p.IsFront() {
			keys = append(keys, specs.KeyCaster+p.String(), specs.KeySAI+p.String())
		}
	}
	return append(keys, specs.KeyFrontTotalToe, specs.KeyRearTotalToe,
		specs.KeyFrontCrossCamber, specs.KeyRearCrossCamber, specs.KeyFrontCrossCaster, specs.KeyThrustAngle)
}

// applySession pushes the session into the hub and returns its view. The
// caller holds s.sess.mu.
func (s *Server) applySessionLocked() SessionView {
	ss := s.sess
	view := SessionView{
		RimDiameterIn: ss.rimIn, TrackFrontMM: ss.trackF, TrackRearMM: ss.trackR,
		FrontSuspension: suspension.MustGet(ss.frontSusp),
		RearSuspension:  suspension.MustGet(ss.rearSusp),
		Params:          map[string]specs.ParamSpec{},
		SimRunning:      s.sim.Running(), SimAuto: s.sim.Auto(),
		HasBefore: ss.before != nil, HasAfter: ss.after != nil,
	}
	cfg := live.Config{
		FrontSuspension: ss.frontSusp, RearSuspension: ss.rearSusp,
		RimDiameterMM: align.Inches(ss.rimIn), TrackFrontMM: ss.trackF, TrackRearMM: ss.trackR,
	}
	if sp, ok := s.db.Get(ss.specID); ok {
		view.Vehicle = s.vehicleView(sp)
		cfg.Vehicle = &sp
		if lim, ok := s.db.Limits(sp); ok {
			view.Limits = s.vehicleView(lim)
			view.Spec = &lim
			view.Conditions = specs.DescribeConditions(lim.Conditions)
			if c := specs.DescribeConditions(sp.Conditions); c != "" {
				view.Conditions = c
			}
			cfg.Limits = &lim
		}
	}
	for _, k := range liveKeys() {
		view.Params[k] = specs.Lookup(cfg.Limits, k)
	}
	s.hub.Configure(cfg)
	return view
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	writeJSON(w, http.StatusOK, s.applySessionLocked())
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request) {
	var req SessionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать запрос: %w", err))
		return
	}
	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	ss := s.sess

	if req.SpecID != nil && *req.SpecID != ss.specID {
		id := *req.SpecID
		if id != "" {
			sp, ok := s.db.Get(id)
			if !ok {
				writeErr(w, http.StatusNotFound, errors.New("автомобиль не найден в базе"))
				return
			}
			ss.frontSusp, ss.rearSusp = sp.FrontSuspension, sp.RearSuspension
			if sp.RimDiameterIn > 0 {
				ss.rimIn = sp.RimDiameterIn
			}
		}
		ss.specID = id
		// A different car: its old snapshots mean nothing any more.
		ss.before, ss.after = nil, nil
	}
	if req.FrontSuspension != nil {
		t := suspension.Type(*req.FrontSuspension)
		if err := suspension.Validate(t, suspension.AxleFront); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		ss.frontSusp = t
	}
	if req.RearSuspension != nil {
		t := suspension.Type(*req.RearSuspension)
		if err := suspension.Validate(t, suspension.AxleRear); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		ss.rearSusp = t
	}
	if req.RimDiameterIn != nil {
		if v := *req.RimDiameterIn; v < 8 || v > 30 {
			writeErr(w, http.StatusBadRequest, errors.New("диаметр обода должен быть от 8 до 30 дюймов"))
			return
		}
		ss.rimIn = *req.RimDiameterIn
	}
	if req.TrackFrontMM != nil {
		ss.trackF = *req.TrackFrontMM
	}
	if req.TrackRearMM != nil {
		ss.trackR = *req.TrackRearMM
	}
	writeJSON(w, http.StatusOK, s.applySessionLocked())
}

// liveStream serves frames as Server-Sent Events. SSE rather than WebSockets:
// it is one-way, which is all the screen needs, it reconnects by itself, and it
// needs nothing beyond the standard library.
func (s *Server) liveStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, errors.New("потоковая передача не поддерживается"))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, stop := s.hub.Subscribe()
	defer stop()

	if b, err := json.Marshal(s.hub.Frame()); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

func (s *Server) liveFrame(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hub.Frame())
}

// ManualLiveRequest is one wheel measured by hand during adjustment: a fresh
// gauge reading, a fresh pair of string-line distances, or both.
type ManualLiveRequest struct {
	Wheel string `json:"wheel"`

	Camber    *float64 `json:"camber"`     // gauge reading, degrees
	Camber180 *float64 `json:"camber_180"` // same spot after half a wheel turn
	Invert    bool     `json:"invert_gauge"`

	ToeFrontMM   *float64 `json:"toe_front_mm"`
	ToeRearMM    *float64 `json:"toe_rear_mm"`
	ToeSpanMM    float64  `json:"toe_span_mm"`
	StringInside bool     `json:"string_inside"`

	Sweep *SweepInput `json:"sweep,omitempty"`
}

func parseWheel(s string) (align.Position, error) {
	for _, p := range align.AllPositions {
		if strings.EqualFold(s, p.String()) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("неизвестное колесо %q (нужно FL, FR, RL или RR)", s)
}

// manualSource is the source id for typed-in readings.
const manualSource = "manual"

func (s *Server) liveManual(w http.ResponseWriter, r *http.Request) {
	var req ManualLiveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать замер: %w", err))
		return
	}
	pos, err := parseWheel(req.Wheel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.sess.mu.Lock()
	rimMM := align.Inches(s.sess.rimIn)
	s.sess.mu.Unlock()

	in := live.Input{Wheel: pos, Instant: true, Source: manualSource}
	var camber align.Angle
	if req.Camber != nil {
		inc := measure.Inclinometer{At0Deg: *req.Camber, Invert: req.Invert}
		if req.Camber180 != nil {
			inc.At180Deg, inc.Has180 = *req.Camber180, true
		}
		camber, _ = inc.Camber()
		v := camber.Deg()
		in.Camber = &v
	}
	var toe align.Angle
	if req.ToeFrontMM != nil && req.ToeRearMM != nil {
		span := req.ToeSpanMM
		if span <= 0 {
			span = rimMM
		}
		side := measure.LineOutside
		if req.StringInside {
			side = measure.LineInside
		}
		toe, err = measure.StringToe{FrontMM: *req.ToeFrontMM, RearMM: *req.ToeRearMM, SpanMM: span, Side: side}.Toe()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		v := toe.Deg()
		in.Toe = &v
	}
	s.hub.Touch(live.SourceInfo{ID: manualSource, Kind: "manual", Name: "Ручной ввод", Detail: "струна и угломер"})
	if in.Camber != nil || in.Toe != nil {
		if err := s.hub.Push(in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}

	if req.Sweep != nil {
		if !pos.IsFront() {
			writeErr(w, http.StatusBadRequest, errors.New("кастер меряется только на передних колёсах"))
			return
		}
		half := req.Sweep.HalfSweepDeg
		if half == 0 {
			half = 20
		}
		sw := measure.SweepReading{
			CamberOut: align.Deg(req.Sweep.CamberOut), CamberIn: align.Deg(req.Sweep.CamberIn),
			HalfSweep: align.Deg(half),
		}
		if req.Camber != nil {
			sw.CamberStraight, sw.HasStraight = camber, true
			sw.ToeStraight = toe
		}
		sol, err := sw.Solve(pos)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		si := live.SweepInput{Wheel: pos, Caster: sol.Caster.Deg(), Source: manualSource, Warnings: sol.Warnings}
		if sol.SAI != nil {
			v := sol.SAI.Deg()
			si.SAI = &v
		}
		if err := s.hub.PushSweep(si); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, s.hub.Frame())
}

// SensorSample is the open protocol for sensor heads — an ESP32 with an
// inclinometer, a laser toe gauge with a serial port, anything. One JSON object
// per reading, or an array of them, POSTed to /api/live/sample. Angles in
// degrees with the program's sign conventions: camber positive with the top of
// the wheel outboard, toe positive toe-in.
type SensorSample struct {
	Source string   `json:"source"`
	Name   string   `json:"name,omitempty"`
	Wheel  string   `json:"wheel"`
	Camber *float64 `json:"camber,omitempty"`
	Toe    *float64 `json:"toe,omitempty"`
	Caster *float64 `json:"caster,omitempty"`
}

func (s *Server) liveSample(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, 1<<18)
	var raw json.RawMessage
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var batch []SensorSample
	if t := strings.TrimSpace(string(raw)); strings.HasPrefix(t, "[") {
		if err := json.Unmarshal(raw, &batch); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	} else {
		var one SensorSample
		if err := json.Unmarshal(raw, &one); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		batch = []SensorSample{one}
	}
	for _, smp := range batch {
		id := strings.TrimSpace(smp.Source)
		if id == "" {
			writeErr(w, http.StatusBadRequest, errors.New("поле source обязательно: по нему датчик виден в строке состояния"))
			return
		}
		pos, err := parseWheel(smp.Wheel)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		name := smp.Name
		if name == "" {
			name = "Датчик " + id
		}
		s.hub.Touch(live.SourceInfo{ID: "sensor:" + id, Kind: "sensor", Name: name, Wheel: pos.String()})
		if err := s.hub.Push(live.Input{Wheel: pos, Camber: smp.Camber, Toe: smp.Toe, Source: "sensor:" + id}); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if smp.Caster != nil {
			if err := s.hub.PushSweep(live.SweepInput{Wheel: pos, Caster: *smp.Caster, Source: "sensor:" + id}); err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "accepted": len(batch)})
}

// pushResult puts a complete one-shot measurement on the live screen: every
// wheel's camber and geometric toe, and caster where it was measured.
func (s *Server) pushResult(res align.Result, source, name string) {
	s.hub.Touch(live.SourceInfo{ID: source, Kind: source, Name: name})
	for _, p := range align.AllPositions {
		w, ok := res.Wheels[p.String()]
		if !ok {
			continue
		}
		c, t := w.Camber.Deg(), w.ToeGeometric.Deg()
		_ = s.hub.Push(live.Input{Wheel: p, Camber: &c, Toe: &t, Instant: true, Source: source})
		if w.Caster != nil && p.IsFront() {
			si := live.SweepInput{Wheel: p, Caster: w.Caster.Deg(), Source: source}
			if w.SAI != nil {
				v := w.SAI.Deg()
				si.SAI = &v
			}
			_ = s.hub.PushSweep(si)
		}
	}
}

func (s *Server) liveClear(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source string `json:"source"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req)
	s.hub.Clear(req.Source)
	writeJSON(w, http.StatusOK, s.hub.Frame())
}

// SimRequest drives the demonstration car.
type SimRequest struct {
	Action string  `json:"action"` // start | stop | restart | auto_on | auto_off | adjust
	Key    string  `json:"key,omitempty"`
	Delta  float64 `json:"delta,omitempty"`
}

func (s *Server) simControl(w http.ResponseWriter, r *http.Request) {
	var req SimRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	switch req.Action {
	case "start":
		s.sim.Start()
	case "stop":
		s.sim.Stop()
	case "restart":
		s.sim.Restart()
		s.sim.Start()
	case "auto_on":
		s.sim.Start()
		s.sim.SetAuto(true)
	case "auto_off":
		s.sim.SetAuto(false)
	case "adjust":
		if req.Delta == 0 || req.Delta > 2 || req.Delta < -2 {
			writeErr(w, http.StatusBadRequest, errors.New("шаг регулировки должен быть от −2° до +2°"))
			return
		}
		if err := s.sim.Adjust(req.Key, req.Delta); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, fmt.Errorf("неизвестное действие %q", req.Action))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"running": s.sim.Running(), "auto": s.sim.Auto()})
}

// liveSnapshot freezes the current readings as the "before" or "after" state.
func (s *Server) liveSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Label != "before" && req.Label != "after" {
		writeErr(w, http.StatusBadRequest, errors.New("снимок бывает «before» или «after»"))
		return
	}
	res, err := s.hub.Result()
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	cfg := s.hub.Config()
	snap := &Snapshot{Label: req.Label, Time: time.Now(), Result: res, Report: specs.Compare(res, cfg.Limits)}

	s.sess.mu.Lock()
	if req.Label == "before" {
		s.sess.before = snap
	} else {
		s.sess.after = snap
	}
	s.sess.mu.Unlock()
	writeJSON(w, http.StatusOK, snap)
}

// ReportView is everything the report page shows.
type ReportView struct {
	Session SessionView `json:"session"`
	Before  *Snapshot   `json:"before,omitempty"`
	After   *Snapshot   `json:"after,omitempty"`
}

func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	writeJSON(w, http.StatusOK, ReportView{Session: s.applySessionLocked(), Before: s.sess.before, After: s.sess.after})
}

func (s *Server) suspensions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"front": suspension.For(suspension.AxleFront),
		"rear":  suspension.For(suspension.AxleRear),
		"all":   suspension.All(),
	})
}
