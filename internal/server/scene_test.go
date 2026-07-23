package server_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"testing"

	"github.com/AristarhUcolov/wheel-alignment/internal/geom"
	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// sceneBoard is one checkerboard placed in the camera's frame.
type sceneBoard struct {
	tg   vision.Target
	pose geom.Pose
}

// renderScenePNG draws several boards in one image, nearest surface winning,
// and encodes it as PNG so the server's own upload and decode path is exercised.
// Two boards in one frame is what registration rests on — the wheel target and
// the fixed floor reference have to be readable together.
func renderScenePNG(t *testing.T, cam vision.Camera, boards []sceneBoard, noise float64, rng *rand.Rand) []byte {
	t.Helper()
	const bg, super = 0.45, 3
	img := image.NewGray(image.Rect(0, 0, cam.Width, cam.Height))

	type prep struct {
		normal   geom.Vec3
		planeD   float64
		inv      geom.Pose
		s        float64
		x0, y0   float64
		wMM, hMM float64
	}
	ps := make([]prep, len(boards))
	for i, b := range boards {
		// A board with Cols inner corners needs Cols+1 squares, with the corners
		// on the square boundaries.
		s := b.tg.SquareMM
		wMM, hMM := float64(b.tg.Cols+1)*s, float64(b.tg.Rows+1)*s
		n := b.pose.R.Col(2)
		ps[i] = prep{n, b.pose.T.Dot(n), b.pose.Inverse(), s, -wMM / 2, -hMM / 2, wMM, hMM}
	}

	for py := 0; py < cam.Height; py++ {
		for px := 0; px < cam.Width; px++ {
			var sum float64
			for sy := 0; sy < super; sy++ {
				for sx := 0; sx < super; sx++ {
					// Gray.Sample places pixel (px,py)'s value AT (px,py), so the
					// area it averages is centred there — hence the −0.5.
					u := float64(px) - 0.5 + (float64(sx)+0.5)/super
					v := float64(py) - 0.5 + (float64(sy)+0.5)/super
					d := cam.Ray(vision.Point2{X: u, Y: v})

					best, bestT := -1, math.Inf(1)
					var bestQ geom.Vec3
					for i := range ps {
						den := d.Dot(ps[i].normal)
						if math.Abs(den) < 1e-9 {
							continue
						}
						tt := ps[i].planeD / den
						if tt <= 0 || tt >= bestT {
							continue
						}
						q := ps[i].inv.Apply(d.Scale(tt))
						if q.X < ps[i].x0 || q.Y < ps[i].y0 ||
							q.X >= ps[i].x0+ps[i].wMM || q.Y >= ps[i].y0+ps[i].hMM {
							continue
						}
						best, bestT, bestQ = i, tt, q
					}
					if best < 0 {
						sum += bg
						continue
					}
					ix := int(math.Floor((bestQ.X - ps[best].x0) / ps[best].s))
					iy := int(math.Floor((bestQ.Y - ps[best].y0) / ps[best].s))
					if (ix+iy)%2 == 0 {
						sum += 0.12
					} else {
						sum += 0.88
					}
				}
			}
			val := sum/(super*super) + rng.NormFloat64()*noise
			img.SetGray(px, py, color.Gray{Y: uint8(math.Max(0, math.Min(255, val*255)))})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// lookAtSrv builds a world→camera pose for a camera at `eye` aimed at `at`,
// held level.
func lookAtSrv(eye, at geom.Vec3) geom.Pose {
	fwd := at.Sub(eye).Unit()
	right := geom.V(0, 0, 1).Cross(fwd).Unit()
	down := fwd.Cross(right).Unit()
	r := geom.FromRows(right, down, fwd)
	return geom.Pose{R: r, T: r.MulVec(eye).Neg()}
}
