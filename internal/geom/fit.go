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
	Direction Vec3 // unit vector along the axis
	Point     Vec3 // the point on the axis closest to the origin of the input frame

	// Center is the point on the axis level with the moving body's own origin:
	// for a wheel target, the point on the spin axis at the board's axial
	// station. For a board clamped to a wheel that is the wheel centre plus the
	// clamp's axial offset — an offset that lies ALONG the axis, so it displaces
	// left and right wheels symmetrically and leaves axle midpoints, and with
	// them the vehicle's longitudinal axis, untouched.
	//
	// Position along the axis is not observable from the rotation: every point
	// on the line is equally stationary. So it is not left to the least-squares
	// solve to decide — doing that let noise slide the point hundreds of
	// millimetres along the axis, which scattered the wheel centres sideways and
	// yawed the whole vehicle frame. Instead the perpendicular position comes
	// from the well-conditioned part of the fit and the axial station is set
	// explicitly, from where the body actually sits.
	Center Vec3

	Sweep    float64 // total rotation observed, radians — a quality indicator
	Residual float64 // rms of the point-fit residual, input length units
}

// FitRotationAxis recovers the line a rigid body rotated about, from a sequence
// of its poses.
//
// The direction comes from the axis of the relative rotations. The position
// takes more care than it first appears. A point of the body that stays still
// has body-frame coordinates p and world position R_i·p + t_i, and that
// position must be the same in every frame. Subtracting the mean gives
//
//	(R_i − R̄)·p = t̄ − t_i
//
// which is linear in p and solvable by least squares. The fixed point in the
// world is then c = mean(R_i·p + t_i).
//
// The tempting shortcut — requiring R_i·c + t_i = c, i.e. (I − R_i)·c = t_i —
// is a special case that silently assumes the body frame is aligned with the
// world frame. It is not, for a target clamped to a wheel at whatever angle the
// clamp happened to sit at, and using it puts the recovered axis hundreds of
// millimetres away from the real one while leaving its direction perfectly
// correct. Direction-only users would never notice; wheelbase and track would
// be nonsense.
//
// The system is singular along the axis itself — every point on the axis is
// equally fixed — so it is solved with a truncated pseudo-inverse. Only the
// line is determined, never a particular point along it.
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

	// Point: least squares on (R_i − R̄)·p = t̄ − t_i, for the stationary point
	// p expressed in the BODY frame.
	n := float64(len(poses))
	var rBar Mat3
	var tBar Vec3
	for _, p := range poses {
		rBar = rBar.AddM(p.R)
		tBar = tBar.Add(p.T)
	}
	rBar = rBar.Scale(1 / n)
	tBar = tBar.Scale(1 / n)

	var ata Mat3
	var atb Vec3
	for _, p := range poses {
		a := p.R.SubM(rBar)
		ata = ata.AddM(a.T().Mul(a))
		atb = atb.Add(a.T().MulVec(tBar.Sub(p.T)))
	}
	body := pseudoSolveSym(ata, atb)

	// Carry it into the world and average: c = mean(R_i·p + t_i).
	var c Vec3
	for _, p := range poses {
		c = c.Add(p.Apply(body))
	}
	c = c.Scale(1 / n)

	// Only the line is determined. The component of c PERPENDICULAR to the axis
	// is well conditioned and fixes where the line runs; the component along it
	// is meaningless, so it is dropped rather than reported.
	axisPoint := c.Sub(dir.Scale(c.Dot(dir)))

	// Give the line a definite station by placing it level with the body's own
	// origin, averaged over the poses. A body origin off the axis traces a
	// circle about it, and the mean of a circle's points is its centre, which
	// lies on the axis — so this is stable whether or not the target was
	// clamped square.
	var meanOrigin Vec3
	for _, p := range poses {
		meanOrigin = meanOrigin.Add(p.T)
	}
	meanOrigin = meanOrigin.Scale(1 / n)
	center := axisPoint.Add(dir.Scale(meanOrigin.Sub(axisPoint).Dot(dir)))

	// Residual: how far the supposedly stationary point actually wandered.
	var ss float64
	for _, p := range poses {
		ss += p.Apply(body).Sub(c).Len2()
	}
	return AxisFit{
		Direction: dir,
		Point:     axisPoint,
		Center:    center,
		Sweep:     sweep,
		Residual:  math.Sqrt(ss / n),
	}, nil
}

// pseudoSolveSym solves A·x = b for symmetric positive-semidefinite A,
// discarding directions whose eigenvalue is negligible relative to the largest.
func pseudoSolveSym(a Mat3, b Vec3) Vec3 {
	vals, vecs := SymEigen(a)
	// The cut is deliberately blunt. In the rotation-axis fit one direction —
	// along the axis — is exactly unconstrained in theory and merely tiny in
	// practice, and dividing a noise-sized residual by a tiny eigenvalue throws
	// the answer hundreds of millimetres along that direction. A relative
	// threshold of 1e-9 is small enough to let that happen; 1e-6 discards the
	// direction outright and returns the minimum-norm solution, which is the
	// honest answer when a direction carries no information.
	tol := 1e-6 * math.Abs(vals[0])
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
