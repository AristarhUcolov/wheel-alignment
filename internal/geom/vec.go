// Package geom provides the 3D linear algebra used by the alignment engine:
// vectors, rotation matrices, and the least-squares fits that turn a cloud of
// observed target poses into a physical axis (wheel spin axis, steering axis,
// road plane).
//
// Coordinate convention used throughout the project (vehicle frame):
//
//	+X — forward, along the vehicle's direction of travel
//	+Y — to the left (driver's left in LHD)
//	+Z — up
//
// Right-handed. Angles in radians internally; conversion to degrees, to
// degrees'minutes, and to millimetres of toe happens only at the presentation
// layer (see package align).
package geom

import "math"

// Vec3 is a 3-component vector.
type Vec3 struct {
	X, Y, Z float64
}

func V(x, y, z float64) Vec3 { return Vec3{x, y, z} }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Neg() Vec3            { return Vec3{-a.X, -a.Y, -a.Z} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}

func (a Vec3) Len() float64  { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Len2() float64 { return a.Dot(a) }

// Unit returns a normalised copy. A zero vector is returned unchanged so that
// callers can detect degenerate input via Len() instead of panicking mid-solve.
func (a Vec3) Unit() Vec3 {
	n := a.Len()
	if n < 1e-15 {
		return a
	}
	return a.Scale(1 / n)
}

// AngleTo returns the unsigned angle between two vectors, in radians. Uses the
// atan2 form, which stays accurate for nearly-parallel vectors where acos(dot)
// loses most of its significant digits — and nearly-parallel is the normal case
// here (a wheel spin axis barely moves between frames).
func (a Vec3) AngleTo(b Vec3) float64 {
	return math.Atan2(a.Cross(b).Len(), a.Dot(b))
}

// Any returns some unit vector perpendicular to a.
func (a Vec3) Any() Vec3 {
	u := a.Unit()
	alt := Vec3{1, 0, 0}
	if math.Abs(u.X) > 0.9 {
		alt = Vec3{0, 1, 0}
	}
	return u.Cross(alt).Unit()
}

// Mat3 is a row-major 3x3 matrix: M[row][col].
type Mat3 [3][3]float64

func Identity() Mat3 {
	return Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
}

// FromRows builds a matrix whose rows are the given vectors.
func FromRows(r0, r1, r2 Vec3) Mat3 {
	return Mat3{
		{r0.X, r0.Y, r0.Z},
		{r1.X, r1.Y, r1.Z},
		{r2.X, r2.Y, r2.Z},
	}
}

// FromCols builds a matrix whose columns are the given vectors. For a rotation
// matrix, the columns are the images of the basis vectors — i.e. the body axes
// expressed in the parent frame.
func FromCols(c0, c1, c2 Vec3) Mat3 {
	return Mat3{
		{c0.X, c1.X, c2.X},
		{c0.Y, c1.Y, c2.Y},
		{c0.Z, c1.Z, c2.Z},
	}
}

func (m Mat3) Row(i int) Vec3 { return Vec3{m[i][0], m[i][1], m[i][2]} }
func (m Mat3) Col(j int) Vec3 { return Vec3{m[0][j], m[1][j], m[2][j]} }

func (m Mat3) MulVec(v Vec3) Vec3 {
	return Vec3{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z,
	}
}

func (m Mat3) Mul(n Mat3) Mat3 {
	var out Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out[i][j] = m[i][0]*n[0][j] + m[i][1]*n[1][j] + m[i][2]*n[2][j]
		}
	}
	return out
}

func (m Mat3) T() Mat3 {
	return Mat3{
		{m[0][0], m[1][0], m[2][0]},
		{m[0][1], m[1][1], m[2][1]},
		{m[0][2], m[1][2], m[2][2]},
	}
}

func (m Mat3) Scale(s float64) Mat3 {
	var out Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out[i][j] = m[i][j] * s
		}
	}
	return out
}

func (m Mat3) AddM(n Mat3) Mat3 {
	var out Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out[i][j] = m[i][j] + n[i][j]
		}
	}
	return out
}

func (m Mat3) SubM(n Mat3) Mat3 { return m.AddM(n.Scale(-1)) }

func (m Mat3) Det() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

func (m Mat3) Trace() float64 { return m[0][0] + m[1][1] + m[2][2] }

// Inv returns the inverse and whether the matrix was invertible.
func (m Mat3) Inv() (Mat3, bool) {
	d := m.Det()
	if math.Abs(d) < 1e-18 {
		return Identity(), false
	}
	id := 1 / d
	var out Mat3
	out[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) * id
	out[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) * id
	out[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) * id
	out[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) * id
	out[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) * id
	out[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) * id
	out[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) * id
	out[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) * id
	out[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) * id
	return out, true
}

// Outer returns the outer product a*bᵀ.
func Outer(a, b Vec3) Mat3 {
	return Mat3{
		{a.X * b.X, a.X * b.Y, a.X * b.Z},
		{a.Y * b.X, a.Y * b.Y, a.Y * b.Z},
		{a.Z * b.X, a.Z * b.Y, a.Z * b.Z},
	}
}

// Skew returns the cross-product matrix [a]× such that [a]× b == a × b.
func Skew(a Vec3) Mat3 {
	return Mat3{
		{0, -a.Z, a.Y},
		{a.Z, 0, -a.X},
		{-a.Y, a.X, 0},
	}
}

// Pose is a rigid transform: a rotation plus a translation. Applying it maps a
// point from the body frame into the parent frame as R*p + T.
type Pose struct {
	R Mat3
	T Vec3
}

func IdentityPose() Pose { return Pose{R: Identity()} }

func (p Pose) Apply(v Vec3) Vec3    { return p.R.MulVec(v).Add(p.T) }
func (p Pose) ApplyDir(v Vec3) Vec3 { return p.R.MulVec(v) }

// Mul composes poses: (p*q) applied to v equals p applied to (q applied to v).
func (p Pose) Mul(q Pose) Pose {
	return Pose{R: p.R.Mul(q.R), T: p.R.MulVec(q.T).Add(p.T)}
}

func (p Pose) Inverse() Pose {
	rt := p.R.T()
	return Pose{R: rt, T: rt.MulVec(p.T).Neg()}
}

// Deg and Rad convert between the internal radian representation and the
// degrees used by every alignment spec sheet ever printed.
func Deg(rad float64) float64 { return rad * 180 / math.Pi }
func Rad(deg float64) float64 { return deg * math.Pi / 180 }
