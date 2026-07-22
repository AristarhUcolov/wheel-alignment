package geom

import (
	"errors"
	"math"
)

// ErrDegenerate is returned by the fitters when the input does not constrain
// the answer — too few samples, or samples that did not actually move.
var ErrDegenerate = errors.New("geom: degenerate input, not enough motion to fit")

func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}

// SymEigen computes the eigenvalues and eigenvectors of a symmetric 3x3 matrix
// by cyclic Jacobi rotations. Eigenvalues come back sorted descending; the
// eigenvectors are the *columns* of vecs, so vecs.Col(2) is the direction of
// least variance — the plane normal, in every fit below.
//
// Jacobi is overkill for 3x3 in raw speed terms but it is unconditionally
// stable and needs no pivoting, which matters more here: the covariance
// matrices we feed it are often nearly rank-deficient by construction.
func SymEigen(a Mat3) (vals [3]float64, vecs Mat3) {
	m := a
	v := Identity()

	for sweep := 0; sweep < 60; sweep++ {
		off := m[0][1]*m[0][1] + m[0][2]*m[0][2] + m[1][2]*m[1][2]
		if off < 1e-32 {
			break
		}
		for p := 0; p < 2; p++ {
			for q := p + 1; q < 3; q++ {
				if math.Abs(m[p][q]) < 1e-20 {
					continue
				}
				theta := (m[q][q] - m[p][p]) / (2 * m[p][q])
				t := sign(theta) / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				c := 1 / math.Sqrt(t*t+1)
				s := t * c

				j := Identity()
				j[p][p], j[q][q] = c, c
				j[p][q], j[q][p] = s, -s

				m = j.T().Mul(m).Mul(j)
				v = v.Mul(j)
			}
		}
	}

	vals = [3]float64{m[0][0], m[1][1], m[2][2]}

	// Selection sort, descending, permuting eigenvector columns alongside.
	idx := [3]int{0, 1, 2}
	for i := 0; i < 2; i++ {
		best := i
		for k := i + 1; k < 3; k++ {
			if vals[idx[k]] > vals[idx[best]] {
				best = k
			}
		}
		idx[i], idx[best] = idx[best], idx[i]
	}
	var sv [3]float64
	var svec Mat3
	for i, j := range idx {
		sv[i] = vals[j]
		col := v.Col(j)
		svec[0][i], svec[1][i], svec[2][i] = col.X, col.Y, col.Z
	}
	return sv, svec
}

// PlaneFit is the result of a total-least-squares plane through a point set.
type PlaneFit struct {
	Point  Vec3    // centroid, a point on the plane
	Normal Vec3    // unit normal
	RMS    float64 // rms distance of the samples from the plane, same units as input
	// Flatness is the ratio of out-of-plane to in-plane spread. Near 0 means a
	// well-determined plane; near 1 means the points are a blob and the normal
	// is meaningless.
	Flatness float64
}

// FitPlane fits a plane through 3 or more points, minimising perpendicular
// distance (not vertical distance — these are physical points in space, there
// is no privileged axis).
//
// Used for the road/rack plane from the four wheel-centre or turnplate
// observations, and — via FitConeAxis — for steering axis recovery.
func FitPlane(pts []Vec3) (PlaneFit, error) {
	if len(pts) < 3 {
		return PlaneFit{}, ErrDegenerate
	}
	var c Vec3
	for _, p := range pts {
		c = c.Add(p)
	}
	c = c.Scale(1 / float64(len(pts)))

	var cov Mat3
	for _, p := range pts {
		d := p.Sub(c)
		cov = cov.AddM(Outer(d, d))
	}
	cov = cov.Scale(1 / float64(len(pts)))

	vals, vecs := SymEigen(cov)
	n := vecs.Col(2).Unit()

	var ss float64
	for _, p := range pts {
		d := p.Sub(c).Dot(n)
		ss += d * d
	}
	rms := math.Sqrt(ss / float64(len(pts)))

	flat := 1.0
	if vals[0] > 1e-30 {
		flat = math.Sqrt(math.Max(0, vals[2]) / vals[0])
	}
	return PlaneFit{Point: c, Normal: n, RMS: rms, Flatness: flat}, nil
}

// FitConeAxis recovers the axis a set of unit direction vectors was swept
// around. If a direction d is rotated about a fixed axis k, then d·k stays
// constant, so the tips of all the sampled directions lie on a plane whose
// normal is k. Fitting that plane recovers k without ever needing to know the
// rotation angles.
//
// This is how the steering axis is found during a caster sweep: we watch the
// wheel's spin axis (already known from runout compensation) trace a cone as
// the wheel is steered, and the cone's axis *is* the steering axis. Crucially
// it is immune to the wheel rolling or slipping on the turnplates during the
// sweep, because spinning the wheel does not move its spin axis at all.
//
// `hint` disambiguates the sign of the result: the returned axis is the one
// with a positive dot product against the hint (pass "up" for a steering axis).
func FitConeAxis(dirs []Vec3, hint Vec3) (Vec3, error) {
	if len(dirs) < 3 {
		return Vec3{}, ErrDegenerate
	}
	pts := make([]Vec3, len(dirs))
	for i, d := range dirs {
		pts[i] = d.Unit()
	}
	// Spread check: if the wheel was barely steered, the tips cluster and the
	// plane through them is arbitrary. 0.5° of sweep is far below anything
	// usable, so refuse rather than return a confident-looking wrong number.
	var maxSep float64
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if a := pts[i].AngleTo(pts[j]); a > maxSep {
				maxSep = a
			}
		}
	}
	if maxSep < Rad(0.5) {
		return Vec3{}, ErrDegenerate
	}

	fit, err := FitPlane(pts)
	if err != nil {
		return Vec3{}, err
	}
	k := fit.Normal
	if k.Dot(hint) < 0 {
		k = k.Neg()
	}
	return k, nil
}

// AxisFit describes a physical axis of rotation recovered from a motion.
type AxisFit struct {
	Direction Vec3    // unit vector along the axis
	Point     Vec3    // the point on the axis closest to the origin of the input frame
	Sweep     float64 // total rotation observed, radians — a quality indicator
	Residual  float64 // rms of the point-fit residual, input length units
}

// FitRotationAxis recovers the line a rigid body rotated about, from a sequence
// of its poses. For a body pinned to a fixed axis through point c, every pose
// satisfies R_i·c + t_i = c, i.e. (I − R_i)·c = t_i. Stacking that over all
// poses and solving in the least-squares sense gives c; the direction comes
// from the axis of the relative rotations.
//
// The system is singular along the axis itself (sliding a point along the axis
// changes nothing), so we solve with a truncated pseudo-inverse and return the
// axis point nearest the origin.
func FitRotationAxis(poses []Pose) (AxisFit, error) {
	if len(poses) < 2 {
		return AxisFit{}, ErrDegenerate
	}

	// Direction: average the axes of each pose relative to the first, sign-
	// aligned and weighted by rotation angle so that near-stationary frames
	// (whose axis is pure noise) contribute nothing.
	base := poses[0].R.T()
	var acc Vec3
	var sweep float64
	var ref Vec3
	for _, p := range poses[1:] {
		ax, ang := AxisAngle(p.R.Mul(base))
		if ang < Rad(0.05) {
			continue
		}
		if ref.Len2() == 0 {
			ref = ax
		} else if ax.Dot(ref) < 0 {
			ax = ax.Neg()
		}
		acc = acc.Add(ax.Scale(ang))
		if ang > sweep {
			sweep = ang
		}
	}
	if acc.Len() < 1e-12 {
		return AxisFit{}, ErrDegenerate
	}
	dir := acc.Unit()

	// Point: least squares on (I − R_i)·c = t_i.
	var ata Mat3
	var atb Vec3
	for _, p := range poses {
		a := Identity().SubM(p.R)
		ata = ata.AddM(a.T().Mul(a))
		atb = atb.Add(a.T().MulVec(p.T))
	}
	c := pseudoSolveSym(ata, atb)
	// Slide to the point closest to the origin (removes the free parameter).
	c = c.Sub(dir.Scale(c.Dot(dir)))

	var ss float64
	for _, p := range poses {
		r := Identity().SubM(p.R).MulVec(c).Sub(p.T)
		ss += r.Len2()
	}
	return AxisFit{
		Direction: dir,
		Point:     c,
		Sweep:     sweep,
		Residual:  math.Sqrt(ss / float64(len(poses))),
	}, nil
}

// pseudoSolveSym solves A·x = b for symmetric positive-semidefinite A,
// discarding directions whose eigenvalue is negligible relative to the largest.
func pseudoSolveSym(a Mat3, b Vec3) Vec3 {
	vals, vecs := SymEigen(a)
	tol := 1e-9 * math.Abs(vals[0])
	var x Vec3
	for i := 0; i < 3; i++ {
		if vals[i] <= tol {
			continue
		}
		v := vecs.Col(i)
		x = x.Add(v.Scale(v.Dot(b) / vals[i]))
	}
	return x
}

// FitCircleAxis3D recovers the spin axis of a wheel from the path traced by a
// point rigidly attached to it (for example one corner of a target, or a bolt
// head), by fitting the plane of the circle. Kept as a fallback for setups
// where full 6-DOF pose is unavailable and only point tracks exist.
func FitCircleAxis3D(track []Vec3, hint Vec3) (Vec3, error) {
	fit, err := FitPlane(track)
	if err != nil {
		return Vec3{}, err
	}
	if fit.Flatness > 0.2 {
		return Vec3{}, ErrDegenerate
	}
	n := fit.Normal
	if n.Dot(hint) < 0 {
		n = n.Neg()
	}
	return n, nil
}
