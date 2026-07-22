package numeric_test

import (
	"math"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/numeric"
)

// TestSymEigenReconstructs: A = V·diag(λ)·Vᵀ, with orthonormal V and
// descending λ. Everything else in the package leans on this.
func TestSymEigenReconstructs(t *testing.T) {
	n := 7
	// A symmetric matrix with a deliberately wide spread of eigenvalues, which
	// is where a careless solver loses the small ones — and the smallest
	// eigenvector is exactly what homography estimation needs.
	b := numeric.New(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			b.Set(i, j, math.Sin(float64(i*3+j*7+1))*math.Pow(10, float64(i%4)))
		}
	}
	a := b.GramTA()

	vals, vecs, err := numeric.SymEigen(a)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < n; i++ {
		if vals[i] > vals[i-1]+1e-9 {
			t.Fatalf("eigenvalues not descending: %v", vals)
		}
	}

	// Orthonormality.
	vtv, err := vecs.T().Mul(vecs)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			want := 0.0
			if i == j {
				want = 1
			}
			if math.Abs(vtv.At(i, j)-want) > 1e-10 {
				t.Fatalf("VᵀV is not identity at [%d][%d]: %.12f", i, j, vtv.At(i, j))
			}
		}
	}

	// Reconstruction.
	d := numeric.New(n, n)
	for i := 0; i < n; i++ {
		d.Set(i, i, vals[i])
	}
	vd, _ := vecs.Mul(d)
	rec, _ := vd.Mul(vecs.T())
	var maxErr float64
	for i := range a.Data {
		maxErr = math.Max(maxErr, math.Abs(rec.Data[i]-a.Data[i]))
	}
	scale := math.Max(vals[0], 1)
	if maxErr/scale > 1e-11 {
		t.Errorf("reconstruction error %.3e relative to largest eigenvalue %.3e", maxErr/scale, vals[0])
	}
}

// TestNullVectorFindsExactNullSpace: a rank-deficient system's null vector is
// what the direct linear transform extracts, so it must be found precisely and
// not merely approximately.
func TestNullVectorFindsExactNullSpace(t *testing.T) {
	// Rows all orthogonal to (1, -2, 3, -4)/‖·‖.
	want := []float64{1, -2, 3, -4}
	norm := numeric.Norm2(want)
	for i := range want {
		want[i] /= norm
	}
	rows := [][]float64{
		{2, 1, 0, 0},  // 2 − 2 = 0
		{0, 3, 2, 0},  // −6 + 6 = 0
		{4, 0, 0, 1},  // 4 − 4 = 0
		{0, 0, 4, 3},  // 12 − 12 = 0
		{3, 0, -1, 0}, // 3 − 3 = 0
	}
	a := numeric.New(len(rows), 4)
	for i, r := range rows {
		for j, v := range r {
			a.Set(i, j, v)
		}
	}

	got, err := numeric.NullVector(a)
	if err != nil {
		t.Fatal(err)
	}
	// Sign is arbitrary for a null vector.
	var dot float64
	for i := range got {
		dot += got[i] * want[i]
	}
	if dot < 0 {
		for i := range got {
			got[i] = -got[i]
		}
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-10 {
			t.Fatalf("null vector %v, want %v", got, want)
		}
	}
}

// TestSolveSPD solves a known system and refuses a non-positive-definite one —
// the refusal is how Levenberg–Marquardt learns to raise its damping.
func TestSolveSPD(t *testing.T) {
	a := numeric.FromSlice(3, 3, []float64{
		4, 1, 1,
		1, 3, 0,
		1, 0, 2,
	})
	want := []float64{1, -2, 3}
	b := make([]float64, 3)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			b[i] += a.At(i, j) * want[j]
		}
	}
	got, err := numeric.SolveSPD(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("solved %v, want %v", got, want)
		}
	}

	indefinite := numeric.FromSlice(2, 2, []float64{1, 2, 2, 1}) // eigenvalues 3 and −1
	if _, err := numeric.SolveSPD(indefinite, []float64{1, 1}); err == nil {
		t.Error("an indefinite matrix must be rejected, not silently factorised")
	}
}

// TestLevenbergMarquardtFitsExactly recovers the parameters of a noise-free
// non-linear model — here a decaying sinusoid, chosen because its parameters
// differ by orders of magnitude, which is exactly what Marquardt's diagonal
// scaling exists to handle and what plain identity damping handles badly.
func TestLevenbergMarquardtFitsExactly(t *testing.T) {
	want := []float64{2.5, 0.35, 4.2, 0.7} // amplitude, decay, frequency, phase
	const m = 80
	ts := make([]float64, m)
	obs := make([]float64, m)
	model := func(p []float64, x float64) float64 {
		return p[0] * math.Exp(-p[1]*x) * math.Sin(p[2]*x+p[3])
	}
	for i := 0; i < m; i++ {
		ts[i] = float64(i) * 0.05
		obs[i] = model(want, ts[i])
	}

	residuals := func(p, r []float64) {
		for i := range r {
			r[i] = model(p, ts[i]) - obs[i]
		}
	}
	start := []float64{1.0, 0.1, 4.0, 0.5}

	res, err := numeric.LevenbergMarquardt(residuals, start, m, numeric.LMOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("сошлось за %d итераций (%s), СКО %.3e", res.Iterations, res.Reason, res.RMS)
	if !res.Converged {
		t.Errorf("did not converge: %s", res.Reason)
	}
	for i := range want {
		if math.Abs(res.Params[i]-want[i]) > 1e-6 {
			t.Errorf("параметр %d = %.9f, ожидалось %.9f", i, res.Params[i], want[i])
		}
	}
	if res.RMS > 1e-9 {
		t.Errorf("residual %.3e on noise-free data", res.RMS)
	}
}

// TestLevenbergMarquardtRejectsUnderdetermined: fewer residuals than parameters
// has no least-squares solution, and pretending otherwise would hand back
// confident nonsense.
func TestLevenbergMarquardtRejectsUnderdetermined(t *testing.T) {
	f := func(p, r []float64) { r[0] = p[0] + p[1] + p[2] }
	if _, err := numeric.LevenbergMarquardt(f, []float64{0, 0, 0}, 1, numeric.LMOptions{}); err == nil {
		t.Error("1 residual cannot determine 3 parameters")
	}
}
