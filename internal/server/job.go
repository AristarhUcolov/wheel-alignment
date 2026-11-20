package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/suspension"
)

// The job: the car on the stand and everything the person has entered for it —
// which car, its suspension and rim, the typed-in readings, the before and
// after snapshots, the preparation checklist, the plate number for the report.
//
// It is kept in session.json in the data directory and written on every
// change, so closing the program, a flat laptop battery or a restart the next
// morning loses nothing: the program opens where the person left off. Readings
// from phones and sensors are not kept — they stream, and come back by
// themselves; typed-in and photographed ones are, because nobody can stream
// them back.

// jobFile is the name of the job's file in the data directory.
const jobFile = "session.json"

// ReportMeta is what the report asks for that no sensor knows.
type ReportMeta struct {
	Plate string `json:"plate,omitempty"`
	Odo   string `json:"odo,omitempty"`
	Who   string `json:"who,omitempty"`
}

// savedSnapshot is a snapshot as kept on disk: the measurement only. The
// verdict is worked out again whenever the report is shown, against the
// tolerances and in the language in use then.
type savedSnapshot struct {
	Label  string       `json:"label"`
	Time   time.Time    `json:"time"`
	Result align.Result `json:"result"`
}

type savedJob struct {
	Version         int     `json:"version"`
	SpecID          string  `json:"spec_id,omitempty"`
	FrontSuspension string  `json:"front_suspension,omitempty"`
	RearSuspension  string  `json:"rear_suspension,omitempty"`
	RimDiameterIn   float64 `json:"rim_diameter_in,omitempty"`
	TrackFrontMM    float64 `json:"track_front_mm,omitempty"`
	TrackRearMM     float64 `json:"track_rear_mm,omitempty"`

	Before *savedSnapshot `json:"before,omitempty"`
	After  *savedSnapshot `json:"after,omitempty"`

	Manual  map[string]ManualLiveRequest `json:"manual,omitempty"`
	Optical *align.Result                `json:"optical,omitempty"`
	Box     map[string]string            `json:"box,omitempty"`
	Checks  map[string]bool              `json:"checks,omitempty"`
	Report  ReportMeta                   `json:"report"`
	Notes   map[string]string            `json:"notes,omitempty"`
}

func toSaved(sn *Snapshot) *savedSnapshot {
	if sn == nil {
		return nil
	}
	return &savedSnapshot{Label: sn.Label, Time: sn.Time, Result: sn.Result}
}

func fromSaved(sn *savedSnapshot) *Snapshot {
	if sn == nil || len(sn.Result.Wheels) == 0 {
		return nil
	}
	return &Snapshot{Label: sn.Label, Time: sn.Time, Result: sn.Result}
}

// saveJobLocked writes the job file. The caller holds s.sess.mu. A failure is
// logged, not returned: the measurement on screen matters more than its copy.
func (s *Server) saveJobLocked() {
	ss := s.sess
	if ss.path == "" {
		return
	}
	j := savedJob{
		Version: 1, SpecID: ss.specID,
		FrontSuspension: string(ss.frontSusp), RearSuspension: string(ss.rearSusp),
		RimDiameterIn: ss.rimIn, TrackFrontMM: ss.trackF, TrackRearMM: ss.trackR,
		Before: toSaved(ss.before), After: toSaved(ss.after),
		Manual: ss.manual, Optical: ss.optical, Box: ss.box, Checks: ss.checks, Report: ss.report,
		Notes: ss.notes,
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(ss.path), 0o755)
	}
	if err == nil {
		tmp := ss.path + ".tmp"
		if err = os.WriteFile(tmp, append(b, '\n'), 0o644); err == nil {
			err = os.Rename(tmp, ss.path)
		}
	}
	if err != nil {
		log.Printf("работа не сохранена: %v", err)
	}
}

// loadJob reads the job file kept in dir and puts it back on screen: the car,
// and the typed-in and photographed readings. A missing or damaged file starts
// an empty job — never a reason to refuse to start.
func (s *Server) loadJob(dir string) {
	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	ss := s.sess
	ss.path = ""
	if dir == "" {
		return
	}
	ss.path = filepath.Join(dir, jobFile)
	b, err := os.ReadFile(ss.path)
	if err != nil {
		return
	}
	var j savedJob
	if err := json.Unmarshal(b, &j); err != nil {
		log.Printf("файл работы повреждён, начинаю с чистого листа: %v", err)
		return
	}
	if _, ok := s.db.Get(j.SpecID); ok || j.SpecID == "" {
		ss.specID = j.SpecID
	}
	if suspension.Validate(suspension.Type(j.FrontSuspension), suspension.AxleFront) == nil {
		ss.frontSusp = suspension.Type(j.FrontSuspension)
	}
	if suspension.Validate(suspension.Type(j.RearSuspension), suspension.AxleRear) == nil {
		ss.rearSusp = suspension.Type(j.RearSuspension)
	}
	if j.RimDiameterIn >= 8 && j.RimDiameterIn <= 30 {
		ss.rimIn = j.RimDiameterIn
	}
	ss.trackF, ss.trackR = max(0, j.TrackFrontMM), max(0, j.TrackRearMM)
	ss.before, ss.after = fromSaved(j.Before), fromSaved(j.After)
	ss.box, ss.checks, ss.report, ss.notes = j.Box, j.Checks, j.Report, j.Notes
	ss.manual = map[string]ManualLiveRequest{}
	s.applySessionLocked()

	rimMM := align.Inches(ss.rimIn)
	for w, req := range j.Manual {
		pos, err := parseWheel(w)
		if err != nil {
			continue
		}
		req.Wheel = pos.String()
		ss.manual[pos.String()] = req
		_ = s.applyManual(pos, req, rimMM)
	}
	if j.Optical != nil && len(j.Optical.Wheels) > 0 {
		ss.optical = j.Optical
		s.pushResult(*j.Optical, opticalSource, opticalName)
	}
}

// opticalSource and opticalName are how the photo measurement appears on the
// live screen.
const opticalSource = "optical"

var opticalName = i18n.N("Камера и мишени")

// JobRequest changes the parts of the job only the interface reads: the
// preparation checklist, the string-line squareness inputs, the report's
// header, and notes — the steering-lock angles, the road test. Each field
// sent replaces the stored one.
type JobRequest struct {
	Checks *map[string]bool   `json:"checks"`
	Box    *map[string]string `json:"box"`
	Report *ReportMeta        `json:"report"`
	Notes  *map[string]string `json:"notes"`
}

// Limits on what the interface may store per job: a checklist of a few dozen
// items and a plate number fit many times over.
const (
	maxJobChecks = 400
	maxJobKey    = 200
	maxJobText   = 300
	maxJobNotes  = 32
)

func (s *Server) setJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%s: %w", i18n.T("не удалось прочитать запрос"), err))
		return
	}
	tooLong := errors.New(i18n.T("слишком длинное значение"))
	if req.Checks != nil {
		if len(*req.Checks) > maxJobChecks {
			writeErr(w, http.StatusRequestEntityTooLarge, tooLong)
			return
		}
		for k := range *req.Checks {
			if len(k) > maxJobKey {
				writeErr(w, http.StatusBadRequest, tooLong)
				return
			}
		}
	}
	if req.Box != nil {
		if len(*req.Box) > 8 {
			writeErr(w, http.StatusBadRequest, tooLong)
			return
		}
		for k, v := range *req.Box {
			if len(k) > 8 || len(v) > 40 {
				writeErr(w, http.StatusBadRequest, tooLong)
				return
			}
		}
	}
	if req.Notes != nil {
		if len(*req.Notes) > maxJobNotes {
			writeErr(w, http.StatusBadRequest, tooLong)
			return
		}
		for k, v := range *req.Notes {
			if len(k) > 40 || len(v) > maxJobText {
				writeErr(w, http.StatusBadRequest, tooLong)
				return
			}
		}
	}
	if m := req.Report; m != nil {
		for _, v := range []string{m.Plate, m.Odo, m.Who} {
			if len(v) > maxJobText {
				writeErr(w, http.StatusBadRequest, tooLong)
				return
			}
		}
	}

	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	ss := s.sess
	if req.Checks != nil {
		ss.checks = map[string]bool{}
		for k, v := range *req.Checks {
			if v {
				ss.checks[k] = true
			}
		}
	}
	if req.Box != nil {
		ss.box = map[string]string{}
		for k, v := range *req.Box {
			if v = strings.TrimSpace(v); v != "" {
				ss.box[k] = v
			}
		}
	}
	if req.Notes != nil {
		ss.notes = map[string]string{}
		for k, v := range *req.Notes {
			if v = strings.TrimSpace(v); v != "" {
				ss.notes[k] = v
			}
		}
	}
	if req.Report != nil {
		ss.report = ReportMeta{
			Plate: strings.TrimSpace(req.Report.Plate),
			Odo:   strings.TrimSpace(req.Report.Odo),
			Who:   strings.TrimSpace(req.Report.Who),
		}
	}
	s.saveJobLocked()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// newJob starts the next car: every reading and snapshot of the previous one
// goes, and so do its checklist, its report header and what the live camera
// learnt about its wheels. Camera and phone calibrations stay — they belong to
// the equipment, not to the car. The demonstration stops.
func (s *Server) newJob(w http.ResponseWriter, r *http.Request) {
	s.sim.Stop()
	s.hub.NewCar(manualSource, opticalSource)
	lo := s.optLive
	lo.mu.Lock()
	lo.resetLocked()
	lo.mu.Unlock()
	if s.phone != nil {
		s.phone.NewCar()
	}

	s.sess.mu.Lock()
	defer s.sess.mu.Unlock()
	ss := s.sess
	ss.specID, ss.frontSusp, ss.rearSusp = "", "", ""
	ss.rimIn, ss.trackF, ss.trackR = 15, 0, 0
	ss.before, ss.after = nil, nil
	ss.manual, ss.optical, ss.box, ss.checks = map[string]ManualLiveRequest{}, nil, nil, nil
	ss.report, ss.notes = ReportMeta{}, nil
	s.saveJobLocked()
	writeJSON(w, http.StatusOK, s.applySessionLocked())
}

// rememberManual keeps a wheel's typed-in readings as the form shows them. A
// request with Replace is the whole form; one without (a photo's camber sent
// to the screen, a script) changes only what it carries. The caller holds
// s.sess.mu.
func (s *Server) rememberManual(pos align.Position, req ManualLiveRequest) {
	ss := s.sess
	if ss.manual == nil {
		ss.manual = map[string]ManualLiveRequest{}
	}
	req.Wheel = pos.String()
	if req.Sweep != nil {
		out, in, half := req.Sweep.CamberOut, req.Sweep.CamberIn, req.Sweep.HalfSweepDeg
		req.SweepOut, req.SweepIn, req.HalfSweep = &out, &in, &half
		req.Sweep = nil
	}
	if !req.Replace {
		old := ss.manual[pos.String()]
		if req.Camber != nil {
			old.Camber, old.Camber180, old.Invert = req.Camber, req.Camber180, req.Invert
		}
		if req.ToeFrontMM != nil && req.ToeRearMM != nil {
			old.ToeFrontMM, old.ToeRearMM, old.ToeSpanMM, old.StringInside = req.ToeFrontMM, req.ToeRearMM, req.ToeSpanMM, req.StringInside
		}
		if req.SweepOut != nil && req.SweepIn != nil {
			old.SweepOut, old.SweepIn, old.HalfSweep = req.SweepOut, req.SweepIn, req.HalfSweep
		}
		req = old
		req.Wheel = pos.String()
	}
	req.Replace = false
	if req.empty() {
		delete(ss.manual, pos.String())
		return
	}
	ss.manual[pos.String()] = req
}

func (m ManualLiveRequest) empty() bool {
	return m.Camber == nil && m.Camber180 == nil && m.ToeFrontMM == nil && m.ToeRearMM == nil &&
		m.SweepOut == nil && m.SweepIn == nil && m.Sweep == nil && !m.Invert
}

// applyManual puts one wheel's typed-in readings on the live screen: what can
// be worked out from the fields filled in is shown, and with Replace whatever
// can no longer be worked out leaves the screen.
func (s *Server) applyManual(pos align.Position, req ManualLiveRequest, rimMM float64) error {
	// Each quantity on its own: a mistyped camber must not keep the toe of
	// the same wheel off the screen. A refused number leaves the screen too —
	// the old one is no longer what the form says.
	var clear live.What
	var firstErr error
	fail := func(err error, what live.What) {
		if req.Replace {
			clear |= what
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	push := func(in live.Input, what live.What) {
		in.Wheel, in.Instant, in.Source = pos, true, manualSource
		if err := s.hub.Push(in); err != nil {
			fail(err, what)
		}
	}
	s.hub.Touch(live.SourceInfo{ID: manualSource, Kind: "manual", Name: i18n.N("Ручной ввод"), Detail: i18n.N("струна и угломер")})

	var camber, toe align.Angle
	if req.Camber != nil {
		inc := inclinometer(*req.Camber, req.Camber180, req.Invert)
		camber, _ = inc.Camber()
		v := camber.Deg()
		push(live.Input{Camber: &v}, live.WhatCamber)
	} else if req.Replace {
		clear |= live.WhatCamber
	}
	if req.ToeFrontMM != nil && req.ToeRearMM != nil {
		var err error
		if toe, err = stringToe(*req.ToeFrontMM, *req.ToeRearMM, req.ToeSpanMM, req.StringInside, rimMM); err != nil {
			fail(err, live.WhatToe)
		} else {
			v := toe.Deg()
			push(live.Input{Toe: &v}, live.WhatToe)
		}
	} else if req.Replace {
		clear |= live.WhatToe
	}

	sweep := req.Sweep
	if sweep == nil && req.SweepOut != nil && req.SweepIn != nil {
		half := 0.0
		if req.HalfSweep != nil {
			half = *req.HalfSweep
		}
		sweep = &SweepInput{CamberOut: *req.SweepOut, CamberIn: *req.SweepIn, HalfSweepDeg: half}
	}
	switch {
	case sweep != nil && !pos.IsFront():
		fail(errors.New(i18n.T("кастер меряется только на передних колёсах")), live.WhatCaster)
	case sweep != nil:
		half := sweep.HalfSweepDeg
		if half == 0 {
			half = 20
		}
		sw := measureSweep(sweep.CamberOut, sweep.CamberIn, half)
		if req.Camber != nil {
			sw.CamberStraight, sw.HasStraight = camber, true
			sw.ToeStraight = toe
		}
		sol, err := sw.Solve(pos)
		if err != nil {
			fail(err, live.WhatCaster)
			break
		}
		si := live.SweepInput{Wheel: pos, Caster: sol.Caster.Deg(), Source: manualSource, Warnings: sol.Warnings}
		if sol.SAI != nil {
			v := sol.SAI.Deg()
			si.SAI = &v
		}
		if err := s.hub.PushSweep(si); err != nil {
			fail(err, live.WhatCaster)
		}
	case req.Replace:
		clear |= live.WhatCaster
	}
	if clear != 0 {
		s.hub.ClearWheel(manualSource, pos, clear)
	}
	return firstErr
}

// reapplyManual puts every wheel's typed-in readings on screen again — after
// the rim diameter changes, string-line millimetres mean a different angle.
// The caller holds s.sess.mu.
func (s *Server) reapplyManual() {
	rimMM := align.Inches(s.sess.rimIn)
	for w, req := range s.sess.manual {
		if pos, err := parseWheel(w); err == nil {
			_ = s.applyManual(pos, req, rimMM)
		}
	}
}
