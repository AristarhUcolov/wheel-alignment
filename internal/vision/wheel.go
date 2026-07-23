package vision

import (
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// FrameResult is one processed photograph of a wheel target.
type FrameResult struct {
	Index     int     `json:"index"`
	OK        bool    `json:"ok"`
	RMSPx     float64 `json:"rms_px,omitempty"`
	Ambiguous bool    `json:"ambiguous,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// SpinAxisResult is a wheel's axis of rotation recovered from several frames of
// it being turned, together with everything needed to judge the measurement.
type SpinAxisResult struct {
	// Axis is the unit spin axis in the CAMERA frame. Its sign has been resolved
	// to point outboard — away from the camera — on the assumption that the
	// camera looks at the outboard face of the wheel, which is the only place a
	// person can stand to photograph it.
	Axis geom.Vec3 `json:"-"`

	// RunoutDeg is how far the target sits out of the wheel's plane: the angle
	// between the target's own normal and the recovered spin axis. This is the
	// mounting error the runout compensation exists to cancel, and its size is
	// worth knowing on its own — a target clamped badly enough wobbles so much
	// that individual frames become imprecise even though their average is right.
	RunoutDeg float64 `json:"runout_deg"`

	// SweepDeg is the total rotation observed across the frames. Too little and
	// the axis is poorly determined; this is the number that says whether the
	// wheel was turned enough.
	SweepDeg float64 `json:"sweep_deg"`

	// AxisResidualMM is how far the supposedly-fixed point on the axis actually
	// wandered — a direct check that the wheel turned about a fixed axis rather
	// than rocking on a loose bearing.
	AxisResidualMM float64 `json:"axis_residual_mm"`

	Frames   []FrameResult `json:"frames"`
	Used     int           `json:"used"`
	Warnings []string      `json:"warnings,omitempty"`
}

// WheelSpinAxis processes a set of photographs of one wheel being turned and
// recovers its spin axis.
//
// Each frame is detected and solved for the target's pose; the sequence of
// poses shares one axis of rotation — the wheel's — whatever angle the target
// was clamped at, and geom.FitRotationAxis recovers it. This is the whole point
// of the optical method over a bubble gauge: the target does not need to be
// mounted accurately, because it is the axis it turns *about* that is measured,
// not the target itself.
func WheelSpinAxis(cam Camera, target Target, imgs []*Gray, opt DetectOptions) (SpinAxisResult, error) {
	if err := target.Validate(); err != nil {
		return SpinAxisResult{}, err
	}
	opt.Target = target

	var res SpinAxisResult
	var poses []geom.Pose
	var normals []geom.Vec3

	for i, img := range imgs {
		fr := FrameResult{Index: i}
		_, pnp, err := DetectAndSolve(img, cam, opt)
		if err != nil {
			fr.Error = err.Error()
			res.Frames = append(res.Frames, fr)
			continue
		}
		fr.OK = true
		fr.RMSPx = pnp.RMSPx
		fr.Ambiguous = pnp.Ambiguous
		res.Frames = append(res.Frames, fr)
		poses = append(poses, pnp.Pose)
		normals = append(normals, pnp.Pose.R.Col(2))
	}

	res.Used = len(poses)
	if len(poses) < 3 {
		return res, fmt.Errorf("%w: распознано только %d кадров из %d, а для оси нужно минимум 3 с поворотом колеса между ними",
			ErrTooFewViews, len(poses), len(imgs))
	}

	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		return res, fmt.Errorf("не удалось восстановить ось вращения: %w", err)
	}

	axis := fit.Direction
	// Resolve the sign to point outboard, toward the camera. The camera sits at
	// the origin looking down +Z, so a vector pointing back toward it has a
	// negative Z component.
	if axis.Z > 0 {
		axis = axis.Neg()
	}
	res.Axis = axis
	res.SweepDeg = geom.Deg(fit.Sweep)
	res.AxisResidualMM = fit.Residual

	res.RunoutDeg = geom.Deg(meanTiltTo(normals, axis))

	res.Warnings = spinAxisWarnings(res, cam)
	return res, nil
}

func spinAxisWarnings(res SpinAxisResult, cam Camera) []string {
	var out []string
	if res.SweepDeg < 15 {
		out = append(out, fmt.Sprintf(
			"Колесо повёрнуто всего на %.0f° за всю серию — ось определена ненадёжно. "+
				"Прокатите колесо так, чтобы между кадрами оно провернулось в сумме на 30–90°.", res.SweepDeg))
	}
	if res.RunoutDeg > 6 {
		out = append(out, fmt.Sprintf(
			"Мишень стоит на колесе с большим перекосом (%.0f°). Компенсация биения это учитывает, "+
				"но такой перекос стоит уменьшить — иначе отдельные кадры сильно «гуляют».", res.RunoutDeg))
	}
	if res.AxisResidualMM > 5 {
		out = append(out, fmt.Sprintf(
			"Точка на оси «плавает» на %.1f мм — колесо вращалось не вокруг жёсткой оси. "+
				"Проверьте ступичный подшипник и что колесо не качается на вывешенной подвеске.", res.AxisResidualMM))
	}
	worstRMS := 0.0
	for _, f := range res.Frames {
		if f.OK && f.RMSPx > worstRMS {
			worstRMS = f.RMSPx
		}
		if f.Ambiguous {
			out = append(out, fmt.Sprintf(
				"Кадр %d: поза мишени неоднозначна — снимайте под большим углом или ближе.", f.Index+1))
		}
	}
	out = append(out, cam.Warnings()...)
	return out
}

// meanTiltTo is the average angle between a set of plane normals and an axis,
// folded onto the axis because a plane's normal has no inherent sign.
//
// For a target clamped to a wheel this is the mounting error: the target's
// normal makes a constant angle with the spin axis and precesses around it as
// the wheel turns. Runout compensation removes that error from the result, but
// its size is worth reporting on its own — a badly clamped target swings so far
// between frames that each individual frame becomes imprecise.
func meanTiltTo(normals []geom.Vec3, axis geom.Vec3) float64 {
	if len(normals) == 0 {
		return 0
	}
	var sum float64
	for _, n := range normals {
		a := n.AngleTo(axis)
		if a > math.Pi/2 {
			a = math.Pi - a
		}
		sum += a
	}
	return sum / float64(len(normals))
}

// CamberFromSpinAxis returns the camber angle from an outboard-pointing spin
// axis and the direction of gravity, both in the same frame.
//
// It needs no vehicle coordinate system at all: camber is the angle between the
// wheel and the vertical the tyre stands against, and that is captured entirely
// by the spin axis and gravity. Formally camber = asin(outboardAxis · gravity),
// where gravity points DOWN — the same quantity align.Camber computes in the
// vehicle frame, here freed of the frame.
//
// gravityDown is the measured direction of gravity in the camera frame: for a
// camera levelled on a tripod it is straight down in the image, (0, 1, 0) in the
// +Y-down camera convention. A phone can report it from its accelerometer.
func CamberFromSpinAxis(outboardAxis, gravityDown geom.Vec3) float64 {
	a := outboardAxis.Unit()
	g := gravityDown.Unit()
	return geom.Deg(math.Asin(math.Max(-1, math.Min(1, a.Dot(g)))))
}
