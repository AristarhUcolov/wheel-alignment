package vision

import (
	"errors"
	"fmt"
	"math"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/numeric"
)

var (
	ErrTooFewPoints   = errors.New("vision: для определения позы нужно не менее 4 точек")
	ErrNotPlanar      = errors.New("vision: точки мишени должны лежать в одной плоскости (Z = 0)")
	ErrDegeneratePose = errors.New("vision: точки вырождены — поза не определяется")
)

// Correspondence pairs a known point on the target with where it was seen.
type Correspondence struct {
	// Model is the point in the target's own frame, in millimetres. A planar
	// target has Z == 0 for every point.
	Model geom.Vec3
	// Image is where that point was detected, in pixels.
	Image Point2
}

// PnPResult is a solved target pose plus everything needed to judge it.
type PnPResult struct {
	// Pose maps target coordinates into camera coordinates.
	Pose geom.Pose

	// RMSPx is the root-mean-square reprojection error. For a printed target
	// detected properly this should be a small fraction of a pixel; a value
	// above about one pixel means the detection, the calibration, or the
	// target's printed dimensions are wrong.
	RMSPx float64
	MaxPx float64

	// Ambiguous marks the planar-pose ambiguity: a flat target admits a second
	// pose, mirrored about the line of sight, that can fit the image almost as
	// well. Choosing wrong flips the sign of the target's tilt, which on a wheel
	// is a camber error of twice that tilt — the classic silent failure of every
	// flat-target vision system.
	//
	// The dangerous regime is weak perspective: a target that is small in the
	// frame, either because it is far away or because it is small, seen at a
	// moderate tilt. What separates the two solutions is the perspective
	// foreshortening across the target — near the camera the far edge is
	// visibly smaller than the near edge and the twin is ruled out, while at
	// range the projection is nearly affine and both fit. A target viewed
	// exactly face-on has no distinct twin at all: there, the two solutions
	// coincide.
	Ambiguous      bool
	AlternateRMSPx float64
	// AlternateTiltDeg is how far the rejected pose differs in target normal.
	// A large value with a close RMS is the dangerous combination.
	AlternateTiltDeg float64

	Iterations int
	Warnings   []string
}

// SolvePnPPlanar recovers the pose of a planar target from its detected points.
//
// The route is the classical one, chosen because every step is checkable:
//
//  1. Undistort the image points and work in normalised camera coordinates,
//     which removes the intrinsics from the geometry entirely.
//  2. Estimate the target-to-image homography by the direct linear transform,
//     with Hartley normalisation on both point sets. The normalisation is not
//     optional: without it the DLT matrix has a condition number in the
//     millions and the result visibly degrades.
//  3. Decompose the homography into rotation and translation.
//  4. Refine by Levenberg–Marquardt on true reprojection error in pixels,
//     which is the quantity that actually matters and the only one whose
//     residual is meaningful to report.
//  5. Repeat from the mirrored starting pose and compare, so the planar
//     ambiguity is detected rather than silently resolved by luck.
func SolvePnPPlanar(cam Camera, corr []Correspondence) (PnPResult, error) {
	if len(corr) < 4 {
		return PnPResult{}, fmt.Errorf("%w (дано %d)", ErrTooFewPoints, len(corr))
	}
	for _, c := range corr {
		if math.Abs(c.Model.Z) > 1e-6 {
			return PnPResult{}, ErrNotPlanar
		}
	}

	// Step 1: normalised, undistorted image points.
	img := make([]Point2, len(corr))
	for i, c := range corr {
		x, y := cam.Normalize(c.Image)
		img[i] = Point2{X: x, Y: y}
	}
	model := make([]Point2, len(corr))
	for i, c := range corr {
		model[i] = Point2{X: c.Model.X, Y: c.Model.Y}
	}

	// Steps 2 and 3.
	h, err := homographyDLT(model, img)
	if err != nil {
		return PnPResult{}, err
	}
	seed, err := poseFromHomography(h)
	if err != nil {
		return PnPResult{}, err
	}

	// Step 4.
	best, iters, err := refinePose(cam, corr, seed)
	if err != nil {
		return PnPResult{}, err
	}
	bestRMS, bestMax := reprojectionError(cam, corr, best)

	// Step 5: the mirrored hypothesis.
	res := PnPResult{Pose: best, RMSPx: bestRMS, MaxPx: bestMax, Iterations: iters}
	if alt, ok := mirrorPose(best, corr); ok {
		altPose, _, err := refinePose(cam, corr, alt)
		if err == nil {
			altRMS, _ := reprojectionError(cam, corr, altPose)
			if altRMS < bestRMS {
				altPose, best = best, altPose
				altRMS, bestRMS = bestRMS, altRMS
				res.Pose = best
				_, bestMax = reprojectionError(cam, corr, best)
				res.RMSPx, res.MaxPx = bestRMS, bestMax
			}
			res.AlternateRMSPx = altRMS
			res.AlternateTiltDeg = geom.Deg(best.R.Col(2).AngleTo(altPose.R.Col(2)))

			// If the rejected pose fits nearly as well, the choice between them
			// is being made by noise.
			if altRMS < bestRMS*2 && res.AlternateTiltDeg > 3 {
				res.Ambiguous = true
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"Поза мишени определена неоднозначно: альтернативное решение отличается наклоном на %.1f° "+
						"и почти так же хорошо ложится на снимок (%.3f против %.3f пикс). "+
						"Ошибка в выборе даст ошибку развала примерно вдвое больше этого наклона. "+
						"Подойдите ближе или возьмите мишень крупнее: различить два решения позволяет "+
						"только перспективное искажение, а оно исчезает, когда мишень мелкая в кадре.",
					res.AlternateTiltDeg, altRMS, bestRMS))
			}
		}
	}

	res.Warnings = append(res.Warnings, cam.Warnings()...)
	if res.RMSPx > 1.0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"Ошибка обратного проецирования %.2f пикс — это много. Проверьте калибровку камеры, "+
				"реальные размеры мишени и качество распознавания углов.", res.RMSPx))
	}
	return res, nil
}

// homographyDLT estimates the 3×3 homography taking planar model points to
// normalised image points, by the direct linear transform with Hartley
// normalisation.
func homographyDLT(model, img []Point2) (geom.Mat3, error) {
	n := len(model)
	tm, mn := hartleyNormalize(model)
	ti, in := hartleyNormalize(img)

	a := numeric.New(2*n, 9)
	for i := 0; i < n; i++ {
		X, Y := mn[i].X, mn[i].Y
		x, y := in[i].X, in[i].Y
		r := 2 * i
		a.Set(r, 0, -X)
		a.Set(r, 1, -Y)
		a.Set(r, 2, -1)
		a.Set(r, 6, x*X)
		a.Set(r, 7, x*Y)
		a.Set(r, 8, x)
		r++
		a.Set(r, 3, -X)
		a.Set(r, 4, -Y)
		a.Set(r, 5, -1)
		a.Set(r, 6, y*X)
		a.Set(r, 7, y*Y)
		a.Set(r, 8, y)
	}
	h, err := numeric.NullVector(a)
	if err != nil {
		return geom.Mat3{}, err
	}
	hn := geom.Mat3{
		{h[0], h[1], h[2]},
		{h[3], h[4], h[5]},
		{h[6], h[7], h[8]},
	}

	// Undo both normalisations: H = Ti⁻¹ · Ĥ · Tm
	tiInv, ok := ti.Inv()
	if !ok {
		return geom.Mat3{}, ErrDegeneratePose
	}
	return tiInv.Mul(hn).Mul(tm), nil
}

// hartleyNormalize returns the similarity transform that centres a point set on
// the origin and scales its mean distance from the origin to √2, together with
// the transformed points. This is the conditioning step that makes the DLT
// usable in floating point.
func hartleyNormalize(pts []Point2) (geom.Mat3, []Point2) {
	n := float64(len(pts))
	var cx, cy float64
	for _, p := range pts {
		cx += p.X
		cy += p.Y
	}
	cx, cy = cx/n, cy/n

	var meanDist float64
	for _, p := range pts {
		meanDist += math.Hypot(p.X-cx, p.Y-cy)
	}
	meanDist /= n

	s := 1.0
	if meanDist > 1e-12 {
		s = math.Sqrt2 / meanDist
	}
	t := geom.Mat3{
		{s, 0, -s * cx},
		{0, s, -s * cy},
		{0, 0, 1},
	}
	out := make([]Point2, len(pts))
	for i, p := range pts {
		out[i] = Point2{X: s * (p.X - cx), Y: s * (p.Y - cy)}
	}
	return t, out
}

// poseFromHomography decomposes H = λ·[r₁ r₂ t] into a rigid pose.
//
// The target plane is Z = 0 in target coordinates, so only the first two
// columns of the rotation appear in the homography; the third is recovered as
// their cross product. The scale is taken from the average of the two column
// norms — using just one column makes the result depend on which way the target
// happens to be turned — and the sign is fixed by requiring the target to be in
// front of the camera.
func poseFromHomography(h geom.Mat3) (geom.Pose, error) {
	h1, h2, h3 := h.Col(0), h.Col(1), h.Col(2)
	n1, n2 := h1.Len(), h2.Len()
	if n1 < 1e-12 || n2 < 1e-12 {
		return geom.Pose{}, ErrDegeneratePose
	}
	lambda := 2 / (n1 + n2)
	if h3.Z < 0 {
		lambda = -lambda // put the target in front of the camera
	}

	r1 := h1.Scale(lambda)
	r2 := h2.Scale(lambda)
	r3 := r1.Cross(r2)
	t := h3.Scale(lambda)

	if t.Z <= 0 {
		return geom.Pose{}, ErrDegeneratePose
	}
	// The two recovered columns are not exactly orthonormal; project onto SO(3).
	r := geom.Orthonormalize(geom.FromCols(r1, r2, r3))
	return geom.Pose{R: r, T: t}, nil
}

// mirrorPose builds the second hypothesis of the planar ambiguity: the pose
// whose target normal is reflected about the line of sight. Reflecting rather
// than perturbing matters — the ambiguity is an exact symmetry of the
// fronto-parallel case, so the mirrored pose is a genuine local minimum, not a
// nearby point on the same basin.
func mirrorPose(p geom.Pose, corr []Correspondence) (geom.Pose, bool) {
	var centroid geom.Vec3
	for _, c := range corr {
		centroid = centroid.Add(p.Apply(c.Model))
	}
	centroid = centroid.Scale(1 / float64(len(corr)))
	v := centroid.Unit()
	if v.Len() < 0.5 {
		return geom.Pose{}, false
	}

	n := p.R.Col(2)
	// Reflection of n in the line spanned by v.
	n2 := v.Scale(2 * n.Dot(v)).Sub(n)
	if n2.AngleTo(n) < geom.Rad(1) {
		return geom.Pose{}, false // already face-on to the axis; no distinct twin
	}
	return geom.Pose{R: geom.RotationBetween(n, n2).Mul(p.R), T: p.T}, true
}

// refinePose minimises true reprojection error in pixels over the six pose
// parameters, starting from a seed. Rotation is carried as an axis-angle
// 3-vector, which is minimal and singularity-free for the small corrections
// involved here.
func refinePose(cam Camera, corr []Correspondence, seed geom.Pose) (geom.Pose, int, error) {
	axis, angle := geom.AxisAngle(seed.R)
	rvec := axis.Scale(angle)
	x0 := []float64{rvec.X, rvec.Y, rvec.Z, seed.T.X, seed.T.Y, seed.T.Z}

	residuals := func(p, r []float64) {
		pose := poseFromParams(p)
		for i, c := range corr {
			cp := pose.Apply(c.Model)
			// A point that has drifted behind the camera has no projection; a
			// large finite penalty steers the solver back rather than poisoning
			// the whole fit with a NaN.
			if cp.Z <= 1e-6 {
				r[2*i], r[2*i+1] = 1e6, 1e6
				continue
			}
			px, _ := cam.Project(cp)
			r[2*i] = px.X - c.Image.X
			r[2*i+1] = px.Y - c.Image.Y
		}
	}

	out, err := numeric.LevenbergMarquardt(residuals, x0, 2*len(corr), numeric.LMOptions{MaxIterations: 60})
	if err != nil {
		return seed, 0, err
	}
	return poseFromParams(out.Params), out.Iterations, nil
}

func poseFromParams(p []float64) geom.Pose {
	rv := geom.V(p[0], p[1], p[2])
	ang := rv.Len()
	r := geom.Identity()
	if ang > 1e-12 {
		r = geom.Rodrigues(rv.Scale(1/ang), ang)
	}
	return geom.Pose{R: r, T: geom.V(p[3], p[4], p[5])}
}

// reprojectionError reports the RMS and worst-case reprojection error.
func reprojectionError(cam Camera, corr []Correspondence, pose geom.Pose) (rms, max float64) {
	var ss float64
	for _, c := range corr {
		px, ok := cam.Project(pose.Apply(c.Model))
		if !ok {
			return math.Inf(1), math.Inf(1)
		}
		d := px.DistTo(c.Image)
		ss += d * d
		if d > max {
			max = d
		}
	}
	return math.Sqrt(ss / float64(len(corr))), max
}
