package vision

import (
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// Live measurement: a camera watching one wheel continuously.
//
// The photo series answers "where is this wheel's axis?" once. Adjusting needs
// the answer several times a second while a tie rod is turned, and that splits
// the problem in two, exactly as on a commercial 3D aligner:
//
//  1. Runout compensation, once per clamping. The wheel is turned through a few
//     positions and the spin axis is fitted — but kept in the TARGET's own
//     frame. The target is clamped rigidly to the rim, so where the axis sits
//     relative to the target is a property of that clamping and does not change
//     until the clamp is moved. That is the ClampCalibration.
//  2. Live frames. From then on a single frame is enough: the target's pose
//     carries the stored axis into the world. No turning, no series — one image,
//     one axis, as fast as frames can be detected.
//
// The floor target in the same frame makes the camera irrelevant, as in the
// photo registration: the wheel's pose is referred to the floor, so the phone
// on its stand may be nudged without the reading moving.

// ClampCalibration is where a wheel's spin axis lies relative to the target
// clamped to that wheel.
type ClampCalibration struct {
	// Axis is the unit spin axis in the target's own frame, pointing outboard —
	// towards the side the camera watched from, which is the only side a camera
	// can see a wheel target from.
	Axis geom.Vec3 `json:"axis"`
	// Center is a point on the axis in the target's frame: the wheel centre
	// plus the clamp's axial offset, which cancels across an axle.
	Center geom.Vec3 `json:"center"`

	RunoutDeg  float64 `json:"runout_deg"`
	SweepDeg   float64 `json:"sweep_deg"`
	ResidualMM float64 `json:"residual_mm"`
	Frames     int     `json:"frames"`
}

// ErrSweepTooSmall means the wheel was not turned far enough to find its axis.
var ErrSweepTooSmall = i18n.Err("колесо провёрнуто слишком мало — нужно не меньше 30° между крайними кадрами")

// ClampFromPoses fits a clamp calibration from poses of the wheel target, all
// expressed in one fixed frame (the camera's, if it stood still, or a floor
// target's), taken while the wheel was turned about its own axis. camOrigin is
// the camera position in that same frame for one of the poses, which fixes
// which way is outboard.
func ClampFromPoses(poses []geom.Pose, camOrigin geom.Vec3) (ClampCalibration, error) {
	if len(poses) < 3 {
		return ClampCalibration{}, fmt.Errorf(i18n.T("%w: годных кадров %d — нужно минимум 3"), ErrTooFewViews, len(poses))
	}
	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		return ClampCalibration{}, fmt.Errorf(i18n.T("не удалось восстановить ось вращения колеса: %w"), err)
	}
	if geom.Deg(fit.Sweep) < 30 {
		return ClampCalibration{}, ErrSweepTooSmall
	}
	dir := fit.Direction
	// Outboard is towards the camera.
	if dir.Dot(camOrigin.Sub(fit.Center)) < 0 {
		dir = dir.Neg()
	}

	// Carry the axis into the target frame of every pose and average: for a
	// rigid clamping these agree, and their spread is the fit's own check.
	var axT, cT geom.Vec3
	var tilt float64
	for _, p := range poses {
		rt := p.R.T()
		axT = axT.Add(rt.MulVec(dir))
		cT = cT.Add(rt.MulVec(fit.Center.Sub(p.T)))
		tilt += math.Acos(math.Max(-1, math.Min(1, math.Abs(p.R.Col(2).Dot(dir)))))
	}
	n := float64(len(poses))
	return ClampCalibration{
		Axis:       axT.Unit(),
		Center:     cT.Scale(1 / n),
		RunoutDeg:  geom.Deg(tilt / n),
		SweepDeg:   geom.Deg(fit.Sweep),
		ResidualMM: fit.Residual,
		Frames:     len(poses),
	}, nil
}

// Place carries the clamp's axis and centre into the frame a target pose is
// given in.
func (c ClampCalibration) Place(target geom.Pose) (axis, center geom.Vec3) {
	return target.R.MulVec(c.Axis).Unit(), target.Apply(c.Center)
}

// LiveFrame is what one image of the wheels and the floor says.
type LiveFrame struct {
	// Wheels holds the pose, in the camera frame, of every wheel target
	// found — by index into the list of wheel targets searched for.
	Wheels map[int]PnPResult
	// Refs holds the pose of every floor target found, by index.
	Refs map[int]PnPResult
	// Ref is the best-fitting floor target found, or −1 when none was.
	Ref int
}

// InRef is wheel target w in floor target r's frame.
func (f LiveFrame) InRef(w, r int) geom.Pose {
	return f.Refs[r].Pose.Inverse().Mul(f.Wheels[w].Pose)
}

// ErrNoWheelTarget is a frame without a readable wheel target.
var ErrNoWheelTarget = i18n.Err("мишень на колесе не найдена в кадре")

// DetectLive finds wheel targets and floor targets in one image.
//
// With a different target on each wheel the list holds all four, and which
// were found says which wheels are in view — the camera needs no telling. The
// layouts are chosen so that no board is a part of another, so a board half
// hidden behind a tyre cannot pass for a different wheel's; and every wheel
// board has more corners than a floor board, so it is found first and its
// corners taken out before a floor board is looked for.
//
// Floor targets are reported even when no wheel target is found: a frame
// showing two floor targets and no wheel is exactly how the floor targets get
// tied to one another. The error then says no wheel was found, and Refs still
// holds what was.
func DetectLive(cam Camera, wheels, refs []Target, img *Gray, opt DetectOptions) (LiveFrame, error) {
	targets := append(append([]Target{}, wheels...), refs...)
	dets, errs := DetectBoards(img, targets, opt)
	f := LiveFrame{Wheels: map[int]PnPResult{}, Refs: map[int]PnPResult{}, Ref: -1}
	best := math.Inf(1)
	for i := range refs {
		k := len(wheels) + i
		if errs[k] != nil {
			continue
		}
		r, err := solveBoard(cam, refs[i], dets[k])
		if err != nil {
			continue
		}
		f.Refs[i] = r
		if r.RMSPx < best {
			best, f.Ref = r.RMSPx, i
		}
	}
	var firstErr error
	for i := range wheels {
		if errs[i] != nil {
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		w, err := solveBoard(cam, wheels[i], dets[i])
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		f.Wheels[i] = w
	}
	if len(f.Wheels) == 0 {
		return f, fmt.Errorf("%w: %v", ErrNoWheelTarget, firstErr)
	}
	return f, nil
}

// CamberFromAxis is the camber of a wheel whose outboard spin axis is given,
// against the up direction of the same frame: positive when the top of the
// wheel leans out, which tips the outboard end of the axis down.
func CamberFromAxis(outboard, up geom.Vec3) float64 {
	s := -outboard.Unit().Dot(up.Unit())
	return geom.Deg(math.Asin(math.Max(-1, math.Min(1, s))))
}
