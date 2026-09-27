package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

// Checking a contributed vehicle specification.
//
// The database is the project's real bottleneck: the maths is finished, and what
// is missing is factory tolerances that cannot be invented. The obstacle to
// people supplying them was never willingness — it was that contributing meant
// knowing the JSON layout, the sign conventions and the Go toolchain.
//
// This endpoint removes that. It takes a candidate entry, runs it through the
// very same Validate and Resolve the program uses at startup, adds the
// plausibility checks, and hands back the canonical file ready to be dropped
// into internal/specs/data/ — plus, importantly, the millimetre figures resolved
// into degrees, so the contributor can see what their numbers actually mean
// before anyone adjusts a car to them.

// SpecCheckResponse is the verdict on a candidate entry.
type SpecCheckResponse struct {
	OK bool `json:"ok"`

	// Errors are structural faults: the entry cannot be loaded at all.
	Errors []string `json:"errors,omitempty"`
	// Warnings are advisory — possible but surprising figures.
	Warnings []string `json:"warnings,omitempty"`

	// Title and Provenance echo how the entry will present itself, so a
	// contributor sees the disclaimer their chosen source earns.
	Title      string `json:"title,omitempty"`
	SourceKind string `json:"source_kind,omitempty"`
	Verified   bool   `json:"verified"`
	Disclaimer string `json:"disclaimer,omitempty"`

	// Resolved shows the derived angles — individual toe from total, and
	// degrees from millimetres — as the program will actually use them.
	Resolved []ResolvedFigure `json:"resolved,omitempty"`

	// File is the canonical JSON, ready to save as a contribution.
	File string `json:"file,omitempty"`
}

// ResolvedFigure is one derived value, shown so the contributor can check it.
type ResolvedFigure struct {
	Axle   string `json:"axle"`
	Name   string `json:"name"`
	Range  string `json:"range"`
	Detail string `json:"detail,omitempty"`
}

// checkSpec validates a candidate vehicle entry.
func (s *Server) checkSpec(w http.ResponseWriter, r *http.Request) {
	var spec specs.Spec
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&spec); err != nil {
		writeJSON(w, http.StatusOK, SpecCheckResponse{
			Errors: []string{i18n.F("Не удалось разобрать данные: %s", err.Error())},
		})
		return
	}

	var resp SpecCheckResponse
	if err := spec.Validate(); err != nil {
		// Validate reports every fault at once, joined by "; ".
		msg := err.Error()
		if i := strings.Index(msg, ": "); i >= 0 {
			msg = msg[i+2:]
		}
		for _, part := range strings.Split(msg, "; ") {
			resp.Errors = append(resp.Errors, strings.TrimSpace(part))
		}
	}

	// Resolve regardless, so a contributor with a structural fault elsewhere
	// still sees what their figures came to.
	spec.Resolve()
	resp.Resolved = resolvedFigures(spec)
	resp.Warnings = specs.Sanity(spec)
	resp.Title = spec.Title()
	resp.SourceKind = string(spec.Source.Kind)
	resp.Verified = spec.Verified()
	resp.Disclaimer = spec.Disclaimer()

	if len(resp.Errors) == 0 {
		resp.OK = true
		file := struct {
			Specs []specs.Spec `json:"specs"`
		}{Specs: []specs.Spec{spec}}
		if b, err := json.MarshalIndent(file, "", "  "); err == nil {
			resp.File = string(b) + "\n"
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// saveSpec stores an owner's own entry — figures from their own manual — in
// their profile, where it is loaded on every start and ranks first in search.
// The same checks as for a contribution apply: an entry that would not load
// from the project's data directory is not saved here either.
func (s *Server) saveSpec(w http.ResponseWriter, r *http.Request) {
	var spec specs.Spec
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&spec); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%s: %w", i18n.T("не удалось разобрать данные"), err))
		return
	}
	if err := s.db.Save(spec); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	saved, _ := s.db.Get(spec.ID)
	writeJSON(w, http.StatusOK, s.summarise(saved))
}

func (s *Server) deleteSpec(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Delete(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func resolvedFigures(s specs.Spec) []ResolvedFigure {
	var out []ResolvedFigure
	rim := s.RimDiameterMM()

	for _, ax := range []struct {
		name string
		spec specs.AxleSpec
		mm   *specs.MMRange
	}{
		{i18n.T("Передняя ось"), s.Front, s.FrontTotalToeMM},
		{i18n.T("Задняя ось"), s.Rear, s.RearTotalToeMM},
	} {
		for _, f := range []struct {
			name string
			r    *align.Range
			toe  bool
		}{
			{i18n.T("Развал"), ax.spec.Camber, false},
			{i18n.T("Кастер"), ax.spec.Caster, false},
			{"SAI", ax.spec.SAI, false},
			{i18n.T("Суммарное схождение"), ax.spec.TotalToe, true},
			{i18n.T("Схождение одного колеса"), ax.spec.IndividualToe, true},
		} {
			if f.r == nil {
				continue
			}
			fig := ResolvedFigure{
				Axle: ax.name, Name: f.name,
				Range: fmt.Sprintf("%s … %s", f.r.Min.FormatDegMin(), f.r.Max.FormatDegMin()),
			}
			// Toe is the figure people quote in millimetres, so show both.
			if f.toe && rim > 0 {
				fig.Detail = i18n.F("%.1f … %.1f мм на ободе %.0f\"",
					f.r.Min.ToeMM(rim), f.r.Max.ToeMM(rim), s.RimDiameterIn)
			}
			out = append(out, fig)
		}
		if ax.mm != nil && rim > 0 {
			ang := ax.mm.ToAngle(rim)
			out = append(out, ResolvedFigure{
				Axle: ax.name, Name: i18n.T("Схождение задано в мм"),
				Range:  i18n.F("%.1f … %.1f мм", ax.mm.Min, ax.mm.Max),
				Detail: i18n.F("это %s … %s на ободе %.0f\"", ang.Min.FormatDegMin(), ang.Max.FormatDegMin(), s.RimDiameterIn),
			})
		}
	}
	return out
}
