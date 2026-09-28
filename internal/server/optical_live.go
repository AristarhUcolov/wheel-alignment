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

// The live camera: a phone (or this computer's camera) streams frames of a
// wheel and the floor, and every frame updates that wheel on the adjustment
// screen. See vision.ClampCalibration for why a single frame is enough once
// the runout has been compensated.
//
// Three things are learnt on the way and kept here:
//
//   - each camera's own calibration, from frames of a board shown at varied
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
// Caster comes from steering a front wheel on its turn plate while the camera
// watches: the spin axis sweeps a cone about the steering axis.
//
// With a different target on each wheel the camera also knows which wheel it
// is looking at, and a frame showing two wheels measures both.

// Live camera modes.
const (
	modeCalib  = "calib"  // learning the camera
	modeSpin   = "spin"   // runout compensation for one wheel
	modeLive   = "live"   // measuring
	modeCaster = "caster" // steering a front wheel for caster
)

const (
	calibMinViews   = 15
	calibMaxViews   = 30
	spinMinFrames   = 5
	spinMinSweep    = 90.0
	spinStepDeg     = 6.0
	casterMinSide   = 12.0 // degrees of steer needed each way
	casterStepDeg   = 1.5
	casterMinFrames = 10
	maxCamberDeg    = 12.0 // beyond this a reading is a mix-up, not a wheel
	liveSource      = "camera-live"
)

// DefaultWheelTargets are the four wheel boards the program prints: all
// different, none a part of another — (11,4), (10,5), (9,6), (8,7) sum to the
// same odd number, so for any two one is longer and the other taller — and
// each fits a sheet of A4 with the printer's margins.
var DefaultWheelTargets = map[align.Position]vision.Target{
	align.FL: {Cols: 11, Rows: 4, SquareMM: 22},
	align.FR: {Cols: 10, Rows: 5, SquareMM: 24},
	align.RL: {Cols: 9, Rows: 6, SquareMM: 24},
	align.RR: {Cols: 8, Rows: 7, SquareMM: 21},
}

// DefaultFloorTargets are the two floor boards, as for the photo measurement.
var DefaultFloorTargets = []vision.Target{
	{Cols: 7, Rows: 6, SquareMM: 100},
	{Cols: 6, Rows: 5, SquareMM: 100},
}

type calibRun struct {
	w, h    int
	target  vision.Target // the board this series is of
	views   []vision.CalibrationView
	normals []geom.Vec3
	cellOf  []int
	cells   [9]bool
}

type spinRun struct {
	frame     string // "ref:N" or "camera": the fixed frame the poses are in
	poses     []geom.Pose
	camOrigin geom.Vec3
}

type casterRun struct {
	straight geom.Vec3   // spin axis with the wheel straight ahead, root frame
	poses    []geom.Pose // the wheel target through the sweep, root frame
	lastAxis geom.Vec3
	lo, hi   float64 // steer reached each way, degrees, outboard positive
}

type seenWheel struct {
	axis, center geom.Vec3
	at           time.Time
	camber       float64
}

type liveOptical struct {
	mu     sync.Mutex
	dir    string
	wheels map[align.Position]vision.Target
	refs   []vision.Target
	rimIn  float64
	toRoot map[int]geom.Pose
	// links keeps every estimate of each floor target's place relative to the
	// root; toRoot is their average. One frame with two floor targets far away
	// and nearly edge-on is the weakest measurement in the whole chain, and its
	// rotation error turns every front wheel's toe by the same amount — so
	// every frame that shows two of them adds to the average.
	links   map[int][]geom.Pose
	cams    map[string]vision.Camera
	calib   map[string]*calibRun
	clamps  map[align.Position]vision.ClampCalibration
	spins   map[align.Position]*spinRun
	casters map[align.Position]*casterRun
	caster  map[align.Position][2]float64 // measured caster and SAI
	seen    map[align.Position]seenWheel
	names   map[string]string // device id → name, for the desktop
}

func newLiveOptical() *liveOptical {
	lo := &liveOptical{rimIn: 15, names: map[string]string{}, cams: map[string]vision.Camera{},
		wheels: map[align.Position]vision.Target{}, refs: append([]vision.Target(nil), DefaultFloorTargets...)}
	for p, t := range DefaultWheelTargets {
		lo.wheels[p] = t
	}
	lo.resetLocked()
	return lo
}

// distinct reports whether every wheel has its own board, so that a frame
// says which wheel it shows.
func (lo *liveOptical) distinct() bool {
	for i, p := range align.AllPositions {
		for _, q := range align.AllPositions[i+1:] {
			if sameBoard(lo.wheels[p], lo.wheels[q]) {
				return false
			}
		}
	}
	return true
}

// search is the list of wheel boards to look for, and which wheel each is.
// With one board shared by all wheels there is one entry, and the wheel comes
// from the phone.
func (lo *liveOptical) search() ([]vision.Target, []align.Position) {
	if !lo.distinct() {
		return []vision.Target{lo.wheels[align.FL]}, nil
	}
	ts := make([]vision.Target, len(align.AllPositions))
	for i, p := range align.AllPositions {
		ts[i] = lo.wheels[p]
	}
	return ts, align.AllPositions[:]
}

func sameBoard(a, b vision.Target) bool {
	return (a.Cols == b.Cols && a.Rows == b.Rows) || (a.Cols == b.Rows && a.Rows == b.Cols)
}

// insideBoard reports whether board a fits inside board b — so that a part of
// b, the rest hidden, would read as a.
func insideBoard(a, b vision.Target) bool {
	return (a.Cols <= b.Cols && a.Rows <= b.Rows) || (a.Cols <= b.Rows && a.Rows <= b.Cols)
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
	Wheel    string   `json:"wheel,omitempty"` // the wheel measured, when known
	Found    []string `json:"found"`
	Message  string   `json:"message"`
	Progress float64  `json:"progress"`
	Done     bool     `json:"done,omitempty"`
	Camber   *float64 `json:"camber,omitempty"`
	Toe      *float64 `json:"toe,omitempty"`
	Caster   *float64 `json:"caster,omitempty"`
	SAI      *float64 `json:"sai,omitempty"`
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
	lo := s.optLive
	lo.mu.Lock()
	if name := strings.TrimSpace(q.Get("name")); name != "" {
		lo.names[dev] = truncateName(name)
	}
	distinct := lo.distinct()
	lo.mu.Unlock()

	t0 := time.Now()
	var resp LiveFrameResponse
	switch mode := q.Get("mode"); mode {
	case modeCalib:
		resp = s.frameCalib(dev, img)
	case modeSpin, modeLive, modeCaster:
		// With one board on every wheel the phone must say which wheel it
		// looks at; with a board per wheel the frame says it.
		if !hasPos && !distinct {
			writeErr(w, http.StatusBadRequest, errors.New(i18n.T("не указано колесо")))
			return
		}
		switch mode {
		case modeSpin:
			resp = s.frameSpin(dev, pos, hasPos, img)
		case modeCaster:
			resp = s.frameCaster(dev, pos, hasPos, img)
		default:
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

// frameCalib collects views of a wheel board for calibrating this camera.
//
// Not every frame is kept: thirty frames of the board held still say no more
// than one. A frame counts only if no view so far had the board in the same
// part of the picture at a similar tilt — variety of tilt and coverage of the
// frame being the two things a calibration actually needs. The first board
// seen is the one the whole series must show: views of different boards
// cannot be solved together.
func (s *Server) frameCalib(dev string, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	lo.mu.Lock()
	boards, _ := lo.search()
	if run := lo.calib[dev]; run != nil && run.w == img.W && run.h == img.H {
		boards = []vision.Target{run.target}
	}
	lo.mu.Unlock()

	resp := LiveFrameResponse{Mode: modeCalib}
	dets, errs := vision.DetectBoards(img, boards, vision.DetectOptions{})
	tg, found := vision.Target{}, -1
	for i := range boards {
		if errs[i] == nil {
			tg, found = boards[i], i
			break
		}
	}
	if found < 0 {
		if len(boards) == 1 {
			resp.Message = i18n.F("Покажите камере мишень с колеса (%d×%d углов) целиком.", boards[0].Cols, boards[0].Rows)
		} else {
			resp.Message = i18n.T("Покажите камере целиком любую мишень с колеса.")
		}
		return lo.calibProgress(dev, resp)
	}
	det := dets[found]
	resp.Found = []string{i18n.F("мишень %d×%d", tg.Cols, tg.Rows)}

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
		run = &calibRun{w: img.W, h: img.H, target: tg}
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

// ── Shared frame handling ─────────────────────────────────────────────

// liveInput is what a measuring frame needs from the state.
type liveInput struct {
	cam    vision.Camera
	boards []vision.Target
	wheels []align.Position // nil: one board, wheel from the phone
	refs   []vision.Target
}

func (lo *liveOptical) input(dev string, img *vision.Gray) (liveInput, string) {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	cam, ok := lo.cams[dev]
	if msg := cameraMismatch(cam, ok, img); msg != "" {
		return liveInput{}, msg
	}
	boards, wheels := lo.search()
	return liveInput{cam: cam, boards: boards, wheels: wheels, refs: append([]vision.Target(nil), lo.refs...)}, ""
}

// wheelsIn says which wheels a frame shows, as indices into f.Wheels. With a
// shared board the one found is the wheel the phone named.
func (in liveInput) wheelsIn(f vision.LiveFrame, named align.Position, hasNamed bool) map[align.Position]int {
	out := map[align.Position]int{}
	for i := range f.Wheels {
		if in.wheels == nil {
			if hasNamed {
				out[named] = i
			}
			continue
		}
		out[in.wheels[i]] = i
	}
	return out
}

// pick chooses the wheel a one-wheel mode works on: the one named, if it is in
// view, otherwise the only one in view.
func pick(ws map[align.Position]int, named align.Position, hasNamed bool) (align.Position, int, bool) {
	if i, ok := ws[named]; hasNamed && ok {
		return named, i, true
	}
	if len(ws) == 1 {
		for p, i := range ws {
			return p, i, true
		}
	}
	return 0, 0, false
}

// ── Runout compensation ───────────────────────────────────────────────

// frameSpin collects poses of one wheel's target while the wheel is turned,
// and fits the clamp calibration once the wheel has turned far enough.
func (s *Server) frameSpin(dev string, named align.Position, hasNamed bool, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	resp := LiveFrameResponse{Mode: modeSpin}
	in, msg := lo.input(dev, img)
	if msg != "" {
		resp.Message = msg
		return resp
	}
	f, err := vision.DetectLive(in.cam, in.boards, in.refs, img, vision.DetectOptions{})
	resp.Found = foundList(f, in)
	p, wi, ok := pick(in.wheelsIn(f, named, hasNamed), named, hasNamed)
	if err != nil || !ok {
		resp.Message = i18n.T("Мишень на колесе не видна — наведите камеру на колесо.")
		return resp
	}
	resp.Wheel = p.String()

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
		pose = f.Wheels[wi].Pose
	} else {
		var k int
		fmt.Sscanf(run.frame, "ref:%d", &k)
		if _, seen := f.Refs[k]; !seen {
			lo.mu.Unlock()
			resp.Message = i18n.T("Напольная мишень пропала из кадра — верните её в кадр или начните заново.")
			return resp
		}
		pose = f.InRef(wi, k)
		origin = f.Refs[k].Pose.Inverse().Apply(geom.V(0, 0, 0))
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

// placed is one wheel located in the root frame from one frame.
type placed struct {
	p            align.Position
	pose         geom.Pose // the wheel target, root frame
	axis, center geom.Vec3
	camber       float64
}

// place locates every wheel in a frame that can be located, and says why the
// others cannot.
func (s *Server) place(in liveInput, f vision.LiveFrame, named align.Position, hasNamed bool) ([]placed, string) {
	lo := s.optLive
	ws := in.wheelsIn(f, named, hasNamed)
	if len(ws) == 0 {
		return nil, ""
	}
	if f.Ref < 0 {
		return nil, i18n.T("Напольная мишень не видна — без неё угол не к чему привязать. Поверните камеру так, чтобы в кадре были и колесо, и мишень на полу.")
	}
	lo.mu.Lock()
	defer lo.mu.Unlock()
	k, root, ok := lo.rootFor(f)
	if !ok {
		return nil, i18n.T("Эта напольная мишень ещё не связана с остальными: снимите кадр, где видны обе мишени на полу.")
	}
	var out []placed
	var why string
	for _, p := range align.AllPositions {
		wi, in := ws[p]
		if !in {
			continue
		}
		clamp, ok := lo.clamps[p]
		if !ok {
			why = i18n.F("%s: сначала учтите биение — режим «Биение».", p.Label())
			continue
		}
		pose := root.Mul(f.InRef(wi, k))
		axis, center := clamp.Place(pose)
		camber := vision.CamberFromAxis(axis, geom.V(0, 0, 1))
		if math.Abs(camber) > maxCamberDeg {
			// No road wheel stands like that: a board read as the wrong
			// one, or a floor target not lying on the floor.
			why = i18n.F("%s: получился развал %.0f° — так колесо стоять не может. Проверьте, что мишени на полу лежат на полу, и что мишень колеса видна целиком.", p.Label(), camber)
			continue
		}
		out = append(out, placed{p: p, pose: pose, axis: axis, center: center, camber: camber})
	}
	return out, why
}

// frameLive places the wheels in one frame and publishes their angles.
func (s *Server) frameLive(dev string, named align.Position, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	resp := LiveFrameResponse{Mode: modeLive}
	in, msg := lo.input(dev, img)
	if msg != "" {
		resp.Message = msg
		return resp
	}
	hasNamed := in.wheels == nil
	f, err := vision.DetectLive(in.cam, in.boards, in.refs, img, vision.DetectOptions{})
	resp.Found = foundList(f, in)

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
			resp.Message = i18n.T("Мишень на колесе не видна — наведите камеру на колесо.")
		}
		return resp
	}
	ps, why := s.place(in, f, named, hasNamed)
	if len(ps) == 0 {
		resp.Message = why
		return resp
	}

	lo.mu.Lock()
	now := time.Now()
	for _, w := range ps {
		lo.seen[w.p] = seenWheel{axis: w.axis, center: w.center, at: now, camber: w.camber}
	}
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

	var res *align.Result
	if len(seen) == 4 {
		wheels := map[align.Position]vision.RegisteredWheel{}
		for q, v := range seen {
			wheels[q] = vision.RegisteredWheel{Axis: v.axis, Center: v.center, Used: 1}
		}
		if r, err := (measure.OpticalSession{Wheels: wheels, RimDiameterMM: align.Inches(rim)}).Result(); err == nil {
			res = &r
		}
	}

	var parts []string
	for _, w := range ps {
		c := round4(w.camber)
		var toe *float64
		if res != nil {
			if wr, ok := res.Wheels[w.p.String()]; ok {
				c = round4(wr.Camber.Deg())
				t := round4(wr.ToeGeometric.Deg())
				toe = &t
			}
		}
		s.hub.Touch(live.SourceInfo{ID: liveSource, Kind: "camera", Name: name, Wheel: w.p.String(),
			Detail: i18n.N("развал, схождение")})
		_ = s.hub.Push(live.Input{Wheel: w.p, Camber: &c, Toe: toe, Source: liveSource})
		if resp.Camber == nil {
			cc := c
			resp.Wheel, resp.Camber, resp.Toe = w.p.String(), &cc, toe
		}
		if toe != nil {
			parts = append(parts, i18n.F("%s: развал %+.2f°, схождение %+.2f°.", w.p.Label(), c, *toe))
		} else {
			parts = append(parts, i18n.F("%s: развал %+.2f°.", w.p.Label(), c))
		}
	}
	if res == nil {
		var missing []string
		for _, q := range align.AllPositions {
			if _, ok := seen[q]; !ok {
				missing = append(missing, q.Label())
			}
		}
		parts = append(parts, i18n.F("Схождение появится, когда камера увидит все четыре колеса — осталось: %s.", strings.Join(missing, ", ")))
	}
	resp.Message = strings.Join(parts, " ")
	resp.Warn = why
	return resp
}

// ── Caster ────────────────────────────────────────────────────────────

// frameCaster follows a front wheel while it is steered on its turn plate,
// with the brake pedal held down so that the wheel does not roll.
//
// Steering turns the whole wheel — and the target clamped to it — about the
// steering axis, so the axis of the target's own rotation from frame to frame
// IS the steering axis. That is found from rotations of 20° and more, which
// fixes it to a small fraction of a degree in every direction.
//
// The alternative that needs no brake — fitting the cone the spin axis sweeps
// (measure.SteeringAxisFromSweep) — is indifferent to the wheel rolling but
// weak exactly where it matters: over a ±20° sweep the cone is a short arc, and
// the tilt of its plane about the arc's chord, which is steering axis
// inclination, is barely constrained. A wheel that did roll shows up here
// instead as a large residual of the rotation fit, and is reported.
func (s *Server) frameCaster(dev string, named align.Position, hasNamed bool, img *vision.Gray) LiveFrameResponse {
	lo := s.optLive
	resp := LiveFrameResponse{Mode: modeCaster}
	in, msg := lo.input(dev, img)
	if msg != "" {
		resp.Message = msg
		return resp
	}
	f, err := vision.DetectLive(in.cam, in.boards, in.refs, img, vision.DetectOptions{})
	resp.Found = foundList(f, in)
	if err != nil {
		resp.Message = i18n.T("Мишень на колесе не видна — наведите камеру на колесо.")
		return resp
	}
	ps, why := s.place(in, f, named, hasNamed)
	var w placed
	found := false
	for _, x := range ps {
		if x.p.IsFront() && (!hasNamed || x.p == named || len(ps) == 1) {
			w, found = x, true
			break
		}
	}
	if !found {
		if why == "" {
			why = i18n.T("Кастер меряется на передних колёсах — наведите камеру на переднее колесо.")
		}
		resp.Message = why
		return resp
	}
	p := w.p
	resp.Wheel = p.String()
	up := geom.V(0, 0, 1)

	lo.mu.Lock()
	run := lo.casters[p]
	if run == nil {
		// Straight ahead is where the wheel was last measured, if that was
		// recently; otherwise where the sweep begins.
		straight := w.axis
		if v, ok := lo.seen[p]; ok && time.Since(v.at) < time.Hour {
			straight = v.axis
		}
		run = &casterRun{straight: straight}
		lo.casters[p] = run
	}
	steer := steerAngle(run.straight, w.axis, up, p)
	add := len(run.poses) == 0
	if !add {
		add = geom.Deg(math.Acos(math.Max(-1, math.Min(1, run.lastAxis.Dot(w.axis))))) >= casterStepDeg
	}
	if add {
		run.poses = append(run.poses, w.pose)
		run.lastAxis = w.axis
		run.lo, run.hi = math.Min(run.lo, steer), math.Max(run.hi, steer)
	}
	poses := append([]geom.Pose(nil), run.poses...)
	lo0, hi0, straight := run.lo, run.hi, run.straight
	lo.mu.Unlock()

	resp.Progress = math.Min(1, (math.Min(-lo0, casterMinSide)+math.Min(hi0, casterMinSide))/(2*casterMinSide))
	if -lo0 < casterMinSide || hi0 < casterMinSide || len(poses) < casterMinFrames {
		resp.Message = i18n.F("%s: поворачивайте руль в обе стороны — колесо повёрнуто от %+.0f° до %+.0f°, нужно хотя бы на %.0f° наружу и внутрь.",
			p.Label(), lo0, hi0, casterMinSide)
		return resp
	}

	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		resp.Message = fmt.Sprintf("%s: %v", p.Label(), err)
		return resp
	}
	k := fit.Direction
	if k.Dot(up) < 0 {
		k = k.Neg()
	}
	fwd := s.forward(p, straight, up)
	vx, vz := fwd, up
	vy := vz.Cross(vx).Unit()
	kv := geom.V(k.Dot(vx), k.Dot(vy), k.Dot(vz))
	caster, sai := align.Caster(kv).Deg(), align.SAIAngle(kv, p).Deg()

	lo.mu.Lock()
	lo.caster[p] = [2]float64{caster, sai}
	delete(lo.casters, p)
	lo.mu.Unlock()
	c, si := round4(caster), round4(sai)
	_ = s.hub.PushSweep(live.SweepInput{Wheel: p, Caster: caster, SAI: &si, Source: liveSource})
	resp.Done, resp.Progress = true, 1
	resp.Caster, resp.SAI = &c, &si
	resp.Message = i18n.F("%s: кастер %+.2f°, поперечный наклон оси %.1f°. Верните колёса прямо.", p.Label(), c, si)
	if fit.Residual > 5 {
		resp.Warn = i18n.F("Колесо, похоже, проворачивалось при повороте руля (разброс %.0f мм) — зафиксируйте педаль тормоза упором и повторите.", fit.Residual)
	}
	return resp
}

// steerAngle is how far a wheel is steered from straight ahead, positive
// outboard: the turn of its spin axis about the vertical.
func steerAngle(straight, now, up geom.Vec3, p align.Position) float64 {
	h := func(v geom.Vec3) geom.Vec3 { return v.Sub(up.Scale(v.Dot(up))).Unit() }
	a, b := h(straight), h(now)
	ang := geom.Deg(math.Atan2(up.Dot(a.Cross(b)), a.Dot(b)))
	// Steering the left wheel outboard (to the left) turns it anticlockwise
	// seen from above, a positive turn about up; the right wheel's outboard
	// steer is clockwise.
	return p.SideSign() * ang
}

// forward is the car's forward direction in the root frame, from a front
// wheel's straight-ahead spin axis — corrected by that wheel's toe when the
// whole car has been measured, since a wheel toed in by t points t off the
// car's centreline.
func (s *Server) forward(p align.Position, straight, up geom.Vec3) geom.Vec3 {
	sg := p.SideSign()
	x := straight.Cross(up).Scale(sg)
	x = x.Sub(up.Scale(x.Dot(up))).Unit()
	lo := s.optLive
	lo.mu.Lock()
	seen := map[align.Position]seenWheel{}
	for q, v := range lo.seen {
		seen[q] = v
	}
	rim := lo.rimIn
	lo.mu.Unlock()
	if len(seen) == 4 {
		wheels := map[align.Position]vision.RegisteredWheel{}
		for q, v := range seen {
			wheels[q] = vision.RegisteredWheel{Axis: v.axis, Center: v.center, Used: 1}
		}
		if res, err := (measure.OpticalSession{Wheels: wheels, RimDiameterMM: align.Inches(rim)}).Result(); err == nil {
			if wr, ok := res.Wheels[p.String()]; ok {
				x = geom.Rodrigues(up, sg*wr.ToeGeometric.Rad()).MulVec(x).Unit()
			}
		}
	}
	return x
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

func foundList(f vision.LiveFrame, in liveInput) []string {
	var out []string
	idx := make([]int, 0, len(f.Wheels))
	for i := range f.Wheels {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		if in.wheels != nil {
			out = append(out, in.wheels[i].Label())
		} else {
			out = append(out, i18n.T("колесо"))
		}
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
	WheelTargets map[string]vision.Target `json:"wheel_targets"`
	Distinct     bool                     `json:"distinct"`
	Refs         []vision.Target          `json:"refs"`
	RimIn        float64                  `json:"rim_in"`
	Cameras      []LiveCameraInfo         `json:"cameras"`
	Linked       []int                    `json:"linked"`
	Wheels       map[string]LiveWheelInfo `json:"wheels"`
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
	Caster    *float64 `json:"caster,omitempty"`
	SAI       *float64 `json:"sai,omitempty"`
}

func (s *Server) liveOpticalState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.optLive.state())
}

func (lo *liveOptical) state() LiveOpticalState {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	st := LiveOpticalState{WheelTargets: map[string]vision.Target{}, Distinct: lo.distinct(), Refs: lo.refs,
		RimIn: lo.rimIn, Cameras: []LiveCameraInfo{}, Linked: []int{}, Wheels: map[string]LiveWheelInfo{}}
	for p, t := range lo.wheels {
		st.WheelTargets[p.String()] = t
	}
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
		if cs, ok := lo.caster[p]; ok {
			c, si := round4(cs[0]), round4(cs[1])
			wi.Caster, wi.SAI = &c, &si
		}
		st.Wheels[p.String()] = wi
	}
	return st
}

// LiveOpticalSettings changes the live camera mode.
type LiveOpticalSettings struct {
	// WheelTargets sets each wheel's board, by position; WheelTarget sets one
	// board for all four (the phone then says which wheel it looks at).
	WheelTargets map[string]vision.Target `json:"wheel_targets,omitempty"`
	WheelTarget  *vision.Target           `json:"wheel_target,omitempty"`
	Refs         []vision.Target          `json:"refs,omitempty"`
	RimIn        float64                  `json:"rim_in,omitempty"`
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
	if req.WheelTarget != nil || len(req.WheelTargets) > 0 || len(req.Refs) > 0 {
		lo.mu.Lock()
		wheels := map[align.Position]vision.Target{}
		for p, t := range lo.wheels {
			wheels[p] = t
		}
		refs := lo.refs
		lo.mu.Unlock()
		if req.WheelTarget != nil {
			for _, p := range align.AllPositions {
				wheels[p] = *req.WheelTarget
			}
		}
		for k, t := range req.WheelTargets {
			found := false
			for _, p := range align.AllPositions {
				if strings.EqualFold(k, p.String()) {
					wheels[p], found = t, true
				}
			}
			if !found {
				writeErr(w, http.StatusBadRequest, errors.New(i18n.F("неизвестное колесо %q", k)))
				return
			}
		}
		if len(req.Refs) > 0 {
			refs = req.Refs
		}
		if err := checkTargets(wheels, refs); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		lo.mu.Lock()
		changed := len(refs) != len(lo.refs)
		for i := range refs {
			if !changed && refs[i] != lo.refs[i] {
				changed = true
			}
		}
		for _, p := range align.AllPositions {
			if wheels[p] != lo.wheels[p] {
				changed = true
			}
		}
		lo.wheels, lo.refs = wheels, refs
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

// checkTargets rejects a set of boards the camera could confuse: wheel boards
// must be either all the same or all different with none inside another, and
// no floor board may look like a wheel board.
func checkTargets(wheels map[align.Position]vision.Target, refs []vision.Target) error {
	for _, p := range align.AllPositions {
		if err := wheels[p].Validate(); err != nil {
			return fmt.Errorf("%s — %s: %w", i18n.T("мишень на колесе"), p.Label(), err)
		}
	}
	same := 0
	for i, p := range align.AllPositions {
		for _, q := range align.AllPositions[i+1:] {
			a, b := wheels[p], wheels[q]
			switch {
			case sameBoard(a, b):
				same++
			case insideBoard(a, b) || insideBoard(b, a):
				return errors.New(i18n.F("мишень %s (%d×%d) помещается внутри мишени %s (%d×%d) — если часть второй закроет шина, программа примет её за первую. Возьмите размеры, где одна длиннее, а другая выше, например 11×4, 10×5, 9×6, 8×7.",
					p.Label(), a.Cols, a.Rows, q.Label(), b.Cols, b.Rows))
			}
		}
	}
	if same != 0 && same != 6 {
		return errors.New(i18n.T("мишени на колёсах должны быть либо все одинаковые, либо все разные"))
	}
	for i, r := range refs {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%s: %w", i18n.F("напольная мишень %d", i+1), err)
		}
		for _, p := range align.AllPositions {
			if sameBoard(r, wheels[p]) {
				return errors.New(i18n.T("напольная мишень должна отличаться размером от мишени на колесе"))
			}
		}
		for j := range refs[:i] {
			if sameBoard(r, refs[j]) {
				return errors.New(i18n.T("напольные мишени должны различаться размером"))
			}
		}
	}
	return nil
}

func (lo *liveOptical) resetLocked() {
	lo.toRoot = map[int]geom.Pose{}
	lo.links = map[int][]geom.Pose{}
	lo.clamps = map[align.Position]vision.ClampCalibration{}
	lo.spins = map[align.Position]*spinRun{}
	lo.casters = map[align.Position]*casterRun{}
	lo.caster = map[align.Position][2]float64{}
	lo.seen = map[align.Position]seenWheel{}
	lo.calib = map[string]*calibRun{}
}

// ── Printing the targets ──────────────────────────────────────────────

// targetsPDF returns the boards to print as a PDF at exact size: the four
// wheel boards and the floor boards of the live mode (which=live, wheels,
// floor), or one board given in the query (which=one: cols, rows, square).
func (s *Server) targetsPDF(w http.ResponseWriter, r *http.Request) {
	items, _, paper, err := s.printItems(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	doc, err := vision.TargetsPDF(items, paper)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="wheelalign-targets.pdf"`)
	_, _ = w.Write(doc)
}

// TargetPlanInfo is how one board will print. Label is what the sheet itself
// says (ASCII: the PDF has no Cyrillic font); Name is for the interface.
type TargetPlanInfo struct {
	Label  string           `json:"label"`
	Name   string           `json:"name"`
	Target vision.Target    `json:"target"`
	Plan   vision.PrintPlan `json:"plan"`
	Error  string           `json:"error,omitempty"`
}

// targetsPlan says how many sheets each board takes on the chosen paper.
func (s *Server) targetsPlan(w http.ResponseWriter, r *http.Request) {
	items, names, paper, err := s.printItems(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	out := make([]TargetPlanInfo, 0, len(items))
	for i, it := range items {
		pl, err := vision.Plan(it.Target, paper)
		info := TargetPlanInfo{Label: it.Label, Name: names[i], Target: it.Target, Plan: pl}
		if err != nil {
			info.Error = err.Error()
		}
		out = append(out, info)
	}
	writeJSON(w, http.StatusOK, out)
}

// printItems lists the boards to print, with the interface name of each.
func (s *Server) printItems(r *http.Request) ([]vision.PrintItem, []string, vision.Paper, error) {
	q := r.URL.Query()
	paper, ok := vision.Papers[strings.ToLower(q.Get("paper"))]
	if !ok {
		paper = vision.Papers["a4"]
	}
	var items []vision.PrintItem
	var names []string
	add := func(label, name string, t vision.Target) {
		items = append(items, vision.PrintItem{Label: label, Target: t})
		names = append(names, name)
	}
	which := q.Get("which")
	if which == "one" {
		t := vision.Target{Cols: formIntQ(q.Get("cols")), Rows: formIntQ(q.Get("rows")), SquareMM: formFloatQ(q.Get("square"))}
		if err := t.Validate(); err != nil {
			return nil, nil, paper, err
		}
		add("TARGET", i18n.T("Мишень"), t)
		return items, names, paper, nil
	}
	lo := s.optLive
	lo.mu.Lock()
	defer lo.mu.Unlock()
	if which == "" || which == "live" || which == "wheels" {
		if lo.distinct() {
			for _, p := range align.AllPositions {
				add("WHEEL "+p.String(), p.Label(), lo.wheels[p])
			}
		} else {
			add("WHEEL (all four)", i18n.T("Все четыре колеса"), lo.wheels[align.FL])
		}
	}
	if which == "" || which == "live" || which == "floor" {
		for i, t := range lo.refs {
			name := i18n.T("Напольная передняя")
			if i > 0 {
				name = i18n.T("Напольная задняя")
			}
			add(fmt.Sprintf("FLOOR %d", i+1), name, t)
		}
	}
	if len(items) == 0 {
		return nil, nil, paper, errors.New(i18n.T("не выбрано ни одной мишени"))
	}
	return items, names, paper, nil
}

func formIntQ(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func formFloatQ(s string) float64 {
	var v float64
	fmt.Sscanf(strings.ReplaceAll(s, ",", "."), "%g", &v)
	return v
}
