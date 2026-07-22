package vision

import (
	"errors"
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/numeric"
)

var (
	ErrTooFewViews           = errors.New("vision: для калибровки нужно не менее 3 снимков мишени")
	ErrDegenerateCalibration = errors.New("vision: снимки не определяют параметры камеры")
)

// CalibrationView is one photograph of the calibration board, already detected.
type CalibrationView struct {
	Corners []Point2
	Label   string // file name or similar, for reporting
}

// CalibrateOptions tunes calibration.
type CalibrateOptions struct {
	// FixTangential drops the two tangential distortion terms. Worth doing for
	// a well-centred lens with few views: they are weakly observed and mostly
	// soak up noise.
	FixTangential bool
	// FixK3 drops the sixth-order radial term, which is only identifiable with
	// a strong fisheye and plenty of coverage.
	FixK3 bool
}

// CalibrationResult is a finished calibration with everything needed to judge it.
type CalibrationResult struct {
	Camera Camera

	// RMSPx is the overall reprojection error. Below roughly 0.3 px is a
	// healthy calibration with a decent camera; above 1 px means something went
	// wrong with the procedure, not with the maths.
	RMSPx        float64
	PerViewRMSPx []float64

	// Poses are where the board was in each view, which is useful for showing
	// the operator that the shots were varied enough.
	Poses []geom.Pose

	// CoverageFraction is how much of the frame the corners between them
	// touched. Distortion is only constrained where there is data, so a
	// calibration built from shots that all sat in the middle of the frame will
	// be confidently wrong at the edges — where the wheels are.
	CoverageFraction float64
	// TiltSpreadDeg is how much the board's orientation varied. Without varied
	// tilt, focal length and distance trade off against each other and neither
	// is determined.
	TiltSpreadDeg float64

	Warnings []string
}

// CalibrateCamera recovers a camera's intrinsics and distortion from several
// views of a planar target, by Zhang's method.
//
// # The idea
//
// Each view gives a homography H from the board's plane to the image. Because
// the board is planar, H = λ·K·[r₁ r₂ t], and the first two columns of a
// rotation matrix are orthonormal. Those two facts —
//
//	r₁ᵀr₂ = 0   and   r₁ᵀr₁ = r₂ᵀr₂
//
// — translate into two linear constraints per view on B = K⁻ᵀK⁻¹, a symmetric
// matrix with six unknowns. Three views therefore over-determine B, K falls out
// of B in closed form, the poses follow from K and each H, and the whole lot is
// then refined together against true reprojection error with distortion in the
// model.
//
// # Why the warnings matter more than the numbers
//
// Calibration fails quietly. A set of photographs all taken face-on, or all
// with the board in the middle of the frame, yields a plausible focal length
// and a reprojection error that looks excellent — while the distortion
// coefficients, which were never constrained by any data, are nonsense at the
// edges of the frame. That is exactly where wheel targets sit. So this function
// measures how varied the views actually were and says so.
func CalibrateCamera(target Target, views []CalibrationView, width, height int, opt CalibrateOptions) (CalibrationResult, error) {
	if err := target.Validate(); err != nil {
		return CalibrationResult{}, err
	}
	if len(views) < 3 {
		return CalibrationResult{}, fmt.Errorf("%w (дано %d)", ErrTooFewViews, len(views))
	}
	model := target.ModelPoints()
	modelXY := make([]Point2, len(model))
	for i, m := range model {
		modelXY[i] = Point2{X: m.X, Y: m.Y}
	}
	for i, v := range views {
		if len(v.Corners) != len(model) {
			return CalibrationResult{}, fmt.Errorf("снимок %d: %d углов, а мишень требует %d",
				i+1, len(v.Corners), len(model))
		}
	}

	// ── Step 1: a homography per view ───────────────────────────────────────
	hs := make([]geom.Mat3, len(views))
	for i, v := range views {
		h, err := homographyDLT(modelXY, v.Corners)
		if err != nil {
			return CalibrationResult{}, fmt.Errorf("снимок %d: %w", i+1, err)
		}
		hs[i] = h
	}

	// ── Step 2: the linear constraints on B = K⁻ᵀK⁻¹ ────────────────────────
	//
	// Writing hᵢ for the i-th COLUMN of H, orthonormality of the rotation's
	// first two columns gives h₁ᵀBh₂ = 0 and h₁ᵀBh₁ − h₂ᵀBh₂ = 0. Both are
	// linear in the six independent entries of B.
	v := numeric.New(2*len(hs), 6)
	for i, h := range hs {
		v12 := zhangConstraint(h, 0, 1)
		v11 := zhangConstraint(h, 0, 0)
		v22 := zhangConstraint(h, 1, 1)
		for j := 0; j < 6; j++ {
			v.Set(2*i, j, v12[j])
			v.Set(2*i+1, j, v11[j]-v22[j])
		}
	}
	b, err := numeric.NullVector(v)
	if err != nil {
		return CalibrationResult{}, err
	}

	k, err := intrinsicsFromB(b)
	if err != nil {
		return CalibrationResult{}, err
	}
	cam := Camera{
		Width: width, Height: height,
		Fx: k.fx, Fy: k.fy, Cx: k.cx, Cy: k.cy,
	}
	if err := cam.Validate(); err != nil {
		return CalibrationResult{}, fmt.Errorf("%w: линейное решение дало невозможные параметры (%v)",
			ErrDegenerateCalibration, err)
	}

	// ── Step 3: poses from K and each homography ────────────────────────────
	poses := make([]geom.Pose, len(hs))
	for i, h := range hs {
		p, err := poseFromHomographyK(h, cam)
		if err != nil {
			return CalibrationResult{}, fmt.Errorf("снимок %d: %w", i+1, err)
		}
		poses[i] = p
	}

	// ── Step 4: refine everything together ──────────────────────────────────
	cam, poses, rms, err := refineCalibration(cam, poses, model, views, opt)
	if err != nil {
		return CalibrationResult{}, err
	}
	cam.Calibrated = true
	cam.CalibrationRMSPx = rms
	cam.CalibrationNote = fmt.Sprintf("Калибровка по %d снимкам мишени %dx%d, клетка %.1f мм",
		len(views), target.Cols, target.Rows, target.SquareMM)

	res := CalibrationResult{Camera: cam, RMSPx: rms, Poses: poses}
	res.PerViewRMSPx = make([]float64, len(views))
	for i, view := range views {
		var ss float64
		for j, m := range model {
			px, ok := cam.Project(poses[i].Apply(m))
			if !ok {
				ss = math.Inf(1)
				break
			}
			d := px.DistTo(view.Corners[j])
			ss += d * d
		}
		res.PerViewRMSPx[i] = math.Sqrt(ss / float64(len(model)))
	}

	res.CoverageFraction = frameCoverage(views, width, height)
	res.TiltSpreadDeg = tiltSpread(poses)
	res.Warnings = calibrationWarnings(res, len(views))
	return res, nil
}

// zhangConstraint builds the vector vᵢⱼ such that vᵢⱼᵀ·b = hᵢᵀ·B·hⱼ, where hᵢ is
// the i-th column of H and b holds the six independent entries of the symmetric
// matrix B in the order (B₁₁, B₁₂, B₂₂, B₁₃, B₂₃, B₃₃).
func zhangConstraint(h geom.Mat3, i, j int) [6]float64 {
	a, c := h.Col(i), h.Col(j)
	return [6]float64{
		a.X * c.X,
		a.X*c.Y + a.Y*c.X,
		a.Y * c.Y,
		a.Z*c.X + a.X*c.Z,
		a.Z*c.Y + a.Y*c.Z,
		a.Z * c.Z,
	}
}

type intrinsics struct{ fx, fy, cx, cy float64 }

// intrinsicsFromB extracts the camera matrix from B = K⁻ᵀK⁻¹ in closed form.
//
// Skew is taken as zero. Every sensor made in the last forty years has square,
// axis-aligned pixels, and leaving skew free lets it absorb noise that belongs
// to the focal lengths.
func intrinsicsFromB(b []float64) (intrinsics, error) {
	B11, B12, B22, B13, B23, B33 := b[0], b[1], b[2], b[3], b[4], b[5]

	den := B11*B22 - B12*B12
	if math.Abs(den) < 1e-18 {
		return intrinsics{}, fmt.Errorf("%w: вырожденная система", ErrDegenerateCalibration)
	}
	cy := (B12*B13 - B11*B23) / den
	lambda := B33 - (B13*B13+cy*(B12*B13-B11*B23))/B11
	if lambda/B11 <= 0 {
		// B is determined only up to sign; the wrong one makes the focal
		// lengths imaginary.
		B11, B12, B22, B13, B23, B33 = -B11, -B12, -B22, -B13, -B23, -B33
		den = B11*B22 - B12*B12
		cy = (B12*B13 - B11*B23) / den
		lambda = B33 - (B13*B13+cy*(B12*B13-B11*B23))/B11
		if lambda/B11 <= 0 {
			return intrinsics{}, fmt.Errorf("%w: фокусное расстояние получилось мнимым — "+
				"снимки, вероятно, слишком похожи друг на друга", ErrDegenerateCalibration)
		}
	}
	fx := math.Sqrt(lambda / B11)
	fyArg := lambda * B11 / den
	if fyArg <= 0 {
		return intrinsics{}, fmt.Errorf("%w: фокусное расстояние по вертикали мнимое", ErrDegenerateCalibration)
	}
	fy := math.Sqrt(fyArg)
	cx := -B13 * fx * fx / lambda
	return intrinsics{fx: fx, fy: fy, cx: cx, cy: cy}, nil
}

// poseFromHomographyK recovers a board pose from its homography and known
// intrinsics. Same decomposition as poseFromHomography, but the homography here
// maps to pixels rather than to normalised coordinates, so K must be divided
// out first.
func poseFromHomographyK(h geom.Mat3, cam Camera) (geom.Pose, error) {
	kInv := geom.Mat3{
		{1 / cam.Fx, 0, -cam.Cx / cam.Fx},
		{0, 1 / cam.Fy, -cam.Cy / cam.Fy},
		{0, 0, 1},
	}
	return poseFromHomography(kInv.Mul(h))
}

// refineCalibration jointly optimises the intrinsics, the distortion and every
// board pose against total reprojection error.
//
// This is where distortion enters at all: the linear steps assume a pinhole,
// which is why their result is only a starting point. Everything is optimised
// at once because the parameters are strongly coupled — barrel distortion and a
// shorter focal length produce nearly the same image, and only the poses'
// consistency across views separates them.
func refineCalibration(cam Camera, poses []geom.Pose, model []geom.Vec3, views []CalibrationView, opt CalibrateOptions) (Camera, []geom.Pose, float64, error) {
	nView := len(views)
	nDist := 5
	if opt.FixK3 {
		nDist = 4
	}
	if opt.FixTangential {
		nDist = 2
		if !opt.FixK3 {
			nDist = 3
		}
	}
	base := 4 + nDist
	params := make([]float64, base+6*nView)
	params[0], params[1], params[2], params[3] = cam.Fx, cam.Fy, cam.Cx, cam.Cy
	for i, p := range poses {
		axis, ang := geom.AxisAngle(p.R)
		rv := axis.Scale(ang)
		o := base + 6*i
		params[o], params[o+1], params[o+2] = rv.X, rv.Y, rv.Z
		params[o+3], params[o+4], params[o+5] = p.T.X, p.T.Y, p.T.Z
	}

	unpack := func(p []float64) (Camera, []geom.Pose) {
		c := Camera{Width: cam.Width, Height: cam.Height, Fx: p[0], Fy: p[1], Cx: p[2], Cy: p[3]}
		switch {
		case opt.FixTangential && opt.FixK3:
			c.K1, c.K2 = p[4], p[5]
		case opt.FixTangential:
			c.K1, c.K2, c.K3 = p[4], p[5], p[6]
		case opt.FixK3:
			c.K1, c.K2, c.P1, c.P2 = p[4], p[5], p[6], p[7]
		default:
			c.K1, c.K2, c.P1, c.P2, c.K3 = p[4], p[5], p[6], p[7], p[8]
		}
		ps := make([]geom.Pose, nView)
		for i := range ps {
			ps[i] = poseFromParams(p[base+6*i : base+6*i+6])
		}
		return c, ps
	}

	nRes := 2 * nView * len(model)
	residuals := func(p, r []float64) {
		c, ps := unpack(p)
		idx := 0
		for vi, view := range views {
			for mi, m := range model {
				cp := ps[vi].Apply(m)
				if cp.Z <= 1e-6 {
					r[idx], r[idx+1] = 1e6, 1e6
					idx += 2
					continue
				}
				px, _ := c.Project(cp)
				r[idx] = px.X - view.Corners[mi].X
				r[idx+1] = px.Y - view.Corners[mi].Y
				idx += 2
			}
		}
	}

	out, err := numeric.LevenbergMarquardt(residuals, params, nRes, numeric.LMOptions{MaxIterations: 80})
	if err != nil {
		return cam, poses, 0, err
	}
	c, ps := unpack(out.Params)
	if err := c.Validate(); err != nil {
		return cam, poses, 0, fmt.Errorf("%w: уточнение дало невозможные параметры (%v)", ErrDegenerateCalibration, err)
	}
	return c, ps, out.RMS, nil
}

// frameCoverage is the fraction of the frame enclosed by all the detected
// corners taken together.
func frameCoverage(views []CalibrationView, width, height int) float64 {
	var all []Point2
	for _, v := range views {
		all = append(all, v.Corners...)
	}
	hull := convexHull(all)
	if len(hull) < 3 {
		return 0
	}
	return polyArea(hull) / float64(width*height)
}

// tiltSpread is the largest angle between any two board normals across the
// views.
func tiltSpread(poses []geom.Pose) float64 {
	var max float64
	for i := range poses {
		for j := i + 1; j < len(poses); j++ {
			a := poses[i].R.Col(2).AngleTo(poses[j].R.Col(2))
			if a > max {
				max = a
			}
		}
	}
	return geom.Deg(max)
}

// calibrationWarnings turns the quality measures into advice a person can act
// on, because the failure modes of calibration are all procedural.
func calibrationWarnings(r CalibrationResult, nViews int) []string {
	var out []string

	if nViews < 8 {
		out = append(out, fmt.Sprintf(
			"Всего %d снимков. Для устойчивой калибровки нужно 10–20 снимков мишени "+
				"в разных положениях и под разными углами.", nViews))
	}
	if r.TiltSpreadDeg < 25 {
		out = append(out, fmt.Sprintf(
			"Мишень снята почти под одним углом (разброс наклона всего %.0f°). "+
				"Фокусное расстояние и расстояние до мишени при этом неразличимы: наклоняйте доску "+
				"в разные стороны на 30–45°, а не просто двигайте её по кадру.", r.TiltSpreadDeg))
	}
	if r.CoverageFraction < 0.5 {
		out = append(out, fmt.Sprintf(
			"Углы мишени покрыли лишь %.0f%% кадра. Дисторсия определяется только там, где есть данные, "+
				"а сильнее всего она у краёв — именно там, где на замере окажутся колёса. "+
				"Обязательно снимите мишень в каждом углу кадра.", r.CoverageFraction*100))
	}
	if r.RMSPx > 1.0 {
		out = append(out, fmt.Sprintf(
			"Ошибка обратного проецирования %.2f пикс — слишком много. Проверьте, что мишень жёсткая и плоская, "+
				"снимки резкие, а размер клетки измерен штангенциркулем по реальной распечатке.", r.RMSPx))
	}
	for i, v := range r.PerViewRMSPx {
		if v > 3*math.Max(r.RMSPx, 0.05) && v > 1.0 {
			out = append(out, fmt.Sprintf(
				"Снимок %d выбивается (%.2f пикс против общих %.2f) — вероятно, смазан или мишень на нём "+
					"была изогнута. Исключите его и пересчитайте.", i+1, v, r.RMSPx))
		}
	}
	return out
}

// CalibrateFromImages detects the board in each image and calibrates from the
// result. Images that fail detection are reported and skipped rather than
// aborting the whole run — one bad photograph out of twenty should not cost the
// other nineteen.
func CalibrateFromImages(target Target, imgs []*Gray, labels []string, opt CalibrateOptions) (CalibrationResult, error) {
	if len(imgs) == 0 {
		return CalibrationResult{}, ErrTooFewViews
	}
	w, h := imgs[0].W, imgs[0].H
	var views []CalibrationView
	var skipped []string

	for i, img := range imgs {
		label := fmt.Sprintf("снимок %d", i+1)
		if i < len(labels) && labels[i] != "" {
			label = labels[i]
		}
		if img.W != w || img.H != h {
			skipped = append(skipped, fmt.Sprintf("%s: другой размер кадра (%dx%d вместо %dx%d)",
				label, img.W, img.H, w, h))
			continue
		}
		det, err := DetectCheckerboard(img, DetectOptions{Target: target})
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", label, err))
			continue
		}
		views = append(views, CalibrationView{Corners: det.Corners, Label: label})
	}

	if len(views) < 3 {
		return CalibrationResult{}, fmt.Errorf("%w: распознано только %d из %d снимков. %v",
			ErrTooFewViews, len(views), len(imgs), skipped)
	}
	res, err := CalibrateCamera(target, views, w, h, opt)
	if err != nil {
		return res, err
	}
	for _, s := range skipped {
		res.Warnings = append(res.Warnings, "Снимок пропущен — "+s)
	}
	return res, nil
}
