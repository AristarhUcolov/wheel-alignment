// Package numeric holds the dense linear algebra and non-linear least squares
// that the optical path needs and that package geom, being fixed at three
// dimensions, cannot provide: null spaces of 9×9 systems for homography
// estimation, and Levenberg–Marquardt for pose refinement and camera
// calibration.
//
// Everything here is small, dependency-free and chosen for robustness over
// speed. The matrices involved are at most a few hundred rows; the cost of an
// unconditionally stable method is irrelevant next to the cost of a solver that
// silently returns nonsense on a badly conditioned frame.
package numeric

import (
	"errors"
	"math"
)

var (
	ErrDimension  = errors.New("numeric: dimension mismatch")
	ErrSingular   = errors.New("numeric: matrix is singular")
	ErrNotSquare  = errors.New("numeric: matrix must be square")
	ErrNotPosDef  = errors.New("numeric: matrix is not positive definite")
	ErrNoProgress = errors.New("numeric: solver made no progress")
)

// Matrix is a dense row-major matrix.
type Matrix struct {
	Rows, Cols int
	Data       []float64
}

func New(rows, cols int) *Matrix {
	return &Matrix{Rows: rows, Cols: cols, Data: make([]float64, rows*cols)}
}

// FromSlice wraps an existing slice; it is not copied.
func FromSlice(rows, cols int, data []float64) *Matrix {
	return &Matrix{Rows: rows, Cols: cols, Data: data}
}

func Eye(n int) *Matrix {
	m := New(n, n)
	for i := 0; i < n; i++ {
		m.Set(i, i, 1)
	}
	return m
}

func (m *Matrix) At(i, j int) float64     { return m.Data[i*m.Cols+j] }
func (m *Matrix) Set(i, j int, v float64) { m.Data[i*m.Cols+j] = v }
func (m *Matrix) Add(i, j int, v float64) { m.Data[i*m.Cols+j] += v }

func (m *Matrix) Clone() *Matrix {
	out := New(m.Rows, m.Cols)
	copy(out.Data, m.Data)
	return out
}

func (m *Matrix) Col(j int) []float64 {
	out := make([]float64, m.Rows)
	for i := range out {
		out[i] = m.At(i, j)
	}
	return out
}

// T returns the transpose.
func (m *Matrix) T() *Matrix {
	out := New(m.Cols, m.Rows)
	for i := 0; i < m.Rows; i++ {
		for j := 0; j < m.Cols; j++ {
			out.Set(j, i, m.At(i, j))
		}
	}
	return out
}

// Mul returns m·n.
func (m *Matrix) Mul(n *Matrix) (*Matrix, error) {
	if m.Cols != n.Rows {
		return nil, ErrDimension
	}
	out := New(m.Rows, n.Cols)
	for i := 0; i < m.Rows; i++ {
		for k := 0; k < m.Cols; k++ {
			a := m.At(i, k)
			if a == 0 {
				continue
			}
			for j := 0; j < n.Cols; j++ {
				out.Add(i, j, a*n.At(k, j))
			}
		}
	}
	return out, nil
}

// GramTA returns mᵀ·m, which is symmetric positive semi-definite. Formed
// directly rather than via T().Mul() because it is the hot path in every
// least-squares solve here, and because computing only the upper triangle keeps
// the result exactly symmetric — Jacobi eigen decomposition is much happier
// when it is not handed a matrix that is symmetric only to within rounding.
func (m *Matrix) GramTA() *Matrix {
	n := m.Cols
	out := New(n, n)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			var s float64
			for k := 0; k < m.Rows; k++ {
				s += m.At(k, i) * m.At(k, j)
			}
			out.Set(i, j, s)
			out.Set(j, i, s)
		}
	}
	return out
}

// MulVecT returns mᵀ·v.
func (m *Matrix) MulVecT(v []float64) []float64 {
	out := make([]float64, m.Cols)
	for k := 0; k < m.Rows; k++ {
		vk := v[k]
		if vk == 0 {
			continue
		}
		for j := 0; j < m.Cols; j++ {
			out[j] += m.At(k, j) * vk
		}
	}
	return out
}

// SymEigen computes the eigenvalues and eigenvectors of a symmetric matrix by
// cyclic Jacobi rotations. Eigenvalues come back sorted descending; the
// eigenvectors are the columns of vecs, so the last column spans the null space
// of a rank-deficient matrix — which is exactly what homography estimation
// needs.
//
// Jacobi is used rather than a faster tridiagonal-QR path for the same reason
// as in package geom: it needs no pivoting, cannot break down, and the
// matrices it is handed here are deliberately near-singular.
func SymEigen(a *Matrix) (vals []float64, vecs *Matrix, err error) {
	if a.Rows != a.Cols {
		return nil, nil, ErrNotSquare
	}
	n := a.Rows
	m := a.Clone()
	v := Eye(n)

	for sweep := 0; sweep < 100; sweep++ {
		var off float64
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				off += m.At(i, j) * m.At(i, j)
			}
		}
		if off < 1e-30 {
			break
		}
		for p := 0; p < n-1; p++ {
			for q := p + 1; q < n; q++ {
				apq := m.At(p, q)
				if math.Abs(apq) < 1e-300 {
					continue
				}
				theta := (m.At(q, q) - m.At(p, p)) / (2 * apq)
				t := 1 / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				if theta < 0 {
					t = -t
				}
				c := 1 / math.Sqrt(t*t+1)
				s := t * c

				// Apply the rotation from both sides, touching only the two
				// affected rows and columns.
				for k := 0; k < n; k++ {
					akp, akq := m.At(k, p), m.At(k, q)
					m.Set(k, p, c*akp-s*akq)
					m.Set(k, q, s*akp+c*akq)
				}
				for k := 0; k < n; k++ {
					apk, aqk := m.At(p, k), m.At(q, k)
					m.Set(p, k, c*apk-s*aqk)
					m.Set(q, k, s*apk+c*aqk)
				}
				for k := 0; k < n; k++ {
					vkp, vkq := v.At(k, p), v.At(k, q)
					v.Set(k, p, c*vkp-s*vkq)
					v.Set(k, q, s*vkp+c*vkq)
				}
			}
		}
	}

	vals = make([]float64, n)
	for i := 0; i < n; i++ {
		vals[i] = m.At(i, i)
	}

	// Sort descending, permuting eigenvector columns alongside.
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for i := 0; i < n-1; i++ {
		best := i
		for k := i + 1; k < n; k++ {
			if vals[idx[k]] > vals[idx[best]] {
				best = k
			}
		}
		idx[i], idx[best] = idx[best], idx[i]
	}
	sortedVals := make([]float64, n)
	vecs = New(n, n)
	for c, src := range idx {
		sortedVals[c] = vals[src]
		for r := 0; r < n; r++ {
			vecs.Set(r, c, v.At(r, src))
		}
	}
	return sortedVals, vecs, nil
}

// NullVector returns the unit vector spanning the null space of aᵀa — the
// least-squares solution of a·x = 0 subject to ‖x‖ = 1. This is the standard
// homogeneous linear least-squares problem behind the direct linear transform.
func NullVector(a *Matrix) ([]float64, error) {
	vals, vecs, err := SymEigen(a.GramTA())
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, ErrDimension
	}
	return vecs.Col(len(vals) - 1), nil
}

// SolveSPD solves a·x = b for symmetric positive definite a, by Cholesky
// factorisation. Returns ErrNotPosDef if the factorisation breaks down, which
// is how Levenberg–Marquardt learns that its damping is too small.
func SolveSPD(a *Matrix, b []float64) ([]float64, error) {
	if a.Rows != a.Cols {
		return nil, ErrNotSquare
	}
	if len(b) != a.Rows {
		return nil, ErrDimension
	}
	n := a.Rows
	l := New(n, n)

	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			s := a.At(i, j)
			for k := 0; k < j; k++ {
				s -= l.At(i, k) * l.At(j, k)
			}
			if i == j {
				if s <= 0 {
					return nil, ErrNotPosDef
				}
				l.Set(i, j, math.Sqrt(s))
			} else {
				l.Set(i, j, s/l.At(j, j))
			}
		}
	}

	// Forward substitution, then back substitution.
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		s := b[i]
		for k := 0; k < i; k++ {
			s -= l.At(i, k) * y[k]
		}
		y[i] = s / l.At(i, i)
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for k := i + 1; k < n; k++ {
			s -= l.At(k, i) * x[k]
		}
		x[i] = s / l.At(i, i)
	}
	return x, nil
}

// Norm2 is the Euclidean norm of a vector.
func Norm2(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}
