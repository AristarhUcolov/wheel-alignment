package geom

import "math"

// Rodrigues builds the rotation matrix for a rotation of `angle` radians about
// the unit axis `axis`, right-handed.
func Rodrigues(axis Vec3, angle float64) Mat3 {
	k := axis.Unit()
	s, c := math.Sin(angle), math.Cos(angle)
	kk := Outer(k, k)
	return Identity().Scale(c).
		AddM(Skew(k).Scale(s)).
		AddM(kk.Scale(1 - c))
}

// RotX, RotY, RotZ are the elementary right-handed rotations.
func RotX(a float64) Mat3 { return Rodrigues(Vec3{1, 0, 0}, a) }
func RotY(a float64) Mat3 { return Rodrigues(Vec3{0, 1, 0}, a) }
func RotZ(a float64) Mat3 { return Rodrigues(Vec3{0, 0, 1}, a) }

// AxisAngle decomposes a rotation matrix into its axis and angle (the matrix
// logarithm of SO(3)). The angle is returned in [0, π].
//
// This is the workhorse of runout compensation: rolling the car produces a
// sequence of target poses whose *relative* rotations all share one axis — the
// wheel's true spin axis — regardless of how crookedly the target was clamped
// to the rim.
func AxisAngle(m Mat3) (axis Vec3, angle float64) {
	// cos θ = (tr(R) − 1)/2, clamped against round-off.
	c := (m.Trace() - 1) / 2
	c = math.Max(-1, math.Min(1, c))
	angle = math.Acos(c)

	if angle < 1e-9 {
		// Essentially no rotation: axis is undefined, report a stable default.
		return Vec3{0, 0, 1}, 0
	}

	if math.Pi-angle > 1e-6 {
		// General case: the antisymmetric part of R gives sin(θ)·axis.
		v := Vec3{
			m[2][1] - m[1][2],
			m[0][2] - m[2][0],
			m[1][0] - m[0][1],
		}
		return v.Unit(), angle
	}

	// θ ≈ π: the antisymmetric part vanishes. Recover the axis from the
	// symmetric part, R + I = 2·kkᵀ, taking the numerically strongest column.
	b := m.AddM(Identity())
	best, bestLen := Vec3{0, 0, 1}, -1.0
	for j := 0; j < 3; j++ {
		col := b.Col(j)
		if l := col.Len2(); l > bestLen {
			best, bestLen = col, l
		}
	}
	return best.Unit(), angle
}

// RotationBetween returns the shortest rotation taking unit vector a onto unit
// vector b.
func RotationBetween(a, b Vec3) Mat3 {
	u, v := a.Unit(), b.Unit()
	cross := u.Cross(v)
	d := u.Dot(v)
	if cross.Len() < 1e-12 {
		if d > 0 {
			return Identity()
		}
		// Antiparallel: rotate π about any perpendicular axis.
		return Rodrigues(u.Any(), math.Pi)
	}
	return Rodrigues(cross, math.Atan2(cross.Len(), d))
}

// Orthonormalize returns the closest rotation matrix to m in the Frobenius
// sense, via polar decomposition R = M(MᵀM)^(-1/2). Needed because poses that
// come out of a PnP solve or that have been averaged drift away from SO(3)
// just enough to poison downstream angle extraction.
func Orthonormalize(m Mat3) Mat3 {
	ata := m.T().Mul(m)
	vals, vecs := SymEigen(ata)

	// (MᵀM)^(-1/2) = V · diag(1/√λ) · Vᵀ
	var inv Mat3
	for i := 0; i < 3; i++ {
		l := vals[i]
		if l < 1e-18 {
			l = 1e-18
		}
		v := vecs.Col(i)
		inv = inv.AddM(Outer(v, v).Scale(1 / math.Sqrt(l)))
	}
	r := m.Mul(inv)
	if r.Det() < 0 {
		// Reflection slipped in; flip the axis with the weakest singular value.
		minI := 0
		for i := 1; i < 3; i++ {
			if vals[i] < vals[minI] {
				minI = i
			}
		}
		v := vecs.Col(minI)
		r = r.Mul(Identity().SubM(Outer(v, v).Scale(2)))
	}
	return r
}

// AverageRotations returns a rotation representing the mean of the inputs,
// computed by orthonormalising the arithmetic mean (the "chordal L2 mean" on
// SO(3)). Exact enough for the small spreads seen when averaging frames of a
// stationary target, and it never diverges.
func AverageRotations(rs []Mat3) Mat3 {
	if len(rs) == 0 {
		return Identity()
	}
	var sum Mat3
	for _, r := range rs {
		sum = sum.AddM(r)
	}
	return Orthonormalize(sum.Scale(1 / float64(len(rs))))
}
