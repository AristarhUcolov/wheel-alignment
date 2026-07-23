package vision

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/numeric"
)

var (
	ErrNoCorners   = errors.New("vision: на снимке не найдено углов шахматного узора")
	ErrNoGrid      = errors.New("vision: найденные углы не складываются в сетку заданного размера")
	ErrGridPartial = errors.New("vision: мишень видна не полностью")
)

// DetectOptions tunes checkerboard detection.
type DetectOptions struct {
	Target Target

	// BlurSigma smooths the image before differentiating. Second derivatives
	// amplify noise fiercely, so some blur is mandatory; too much merges
	// neighbouring corners. About a quarter of the on-screen square size is a
	// good value. Default 1.2 px.
	BlurSigma float64

	// NMSRadius is the minimum separation between detected corners, in pixels.
	// It must be smaller than the on-screen square size and larger than the
	// blur radius. Default 5 px, which works down to squares of roughly 14 px.
	NMSRadius int

	// RefineWindow is the half-width of the window used for subpixel
	// refinement. Default 4, giving a 9×9 patch.
	RefineWindow int

	// MaxCandidates caps how many corner peaks are considered. The strongest
	// are kept. Default is four times the number of corners on the target,
	// which leaves room for clutter without letting the grid search explode.
	MaxCandidates int
}

func (o DetectOptions) withDefaults() DetectOptions {
	if o.BlurSigma <= 0 {
		o.BlurSigma = 1.2
	}
	if o.NMSRadius <= 0 {
		o.NMSRadius = 5
	}
	if o.RefineWindow <= 0 {
		o.RefineWindow = 4
	}
	if o.MaxCandidates <= 0 {
		o.MaxCandidates = 4 * o.Target.Cols * o.Target.Rows
	}
	return o
}

// Detection is a located checkerboard.
type Detection struct {
	// Corners are the inner corners in target order — row by row from the
	// origin corner — so they pair one-to-one with Target.ModelPoints().
	Corners []Point2

	// CandidatesFound is how many corner peaks the response stage produced
	// before the grid was fitted. Far more than the target has means clutter;
	// far fewer means the pattern was not resolved.
	CandidatesFound int

	// GridRMSPx is how far the corners sit from a perfect projected grid. It
	// measures detection quality on its own, before any camera model is
	// involved, which makes it the right number to look at when deciding
	// whether a bad pose is the detector's fault or the calibration's.
	GridRMSPx float64

	// MeanSpacingPx is the average distance between neighbouring corners.
	MeanSpacingPx float64

	Warnings []string
}

// DetectCheckerboard finds a checkerboard and returns its inner corners in
// target order.
//
// # How the corners are found
//
// A checkerboard's inner corner is a saddle point of image intensity: along one
// diagonal the brightness rises, along the other it falls. That is an exact
// statement about the pattern, not a heuristic, and it gives a response with no
// free parameters —
//
//	R = I_xy² − I_xx·I_yy
//
// which is the negated determinant of the Hessian, positive exactly where the
// surface is a saddle. Edges, blobs and flat areas all score at or below zero.
//
// Subpixel position comes from fitting a quadratic surface to the intensity in
// a small window and solving for where its gradient vanishes. Near an X
// junction the intensity really is close to a hyperbolic paraboloid, so the fit
// is not an approximation of convenience — it matches the physics of the
// pattern, and it is what delivers hundredths of a pixel.
//
// # How the corners are ordered
//
// Finding points is only half the job: they have to be matched to the right
// model points, or the pose is confidently wrong. The corners are enclosed in
// their convex hull, the largest quadrilateral inscribed in that hull is taken
// as the board's outline, and a homography from the ideal grid to that
// quadrilateral is used to label every corner. Wrong labellings fail the
// coverage check and the next-largest quadrilateral is tried, so stray corners
// outside the board do not derail the search.
//
// The last ambiguity is rotation by 180°, which no amount of geometry can
// resolve — a checkerboard looks identical upside down. It is resolved from the
// black-and-white pattern itself, which is why Target.Validate insists the two
// dimensions sum to an odd number: only then does turning the board over swap
// black squares for white ones.
func DetectCheckerboard(img *Gray, opt DetectOptions) (Detection, error) {
	opt = opt.withDefaults()
	if err := opt.Target.Validate(); err != nil {
		return Detection{}, err
	}
	work := img.Normalize().Blur(opt.BlurSigma)

	corners, det, err := detectCorners(work, opt)
	if err != nil {
		return det, err
	}
	fitted, _, ok := fitBoard(work, corners, opt.Target)
	if !ok {
		return det, fmt.Errorf("%w: кандидатов %d, требуется сетка %dx%d",
			ErrNoGrid, len(corners), opt.Target.Cols, opt.Target.Rows)
	}
	fitted.CandidatesFound = len(corners)
	return fitted, nil
}

// detectCorners finds and cleans the checkerboard corner cloud, without yet
// committing to any particular board layout. It is the shared front half of the
// detector: single-board detection fits one grid to its output, multi-board
// detection fits several. Splitting here is what lets one image hold a wheel
// target and a floor reference target and be read for both.
func detectCorners(work *Gray, opt DetectOptions) ([]Point2, Detection, error) {
	var det Detection
	peaks := saddlePeaks(work, opt.NMSRadius, opt.MaxCandidates)
	det.CandidatesFound = len(peaks)
	if len(peaks) < 8 {
		return nil, det, fmt.Errorf("%w: найдено всего %d углов. Проверьте резкость, освещение и что мишень в кадре",
			ErrNoCorners, len(peaks))
	}

	// A rough square size from the strongest peaks, used only to merge near-
	// duplicate detections. It is measured over the strongest handful because
	// weak responses along a board's edges cluster near real corners and drag a
	// global median down to a fraction of the true spacing.
	strongest := peaks
	if len(strongest) > 40 {
		strongest = strongest[:40]
	}
	spacing := medianNearestNeighbour(strongest)
	if spacing < 4 {
		return nil, det, fmt.Errorf("%w: клетки мишени неразличимы (оценка шага %.1f пикс)", ErrNoCorners, spacing)
	}

	corners := refineAll(work, peaks, opt.RefineWindow)
	corners = mergeDuplicates(corners, spacing*0.25)

	// Every threshold from here is per-corner, against that corner's own nearest
	// neighbours, not one figure for the whole frame. A tilted board does not
	// have a single square size — at 35° the near edge is a fifth wider than the
	// far edge — and with two boards at different depths there is certainly no
	// common scale. Local scales are the only thing that works for both.
	corners = filterCrossings(work, corners, localScales(corners), 0.30)
	corners = filterConnected(corners, localScales(corners), 1.35)
	if len(corners) < 4 {
		return nil, det, fmt.Errorf("%w: после отсева осталось %d настоящих пересечений клеток",
			ErrNoCorners, len(corners))
	}
	det.MeanSpacingPx = medianNearestNeighbour(corners)
	return corners, det, nil
}

// fitBoard recovers one board of the given layout from a corner cloud. It
// returns the ordered corners, the corners it consumed (so a caller looking for
// a second board can set them aside), and whether it succeeded.
func fitBoard(work *Gray, corners []Point2, target Target) (Detection, []Point2, bool) {
	cols, rows := target.Cols, target.Rows
	ordered, h, ok := fitGrid(corners, cols, rows)
	if !ok {
		return Detection{}, nil, false
	}

	// Resolve the 180° ambiguity from the pattern's own colours.
	var det Detection
	if flipped, err := needsHalfTurn(work, h, cols, rows); err != nil {
		det.Warnings = append(det.Warnings, err.Error())
	} else if flipped {
		ordered = halfTurn(ordered, cols, rows)
		h, _ = gridHomography(ordered, cols, rows)
	}

	det.Corners = ordered
	det.GridRMSPx, det.MeanSpacingPx = gridQuality(ordered, h, cols, rows)
	if det.GridRMSPx > 1.0 {
		det.Warnings = append(det.Warnings, fmt.Sprintf(
			"Углы ложатся на проективную сетку с разбросом %.2f пикс — это много. "+
				"Возможна смазанность, отражения на мишени или изгиб листа: мишень должна быть жёсткой и плоской.",
			det.GridRMSPx))
	}
	if det.MeanSpacingPx < 12 {
		det.Warnings = append(det.Warnings, fmt.Sprintf(
			"Клетка мишени занимает всего %.0f пикс — субпиксельное уточнение на таком масштабе работает плохо. "+
				"Подойдите ближе или возьмите мишень крупнее.", det.MeanSpacingPx))
	}
	return det, ordered, true
}

// DetectBoards finds several checkerboards of different layouts in one image —
// the wheel target and the fixed floor reference, together in every frame of a
// full optical alignment. The boards must differ in size (cols×rows or square
// count), because two identical boards in one frame cannot be told apart and,
// laid near each other, would be grown into a single lattice.
//
// The corner cloud is found once and shared. Boards are fitted largest-first,
// and each board's corners are removed before the next is sought, so a corner is
// never claimed by two boards. The result is aligned one-to-one with targets;
// a board that could not be found is a nil-Corners entry with an error in the
// returned slice.
func DetectBoards(img *Gray, targets []Target, opt DetectOptions) ([]Detection, []error) {
	errs := make([]error, len(targets))
	dets := make([]Detection, len(targets))
	for i, t := range targets {
		if err := t.Validate(); err != nil {
			errs[i] = err
		}
	}

	opt = opt.withDefaults()
	if opt.MaxCandidates < 4*totalCorners(targets) {
		opt.MaxCandidates = 4 * totalCorners(targets)
	}
	work := img.Normalize().Blur(opt.BlurSigma)

	corners, base, err := detectCorners(work, opt)
	if err != nil {
		for i := range errs {
			if errs[i] == nil {
				errs[i] = err
			}
		}
		return dets, errs
	}

	// Largest board first, and this ordering is load-bearing rather than a
	// preference. A smaller layout can sit entirely inside a larger one — an
	// 8×5 lattice is a sub-window of a 9×6 — so searching for the small board
	// first can latch it onto part of the big board and return a confident,
	// completely wrong pose. Fitting the largest first cannot go that way round,
	// because a board has too few corners to host a bigger lattice, and removing
	// its corners before the next search leaves the small board only its own.
	order := make([]int, len(targets))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return targets[order[a]].Cols*targets[order[a]].Rows > targets[order[b]].Cols*targets[order[b]].Rows
	})

	remaining := corners
	for _, idx := range order {
		if errs[idx] != nil {
			continue
		}
		det, used, ok := fitBoard(work, remaining, targets[idx])
		if !ok {
			errs[idx] = fmt.Errorf("%w: мишень %dx%d не найдена среди %d углов",
				ErrNoGrid, targets[idx].Cols, targets[idx].Rows, len(remaining))
			continue
		}
		det.CandidatesFound = base.CandidatesFound
		dets[idx] = det
		remaining = removePoints(remaining, used)
	}
	return dets, errs
}

func totalCorners(targets []Target) int {
	n := 0
	for _, t := range targets {
		n += t.Cols * t.Rows
	}
	return n
}

// removePoints returns src with every point in drop removed. The points in drop
// are the exact cloud values fitBoard consumed, so identity comparison is right.
func removePoints(src, drop []Point2) []Point2 {
	gone := make(map[Point2]bool, len(drop))
	for _, p := range drop {
		gone[p] = true
	}
	out := src[:0:0]
	for _, p := range src {
		if !gone[p] {
			out = append(out, p)
		}
	}
	return out
}

// saddlePeaks computes the saddle response and returns the strongest local
// maxima, sorted by strength.
func saddlePeaks(img *Gray, radius, maxCount int) []Point2 {
	w, h := img.W, img.H
	resp := NewGray(w, h)
	var maxResp float64

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := img.At(x, y)
			ixx := img.At(x+1, y) - 2*c + img.At(x-1, y)
			iyy := img.At(x, y+1) - 2*c + img.At(x, y-1)
			ixy := (img.At(x+1, y+1) - img.At(x+1, y-1) - img.At(x-1, y+1) + img.At(x-1, y-1)) / 4

			// Negated Hessian determinant: positive only at saddles.
			r := ixy*ixy - ixx*iyy
			if r > 0 {
				resp.Pix[y*w+x] = r
				if r > maxResp {
					maxResp = r
				}
			}
		}
	}
	if maxResp <= 0 {
		return nil
	}

	type peak struct {
		p Point2
		v float64
	}
	// A floor at a small fraction of the strongest response drops the noise
	// floor without assuming anything about how many corners there should be.
	floor := maxResp * 0.01
	var peaks []peak

	margin := radius + 2
	for y := margin; y < h-margin; y++ {
		for x := margin; x < w-margin; x++ {
			v := resp.Pix[y*w+x]
			if v < floor {
				continue
			}
			isMax := true
			for dy := -radius; dy <= radius && isMax; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					if resp.At(x+dx, y+dy) > v {
						isMax = false
						break
					}
				}
			}
			if isMax {
				peaks = append(peaks, peak{Point2{float64(x), float64(y)}, v})
			}
		}
	}

	sort.Slice(peaks, func(i, j int) bool { return peaks[i].v > peaks[j].v })
	if len(peaks) > maxCount {
		peaks = peaks[:maxCount]
	}
	out := make([]Point2, len(peaks))
	for i, p := range peaks {
		out[i] = p.p
	}
	return out
}

// quadFitter precomputes the least-squares operator for fitting
// I ≈ a₀ + a₁x + a₂y + a₃x² + a₄xy + a₅y² over a fixed window, so the per-corner
// work is a single matrix-vector product.
type quadFitter struct {
	w  int
	op *numeric.Matrix // 6 × N
}

func newQuadFitter(w int) (*quadFitter, error) {
	n := (2*w + 1) * (2*w + 1)
	a := numeric.New(n, 6)
	i := 0
	for dy := -w; dy <= w; dy++ {
		for dx := -w; dx <= w; dx++ {
			x, y := float64(dx), float64(dy)
			a.Set(i, 0, 1)
			a.Set(i, 1, x)
			a.Set(i, 2, y)
			a.Set(i, 3, x*x)
			a.Set(i, 4, x*y)
			a.Set(i, 5, y*y)
			i++
		}
	}
	ata := a.GramTA()
	op := numeric.New(6, n)
	col := make([]float64, 6)
	for k := 0; k < n; k++ {
		for r := 0; r < 6; r++ {
			col[r] = a.At(k, r) // the k-th column of Aᵀ
		}
		x, err := numeric.SolveSPD(ata, col)
		if err != nil {
			return nil, err
		}
		for r := 0; r < 6; r++ {
			op.Set(r, k, x[r])
		}
	}
	return &quadFitter{w: w, op: op}, nil
}

// refine locates the saddle point of the fitted surface, in image coordinates.
func (q *quadFitter) refine(img *Gray, p Point2) (Point2, bool) {
	patch := make([]float64, q.op.Cols)
	i := 0
	for dy := -q.w; dy <= q.w; dy++ {
		for dx := -q.w; dx <= q.w; dx++ {
			patch[i] = img.Sample(p.X+float64(dx), p.Y+float64(dy))
			i++
		}
	}
	var c [6]float64
	for r := 0; r < 6; r++ {
		var s float64
		for k := 0; k < q.op.Cols; k++ {
			s += q.op.At(r, k) * patch[k]
		}
		c[r] = s
	}

	// ∇I = 0:  [2a₃  a₄ ][x]   [−a₁]
	//          [ a₄ 2a₅ ][y] = [−a₂]
	det := 4*c[3]*c[5] - c[4]*c[4]
	// A genuine checkerboard corner is a saddle, so this determinant must be
	// negative. A positive one means the fit found a peak or a pit, which is
	// some other structure entirely — a highlight, a bolt head, a speck of dirt.
	if det >= -1e-12 {
		return p, false
	}
	dx := (-2*c[5]*c[1] + c[4]*c[2]) / det
	dy := (c[4]*c[1] - 2*c[3]*c[2]) / det

	limit := float64(q.w)
	if math.Abs(dx) > limit || math.Abs(dy) > limit || math.IsNaN(dx) || math.IsNaN(dy) {
		return p, false
	}
	return Point2{p.X + dx, p.Y + dy}, true
}

func refineAll(img *Gray, peaks []Point2, window int) []Point2 {
	q, err := newQuadFitter(window)
	if err != nil {
		return peaks
	}
	margin := float64(window + 1)
	out := make([]Point2, 0, len(peaks))
	for _, p := range peaks {
		if !img.InBounds(p.X, p.Y, margin) {
			continue
		}
		// Two passes: the first moves the window onto the corner, the second
		// fits with the corner centred, which is where the quadratic model of
		// the intensity is most nearly true.
		r, ok := q.refine(img, p)
		if !ok {
			continue
		}
		if r2, ok2 := q.refine(img, r); ok2 {
			r = r2
		}
		if img.InBounds(r.X, r.Y, margin) {
			out = append(out, r)
		}
	}
	return out
}

// medianNearestNeighbour estimates the on-screen square size from the corners
// themselves. On a checkerboard every corner's nearest neighbour is the corner
// one square away, so the median of those distances is the square size — which
// means the detector calibrates its own length scale from the image and needs
// no advance knowledge of how far away the board is.
func medianNearestNeighbour(pts []Point2) float64 {
	if len(pts) < 2 {
		return 0
	}
	d := make([]float64, len(pts))
	for i, p := range pts {
		best := math.Inf(1)
		for j, q := range pts {
			if i == j {
				continue
			}
			if dist := p.DistTo(q); dist < best {
				best = dist
			}
		}
		d[i] = best
	}
	sort.Float64s(d)
	return d[len(d)/2]
}

// localScales estimates the on-screen square size in the neighbourhood of each
// point, as the median of its four nearest neighbour distances.
//
// The obvious choice — the single nearest neighbour — is not robust, and its
// failure is nasty. One spurious response eight pixels from a real corner makes
// that corner's nearest-neighbour distance eight instead of thirty; every
// threshold scaled from it then collapses, the sampling ring shrinks to nothing
// and the connectivity reach no longer stretches to the corner's genuine
// neighbours. A single intruder does not merely add a bad point, it destroys a
// good one.
//
// The median of four survives up to two intruders. For a real corner the four
// smallest distances are two orthogonal neighbours at one square and two
// diagonals at √2, so the result lands around 1.2 squares — a consistent
// overestimate the callers' coefficients are chosen against.
func localScales(pts []Point2) []float64 {
	const k = 4
	out := make([]float64, len(pts))
	d := make([]float64, 0, len(pts))

	for i, p := range pts {
		d = d[:0]
		for j, q := range pts {
			if i != j {
				d = append(d, p.DistTo(q))
			}
		}
		if len(d) == 0 {
			out[i] = math.Inf(1)
			continue
		}
		n := k
		if len(d) < n {
			n = len(d)
		}
		sort.Float64s(d)
		out[i] = d[(n-1)/2]
	}
	return out
}

// filterCrossings keeps only true checkerboard crossings.
//
// Walk a circle around a candidate and watch the brightness. At a real X
// junction the circle passes through four quadrants, alternating dark, light,
// dark, light — exactly four crossings of the local mean. At a T junction,
// where two squares of the pattern meet the paper's border, the circle sees
// three regions and crosses the mean twice. At the board's outer corner, once
// or twice. Counting sign changes therefore separates the pattern's interior
// from its edge with no threshold to tune.
//
// The ring radius is a fraction of each corner's OWN local scale rather than of
// one figure for the board, so that a tilted board — which is what a useful
// calibration shot is — does not have the ring spilling into the next square at
// its far edge.
func filterCrossings(img *Gray, pts []Point2, scales []float64, fraction float64) []Point2 {
	const samples = 32
	ring := make([]float64, samples)

	out := pts[:0:0]
	for pi, p := range pts {
		radius := fraction * scales[pi]
		if radius < 1.5 || math.IsInf(radius, 0) {
			continue
		}
		if !img.InBounds(p.X, p.Y, radius+2) {
			continue
		}
		var sum, lo, hi float64
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 0; i < samples; i++ {
			a := 2 * math.Pi * float64(i) / samples
			v := img.Sample(p.X+radius*math.Cos(a), p.Y+radius*math.Sin(a))
			ring[i] = v
			sum += v
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		// A flat ring is noise, not a corner.
		if hi-lo < 0.15 {
			continue
		}
		mean := sum / samples
		changes := 0
		prev := ring[samples-1] >= mean
		for i := 0; i < samples; i++ {
			cur := ring[i] >= mean
			if cur != prev {
				changes++
			}
			prev = cur
		}
		if changes == 4 {
			out = append(out, p)
		}
	}
	return out
}

// mergeDuplicates collapses candidates that refined onto the same corner.
//
// Non-maximum suppression only guarantees peaks are a few pixels apart, so one
// physical corner can raise two of them, and subpixel refinement then pulls
// both onto very nearly the same spot. Left alone, the near-twin competes with
// the real corner for its grid slot and can win, putting a several-pixel error
// into an otherwise perfect detection. Input is assumed ordered by response
// strength, so the survivor is the stronger of each pair.
func mergeDuplicates(pts []Point2, minSep float64) []Point2 {
	if minSep <= 0 {
		return pts
	}
	out := make([]Point2, 0, len(pts))
	for _, p := range pts {
		dup := false
		for _, q := range out {
			if p.DistTo(q) < minSep {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return out
}

// filterConnected keeps only corners that sit in a grid.
//
// Every corner of a checkerboard has neighbours one square away: two at the
// pattern's own corners, three along its edges, four inside. A response raised
// by sensor noise, a bolt head or a reflection has none — nothing else in the
// frame happens to lie exactly one square size away from it.
//
// The reach is a multiple of each corner's own local scale, so a tilted board
// keeps its far-edge corners. Since that scale already sits around 1.2 squares,
// a reach of 1.35 spans roughly 1.6 squares — enough to hold the orthogonal
// neighbours through heavy foreshortening, at the cost of sometimes counting a
// diagonal too. That trade is the right way round: admitting a diagonal only
// makes this filter more permissive, and it is the second line of defence. The
// ring test above is what actually rejects clutter, and the lattice growth
// downstream ignores whatever gets through.
func filterConnected(pts []Point2, scales []float64, reach float64) []Point2 {
	out := pts[:0:0]
	for i, p := range pts {
		if math.IsInf(scales[i], 0) {
			continue
		}
		limit := reach * scales[i]
		n := 0
		for j, q := range pts {
			if i == j {
				continue
			}
			if p.DistTo(q) <= limit {
				n++
				if n >= 2 {
					break
				}
			}
		}
		if n >= 2 {
			out = append(out, p)
		}
	}
	return out
}

// ─── Grid recovery ───────────────────────────────────────────────────────────

// idealGrid returns the model grid nodes in row-major order: (col, row).
func idealGrid(cols, rows int) []Point2 {
	out := make([]Point2, 0, cols*rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out = append(out, Point2{float64(c), float64(r)})
		}
	}
	return out
}

// fitGrid labels detected corners with their grid positions.
//
// Growing the lattice from a seed comes first: it asks only local questions and
// so is indifferent to how much clutter is in the frame. The convex-hull search
// stays as a fallback for the case where growth cannot get started — a board so
// foreshortened or so sparsely detected that no seed has four usable
// neighbours — since on a clean detection the hull genuinely is the board.
//
// Both routes end at the same place: a candidate quadrilateral fed to
// assignByQuad, which tries the four rotations of the board's one physically
// possible winding and accepts only a labelling that covers every grid node.
func fitGrid(corners []Point2, cols, rows int) ([]Point2, geom.Mat3, bool) {
	sign := gridWindingSign(cols, rows)

	for _, quad := range growGridQuads(corners, cols, rows) {
		for _, oriented := range orientations(quad, sign) {
			if ordered, h, ok := assignByQuad(corners, oriented, cols, rows); ok {
				return ordered, h, true
			}
		}
	}

	hull := convexHull(corners)
	if len(hull) < 4 {
		return nil, geom.Mat3{}, false
	}
	for _, quad := range candidateQuads(hull) {
		for _, oriented := range orientations(quad, sign) {
			ordered, h, ok := assignByQuad(corners, oriented, cols, rows)
			if ok {
				return ordered, h, true
			}
		}
	}
	return nil, geom.Mat3{}, false
}

// assignByQuad maps the ideal grid's four outer nodes onto the given
// quadrilateral, labels every corner through the resulting homography, and then
// re-fits using all the labelled pairs.
func assignByQuad(corners []Point2, quad [4]Point2, cols, rows int) ([]Point2, geom.Mat3, bool) {
	outer := []Point2{
		{0, 0}, {float64(cols - 1), 0},
		{float64(cols - 1), float64(rows - 1)}, {0, float64(rows - 1)},
	}
	h, err := homographyDLT(outer, quad[:])
	if err != nil {
		return nil, geom.Mat3{}, false
	}

	ordered, ok := labelCorners(corners, h, cols, rows, 0.35)
	if !ok {
		return nil, geom.Mat3{}, false
	}
	// Refit from all matches — four points make a homography that is exact at
	// the corners and drifts in the middle, which matters once perspective is
	// strong.
	h2, err := gridHomography(ordered, cols, rows)
	if err != nil {
		return ordered, h, true
	}
	if refined, ok := labelCorners(corners, h2, cols, rows, 0.3); ok {
		return refined, h2, true
	}
	return ordered, h, true
}

// labelCorners assigns each grid node the nearest detected corner, requiring
// every node to be filled and no corner to serve two nodes.
func labelCorners(corners []Point2, h geom.Mat3, cols, rows int, tol float64) ([]Point2, bool) {
	hInv, ok := h.Inv()
	if !ok {
		return nil, false
	}
	grid := make([]Point2, cols*rows)
	filled := make([]bool, cols*rows)
	dist := make([]float64, cols*rows)
	used := make([]bool, len(corners))

	for ci, c := range corners {
		g := applyH(hInv, c)
		gx, gy := math.Round(g.X), math.Round(g.Y)
		if gx < 0 || gy < 0 || int(gx) >= cols || int(gy) >= rows {
			continue
		}
		d := math.Hypot(g.X-gx, g.Y-gy)
		if d > tol {
			continue
		}
		idx := int(gy)*cols + int(gx)
		if filled[idx] && dist[idx] <= d {
			continue
		}
		grid[idx], filled[idx], dist[idx] = c, true, d
		used[ci] = true
	}
	for _, f := range filled {
		if !f {
			return nil, false
		}
	}
	return grid, true
}

// gridHomography fits the grid-to-image homography from all labelled corners.
func gridHomography(ordered []Point2, cols, rows int) (geom.Mat3, error) {
	return homographyDLT(idealGrid(cols, rows), ordered)
}

func applyH(h geom.Mat3, p Point2) Point2 {
	v := h.MulVec(geom.V(p.X, p.Y, 1))
	if math.Abs(v.Z) < 1e-12 {
		return Point2{math.Inf(1), math.Inf(1)}
	}
	return Point2{v.X / v.Z, v.Y / v.Z}
}

// halfTurn relabels the grid as if the board had been rotated 180°.
func halfTurn(ordered []Point2, cols, rows int) []Point2 {
	out := make([]Point2, len(ordered))
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out[r*cols+c] = ordered[(rows-1-r)*cols+(cols-1-c)]
		}
	}
	return out
}

// needsHalfTurn decides whether the board was labelled upside down, by reading
// the colours of the squares.
//
// The square whose corners are grid nodes (c,r)…(c+1,r+1) is dark when c+r is
// even, by the convention of Target.ModelPoints. Turning the board 180° maps
// square (c,r) to (cols−2−c, rows−2−r), which flips that parity exactly when
// cols+rows is odd — the reason Target.Validate requires it. Sampling the
// square centres therefore settles the orientation outright.
func needsHalfTurn(img *Gray, h geom.Mat3, cols, rows int) (bool, error) {
	var evenSum, oddSum float64
	var evenN, oddN int
	for r := 0; r < rows-1; r++ {
		for c := 0; c < cols-1; c++ {
			p := applyH(h, Point2{float64(c) + 0.5, float64(r) + 0.5})
			if !img.InBounds(p.X, p.Y, 1) {
				continue
			}
			v := img.Sample(p.X, p.Y)
			if (c+r)%2 == 0 {
				evenSum += v
				evenN++
			} else {
				oddSum += v
				oddN++
			}
		}
	}
	if evenN == 0 || oddN == 0 {
		return false, errors.New("не удалось прочитать цвета клеток — ориентация мишени определена только по геометрии " +
			"и может быть повёрнута на 180°")
	}
	evenMean, oddMean := evenSum/float64(evenN), oddSum/float64(oddN)
	if math.Abs(evenMean-oddMean) < 0.05 {
		return false, fmt.Errorf("клетки мишени почти не различаются по яркости (%.3f и %.3f) — "+
			"ориентация может быть определена с поворотом на 180°. Проверьте освещение и контраст мишени",
			evenMean, oddMean)
	}
	// Convention: squares with even (c+r) are the dark ones.
	return evenMean > oddMean, nil
}

// gridQuality reports how well the corners fit a projective grid, and their
// mean spacing.
func gridQuality(ordered []Point2, h geom.Mat3, cols, rows int) (rms, spacing float64) {
	ideal := idealGrid(cols, rows)
	var ss float64
	for i, g := range ideal {
		d := applyH(h, g).DistTo(ordered[i])
		ss += d * d
	}
	rms = math.Sqrt(ss / float64(len(ideal)))

	var sum float64
	var n int
	for r := 0; r < rows; r++ {
		for c := 0; c < cols-1; c++ {
			sum += ordered[r*cols+c].DistTo(ordered[r*cols+c+1])
			n++
		}
	}
	if n > 0 {
		spacing = sum / float64(n)
	}
	return rms, spacing
}

// ─── Convex hull and quadrilateral search ────────────────────────────────────

// convexHull returns the hull in counter-clockwise order (Andrew's monotone
// chain).
func convexHull(pts []Point2) []Point2 {
	if len(pts) < 3 {
		return nil
	}
	p := make([]Point2, len(pts))
	copy(p, pts)
	sort.Slice(p, func(i, j int) bool {
		if p[i].X != p[j].X {
			return p[i].X < p[j].X
		}
		return p[i].Y < p[j].Y
	})
	cross := func(o, a, b Point2) float64 {
		return (a.X-o.X)*(b.Y-o.Y) - (a.Y-o.Y)*(b.X-o.X)
	}
	build := func(src []Point2) []Point2 {
		var out []Point2
		for _, q := range src {
			for len(out) >= 2 && cross(out[len(out)-2], out[len(out)-1], q) <= 0 {
				out = out[:len(out)-1]
			}
			out = append(out, q)
		}
		return out[:len(out)-1]
	}
	lower := build(p)
	rev := make([]Point2, len(p))
	for i := range p {
		rev[i] = p[len(p)-1-i]
	}
	return append(lower, build(rev)...)
}

// candidateQuads returns quadrilaterals inscribed in the hull, largest area
// first. The board's outline is the largest; smaller ones are fallbacks for
// when clutter outside the board has distorted the hull.
func candidateQuads(hull []Point2) [][4]Point2 {
	// Cap the hull so the O(n⁴) enumeration stays trivial. A clean detection
	// gives a hull of four to eight points; a longer one means clutter, and the
	// points furthest from the centroid are the ones worth keeping.
	const maxHull = 16
	if len(hull) > maxHull {
		var cx, cy float64
		for _, p := range hull {
			cx += p.X
			cy += p.Y
		}
		cx, cy = cx/float64(len(hull)), cy/float64(len(hull))
		sort.SliceStable(hull, func(i, j int) bool {
			return math.Hypot(hull[i].X-cx, hull[i].Y-cy) > math.Hypot(hull[j].X-cx, hull[j].Y-cy)
		})
		hull = hull[:maxHull]
		// Restore hull order so the quads stay non-self-intersecting.
		hull = convexHull(hull)
	}

	n := len(hull)
	type cand struct {
		q    [4]Point2
		area float64
	}
	var out []cand
	for a := 0; a < n-3; a++ {
		for b := a + 1; b < n-2; b++ {
			for c := b + 1; c < n-1; c++ {
				for d := c + 1; d < n; d++ {
					q := [4]Point2{hull[a], hull[b], hull[c], hull[d]}
					out = append(out, cand{q, polyArea(q[:])})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].area > out[j].area })
	if len(out) > 40 {
		out = out[:40]
	}
	res := make([][4]Point2, len(out))
	for i, c := range out {
		res[i] = c.q
	}
	return res
}

// orientations returns the four ways a quadrilateral's vertices can be matched
// to the grid's four outer nodes: the four rotations, in the single winding a
// visible board can actually have.
//
// Excluding the other winding is a physical statement, not an optimisation. A
// checkerboard is mirror-symmetric, so the flipped labelling — the board turned
// over, 180° about an axis in its own plane — reproduces the image just as
// exactly, at a fraction of a pixel. It differs only in that its recovered
// normal points the other way. Left available, it gets chosen about half the
// time, and then the whole reference frame is upside down: rolling radii come
// out negative and every camber sign inverts. Nothing downstream can detect
// that, because the fit is perfect.
//
// Which winding is the real one follows from the board being opaque. The model
// frame is right-handed with +Z the outward normal of the printed face, so that
// face is visible only when its normal points back toward the camera — and a
// plane seen from the side its normal points at projects with its (X, Y) axes
// REVERSED in the image relative to the model's own winding. Hence the test:
// keep the image quad wound opposite to the model grid.
//
// Getting this backwards is not a subtle failure. It is the difference between
// the reference frame being right side up and upside down.
func orientations(q [4]Point2, modelSign float64) [][4]Point2 {
	base := q
	if signedArea(q)*modelSign > 0 {
		base = [4]Point2{q[3], q[2], q[1], q[0]}
	}
	out := make([][4]Point2, 0, 4)
	for s := 0; s < 4; s++ {
		out = append(out, [4]Point2{base[s%4], base[(s+1)%4], base[(s+2)%4], base[(s+3)%4]})
	}
	return out
}

// signedArea is the shoelace area of a quadrilateral, whose sign encodes the
// winding.
func signedArea(q [4]Point2) float64 {
	var a float64
	for i := range q {
		j := (i + 1) % len(q)
		a += q[i].X*q[j].Y - q[j].X*q[i].Y
	}
	return a / 2
}

// gridWindingSign is the winding of the model grid's four outer nodes, taken in
// the order assignByQuad matches them.
func gridWindingSign(cols, rows int) float64 {
	c, r := float64(cols-1), float64(rows-1)
	return signedArea([4]Point2{{0, 0}, {c, 0}, {c, r}, {0, r}})
}

func polyArea(p []Point2) float64 {
	var a float64
	for i := range p {
		j := (i + 1) % len(p)
		a += p[i].X*p[j].Y - p[j].X*p[i].Y
	}
	return math.Abs(a) / 2
}

// DetectAndSolve runs detection and pose estimation together — the whole
// optical stage for one frame.
func DetectAndSolve(img *Gray, cam Camera, opt DetectOptions) (Detection, PnPResult, error) {
	det, err := DetectCheckerboard(img, opt)
	if err != nil {
		return det, PnPResult{}, err
	}
	corr, err := opt.Target.Correspondences(det.Corners)
	if err != nil {
		return det, PnPResult{}, err
	}
	pose, err := SolvePnPPlanar(cam, corr)
	if err != nil {
		return det, pose, err
	}
	pose.Warnings = append(det.Warnings, pose.Warnings...)
	return det, pose, nil
}
