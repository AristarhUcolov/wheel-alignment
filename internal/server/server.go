// Package server exposes the alignment engine over HTTP and serves the web
// interface.
//
// The whole program is one binary with the interface embedded in it: no
// installer, no runtime, no internet connection. That matters for the people
// this is for — it has to run on whatever elderly laptop is in the garage, and
// it has to keep running when the project's website eventually goes away.
package server

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/simulate"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

//go:embed web
var webFS embed.FS

// Server wires the engine to HTTP.
type Server struct {
	db  *specs.DB
	mux *http.ServeMux
}

// New builds the server and its routes.
func New(db *specs.DB) (*Server, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	s := &Server{db: db, mux: http.NewServeMux()}

	s.mux.Handle("GET /", http.FileServerFS(sub))
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/specs/search", s.searchSpecs)
	s.mux.HandleFunc("GET /api/specs/{id}", s.getSpec)
	s.mux.HandleFunc("POST /api/measure/manual", s.measureManual)
	s.mux.HandleFunc("POST /api/optical/calibrate", s.calibrate)
	s.mux.HandleFunc("POST /api/optical/camber", s.opticalCamber)
	s.mux.HandleFunc("POST /api/optical/align", s.opticalAlign)
	s.mux.HandleFunc("POST /api/specs/check", s.checkSpec)
	s.mux.HandleFunc("GET /api/demo", s.demo)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"specs":       s.db.Count(),
		"load_errors": s.db.LoadErrors,
	})
}

// specSummary is the light form used in search results.
type specSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Make        string `json:"make"`
	Model       string `json:"model"`
	SourceKind  string `json:"source_kind"`
	SourceLabel string `json:"source_label"`
	Verified    bool   `json:"verified"`
	Disclaimer  string `json:"disclaimer,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func summarise(sp specs.Spec) specSummary {
	return specSummary{
		ID: sp.ID, Title: sp.Title(), Make: sp.Make, Model: sp.Model,
		SourceKind: string(sp.Source.Kind), SourceLabel: sp.Source.Kind.RussianName(),
		Verified: sp.Verified(), Disclaimer: sp.Disclaimer(), Notes: sp.Notes,
	}
}

func (s *Server) searchSpecs(w http.ResponseWriter, r *http.Request) {
	q := specs.Query{
		Text:            r.URL.Query().Get("q"),
		Make:            r.URL.Query().Get("make"),
		IncludeGuidance: r.URL.Query().Get("guidance") == "1",
	}
	if y := r.URL.Query().Get("year"); y != "" {
		q.Year, _ = strconv.Atoi(y)
	}

	matches := s.db.Search(q)
	out := make([]specSummary, 0, len(matches))
	for i, m := range matches {
		if i >= 50 {
			break
		}
		out = append(out, summarise(m.Spec))
	}

	// Always offer the class-based fallbacks alongside, clearly separated, so
	// that "my car isn't here" is never a dead end.
	guidance := make([]specSummary, 0, 4)
	if !q.IncludeGuidance {
		for _, m := range s.db.Search(specs.Query{IncludeGuidance: true}) {
			if m.Spec.Source.Kind == specs.SourceClassGuidance {
				guidance = append(guidance, summarise(m.Spec))
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "guidance": guidance})
}

func (s *Server) getSpec(w http.ResponseWriter, r *http.Request) {
	sp, ok := s.db.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, errors.New("автомобиль не найден в базе"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"spec":       sp,
		"title":      sp.Title(),
		"verified":   sp.Verified(),
		"disclaimer": sp.Disclaimer(),
		"source":     sp.Source.Kind.RussianName(),
	})
}

// ---------------------------------------------------------------------------
// Manual measurement
// ---------------------------------------------------------------------------

// SweepInput is one wheel's caster sweep, in degrees.
type SweepInput struct {
	CamberOut    float64 `json:"camber_out"`
	CamberIn     float64 `json:"camber_in"`
	HalfSweepDeg float64 `json:"half_sweep_deg"`
}

// WheelInput is one corner as measured by hand.
type WheelInput struct {
	// Camber gauge readings in degrees, positive = top of the wheel outboard.
	Camber0   float64 `json:"camber_0"`
	Camber180 float64 `json:"camber_180"`
	Has180    bool    `json:"has_180"`
	Invert    bool    `json:"invert_gauge"`

	// String-line readings in millimetres.
	ToeFrontMM   float64 `json:"toe_front_mm"`
	ToeRearMM    float64 `json:"toe_rear_mm"`
	ToeSpanMM    float64 `json:"toe_span_mm"`
	StringInside bool    `json:"string_inside"`

	Sweep *SweepInput `json:"sweep,omitempty"`
}

// BoxInput is the string-line setup, for the squareness check.
type BoxInput struct {
	LeftFrontMM  float64 `json:"left_front_mm"`
	LeftRearMM   float64 `json:"left_rear_mm"`
	RightFrontMM float64 `json:"right_front_mm"`
	RightRearMM  float64 `json:"right_rear_mm"`
}

// ManualRequest is a complete four-corner manual measurement.
type ManualRequest struct {
	SpecID        string                `json:"spec_id"`
	RimDiameterIn float64               `json:"rim_diameter_in"`
	TrackFrontMM  float64               `json:"track_front_mm"`
	TrackRearMM   float64               `json:"track_rear_mm"`
	Box           *BoxInput             `json:"box,omitempty"`
	Wheels        map[string]WheelInput `json:"wheels"`
}

// MeasureResponse pairs the raw angles with the verdict against the spec.
type MeasureResponse struct {
	Result align.Result `json:"result"`
	Report specs.Report `json:"report"`
}

func (s *Server) measureManual(w http.ResponseWriter, r *http.Request) {
	var req ManualRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать данные замера: %w", err))
		return
	}

	var spec *specs.Spec
	if req.SpecID != "" {
		if sp, ok := s.db.Get(req.SpecID); ok {
			spec = &sp
			if req.RimDiameterIn <= 0 {
				req.RimDiameterIn = sp.RimDiameterIn
			}
			if req.TrackFrontMM <= 0 {
				req.TrackFrontMM = 0 // unknown; the box check will say so
			}
		}
	}
	rimMM := align.Inches(req.RimDiameterIn)
	if rimMM <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New(
			"не указан диаметр обода: без него схождение в миллиметрах невозможно перевести в угол"))
		return
	}

	sess := measure.ManualSession{
		Wheels:       map[align.Position]measure.ManualWheel{},
		TrackFrontMM: req.TrackFrontMM,
		TrackRearMM:  req.TrackRearMM,
	}
	if req.Box != nil {
		sess.Box = &measure.StringBox{
			LeftFrontMM: req.Box.LeftFrontMM, LeftRearMM: req.Box.LeftRearMM,
			RightFrontMM: req.Box.RightFrontMM, RightRearMM: req.Box.RightRearMM,
			TrackFrontMM: req.TrackFrontMM, TrackRearMM: req.TrackRearMM,
		}
	}

	for _, p := range align.AllPositions {
		in, ok := req.Wheels[p.String()]
		if !ok {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("нет данных по колесу %s (%s)", p, p.RussianName()))
			return
		}
		side := measure.LineOutside
		if in.StringInside {
			side = measure.LineInside
		}
		span := in.ToeSpanMM
		if span <= 0 {
			span = rimMM // readings taken on the rim edges
		}
		mw := measure.ManualWheel{
			Camber: measure.Inclinometer{
				At0Deg: in.Camber0, At180Deg: in.Camber180, Has180: in.Has180, Invert: in.Invert,
			},
			Toe: measure.StringToe{
				FrontMM: in.ToeFrontMM, RearMM: in.ToeRearMM, SpanMM: span, Side: side,
			},
			RimDiameterMM: rimMM,
		}
		if in.Sweep != nil && p.IsFront() {
			half := in.Sweep.HalfSweepDeg
			if half == 0 {
				half = 20
			}
			mw.Sweep = &measure.SweepReading{
				CamberOut: align.Deg(in.Sweep.CamberOut),
				CamberIn:  align.Deg(in.Sweep.CamberIn),
				HalfSweep: align.Deg(half),
			}
		}
		sess.Wheels[p] = mw
	}

	res, err := sess.Result()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, MeasureResponse{Result: res, Report: specs.Compare(res, spec)})
}

// demo runs the forward model through the whole pipeline so the interface can
// be explored — and understood — without a car on the floor.
func (s *Server) demo(w http.ResponseWriter, r *http.Request) {
	res, err := align.Compute(simulate.Nominal().WheelSet(), align.FrameOptions{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	id := r.URL.Query().Get("spec")
	if id == "" {
		id = "guidance-fwd-mcpherson"
	}
	var spec *specs.Spec
	if sp, ok := s.db.Get(id); ok {
		spec = &sp
	}
	writeJSON(w, http.StatusOK, MeasureResponse{Result: res, Report: specs.Compare(res, spec)})
}

// LoadErrorSummary renders any database problems for display at startup.
func LoadErrorSummary(db *specs.DB) string {
	if len(db.LoadErrors) == 0 {
		return ""
	}
	return "Проблемы в базе данных автомобилей:\n  - " + strings.Join(db.LoadErrors, "\n  - ")
}
