package vision

import (
	"fmt"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
)

// Linking several reference boards into one frame.
//
// A single board on the floor cannot be seen from all four wheels: photographing
// the rear left wheel puts the whole car between the camera and a board lying at
// the front. The practical answer is more than one reference board — one within
// sight of each pair of wheels — plus a few "link" photographs taken from far
// enough back that two boards appear together.
//
// A link photograph gives the two boards' poses in that camera's frame, and
// dividing one by the other gives their relative pose with the camera cancelled:
//
//	T_A→B = (T_cam→A)⁻¹ · T_cam→B
//
// That is an edge in a small graph whose nodes are the boards. Walking it from a
// chosen root puts every board — and so every wheel registered against any of
// them — into one frame. The camera never has to see everything at once, which
// is what makes a four-wheel optical alignment possible with one phone.

// LinkResult holds the recovered pose of every reference board in the root
// board's frame.
type LinkResult struct {
	// ToRoot[i] maps coordinates in reference board i into the root board's
	// frame. The root itself maps by the identity.
	ToRoot map[int]geom.Pose

	// Edges records which pairs were observed together, for reporting.
	Edges []LinkEdge

	Warnings []string
}

// LinkEdge is one observed relationship between two boards.
type LinkEdge struct {
	Frame  int     `json:"frame"`
	A, B   int     `json:"-"`
	RMSPx  float64 `json:"rms_px"`
	Detail string  `json:"detail"`
}

// LinkReferences relates several reference boards to the first one, from
// photographs that each show two or more of them.
//
// Boards must all differ in layout — two identical boards in one frame cannot be
// told apart. Any board never seen together with the others, directly or through
// a chain, cannot be placed and is reported as such rather than guessed at.
func LinkReferences(cam Camera, refs []Target, imgs []*Gray, opt DetectOptions) (LinkResult, error) {
	if len(refs) == 0 {
		return LinkResult{}, fmt.Errorf("не задано ни одной напольной мишени")
	}
	for i, t := range refs {
		if err := t.Validate(); err != nil {
			return LinkResult{}, fmt.Errorf("напольная мишень %d: %w", i+1, err)
		}
		for j := i + 1; j < len(refs); j++ {
			if sameLayout(t, refs[j]) {
				return LinkResult{}, fmt.Errorf(
					"напольные мишени %d и %d одинаковые (%dx%d) — их невозможно различить в кадре",
					i+1, j+1, t.Cols, t.Rows)
			}
		}
	}

	res := LinkResult{ToRoot: map[int]geom.Pose{0: geom.IdentityPose()}}
	if len(refs) == 1 {
		return res, nil // nothing to link
	}

	// Collect edges: for every frame, every pair of boards seen together.
	type edge struct {
		to   int
		pose geom.Pose // maps `from` coordinates into `to` coordinates
	}
	adj := make(map[int][]edge)

	for fi, img := range imgs {
		dets, errs := DetectBoards(img, refs, opt)
		poses := make(map[int]geom.Pose)
		for i := range refs {
			if errs[i] != nil {
				continue
			}
			p, err := solveBoard(cam, refs[i], dets[i])
			if err != nil || p.Ambiguous {
				if p.Ambiguous {
					res.Warnings = append(res.Warnings, fmt.Sprintf(
						"Связующий снимок %d: поза мишени %dx%d неоднозначна, снимок не использован. "+
							"Снимайте ближе или под большим углом.", fi+1, refs[i].Cols, refs[i].Rows))
				}
				continue
			}
			poses[i] = p.Pose
		}
		if len(poses) < 2 {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"Связующий снимок %d: видно меньше двух напольных мишеней — он ничего не связывает.", fi+1))
			continue
		}
		for a, pa := range poses {
			for b, pb := range poses {
				if a == b {
					continue
				}
				// From a's coordinates into b's coordinates.
				adj[a] = append(adj[a], edge{to: b, pose: pb.Inverse().Mul(pa)})
			}
		}
		res.Edges = append(res.Edges, LinkEdge{
			Frame:  fi,
			Detail: fmt.Sprintf("видно мишеней: %d", len(poses)),
		})
	}

	// Walk outward from the root, composing transforms.
	queue := []int{0}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range adj[cur] {
			if _, done := res.ToRoot[e.to]; done {
				continue
			}
			// e.pose maps cur → e.to, so its inverse maps e.to → cur, and then
			// cur → root composes on the left.
			res.ToRoot[e.to] = res.ToRoot[cur].Mul(e.pose.Inverse())
			queue = append(queue, e.to)
		}
	}

	var missing []int
	for i := range refs {
		if _, ok := res.ToRoot[i]; !ok {
			missing = append(missing, i+1)
		}
	}
	if len(missing) > 0 {
		return res, fmt.Errorf(
			"напольные мишени %v не удалось связать с остальными: нужен снимок, где такая мишень видна "+
				"вместе с уже связанной", missing)
	}
	return res, nil
}

// InFrameOf re-expresses a registered wheel in another frame, given the pose
// that maps its current reference frame into that one. This is how a wheel
// registered against a rear floor board joins wheels registered against the
// front one.
func (w RegisteredWheel) InFrameOf(toRoot geom.Pose) RegisteredWheel {
	out := w
	out.Axis = toRoot.ApplyDir(w.Axis).Unit()
	out.Center = toRoot.Apply(w.Center)
	return out
}
