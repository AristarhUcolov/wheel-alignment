package vision

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
)

// Gray is a single-channel image with intensities in [0, 1].
//
// Floating point rather than bytes because every stage after loading —
// blurring, second derivatives, the quadratic surface fit that finds a corner
// to a hundredth of a pixel — works below the quantisation step of an 8-bit
// image. Rounding back to bytes anywhere in that chain would throw away
// precisely the precision the whole optical mode exists to gain.
type Gray struct {
	W, H int
	Pix  []float64
}

func NewGray(w, h int) *Gray {
	return &Gray{W: w, H: h, Pix: make([]float64, w*h)}
}

func (g *Gray) At(x, y int) float64 {
	if x < 0 {
		x = 0
	} else if x >= g.W {
		x = g.W - 1
	}
	if y < 0 {
		y = 0
	} else if y >= g.H {
		y = g.H - 1
	}
	return g.Pix[y*g.W+x]
}

func (g *Gray) Set(x, y int, v float64) {
	if x < 0 || y < 0 || x >= g.W || y >= g.H {
		return
	}
	g.Pix[y*g.W+x] = v
}

func (g *Gray) InBounds(x, y float64, margin float64) bool {
	return x >= margin && y >= margin && x < float64(g.W)-margin && y < float64(g.H)-margin
}

// Sample reads the image at fractional coordinates by bilinear interpolation.
func (g *Gray) Sample(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	ix, iy := int(x0), int(y0)
	a := g.At(ix, iy)*(1-fx) + g.At(ix+1, iy)*fx
	b := g.At(ix, iy+1)*(1-fx) + g.At(ix+1, iy+1)*fx
	return a*(1-fy) + b*fy
}

// FromImage converts any decoded image to grayscale, using the Rec. 601 luma
// weights that match how a camera's own monochrome conversion behaves.
func FromImage(src image.Image) *Gray {
	b := src.Bounds()
	g := NewGray(b.Dx(), b.Dy())
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, gg, bb, _ := src.At(x, y).RGBA() // 16-bit
			g.Pix[i] = (0.299*float64(r) + 0.587*float64(gg) + 0.114*float64(bb)) / 65535
			i++
		}
	}
	return g
}

// DecodeImage reads a PNG or JPEG.
func DecodeImage(r io.Reader) (*Gray, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("не удалось разобрать изображение: %w", err)
	}
	return FromImage(src), nil
}

// LoadImage reads a PNG or JPEG from disk.
func LoadImage(path string) (*Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть снимок: %w", err)
	}
	defer f.Close()
	return DecodeImage(f)
}

// Blur applies a separable Gaussian.
//
// A little blurring before differentiating is not optional: the second
// derivatives used to find corners amplify sensor noise enormously, and on a
// sharp image the response is dominated by grain rather than by the pattern.
// Too much blur and neighbouring corners merge, so the radius is tied to the
// expected square size by the caller.
func (g *Gray) Blur(sigma float64) *Gray {
	if sigma <= 0 {
		return g
	}
	radius := int(math.Ceil(3 * sigma))
	kernel := make([]float64, 2*radius+1)
	var sum float64
	for i := range kernel {
		d := float64(i - radius)
		kernel[i] = math.Exp(-d * d / (2 * sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}

	tmp := NewGray(g.W, g.H)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			var s float64
			for k, w := range kernel {
				s += w * g.At(x+k-radius, y)
			}
			tmp.Pix[y*g.W+x] = s
		}
	}
	out := NewGray(g.W, g.H)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			var s float64
			for k, w := range kernel {
				s += w * tmp.At(x, y+k-radius)
			}
			out.Pix[y*g.W+x] = s
		}
	}
	return out
}

// Normalize rescales intensities to span [0, 1], which makes response
// thresholds independent of exposure. A garage is badly lit and no two frames
// have the same brightness.
func (g *Gray) Normalize() *Gray {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range g.Pix {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	if hi-lo < 1e-9 {
		return g
	}
	out := NewGray(g.W, g.H)
	scale := 1 / (hi - lo)
	for i, v := range g.Pix {
		out.Pix[i] = (v - lo) * scale
	}
	return out
}
