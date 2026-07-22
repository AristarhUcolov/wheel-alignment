package vision

import (
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Target describes a printed checkerboard clamped to a wheel.
//
// The target's own frame has its origin at the centre of the pattern, X to the
// right and Y down when the board is viewed face-on, and Z out of the printed
// face. Putting the origin at the centre rather than at a corner matters for
// conditioning: the pose solve is far better behaved when the model points are
// spread symmetrically about zero.
type Target struct {
	Name string `json:"name"`

	// Cols and Rows count INNER corners — the crossings between squares, which
	// is what a detector finds. A board printed 9 squares by 7 has 8 by 6 inner
	// corners.
	Cols int `json:"cols"`
	Rows int `json:"rows"`

	// SquareMM is the printed size of one square. Measure it on the actual
	// print with a caliper rather than trusting the file: printers scale, and
	// a 2 % scale error is a 2 % range error and a proportional error in every
	// distance the system reports.
	SquareMM float64 `json:"square_mm"`

	// Position on the vehicle, if known.
	Wheel string `json:"wheel,omitempty"`
}

// Validate rejects a target that cannot give a well-conditioned pose.
func (t Target) Validate() error {
	switch {
	case t.Cols < 3 || t.Rows < 3:
		return fmt.Errorf("vision: мишень %dx%d слишком мала — нужно минимум 3x3 внутренних угла", t.Cols, t.Rows)
	case t.Cols == t.Rows:
		// A square board has a 90° rotational symmetry that no detector can
		// resolve, so the recovered pose can be a quarter-turn out. On a wheel
		// that swaps camber with toe.
		return fmt.Errorf("vision: мишень %dx%d квадратная — её ориентация неоднозначна с точностью до поворота на 90°, "+
			"из-за чего развал можно принять за схождение. Используйте прямоугольную, например 9x6",
			t.Cols, t.Rows)
	case (t.Cols+t.Rows)%2 == 0:
		// A checkerboard looks identical rotated 180°, and geometry alone can
		// never tell the two apart. The only thing that can is the pattern's own
		// colouring — and turning the board over swaps black squares for white
		// ones only when the two dimensions sum to an odd number. With an even
		// sum the board is genuinely, irreducibly ambiguous, and a detector that
		// silently guessed would flip the recovered pose at random between
		// frames, destroying runout compensation.
		return fmt.Errorf("vision: у мишени %dx%d сумма сторон чётная — такая доска выглядит одинаково "+
			"при повороте на 180°, и ориентацию невозможно определить в принципе. "+
			"Возьмите доску с нечётной суммой, например 9x6 или 7x6", t.Cols, t.Rows)
	case t.SquareMM <= 0:
		return fmt.Errorf("vision: не задан размер клетки мишени")
	}
	return nil
}

// ModelPoints returns the inner corners in the target frame, in millimetres,
// ordered row by row from top-left.
func (t Target) ModelPoints() []geom.Vec3 {
	out := make([]geom.Vec3, 0, t.Cols*t.Rows)
	ox := float64(t.Cols-1) * t.SquareMM / 2
	oy := float64(t.Rows-1) * t.SquareMM / 2
	for r := 0; r < t.Rows; r++ {
		for c := 0; c < t.Cols; c++ {
			out = append(out, geom.V(
				float64(c)*t.SquareMM-ox,
				float64(r)*t.SquareMM-oy,
				0,
			))
		}
	}
	return out
}

// DiagonalMM is the corner-to-corner span of the pattern. Pose accuracy scales
// with this over the viewing distance, so it is the number to grow when the
// angles come out noisy.
func (t Target) DiagonalMM() float64 {
	w := float64(t.Cols-1) * t.SquareMM
	h := float64(t.Rows-1) * t.SquareMM
	return geom.V(w, h, 0).Len()
}

// DefaultTarget is a 9×6 board of 30 mm squares: about 240 × 150 mm of pattern,
// which fits on one A4 sheet with a margin and is large enough to give useful
// angular resolution at the two to three metres a camera stands from a wheel.
func DefaultTarget() Target {
	return Target{Name: "A4 9x6 / 30 мм", Cols: 9, Rows: 6, SquareMM: 30}
}

// Correspondences pairs this target's model points with detected image points.
// The slices must be the same length and in the same order.
func (t Target) Correspondences(detected []Point2) ([]Correspondence, error) {
	model := t.ModelPoints()
	if len(detected) != len(model) {
		return nil, fmt.Errorf("vision: мишень имеет %d углов, а распознано %d", len(model), len(detected))
	}
	out := make([]Correspondence, len(model))
	for i := range model {
		out[i] = Correspondence{Model: model[i], Image: detected[i]}
	}
	return out, nil
}
