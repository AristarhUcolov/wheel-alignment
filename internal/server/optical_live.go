package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/align"
	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/live"
	"github.com/AristarhUcolov/wheel-alignment/internal/measure"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// The live camera: a phone (or this computer's camera) streams frames of one
// wheel and the floor, and every frame updates that wheel on the adjustment
// screen. See vision.ClampCalibration for why a single frame is enough once
// the runout has been compensated.
//
// Three things are learnt on the way and kept here:
//
//   - each camera's own calibration, from frames of the board shown at varied
//     angles — video frames, because a phone's camera app crops and scales
//     differently from its video stream, so a calibration from photos does
//     not fit;
//   - each wheel's clamp calibration, from frames of the wheel being turned;
//   - how the floor targets sit relative to one another, from any frame that
//     shows two of them.
//
// After that each wheel only has to be seen. Camber needs nothing else — the
// floor is the reference. Toe needs the whole car, so it appears once all four
// wheels have been seen with the car standing where it will be adjusted.

// Live camera modes.
const (
	modeCalib = "calib" // learning the camera
	modeSpin  = "spin"  // runout compensation for one wheel
	modeLive  = "live"  // measuring
)

const (
	calibMinViews = 15
	calibMaxViews = 30
	spinMinFrames = 5
	spinMinSweep  = 90.0
	spinStepDeg   = 6.0
	liveSource    = "camera-live"
)

type calibRun struct {
	w, h    int
	views   []vision.CalibrationView
	normals []geom.Vec3
	cellOf  []int
	cells   [9]bool
	failed  string
}

type spinRun struct {
	frame     string // "ref:N" or "camera": the fixed frame the poses are in
	poses     []geom.Pose
	camOrigin geom.Vec3
}

type seenWheel struct {
	axis, center geom.Vec3
	at           time.Time
	camber       float64
}

type liveOptical struct {
	mu     sync.Mutex
	dir    string
	wheel  vision.Target
	refs   []vision.Target
	rimIn  float64
	toRoot map[int]geom.Pose
	// links keeps every estimate of each floor target's place relative to the
	// root; toRoot is their average. One frame with two floor targets far away
	// and nearly edge-on is the weakest measurement in the whole chain, and its
	// rotation error turns every front wheel's toe by the same amount — so
	// every frame that shows two of them adds to the average.
	links  map[int][]geom.Pose
	cams   map[string]vision.Camera
	calib  map[string]*calibRun
	clamps map[align.Position]vision.ClampCalibration
	spins  map[align.Position]*spinRun
	seen   map[align.Position]seenWheel
	names  map[string]string // device id → name, for the desktop
}

func newLiveOptical() *liveOptical {
	return &liveOptical{
		// The same defaults as the photo measurement on the desktop.
		wheel: vision.Target{Cols: 8, Rows: 5, SquareMM: 32},
		refs: []vision.Target{
			{Cols: 7, Rows: 6, SquareMM: 100},
			{Cols: 6, Rows: 5, SquareMM: 100},
		},
		rimIn:  15,
		toRoot: map[int]geom.Pose{},
		links:  map[int][]geom.Pose{},
		cams:   map[string]vision.Camera{},
		calib:  map[string]*calibRun{},
		clamps: map[align.Position]vision.ClampCalibration{},
		spins:  map[align.Position]*spinRun{},
		seen:   map[align.Position]seenWheel{},
		names:  map[string]string{},
	}
}

// SetDataDir is where camera calibrations are kept between runs.
func (s *Server) SetDataDir(dir string) {
	lo := s.optLive
	lo.mu.Lock()
	defer lo.mu.Unlock()
	lo.dir = dir
	if dir == "" {
		return
	}
	files, _ := filepath.Glob(filepath.Join(dir, "cameras", "*.json"))
	for _, f := range files {
		cam, err := vision.LoadCamera(f)
		if err != nil {
			continue
		}
		lo.cams[strings.TrimSuffix(filepath.Base(f), ".json")] = cam
	}
}

var deviceIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)

// LiveFrameResponse is what the camera page shows after each frame.
type LiveFrameResponse struct {
	Mode     string   `json:"mode"`
	Found    []string `json:"found"`
	Message  string   `json:"message"`
	Progress float64  `json:"progress"`
	Done     bool     `json:"done,omitempty"`
	Camber   *float64 `json:"camber,omitempty"`
	Toe      *float64 `json:"toe,omitempty"`
	Warn     string   `json:"warn,omitempty"`
	Ms       int64    `json:"ms"`
}

// opticalFrame takes one camera frame: the image as the request body, the
// device, mode and wheel in the query.
func (s *Server) opticalFrame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dev := q.Get("d")
	if !deviceIDRe.MatchString(dev) {
		writeErr(w, http.StatusBadRequest, errors.New(i18n.T("нет идентификатора камеры")))
		return
	}
	img, err := vision.DecodeImage(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%s: %w", i18n.T("кадр не прочитан"), err))
		return
	}
	var pos align.Position
	hasPos := false
	for _, p := range align.AllPositions {
		if strings.EqualFold(q.Get("wheel"), p.String()) {
			pos, hasPos = p, true
		}
	}
	if name := strings.TrimSpace(q.Get("name")); name != "" {
		s.optLive.mu.Lock()
		s.optLive.names[dev] = truncateName(name)
		s.optLive.mu.Unlock()
	}

	t0 := time.Now()
	var resp LiveFrameResponse
	switch mode := q.Get("mode"); mode {
	case modeCalib:
		resp = s.frameCalib(dev, img)
	case modeSpin, modeLive:
		if !hasPos {
			writeErr(w, http.StatusBadRequest, errors.New(i18n.T("не указано колесо")))
			return
		}
		if mode == modeSpin {
			resp = s.frameSpin(dev, pos, img)
		} else {
			resp = s.frameLive(dev, pos, img)
		}
	default:
		writeErr(w, http.StatusBadRequest, errors.New(i18n.T("неизвестный режим камеры")))
		return
	}
	resp.Ms = time.Since(t0).Milliseconds()
	writeJSON(w, http.StatusOK, resp)
}

func truncateName(s string) string {
	r := []rune(s)
	if len(r) > 40 {
		r = r[:40]
	}
	return string(r)
}

// ── Camera calibration ────────────────────────────────────────────────

// frameCalib collects views of the wheel target for calibrating this camera.
//
// Not every frame is kept: thirty frames of the board held still say no more
// than one. A frame counts only if no view so far had the board in the same
// part of the picture at a similar tilt — variety of tilt and coverage of the
// frame being the two things a calibration actually needs.
func (s *Server) frameCalib(dev string, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	lo.mu.Lock()
	tg := lo.wheel
	lo.mu.Unlock()

	resp := LiveFrameResponse{Mode: modeCalib}
	det, err := vision.DetectCheckerboard(img, vision.DetectOptions{Target: tg})
	if err != nil {
		resp.Message = i18n.F("Покажите камере мишень с колеса (%d×%d углов) целиком.", tg.Cols, tg.Rows)
		return lo.calibProgress(dev, resp)
	}
	resp.Found = []string{i18n.T("мишень")}

	// A provisional camera is good enough to tell one tilt from another.
	guess := vision.GuessFromFOV(img.W, img.H, 65)
	corr := make([]vision.Correspondence, len(det.Corners))
	model := tg.ModelPoints()
	for i := range corr {
		corr[i] = vision.Correspondence{Model: model[i], Image: det.Corners[i]}
	}
	pose, err := vision.SolvePnPPlanar(guess, corr)
	if err != nil {
		resp.Message = i18n.T("Мишень видна, но слишком искажена — держите её ровнее.")
		return lo.calibProgress(dev, resp)
	}
	normal := pose.Pose.R.Col(2)
	var cx, cy float64
	for _, c := range det.Corners {
		cx += c.X
		cy += c.Y
	}
	cx /= float64(len(det.Corners))
	cy /= float64(len(det.Corners))
	cell := min(int(3*cy/float64(img.H)), 2)*3 + min(int(3*cx/float64(img.W)), 2)

	lo.mu.Lock()
	run := lo.calib[dev]
	if run == nil || run.w != img.W || run.h != img.H {
		run = &calibRun{w: img.W, h: img.H}
		lo.calib[dev] = run
	}
	novel := true
	for i, n := range run.normals {
		if run.cellOf[i] == cell && geom.Deg(math.Acos(math.Max(-1, math.Min(1, n.Dot(normal))))) < 7 {
			novel = false
			break
		}
	}
	accept := len(run.views) < calibMaxViews && novel
	if accept {
		run.views = append(run.views, vision.CalibrationView{Corners: det.Corners, Label: i18n.F("кадр %d", len(run.views)+1)})
		run.normals = append(run.normals, normal)
		run.cellOf = append(run.cellOf, cell)
		run.cells[cell] = true
	}
	views := append([]vision.CalibrationView(nil), run.views...)
	covered := 0
	for _, c := range run.cells {
		if c {
			covered++
		}
	}
	lo.mu.Unlock()

	if !accept {
		resp.Message = i18n.T("Такой кадр уже есть — наклоните мишень иначе или сдвиньте её к другому краю кадра.")
		return lo.calibProgress(dev, resp)
	}
	if len(views) < calibMinViews || covered < 6 {
		resp.Message = i18n.F("Кадр принят (%d из %d). Меняйте наклон и ведите мишень по всем краям кадра.", len(views), calibMinViews)
		return lo.calibProgress(dev, resp)
	}

	res, err := vision.CalibrateCamera(tg, views, img.W, img.H, vision.CalibrateOptions{})
	if err == nil && (res.RMSPx > 1.0 || res.TiltSpreadDeg < 25) {
		err = errors.New(i18n.F("СКО %.2f пикс, разброс наклона %.0f°", res.RMSPx, res.TiltSpreadDeg))
	}
	if err != nil {
		lo.mu.Lock()
		run.failed = err.Error()
		lo.mu.Unlock()
		resp.Message = i18n.F("Пока не сходится (%s) — продолжайте: больше наклона и кадров у краёв.", err.Error())
		return lo.calibProgress(dev, resp)
	}

	lo.mu.Lock()
	lo.cams[dev] = res.Camera
	delete(lo.calib, dev)
	dir := lo.dir
	lo.mu.Unlock()
	if dir != "" {
		_ = os.MkdirAll(filepath.Join(dir, "cameras"), 0o755)
		_ = res.Camera.Save(filepath.Join(dir, "cameras", dev+".json"))
	}
	resp.Done, resp.Progress = true, 1
	resp.Message = i18n.F("Камера откалибрована: СКО %.2f пикс по %d кадрам. Калибровка сохранена для этого телефона.", res.RMSPx, len(views))
	if len(res.Warnings) > 0 {
		resp.Warn = strings.Join(res.Warnings, " ")
	}
	return resp
}

func (lo *liveOptical) calibProgress(dev string, resp LiveFrameResponse) LiveFrameResponse {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	if run := lo.calib[dev]; run != nil {
		resp.Progress = math.Min(1, float64(len(run.views))/calibMinViews)
	}
	return resp
}

// ── Runout compensation ───────────────────────────────────────────────

// frameSpin collects poses of one wheel's target while the wheel is turned,
// and fits the clamp calibration once the wheel has turned far enough.
func (s *Server) frameSpin(dev string, p align.Position, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	resp := LiveFrameResponse{Mode: modeSpin}
	lo.mu.Lock()
	cam, ok := lo.cams[dev]
	tg, refs := lo.wheel, append([]vision.Target(nil), lo.refs...)
	lo.mu.Unlock()
	if msg := cameraMismatch(cam, ok, img); msg != "" {
		resp.Message = msg
		return resp
	}

	f, err := vision.DetectLive(cam, tg, refs, img, vision.DetectOptions{})
	resp.Found = foundList(f, err)
	if err != nil {
		resp.Message = i18n.F("%s: мишень на колесе не видна.", p.Label())
		return resp
	}

	lo.mu.Lock()
	run := lo.spins[p]
	if run == nil {
		run = &spinRun{}
		lo.spins[p] = run
	}
	// The first frame decides the fixed frame: a floor target if one is in
	// view — then the phone may move — otherwise the camera itself.
	var pose geom.Pose
	var origin geom.Vec3
	switch {
	case run.frame == "" && f.Ref >= 0:
		run.frame = fmt.Sprintf("ref:%d", f.Ref)
	case run.frame == "":
		run.frame = "camera"
	}
	if run.frame == "camera" {
		pose = f.Wheel.Pose
	} else {
		var k int
		fmt.Sscanf(run.frame, "ref:%d", &k)
		r, seen := f.Refs[k]
		if !seen {
			lo.mu.Unlock()
			resp.Message = i18n.T("Напольная мишень пропала из кадра — верните её в кадр или начните заново.")
			return resp
		}
		inv := r.Pose.Inverse()
		pose = inv.Mul(f.Wheel.Pose)
		origin = inv.Apply(geom.V(0, 0, 0))
	}

	moved := len(run.poses) == 0
	if !moved {
		_, ang := geom.AxisAngle(pose.R.Mul(run.poses[len(run.poses)-1].R.T()))
		moved = geom.Deg(ang) >= spinStepDeg
	}
	if moved {
		run.poses = append(run.poses, pose)
		run.camOrigin = origin
	}
	poses := append([]geom.Pose(nil), run.poses...)
	camOrigin := run.camOrigin
	lo.mu.Unlock()

	var sweep float64
	if len(poses) > 1 {
		if fit, err := geom.FitRotationAxis(poses); err == nil {
			sweep = geom.Deg(fit.Sweep)
		}
	}
	// Toe is only as good as the axis, and the axis is found far better from
	// a quarter turn than from a few degrees: the same tilt of a target reads
	// the same over a small arc whatever the axis is.
	resp.Progress = math.Min(1, math.Min(sweep/spinMinSweep, float64(len(poses))/spinMinFrames))
	if len(poses) < spinMinFrames || sweep < spinMinSweep {
		resp.Message = i18n.F("%s: поворачивайте колесо — провёрнуто на %.0f° из %.0f°, кадров %d из %d.",
			p.Label(), sweep, spinMinSweep, len(poses), spinMinFrames)
		return resp
	}

	c, err := vision.ClampFromPoses(poses, camOrigin)
	if err != nil {
		resp.Message = fmt.Sprintf("%s: %v", p.Label(), err)
		return resp
	}
	lo.mu.Lock()
	lo.clamps[p] = c
	delete(lo.spins, p)
	delete(lo.seen, p)
	lo.mu.Unlock()
	resp.Done, resp.Progress = true, 1
	resp.Message = i18n.F("%s: биение учтено — мишень стоит с перекосом %.1f°. Опустите колесо, поставьте машину и переходите к замеру.", p.Label(), c.RunoutDeg)
	if c.ResidualMM > 3 {
		resp.Warn = i18n.F("Ось найдена с разбросом %.1f мм — колесо, похоже, не только вращали, но и сдвигали. Надёжнее вывесить колесо и провернуть его рукой.", c.ResidualMM)
	}
	return resp
}

// ── Live measurement ──────────────────────────────────────────────────

// frameLive places one wheel from one frame and publishes its angles.
func (s *Server) frameLive(dev string, p align.Position, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	resp := LiveFrameResponse{Mode: modeLive}
	lo.mu.Lock()
	cam, ok := lo.cams[dev]
	tg, refs := lo.wheel, append([]vision.Target(nil), lo.refs...)
	clamp, clamped := lo.clamps[p]
	lo.mu.Unlock()
	if msg := cameraMismatch(cam, ok, img); msg != "" {
		resp.Message = msg
		return resp
	}

	f, err := vision.DetectLive(cam, tg, refs, img, vision.DetectOptions{})
	resp.Found = foundList(f, err)

	// Any frame with two floor targets ties them together — with or without
	// a wheel in it.
	linked, samples := lo.link(f)

	if err != nil {
		switch {
		case linked:
			resp.Message = i18n.T("Напольные мишени связаны между собой. Ещё два-три таких кадра с разных сторон сделают связь точнее; потом наведите камеру на колесо.")
		case samples > 0:
			resp.Message = i18n.F("Связь напольных мишеней уточнена (кадров: %d). Наведите камеру на колесо.", samples)
		default:
			resp.Message = i18n.F("%s: мишень на колесе не видна.", p.Label())
		}
		return resp
	}
	if !clamped {
		resp.Message = i18n.F("%s: сначала учтите биение — режим «Биение».", p.Label())
		return resp
	}
	if f.Ref < 0 {
		resp.Message = i18n.T("Напольная мишень не видна — без неё угол не к чему привязать. Поверните камеру так, чтобы в кадре были и колесо, и мишень на полу.")
		return resp
	}

	lo.mu.Lock()
	k, root, ok := lo.rootFor(f)
	if !ok {
		lo.mu.Unlock()
		resp.Message = i18n.T("Эта напольная мишень ещё не связана с остальными: снимите кадр, где видны обе мишени на полу.")
		return resp
	}
	inRef := f.Refs[k].Pose.Inverse().Mul(f.Wheel.Pose)
	axis, center := clamp.Place(root.Mul(inRef))
	camber := vision.CamberFromAxis(axis, geom.V(0, 0, 1))
	lo.seen[p] = seenWheel{axis: axis, center: center, at: time.Now(), camber: camber}
	seen := map[align.Position]seenWheel{}
	for q, v := range lo.seen {
		seen[q] = v
	}
	name := lo.names[dev]
	rim := lo.rimIn
	lo.mu.Unlock()

	if name == "" {
		name = i18n.N("Камера")
	}
	s.hub.Touch(live.SourceInfo{ID: liveSource, Kind: "camera", Name: name, Wheel: p.String(),
		Detail: i18n.N("развал, схождение")})

	c := round4(camber)
	resp.Camber = &c
	var toe *float64
	if len(seen) == 4 {
		wheels := map[align.Position]vision.RegisteredWheel{}
		for q, v := range seen {
			wheels[q] = vision.RegisteredWheel{Axis: v.axis, Center: v.center, Used: 1}
		}
		res, err := measure.OpticalSession{Wheels: wheels, RimDiameterMM: align.Inches(rim)}.Result()
		if err == nil {
			if wr, ok := res.Wheels[p.String()]; ok {
				c = round4(wr.Camber.Deg())
				t := round4(wr.ToeGeometric.Deg())
				resp.Camber, toe = &c, &t
			}
		}
	}
	resp.Toe = toe
	_ = s.hub.Push(live.Input{Wheel: p, Camber: resp.Camber, Toe: toe, Source: liveSource})

	if toe == nil {
		missing := []string{}
		for _, q := range align.AllPositions {
			if _, ok := seen[q]; !ok {
				missing = append(missing, q.Label())
			}
		}
		resp.Message = i18n.F("%s: развал %+.2f°. Схождение появится, когда камера увидит все четыре колеса — осталось: %s.",
			p.Label(), c, strings.Join(missing, ", "))
	} else {
		resp.Message = i18n.F("%s: развал %+.2f°, схождение %+.2f°.", p.Label(), c, *toe)
	}
	return resp
}

// link records how floor targets seen together sit relative to one another.
// The first floor target ever seen becomes the root. Reports whether this
// frame tied in a target that was not tied before, and how many frames the
// best-measured link now rests on (0 when this frame added none).
func (lo *liveOptical) link(f vision.LiveFrame) (bool, int) {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	if len(f.Refs) == 0 {
		return false, 0
	}
	ids := make([]int, 0, len(f.Refs))
	for k := range f.Refs {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	if len(lo.toRoot) == 0 {
		lo.toRoot[ids[0]] = geom.IdentityPose()
		lo.links[ids[0]] = nil
	}
	added, samples := false, 0
	// Only floor targets tied to the root through some OTHER target in this
	// frame get a new estimate; the root itself never moves.
	var root int
	for k, l := range lo.links {
		if l == nil {
			root = k
		}
	}
	for changed := true; changed; {
		changed = false
		for _, j := range ids {
			rj, known := lo.toRoot[j]
			if !known {
				continue
			}
			for _, k := range ids {
				if k == j || k == root {
					continue
				}
				// root←k = root←j · j←k, and j←k = (cam←j)⁻¹ · cam←k.
				est := rj.Mul(f.Refs[j].Pose.Inverse().Mul(f.Refs[k].Pose))
				if _, have := lo.toRoot[k]; !have {
					changed, added = true, true
				} else if j != root {
					// Estimates through a non-root target would only echo
					// the average back into itself.
					continue
				}
				lo.links[k] = append(lo.links[k], est)
				if len(lo.links[k]) > 30 {
					lo.links[k] = lo.links[k][1:]
				}
				lo.toRoot[k] = averagePose(lo.links[k])
				samples = max(samples, len(lo.links[k]))
			}
		}
	}
	return added, samples
}

// averagePose averages poses: translations directly, rotations as small
// turns away from the first.
func averagePose(ps []geom.Pose) geom.Pose {
	if len(ps) == 1 {
		return ps[0]
	}
	base := ps[0].R
	var rot, tr geom.Vec3
	for _, p := range ps {
		ax, ang := geom.AxisAngle(base.T().Mul(p.R))
		rot = rot.Add(ax.Scale(ang))
		tr = tr.Add(p.T)
	}
	n := float64(len(ps))
	rot, tr = rot.Scale(1/n), tr.Scale(1/n)
	r := base
	if a := rot.Len(); a > 1e-12 {
		r = base.Mul(geom.Rodrigues(rot.Scale(1/a), a))
	}
	return geom.Pose{R: geom.Orthonormalize(r), T: tr}
}

// rootFor picks a floor target in the frame that is tied to the root.
func (lo *liveOptical) rootFor(f vision.LiveFrame) (int, geom.Pose, bool) {
	if r, ok := lo.toRoot[f.Ref]; ok {
		return f.Ref, r, true
	}
	for k := range f.Refs {
		if r, ok := lo.toRoot[k]; ok {
			return k, r, true
		}
	}
	return 0, geom.Pose{}, false
}

// cameraMismatch says why a frame cannot be measured with this camera's
// calibration: there is none, or it was made for a different frame size — a
// phone turned upright gives a portrait frame, and a calibration from
// landscape frames does not fit it at all.
func cameraMismatch(cam vision.Camera, ok bool, img *vision.Gray) string {
	if !ok {
		return i18n.T("Сначала откалибруйте камеру этого телефона: режим «Калибровка камеры».")
	}
	if cam.Width != img.W || cam.Height != img.H {
		return i18n.F("Камера откалибрована для кадра %d×%d, а сейчас кадр %d×%d. Держите телефон так же, как при калибровке, или откалибруйте заново.",
			cam.Width, cam.Height, img.W, img.H)
	}
	return ""
}

func foundList(f vision.LiveFrame, err error) []string {
	var out []string
	if err == nil {
		out = append(out, i18n.T("колесо"))
	}
	ids := make([]int, 0, len(f.Refs))
	for k := range f.Refs {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	for _, k := range ids {
		out = append(out, i18n.F("пол %d", k+1))
	}
	return out
}

// ── Desktop status and settings ───────────────────────────────────────

// LiveOpticalState is the live camera mode as the desktop shows it.
type LiveOpticalState struct {
	WheelTarget vision.Target            `json:"wheel_target"`
	Refs        []vision.Target          `json:"refs"`
	RimIn       float64                  `json:"rim_in"`
	Cameras     []LiveCameraInfo         `json:"cameras"`
	Linked      []int                    `json:"linked"`
	Wheels      map[string]LiveWheelInfo `json:"wheels"`
}

// LiveCameraInfo is one calibrated camera.
type LiveCameraInfo struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	RMSPx float64 `json:"rms_px"`
	Size  string  `json:"size"`
	FxPx  float64 `json:"fx_px"`
}

// LiveWheelInfo is one wheel's progress.
type LiveWheelInfo struct {
	Clamped   bool     `json:"clamped"`
	RunoutDeg float64  `json:"runout_deg,omitempty"`
	SpinDeg   float64  `json:"spin_deg,omitempty"`
	SeenAgoS  *float64 `json:"seen_ago_s,omitempty"`
	Camber    *float64 `json:"camber,omitempty"`
}

func (s *Server) liveOpticalState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.optLive.state())
}

func (lo *liveOptical) state() LiveOpticalState {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	st := LiveOpticalState{WheelTarget: lo.wheel, Refs: lo.refs, RimIn: lo.rimIn,
		Cameras: []LiveCameraInfo{}, Linked: []int{}, Wheels: map[string]LiveWheelInfo{}}
	for id, c := range lo.cams {
		name := lo.names[id]
		if name == "" {
			name = id
		}
		st.Cameras = append(st.Cameras, LiveCameraInfo{ID: id, Name: name, RMSPx: round4(c.CalibrationRMSPx),
			Size: fmt.Sprintf("%d×%d", c.Width, c.Height), FxPx: math.Round(c.Fx)})
	}
	sort.Slice(st.Cameras, func(i, j int) bool { return st.Cameras[i].ID < st.Cameras[j].ID })
	for k := range lo.toRoot {
		st.Linked = append(st.Linked, k)
	}
	sort.Ints(st.Linked)
	for _, p := range align.AllPositions {
		var wi LiveWheelInfo
		if c, ok := lo.clamps[p]; ok {
			wi.Clamped, wi.RunoutDeg = true, round4(c.RunoutDeg)
		}
		if sp := lo.spins[p]; sp != nil && len(sp.poses) > 1 {
			if fit, err := geom.FitRotationAxis(sp.poses); err == nil {
				wi.SpinDeg = round4(geom.Deg(fit.Sweep))
			}
		}
		if v, ok := lo.seen[p]; ok {
			ago := math.Round(time.Since(v.at).Seconds()*10) / 10
			c := round4(v.camber)
			wi.SeenAgoS, wi.Camber = &ago, &c
		}
		st.Wheels[p.String()] = wi
	}
	return st
}

// LiveOpticalSettings changes the live camera mode.
type LiveOpticalSettings struct {
	WheelTarget *vision.Target  `json:"wheel_target,omitempty"`
	Refs        []vision.Target `json:"refs,omitempty"`
	RimIn       float64         `json:"rim_in,omitempty"`
	// Reset forgets clamp calibrations, links and seen wheels — for a new car
	// or targets moved. Camera calibrations are kept: they belong to the
	// cameras.
	Reset bool `json:"reset,omitempty"`
	// ForgetCamera drops one camera's calibration.
	ForgetCamera string `json:"forget_camera,omitempty"`
	// Camera sets a camera's calibration directly (a camera.json made from
	// frames of the same video stream).
	Camera   *vision.Camera `json:"camera,omitempty"`
	CameraID string         `json:"camera_id,omitempty"`
}

func (s *Server) setLiveOptical(w http.ResponseWriter, r *http.Request) {
	var req LiveOpticalSettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	lo := s.optLive
	if req.WheelTarget != nil || len(req.Refs) > 0 {
		wt := lo.wheel
		if req.WheelTarget != nil {
			wt = *req.WheelTarget
		}
		refs := lo.refs
		if len(req.Refs) > 0 {
			refs = req.Refs
		}
		if err := wt.Validate(); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("%s: %w", i18n.T("мишень на колесе"), err))
			return
		}
		for i, t := range refs {
			if err := t.Validate(); err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("%s: %w", i18n.F("напольная мишень %d", i+1), err))
				return
			}
			if (t.Cols == wt.Cols && t.Rows == wt.Rows) || (t.Cols == wt.Rows && t.Rows == wt.Cols) {
				writeErr(w, http.StatusBadRequest, errors.New(i18n.T("напольная мишень должна отличаться размером от мишени на колесе")))
				return
			}
		}
		lo.mu.Lock()
		changed := wt != lo.wheel || len(refs) != len(lo.refs)
		for i := range refs {
			if !changed && refs[i] != lo.refs[i] {
				changed = true
			}
		}
		lo.wheel, lo.refs = wt, refs
		if changed {
			lo.resetLocked()
		}
		lo.mu.Unlock()
	}
	lo.mu.Lock()
	if req.RimIn > 0 {
		lo.rimIn = req.RimIn
	}
	if req.Reset {
		lo.resetLocked()
		s.hub.Clear(liveSource)
	}
	if req.ForgetCamera != "" {
		delete(lo.cams, req.ForgetCamera)
		if lo.dir != "" && deviceIDRe.MatchString(req.ForgetCamera) {
			_ = os.Remove(filepath.Join(lo.dir, "cameras", req.ForgetCamera+".json"))
		}
	}
	if req.Camera != nil {
		if !deviceIDRe.MatchString(req.CameraID) {
			lo.mu.Unlock()
			writeErr(w, http.StatusBadRequest, errors.New(i18n.T("нет идентификатора камеры")))
			return
		}
		if err := req.Camera.Validate(); err != nil {
			lo.mu.Unlock()
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		lo.cams[req.CameraID] = *req.Camera
	}
	lo.mu.Unlock()
	writeJSON(w, http.StatusOK, lo.state())
}

func (lo *liveOptical) resetLocked() {
	lo.toRoot = map[int]geom.Pose{}
	lo.links = map[int][]geom.Pose{}
	lo.clamps = map[align.Position]vision.ClampCalibration{}
	lo.spins = map[align.Position]*spinRun{}
	lo.seen = map[align.Position]seenWheel{}
	lo.calib = map[string]*calibRun{}
}
