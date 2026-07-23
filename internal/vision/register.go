package vision

import (
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Registration ties separately-photographed wheels into one coordinate system.
//
// A single camera can only look at one wheel at a time, and each photograph
// gives that wheel's target pose in the camera's own frame — a frame that moves
// when the operator walks to the next wheel. Camber survives that, because it is
// measured against gravity and needs no vehicle frame. Toe and thrust do not:
// they are relationships *between* wheels, so every wheel has to be expressed in
// one frame that stays put.
//
// The fix is a second, fixed board lying flat on the floor, in shot alongside
// the wheel target in every frame. Both boards are detected in the same image,
// so each frame yields the wheel's pose relative to the reference rather than
// relative to the camera:
//
//	T_ref→wheel = (T_cam→ref)⁻¹ · T_cam→wheel
//
// The camera drops out entirely — it may move freely between frames and between
// wheels. And because the reference board lies on the floor, its own plane is
// the road plane: the reference frame's +Z is the road normal, and a wheel
// centre's height in that frame is its loaded rolling radius, measured rather
// than assumed.

// RegisteredWheel is one wheel's axis of rotation expressed in the fixed
// reference frame.
type RegisteredWheel struct {
	// Axis is the unit spin axis in the reference frame. Its sign is NOT
	// resolved to outboard here: which way is outboard is a fact about the
	// vehicle, not about this wheel, and it is settled once all four wheels are
	// known.
	Axis geom.Vec3

	// Center is a point on the spin axis in the reference frame, nearest the
	// target's own centre — the practical stand-in for the wheel centre.
	Center geom.Vec3

	RunoutDeg      float64
	SweepDeg       float64
	AxisResidualMM float64

	Frames   []FrameResult
	Used     int
	Warnings []string
}

// RegisterWheel recovers one wheel's spin axis in the reference frame, from
// frames that each show both the wheel target and the fixed floor reference.
//
// The wheel is turned between frames exactly as for the camber measurement; what
// is new is that every pose is referred to the floor board before the axis is
// fitted, so the camera is free to move.
func RegisterWheel(cam Camera, wheelTarget, refTarget Target, imgs []*Gray, opt DetectOptions) (RegisteredWheel, error) {
	if err := wheelTarget.Validate(); err != nil {
		return RegisteredWheel{}, fmt.Errorf("мишень на колесе: %w", err)
	}
	if err := refTarget.Validate(); err != nil {
		return RegisteredWheel{}, fmt.Errorf("напольная мишень: %w", err)
	}
	if sameLayout(wheelTarget, refTarget) {
		return RegisteredWheel{}, fmt.Errorf(
			"мишень на колесе и напольная мишень одинаковые (%dx%d) — их невозможно различить в кадре. "+
				"Напечатайте напольную другого размера, например %dx%d",
			wheelTarget.Cols, wheelTarget.Rows, wheelTarget.Cols-2, wheelTarget.Rows)
	}

	var res RegisteredWheel
	var poses []geom.Pose
	var normals []geom.Vec3
	targets := []Target{wheelTarget, refTarget}

	for i, img := range imgs {
		fr := FrameResult{Index: i}
		dets, errs := DetectBoards(img, targets, opt)

		switch {
		case errs[0] != nil && errs[1] != nil:
			fr.Error = "не найдены обе мишени: " + errs[0].Error()
		case errs[0] != nil:
			fr.Error = "не найдена мишень на колесе: " + errs[0].Error()
		case errs[1] != nil:
			fr.Error = "не найдена напольная мишень: " + errs[1].Error()
		}
		if fr.Error != "" {
			res.Frames = append(res.Frames, fr)
			continue
		}

		wheelPose, err := solveBoard(cam, wheelTarget, dets[0])
		if err != nil {
			fr.Error = "мишень на колесе: " + err.Error()
			res.Frames = append(res.Frames, fr)
			continue
		}
		refPose, err := solveBoard(cam, refTarget, dets[1])
		if err != nil {
			fr.Error = "напольная мишень: " + err.Error()
			res.Frames = append(res.Frames, fr)
			continue
		}

		// Refer the wheel to the floor board; the camera cancels out.
		inRef := refPose.Pose.Inverse().Mul(wheelPose.Pose)

		fr.OK = true
		fr.RMSPx = maxf(wheelPose.RMSPx, refPose.RMSPx)
		fr.Ambiguous = wheelPose.Ambiguous || refPose.Ambiguous
		res.Frames = append(res.Frames, fr)
		poses = append(poses, inRef)
		normals = append(normals, inRef.R.Col(2))
	}

	res.Used = len(poses)
	if len(poses) < 3 {
		return res, fmt.Errorf("%w: годных кадров %d из %d — нужно минимум 3, где видны обе мишени и колесо провёрнуто между ними",
			ErrTooFewViews, len(poses), len(imgs))
	}

	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		return res, fmt.Errorf("не удалось восстановить ось вращения колеса: %w", err)
	}
	res.Axis = fit.Direction
	res.Center = fit.Center
	res.SweepDeg = geom.Deg(fit.Sweep)
	res.AxisResidualMM = fit.Residual
	res.RunoutDeg = geom.Deg(meanTiltTo(normals, fit.Direction))
	res.Warnings = registerWarnings(res, cam)
	return res, nil
}

func sameLayout(a, b Target) bool {
	return (a.Cols == b.Cols && a.Rows == b.Rows) || (a.Cols == b.Rows && a.Rows == b.Cols)
}

func solveBoard(cam Camera, t Target, det Detection) (PnPResult, error) {
	corr, err := t.Correspondences(det.Corners)
	if err != nil {
		return PnPResult{}, err
	}
	return SolvePnPPlanar(cam, corr)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func registerWarnings(res RegisteredWheel, cam Camera) []string {
	var out []string
	if res.SweepDeg < 15 {
		out = append(out, fmt.Sprintf(
			"Колесо провёрнуто всего на %.0f° за серию — ось определена ненадёжно. Нужно 30–90°.", res.SweepDeg))
	}
	if res.RunoutDeg > 6 {
		out = append(out, fmt.Sprintf(
			"Мишень стоит на колесе с перекосом %.0f°. Компенсация биения это учтёт, но перекос лучше уменьшить.",
			res.RunoutDeg))
	}
	if res.AxisResidualMM > 5 {
		out = append(out, fmt.Sprintf(
			"Точка на оси «плавает» на %.1f мм — колесо вращалось не вокруг жёсткой оси. "+
				"Проверьте ступичный подшипник.", res.AxisResidualMM))
	}
	for _, f := range res.Frames {
		if f.Ambiguous {
			out = append(out, fmt.Sprintf(
				"Кадр %d: поза мишени неоднозначна — снимайте ближе или под большим углом.", f.Index+1))
		}
	}
	return out
}
