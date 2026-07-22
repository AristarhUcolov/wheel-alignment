package numeric

import (
	"fmt"
	"math"
)

// ResidualFunc evaluates the residual vector for a parameter vector. It must
// fill r completely and must not modify params.
type ResidualFunc func(params, r []float64)

// LMOptions tunes the Levenberg–Marquardt solver. The zero value is sensible.
type LMOptions struct {
	MaxIterations  int     // default 100
	GradientTol    float64 // stop when ‖Jᵀr‖∞ falls below this; default 1e-12
	StepTol        float64 // stop when the relative parameter step is below this; default 1e-12
	CostTol        float64 // stop when the relative cost improvement is below this; default 1e-14
	InitialDamping float64 // default 1e-3, relative to the diagonal of JᵀJ
}

func (o LMOptions) withDefaults() LMOptions {
	if o.MaxIterations <= 0 {
		o.MaxIterations = 100
	}
	if o.GradientTol <= 0 {
		o.GradientTol = 1e-12
	}
	if o.StepTol <= 0 {
		o.StepTol = 1e-12
	}
	if o.CostTol <= 0 {
		o.CostTol = 1e-14
	}
	if o.InitialDamping <= 0 {
		o.InitialDamping = 1e-3
	}
	return o
}

// LMResult is the outcome of a fit.
type LMResult struct {
	Params     []float64
	Cost       float64 // sum of squared residuals
	RMS        float64 // root mean square residual — reprojection error, in pixels, for a PnP fit
	Iterations int
	Converged  bool
	Reason     string
}

// LevenbergMarquardt minimises the sum of squared residuals over params.
//
// The Jacobian is computed by central differences. Forward differences would be
// half the cost, but their truncation error scales as √ε rather than ε^(2/3),
// and that shows up directly in the final pose: this solver is what decides
// whether a camber angle is good to a hundredth of a degree or a tenth, so the
// extra evaluations are bought cheaply.
//
// Damping follows the classic Marquardt scaling — proportional to the diagonal
// of JᵀJ rather than to the identity — so that parameters with very different
// units (radians of rotation against millimetres of translation, in a pose fit)
// are damped comparably.
func LevenbergMarquardt(f ResidualFunc, x0 []float64, numResiduals int, opt LMOptions) (LMResult, error) {
	opt = opt.withDefaults()
	n := len(x0)
	m := numResiduals
	if n == 0 || m < n {
		return LMResult{}, fmt.Errorf("%w: %d residuals cannot determine %d parameters", ErrDimension, m, n)
	}

	x := make([]float64, n)
	copy(x, x0)

	r := make([]float64, m)
	f(x, r)
	cost := dot(r, r)

	j := New(m, n)
	trial := make([]float64, n)
	rTrial := make([]float64, m)
	lambda := 0.0
	res := LMResult{Params: x, Cost: cost, RMS: math.Sqrt(cost / float64(m))}

	for iter := 0; iter < opt.MaxIterations; iter++ {
		res.Iterations = iter + 1

		jacobian(f, x, r, j, trial, rTrial)
		a := j.GramTA()
		g := j.MulVecT(r) // ∇(½‖r‖²) = Jᵀr

		if maxAbs(g) < opt.GradientTol {
			res.Converged, res.Reason = true, "градиент близок к нулю"
			break
		}

		if lambda == 0 {
			// Seed the damping from the scale of the problem rather than from a
			// fixed number, so the first step is neither timid nor wild.
			var maxDiag float64
			for i := 0; i < n; i++ {
				maxDiag = math.Max(maxDiag, a.At(i, i))
			}
			lambda = opt.InitialDamping * maxDiag
			if lambda == 0 {
				lambda = opt.InitialDamping
			}
		}

		accepted := false
		for attempt := 0; attempt < 30; attempt++ {
			damped := a.Clone()
			for i := 0; i < n; i++ {
				// Marquardt scaling: damp each parameter in proportion to its
				// own curvature, with a floor so a flat direction is still
				// regularised.
				damped.Add(i, i, lambda*math.Max(a.At(i, i), 1e-12))
			}
			delta, err := SolveSPD(damped, negate(g))
			if err != nil {
				lambda *= 10
				continue
			}

			for i := 0; i < n; i++ {
				trial[i] = x[i] + delta[i]
			}
			f(trial, rTrial)
			newCost := dot(rTrial, rTrial)

			if newCost < cost && !math.IsNaN(newCost) {
				stepRel := Norm2(delta) / (Norm2(x) + 1e-30)
				costRel := (cost - newCost) / (cost + 1e-30)

				copy(x, trial)
				copy(r, rTrial)
				cost = newCost
				lambda = math.Max(lambda/10, 1e-15)
				accepted = true

				if stepRel < opt.StepTol {
					res.Converged, res.Reason = true, "шаг по параметрам стал пренебрежимо мал"
				} else if costRel < opt.CostTol {
					res.Converged, res.Reason = true, "невязка перестала уменьшаться"
				}
				break
			}
			lambda *= 10
		}

		if !accepted {
			res.Reason = "не удалось уменьшить невязку ни при каком демпфировании"
			break
		}
		if res.Converged {
			break
		}
	}

	res.Params = x
	res.Cost = cost
	res.RMS = math.Sqrt(cost / float64(m))
	if res.Reason == "" {
		res.Reason = "достигнут предел числа итераций"
	}
	return res, nil
}

// jacobian fills j with ∂r/∂x by central differences.
func jacobian(f ResidualFunc, x, r []float64, j *Matrix, scratchX, scratchR []float64) {
	n := len(x)
	m := len(r)
	minus := make([]float64, m)

	// ε^(1/3) is the step that balances truncation against round-off for a
	// central difference.
	const rel = 6.06e-6

	for c := 0; c < n; c++ {
		h := rel * math.Max(math.Abs(x[c]), 1)
		copy(scratchX, x)

		scratchX[c] = x[c] + h
		f(scratchX, scratchR)
		scratchX[c] = x[c] - h
		f(scratchX, minus)

		inv := 1 / (2 * h)
		for row := 0; row < m; row++ {
			j.Set(row, c, (scratchR[row]-minus[row])*inv)
		}
	}
}

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func negate(v []float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = -x
	}
	return out
}

func maxAbs(v []float64) float64 {
	var m float64
	for _, x := range v {
		if a := math.Abs(x); a > m {
			m = a
		}
	}
	return m
}
