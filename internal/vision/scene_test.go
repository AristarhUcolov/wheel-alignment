package vision_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// placedBoard is one checkerboard somewhere in the world.
type placedBoard struct {
	tg   vision.Target
	pose geom.Pose // board → camera frame
}

// renderScene draws several boards in one image, with the nearest surface
// winning each ray. Two boards in one frame is the whole basis of registration —
// the wheel target and the fixed floor reference must be readable together — so
// the test scene has to be able to produce that, not just single-board images.
//
// There is no car body in the scene, so nothing occludes anything. That is a
// convenience of the simulation and it is worth being clear about: it validates
// the geometry of registration, not whether a given board placement is actually
// visible past real sheet metal. Choosing placements the camera can really see
// is the operator's problem, and the reason reference linking exists.
func renderScene(cam vision.Camera, boards []placedBoard, super int, noise float64, rng *rand.Rand) *vision.Gray {
	const bg = 0.45
	img := vision.NewGray(cam.Width, cam.Height)

	type prepared struct {
		normal   geom.Vec3
		planeD   float64
		inv      geom.Pose
		s        float64
		x0, y0   float64
		wMM, hMM float64
	}
	prep := make([]prepared, len(boards))
	for i, b := range boards {
		s := b.tg.SquareMM
		wMM, hMM := float64(b.tg.Cols+1)*s, float64(b.tg.Rows+1)*s
		n := b.pose.R.Col(2)
		prep[i] = prepared{
			normal: n, planeD: b.pose.T.Dot(n), inv: b.pose.Inverse(),
			s: s, x0: -wMM / 2, y0: -hMM / 2, wMM: wMM, hMM: hMM,
		}
	}

	step := 1 / float64(super)
	for py := 0; py < cam.Height; py++ {
		for px := 0; px < cam.Width; px++ {
			var sum float64
			for sy := 0; sy < super; sy++ {
				for sx := 0; sx < super; sx++ {
					u := float64(px) - 0.5 + (float64(sx)+0.5)*step
					v := float64(py) - 0.5 + (float64(sy)+0.5)*step
					d := cam.Ray(vision.Point2{X: u, Y: v})

					best, bestT := -1, math.Inf(1)
					var bestQ geom.Vec3
					for i := range prep {
						den := d.Dot(prep[i].normal)
						if math.Abs(den) < 1e-9 {
							continue
						}
						t := prep[i].planeD / den
						if t <= 0 || t >= bestT {
							continue
						}
						q := prep[i].inv.Apply(d.Scale(t))
						if q.X < prep[i].x0 || q.Y < prep[i].y0 ||
							q.X >= prep[i].x0+prep[i].wMM || q.Y >= prep[i].y0+prep[i].hMM {
							continue
						}
						best, bestT, bestQ = i, t, q
					}
					if best < 0 {
						sum += bg
						continue
					}
					ix := int(math.Floor((bestQ.X - prep[best].x0) / prep[best].s))
					iy := int(math.Floor((bestQ.Y - prep[best].y0) / prep[best].s))
					if (ix+iy)%2 == 0 {
						sum += 0.12
					} else {
						sum += 0.88
					}
				}
			}
			val := sum/float64(super*super) + rng.NormFloat64()*noise
			img.Set(px, py, math.Max(0, math.Min(1, val)))
		}
	}
	return img
}

// faceCamera turns a board over if its printed face would otherwise point away
// from the camera.
//
// The renderers here draw a board's pattern from either side, because they do
// not cull back faces. A real board is opaque: photographed from behind it shows
// a blank back, and no detector could read it. So a scene that places a board
// facing away is not a scene that can exist, and the detector is right to refuse
// the labelling it implies — it is the labelling of a board seen from behind.
// Rather than weaken that rule, scenes are built facing the camera.
//
// Turning it over is a proper rotation (180° about the board's own Y axis), so
// the result is a real pose, and callers must use the returned pose for both
// rendering and ground truth.
func faceCamera(p geom.Pose) geom.Pose {
	if p.R.Col(2).Dot(p.T) < 0 {
		return p // normal already points back toward the camera at the origin
	}
	return geom.Pose{R: p.R.Mul(geom.RotY(math.Pi)), T: p.T}
}

// lookAt builds a world→camera pose for a camera at `eye` aimed at `at`, kept
// level (no roll) by using world up as the reference.
func lookAt(eye, at geom.Vec3) geom.Pose {
	fwd := at.Sub(eye).Unit()
	worldUp := geom.V(0, 0, 1)
	right := worldUp.Cross(fwd).Unit()
	down := fwd.Cross(right).Unit()
	r := geom.FromRows(right, down, fwd) // camera axes as rows
	return geom.Pose{R: r, T: r.MulVec(eye).Neg()}
}

// assertVisible checks a board projects fully inside the frame, so a test that
// fails does so for a real reason and not because the scene was framed badly.
func assertVisible(t *testing.T, cam vision.Camera, b placedBoard, what string) {
	t.Helper()
	pts, ok := cam.ProjectPose(b.pose, b.tg.ModelPoints())
	if !ok {
		t.Fatalf("%s: не проецируется (за камерой)", what)
	}
	for _, p := range pts {
		if p.X < 5 || p.Y < 5 || p.X > float64(cam.Width-5) || p.Y > float64(cam.Height-5) {
			t.Fatalf("%s: выходит за кадр (%.0f, %.0f)", what, p.X, p.Y)
		}
	}
}
