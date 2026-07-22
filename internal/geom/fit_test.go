package geom_test

import (
	"math"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// TestAxisAngleRoundTrip covers the matrix logarithm, including the θ ≈ π case
// where the antisymmetric part vanishes and the axis has to come from the
// symmetric part instead.
func TestAxisAngleRoundTrip(t *testing.T) {
	axes := []geom.Vec3{
		{X: 0, Y: 0, Z: 1}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0},
		{X: 1, Y: 2, Z: -3}, {X: -0.3, Y: 0.9, Z: 0.1},
	}
	angles := []float64{0.001, 0.5, 1.5, 3.0, math.Pi - 1e-8, math.Pi}

	for _, ax := range axes {
		u := ax.Unit()
		for _, ang := range angles {
			r := geom.Rodrigues(u, ang)
			gotAx, gotAng := geom.AxisAngle(r)

			if d := math.Abs(gotAng - ang); d > 1e-7 {
				t.Errorf("axis %v angle %.9f: recovered angle %.9f", u, ang, gotAng)
			}
			// At exactly π the axis sign is genuinely ambiguous, so compare the
			// rotations rather than the axes.
			back := geom.Rodrigues(gotAx, gotAng)
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if math.Abs(back[i][j]-r[i][j]) > 1e-7 {
						t.Fatalf("axis %v angle %.6f: round trip differs at [%d][%d]", u, ang, i, j)
					}
				}
			}
		}
	}
}

// TestOrthonormalize pulls a perturbed matrix back onto SO(3). Pose solves and
// averaged rotations drift off the manifold just enough to poison angle
// extraction, and this is what catches them.
func TestOrthonormalize(t *testing.T) {
	r := geom.RotZ(0.7).Mul(geom.RotX(-0.4))
	dirty := r
	dirty[0][1] += 0.02
	dirty[2][0] -= 0.015
	dirty[1][1] += 0.01

	clean := geom.Orthonormalize(dirty)
	if d := math.Abs(clean.Det() - 1); d > 1e-12 {
		t.Errorf("determinant %.15f, want 1", clean.Det())
	}
	should := clean.T().Mul(clean)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			want := 0.0
			if i == j {
				want = 1
			}
			if math.Abs(should[i][j]-want) > 1e-12 {
				t.Fatalf("RᵀR is not identity at [%d][%d]: %.15f", i, j, should[i][j])
			}
		}
	}
	// A reflection must be repaired into a rotation, not passed through.
	flipped := geom.Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, -1}}
	if geom.Orthonormalize(flipped).Det() < 0 {
		t.Error("Orthonormalize returned a reflection")
	}
}

// TestFitPlane recovers a known plane from noisy samples and reports honest
// diagnostics for a point cloud that has no plane at all.
func TestFitPlane(t *testing.T) {
	n := geom.V(0.2, -0.3, 1).Unit()
	origin := geom.V(10, -5, 3)
	e1 := n.Any()
	e2 := n.Cross(e1).Unit()

	var pts []geom.Vec3
	for i := 0; i < 12; i++ {
		a := float64(i) * 0.7
		pts = append(pts, origin.
			Add(e1.Scale(math.Cos(a)*100)).
			Add(e2.Scale(math.Sin(a)*80)))
	}
	fit, err := geom.FitPlane(pts)
	if err != nil {
		t.Fatal(err)
	}
	if ang := geom.Deg(fit.Normal.AngleTo(n)); ang > 1e-9 && math.Abs(ang-180) > 1e-9 {
		t.Errorf("plane normal off by %.9f°", ang)
	}
	if fit.RMS > 1e-9 {
		t.Errorf("rms %.9f on exactly planar input", fit.RMS)
	}
	if fit.Flatness > 1e-6 {
		t.Errorf("flatness %.9f on exactly planar input", fit.Flatness)
	}

	if _, err := geom.FitPlane([]geom.Vec3{{}, {X: 1}}); err == nil {
		t.Error("two points do not determine a plane")
	}
}

// TestFitRotationAxis is the runout-compensation primitive: given poses of a
// body pinned to a fixed axis, recover that axis as a line in space. Both the
// direction and the position must come back, and the position is the part the
// least-squares fit exists for.
func TestFitRotationAxis(t *testing.T) {
	dir := geom.V(-0.08, 0.05, 1).Unit() // a steering axis: caster and SAI
	pivot := geom.V(120, -340, 55)

	var poses []geom.Pose
	for _, ang := range []float64{-0.35, -0.18, 0, 0.2, 0.4} {
		r := geom.Rodrigues(dir, ang)
		// A body rotating about a line through `pivot` has translation
		// (I − R)·pivot, which is exactly what the fit inverts.
		poses = append(poses, geom.Pose{R: r, T: pivot.Sub(r.MulVec(pivot))})
	}

	fit, err := geom.FitRotationAxis(poses)
	if err != nil {
		t.Fatal(err)
	}
	if ang := geom.Deg(fit.Direction.AngleTo(dir)); ang > 1e-6 {
		t.Errorf("axis direction off by %.9f°", ang)
	}
	// The returned point is the one nearest the origin, so compare the lines
	// rather than the points: the offset from pivot must lie along the axis.
	off := fit.Point.Sub(pivot)
	perp := off.Sub(dir.Scale(off.Dot(dir)))
	if perp.Len() > 1e-6 {
		t.Errorf("axis line misses the true pivot by %.9f mm", perp.Len())
	}
	if fit.Residual > 1e-6 {
		t.Errorf("residual %.9f on exact input", fit.Residual)
	}
	if geom.Deg(fit.Sweep) < 40 {
		t.Errorf("reported sweep %.2f°, expected about 43°", geom.Deg(fit.Sweep))
	}

	// A body that never moved constrains nothing and must be refused rather
	// than answered with noise.
	still := []geom.Pose{geom.IdentityPose(), geom.IdentityPose(), geom.IdentityPose()}
	if _, err := geom.FitRotationAxis(still); err == nil {
		t.Error("a stationary body has no axis of rotation")
	}
}

// TestFitConeAxis is the optical caster measurement in miniature: a direction
// swept about a fixed axis traces a cone, and the cone's axis is recoverable
// from the directions alone — no knowledge of how far it was swept.
func TestFitConeAxis(t *testing.T) {
	axis := geom.V(-0.07, 0.22, 1).Unit()
	spin := geom.V(0.01, 1, -0.02).Unit()

	var dirs []geom.Vec3
	for _, ang := range []float64{-0.35, -0.1, 0.15, 0.38} {
		dirs = append(dirs, geom.Rodrigues(axis, ang).MulVec(spin))
	}
	got, err := geom.FitConeAxis(dirs, geom.V(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if ang := geom.Deg(got.AngleTo(axis)); ang > 1e-6 {
		t.Errorf("cone axis off by %.9f°", ang)
	}
	if got.Z < 0 {
		t.Error("the hint should have fixed the sign upward")
	}

	// Barely any sweep: the plane through the tips is arbitrary, so refuse.
	tiny := []geom.Vec3{
		geom.Rodrigues(axis, -0.001).MulVec(spin),
		spin,
		geom.Rodrigues(axis, 0.001).MulVec(spin),
	}
	if _, err := geom.FitConeAxis(tiny, geom.V(0, 0, 1)); err == nil {
		t.Error("a 0.1° sweep should be refused, not answered")
	}
}
