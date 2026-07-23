package server

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// WheelOpticalInfo is the per-wheel diagnostic the operator needs to judge a
// full optical measurement — not the angles, which are in the report, but
// whether the photographs themselves were good enough to believe.
type WheelOpticalInfo struct {
	Position       string               `json:"position"`
	PositionRU     string               `json:"position_ru"`
	Used           int                  `json:"used"`
	Total          int                  `json:"total"`
	RunoutDeg      float64              `json:"runout_deg"`
	SweepDeg       float64              `json:"sweep_deg"`
	AxisResidualMM float64              `json:"axis_residual_mm"`
	Reference      int                  `json:"reference"`
	Frames         []vision.FrameResult `json:"frames"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// OpticalAlignResponse carries the same result and report the string-line path
// produces, plus the optical diagnostics. Being the same shape is the point:
// the interface renders it with exactly the code that renders a manual
// measurement, because how the angles were obtained was never allowed to reach
// the reporting layer.
type OpticalAlignResponse struct {
	Result  align.Result       `json:"result"`
	Report  specs.Report       `json:"report"`
	Optical []WheelOpticalInfo `json:"optical"`
}

// opticalAlign performs a complete four-wheel optical alignment: every wheel's
// frames registered against a floor reference board, the reference boards linked
// to one another, and the whole assembled into a normal alignment report.
func (s *Server) opticalAlign(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать загруженные снимки: %w", err))
		return
	}
	form := r.MultipartForm

	cam, err := cameraFromForm(form)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	wheelTarget, err := targetFromPrefix(form, "wheel_")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("мишень на колесе: %w", err))
		return
	}

	// Reference boards, described as ref0_*, ref1_*, …
	var refs []vision.Target
	for i := 0; ; i++ {
		prefix := fmt.Sprintf("ref%d_", i)
		if !hasPrefixFields(form, prefix) {
			break
		}
		t, err := targetFromPrefix(form, prefix)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("напольная мишень %d: %w", i+1, err))
			return
		}
		refs = append(refs, t)
	}
	if len(refs) == 0 {
		writeErr(w, http.StatusBadRequest, errors.New(
			"не описана ни одна напольная мишень — без неё колёса невозможно свести в одну систему координат"))
		return
	}

	// Link the reference boards, when there is more than one.
	toRoot := map[int]geom.Pose{0: geom.IdentityPose()}
	var linkWarnings []string
	if len(refs) > 1 {
		linkFiles := form.File["images_link"]
		if len(linkFiles) == 0 {
			writeErr(w, http.StatusBadRequest, errors.New(
				"напольных мишеней несколько, но нет связующих снимков — нужен хотя бы один кадр, "+
					"где видно сразу две мишени"))
			return
		}
		sort.Slice(linkFiles, func(i, j int) bool { return linkFiles[i].Filename < linkFiles[j].Filename })
		linkImgs, _, failed := decodeUploads(linkFiles)

		link, err := vision.LinkReferences(cam, refs, linkImgs, vision.DetectOptions{})
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, err)
			return
		}
		for k, v := range link.ToRoot {
			toRoot[k] = v
		}
		linkWarnings = link.Warnings
		for _, f := range failed {
			linkWarnings = append(linkWarnings, "Связующий снимок не прочитан — "+f)
		}
	}

	// Register each wheel against its reference board and bring it to the root.
	inRoot := map[align.Position]vision.RegisteredWheel{}
	infos := make([]WheelOpticalInfo, 0, 4)

	for _, p := range align.AllPositions {
		files := form.File["images_"+p.String()]
		if len(files) < 3 {
			writeErr(w, http.StatusBadRequest, fmt.Errorf(
				"%s (%s): нужно минимум 3 снимка с проворотом колеса между ними, загружено %d",
				p, p.RussianName(), len(files)))
			return
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })
		imgs, _, failed := decodeUploads(files)

		refIdx := formInt(form, "ref_"+p.String())
		if refIdx < 0 || refIdx >= len(refs) {
			refIdx = 0
		}

		reg, err := vision.RegisterWheel(cam, wheelTarget, refs[refIdx], imgs, vision.DetectOptions{})
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":    fmt.Sprintf("%s (%s): %v", p, p.RussianName(), err),
				"position": p.String(),
				"frames":   reg.Frames,
				"used":     reg.Used,
			})
			return
		}

		root, ok := toRoot[refIdx]
		if !ok {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Errorf(
				"%s: напольная мишень %d не связана с остальными", p, refIdx+1))
			return
		}
		inRoot[p] = reg.InFrameOf(root)

		warns := append([]string(nil), reg.Warnings...)
		for _, f := range failed {
			warns = append(warns, "Снимок не прочитан — "+f)
		}
		infos = append(infos, WheelOpticalInfo{
			Position: p.String(), PositionRU: p.RussianName(),
			Used: reg.Used, Total: len(files),
			RunoutDeg: round4(reg.RunoutDeg), SweepDeg: round4(reg.SweepDeg),
			AxisResidualMM: round4(reg.AxisResidualMM),
			Reference:      refIdx, Frames: reg.Frames, Warnings: warns,
		})
	}

	rimMM := align.Inches(formFloat(form, "rim_diameter_in"))
	var spec *specs.Spec
	if id := formValue(form, "spec_id"); id != "" {
		if sp, ok := s.db.Get(id); ok {
			spec = &sp
			if rimMM <= 0 {
				rimMM = sp.RimDiameterMM()
			}
		}
	}
	if rimMM <= 0 {
		rimMM = align.Inches(15)
	}

	res, err := measure.OpticalSession{Wheels: inRoot, RimDiameterMM: rimMM}.Result()
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}
	res.Warnings = append(linkWarnings, res.Warnings...)
	res.Warnings = append(res.Warnings, cam.Warnings()...)

	writeJSON(w, http.StatusOK, OpticalAlignResponse{
		Result:  res,
		Report:  specs.Compare(res, spec),
		Optical: infos,
	})
}
