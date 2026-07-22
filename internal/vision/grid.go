package vision

import (
	"math"
	"sort"
)

// Seed-and-grow recovery of the checkerboard's lattice.
//
// The convex-hull search this replaces asks a question the image cannot always
// answer: "which four points are the board's corners?" It answers by taking the
// largest quadrilateral inscribed in the hull of everything detected, which is
// right only when the detections are exactly the pattern. Let a handful of
// stray points survive the earlier filters and the board's real outline stops
// being the largest quadrilateral, at which point no amount of tuning helps —
// the premise is wrong, not the threshold.
//
// Growing asks a local question instead. Take two adjacent corners; the next
// one along is where the pattern says it should be, and the pattern is a
// lattice, so its position is predictable from the neighbours already found.
// Snap to the nearest real detection, repeat. Clutter is simply never reached:
// it is not adjacent to anything in the lattice, so it never gets asked about.
// The number of stray points in the frame stops mattering entirely.

// cell is a lattice coordinate during growth. The origin is wherever the seed
// happened to land and the axes are whichever two directions it happened to
// pick; both are normalised away at the end.
type cell struct{ i, j int }

// lattice is a partially grown grid.
type lattice struct {
	pts   []Point2
	at    map[cell]int // lattice coordinate → index into pts
	taken map[int]bool // which detections are already placed
}

func newLattice(pts []Point2) *lattice {
	return &lattice{pts: pts, at: map[cell]int{}, taken: map[int]bool{}}
}

func (l *lattice) point(c cell) (Point2, bool) {
	i, ok := l.at[c]
	if !ok {
		return Point2{}, false
	}
	return l.pts[i], true
}

func (l *lattice) place(c cell, idx int) {
	l.at[c] = idx
	l.taken[idx] = true
}

// growGridQuads recovers the board's lattice and returns candidate outlines —
// each a set of four corner points that might be the board.
//
// Only outlines are returned, and deliberately so: the caller feeds each to the
// same orientation search and coverage check the hull path used, so everything
// downstream — the eight labellings, the completeness test, the half-turn
// resolution from the pattern's colours — stays exactly as it was and as it was
// tested. Growth replaces the one fragile step and nothing else, and the
// caller's validation is what decides which candidate is real.
//
// More than one candidate arises because growth does not always stop at the
// inner corners. A stray point or a perimeter T-junction that slipped through
// the filters sits right on the continuation of the lattice, so the grown grid
// can come out a row or a column too big. The board's inner corners are then a
// cols×rows window inside it, and every fully-filled window of that size is a
// candidate worth handing to the caller to confirm or reject.
func growGridQuads(pts []Point2, cols, rows int) [][4]Point2 {
	if len(pts) < cols*rows {
		return nil
	}
	nbrs := nearestLists(pts, 8)

	var quads [][4]Point2
	seen := map[[4]Point2]bool{}
	add := func(q [4]Point2, ok bool) {
		if ok && !seen[q] {
			seen[q] = true
			quads = append(quads, q)
		}
	}

	// Seeds are tried strongest-first — the input arrives ordered by response.
	// An exact-size lattice is the clean case and comes first; windows of a
	// larger lattice are the fallback for when growth overran.
	for s := range pts {
		if len(nbrs[s]) < 4 {
			continue
		}
		for _, basis := range seedBases(pts, nbrs, s) {
			l := newLattice(pts)
			l.place(cell{0, 0}, s)
			l.place(cell{1, 0}, basis[0])
			l.place(cell{0, 1}, basis[1])
			grow(l, nbrs)

			add(latticeQuad(l, cols, rows))
			for _, q := range windowQuads(l, cols, rows) {
				add(q, true)
			}
		}
		if len(quads) > 24 {
			break
		}
	}
	return quads
}

// nearestLists returns each point's k nearest neighbours, closest first.
func nearestLists(pts []Point2, k int) [][]int {
	out := make([][]int, len(pts))
	type nd struct {
		idx int
		d   float64
	}
	buf := make([]nd, 0, len(pts))

	for i, p := range pts {
		buf = buf[:0]
		for j, q := range pts {
			if i != j {
				buf = append(buf, nd{j, p.DistTo(q)})
			}
		}
		sort.Slice(buf, func(a, b int) bool { return buf[a].d < buf[b].d })
		n := k
		if len(buf) < n {
			n = len(buf)
		}
		ids := make([]int, n)
		for m := 0; m < n; m++ {
			ids[m] = buf[m].idx
		}
		out[i] = ids
	}
	return out
}

// seedBases proposes pairs of neighbours that could be the seed's two lattice
// directions: comparable in length and roughly perpendicular.
//
// The tolerances are wide because perspective is wide. A board tilted 45° turns
// its right angles into 60° and 120° on screen and compresses one axis by a
// third, and a calibration shot is supposed to be tilted like that. What the
// bounds still exclude is a diagonal neighbour, which sits at √2 the spacing
// and 45° off.
func seedBases(pts []Point2, nbrs [][]int, s int) [][2]int {
	p := pts[s]
	var out [][2]int
	list := nbrs[s]

	for ai := 0; ai < len(list); ai++ {
		a := pts[list[ai]]
		la := p.DistTo(a)
		if la < 1e-9 {
			continue
		}
		for bi := ai + 1; bi < len(list); bi++ {
			b := pts[list[bi]]
			lb := p.DistTo(b)
			if lb < 1e-9 {
				continue
			}
			if ratio := lb / la; ratio < 0.55 || ratio > 1.8 {
				continue
			}
			ux, uy := (a.X-p.X)/la, (a.Y-p.Y)/la
			vx, vy := (b.X-p.X)/lb, (b.Y-p.Y)/lb
			cos := ux*vx + uy*vy
			if math.Abs(cos) > 0.62 { // outside roughly 52°…128°
				continue
			}
			// Keep a consistent winding so the lattice axes do not flip between
			// seeds; the caller's orientation search handles the rest.
			if ux*vy-uy*vx < 0 {
				out = append(out, [2]int{list[bi], list[ai]})
			} else {
				out = append(out, [2]int{list[ai], list[bi]})
			}
		}
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

// grow extends the lattice until nothing more can be added.
func grow(l *lattice, nbrs [][]int) {
	for pass := 0; pass < 200; pass++ {
		added := 0
		// Collect the empty cells adjacent to something already placed.
		frontier := map[cell]bool{}
		for c := range l.at {
			for _, d := range [4]cell{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				n := cell{c.i + d.i, c.j + d.j}
				if _, filled := l.at[n]; !filled {
					frontier[n] = true
				}
			}
		}
		for c := range frontier {
			if l.tryFill(c) {
				added++
			}
		}
		if added == 0 {
			return
		}
	}
}

// tryFill predicts where a lattice cell should land and snaps to the nearest
// unused detection.
func (l *lattice) tryFill(c cell) bool {
	pred, scale, ok := l.predict(c)
	if !ok {
		return false
	}
	// A third of the local spacing: loose enough for perspective and for a
	// corner located to a tenth of a pixel, tight enough that the neighbouring
	// lattice site is never a candidate.
	tol := 0.34 * scale

	best, bestD := -1, math.Inf(1)
	for i, p := range l.pts {
		if l.taken[i] {
			continue
		}
		if d := p.DistTo(pred); d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 || bestD > tol {
		return false
	}
	l.place(c, best)
	return true
}

// predict estimates where cell c lies, from whichever neighbours are known.
//
// The parallelogram rule is tried first and is the workhorse: three corners of
// a lattice square give the fourth exactly under any affine map, and projection
// is affine to first order over one square. Collinear extrapolation along a row
// or column is the fallback for growing off an edge, where no such triple
// exists.
func (l *lattice) predict(c cell) (Point2, float64, bool) {
	// Parallelogram: c ≈ A + B − corner, over each of the four quadrants.
	for _, d := range [4]cell{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		a, okA := l.point(cell{c.i + d.i, c.j})
		b, okB := l.point(cell{c.i, c.j + d.j})
		o, okO := l.point(cell{c.i + d.i, c.j + d.j})
		if okA && okB && okO {
			return Point2{a.X + b.X - o.X, a.Y + b.Y - o.Y},
				math.Min(a.DistTo(o), b.DistTo(o)), true
		}
	}
	// Collinear: two in a row give the next by reflection.
	for _, d := range [4]cell{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		p1, ok1 := l.point(cell{c.i + d.i, c.j + d.j})
		p2, ok2 := l.point(cell{c.i + 2*d.i, c.j + 2*d.j})
		if ok1 && ok2 {
			return Point2{2*p1.X - p2.X, 2*p1.Y - p2.Y}, p1.DistTo(p2), true
		}
	}
	return Point2{}, 0, false
}

// latticeBounds returns the grown lattice's extent.
func latticeBounds(l *lattice) (minI, minJ, w, h int) {
	minI, maxI := math.MaxInt32, math.MinInt32
	minJ, maxJ := math.MaxInt32, math.MinInt32
	for c := range l.at {
		minI, maxI = min(minI, c.i), max(maxI, c.i)
		minJ, maxJ = min(minJ, c.j), max(maxJ, c.j)
	}
	return minI, minJ, maxI - minI + 1, maxJ - minJ + 1
}

// cornerQuad reads the four corners of the cols×rows block whose origin is
// (i0,j0), requiring every one of its nodes to be present.
func cornerQuad(l *lattice, i0, j0, cols, rows int) ([4]Point2, bool) {
	for dj := 0; dj < rows; dj++ {
		for di := 0; di < cols; di++ {
			if _, ok := l.at[cell{i0 + di, j0 + dj}]; !ok {
				return [4]Point2{}, false
			}
		}
	}
	corners := [4]cell{
		{i0, j0}, {i0 + cols - 1, j0}, {i0 + cols - 1, j0 + rows - 1}, {i0, j0 + rows - 1},
	}
	var quad [4]Point2
	for k, c := range corners {
		quad[k] = l.pts[l.at[c]]
	}
	return quad, true
}

// latticeQuad returns the outline when the grown lattice is exactly the target
// size — the clean case, no window search needed.
func latticeQuad(l *lattice, cols, rows int) ([4]Point2, bool) {
	if len(l.at) < cols*rows {
		return [4]Point2{}, false
	}
	i0, j0, w, h := latticeBounds(l)
	if len(l.at) != w*h {
		return [4]Point2{}, false // holes, or an L-shape
	}
	// Axes may have grown either way round; both are fine, the orientation
	// search downstream sorts it out.
	if w == cols && h == rows {
		return cornerQuad(l, i0, j0, cols, rows)
	}
	if w == rows && h == cols {
		return cornerQuad(l, i0, j0, rows, cols)
	}
	return [4]Point2{}, false
}

// windowQuads returns the outline of every fully-filled cols×rows block inside a
// lattice that grew larger than the target. This is what recovers the board
// when growth overran onto strays or perimeter junctions: the board's inner
// corners are one such window, and the caller's coverage check rejects any
// window that is not actually the pattern.
func windowQuads(l *lattice, cols, rows int) [][4]Point2 {
	i0, j0, w, h := latticeBounds(l)
	if w <= cols && h <= rows { // not larger than target in any axis
		return nil
	}
	var out [][4]Point2
	for _, dims := range [2][2]int{{cols, rows}, {rows, cols}} {
		bw, bh := dims[0], dims[1]
		if bw > w || bh > h {
			continue
		}
		for j := j0; j+bh-1 < j0+h; j++ {
			for i := i0; i+bw-1 < i0+w; i++ {
				if q, ok := cornerQuad(l, i, j, bw, bh); ok {
					out = append(out, q)
				}
			}
		}
	}
	return out
}
