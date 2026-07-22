package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"sort"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// The optical endpoints are deliberately per-wheel, and the interface says so.
//
// One camera photographing one wheel at a time gives that wheel's spin axis
// solidly — and from it camber, once gravity is known. What it does not give,
// yet, is the four wheels in one shared vehicle frame: that needs cross-wheel
// registration (a fixed floor reference visible in every shot) which is not
// built. So the optical mode here measures camber, which it does well, and the
// string method stays the way to set toe. Claiming a full four-wheel optical
// alignment would be claiming something the code cannot do.

const maxUpload = 64 << 20 // 64 MiB of photographs per request

// targetFromForm reads the checkerboard description from form fields, with the
// A4 9×6 / 30 mm default the docs recommend.
func targetFromForm(f *multipart.Form) (vision.Target, error) {
	t := vision.DefaultTarget()
	if v := formInt(f, "cols"); v > 0 {
		t.Cols = v
	}
	if v := formInt(f, "rows"); v > 0 {
		t.Rows = v
	}
	if v := formFloat(f, "square_mm"); v > 0 {
		t.SquareMM = v
	}
	return t, t.Validate()
}

func decodeUploads(files []*multipart.FileHeader) ([]*vision.Gray, []string, []string) {
	var imgs []*vision.Gray
	var labels, failed []string
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", fh.Filename, err))
			continue
		}
		img, err := vision.DecodeImage(f)
		f.Close()
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", fh.Filename, err))
			continue
		}
		imgs = append(imgs, img)
		labels = append(labels, fh.Filename)
	}
	return imgs, labels, failed
}

// calibrate runs Zhang's method on uploaded board photographs and returns a
// camera the operator can save and reuse.
func (s *Server) calibrate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать загруженные снимки: %w", err))
		return
	}
	target, err := targetFromForm(r.MultipartForm)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) < 3 {
		writeErr(w, http.StatusBadRequest, errors.New(
			"нужно минимум 3 снимка мишени, а лучше 10–20 — под разными углами и по всему кадру"))
		return
	}
	// Deterministic order so the "snapshot N" labels match what the operator
	// uploaded rather than the browser's arbitrary ordering.
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

	imgs, labels, failed := decodeUploads(files)
	if len(imgs) < 3 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("удалось прочитать только %d снимков: %v", len(imgs), failed))
		return
	}

	res, err := vision.CalibrateFromImages(target, imgs, labels, vision.CalibrateOptions{})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}
	for _, f := range failed {
		res.Warnings = append(res.Warnings, "Файл не прочитан — "+f)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"camera":            res.Camera,
		"rms_px":            res.RMSPx,
		"per_view_rms_px":   res.PerViewRMSPx,
		"coverage_fraction": res.CoverageFraction,
		"tilt_spread_deg":   res.TiltSpreadDeg,
		"views_used":        len(res.PerViewRMSPx),
		"quality":           calibrationGrade(res),
		"warnings":          res.Warnings,
	})
}

func calibrationGrade(res vision.CalibrationResult) string {
	switch {
	case res.RMSPx < 0.3 && res.TiltSpreadDeg >= 25 && res.CoverageFraction >= 0.5:
		return "good"
	case res.RMSPx < 1.0:
		return "ok"
	default:
		return "bad"
	}
}

// opticalCamber recovers one wheel's camber from photographs of it being turned.
func (s *Server) opticalCamber(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("не удалось прочитать загруженные снимки: %w", err))
		return
	}
	form := r.MultipartForm

	target, err := targetFromForm(form)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	cam, err := cameraFromForm(form)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	pos, err := positionFromForm(form)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	files := form.File["images"]
	if len(files) < 3 {
		writeErr(w, http.StatusBadRequest, errors.New(
			"нужно минимум 3 снимка колеса, провёрнутого между кадрами"))
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })
	imgs, _, failed := decodeUploads(files)

	res, err := vision.WheelSpinAxis(cam, target, imgs, vision.DetectOptions{})
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  err.Error(),
			"frames": res.Frames,
			"used":   res.Used,
		})
		return
	}

	// Gravity, in the camera frame. Default: a levelled camera, where down in
	// the world is down in the image. The +Y-down camera convention makes that
	// (0, 1, 0).
	gravity := geom.V(0, 1, 0)
	levelAssumed := true
	if g, ok := gravityFromForm(form); ok {
		gravity, levelAssumed = g, false
	}

	camber := vision.CamberFromSpinAxis(res.Axis, gravity)

	warnings := append([]string(nil), res.Warnings...)
	for _, f := range failed {
		warnings = append(warnings, "Файл не прочитан — "+f)
	}
	if levelAssumed {
		warnings = append(warnings, "Развал посчитан в предположении, что камера стояла строго горизонтально "+
			"(по уровню). Если камера была наклонена, развал сместится ровно на этот наклон — "+
			"выставьте камеру по пузырьковому уровню или укажите её наклон.")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"position":         pos.String(),
		"position_ru":      pos.RussianName(),
		"camber_deg":       round4(camber),
		"camber_degmin":    align.Deg(camber).FormatDegMin(),
		"runout_deg":       round4(res.RunoutDeg),
		"sweep_deg":        round4(res.SweepDeg),
		"axis_residual_mm": round4(res.AxisResidualMM),
		"frames":           res.Frames,
		"used":             res.Used,
		"level_assumed":    levelAssumed,
		"warnings":         warnings,
	})
}

func cameraFromForm(f *multipart.Form) (vision.Camera, error) {
	// The camera may arrive as an uploaded camera.json or pasted into a field.
	if fh := firstFile(f, "camera"); fh != nil {
		file, err := fh.Open()
		if err != nil {
			return vision.Camera{}, err
		}
		defer file.Close()
		var c vision.Camera
		if err := json.NewDecoder(file).Decode(&c); err != nil {
			return vision.Camera{}, fmt.Errorf("файл калибровки повреждён: %w", err)
		}
		return c, c.Validate()
	}
	if v := formValue(f, "camera_json"); v != "" {
		var c vision.Camera
		if err := json.Unmarshal([]byte(v), &c); err != nil {
			return vision.Camera{}, fmt.Errorf("калибровка камеры не разобрана: %w", err)
		}
		return c, c.Validate()
	}
	return vision.Camera{}, errors.New(
		"не приложена калибровка камеры (camera.json). Сначала откалибруйте камеру: wheelalign calibrate")
}

func positionFromForm(f *multipart.Form) (align.Position, error) {
	switch formValue(f, "position") {
	case "FL":
		return align.FL, nil
	case "FR":
		return align.FR, nil
	case "RL":
		return align.RL, nil
	case "RR":
		return align.RR, nil
	default:
		return align.FL, errors.New("не указано, какое это колесо (FL/FR/RL/RR)")
	}
}

func gravityFromForm(f *multipart.Form) (geom.Vec3, bool) {
	x, okX := formFloatOK(f, "gravity_x")
	y, okY := formFloatOK(f, "gravity_y")
	z, okZ := formFloatOK(f, "gravity_z")
	if !okX && !okY && !okZ {
		return geom.Vec3{}, false
	}
	g := geom.V(x, y, z)
	if g.Len() < 0.5 {
		return geom.Vec3{}, false
	}
	return g, true
}

func round4(v float64) float64 {
	return float64(int64(v*1e4+0.5*sign(v))) / 1e4
}
func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
